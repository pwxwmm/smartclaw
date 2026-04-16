package fingerprint

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type FingerprintEngine struct {
	mu    sync.RWMutex
	db    *sql.DB
	cache map[string]*IncidentFingerprint

	incidentStore IncidentStore
}

func NewFingerprintEngine(db *sql.DB) *FingerprintEngine {
	return &FingerprintEngine{
		db:    db,
		cache: make(map[string]*IncidentFingerprint),
	}
}

func Shutdown() {
}

func (e *FingerprintEngine) SetIncidentStore(is IncidentStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.incidentStore = is
}

func (e *FingerprintEngine) getIncidentStore() IncidentStore {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.incidentStore
}

func (e *FingerprintEngine) StoreFingerprint(data IncidentData) (*IncidentFingerprint, error) {
	fp := GenerateFingerprint(data)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := saveFingerprint(e.db, &fp); err != nil {
			return nil, fmt.Errorf("fingerprint: store: %w", err)
		}
	}

	e.cache[fp.IncidentID] = &fp

	if len(e.cache) > config.MaxCacheSize {
		var oldest string
		var oldestTime time.Time
		first := true
		for k, v := range e.cache {
			if first || v.GeneratedAt.Before(oldestTime) {
				oldest = k
				oldestTime = v.GeneratedAt
				first = false
			}
		}
		delete(e.cache, oldest)
	}

	metricFingerprintStored.Inc()
	metricFingerprintCacheSize.Set(float64(len(e.cache)))

	return &fp, nil
}

func (e *FingerprintEngine) SearchSimilar(incidentID string, threshold float64, limit int) ([]SimilarityResult, error) {
	metricFingerprintSearches.Inc()

	if limit <= 0 {
		limit = 10
	}

	e.mu.RLock()
	targetFP, ok := e.cache[incidentID]
	if !ok {
		e.mu.RUnlock()
		if e.db != nil {
			loaded, err := loadFingerprint(e.db, incidentID)
			if err != nil {
				return nil, fmt.Errorf("fingerprint: search: load target: %w", err)
			}
			if loaded == nil {
				return nil, fmt.Errorf("fingerprint: incident %s not found", incidentID)
			}
			e.mu.Lock()
			e.cache[incidentID] = loaded
			e.mu.Unlock()
			targetFP = loaded
		} else {
			return nil, fmt.Errorf("fingerprint: incident %s not found", incidentID)
		}
	} else {
		e.mu.RUnlock()
	}

	return e.SearchByVector(targetFP.Vector, threshold, limit)
}

func (e *FingerprintEngine) SearchByVector(vector [VectorSize]float64, threshold float64, limit int) ([]SimilarityResult, error) {
	metricFingerprintSearches.Inc()

	if limit <= 0 {
		limit = 10
	}

	e.mu.RLock()
	candidates := make(map[string]*IncidentFingerprint, len(e.cache))
	for k, v := range e.cache {
		candidates[k] = v
	}
	e.mu.RUnlock()

	type scored struct {
		id         string
		similarity float64
		vector     [VectorSize]float64
	}

	var results []scored
	for id, fp := range candidates {
		sim := CosineSimilarity(vector, fp.Vector)
		if sim >= threshold {
			results = append(results, scored{id: id, similarity: sim, vector: fp.Vector})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].similarity > results[j].similarity
	})

	if len(results) > limit {
		results = results[:limit]
	}

	is := e.getIncidentStore()

	output := make([]SimilarityResult, 0, len(results))
	for _, r := range results {
		featureMatch := identifyFeatureMatch(vector, r.vector, 0.8)

		result := SimilarityResult{
			IncidentID:   r.id,
			Similarity:   r.similarity,
			FeatureMatch: featureMatch,
		}

		if is != nil {
			if brief, err := is.GetIncident(r.id); err == nil && brief != nil {
				result.IncidentTitle = brief.Title
				result.IncidentSeverity = brief.Severity
				result.IncidentService = brief.Service
			}
		}

		output = append(output, result)
	}

	return output, nil
}

func (e *FingerprintEngine) GetFingerprint(incidentID string) (*IncidentFingerprint, error) {
	e.mu.RLock()
	fp, ok := e.cache[incidentID]
	if ok {
		e.mu.RUnlock()
		return fp, nil
	}
	e.mu.RUnlock()

	if e.db == nil {
		return nil, nil
	}

	loaded, err := loadFingerprint(e.db, incidentID)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: get: %w", err)
	}

	if loaded != nil {
		e.mu.Lock()
		e.cache[incidentID] = loaded
		e.mu.Unlock()
	}

	return loaded, nil
}

func (e *FingerprintEngine) LoadCache() error {
	if e.db == nil {
		return nil
	}

	fingerprints, err := loadAllFingerprints(e.db)
	if err != nil {
		return fmt.Errorf("fingerprint: load cache: %w", err)
	}

	e.mu.Lock()
	e.cache = fingerprints
	e.mu.Unlock()

	metricFingerprintCacheSize.Set(float64(len(fingerprints)))

	slog.Info("fingerprint: loaded cache", "count", len(fingerprints))
	return nil
}

func CosineSimilarity(a, b [VectorSize]float64) float64 {
	var dotProduct, normA, normB float64
	for i := 0; i < VectorSize; i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func identifyFeatureMatch(a, b [VectorSize]float64, categoryThreshold float64) string {
	var matching []string

	// Order matches the CategoryRanges map iteration; sort for deterministic output
	categories := []string{"temporal", "severity", "topology", "service", "impact", "response", "category", "label"}

	for _, cat := range categories {
		r, ok := CategoryRanges[cat]
		if !ok {
			continue
		}
		start, end := r[0], r[1]

		var dotProd, normA, normB float64
		for i := start; i < end; i++ {
			dotProd += a[i] * b[i]
			normA += a[i] * a[i]
			normB += b[i] * b[i]
		}

		if normA == 0 || normB == 0 {
			continue
		}

		catSim := dotProd / (math.Sqrt(normA) * math.Sqrt(normB))
		if catSim >= categoryThreshold {
			matching = append(matching, cat)
		}
	}

	return strings.Join(matching, ",")
}

func InitFingerprintEngine(db *sql.DB, incidentStore IncidentStore) (*FingerprintEngine, error) {
	e := NewFingerprintEngine(db)
	if err := initDB(db); err != nil {
		return nil, fmt.Errorf("fingerprint: init tables: %w", err)
	}
	if incidentStore != nil {
		e.SetIncidentStore(incidentStore)
	}
	if err := e.LoadCache(); err != nil {
		slog.Warn("fingerprint: cache load failed, continuing", "error", err)
	}
	SetFingerprintEngine(e)
	return e, nil
}

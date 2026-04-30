package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type EmbeddingRecord struct {
	ID         int64
	SourceType string
	SourceID   string
	Content    string
	Vector     Vector
	CreatedAt  time.Time
}

type SemanticSearchResult struct {
	SourceType string
	SourceID   string
	Content    string
	Score      float64
}

func vectorToBlob(v Vector) ([]byte, error) {
	return json.Marshal(v)
}

func blobToVector(data []byte) (Vector, error) {
	var v Vector
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Store) StoreEmbedding(ctx context.Context, sourceType, sourceID, content string, vector Vector) error {
	blob, err := vectorToBlob(vector)
	if err != nil {
		return fmt.Errorf("store: encode vector: %w", err)
	}

	return s.WriteWithRetry(ctx, `
		INSERT INTO embeddings (source_type, source_id, content, vector, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_type, source_id) DO UPDATE SET
			content = excluded.content,
			vector = excluded.vector,
			created_at = excluded.created_at
	`, sourceType, sourceID, content, blob, time.Now().Format("2006-01-02 15:04:05"))
}

func (s *Store) SearchEmbeddings(ctx context.Context, queryVector Vector, limit int, minScore float64) ([]SemanticSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if minScore <= 0 {
		minScore = 0.5
	}

	rows, err := s.db.QueryContext(ctx, `SELECT source_type, source_id, content, vector FROM embeddings`)
	if err != nil {
		return nil, fmt.Errorf("store: search embeddings: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		SourceType string
		SourceID   string
		Content    string
		Score      float64
	}
	var candidates []candidate

	for rows.Next() {
		var sourceType, sourceID, content string
		var blob []byte
		if err := rows.Scan(&sourceType, &sourceID, &content, &blob); err != nil {
			continue
		}
		vec, err := blobToVector(blob)
		if err != nil {
			continue
		}
		score := CosineSimilarity(queryVector, vec)
		if score >= minScore {
			candidates = append(candidates, candidate{
				SourceType: sourceType,
				SourceID:   sourceID,
				Content:    content,
				Score:      score,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]SemanticSearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SemanticSearchResult{
			SourceType: c.SourceType,
			SourceID:   c.SourceID,
			Content:    c.Content,
			Score:      c.Score,
		}
	}
	return results, nil
}

func (s *Store) DeleteEmbedding(ctx context.Context, sourceType, sourceID string) error {
	return s.WriteWithRetry(ctx, `DELETE FROM embeddings WHERE source_type = ? AND source_id = ?`, sourceType, sourceID)
}

func (s *Store) GetEmbedding(ctx context.Context, sourceType, sourceID string) (*EmbeddingRecord, error) {
	var rec EmbeddingRecord
	var blob []byte
	var createdAt sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, source_type, source_id, content, vector, created_at FROM embeddings WHERE source_type = ? AND source_id = ?`,
		sourceType, sourceID,
	).Scan(&rec.ID, &rec.SourceType, &rec.SourceID, &rec.Content, &blob, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get embedding: %w", err)
	}

	vec, err := blobToVector(blob)
	if err != nil {
		return nil, fmt.Errorf("store: decode vector: %w", err)
	}
	rec.Vector = vec

	if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
		rec.CreatedAt = t
	}
	return &rec, nil
}

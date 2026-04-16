package alertengine

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Correlate runs the three-stage correlation pipeline:
// 1. Fingerprint deduplication
// 2. Time-window grouping
// 3. Topology-aware correlation
func (e *AlertEngine) Correlate() *CorrelationResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	deduped := e.dedupStage()
	timeGroups := e.timeWindowStage(deduped)
	groups, unmatched := e.topologyStage(timeGroups)

	totalUnmatched := 0
	for _, da := range unmatched {
		groups = append(groups, AlertGroup{
			ID:          uuid.New().String(),
			Alerts:      []DedupedAlert{da},
			RootAlert:   &da,
			Correlation: "unmatched",
			Score:       0,
			CreatedAt:   time.Now(),
			Services:    []string{da.Service},
		})
		totalUnmatched++
	}

	e.groups = groups

	metricAlertGroups.Set(float64(len(groups)))

	return &CorrelationResult{
		Groups:    groups,
		Unmatched: unmatched,
		Stats: CorrelationStats{
			TotalRaw:       len(e.rawAlerts),
			TotalDeduped:   len(deduped),
			TotalGroups:    len(groups),
			TotalUnmatched: totalUnmatched,
		},
	}
}

// dedupStage groups raw alerts by fingerprint and merges them into DedupedAlerts.
// If count > autoEscalateThreshold within dedupWindow, severity is escalated.
func (e *AlertEngine) dedupStage() []DedupedAlert {
	result := make([]DedupedAlert, 0, len(e.deduped))

	for fp, da := range e.deduped {
		d := *da

		countInWindow := 0
		cutoff := time.Now().Add(-e.dedupWindow)
		for _, a := range e.rawAlerts {
			if a.Fingerprint == fp && a.FiredAt.After(cutoff) {
				countInWindow++
			}
		}

		if countInWindow > e.autoEscalateThreshold {
			d.Severity = EscalateSeverity(d.Severity)
		}

		result = append(result, d)
	}

	sort.Slice(result, func(i, j int) bool {
		if SeverityLevel(result[i].Severity) != SeverityLevel(result[j].Severity) {
			return SeverityLevel(result[i].Severity) > SeverityLevel(result[j].Severity)
		}
		return result[i].FirstFiredAt.Before(result[j].FirstFiredAt)
	})

	return result
}

// timeWindowStage groups DedupedAlerts that fired within corrWindow of each other.
func (e *AlertEngine) timeWindowStage(deduped []DedupedAlert) []AlertGroup {
	if len(deduped) == 0 {
		return nil
	}

	sorted := make([]DedupedAlert, len(deduped))
	copy(sorted, deduped)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FirstFiredAt.Before(sorted[j].FirstFiredAt)
	})

	var groups []AlertGroup
	currentGroup := []DedupedAlert{sorted[0]}
	groupStart := sorted[0].FirstFiredAt

	for i := 1; i < len(sorted); i++ {
		delta := sorted[i].FirstFiredAt.Sub(groupStart)
		if delta <= e.corrWindow {
			currentGroup = append(currentGroup, sorted[i])
		} else {
			groups = append(groups, e.buildTimeWindowGroup(currentGroup))
			currentGroup = []DedupedAlert{sorted[i]}
			groupStart = sorted[i].FirstFiredAt
		}
	}
	groups = append(groups, e.buildTimeWindowGroup(currentGroup))

	return groups
}

func (e *AlertEngine) buildTimeWindowGroup(alerts []DedupedAlert) AlertGroup {
	sorted := make([]DedupedAlert, len(alerts))
	copy(sorted, alerts)
	sort.Slice(sorted, func(i, j int) bool {
		return CompareSeverity(sorted[i].Severity, sorted[j].Severity)
	})

	root := sorted[0]
	services := uniqueServices(sorted)

	maxDelta := time.Duration(0)
	earliest := sorted[0].FirstFiredAt
	latest := sorted[0].FirstFiredAt
	for _, a := range sorted {
		if a.FirstFiredAt.Before(earliest) {
			earliest = a.FirstFiredAt
		}
		if a.LastFiredAt.After(latest) {
			latest = a.LastFiredAt
		}
	}
	if maxDelta == 0 {
		maxDelta = latest.Sub(earliest)
	}

	var score float64
	if e.corrWindow > 0 {
		score = 1.0 - float64(maxDelta)/float64(e.corrWindow)
	}
	if score < 0 {
		score = 0
	}

	return AlertGroup{
		ID:          uuid.New().String(),
		Alerts:      sorted,
		RootAlert:   &root,
		Correlation: "time_window",
		Score:       score,
		CreatedAt:   time.Now(),
		Services:    services,
	}
}

// topologyStage checks topology proximity between groups and merges
// groups whose alerts affect services within 2 hops.
func (e *AlertEngine) topologyStage(groups []AlertGroup) ([]AlertGroup, []DedupedAlert) {
	if e.topology == nil {
		return groups, nil
	}

	serviceToGroups := make(map[string][]int)
	for i, g := range groups {
		for _, svc := range g.Services {
			serviceToGroups[svc] = append(serviceToGroups[svc], i)
		}
	}

	parent := make([]int, len(groups))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i, g := range groups {
		for _, svc := range g.Services {
			neighbors, err := e.topology.GetNeighbors(svc, 2)
			if err != nil {
				continue
			}
			for _, neighbor := range neighbors {
				if connectedGroups, ok := serviceToGroups[neighbor]; ok {
					for _, j := range connectedGroups {
						if i != j {
							union(i, j)
						}
					}
				}
			}
		}
	}

	rootGroups := make(map[int][]int)
	for i := range groups {
		root := find(i)
		rootGroups[root] = append(rootGroups[root], i)
	}

	var merged []AlertGroup
	var unmatched []DedupedAlert

	for _, indices := range rootGroups {
		if len(indices) == 1 {
			g := groups[indices[0]]
			if len(g.Alerts) == 1 {
				unmatched = append(unmatched, g.Alerts[0])
			} else {
				proximity := e.computeTopologyProximity(g.Services)
				if proximity > 0 {
					g.Correlation = "topology"
					g.Score = g.Score + 0.3*proximity
					if g.Score > 1.0 {
						g.Score = 1.0
					}
				}
				merged = append(merged, g)
			}
			continue
		}

		var allAlerts []DedupedAlert
		var allServices []string
		var bestScore float64

		for _, idx := range indices {
			allAlerts = append(allAlerts, groups[idx].Alerts...)
			allServices = append(allServices, groups[idx].Services...)
			if groups[idx].Score > bestScore {
				bestScore = groups[idx].Score
			}
		}

		proximity := e.computeTopologyProximity(allServices)
		boostedScore := bestScore + 0.3*proximity
		if boostedScore > 1.0 {
			boostedScore = 1.0
		}

		sort.Slice(allAlerts, func(i, j int) bool {
			return CompareSeverity(allAlerts[i].Severity, allAlerts[j].Severity)
		})

		root := allAlerts[0]
		merged = append(merged, AlertGroup{
			ID:          uuid.New().String(),
			Alerts:      allAlerts,
			RootAlert:   &root,
			Correlation: "topology",
			Score:       boostedScore,
			CreatedAt:   time.Now(),
			Services:    uniqueServices(allAlerts),
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged, unmatched
}

// computeTopologyProximity returns a proximity score based on minimum
// hop count between any pair of services in the list.
func (e *AlertEngine) computeTopologyProximity(services []string) float64 {
	if e.topology == nil || len(services) < 2 {
		return 0
	}

	uniqueSvc := uniqueStrings(services)
	minHops := -1

	for i, svc := range uniqueSvc {
		neighbors, err := e.topology.GetNeighbors(svc, 2)
		if err != nil {
			continue
		}

		neighborSet := make(map[string]bool)
		for _, n := range neighbors {
			neighborSet[n] = true
		}

		for j, other := range uniqueSvc {
			if j <= i {
				continue
			}
			if other == svc {
				continue
			}
			if neighborSet[other] {
				if minHops == -1 || 1 < minHops {
					minHops = 1
				}
			}
		}
	}

	if minHops == -1 {
		for _, svc := range uniqueSvc {
			neighbors, err := e.topology.GetNeighbors(svc, 2)
			if err != nil {
				continue
			}
			for _, n := range neighbors {
				nNeighbors, err2 := e.topology.GetNeighbors(n, 1)
				if err2 != nil {
					continue
				}
				for _, nn := range nNeighbors {
					for _, other := range uniqueSvc {
						if other == svc || other == n {
							continue
						}
						if nn == other {
							if minHops == -1 || 2 < minHops {
								minHops = 2
							}
						}
					}
				}
			}
		}
	}

	if minHops == -1 {
		return 0
	}

	return 1.0 / (1.0 + float64(minHops))
}

func uniqueServices(alerts []DedupedAlert) []string {
	seen := make(map[string]bool)
	var result []string
	for _, a := range alerts {
		if !seen[a.Service] {
			seen[a.Service] = true
			result = append(result, a.Service)
		}
	}
	return result
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// ensure AlertEngine has String method for debugging
func (e *AlertEngine) String() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return fmt.Sprintf("AlertEngine{raw:%d, deduped:%d, groups:%d}",
		len(e.rawAlerts), len(e.deduped), len(e.groups))
}

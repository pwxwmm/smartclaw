package topology

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

const topologySchemaSQL = `
CREATE TABLE IF NOT EXISTS topology_nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    labels TEXT DEFAULT '{}',
    health TEXT DEFAULT 'unknown',
    last_seen DATETIME,
    metadata TEXT DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS topology_edges (
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    type TEXT NOT NULL,
    weight REAL DEFAULT 0.0,
    labels TEXT DEFAULT '{}',
    last_seen DATETIME,
    PRIMARY KEY (source, target, type)
);
CREATE INDEX IF NOT EXISTS idx_topology_nodes_type ON topology_nodes(type);
CREATE INDEX IF NOT EXISTS idx_topology_nodes_health ON topology_nodes(health);
CREATE INDEX IF NOT EXISTS idx_topology_edges_source ON topology_edges(source);
CREATE INDEX IF NOT EXISTS idx_topology_edges_target ON topology_edges(target);
`

// InitSchema creates the topology tables if they don't exist.
func InitSchema(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(topologySchemaSQL); err != nil {
		slog.Warn("topology: schema init failed", "error", err)
	}
}

// SnapshotGraph serializes the entire topology graph to SQLite.
func SnapshotGraph(graph *TopologyGraph, db *sql.DB) error {
	start := time.Now()

	if graph == nil || db == nil {
		return nil
	}

	InitSchema(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("topology snapshot: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM topology_edges"); err != nil {
		return fmt.Errorf("topology snapshot: clear edges: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM topology_nodes"); err != nil {
		return fmt.Errorf("topology snapshot: clear nodes: %w", err)
	}

	nodes := graph.AllNodes()
	for _, n := range nodes {
		labelsJSON, _ := json.Marshal(n.Labels)
		metadataJSON, _ := json.Marshal(n.Metadata)

		_, err := tx.Exec(
			`INSERT INTO topology_nodes (id, type, name, labels, health, last_seen, metadata)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			n.ID, string(n.Type), n.Name, string(labelsJSON), string(n.Health), n.LastSeen, string(metadataJSON),
		)
		if err != nil {
			return fmt.Errorf("topology snapshot: insert node %s: %w", n.ID, err)
		}
	}

	edges := graph.AllEdges()
	for _, e := range edges {
		labelsJSON, _ := json.Marshal(e.Labels)

		_, err := tx.Exec(
			`INSERT INTO topology_edges (source, target, type, weight, labels, last_seen)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			e.Source, e.Target, string(e.Type), e.Weight, string(labelsJSON), e.LastSeen,
		)
		if err != nil {
			return fmt.Errorf("topology snapshot: insert edge %s->%s: %w", e.Source, e.Target, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("topology snapshot: commit: %w", err)
	}

	slog.Debug("topology snapshot saved", "nodes", len(nodes), "edges", len(edges))
	metricSnapshotDuration.Observe(time.Since(start).Seconds())
	return nil
}

// LoadSnapshot deserializes a topology graph from SQLite.
func LoadSnapshot(db *sql.DB) (*TopologyGraph, error) {
	if db == nil {
		return nil, nil
	}

	InitSchema(db)

	graph := NewTopologyGraph()

	rows, err := db.Query(
		`SELECT id, type, name, labels, health, last_seen, metadata FROM topology_nodes`,
	)
	if err != nil {
		return nil, fmt.Errorf("topology load: query nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var n Node
		var typeStr, labelsJSON, healthStr, metadataJSON string
		if err := rows.Scan(&n.ID, &typeStr, &n.Name, &labelsJSON, &healthStr, &n.LastSeen, &metadataJSON); err != nil {
			continue
		}
		n.Type = NodeType(typeStr)
		n.Health = HealthStatus(healthStr)
		if err := json.Unmarshal([]byte(labelsJSON), &n.Labels); err != nil {
			slog.Warn("failed to unmarshal node labels", "error", err, "node_id", n.ID)
		}
		if n.Labels == nil {
			n.Labels = make(map[string]string)
		}
		if err := json.Unmarshal([]byte(metadataJSON), &n.Metadata); err != nil {
			slog.Warn("failed to unmarshal node metadata", "error", err, "node_id", n.ID)
		}
		if n.Metadata == nil {
			n.Metadata = make(map[string]any)
		}
		graph.AddNode(&n)
	}

	edgeRows, err := db.Query(
		`SELECT source, target, type, weight, labels, last_seen FROM topology_edges`,
	)
	if err != nil {
		return nil, fmt.Errorf("topology load: query edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var e Edge
		var typeStr, labelsJSON string
		if err := edgeRows.Scan(&e.Source, &e.Target, &typeStr, &e.Weight, &labelsJSON, &e.LastSeen); err != nil {
			continue
		}
		e.Type = EdgeType(typeStr)
		if err := json.Unmarshal([]byte(labelsJSON), &e.Labels); err != nil {
			slog.Warn("failed to unmarshal edge labels", "error", err, "source", e.Source, "target", e.Target)
		}
		if e.Labels == nil {
			e.Labels = make(map[string]string)
		}
		graph.AddEdge(&e)
	}

	slog.Debug("topology snapshot loaded", "nodes", len(graph.nodes), "edges", len(graph.edges))
	return graph, nil
}

// StartAutoSnapshot starts a goroutine that periodically snapshots the graph to SQLite.
// Returns a stop function that should be called to stop the auto-snapshot.
func StartAutoSnapshot(graph *TopologyGraph, db *sql.DB, interval time.Duration) func() {
	if graph == nil || db == nil || interval <= 0 {
		return func() {}
	}

	var mu sync.Mutex
	stopped := false

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			mu.Lock()
			if stopped {
				mu.Unlock()
				return
			}
			mu.Unlock()

			if err := SnapshotGraph(graph, db); err != nil {
				slog.Warn("topology auto-snapshot failed", "error", err)
			}
		}
	}()

	return func() {
		mu.Lock()
		stopped = true
		mu.Unlock()
	}
}

// InitTopology initializes the default topology graph, loading from snapshot if available.
func InitTopology(s *store.Store) *TopologyGraph {
	g := NewTopologyGraph()

	if s != nil {
		InitSchema(s.DB())
		if snap, err := LoadSnapshot(s.DB()); err == nil && snap != nil && len(snap.nodes) > 0 {
			g = snap
		}
	}

	SetDefaultTopology(g)

	if s != nil && config.AutoSnapshot {
		snapshotStop = StartAutoSnapshot(g, s.DB(), config.SnapshotInterval)
	}

	return g
}

var (
	defaultGraphMu sync.RWMutex
	defaultGraph   *TopologyGraph
	snapshotStop   func()
)

// DefaultTopology returns the package-level singleton topology graph.
func DefaultTopology() *TopologyGraph {
	defaultGraphMu.RLock()
	defer defaultGraphMu.RUnlock()
	return defaultGraph
}

// SetDefaultTopology sets the package-level singleton topology graph.
func SetDefaultTopology(g *TopologyGraph) {
	defaultGraphMu.Lock()
	defaultGraph = g
	defaultGraphMu.Unlock()
}

// StopAutoSnapshot stops the auto-snapshot goroutine if running.
func StopAutoSnapshot() {
	if snapshotStop != nil {
		snapshotStop()
		snapshotStop = nil
	}
}

package topology

import (
	"sync"
	"time"
)

// NodeType categorizes infrastructure nodes.
type NodeType string

const (
	NodeTypeService       NodeType = "service"
	NodeTypeDatabase      NodeType = "database"
	NodeTypeQueue         NodeType = "queue"
	NodeTypeCache         NodeType = "cache"
	NodeTypeLoadBalancer  NodeType = "load_balancer"
	NodeTypeK8sCluster    NodeType = "k8s_cluster"
	NodeTypeK8sNamespace  NodeType = "k8s_namespace"
	NodeTypeK8sDeployment NodeType = "k8s_deployment"
	NodeTypeK8sPod        NodeType = "k8s_pod"
	NodeTypeHost          NodeType = "host"
)

// HealthStatus represents the health of a node.
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"
	HealthDegraded HealthStatus = "degraded"
	HealthDown     HealthStatus = "down"
	HealthUnknown  HealthStatus = "unknown"
)

// EdgeType categorizes relationships between nodes.
type EdgeType string

const (
	EdgeDependsOn  EdgeType = "depends_on"
	EdgeCalls      EdgeType = "calls"
	EdgeReadsFrom  EdgeType = "reads_from"
	EdgeWritesTo   EdgeType = "writes_to"
	EdgeDeployedOn EdgeType = "deployed_on"
	EdgeRoutesTo   EdgeType = "routes_to"
)

// Node represents an infrastructure entity in the topology graph.
type Node struct {
	ID       string            `json:"id"`
	Type     NodeType          `json:"type"`
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels,omitempty"`
	Health   HealthStatus      `json:"health"`
	LastSeen time.Time         `json:"last_seen"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	Source   string            `json:"source"`
	Target   string            `json:"target"`
	Type     EdgeType          `json:"type"`
	Weight   float64           `json:"weight"`
	Labels   map[string]string `json:"labels,omitempty"`
	LastSeen time.Time         `json:"last_seen"`
}

// GraphStats holds summary statistics about the topology graph.
type GraphStats struct {
	NodeCount    int                  `json:"node_count"`
	EdgeCount    int                  `json:"edge_count"`
	NodesByType  map[NodeType]int     `json:"nodes_by_type"`
	EdgesByType  map[EdgeType]int     `json:"edges_by_type"`
	HealthCounts map[HealthStatus]int `json:"health_counts"`
}

// TopologyGraph is a thread-safe directed graph of infrastructure nodes and edges.
type TopologyGraph struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	edges   map[string][]*Edge // source_node_id -> edges
	reverse map[string][]*Edge // target_node_id -> edges (reverse index)
}

// NewTopologyGraph creates an empty topology graph.
func NewTopologyGraph() *TopologyGraph {
	return &TopologyGraph{
		nodes:   make(map[string]*Node),
		edges:   make(map[string][]*Edge),
		reverse: make(map[string][]*Edge),
	}
}

// AddNode adds a node to the graph or updates an existing one (merges labels).
func (g *TopologyGraph) AddNode(node *Node) {
	if node == nil {
		return
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]any)
	}
	if node.Health == "" {
		node.Health = HealthUnknown
	}
	if node.LastSeen.IsZero() {
		node.LastSeen = time.Now()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.nodes[node.ID]; ok {
		for k, v := range node.Labels {
			existing.Labels[k] = v
		}
		for k, v := range node.Metadata {
			existing.Metadata[k] = v
		}
		if node.Name != "" {
			existing.Name = node.Name
		}
		if node.Type != "" {
			existing.Type = node.Type
		}
		if node.Health != HealthUnknown {
			existing.Health = node.Health
		}
		if !node.LastSeen.IsZero() {
			existing.LastSeen = node.LastSeen
		}
	} else {
		n := &Node{
			ID:       node.ID,
			Type:     node.Type,
			Name:     node.Name,
			Health:   node.Health,
			LastSeen: node.LastSeen,
			Labels:   make(map[string]string, len(node.Labels)),
			Metadata: make(map[string]any, len(node.Metadata)),
		}
		for k, v := range node.Labels {
			n.Labels[k] = v
		}
		for k, v := range node.Metadata {
			n.Metadata[k] = v
		}
		g.nodes[node.ID] = n
		metricNodesTotal.Inc()
	}
}

// RemoveNode removes a node and all its edges from the graph.
func (g *TopologyGraph) RemoveNode(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.nodes, id)
	metricNodesTotal.Dec()

	outEdges := g.edges[id]
	for _, e := range outEdges {
		g.removeReverseEdge(e.Target, id, e.Type)
	}
	delete(g.edges, id)

	inEdges := g.reverse[id]
	for _, e := range inEdges {
		g.removeForwardEdge(e.Source, id, e.Type)
	}
	delete(g.reverse, id)
}

// AddEdge adds an edge to the graph or updates an existing one.
func (g *TopologyGraph) AddEdge(edge *Edge) {
	if edge == nil {
		return
	}
	if edge.Labels == nil {
		edge.Labels = make(map[string]string)
	}
	if edge.LastSeen.IsZero() {
		edge.LastSeen = time.Now()
	}
	if edge.Weight == 0 {
		edge.Weight = 0.5
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.removeForwardEdge(edge.Source, edge.Target, edge.Type)
	g.removeReverseEdge(edge.Target, edge.Source, edge.Type)

	e := &Edge{
		Source:   edge.Source,
		Target:   edge.Target,
		Type:     edge.Type,
		Weight:   edge.Weight,
		LastSeen: edge.LastSeen,
		Labels:   make(map[string]string, len(edge.Labels)),
	}
	for k, v := range edge.Labels {
		e.Labels[k] = v
	}

	g.edges[edge.Source] = append(g.edges[edge.Source], e)
	g.reverse[edge.Target] = append(g.reverse[edge.Target], e)
	metricEdgesTotal.Inc()
}

// RemoveEdge removes a specific edge from the graph.
func (g *TopologyGraph) RemoveEdge(source, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	newForward := make([]*Edge, 0, len(g.edges[source]))
	for _, e := range g.edges[source] {
		if e.Target != target {
			newForward = append(newForward, e)
		}
	}
	g.edges[source] = newForward

	newReverse := make([]*Edge, 0, len(g.reverse[target]))
	for _, e := range g.reverse[target] {
		if e.Source != source {
			newReverse = append(newReverse, e)
		}
	}
	g.reverse[target] = newReverse
}

// GetNode returns a node by ID, or nil if not found.
func (g *TopologyGraph) GetNode(id string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// GetNeighbors returns all nodes and edges reachable from nodeID up to the given BFS depth.
func (g *TopologyGraph) GetNeighbors(nodeID string, depth int) ([]*Node, []*Edge) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if depth <= 0 {
		return nil, nil
	}

	visited := make(map[string]bool)
	var nodes []*Node
	var edges []*Edge

	queue := []struct {
		id    string
		depth int
	}{{id: nodeID, depth: 0}}
	visited[nodeID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if n, ok := g.nodes[curr.id]; ok {
			nodes = append(nodes, n)
		}

		if curr.depth >= depth {
			continue
		}

		for _, e := range g.edges[curr.id] {
			edges = append(edges, e)
			if !visited[e.Target] {
				visited[e.Target] = true
				queue = append(queue, struct {
					id    string
					depth int
				}{id: e.Target, depth: curr.depth + 1})
			}
		}
	}

	return nodes, edges
}

// GetPath finds the shortest path between source and target using BFS.
// Returns the ordered list of nodes and edges along the path, or nil if no path exists.
func (g *TopologyGraph) GetPath(source, target string) ([]*Node, []*Edge) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if source == target {
		if n, ok := g.nodes[source]; ok {
			return []*Node{n}, nil
		}
		return nil, nil
	}

	type bfsEntry struct {
		nodeID string
		path   []struct {
			nodeID string
			edge   *Edge
		}
	}

	visited := map[string]bool{source: true}
	queue := []bfsEntry{{nodeID: source}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, e := range g.edges[curr.nodeID] {
			if visited[e.Target] {
				continue
			}
			visited[e.Target] = true

			newPath := append(curr.path, struct {
				nodeID string
				edge   *Edge
			}{nodeID: e.Target, edge: e})

			if e.Target == target {
				var nodes []*Node
				var edges []*Edge

				if n, ok := g.nodes[source]; ok {
					nodes = append(nodes, n)
				}
				for _, hop := range newPath {
					if n, ok := g.nodes[hop.nodeID]; ok {
						nodes = append(nodes, n)
					}
					edges = append(edges, hop.edge)
				}
				return nodes, edges
			}

			queue = append(queue, bfsEntry{nodeID: e.Target, path: newPath})
		}
	}

	return nil, nil
}

// AllNodes returns all nodes in the graph.
func (g *TopologyGraph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// AllEdges returns all edges in the graph.
func (g *TopologyGraph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []*Edge
	for _, edgeList := range g.edges {
		edges = append(edges, edgeList...)
	}
	return edges
}

// Stats returns summary statistics about the graph.
func (g *TopologyGraph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := GraphStats{
		NodeCount:    len(g.nodes),
		NodesByType:  make(map[NodeType]int),
		EdgesByType:  make(map[EdgeType]int),
		HealthCounts: make(map[HealthStatus]int),
	}

	for _, n := range g.nodes {
		stats.NodesByType[n.Type]++
		stats.HealthCounts[n.Health]++
	}

	edgeCount := 0
	for _, edgeList := range g.edges {
		edgeCount += len(edgeList)
		for _, e := range edgeList {
			stats.EdgesByType[e.Type]++
		}
	}
	stats.EdgeCount = edgeCount

	return stats
}

// Merge merges another topology graph into this one.
// Nodes are merged (labels merged), edges are added or updated.
func (g *TopologyGraph) Merge(other *TopologyGraph) {
	if other == nil {
		return
	}

	other.mu.RLock()
	otherNodes := make([]*Node, 0, len(other.nodes))
	for _, n := range other.nodes {
		otherNodes = append(otherNodes, n)
	}
	otherEdges := make([]*Edge, 0)
	for _, edgeList := range other.edges {
		otherEdges = append(otherEdges, edgeList...)
	}
	other.mu.RUnlock()

	for _, n := range otherNodes {
		g.AddNode(n)
	}
	for _, e := range otherEdges {
		g.AddEdge(e)
	}
}

// Prune removes nodes and edges not seen within maxAge.
func (g *TopologyGraph) Prune(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)

	g.mu.Lock()
	defer g.mu.Unlock()

	for source, edgeList := range g.edges {
		var fresh []*Edge
		for _, e := range edgeList {
			if e.LastSeen.After(cutoff) {
				fresh = append(fresh, e)
			} else {
				g.removeReverseEdge(e.Target, e.Source, e.Type)
			}
		}
		if len(fresh) == 0 {
			delete(g.edges, source)
		} else {
			g.edges[source] = fresh
		}
	}

	for id, n := range g.nodes {
		if n.LastSeen.Before(cutoff) {
			delete(g.nodes, id)
			delete(g.edges, id)
			delete(g.reverse, id)
		}
	}
}

// GetEdgesFrom returns all outgoing edges from a node.
func (g *TopologyGraph) GetEdgesFrom(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := make([]*Edge, len(g.edges[nodeID]))
	copy(edges, g.edges[nodeID])
	return edges
}

// GetEdgesTo returns all incoming edges to a node.
func (g *TopologyGraph) GetEdgesTo(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := make([]*Edge, len(g.reverse[nodeID]))
	copy(edges, g.reverse[nodeID])
	return edges
}

// GetNodesByType returns all nodes of a given type.
func (g *TopologyGraph) GetNodesByType(nodeType NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, n := range g.nodes {
		if n.Type == nodeType {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// removeForwardEdge removes an edge from the forward index. Must be called with write lock held.
func (g *TopologyGraph) removeForwardEdge(source, target string, edgeType EdgeType) {
	edges := g.edges[source]
	for i, e := range edges {
		if e.Target == target && e.Type == edgeType {
			g.edges[source] = append(edges[:i], edges[i+1:]...)
			return
		}
	}
}

// removeReverseEdge removes an edge from the reverse index. Must be called with write lock held.
func (g *TopologyGraph) removeReverseEdge(target, source string, edgeType EdgeType) {
	edges := g.reverse[target]
	for i, e := range edges {
		if e.Source == source && e.Type == edgeType {
			g.reverse[target] = append(edges[:i], edges[i+1:]...)
			return
		}
	}
}

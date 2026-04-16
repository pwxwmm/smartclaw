package topology

import (
	"context"
	"fmt"

	"github.com/instructkr/smartclaw/internal/tools"
)

type topoGetStringFunc func(map[string]any, string) string
type topoGetIntFunc func(map[string]any, string) int
type topoGetFloatFunc func(map[string]any, string) float64

func topoGetString(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

func topoGetInt(input map[string]any, key string) int {
	if f, ok := input[key].(float64); ok {
		return int(f)
	}
	if i, ok := input[key].(int); ok {
		return i
	}
	return 0
}

func topoGetFloat(input map[string]any, key string) float64 {
	if f, ok := input[key].(float64); ok {
		return f
	}
	return 0
}

func topoGetStringMap(input map[string]any, key string) map[string]string {
	result := make(map[string]string)
	v, ok := input[key]
	if !ok {
		return result
	}
	switch m := v.(type) {
	case map[string]string:
		return m
	case map[string]any:
		for k, val := range m {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}

// TopologyQueryTool queries the topology graph.
type TopologyQueryTool struct{}

func (t *TopologyQueryTool) Name() string { return "topology_query" }

func (t *TopologyQueryTool) Description() string {
	return "Query the infrastructure topology graph. Returns node details with neighbors, nodes by type, or graph stats."
}

func (t *TopologyQueryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"node_id":   map[string]any{"type": "string", "description": "Specific node ID to query"},
			"node_type": map[string]any{"type": "string", "description": "Filter nodes by type (service, database, queue, cache, load_balancer, k8s_cluster, k8s_namespace, k8s_deployment, k8s_pod, host)"},
			"depth":     map[string]any{"type": "integer", "description": "BFS traversal depth for neighbors (default 1)", "default": 1},
		},
	}
}

func (t *TopologyQueryTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	graph := DefaultTopology()
	if graph == nil {
		return nil, fmt.Errorf("topology: graph not initialized")
	}

	nodeID := topoGetString(input, "node_id")
	nodeType := topoGetString(input, "node_type")
	depth := topoGetInt(input, "depth")
	if depth <= 0 {
		depth = 1
	}

	if nodeID != "" {
		node := graph.GetNode(nodeID)
		if node == nil {
			return map[string]any{"error": "node not found", "node_id": nodeID}, nil
		}
		nodes, edges := graph.GetNeighbors(nodeID, depth)
		return map[string]any{
			"node":      node,
			"neighbors": nodes,
			"edges":     edges,
		}, nil
	}

	if nodeType != "" {
		nodes := graph.GetNodesByType(NodeType(nodeType))
		return map[string]any{
			"node_type": nodeType,
			"nodes":     nodes,
			"count":     len(nodes),
		}, nil
	}

	stats := graph.Stats()
	return stats, nil
}

// TopologyBlastRadiusTool calculates blast radius for a node.
type TopologyBlastRadiusTool struct{}

func (t *TopologyBlastRadiusTool) Name() string { return "topology_blast_radius" }

func (t *TopologyBlastRadiusTool) Description() string {
	return "Calculate the blast radius of a node failure. Shows downstream impact, upstream dependencies, or both."
}

func (t *TopologyBlastRadiusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"node_id":   map[string]any{"type": "string", "description": "Node ID to analyze blast radius for"},
			"depth":     map[string]any{"type": "integer", "description": "Traversal depth (default 3)", "default": 3},
			"direction": map[string]any{"type": "string", "description": "Direction: downstream, upstream, or both (default downstream)", "default": "downstream"},
		},
		"required": []string{"node_id"},
	}
}

func (t *TopologyBlastRadiusTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	graph := DefaultTopology()
	if graph == nil {
		return nil, fmt.Errorf("topology: graph not initialized")
	}

	nodeID := topoGetString(input, "node_id")
	if nodeID == "" {
		return nil, fmt.Errorf("topology: node_id is required")
	}

	depth := topoGetInt(input, "depth")
	if depth <= 0 {
		depth = 3
	}

	direction := topoGetString(input, "direction")
	if direction == "" {
		direction = "downstream"
	}

	var result *BlastResult
	switch direction {
	case "upstream":
		result = BlastRadiusUpstream(graph, nodeID, depth)
	case "both":
		result = BlastRadius(graph, nodeID, depth)
	default:
		result = BlastRadiusDownstream(graph, nodeID, depth)
	}

	if result.RootCause == nil {
		return map[string]any{"error": "node not found", "node_id": nodeID}, nil
	}

	return result, nil
}

// TopologyAddNodeTool adds a node to the topology graph.
type TopologyAddNodeTool struct{}

func (t *TopologyAddNodeTool) Name() string { return "topology_add_node" }

func (t *TopologyAddNodeTool) Description() string {
	return "Add a node to the infrastructure topology graph."
}

func (t *TopologyAddNodeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":     map[string]any{"type": "string", "description": "Node ID (e.g. 'svc:payment-api')"},
			"type":   map[string]any{"type": "string", "description": "Node type (service, database, queue, cache, load_balancer, k8s_cluster, k8s_namespace, k8s_deployment, k8s_pod, host)"},
			"name":   map[string]any{"type": "string", "description": "Human-readable name"},
			"labels": map[string]any{"type": "object", "description": "Key-value labels (env, team, region, etc.)"},
		},
		"required": []string{"id", "type", "name"},
	}
}

func (t *TopologyAddNodeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	graph := DefaultTopology()
	if graph == nil {
		return nil, fmt.Errorf("topology: graph not initialized")
	}

	id := topoGetString(input, "id")
	if id == "" {
		return nil, fmt.Errorf("topology: id is required")
	}
	nodeType := topoGetString(input, "type")
	if nodeType == "" {
		return nil, fmt.Errorf("topology: type is required")
	}
	name := topoGetString(input, "name")
	if name == "" {
		return nil, fmt.Errorf("topology: name is required")
	}

	labels := topoGetStringMap(input, "labels")

	graph.AddNode(&Node{
		ID:     id,
		Type:   NodeType(nodeType),
		Name:   name,
		Labels: labels,
		Health: HealthUnknown,
	})

	return map[string]any{
		"status":  "added",
		"node_id": id,
		"type":    nodeType,
		"name":    name,
	}, nil
}

// TopologyAddEdgeTool adds an edge to the topology graph.
type TopologyAddEdgeTool struct{}

func (t *TopologyAddEdgeTool) Name() string { return "topology_add_edge" }

func (t *TopologyAddEdgeTool) Description() string {
	return "Add an edge (dependency relationship) to the infrastructure topology graph."
}

func (t *TopologyAddEdgeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source": map[string]any{"type": "string", "description": "Source node ID"},
			"target": map[string]any{"type": "string", "description": "Target node ID"},
			"type":   map[string]any{"type": "string", "description": "Edge type (depends_on, calls, reads_from, writes_to, deployed_on, routes_to)"},
			"weight": map[string]any{"type": "number", "description": "Edge weight 0.0-1.0 (default 0.5)"},
			"labels": map[string]any{"type": "object", "description": "Key-value labels (protocol, latency_p99, etc.)"},
		},
		"required": []string{"source", "target", "type"},
	}
}

func (t *TopologyAddEdgeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	graph := DefaultTopology()
	if graph == nil {
		return nil, fmt.Errorf("topology: graph not initialized")
	}

	source := topoGetString(input, "source")
	if source == "" {
		return nil, fmt.Errorf("topology: source is required")
	}
	target := topoGetString(input, "target")
	if target == "" {
		return nil, fmt.Errorf("topology: target is required")
	}
	edgeType := topoGetString(input, "type")
	if edgeType == "" {
		return nil, fmt.Errorf("topology: type is required")
	}

	weight := topoGetFloat(input, "weight")
	if weight <= 0 {
		weight = 0.5
	}

	labels := topoGetStringMap(input, "labels")

	graph.AddEdge(&Edge{
		Source: source,
		Target: target,
		Type:   EdgeType(edgeType),
		Weight: weight,
		Labels: labels,
	})

	return map[string]any{
		"status": "added",
		"source": source,
		"target": target,
		"type":   edgeType,
		"weight": weight,
	}, nil
}

// RegisterTopologyTools registers all topology tools with the given register function.
func RegisterTopologyTools(register func(name string, tool any)) {
	if register == nil {
		return
	}
	register("topology_query", &TopologyQueryTool{})
	register("topology_blast_radius", &TopologyBlastRadiusTool{})
	register("topology_add_node", &TopologyAddNodeTool{})
	register("topology_add_edge", &TopologyAddEdgeTool{})
}

func RegisterAllTools() {
	tools.Register(&TopologyQueryTool{})
	tools.Register(&TopologyBlastRadiusTool{})
	tools.Register(&TopologyAddNodeTool{})
	tools.Register(&TopologyAddEdgeTool{})
}

package topology

// ImpactHop represents a single hop in an impact path from root cause to affected node.
type ImpactHop struct {
	Node  *Node `json:"node"`
	Edge  *Edge `json:"edge,omitempty"`
	Depth int   `json:"depth"`
}

// BlastResult holds the output of a blast radius analysis.
type BlastResult struct {
	RootCause  *Node       `json:"root_cause"`
	Affected   []*Node     `json:"affected"`
	ImpactPath []ImpactHop `json:"impact_path"`
	Score      float64     `json:"score"`
}

// BlastRadiusDownstream calculates which nodes would be impacted if the given node goes down.
// It traverses the graph following outgoing edges (downstream dependencies).
func BlastRadiusDownstream(graph *TopologyGraph, nodeID string, depth int) *BlastResult {
	if graph == nil {
		return nil
	}

	root := graph.GetNode(nodeID)
	if root == nil {
		return &BlastResult{RootCause: nil, Score: 0}
	}

	visited := map[string]bool{nodeID: true}
	var affected []*Node
	var impactPath []ImpactHop

	type bfsEntry struct {
		id    string
		edge  *Edge
		depth int
	}

	queue := []bfsEntry{{id: nodeID, depth: 0}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth > 0 {
			n := graph.GetNode(curr.id)
			if n != nil {
				affected = append(affected, n)
				hop := ImpactHop{Node: n, Depth: curr.depth}
				if curr.edge != nil {
					hop.Edge = curr.edge
				}
				impactPath = append(impactPath, hop)
			}
		}

		if curr.depth >= depth {
			continue
		}

		for _, e := range graph.GetEdgesFrom(curr.id) {
			if !visited[e.Target] {
				visited[e.Target] = true
				queue = append(queue, bfsEntry{id: e.Target, edge: e, depth: curr.depth + 1})
			}
		}
	}

	return &BlastResult{
		RootCause:  root,
		Affected:   affected,
		ImpactPath: impactPath,
		Score:      blastScore(len(affected)),
	}
}

// BlastRadiusUpstream calculates which nodes could cause the given node to go down.
// It traverses the graph following incoming edges (upstream dependencies).
func BlastRadiusUpstream(graph *TopologyGraph, nodeID string, depth int) *BlastResult {
	if graph == nil {
		return nil
	}

	root := graph.GetNode(nodeID)
	if root == nil {
		return &BlastResult{RootCause: nil, Score: 0}
	}

	visited := map[string]bool{nodeID: true}
	var affected []*Node
	var impactPath []ImpactHop

	type bfsEntry struct {
		id    string
		edge  *Edge
		depth int
	}

	queue := []bfsEntry{{id: nodeID, depth: 0}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth > 0 {
			n := graph.GetNode(curr.id)
			if n != nil {
				affected = append(affected, n)
				hop := ImpactHop{Node: n, Depth: curr.depth}
				if curr.edge != nil {
					hop.Edge = curr.edge
				}
				impactPath = append(impactPath, hop)
			}
		}

		if curr.depth >= depth {
			continue
		}

		for _, e := range graph.GetEdgesTo(curr.id) {
			if !visited[e.Source] {
				visited[e.Source] = true
				queue = append(queue, bfsEntry{id: e.Source, edge: e, depth: curr.depth + 1})
			}
		}
	}

	return &BlastResult{
		RootCause:  root,
		Affected:   affected,
		ImpactPath: impactPath,
		Score:      blastScore(len(affected)),
	}
}

// BlastRadius calculates blast radius in both directions (upstream + downstream).
func BlastRadius(graph *TopologyGraph, nodeID string, depth int) *BlastResult {
	if graph == nil {
		return nil
	}

	downstream := BlastRadiusDownstream(graph, nodeID, depth)
	upstream := BlastRadiusUpstream(graph, nodeID, depth)

	affectedSet := map[string]*Node{}
	for _, n := range downstream.Affected {
		affectedSet[n.ID] = n
	}
	for _, n := range upstream.Affected {
		affectedSet[n.ID] = n
	}

	var allAffected []*Node
	for _, n := range affectedSet {
		allAffected = append(allAffected, n)
	}

	var allPaths []ImpactHop
	allPaths = append(allPaths, downstream.ImpactPath...)
	allPaths = append(allPaths, upstream.ImpactPath...)

	return &BlastResult{
		RootCause:  downstream.RootCause,
		Affected:   allAffected,
		ImpactPath: allPaths,
		Score:      blastScore(len(allAffected)),
	}
}

// blastScore computes a severity score 0.0-1.0 based on fan-out.
// More affected nodes = higher score.
func blastScore(affectedCount int) float64 {
	return 1.0 - (1.0 / (1.0 + float64(affectedCount)))
}

package repomap

// PageRank computes personalized PageRank over the given adjacency graph.
//
// adjacency maps each node to its outgoing neighbors (directed edges: node → neighbors).
// personalization assigns bias weights to specific nodes (sum need not be 1; will be normalized).
// damping is the restart probability (typically 0.85).
// iterations is the number of power-iteration steps (typically 100).
//
// Returns a map of node → rank score.
func PageRank(
	adjacency map[string][]string,
	personalization map[string]float64,
	damping float64,
	iterations int,
) map[string]float64 {
	if len(adjacency) == 0 {
		return map[string]float64{}
	}

	// Collect all nodes (some may only appear as targets)
	nodes := make(map[string]struct{})
	for src, dsts := range adjacency {
		nodes[src] = struct{}{}
		for _, dst := range dsts {
			nodes[dst] = struct{}{}
		}
	}
	n := len(nodes)
	if n == 0 {
		return map[string]float64{}
	}

	// Normalize personalization vector; default to uniform if empty or sums to zero
	teleport := make(map[string]float64, n)
	pSum := 0.0
	for _, v := range personalization {
		pSum += v
	}
	for node := range nodes {
		if pSum > 0 {
			teleport[node] = personalization[node] / pSum
		} else {
			teleport[node] = 1.0 / float64(n)
		}
	}

	// Compute out-degree for each node
	outDegree := make(map[string]int, n)
	for src, dsts := range adjacency {
		outDegree[src] = len(dsts)
	}

	// Initialize scores uniformly
	score := make(map[string]float64, n)
	for node := range nodes {
		score[node] = 1.0 / float64(n)
	}

	// Power iteration
	for i := 0; i < iterations; i++ {
		newScore := make(map[string]float64, n)

		// Teleport term: (1-d)/N * personalization
		for node := range nodes {
			newScore[node] = (1.0 - damping) * teleport[node]
		}

		// Propagation term: d * sum(score[j]/outDegree[j]) for all j→i
		for src, dsts := range adjacency {
			if outDegree[src] == 0 {
				continue
			}
			share := score[src] / float64(outDegree[src])
			for _, dst := range dsts {
				newScore[dst] += damping * share
			}
		}

		score = newScore
	}

	return score
}

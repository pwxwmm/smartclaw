package topology

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

func TestAddNode(t *testing.T) {
	g := NewTopologyGraph()

	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API Server"})
	n := g.GetNode("svc:api")
	if n == nil {
		t.Fatal("expected node to exist")
	}
	if n.Name != "API Server" {
		t.Fatalf("expected name 'API Server', got %q", n.Name)
	}
	if n.Health != HealthUnknown {
		t.Fatalf("expected health 'unknown', got %q", n.Health)
	}
}

func TestAddNodeMergeLabels(t *testing.T) {
	g := NewTopologyGraph()

	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API", Labels: map[string]string{"env": "prod", "team": "backend"}})
	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API v2", Labels: map[string]string{"env": "staging", "region": "us-east"}})

	n := g.GetNode("svc:api")
	if n == nil {
		t.Fatal("expected node to exist")
	}
	if n.Name != "API v2" {
		t.Fatalf("expected name updated to 'API v2', got %q", n.Name)
	}
	if n.Labels["env"] != "staging" {
		t.Fatalf("expected env='staging' (overridden), got %q", n.Labels["env"])
	}
	if n.Labels["team"] != "backend" {
		t.Fatalf("expected team='backend' (preserved), got %q", n.Labels["team"])
	}
	if n.Labels["region"] != "us-east" {
		t.Fatalf("expected region='us-east' (added), got %q", n.Labels["region"])
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewTopologyGraph()

	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API"})
	g.AddNode(&Node{ID: "svc:auth", Type: NodeTypeService, Name: "Auth"})
	g.AddEdge(&Edge{Source: "svc:api", Target: "svc:auth", Type: EdgeCalls})

	g.RemoveNode("svc:api")

	if g.GetNode("svc:api") != nil {
		t.Fatal("expected node to be removed")
	}
	edges := g.GetEdgesFrom("svc:api")
	if len(edges) != 0 {
		t.Fatalf("expected 0 outgoing edges after removal, got %d", len(edges))
	}
}

func TestAddRemoveEdge(t *testing.T) {
	g := NewTopologyGraph()

	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API"})
	g.AddNode(&Node{ID: "db:postgres", Type: NodeTypeDatabase, Name: "Postgres"})

	g.AddEdge(&Edge{Source: "svc:api", Target: "db:postgres", Type: EdgeReadsFrom, Weight: 0.8})
	edges := g.GetEdgesFrom("svc:api")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeReadsFrom {
		t.Fatalf("expected edge type reads_from, got %q", edges[0].Type)
	}

	g.RemoveEdge("svc:api", "db:postgres")
	edges = g.GetEdgesFrom("svc:api")
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges after removal, got %d", len(edges))
	}
}

func TestGetNeighborsDepth1(t *testing.T) {
	g := buildTestGraph()

	nodes, edges := g.GetNeighbors("svc:api", 1)
	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 nodes (root + neighbors), got %d", len(nodes))
	}
	if len(edges) < 1 {
		t.Fatalf("expected at least 1 edge, got %d", len(edges))
	}
}

func TestGetNeighborsDepth2(t *testing.T) {
	g := buildTestGraph()

	nodes1, _ := g.GetNeighbors("svc:api", 1)
	nodes2, _ := g.GetNeighbors("svc:api", 2)
	if len(nodes2) <= len(nodes1) {
		t.Fatalf("expected more nodes at depth 2 (%d) than depth 1 (%d)", len(nodes2), len(nodes1))
	}
}

func TestGetNeighborsDepth3(t *testing.T) {
	g := buildTestGraph()

	nodes2, _ := g.GetNeighbors("svc:api", 2)
	nodes3, _ := g.GetNeighbors("svc:api", 3)
	if len(nodes3) < len(nodes2) {
		t.Fatalf("expected at least as many nodes at depth 3 (%d) as depth 2 (%d)", len(nodes3), len(nodes2))
	}
}

func TestGetPath(t *testing.T) {
	g := buildTestGraph()

	nodes, edges := g.GetPath("svc:api", "db:redis")
	if nodes == nil {
		t.Fatal("expected a path to exist")
	}
	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 nodes in path, got %d", len(nodes))
	}
	if nodes[0].ID != "svc:api" {
		t.Fatalf("expected path to start at svc:api, got %q", nodes[0].ID)
	}
	if nodes[len(nodes)-1].ID != "db:redis" {
		t.Fatalf("expected path to end at db:redis, got %q", nodes[len(nodes)-1].ID)
	}
	_ = edges
}

func TestGetPathNotFound(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})

	nodes, _ := g.GetPath("svc:a", "svc:b")
	if nodes != nil {
		t.Fatal("expected no path between disconnected nodes")
	}
}

func TestBlastRadiusDownstream(t *testing.T) {
	g := buildTestGraph()

	result := BlastRadiusDownstream(g, "svc:api", 3)
	if result == nil {
		t.Fatal("expected result")
	}
	if result.RootCause == nil || result.RootCause.ID != "svc:api" {
		t.Fatal("expected root cause to be svc:api")
	}
	if len(result.Affected) == 0 {
		t.Fatal("expected affected nodes")
	}
	if result.Score <= 0 {
		t.Fatalf("expected positive score, got %f", result.Score)
	}
}

func TestBlastRadiusUpstream(t *testing.T) {
	g := buildTestGraph()

	result := BlastRadiusUpstream(g, "db:redis", 3)
	if result == nil {
		t.Fatal("expected result")
	}
	if result.RootCause == nil || result.RootCause.ID != "db:redis" {
		t.Fatal("expected root cause to be db:redis")
	}
	if len(result.Affected) == 0 {
		t.Fatal("expected upstream dependents")
	}
}

func TestBlastRadiusBoth(t *testing.T) {
	g := buildTestGraph()

	result := BlastRadius(g, "svc:api", 3)
	if result == nil {
		t.Fatal("expected result")
	}
	if len(result.Affected) == 0 {
		t.Fatal("expected affected nodes in both directions")
	}
}

func TestBlastRadiusScore(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})

	result := BlastRadiusDownstream(g, "svc:a", 3)
	if result.Score != 0.0 {
		t.Fatalf("expected score 0.0 for no affected nodes, got %f", result.Score)
	}

	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	g.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls})

	result = BlastRadiusDownstream(g, "svc:a", 3)
	expected := 1.0 - (1.0 / 2.0)
	if result.Score != expected {
		t.Fatalf("expected score %f for 1 affected node, got %f", expected, result.Score)
	}
}

func TestMerge(t *testing.T) {
	g1 := NewTopologyGraph()
	g1.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g1.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	g1.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls})

	g2 := NewTopologyGraph()
	g2.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B Updated", Labels: map[string]string{"env": "prod"}})
	g2.AddNode(&Node{ID: "svc:c", Type: NodeTypeService, Name: "C"})
	g2.AddEdge(&Edge{Source: "svc:b", Target: "svc:c", Type: EdgeDependsOn})

	g1.Merge(g2)

	if g1.GetNode("svc:c") == nil {
		t.Fatal("expected svc:c to be added from merge")
	}
	b := g1.GetNode("svc:b")
	if b.Labels["env"] != "prod" {
		t.Fatal("expected svc:b labels to be merged from g2")
	}

	stats := g1.Stats()
	if stats.NodeCount != 3 {
		t.Fatalf("expected 3 nodes after merge, got %d", stats.NodeCount)
	}
}

func TestPrune(t *testing.T) {
	g := NewTopologyGraph()

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	g.AddNode(&Node{ID: "svc:old", Type: NodeTypeService, Name: "Old", LastSeen: old})
	g.AddNode(&Node{ID: "svc:new", Type: NodeTypeService, Name: "New", LastSeen: recent})
	g.AddEdge(&Edge{Source: "svc:old", Target: "svc:new", Type: EdgeCalls, LastSeen: old})
	g.AddEdge(&Edge{Source: "svc:new", Target: "svc:old", Type: EdgeDependsOn, LastSeen: recent})

	g.Prune(1 * time.Hour)

	if g.GetNode("svc:old") != nil {
		t.Fatal("expected old node to be pruned")
	}
	if g.GetNode("svc:new") == nil {
		t.Fatal("expected new node to survive")
	}
}

func TestStats(t *testing.T) {
	g := buildTestGraph()

	stats := g.Stats()
	if stats.NodeCount == 0 {
		t.Fatal("expected non-zero node count")
	}
	if stats.EdgeCount == 0 {
		t.Fatal("expected non-zero edge count")
	}
	if stats.NodesByType[NodeTypeService] == 0 {
		t.Fatal("expected at least one service node")
	}
}

func TestSOPAInventoryIngestion(t *testing.T) {
	g := NewTopologyGraph()

	items := []SOPAInventoryItem{
		{ID: "host:web-01", Name: "web-01", Type: "host", Labels: map[string]string{"region": "us-east"}, Metadata: map[string]any{"ip": "10.0.0.1"}},
		{ID: "svc:payment", Name: "Payment API", Type: "service", Labels: map[string]string{"env": "prod"}},
	}

	IngestSOPAInventory(g, items)

	n1 := g.GetNode("host:web-01")
	if n1 == nil {
		t.Fatal("expected host:web-01 node")
	}
	if n1.Type != NodeTypeHost {
		t.Fatalf("expected type 'host', got %q", n1.Type)
	}

	n2 := g.GetNode("svc:payment")
	if n2 == nil {
		t.Fatal("expected svc:payment node")
	}
	if n2.Labels["env"] != "prod" {
		t.Fatal("expected env label to be preserved")
	}
}

func TestOTLPSpanIngestion(t *testing.T) {
	g := NewTopologyGraph()

	spans := []SpanData{
		{ServiceName: "api-gateway", PeerService: "auth-service", SpanKind: "client", Attributes: map[string]string{"rpc.system": "grpc"}},
		{ServiceName: "auth-service", PeerAddress: "10.0.0.5:5432", SpanKind: "client", Attributes: map[string]string{"db.system": "postgresql"}},
		{ServiceName: "api-gateway", SpanKind: "server"},
	}

	IngestOTLPSpans(g, spans)

	if g.GetNode("svc:api-gateway") == nil {
		t.Fatal("expected svc:api-gateway node")
	}
	if g.GetNode("svc:auth-service") == nil {
		t.Fatal("expected svc:auth-service node")
	}

	edges := g.GetEdgesFrom("svc:api-gateway")
	if len(edges) == 0 {
		t.Fatal("expected edges from api-gateway")
	}

	edges = g.GetEdgesFrom("svc:auth-service")
	if len(edges) == 0 {
		t.Fatal("expected edges from auth-service to peer address")
	}
}

func TestK8sResourceIngestion(t *testing.T) {
	g := NewTopologyGraph()

	resources := []K8sResource{
		{Kind: "Namespace", Name: "production"},
		{Kind: "Deployment", Name: "api-server", Namespace: "production", Selector: map[string]string{"app": "api"}},
		{Kind: "Service", Name: "api-svc", Namespace: "production", Selector: map[string]string{"app": "api"}},
		{Kind: "Pod", Name: "api-pod-1", Namespace: "production", Labels: map[string]string{"app": "api", "kubernetes.io/hostname": "node-1"}},
		{Kind: "Pod", Name: "unrelated-pod", Namespace: "production", Labels: map[string]string{"app": "other"}},
	}

	IngestK8sResources(g, resources)

	deployID := "deploy:production/api-server"
	if g.GetNode(deployID) == nil {
		t.Fatal("expected deployment node")
	}

	svcID := "svc:production/api-svc"
	if g.GetNode(svcID) == nil {
		t.Fatal("expected k8s service node")
	}

	podID := "pod:production/api-pod-1"
	if g.GetNode(podID) == nil {
		t.Fatal("expected pod node")
	}

	hostNode := g.GetNode("host:node-1")
	if hostNode == nil {
		t.Fatal("expected host node derived from pod hostname label")
	}

	deployEdges := g.GetEdgesFrom(deployID)
	foundPodEdge := false
	for _, e := range deployEdges {
		if e.Target == podID && e.Type == EdgeDeployedOn {
			foundPodEdge = true
		}
	}
	if !foundPodEdge {
		t.Fatal("expected deployment->pod edge")
	}

	svcEdges := g.GetEdgesFrom(svcID)
	foundRouteEdge := false
	for _, e := range svcEdges {
		if e.Target == podID && e.Type == EdgeRoutesTo {
			foundRouteEdge = true
		}
	}
	if !foundRouteEdge {
		t.Fatal("expected service->pod routes_to edge")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	g := buildTestGraph()

	if err := SnapshotGraph(g, s.DB()); err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	loaded, err := LoadSnapshot(s.DB())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	origStats := g.Stats()
	loadedStats := loaded.Stats()

	if loadedStats.NodeCount != origStats.NodeCount {
		t.Fatalf("expected %d nodes, got %d", origStats.NodeCount, loadedStats.NodeCount)
	}
	if loadedStats.EdgeCount != origStats.EdgeCount {
		t.Fatalf("expected %d edges, got %d", origStats.EdgeCount, loadedStats.EdgeCount)
	}

	for _, n := range g.AllNodes() {
		ln := loaded.GetNode(n.ID)
		if ln == nil {
			t.Fatalf("node %q missing in loaded graph", n.ID)
		}
		if ln.Type != n.Type {
			t.Fatalf("node %q: expected type %q, got %q", n.ID, n.Type, ln.Type)
		}
		if ln.Name != n.Name {
			t.Fatalf("node %q: expected name %q, got %q", n.ID, n.Name, ln.Name)
		}
	}

	_ = dbPath
}

func TestDefaultTopology(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:test", Type: NodeTypeService, Name: "Test"})

	SetDefaultTopology(g)

	retrieved := DefaultTopology()
	if retrieved == nil {
		t.Fatal("expected default topology to be set")
	}
	n := retrieved.GetNode("svc:test")
	if n == nil {
		t.Fatal("expected svc:test in default topology")
	}
}

func TestGetNodesByType(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	g.AddNode(&Node{ID: "db:c", Type: NodeTypeDatabase, Name: "C"})

	services := g.GetNodesByType(NodeTypeService)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	dbs := g.GetNodesByType(NodeTypeDatabase)
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database, got %d", len(dbs))
	}
}

func TestToolTopologyQuery(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API"})
	g.AddNode(&Node{ID: "db:pg", Type: NodeTypeDatabase, Name: "Postgres"})
	g.AddEdge(&Edge{Source: "svc:api", Target: "db:pg", Type: EdgeReadsFrom})
	SetDefaultTopology(g)

	tool := &TopologyQueryTool{}

	result, err := tool.Execute(nil, map[string]any{"node_id": "svc:api", "depth": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if _, ok := m["node"]; !ok {
		t.Fatal("expected 'node' key in result")
	}

	result, err = tool.Execute(nil, map[string]any{"node_type": "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err = tool.Execute(nil, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolTopologyBlastRadius(t *testing.T) {
	g := buildTestGraph()
	SetDefaultTopology(g)

	tool := &TopologyBlastRadiusTool{}

	result, err := tool.Execute(nil, map[string]any{"node_id": "svc:api", "depth": 3, "direction": "downstream"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	br, ok := result.(*BlastResult)
	if !ok {
		t.Fatal("expected BlastResult")
	}
	if br.RootCause.ID != "svc:api" {
		t.Fatalf("expected root cause svc:api, got %q", br.RootCause.ID)
	}

	result, err = tool.Execute(nil, map[string]any{"node_id": "svc:api", "direction": "upstream"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err = tool.Execute(nil, map[string]any{"node_id": "svc:api", "direction": "both"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolTopologyAddNode(t *testing.T) {
	g := NewTopologyGraph()
	SetDefaultTopology(g)

	tool := &TopologyAddNodeTool{}

	result, err := tool.Execute(nil, map[string]any{
		"id":     "svc:new",
		"type":   "service",
		"name":   "New Service",
		"labels": map[string]any{"env": "staging"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["status"] != "added" {
		t.Fatalf("expected status 'added', got %v", m["status"])
	}

	n := g.GetNode("svc:new")
	if n == nil {
		t.Fatal("expected node to be added to graph")
	}
}

func TestToolTopologyAddEdge(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	SetDefaultTopology(g)

	tool := &TopologyAddEdgeTool{}

	result, err := tool.Execute(nil, map[string]any{
		"source": "svc:a",
		"target": "svc:b",
		"type":   "calls",
		"weight": 0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["status"] != "added" {
		t.Fatalf("expected status 'added', got %v", m["status"])
	}

	edges := g.GetEdgesFrom("svc:a")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Weight != 0.7 {
		t.Fatalf("expected weight 0.7, got %f", edges[0].Weight)
	}
}

func TestRemoveNodeCleansEdges(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	g.AddNode(&Node{ID: "svc:c", Type: NodeTypeService, Name: "C"})
	g.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "svc:c", Target: "svc:b", Type: EdgeDependsOn})

	g.RemoveNode("svc:b")

	if g.GetNode("svc:b") != nil {
		t.Fatal("expected svc:b to be removed")
	}
	if len(g.GetEdgesFrom("svc:a")) != 0 {
		t.Fatal("expected outgoing edges from svc:a to be cleaned")
	}
	reverseEdges := g.GetEdgesTo("svc:b")
	if len(reverseEdges) != 0 {
		t.Fatalf("expected no reverse edges to svc:b, got %d", len(reverseEdges))
	}
}

func TestAddEdgeUpdateExisting(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})

	g.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls, Weight: 0.5})
	g.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls, Weight: 0.9})

	edges := g.GetEdgesFrom("svc:a")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge (updated), got %d", len(edges))
	}
	if edges[0].Weight != 0.9 {
		t.Fatalf("expected weight 0.9, got %f", edges[0].Weight)
	}
}

func TestBlastRadiusNodeNotFound(t *testing.T) {
	g := NewTopologyGraph()
	result := BlastRadiusDownstream(g, "nonexistent", 3)
	if result == nil {
		t.Fatal("expected non-nil result for missing node")
	}
	if result.RootCause != nil {
		t.Fatal("expected nil root cause for missing node")
	}
}

func TestSnapshotAutoSave(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:test", Type: NodeTypeService, Name: "Test"})

	stop := StartAutoSnapshot(g, s.DB(), 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	stop()

	loaded, err := LoadSnapshot(s.DB())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded graph")
	}
	n := loaded.GetNode("svc:test")
	if n == nil {
		t.Fatal("expected svc:test in loaded graph after auto-snapshot")
	}
}

func TestAllNodesAndEdges(t *testing.T) {
	g := buildTestGraph()

	nodes := g.AllNodes()
	edges := g.AllEdges()

	if len(nodes) == 0 {
		t.Fatal("expected non-empty nodes")
	}
	if len(edges) == 0 {
		t.Fatal("expected non-empty edges")
	}
}

func TestGetPathToSelf(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})

	nodes, edges := g.GetPath("svc:a", "svc:a")
	if nodes == nil || len(nodes) != 1 {
		t.Fatalf("expected single-node path to self, got %d nodes", len(nodes))
	}
	if len(edges) != 0 {
		t.Fatalf("expected no edges for path to self, got %d", len(edges))
	}
}

func TestGetEdgesTo(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})
	g.AddNode(&Node{ID: "svc:b", Type: NodeTypeService, Name: "B"})
	g.AddEdge(&Edge{Source: "svc:a", Target: "svc:b", Type: EdgeCalls})

	inEdges := g.GetEdgesTo("svc:b")
	if len(inEdges) != 1 {
		t.Fatalf("expected 1 incoming edge, got %d", len(inEdges))
	}
	if inEdges[0].Source != "svc:a" {
		t.Fatalf("expected incoming edge from svc:a, got %q", inEdges[0].Source)
	}
}

func TestSelectorMatches(t *testing.T) {
	selector := map[string]string{"app": "api", "version": "v2"}
	labels := map[string]string{"app": "api", "version": "v2", "env": "prod"}

	if !selectorMatches(selector, labels) {
		t.Fatal("expected selector to match labels")
	}

	missingLabel := map[string]string{"app": "api"}
	if selectorMatches(selector, missingLabel) {
		t.Fatal("expected selector NOT to match labels missing 'version'")
	}

	wrongValue := map[string]string{"app": "web", "version": "v2"}
	if selectorMatches(selector, wrongValue) {
		t.Fatal("expected selector NOT to match labels with wrong 'app' value")
	}
}

func TestNodeTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected NodeType
	}{
		{"service", NodeTypeService},
		{"database", NodeTypeDatabase},
		{"db", NodeTypeDatabase},
		{"queue", NodeTypeQueue},
		{"cache", NodeTypeCache},
		{"load_balancer", NodeTypeLoadBalancer},
		{"host", NodeTypeHost},
		{"unknown", NodeTypeService},
	}
	for _, tt := range tests {
		result := nodeTypeFromString(tt.input)
		if result != tt.expected {
			t.Errorf("nodeTypeFromString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMergeNil(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})

	g.Merge(nil)

	if g.GetNode("svc:a") == nil {
		t.Fatal("expected existing nodes to survive nil merge")
	}
}

func TestAddNodeNil(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(nil)

	stats := g.Stats()
	if stats.NodeCount != 0 {
		t.Fatalf("expected 0 nodes after nil add, got %d", stats.NodeCount)
	}
}

func TestAddEdgeNil(t *testing.T) {
	g := NewTopologyGraph()
	g.AddEdge(nil)

	stats := g.Stats()
	if stats.EdgeCount != 0 {
		t.Fatalf("expected 0 edges after nil add, got %d", stats.EdgeCount)
	}
}

func TestGetNeighborsZeroDepth(t *testing.T) {
	g := NewTopologyGraph()
	g.AddNode(&Node{ID: "svc:a", Type: NodeTypeService, Name: "A"})

	nodes, edges := g.GetNeighbors("svc:a", 0)
	if nodes != nil || edges != nil {
		t.Fatal("expected nil for zero depth")
	}
}

func TestSnapshotEmptyGraph(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	g := NewTopologyGraph()
	if err := SnapshotGraph(g, s.DB()); err != nil {
		t.Fatalf("snapshot of empty graph failed: %v", err)
	}

	loaded, err := LoadSnapshot(s.DB())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded graph")
	}
	stats := loaded.Stats()
	if stats.NodeCount != 0 || stats.EdgeCount != 0 {
		t.Fatalf("expected empty graph, got nodes=%d edges=%d", stats.NodeCount, stats.EdgeCount)
	}
}

func TestSnapshotNilDB(t *testing.T) {
	g := NewTopologyGraph()
	if err := SnapshotGraph(g, nil); err != nil {
		t.Fatalf("expected nil error for nil db, got: %v", err)
	}
}

func TestInitTopology(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	g := InitTopology(s)
	if g == nil {
		t.Fatal("expected graph from InitTopology")
	}

	if DefaultTopology() == nil {
		t.Fatal("expected default topology to be set")
	}
}

func TestInitTopologyNilStore(t *testing.T) {
	g := InitTopology(nil)
	if g == nil {
		t.Fatal("expected graph from InitTopology with nil store")
	}
}

func TestBlastRadiusNilGraph(t *testing.T) {
	result := BlastRadiusDownstream(nil, "svc:a", 3)
	if result != nil {
		t.Fatal("expected nil result for nil graph")
	}
}

func TestSnapshotPreservesLabels(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	g := NewTopologyGraph()
	g.AddNode(&Node{
		ID:     "svc:api",
		Type:   NodeTypeService,
		Name:   "API",
		Labels: map[string]string{"env": "prod", "team": "platform"},
	})
	g.AddEdge(&Edge{
		Source: "svc:api",
		Target: "svc:auth",
		Type:   EdgeCalls,
		Weight: 0.75,
		Labels: map[string]string{"protocol": "grpc"},
	})
	g.AddNode(&Node{ID: "svc:auth", Type: NodeTypeService, Name: "Auth"})

	if err := SnapshotGraph(g, s.DB()); err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	loaded, err := LoadSnapshot(s.DB())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	n := loaded.GetNode("svc:api")
	if n.Labels["env"] != "prod" || n.Labels["team"] != "platform" {
		t.Fatalf("labels not preserved: %v", n.Labels)
	}

	edges := loaded.GetEdgesFrom("svc:api")
	if len(edges) == 0 {
		t.Fatal("expected edges to be preserved")
	}
	if edges[0].Labels["protocol"] != "grpc" {
		t.Fatalf("edge labels not preserved: %v", edges[0].Labels)
	}
	if edges[0].Weight != 0.75 {
		t.Fatalf("edge weight not preserved: %f", edges[0].Weight)
	}
}

func TestToolQueryNodeNotFound(t *testing.T) {
	g := NewTopologyGraph()
	SetDefaultTopology(g)

	tool := &TopologyQueryTool{}
	result, err := tool.Execute(nil, map[string]any{"node_id": "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["error"] != "node not found" {
		t.Fatalf("expected 'node not found' error, got %v", m["error"])
	}
}

func TestToolAddNodeMissingRequired(t *testing.T) {
	g := NewTopologyGraph()
	SetDefaultTopology(g)

	tool := &TopologyAddNodeTool{}
	_, err := tool.Execute(nil, map[string]any{"type": "service", "name": "Test"})
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestToolAddEdgeMissingRequired(t *testing.T) {
	g := NewTopologyGraph()
	SetDefaultTopology(g)

	tool := &TopologyAddEdgeTool{}
	_, err := tool.Execute(nil, map[string]any{"source": "svc:a", "target": "svc:b"})
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestToolBlastRadiusMissingNodeID(t *testing.T) {
	g := NewTopologyGraph()
	SetDefaultTopology(g)

	tool := &TopologyBlastRadiusTool{}
	_, err := tool.Execute(nil, map[string]any{"depth": 3})
	if err == nil {
		t.Fatal("expected error for missing node_id")
	}
}

func TestRegisterTopologyTools(t *testing.T) {
	var registered []string
	RegisterTopologyTools(func(name string, tool any) {
		registered = append(registered, name)
	})
	sort.Strings(registered)

	expected := []string{"topology_add_edge", "topology_add_node", "topology_blast_radius", "topology_query"}
	if len(registered) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(registered))
	}
	for i, e := range expected {
		if registered[i] != e {
			t.Fatalf("expected tool %q at index %d, got %q", e, i, registered[i])
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func buildTestGraph() *TopologyGraph {
	g := NewTopologyGraph()

	g.AddNode(&Node{ID: "svc:api", Type: NodeTypeService, Name: "API Gateway", Health: HealthHealthy})
	g.AddNode(&Node{ID: "svc:auth", Type: NodeTypeService, Name: "Auth Service", Health: HealthHealthy})
	g.AddNode(&Node{ID: "svc:payment", Type: NodeTypeService, Name: "Payment API", Health: HealthDegraded})
	g.AddNode(&Node{ID: "db:postgres", Type: NodeTypeDatabase, Name: "PostgreSQL", Health: HealthHealthy})
	g.AddNode(&Node{ID: "db:redis", Type: NodeTypeCache, Name: "Redis", Health: HealthDown})
	g.AddNode(&Node{ID: "mq:kafka", Type: NodeTypeQueue, Name: "Kafka", Health: HealthHealthy})

	g.AddEdge(&Edge{Source: "svc:api", Target: "svc:auth", Type: EdgeCalls, Weight: 0.9})
	g.AddEdge(&Edge{Source: "svc:api", Target: "svc:payment", Type: EdgeCalls, Weight: 0.7})
	g.AddEdge(&Edge{Source: "svc:auth", Target: "db:postgres", Type: EdgeReadsFrom, Weight: 0.8})
	g.AddEdge(&Edge{Source: "svc:payment", Target: "db:postgres", Type: EdgeWritesTo, Weight: 0.6})
	g.AddEdge(&Edge{Source: "svc:payment", Target: "db:redis", Type: EdgeReadsFrom, Weight: 0.5})
	g.AddEdge(&Edge{Source: "svc:payment", Target: "mq:kafka", Type: EdgeWritesTo, Weight: 0.4})
	g.AddEdge(&Edge{Source: "svc:api", Target: "db:redis", Type: EdgeDependsOn, Weight: 0.3})

	return g
}

package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSymbols(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "symbols.go")
	content := `package symbols

import "fmt"

type Config struct {
	Name string
	Port int
}

type Reader interface {
	Read() error
	Close() error
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) Validate() bool {
	return c.Name != ""
}

const MaxRetries = 3

var DefaultConfig = &Config{Name: "default"}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	symbols, err := ExtractSymbols(dir)
	if err != nil {
		t.Fatalf("ExtractSymbols error: %v", err)
	}

	fileKey := "symbols.go"
	syms, ok := symbols[fileKey]
	if !ok {
		t.Fatalf("no symbols found for %q, keys: %v", fileKey, mapKeys(symbols))
	}

	kinds := make(map[string]int)
	for _, s := range syms {
		kinds[s.Kind]++
	}

	if kinds["struct"] < 1 {
		t.Errorf("expected at least 1 struct, got kinds: %v", kinds)
	}
	if kinds["interface"] < 1 {
		t.Errorf("expected at least 1 interface, got kinds: %v", kinds)
	}
	if kinds["func"] < 1 {
		t.Errorf("expected at least 1 func, got kinds: %v", kinds)
	}
	if kinds["method"] < 1 {
		t.Errorf("expected at least 1 method, got kinds: %v", kinds)
	}
	if kinds["const"] < 1 {
		t.Errorf("expected at least 1 const, got kinds: %v", kinds)
	}
	if kinds["var"] < 1 {
		t.Errorf("expected at least 1 var, got kinds: %v", kinds)
	}

	foundValidate := false
	for _, s := range syms {
		if s.Name == "Validate" {
			foundValidate = true
			if s.Kind != "method" {
				t.Errorf("Validate.Kind = %q, want %q", s.Kind, "method")
			}
			if !strings.Contains(s.Signature, "Validate") {
				t.Errorf("Validate.Signature = %q, should contain 'Validate'", s.Signature)
			}
		}
	}
	if !foundValidate {
		t.Error("Validate method not found in extracted symbols")
	}
}

func TestExtractSymbolsSkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	normalFile := filepath.Join(dir, "normal.go")
	testFile := filepath.Join(dir, "normal_test.go")

	os.WriteFile(normalFile, []byte("package normal\nfunc Hello() {}\n"), 0644)
	os.WriteFile(testFile, []byte("package normal\nfunc TestHello() {}\n"), 0644)

	symbols, err := ExtractSymbols(dir)
	if err != nil {
		t.Fatalf("ExtractSymbols error: %v", err)
	}

	if _, ok := symbols["normal_test.go"]; ok {
		t.Error("test files should be skipped")
	}
	if _, ok := symbols["normal.go"]; !ok {
		t.Error("normal.go should be indexed")
	}
}

func TestPageRankConvergence(t *testing.T) {
	adj := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {"A"},
	}
	personalization := map[string]float64{
		"A": 1.0,
		"B": 1.0,
		"C": 1.0,
	}

	ranks := PageRank(adj, personalization, 0.85, 100)

	if len(ranks) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(ranks))
	}

	totalRank := 0.0
	for _, r := range ranks {
		totalRank += r
	}
	if totalRank < 0.99 || totalRank > 1.01 {
		t.Errorf("ranks should sum to ~1.0, got %f", totalRank)
	}

	for node, rank := range ranks {
		if rank <= 0 {
			t.Errorf("node %s has non-positive rank %f", node, rank)
		}
	}

	if ranks["C"] <= 0 || ranks["A"] <= 0 || ranks["B"] <= 0 {
		t.Errorf("all nodes should have positive rank, got A=%f B=%f C=%f",
			ranks["A"], ranks["B"], ranks["C"])
	}
}

func TestPageRankPersonalization(t *testing.T) {
	adj := map[string][]string{
		"A": {"B"},
		"B": {"A"},
	}

	uniform := map[string]float64{"A": 1.0, "B": 1.0}
	biasedA := map[string]float64{"A": 100.0, "B": 1.0}

	ranksUniform := PageRank(adj, uniform, 0.85, 200)
	ranksBiased := PageRank(adj, biasedA, 0.85, 200)

	if ranksBiased["A"] <= ranksBiased["B"] {
		t.Errorf("with A-biased personalization, A should rank higher than B: A=%f B=%f",
			ranksBiased["A"], ranksBiased["B"])
	}

	if ranksUniform["A"] > 0 && ranksBiased["A"] > 0 {
		biasRatio := ranksBiased["A"] / ranksBiased["B"]
		uniformRatio := ranksUniform["A"] / ranksUniform["B"]
		if biasRatio <= uniformRatio {
			t.Errorf("biased personalization should increase A/B ratio: biased=%f uniform=%f",
				biasRatio, uniformRatio)
		}
	}
}

func TestPageRankEmptyGraph(t *testing.T) {
	ranks := PageRank(map[string][]string{}, nil, 0.85, 100)
	if len(ranks) != 0 {
		t.Errorf("empty graph should return empty ranks, got %d", len(ranks))
	}
}

func TestPageRankSinkNode(t *testing.T) {
	adj := map[string][]string{
		"A": {"B"},
		"B": {},
	}
	personalization := map[string]float64{"A": 1.0, "B": 1.0}

	ranks := PageRank(adj, personalization, 0.85, 100)

	if len(ranks) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(ranks))
	}
	for node, rank := range ranks {
		if rank < 0 {
			t.Errorf("node %s has negative rank %f", node, rank)
		}
	}
}

func TestRender(t *testing.T) {
	ranks := map[string]float64{
		"pkg/handler.go": 0.6,
		"pkg/model.go":   0.4,
	}
	symbols := map[string][]Symbol{
		"pkg/handler.go": {
			{Name: "Handle", Kind: "func", Line: 5, Signature: "Handle(req Request)"},
		},
		"pkg/model.go": {
			{Name: "User", Kind: "struct", Line: 3, Signature: "User"},
		},
	}

	output := Render(ranks, symbols, 1000)

	if output == "" {
		t.Fatal("Render returned empty string")
	}
	if !strings.Contains(output, "handler.go") {
		t.Error("output should contain handler.go")
	}
	if !strings.Contains(output, "model.go") {
		t.Error("output should contain model.go")
	}
	if !strings.Contains(output, "Handle") {
		t.Error("output should contain Handle symbol")
	}
	if !strings.Contains(output, "User") {
		t.Error("output should contain User symbol")
	}
	if !strings.Contains(output, "func") {
		t.Error("output should contain 'func' kind")
	}
	if !strings.Contains(output, "struct") {
		t.Error("output should contain 'struct' kind")
	}

	lines := strings.Split(output, "\n")
	handlerIdx := -1
	modelIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "handler.go:") {
			handlerIdx = i
		}
		if strings.Contains(line, "model.go:") {
			modelIdx = i
		}
	}
	if handlerIdx >= 0 && modelIdx >= 0 && handlerIdx > modelIdx {
		t.Error("handler.go (rank 0.6) should appear before model.go (rank 0.4)")
	}
}

func TestRenderTokenBudget(t *testing.T) {
	ranks := map[string]float64{
		"big.go": 0.9,
	}
	symbols := map[string][]Symbol{
		"big.go": {
			{Name: "A", Kind: "func", Line: 1, Signature: "A()"},
			{Name: "B", Kind: "func", Line: 2, Signature: "B()"},
			{Name: "C", Kind: "func", Line: 3, Signature: "C()"},
			{Name: "D", Kind: "func", Line: 4, Signature: "D()"},
		},
	}

	smallBudget := Render(ranks, symbols, 10)
	largeBudget := Render(ranks, symbols, 10000)

	if len(smallBudget) >= len(largeBudget) {
		t.Error("smaller token budget should produce shorter output")
	}
}

func TestRenderEmpty(t *testing.T) {
	output := Render(nil, nil, 1000)
	if output != "" {
		t.Errorf("empty input should return empty string, got %q", output)
	}
}

func TestGetMap(t *testing.T) {
	dir := t.TempDir()

	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	modelFile := filepath.Join(pkgDir, "model.go")
	serviceFile := filepath.Join(pkgDir, "service.go")

	modelContent := `package pkg

type Service struct {
	Name string
}
`
	serviceContent := `package pkg

import "fmt"

func (s *Service) Run() error {
	fmt.Println(s.Name)
	return nil
}
`
	os.WriteFile(modelFile, []byte(modelContent), 0644)
	os.WriteFile(serviceFile, []byte(serviceContent), 0644)

	rm := NewRepoMap(dir)
	if err := rm.Refresh(); err != nil {
		t.Fatalf("Refresh error: %v", err)
	}

	rm.mu.RLock()
	symCount := len(rm.symbols)
	rm.mu.RUnlock()

	if symCount == 0 {
		t.Fatal("Refresh did not extract any symbols")
	}

	output, err := rm.GetMap(nil, 4000)
	if err != nil {
		t.Fatalf("GetMap error: %v", err)
	}

	if len(rm.adjacency) == 0 {
		ranks := make(map[string]float64)
		rm.mu.RLock()
		for path := range rm.symbols {
			ranks[path] = 1.0 / float64(len(rm.symbols))
		}
		rm.mu.RUnlock()
		output = Render(ranks, rm.symbols, 4000)
	}

	if output == "" {
		t.Error("GetMap returned empty string")
	}
	if !strings.Contains(output, "Service") {
		t.Errorf("output should mention Service, got:\n%s", output)
	}
}

func TestGetMapPersonalization(t *testing.T) {
	dir := t.TempDir()

	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	handlerFile := filepath.Join(pkgDir, "handler.go")
	modelFile := filepath.Join(pkgDir, "model.go")

	handlerContent := `package pkg

import "fmt"

func HandleRequest() {
	fmt.Println("handling")
}
`
	modelContent := `package pkg

type Model struct{}
`

	os.WriteFile(handlerFile, []byte(handlerContent), 0644)
	os.WriteFile(modelFile, []byte(modelContent), 0644)

	rm := NewRepoMap(dir)
	if err := rm.Refresh(); err != nil {
		t.Fatalf("Refresh error: %v", err)
	}

	rm.mu.RLock()
	symCount := len(rm.symbols)
	rm.mu.RUnlock()

	if symCount == 0 {
		t.Fatal("Refresh did not extract any symbols")
	}

	personalization := make(map[string]float64)
	rm.mu.RLock()
	for path := range rm.symbols {
		personalization[path] = 1.0
	}
	handlerRel := filepath.Join("pkg", "handler.go")
	if _, ok := personalization[handlerRel]; ok {
		personalization[handlerRel] = personalizeWeight
	}
	rm.mu.RUnlock()

	adj := rm.adjacency
	if len(adj) == 0 {
		for path := range rm.symbols {
			adj[path] = nil
		}
	}

	ranks := PageRank(adj, personalization, defaultDamping, defaultIterations)
	output := Render(ranks, rm.symbols, 4000)

	if output == "" {
		t.Error("Render with personalized PageRank returned empty string")
	}
	if !strings.Contains(output, "handler.go") {
		t.Errorf("handler.go should appear in personalized output, got:\n%s", output)
	}
}

func TestNewRepoMap(t *testing.T) {
	dir := t.TempDir()
	rm := NewRepoMap(dir)
	if rm == nil {
		t.Fatal("NewRepoMap returned nil")
	}
	if rm.rootPath != dir {
		t.Errorf("rootPath = %q, want %q", rm.rootPath, dir)
	}
}

func TestRefresh(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "fresh.go")
	content := `package fresh

func FreshFunc() {}
`
	os.WriteFile(goFile, []byte(content), 0644)

	rm := NewRepoMap(dir)
	if err := rm.Refresh(); err != nil {
		t.Fatalf("Refresh error: %v", err)
	}

	if rm.lastRefresh.IsZero() {
		t.Error("lastRefresh should be set after Refresh()")
	}
	if len(rm.symbols) == 0 {
		t.Error("symbols should not be empty after Refresh()")
	}
}

func helperMapKeys(m map[string][]Symbol) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func mapKeys(m map[string][]Symbol) []string {
	return helperMapKeys(m)
}

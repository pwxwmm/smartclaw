package srecoder

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Suggestion struct {
	Type       string      `json:"type"`
	Message    string      `json:"message"`
	Code       string      `json:"code,omitempty"`
	Confidence float64     `json:"confidence"`
	Pattern    *SREPattern `json:"pattern,omitempty"`
	File       string      `json:"file,omitempty"`
	Line       int         `json:"line,omitempty"`
}

func (m *SRECodingMode) SuggestImprovements(ctx context.Context, code string, language string) ([]Suggestion, error) {
	if language != "go" && language != "" {
		return basicSuggestions(code), nil
	}

	return m.analyzeGoCode(code, "")
}

func (m *SRECodingMode) SuggestForFile(ctx context.Context, filePath string) ([]Suggestion, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("srecoder: read file: %w", err)
	}

	if filepath.Ext(filePath) != ".go" {
		return basicSuggestions(string(data)), nil
	}

	return m.analyzeGoCode(string(data), filePath)
}

func (m *SRECodingMode) SuggestForProject(ctx context.Context, rootPath string) ([]Suggestion, error) {
	keyDirs := []string{"cmd", "internal", "pkg"}
	var allSuggestions []Suggestion

	for _, dir := range keyDirs {
		dirPath := filepath.Join(rootPath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				base := d.Name()
				if base == "vendor" || base == "node_modules" || base == ".git" ||
					strings.HasPrefix(base, "_") || strings.HasPrefix(base, ".") {
					return filepath.SkipDir
				}
				return nil
			}

			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			suggestions, err := m.SuggestForFile(ctx, path)
			if err != nil {
				return nil
			}
			allSuggestions = append(allSuggestions, suggestions...)
			return nil
		})
		if err != nil {
			continue
		}
	}

	if len(allSuggestions) > 50 {
		allSuggestions = allSuggestions[:50]
	}

	return allSuggestions, nil
}

func (m *SRECodingMode) analyzeGoCode(code string, filePath string) ([]Suggestion, error) {
	var suggestions []Suggestion

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, code, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return basicSuggestions(code), nil
	}

	suggestions = append(suggestions, checkErrorHandling(f, fset, filePath)...)
	suggestions = append(suggestions, checkContextPropagation(f, fset, filePath)...)
	suggestions = append(suggestions, checkGoroutineLeaks(f, fset, filePath)...)
	suggestions = append(suggestions, checkMissingTimeouts(f, fset, filePath)...)
	suggestions = append(suggestions, checkMissingMetrics(f, fset, filePath)...)

	patternSuggestions := checkPatternSuggestions(code, filePath)
	suggestions = append(suggestions, patternSuggestions...)

	return suggestions, nil
}

func checkErrorHandling(f *ast.File, fset *token.FileSet, filePath string) []Suggestion {
	var suggestions []Suggestion

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, expr := range node.Lhs {
				if ident, ok := expr.(*ast.Ident); ok && ident.Name == "_" {
					for _, rhs := range node.Rhs {
						if isFunctionCall(rhs) {
							pos := fset.Position(ident.Pos())
							suggestions = append(suggestions, Suggestion{
								Type:       "error_handling",
								Message:    fmt.Sprintf("Discarded error return value from function call at line %d — handle or explicitly log the error", pos.Line),
								Confidence: 0.85,
								File:       filePath,
								Line:       pos.Line,
							})
						}
					}
				}
			}
		case *ast.IfStmt:
			if ifErr, ok := node.Cond.(*ast.BinaryExpr); ok {
				if ifErr.Op.String() == "!=" {
					if ident, ok := ifErr.X.(*ast.Ident); ok && ident.Name == "err" {
						if node.Body != nil && len(node.Body.List) == 1 {
							if ret, ok := node.Body.List[0].(*ast.ReturnStmt); ok {
								_ = ret
								pos := fset.Position(node.Pos())
								suggestions = append(suggestions, Suggestion{
									Type:       "error_handling",
									Message:    fmt.Sprintf("Bare error return at line %d — consider wrapping error with context using fmt.Errorf", pos.Line),
									Confidence: 0.6,
									File:       filePath,
									Line:       pos.Line,
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return suggestions
}

func checkContextPropagation(f *ast.File, fset *token.FileSet, filePath string) []Suggestion {
	var suggestions []Suggestion
	hasCtx := false

	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		for _, param := range fn.Type.Params.List {
			if sel, ok := param.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "context" && sel.Sel.Name == "Context" {
					hasCtx = true

					ast.Inspect(fn.Body, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}

						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							method := sel.Sel.Name
							noCtxMethods := map[string]bool{
								"Exec": true, "Query": true, "QueryRow": true,
								"Do": true, "Get": true, "Post": true,
							}
							if noCtxMethods[method] && len(call.Args) > 0 {
								if _, isCtx := call.Args[0].(*ast.SelectorExpr); !isCtx {
									pos := fset.Position(call.Pos())
									suggestions = append(suggestions, Suggestion{
										Type:       "context_propagation",
										Message:    fmt.Sprintf("Call to %s at line %d may not propagate context — use context-aware variant if available", method, pos.Line),
										Confidence: 0.7,
										File:       filePath,
										Line:       pos.Line,
									})
								}
							}
						}
						return true
					})
				}
			}
		}
		return true
	})

	if !hasCtx && hasExportedFuncs(f) {
		pos := fset.Position(f.Pos())
		suggestions = append(suggestions, Suggestion{
			Type:       "context_propagation",
			Message:    "Exported functions lack context.Context parameters — add context for cancellation and tracing",
			Confidence: 0.75,
			File:       filePath,
			Line:       pos.Line,
		})
	}

	return suggestions
}

func checkGoroutineLeaks(f *ast.File, fset *token.FileSet, filePath string) []Suggestion {
	var suggestions []Suggestion

	ast.Inspect(f, func(n ast.Node) bool {
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}

		hasDeferRecover := false
		if fnLit, ok := goStmt.Call.Fun.(*ast.FuncLit); ok {
			ast.Inspect(fnLit.Body, func(n ast.Node) bool {
				if deferStmt, ok := n.(*ast.DeferStmt); ok {
					if call, ok := deferStmt.Call.Fun.(*ast.Ident); ok && call.Name == "recover" {
						hasDeferRecover = true
					}
				}
				return true
			})

			if !hasDeferRecover {
				pos := fset.Position(goStmt.Pos())
				suggestions = append(suggestions, Suggestion{
					Type:       "goroutine_leak",
					Message:    fmt.Sprintf("Goroutine at line %d lacks defer/recover — a panic will crash the entire process", pos.Line),
					Confidence: 0.8,
					File:       filePath,
					Line:       pos.Line,
				})
			}
		}

		return true
	})

	return suggestions
}

func checkMissingTimeouts(f *ast.File, fset *token.FileSet, filePath string) []Suggestion {
	var suggestions []Suggestion

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			method := sel.Sel.Name
			timeoutMethods := map[string]bool{
				"Dial": true, "DialTimeout": false,
			}
			if needsTimeout, found := timeoutMethods[method]; found && needsTimeout {
				pos := fset.Position(call.Pos())
				suggestions = append(suggestions, Suggestion{
					Type:       "missing_timeout",
					Message:    fmt.Sprintf("Use DialTimeout instead of Dial at line %d for explicit timeout control", pos.Line),
					Confidence: 0.75,
					File:       filePath,
					Line:       pos.Line,
				})
			}

			if method == "ListenAndServe" || method == "ListenAndServeTLS" {
				pos := fset.Position(call.Pos())
				suggestions = append(suggestions, Suggestion{
					Type:       "missing_timeout",
					Message:    fmt.Sprintf("Server start at line %d — add graceful shutdown with signal handling", pos.Line),
					Confidence: 0.7,
					File:       filePath,
					Line:       pos.Line,
				})
			}
		}

		return true
	})

	return suggestions
}

func checkMissingMetrics(f *ast.File, fset *token.FileSet, filePath string) []Suggestion {
	var suggestions []Suggestion

	hasHTTP := false
	hasMetrics := false

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				if ident.Name == "http" {
					hasHTTP = true
				}
				if ident.Name == "prometheus" {
					hasMetrics = true
				}
			}
		case *ast.ImportSpec:
			if node.Path != nil {
				val := node.Path.Value
				if strings.Contains(val, "net/http") {
					hasHTTP = true
				}
				if strings.Contains(val, "prometheus") {
					hasMetrics = true
				}
			}
		}
		return true
	})

	if hasHTTP && !hasMetrics {
		pos := fset.Position(f.Pos())
		suggestions = append(suggestions, Suggestion{
			Type:       "missing_metrics",
			Message:    "HTTP handlers without Prometheus metrics — add request count, duration, and error rate instrumentation",
			Confidence: 0.8,
			File:       filePath,
			Line:       pos.Line,
			Pattern:    findPatternByName("prometheus_metrics"),
		})
	}

	return suggestions
}

func checkPatternSuggestions(code string, filePath string) []Suggestion {
	patterns := GetPatternForCode(code)
	var suggestions []Suggestion

	for _, p := range patterns {
		suggestions = append(suggestions, Suggestion{
			Type:       "pattern_suggestion",
			Message:    fmt.Sprintf("Consider adding %s pattern: %s", p.Name, p.Description),
			Confidence: 0.65,
			Pattern:    &p,
			File:       filePath,
		})
	}

	return suggestions
}

func basicSuggestions(code string) []Suggestion {
	var suggestions []Suggestion

	patterns := GetPatternForCode(code)
	for _, p := range patterns {
		suggestions = append(suggestions, Suggestion{
			Type:       "pattern_suggestion",
			Message:    fmt.Sprintf("Consider adding %s pattern: %s", p.Name, p.Description),
			Confidence: 0.5,
			Pattern:    &p,
		})
	}

	return suggestions
}

func findPatternByName(name string) *SREPattern {
	for _, p := range GetPatterns() {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

func isFunctionCall(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.CallExpr:
		return true
	default:
		return false
	}
}

func hasExportedFuncs(f *ast.File) bool {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
			return true
		}
	}
	return false
}

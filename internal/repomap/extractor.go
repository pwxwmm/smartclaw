package repomap

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Symbol represents a code symbol extracted from a Go source file.
type Symbol struct {
	Name      string // identifier name
	Kind      string // func, method, type, interface, struct, const, var
	File      string // relative file path from root
	Line      int    // start line (1-based)
	EndLine   int    // end line (1-based)
	Signature string // function/method signature or type declaration
}

// ExtractSymbols walks all .go files under rootPath (skipping vendor/ and .git/),
// parses them with go/parser, and returns a map of filePath → []Symbol.
func ExtractSymbols(rootPath string) (map[string][]Symbol, error) {
	fset := token.NewFileSet()
	result := make(map[string][]Symbol)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, relErr := filepath.Rel(rootPath, path)
		if relErr != nil {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil
		}

		syms := extractFromFile(fset, file, rel)
		if len(syms) > 0 {
			result[rel] = syms
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", rootPath, err)
	}
	return result, nil
}

func extractFromFile(fset *token.FileSet, file *ast.File, relPath string) []Symbol {
	var syms []Symbol

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			syms = append(syms, extractGenDecl(fset, d, relPath)...)
		case *ast.FuncDecl:
			syms = append(syms, extractFuncDecl(fset, d, relPath))
		}
	}

	return syms
}

func extractGenDecl(fset *token.FileSet, d *ast.GenDecl, relPath string) []Symbol {
	var syms []Symbol

	startLine := fset.Position(d.Pos()).Line
	endLine := fset.Position(d.End()).Line

	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			kind := "type"
			sig := s.Name.Name
			if s.TypeParams != nil {
				sig += formatFieldList(s.TypeParams)
			}

			switch s.Type.(type) {
			case *ast.InterfaceType:
				kind = "interface"
			case *ast.StructType:
				kind = "struct"
			}

			syms = append(syms, Symbol{
				Name:      s.Name.Name,
				Kind:      kind,
				File:      relPath,
				Line:      startLine,
				EndLine:   endLine,
				Signature: sig,
			})

		case *ast.ValueSpec:
			kind := "const"
			if d.Tok.String() == "var" {
				kind = "var"
			}
			for _, name := range s.Names {
				sig := name.Name
				if s.Type != nil {
					sig += " " + exprToString(s.Type)
				}
				syms = append(syms, Symbol{
					Name:      name.Name,
					Kind:      kind,
					File:      relPath,
					Line:      startLine,
					EndLine:   endLine,
					Signature: sig,
				})
			}
		}
	}

	return syms
}

func extractFuncDecl(fset *token.FileSet, d *ast.FuncDecl, relPath string) Symbol {
	kind := "func"
	name := d.Name.Name
	sig := name

	if d.Recv != nil && len(d.Recv.List) > 0 {
		kind = "method"
		recv := formatFieldList(d.Recv)
		sig = recv + "." + name
	}

	sig += formatFieldList(d.Type.Params)
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sig += " " + formatFieldList(d.Type.Results)
	}

	startLine := fset.Position(d.Pos()).Line
	endLine := fset.Position(d.End()).Line

	return Symbol{
		Name:      name,
		Kind:      kind,
		File:      relPath,
		Line:      startLine,
		EndLine:   endLine,
		Signature: sig,
	}
}

func formatFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return "()"
	}
	var parts []string
	for _, f := range fl.List {
		typeStr := exprToString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			for _, n := range f.Names {
				parts = append(parts, n.Name+" "+typeStr)
			}
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + exprToString(e.Value)
		case ast.RECV:
			return "<-chan " + exprToString(e.Value)
		default:
			return "chan " + exprToString(e.Value)
		}
	case *ast.FuncType:
		return "func" + formatFieldList(e.Params) + formatFieldList(e.Results)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.ParenExpr:
		return "(" + exprToString(e.X) + ")"
	case *ast.UnaryExpr:
		return e.Op.String() + exprToString(e.X)
	case *ast.BasicLit:
		return e.Value
	default:
		return "any"
	}
}

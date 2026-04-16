package index

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FileInfo holds metadata about an indexed file.
type FileInfo struct {
	Path       string    `json:"path"`
	ModTime    time.Time `json:"mod_time"`
	Language   string    `json:"language"`
	LinesCount int       `json:"lines_count"`
	Symbols    []string  `json:"symbols"`
	Embedding  []float64 `json:"embedding,omitempty"`
}

// Symbol represents an extracted code symbol (function, type, method, etc.).
type Symbol struct {
	Name          string    `json:"name"`
	QualifiedName string    `json:"qualified_name"`
	Kind          string    `json:"kind"` // function, type, method, interface, struct, variable, constant
	File          string    `json:"file"`
	Line          int       `json:"line"`
	Signature     string    `json:"signature,omitempty"`
	DocComment    string    `json:"doc_comment,omitempty"`
	Embedding     []float64 `json:"embedding,omitempty"`
	Receiver      string    `json:"receiver,omitempty"` // for methods
	Exported      bool      `json:"exported"`
}

// Chunk represents a searchable text chunk from the codebase.
type Chunk struct {
	ID        string    `json:"id"`
	File      string    `json:"file"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Content   string    `json:"content"`
	Embedding []float64 `json:"embedding,omitempty"`
	Type      string    `json:"type"`                 // file, symbol, doc
	SymbolRef string    `json:"symbol_ref,omitempty"` // qualified name if type=symbol
}

// IndexStats provides summary statistics about the index.
type IndexStats struct {
	FileCount   int       `json:"file_count"`
	SymbolCount int       `json:"symbol_count"`
	ChunkCount  int       `json:"chunk_count"`
	LastIndexed time.Time `json:"last_indexed"`
	IndexSizeKB int64     `json:"index_size_kb"`
}

// CodebaseIndex is the main indexing structure for a project.
type CodebaseIndex struct {
	rootPath   string
	files      map[string]*FileInfo
	symbols    map[string]*Symbol   // qualified name -> symbol
	chunks     map[string]*Chunk    // chunk ID -> chunk
	embeddings map[string][]float64 // chunk ID -> embedding

	// BM25 data
	docFreq   map[string]int // term -> document frequency
	totalDocs int

	mu          sync.RWMutex
	lastIndexed time.Time

	// directories to skip
	skipDirs map[string]bool
}

// NewCodebaseIndex creates a new index for the given project root.
func NewCodebaseIndex(rootPath string) *CodebaseIndex {
	absPath, _ := filepath.Abs(rootPath)
	return &CodebaseIndex{
		rootPath:   absPath,
		files:      make(map[string]*FileInfo),
		symbols:    make(map[string]*Symbol),
		chunks:     make(map[string]*Chunk),
		embeddings: make(map[string][]float64),
		docFreq:    make(map[string]int),
		skipDirs: map[string]bool{
			".git": true, "node_modules": true, "vendor": true,
			"dist": true, "build": true, "bin": true, "__pycache__": true,
			".smartclaw": true, ".idea": true, ".vscode": true,
		},
	}
}

// Index performs a full project scan — walks source files, parses ASTs,
// extracts symbols, generates chunks, and computes embeddings.
func (idx *CodebaseIndex) Index() error {
	start := time.Now()
	idx.mu.Lock()
	// Reset
	idx.files = make(map[string]*FileInfo)
	idx.symbols = make(map[string]*Symbol)
	idx.chunks = make(map[string]*Chunk)
	idx.embeddings = make(map[string][]float64)
	idx.docFreq = make(map[string]int)
	idx.totalDocs = 0
	idx.mu.Unlock()

	err := filepath.WalkDir(idx.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if idx.skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !idx.shouldIndex(path) {
			return nil
		}
		return idx.IndexFile(path)
	})

	if err != nil {
		return fmt.Errorf("index walk failed: %w", err)
	}

	idx.mu.Lock()
	idx.lastIndexed = time.Now()
	idx.mu.Unlock()

	slog.Info("CodebaseIndex: full index complete",
		"root", idx.rootPath,
		"files", idx.GetStats().FileCount,
		"symbols", idx.GetStats().SymbolCount,
		"chunks", idx.GetStats().ChunkCount,
		"duration", time.Since(start),
	)
	return nil
}

// IndexFile indexes a single file — parses AST if Go, extracts symbols,
// generates chunks, and computes embeddings.
func (idx *CodebaseIndex) IndexFile(path string) error {
	relPath, err := filepath.Rel(idx.rootPath, path)
	if err != nil {
		relPath = path
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	lang := detectLanguage(path)
	if lang == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Remove old data for this file
	idx.removeFileData(relPath)

	// Create file info
	fileInfo := &FileInfo{
		Path:       relPath,
		ModTime:    info.ModTime(),
		Language:   lang,
		LinesCount: len(lines),
	}

	// Parse and extract symbols based on language
	var symbolNames []string
	if lang == "go" {
		symbolNames = idx.parseGoFile(path, relPath, content)
	} else {
		symbolNames = idx.parseGenericFile(path, relPath, content, lang)
	}
	fileInfo.Symbols = symbolNames

	// Generate file-level chunk
	fileChunk := idx.createChunk(relPath, 1, len(lines), content, "file", "")
	fileInfo.Embedding = fileChunk.Embedding

	// Store
	idx.mu.Lock()
	idx.files[relPath] = fileInfo
	idx.mu.Unlock()

	return nil
}

// RemoveFile removes a file and all its associated data from the index.
func (idx *CodebaseIndex) RemoveFile(path string) {
	relPath, err := filepath.Rel(idx.rootPath, path)
	if err != nil {
		relPath = path
	}
	idx.mu.Lock()
	idx.removeFileData(relPath)
	idx.mu.Unlock()
}

// removeFileData removes all index data for a file (must be called with lock held or from locked context).
func (idx *CodebaseIndex) removeFileData(relPath string) {
	// Remove symbols for this file
	for qname, sym := range idx.symbols {
		if sym.File == relPath {
			delete(idx.symbols, qname)
		}
	}

	// Remove chunks for this file
	for cid, chunk := range idx.chunks {
		if chunk.File == relPath {
			delete(idx.chunks, cid)
			delete(idx.embeddings, cid)
		}
	}

	delete(idx.files, relPath)
}

// GetStats returns summary statistics about the index.
func (idx *CodebaseIndex) GetStats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var sizeKB int64
	for _, chunk := range idx.chunks {
		sizeKB += int64(len(chunk.Content))
	}
	sizeKB = sizeKB / 1024

	return IndexStats{
		FileCount:   len(idx.files),
		SymbolCount: len(idx.symbols),
		ChunkCount:  len(idx.chunks),
		LastIndexed: idx.lastIndexed,
		IndexSizeKB: sizeKB,
	}
}

// Save persists the index to disk as JSON.
func (idx *CodebaseIndex) Save(indexPath string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return fmt.Errorf("create index dir: %w", err)
	}

	data := struct {
		RootPath    string               `json:"root_path"`
		Files       map[string]*FileInfo `json:"files"`
		Symbols     map[string]*Symbol   `json:"symbols"`
		Chunks      map[string]*Chunk    `json:"chunks"`
		DocFreq     map[string]int       `json:"doc_freq"`
		TotalDocs   int                  `json:"total_docs"`
		LastIndexed time.Time            `json:"last_indexed"`
	}{
		RootPath:    idx.rootPath,
		Files:       idx.files,
		Symbols:     idx.symbols,
		Chunks:      idx.chunks,
		DocFreq:     idx.docFreq,
		TotalDocs:   idx.totalDocs,
		LastIndexed: idx.lastIndexed,
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	path := filepath.Join(indexPath, "codebase.json")
	return os.WriteFile(path, b, 0644)
}

// Load restores the index from a previously saved JSON file.
func (idx *CodebaseIndex) Load(indexPath string) error {
	path := filepath.Join(indexPath, "codebase.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}

	var saved struct {
		RootPath    string               `json:"root_path"`
		Files       map[string]*FileInfo `json:"files"`
		Symbols     map[string]*Symbol   `json:"symbols"`
		Chunks      map[string]*Chunk    `json:"chunks"`
		DocFreq     map[string]int       `json:"doc_freq"`
		TotalDocs   int                  `json:"total_docs"`
		LastIndexed time.Time            `json:"last_indexed"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("unmarshal index: %w", err)
	}

	idx.mu.Lock()
	idx.rootPath = saved.RootPath
	idx.files = saved.Files
	idx.symbols = saved.Symbols
	idx.chunks = saved.Chunks
	idx.docFreq = saved.DocFreq
	idx.totalDocs = saved.TotalDocs
	idx.lastIndexed = saved.LastIndexed

	// Rebuild embeddings map from chunks
	idx.embeddings = make(map[string][]float64)
	for cid, chunk := range idx.chunks {
		if len(chunk.Embedding) > 0 {
			idx.embeddings[cid] = chunk.Embedding
		}
	}
	idx.mu.Unlock()

	slog.Info("CodebaseIndex: loaded from disk",
		"path", indexPath,
		"files", len(idx.files),
		"symbols", len(idx.symbols),
	)
	return nil
}

// RootPath returns the project root path.
func (idx *CodebaseIndex) RootPath() string {
	return idx.rootPath
}

// parseGoFile parses a Go file using go/ast and extracts symbols.
func (idx *CodebaseIndex) parseGoFile(absPath, relPath, _ string) []string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		slog.Debug("CodebaseIndex: failed to parse Go file", "file", relPath, "error", err)
		return nil
	}

	pkgName := node.Name.Name
	var symbolNames []string

	// Extract package-level declarations
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := idx.extractFuncDecl(d, relPath, pkgName, fset)
			if sym != nil {
				idx.mu.Lock()
				idx.symbols[sym.QualifiedName] = sym
				idx.mu.Unlock()
				symbolNames = append(symbolNames, sym.QualifiedName)

				// Create symbol chunk
				sig := sym.Signature
				if sym.DocComment != "" {
					sig = sym.DocComment + "\n" + sig
				}
				idx.createChunk(relPath, sym.Line, sym.Line, sig, "symbol", sym.QualifiedName)
			}

		case *ast.GenDecl:
			syms := idx.extractGenDecl(d, relPath, pkgName, fset)
			for _, sym := range syms {
				idx.mu.Lock()
				idx.symbols[sym.QualifiedName] = sym
				idx.mu.Unlock()
				symbolNames = append(symbolNames, sym.QualifiedName)

				sig := sym.Signature
				if sym.DocComment != "" {
					sig = sym.DocComment + "\n" + sig
				}
				idx.createChunk(relPath, sym.Line, sym.Line, sig, "symbol", sym.QualifiedName)
			}
		}
	}

	return symbolNames
}

// extractFuncDecl extracts a function or method symbol.
func (idx *CodebaseIndex) extractFuncDecl(decl *ast.FuncDecl, relPath, pkg string, fset *token.FileSet) *Symbol {
	name := decl.Name.Name
	line := fset.Position(decl.Pos()).Line
	exported := decl.Name.IsExported()

	kind := "function"
	receiver := ""
	qualifiedName := pkg + "." + name

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		kind = "method"
		recvType := receiverType(decl.Recv.List[0].Type)
		receiver = recvType
		qualifiedName = pkg + "." + recvType + "." + name
	}

	signature := formatFuncSignature(decl)

	docComment := ""
	if decl.Doc != nil {
		docComment = strings.TrimSpace(decl.Doc.Text())
	}

	// Generate embedding for the symbol
	textForEmbedding := name + " " + kind + " " + signature
	if docComment != "" {
		textForEmbedding += " " + docComment
	}
	embedding := GenerateEmbedding(textForEmbedding)

	return &Symbol{
		Name:          name,
		QualifiedName: qualifiedName,
		Kind:          kind,
		File:          relPath,
		Line:          line,
		Signature:     signature,
		DocComment:    docComment,
		Embedding:     embedding,
		Receiver:      receiver,
		Exported:      exported,
	}
}

// extractGenDecl extracts type, variable, and constant declarations.
func (idx *CodebaseIndex) extractGenDecl(decl *ast.GenDecl, relPath, pkg string, fset *token.FileSet) []*Symbol {
	var symbols []*Symbol

	docComment := ""
	if decl.Doc != nil {
		docComment = strings.TrimSpace(decl.Doc.Text())
	}

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			name := s.Name.Name
			line := fset.Position(s.Pos()).Line
			kind := "type"
			iface, isIface := s.Type.(*ast.InterfaceType)
			_, isStruct := s.Type.(*ast.StructType)

			if isIface {
				kind = "interface"
				_ = iface
			} else if isStruct {
				kind = "struct"
			}

			signature := formatTypeSignature(s, decl.Tok)
			qualifiedName := pkg + "." + name

			textForEmbedding := name + " " + kind + " " + signature
			if docComment != "" {
				textForEmbedding += " " + docComment
			}
			embedding := GenerateEmbedding(textForEmbedding)

			symbols = append(symbols, &Symbol{
				Name:          name,
				QualifiedName: qualifiedName,
				Kind:          kind,
				File:          relPath,
				Line:          line,
				Signature:     signature,
				DocComment:    docComment,
				Embedding:     embedding,
				Exported:      s.Name.IsExported(),
			})

		case *ast.ValueSpec:
			for _, ident := range s.Names {
				name := ident.Name
				line := fset.Position(ident.Pos()).Line
				kind := "variable"
				if decl.Tok.String() == "const" {
					kind = "constant"
				}
				qualifiedName := pkg + "." + name
				signature := fmt.Sprintf("%s %s", decl.Tok, name)
				if s.Type != nil {
					signature += " " + formatExpr(s.Type)
				}

				textForEmbedding := name + " " + kind
				embedding := GenerateEmbedding(textForEmbedding)

				symbols = append(symbols, &Symbol{
					Name:          name,
					QualifiedName: qualifiedName,
					Kind:          kind,
					File:          relPath,
					Line:          line,
					Signature:     signature,
					DocComment:    docComment,
					Embedding:     embedding,
					Exported:      ident.IsExported(),
				})
			}
		}
	}

	return symbols
}

// parseGenericFile provides basic symbol extraction for non-Go files.
func (idx *CodebaseIndex) parseGenericFile(_, relPath, content, lang string) []string {
	lines := strings.Split(content, "\n")
	var symbolNames []string

	patterns := languageSymbolPatterns(lang)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pat := range patterns {
			if pat.regex.MatchString(trimmed) {
				name := pat.regex.FindStringSubmatch(trimmed)[1]
				qualifiedName := filepath.Base(relPath) + ":" + name
				kind := pat.kind

				textForEmbedding := name + " " + kind + " " + trimmed
				embedding := GenerateEmbedding(textForEmbedding)

				sym := &Symbol{
					Name:          name,
					QualifiedName: qualifiedName,
					Kind:          kind,
					File:          relPath,
					Line:          i + 1,
					Signature:     trimmed,
					Embedding:     embedding,
				}

				idx.mu.Lock()
				idx.symbols[qualifiedName] = sym
				idx.mu.Unlock()
				symbolNames = append(symbolNames, qualifiedName)

				// Create symbol chunk
				idx.createChunk(relPath, i+1, i+1, trimmed, "symbol", qualifiedName)
				break
			}
		}
	}

	return symbolNames
}

// createChunk creates and stores a new chunk with embedding.
func (idx *CodebaseIndex) createChunk(file string, startLine, endLine int, content, chunkType, symbolRef string) *Chunk {
	id := fmt.Sprintf("%s:%d:%d:%s", file, startLine, endLine, chunkType)
	if symbolRef != "" {
		id = symbolRef
	}

	// For large file chunks, split into smaller pieces
	if chunkType == "file" && len(content) > 2000 {
		return idx.createFileChunks(file, startLine, content)
	}

	embedding := GenerateEmbedding(content)
	chunk := &Chunk{
		ID:        id,
		File:      file,
		StartLine: startLine,
		EndLine:   endLine,
		Content:   content,
		Embedding: embedding,
		Type:      chunkType,
		SymbolRef: symbolRef,
	}

	idx.mu.Lock()
	idx.chunks[id] = chunk
	idx.embeddings[id] = embedding
	idx.totalDocs++
	// Update doc frequency
	terms := tokenize(content)
	seen := make(map[string]bool)
	for _, t := range terms {
		if !seen[t] {
			idx.docFreq[t]++
			seen[t] = true
		}
	}
	idx.mu.Unlock()

	return chunk
}

// createFileChunks splits a large file into overlapping chunks.
func (idx *CodebaseIndex) createFileChunks(file string, startLine int, content string) *Chunk {
	lines := strings.Split(content, "\n")
	chunkSize := 50
	overlap := 10

	var firstChunk *Chunk
	for i := 0; i < len(lines); i += (chunkSize - overlap) {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		if i >= len(lines) {
			break
		}

		chunkContent := strings.Join(lines[i:end], "\n")
		actualStart := startLine + i
		actualEnd := startLine + end - 1

		id := fmt.Sprintf("%s:%d:%d:file", file, actualStart, actualEnd)
		embedding := GenerateEmbedding(chunkContent)

		chunk := &Chunk{
			ID:        id,
			File:      file,
			StartLine: actualStart,
			EndLine:   actualEnd,
			Content:   chunkContent,
			Embedding: embedding,
			Type:      "file",
		}

		idx.mu.Lock()
		idx.chunks[id] = chunk
		idx.embeddings[id] = embedding
		idx.totalDocs++
		terms := tokenize(chunkContent)
		seen := make(map[string]bool)
		for _, t := range terms {
			if !seen[t] {
				idx.docFreq[t]++
				seen[t] = true
			}
		}
		idx.mu.Unlock()

		if firstChunk == nil {
			firstChunk = chunk
		}

		if end >= len(lines) {
			break
		}
	}

	return firstChunk
}

// shouldIndex returns true if the file should be indexed based on extension.
func (idx *CodebaseIndex) shouldIndex(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supported := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".jsx": true, ".rs": true, ".java": true, ".rb": true, ".c": true,
		".cpp": true, ".h": true, ".hpp": true, ".cs": true, ".php": true,
		".swift": true, ".kt": true, ".scala": true, ".sh": true, ".yaml": true,
		".yml": true, ".json": true, ".toml": true, ".md": true,
	}
	return supported[ext]
}

// GetFileSymbols returns all symbols in a given file.
func (idx *CodebaseIndex) GetFileSymbols(relPath string) []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []*Symbol
	for _, sym := range idx.symbols {
		if sym.File == relPath {
			result = append(result, sym)
		}
	}
	return result
}

// GetFile returns file info for a given relative path.
func (idx *CodebaseIndex) GetFile(relPath string) *FileInfo {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.files[relPath]
}

// AllSymbols returns all indexed symbols.
func (idx *CodebaseIndex) AllSymbols() []*Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]*Symbol, 0, len(idx.symbols))
	for _, sym := range idx.symbols {
		result = append(result, sym)
	}
	return result
}

// AllChunks returns all indexed chunks.
func (idx *CodebaseIndex) AllChunks() []*Chunk {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]*Chunk, 0, len(idx.chunks))
	for _, chunk := range idx.chunks {
		result = append(result, chunk)
	}
	return result
}

// ChunkEmbeddings returns all chunk embeddings as a map (chunk ID -> embedding).
func (idx *CodebaseIndex) ChunkEmbeddings() map[string][]float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make(map[string][]float64, len(idx.chunks))
	for id, chunk := range idx.chunks {
		if len(chunk.Embedding) > 0 {
			result[id] = chunk.Embedding
		}
	}
	return result
}

// GetChunk returns a chunk by ID.
func (idx *CodebaseIndex) GetChunk(id string) (*Chunk, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	chunk, ok := idx.chunks[id]
	return chunk, ok
}

// symbolPattern holds a regex pattern for extracting symbols from non-Go languages.
type symbolPattern struct {
	regex *regexp.Regexp
	kind  string
}

// languageSymbolPatterns returns symbol extraction patterns for non-Go languages.
func languageSymbolPatterns(lang string) []symbolPattern {
	switch lang {
	case "python":
		return []symbolPattern{
			{regexp.MustCompile(`^def\s+(\w+)`), "function"},
			{regexp.MustCompile(`^async\s+def\s+(\w+)`), "function"},
			{regexp.MustCompile(`^class\s+(\w+)`), "class"},
		}
	case "typescript", "javascript":
		return []symbolPattern{
			{regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`), "function"},
			{regexp.MustCompile(`^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`), "interface"},
			{regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)`), "variable"},
			{regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)`), "type"},
			{regexp.MustCompile(`^(?:export\s+)?enum\s+(\w+)`), "type"},
		}
	case "rust":
		return []symbolPattern{
			{regexp.MustCompile(`^(?:pub\s+)?fn\s+(\w+)`), "function"},
			{regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`), "struct"},
			{regexp.MustCompile(`^(?:pub\s+)?enum\s+(\w+)`), "type"},
			{regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`), "interface"},
			{regexp.MustCompile(`^impl\s+(\w+)`), "impl"},
		}
	case "java":
		return []symbolPattern{
			{regexp.MustCompile(`^(?:public|private|protected)?\s*(?:static\s+)?(?:\w+\s+)+(\w+)\s*\(`), "method"},
			{regexp.MustCompile(`^(?:public|private|protected)?\s*(?:abstract\s+)?class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^(?:public|private|protected)?\s*interface\s+(\w+)`), "interface"},
		}
	case "ruby":
		return []symbolPattern{
			{regexp.MustCompile(`^def\s+(\w+)`), "function"},
			{regexp.MustCompile(`^class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^module\s+(\w+)`), "type"},
		}
	default:
		return nil
	}
}

// detectLanguage returns the language name based on file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	default:
		return ""
	}
}

// receiverType extracts the type name from a receiver expression.
func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	default:
		return fmt.Sprintf("%T", t)
	}
}

// formatFuncSignature creates a human-readable function signature.
func formatFuncSignature(decl *ast.FuncDecl) string {
	var sb strings.Builder

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		sb.WriteString("func (")
		recv := decl.Recv.List[0]
		if len(recv.Names) > 0 {
			sb.WriteString(recv.Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(formatExpr(recv.Type))
		sb.WriteString(") ")
	} else {
		sb.WriteString("func ")
	}

	sb.WriteString(decl.Name.Name)
	sb.WriteString(formatFieldList(decl.Type.Params))
	sb.WriteString(formatFieldList(decl.Type.Results))

	return sb.String()
}

// formatTypeSignature creates a human-readable type signature.
func formatTypeSignature(spec *ast.TypeSpec, tok token.Token) string {
	var sb strings.Builder
	sb.WriteString(tok.String())
	sb.WriteString(" ")
	sb.WriteString(spec.Name.Name)
	if spec.TypeParams != nil {
		sb.WriteString("[")
		for i, param := range spec.TypeParams.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			if len(param.Names) > 0 {
				sb.WriteString(param.Names[0].Name)
				sb.WriteString(" ")
			}
			sb.WriteString(formatExpr(param.Type))
		}
		sb.WriteString("]")
	}
	sb.WriteString(" ")
	sb.WriteString(formatExpr(spec.Type))
	return sb.String()
}

// formatFieldList formats a function parameter or result list.
func formatFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return "()"
	}
	var sb strings.Builder
	sb.WriteString("(")
	for i, field := range fl.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		if len(field.Names) > 0 {
			sb.WriteString(field.Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(formatExpr(field.Type))
	}
	sb.WriteString(")")
	return sb.String()
}

// formatExpr creates a string representation of an AST expression.
func formatExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatExpr(t.X)
	case *ast.SelectorExpr:
		return formatExpr(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + formatExpr(t.Elt)
		}
		return "[" + formatExpr(t.Len) + "]" + formatExpr(t.Elt)
	case *ast.MapType:
		return "map[" + formatExpr(t.Key) + "]" + formatExpr(t.Value)
	case *ast.ChanType:
		return "chan " + formatExpr(t.Value)
	case *ast.FuncType:
		return "func" + formatFieldList(t.Params) + formatFieldList(t.Results)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.Ellipsis:
		return "..." + formatExpr(t.Elt)
	case *ast.IndexExpr:
		return formatExpr(t.X) + "[" + formatExpr(t.Index) + "]"
	case *ast.BasicLit:
		return t.Value
	case *ast.UnaryExpr:
		return t.Op.String() + formatExpr(t.X)
	default:
		return fmt.Sprintf("<%T>", t)
	}
}

// GetIndexPath returns the default path for persisting the index.
func GetIndexPath(rootPath string) string {
	return filepath.Join(rootPath, ".smartclaw", "index")
}

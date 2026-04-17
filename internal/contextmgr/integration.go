package contextmgr

import (
	"os"

	"github.com/instructkr/smartclaw/internal/git"
	"github.com/instructkr/smartclaw/internal/index"
	"github.com/instructkr/smartclaw/internal/memory"
)

func NewDefaultContextManager(memMgr *memory.MemoryManager, store workDirProvider, gitCtx *git.Context) *ContextManager {
	workDir := resolveWorkDir(store)

	var providers []ContextProvider

	providers = append(providers, NewMemoryProvider(memMgr))

	if workDir != "" {
		providers = append(providers, NewFileProvider(workDir))
		providers = append(providers, NewSearchProvider(workDir))

		if idx := loadIndexIfAvailable(workDir); idx != nil {
			providers = append(providers, NewSymbolProvider(idx, workDir))
		}
	}

	if gitCtx != nil {
		providers = append(providers, NewGitProvider(gitCtx))
	}

	return NewContextManager(providers...)
}

type workDirProvider interface {
	GetWorkDir() string
}

type dirProvider struct {
	dir string
}

func (d *dirProvider) GetWorkDir() string { return d.dir }

func resolveWorkDir(store workDirProvider) string {
	if store != nil {
		return store.GetWorkDir()
	}
	dir, _ := os.Getwd()
	return dir
}

func loadIndexIfAvailable(workDir string) *index.CodebaseIndex {
	indexPath := index.GetIndexPath(workDir)
	idx := index.NewCodebaseIndex(workDir)
	if err := idx.Load(indexPath); err != nil {
		return nil
	}
	return idx
}

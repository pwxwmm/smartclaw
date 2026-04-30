//go:build !windows && !plan9

package plugins

import (
	"context"
	"fmt"
	"path/filepath"
	"plugin"
	"strings"
)

// NativeStrategy loads Go .so shared libraries via plugin.Open().
type NativeStrategy struct{}

func (s *NativeStrategy) CanLoad(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".so")
}

func (s *NativeStrategy) Name() string { return "native" }

func (s *NativeStrategy) Load(_ context.Context, path string) (PluginInterface, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	p, err := plugin.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("plugin.Open %s: %w", absPath, err)
	}

	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		return nil, fmt.Errorf("lookup NewPlugin in %s: %w", absPath, err)
	}

	newFn, ok := sym.(func() PluginInterface)
	if !ok {
		return nil, fmt.Errorf("NewPlugin in %s is not func() PluginInterface", absPath)
	}

	instance := newFn()
	if instance == nil {
		return nil, fmt.Errorf("NewPlugin() in %s returned nil", absPath)
	}

	return instance, nil
}

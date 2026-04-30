//go:build windows || plan9

package plugins

import (
	"context"
	"fmt"
	"strings"
)

// NativeStrategy is a stub that returns an error on platforms where
// plugin.Open is unavailable (Windows, Plan 9).
type NativeStrategy struct{}

func (s *NativeStrategy) CanLoad(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".so")
}

func (s *NativeStrategy) Name() string { return "native" }

func (s *NativeStrategy) Load(_ context.Context, path string) (PluginInterface, error) {
	return nil, fmt.Errorf("native plugin loading is not supported on this platform: %s", path)
}

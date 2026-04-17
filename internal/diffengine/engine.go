package diffengine

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// DiffEngine applies SEARCH/REPLACE diffs with fuzzy matching, verification, and rollback.
type DiffEngine struct {
	FuzzyMatch       bool
	MaxFuzzyDist     int
	VerifyAfterApply bool
	AutoRollback     bool
}

// Option configures a DiffEngine.
type Option func(*DiffEngine)

func WithFuzzyMatch(enabled bool) Option {
	return func(e *DiffEngine) { e.FuzzyMatch = enabled }
}

func WithMaxFuzzyDist(d int) Option {
	return func(e *DiffEngine) { e.MaxFuzzyDist = d }
}

func WithVerifyAfterApply(enabled bool) Option {
	return func(e *DiffEngine) { e.VerifyAfterApply = enabled }
}

func WithAutoRollback(enabled bool) Option {
	return func(e *DiffEngine) { e.AutoRollback = enabled }
}

// NewDiffEngine creates a DiffEngine with sensible defaults.
func NewDiffEngine(opts ...Option) *DiffEngine {
	e := &DiffEngine{
		FuzzyMatch:       true,
		MaxFuzzyDist:     2,
		VerifyAfterApply: true,
		AutoRollback:     true,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Apply applies a single DiffBlock to a file with the full pipeline:
// read → apply → verify → (rollback on failure).
func (e *DiffEngine) Apply(ctx context.Context, filePath string, block DiffBlock) (*ApplyResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}
	originalStr := string(originalContent)

	result, err := ApplyDiffWithOptions(filePath, block, e.FuzzyMatch)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return result, nil
	}

	if e.VerifyAfterApply {
		verifyResult := VerifyFile(filePath)

		if !verifyResult.Valid && e.AutoRollback {
			if rollbackErr := Rollback(filePath, originalStr); rollbackErr != nil {
				result.Error = fmt.Errorf("verification failed: %s; rollback also failed: %w",
					strings.Join(verifyResult.Issues, "; "), rollbackErr)
			} else {
				result.Success = false
				result.Error = fmt.Errorf("verification failed (rolled back): %s",
					strings.Join(verifyResult.Issues, "; "))
			}
			return result, nil
		}

		if !verifyResult.Valid {
			result.Error = fmt.Errorf("verification warning: %s",
				strings.Join(verifyResult.Issues, "; "))
		}
	}

	return result, nil
}

// ApplyFromOutput parses LLM output for SEARCH/REPLACE blocks and applies them all.
func (e *DiffEngine) ApplyFromOutput(ctx context.Context, output string) ([]ApplyResult, error) {
	blocks, err := ParseDiffBlocks(output)
	if err != nil {
		return nil, fmt.Errorf("parse diff blocks: %w", err)
	}

	return e.ApplyBlocks(ctx, blocks)
}

// ApplyBlocks applies a slice of DiffBlocks, returning all results.
func (e *DiffEngine) ApplyBlocks(ctx context.Context, blocks []DiffBlock) ([]ApplyResult, error) {
	fileBlocks := make(map[string][]DiffBlock)
	var fileOrder []string

	for _, b := range blocks {
		fp := b.FilePath
		if fp == "" {
			return nil, fmt.Errorf("diff block has no file path")
		}
		if _, exists := fileBlocks[fp]; !exists {
			fileOrder = append(fileOrder, fp)
		}
		fileBlocks[fp] = append(fileBlocks[fp], b)
	}

	var allResults []ApplyResult

	for _, fp := range fileOrder {
		select {
		case <-ctx.Done():
			return allResults, ctx.Err()
		default:
		}

		bs := fileBlocks[fp]

		originalContent, err := os.ReadFile(fp)
		if err != nil {
			for range bs {
				allResults = append(allResults, ApplyResult{
					FilePath: fp,
					Success:  false,
					Error:    fmt.Errorf("read file %s: %w", fp, err),
				})
			}
			continue
		}
		originalStr := string(originalContent)

		results, err := ApplyDiffs(bs)
		if err != nil {
			return allResults, err
		}

		if e.VerifyAfterApply {
			verifyResult := VerifyFile(fp)
			if !verifyResult.Valid {
				if e.AutoRollback {
					if rollbackErr := Rollback(fp, originalStr); rollbackErr != nil {
						for i := range results {
							results[i].Error = fmt.Errorf("verification failed: %s; rollback also failed: %w",
								strings.Join(verifyResult.Issues, "; "), rollbackErr)
						}
					} else {
						for i := range results {
							results[i].Success = false
							results[i].Error = fmt.Errorf("verification failed (rolled back): %s",
								strings.Join(verifyResult.Issues, "; "))
						}
					}
				} else {
					for i := range results {
						if results[i].Success {
							results[i].Error = fmt.Errorf("verification warning: %s",
								strings.Join(verifyResult.Issues, "; "))
						}
					}
				}
			}
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// DryRunFromOutput parses LLM output and previews all blocks without writing.
func (e *DiffEngine) DryRunFromOutput(ctx context.Context, output string) ([]ApplyResult, error) {
	blocks, err := ParseDiffBlocks(output)
	if err != nil {
		return nil, fmt.Errorf("parse diff blocks: %w", err)
	}

	var results []ApplyResult
	for _, b := range blocks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		if b.FilePath == "" {
			results = append(results, ApplyResult{
				Success: false,
				Error:   fmt.Errorf("diff block has no file path"),
			})
			continue
		}

		result, err := DryRun(b.FilePath, b)
		if err != nil {
			results = append(results, ApplyResult{
				FilePath: b.FilePath,
				Success:  false,
				Error:    err,
			})
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

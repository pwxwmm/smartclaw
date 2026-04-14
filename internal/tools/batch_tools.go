package tools

import (
	"context"
	"sync"
)

type BatchTool struct{ BaseTool }

func (t *BatchTool) Name() string		{ return "batch" }
func (t *BatchTool) Description() string	{ return "Execute multiple operations in batch" }

func (t *BatchTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"operations": map[string]any{
				"type":	"array",
				"items": map[string]any{
					"type":	"object",
					"properties": map[string]any{
						"tool":		map[string]any{"type": "string"},
						"input":	map[string]any{"type": "object"},
					},
				},
			},
			"parallel":	map[string]any{"type": "boolean"},
		},
		"required":	[]string{"operations"},
	}
}

func (t *BatchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	operations, ok := input["operations"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "operations must be an array"}
	}

	parallel, _ := input["parallel"].(bool)

	results := make([]map[string]any, 0, len(operations))

	if parallel {
		results = t.executeParallel(ctx, operations)
	} else {
		results = t.executeSequential(ctx, operations)
	}

	return map[string]any{
		"results":	results,
		"count":	len(results),
	}, nil
}

func (t *BatchTool) executeSequential(ctx context.Context, operations []any) []map[string]any {
	results := make([]map[string]any, 0, len(operations))

	for i, op := range operations {
		opMap, ok := op.(map[string]any)
		if !ok {
			results = append(results, map[string]any{
				"index":	i,
				"error":	"invalid operation format",
				"status":	"failed",
			})
			continue
		}

		toolName, _ := opMap["tool"].(string)
		toolInput, _ := opMap["input"].(map[string]any)

		result, err := defaultRegistry.Execute(ctx, toolName, toolInput)
		if err != nil {
			results = append(results, map[string]any{
				"index":	i,
				"tool":		toolName,
				"error":	err.Error(),
				"status":	"failed",
			})
			continue
		}

		results = append(results, map[string]any{
			"index":	i,
			"tool":		toolName,
			"result":	result,
			"status":	"success",
		})
	}

	return results
}

func (t *BatchTool) executeParallel(ctx context.Context, operations []any) []map[string]any {
	results := make([]map[string]any, len(operations))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation any) {
			defer wg.Done()

			opMap, ok := operation.(map[string]any)
			if !ok {
				mu.Lock()
				results[index] = map[string]any{
					"index":	index,
					"error":	"invalid operation format",
					"status":	"failed",
				}
				mu.Unlock()
				return
			}

			toolName, _ := opMap["tool"].(string)
			toolInput, _ := opMap["input"].(map[string]any)

			result, err := defaultRegistry.Execute(ctx, toolName, toolInput)

			mu.Lock()
			if err != nil {
				results[index] = map[string]any{
					"index":	index,
					"tool":		toolName,
					"error":	err.Error(),
					"status":	"failed",
				}
			} else {
				results[index] = map[string]any{
					"index":	index,
					"tool":		toolName,
					"result":	result,
					"status":	"success",
				}
			}
			mu.Unlock()
		}(i, op)
	}

	wg.Wait()
	return results
}

type ParallelTool struct{ BaseTool }

func (t *ParallelTool) Name() string		{ return "parallel" }
func (t *ParallelTool) Description() string	{ return "Execute multiple tools in parallel" }

func (t *ParallelTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"tasks": map[string]any{
				"type":	"array",
				"items": map[string]any{
					"type":	"object",
					"properties": map[string]any{
						"id":		map[string]any{"type": "string"},
						"tool":		map[string]any{"type": "string"},
						"input":	map[string]any{"type": "object"},
					},
				},
			},
		},
		"required":	[]string{"tasks"},
	}
}

func (t *ParallelTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	tasks, ok := input["tasks"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "tasks must be an array"}
	}

	results := make(map[string]any)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		taskMap, ok := task.(map[string]any)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(tm map[string]any) {
			defer wg.Done()

			id, _ := tm["id"].(string)
			toolName, _ := tm["tool"].(string)
			toolInput, _ := tm["input"].(map[string]any)

			result, err := defaultRegistry.Execute(ctx, toolName, toolInput)

			mu.Lock()
			if err != nil {
				results[id] = map[string]any{
					"error":	err.Error(),
					"status":	"failed",
				}
			} else {
				results[id] = map[string]any{
					"result":	result,
					"status":	"success",
				}
			}
			mu.Unlock()
		}(taskMap)
	}

	wg.Wait()

	return map[string]any{
		"results":	results,
		"count":	len(results),
	}, nil
}

type PipelineTool struct{ BaseTool }

func (t *PipelineTool) Name() string		{ return "pipeline" }
func (t *PipelineTool) Description() string	{ return "Execute tools in a pipeline (output -> input)" }

func (t *PipelineTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"steps": map[string]any{
				"type":	"array",
				"items": map[string]any{
					"type":	"object",
					"properties": map[string]any{
						"tool":		map[string]any{"type": "string"},
						"input":	map[string]any{"type": "object"},
					},
				},
			},
		},
		"required":	[]string{"steps"},
	}
}

func (t *PipelineTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	steps, ok := input["steps"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "steps must be an array"}
	}

	var lastResult any

	for i, step := range steps {
		stepMap, ok := step.(map[string]any)
		if !ok {
			return nil, &Error{Code: "INVALID_STEP", Message: "step must be an object"}
		}

		toolName, _ := stepMap["tool"].(string)
		toolInput, _ := stepMap["input"].(map[string]any)

		if i > 0 && toolInput != nil && lastResult != nil {
			if lastMap, ok := lastResult.(map[string]any); ok {
				for k, v := range lastMap {
					if _, exists := toolInput[k]; !exists {
						toolInput[k] = v
					}
				}
			}
		}

		result, err := defaultRegistry.Execute(ctx, toolName, toolInput)
		if err != nil {
			return map[string]any{
				"step":		i,
				"tool":		toolName,
				"error":	err.Error(),
				"status":	"failed",
				"partial":	lastResult,
			}, nil
		}

		lastResult = result
	}

	return map[string]any{
		"result":	lastResult,
		"steps":	len(steps),
		"status":	"completed",
	}, nil
}

package tools

import (
	"context"
	"sync"
)

type BatchTool struct{}

func (t *BatchTool) Name() string        { return "batch" }
func (t *BatchTool) Description() string { return "Execute multiple operations in batch" }

func (t *BatchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operations": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tool":  map[string]interface{}{"type": "string"},
						"input": map[string]interface{}{"type": "object"},
					},
				},
			},
			"parallel": map[string]interface{}{"type": "boolean"},
		},
		"required": []string{"operations"},
	}
}

func (t *BatchTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operations, ok := input["operations"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "operations must be an array"}
	}

	parallel, _ := input["parallel"].(bool)

	results := make([]map[string]interface{}, 0, len(operations))

	if parallel {
		results = t.executeParallel(ctx, operations)
	} else {
		results = t.executeSequential(ctx, operations)
	}

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, nil
}

func (t *BatchTool) executeSequential(ctx context.Context, operations []interface{}) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(operations))

	for i, op := range operations {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			results = append(results, map[string]interface{}{
				"index":  i,
				"error":  "invalid operation format",
				"status": "failed",
			})
			continue
		}

		toolName, _ := opMap["tool"].(string)
		toolInput, _ := opMap["input"].(map[string]interface{})

		result, err := defaultRegistry.Execute(ctx, toolName, toolInput)
		if err != nil {
			results = append(results, map[string]interface{}{
				"index":  i,
				"tool":   toolName,
				"error":  err.Error(),
				"status": "failed",
			})
			continue
		}

		results = append(results, map[string]interface{}{
			"index":  i,
			"tool":   toolName,
			"result": result,
			"status": "success",
		})
	}

	return results
}

func (t *BatchTool) executeParallel(ctx context.Context, operations []interface{}) []map[string]interface{} {
	results := make([]map[string]interface{}, len(operations))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation interface{}) {
			defer wg.Done()

			opMap, ok := operation.(map[string]interface{})
			if !ok {
				mu.Lock()
				results[index] = map[string]interface{}{
					"index":  index,
					"error":  "invalid operation format",
					"status": "failed",
				}
				mu.Unlock()
				return
			}

			toolName, _ := opMap["tool"].(string)
			toolInput, _ := opMap["input"].(map[string]interface{})

			result, err := defaultRegistry.Execute(ctx, toolName, toolInput)

			mu.Lock()
			if err != nil {
				results[index] = map[string]interface{}{
					"index":  index,
					"tool":   toolName,
					"error":  err.Error(),
					"status": "failed",
				}
			} else {
				results[index] = map[string]interface{}{
					"index":  index,
					"tool":   toolName,
					"result": result,
					"status": "success",
				}
			}
			mu.Unlock()
		}(i, op)
	}

	wg.Wait()
	return results
}

type ParallelTool struct{}

func (t *ParallelTool) Name() string        { return "parallel" }
func (t *ParallelTool) Description() string { return "Execute multiple tools in parallel" }

func (t *ParallelTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tasks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":    map[string]interface{}{"type": "string"},
						"tool":  map[string]interface{}{"type": "string"},
						"input": map[string]interface{}{"type": "object"},
					},
				},
			},
		},
		"required": []string{"tasks"},
	}
}

func (t *ParallelTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	tasks, ok := input["tasks"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "tasks must be an array"}
	}

	results := make(map[string]interface{})
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		wg.Add(1)
		go func(tm map[string]interface{}) {
			defer wg.Done()

			id, _ := tm["id"].(string)
			toolName, _ := tm["tool"].(string)
			toolInput, _ := tm["input"].(map[string]interface{})

			result, err := defaultRegistry.Execute(ctx, toolName, toolInput)

			mu.Lock()
			if err != nil {
				results[id] = map[string]interface{}{
					"error":  err.Error(),
					"status": "failed",
				}
			} else {
				results[id] = map[string]interface{}{
					"result": result,
					"status": "success",
				}
			}
			mu.Unlock()
		}(taskMap)
	}

	wg.Wait()

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, nil
}

type PipelineTool struct{}

func (t *PipelineTool) Name() string        { return "pipeline" }
func (t *PipelineTool) Description() string { return "Execute tools in a pipeline (output -> input)" }

func (t *PipelineTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"steps": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tool":  map[string]interface{}{"type": "string"},
						"input": map[string]interface{}{"type": "object"},
					},
				},
			},
		},
		"required": []string{"steps"},
	}
}

func (t *PipelineTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	steps, ok := input["steps"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "steps must be an array"}
	}

	var lastResult interface{}

	for i, step := range steps {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			return nil, &Error{Code: "INVALID_STEP", Message: "step must be an object"}
		}

		toolName, _ := stepMap["tool"].(string)
		toolInput, _ := stepMap["input"].(map[string]interface{})

		if i > 0 && toolInput != nil && lastResult != nil {
			if lastMap, ok := lastResult.(map[string]interface{}); ok {
				for k, v := range lastMap {
					if _, exists := toolInput[k]; !exists {
						toolInput[k] = v
					}
				}
			}
		}

		result, err := defaultRegistry.Execute(ctx, toolName, toolInput)
		if err != nil {
			return map[string]interface{}{
				"step":    i,
				"tool":    toolName,
				"error":   err.Error(),
				"status":  "failed",
				"partial": lastResult,
			}, nil
		}

		lastResult = result
	}

	return map[string]interface{}{
		"result": lastResult,
		"steps":  len(steps),
		"status": "completed",
	}, nil
}

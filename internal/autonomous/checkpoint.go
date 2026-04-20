package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type Checkpoint struct {
	ID        string
	State     LoopState
	CreatedAt time.Time
}

func checkpointPath(dir, id string) string {
	return filepath.Join(dir, fmt.Sprintf("autonomous-%s.json", id))
}

// SaveCheckpoint writes the current loop state to dir/autonomous-{id}.json.
func SaveCheckpoint(dir string, loop *Loop) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}

	state := loop.GetState()
	cp := Checkpoint{
		ID:        fmt.Sprintf("%d", state.StartTime.UnixMilli()),
		State:     state,
		CreatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath(dir, cp.ID), data, 0o644); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	return nil
}

// LoadCheckpoint reads a checkpoint by ID from the given directory.
func LoadCheckpoint(dir string, id string) (*LoopState, error) {
	data, err := os.ReadFile(checkpointPath(dir, id))
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}
	return &cp.State, nil
}

// ListCheckpoints returns all checkpoints in the directory, sorted by creation time (newest first).
func ListCheckpoints(dir string) ([]Checkpoint, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint dir: %w", err)
	}

	re := regexp.MustCompile(`^autonomous-(.+)\.json$`)
	var checkpoints []Checkpoint

	for _, entry := range entries {
		matches := re.FindStringSubmatch(entry.Name())
		if len(matches) != 2 {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		checkpoints = append(checkpoints, cp)
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.After(checkpoints[j].CreatedAt)
	})
	return checkpoints, nil
}

// DeleteCheckpoint removes a checkpoint file by ID.
func DeleteCheckpoint(dir string, id string) error {
	if err := os.Remove(checkpointPath(dir, id)); err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

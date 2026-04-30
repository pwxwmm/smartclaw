package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ExportPipeline struct {
	compressor *TrajectoryCompressor
	outputDir  string
	format     string
}

type PipelineResult struct {
	TotalInput        int       `json:"total_input"`
	TotalOutput       int       `json:"total_output"`
	DuplicatesRemoved int       `json:"duplicates_removed"`
	FilesWritten      int       `json:"files_written"`
	AvgCompressionRatio float64 `json:"avg_compression_ratio"`
	Duration          time.Duration `json:"duration"`
	Errors            []string `json:"errors,omitempty"`
}

func NewExportPipeline(compressor *TrajectoryCompressor, outputDir string, format string) *ExportPipeline {
	return &ExportPipeline{
		compressor: compressor,
		outputDir:  outputDir,
		format:     format,
	}
}

func (ep *ExportPipeline) ProcessAndExport(ctx context.Context, trajectories []*Trajectory) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{
		TotalInput: len(trajectories),
		Errors:     []string{},
	}

	if err := os.MkdirAll(ep.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("rl: export pipeline: mkdir: %w", err)
	}

	deduped := ep.deduplicate(trajectories)
	result.DuplicatesRemoved = len(trajectories) - len(deduped)

	var totalRatio float64
	compressedCount := 0

	for _, traj := range deduped {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(start)
			return result, ctx.Err()
		default:
		}

		compressed, stats, err := ep.compressor.CompressWithStats(traj)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("compress %s: %v", traj.ID, err))
			continue
		}

		if stats != nil && stats.OriginalBytes > 0 {
			totalRatio += stats.CompressionRatio
			compressedCount++
		}

		if err := ep.exportOne(compressed); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("export %s: %v", traj.ID, err))
			continue
		}

		result.FilesWritten++
	}

	result.TotalOutput = len(deduped)
	if compressedCount > 0 {
		result.AvgCompressionRatio = totalRatio / float64(compressedCount)
	}
	result.Duration = time.Since(start)

	if err := ep.writeManifest(result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest: %v", err))
	}

	return result, nil
}

func (ep *ExportPipeline) deduplicate(trajectories []*Trajectory) []*Trajectory {
	seen := make(map[string]bool, len(trajectories))
	deduped := make([]*Trajectory, 0, len(trajectories))
	for _, traj := range trajectories {
		if seen[traj.ID] {
			continue
		}
		seen[traj.ID] = true
		deduped = append(deduped, traj)
	}
	return deduped
}

func (ep *ExportPipeline) exportOne(traj *Trajectory) error {
	var data any
	var err error
	var ext string

	switch ep.format {
	case "sharegpt":
		data, err = ep.compressor.ExportShareGPT(traj)
		ext = "json"
	case "openai":
		data, err = ep.compressor.ExportOpenAIFineTuning(traj)
		ext = "json"
	case "jsonl":
		ext = "jsonl"
	default:
		return fmt.Errorf("rl: export pipeline: unsupported format %q", ep.format)
	}

	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.%s", traj.ID, ext)
	path := filepath.Join(ep.outputDir, filename)

	if ep.format == "jsonl" {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("rl: export pipeline: create: %w", err)
		}
		defer f.Close()
		return ep.compressor.ExportJSONL([]*Trajectory{traj}, f)
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("rl: export pipeline: marshal: %w", err)
	}

	return os.WriteFile(path, bytes, 0644)
}

func (ep *ExportPipeline) writeManifest(result *PipelineResult) error {
	manifestPath := filepath.Join(ep.outputDir, "_manifest.json")
	bytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("rl: export pipeline: marshal manifest: %w", err)
	}
	return os.WriteFile(manifestPath, bytes, 0644)
}

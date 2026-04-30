package rl

import "time"

// CompressionStats tracks statistics about a single trajectory compression operation.
type CompressionStats struct {
	OriginalBytes       int
	CompressedBytes     int
	CompressionRatio    float64 // compressed/original
	StepsRemoved        int
	StepsMerged         int // merged by semantic dedup
	Method              string // "none", "truncate", "remove_zero_reward", "aggressive", "iterative", "semantic_dedup"
	Timestamp           time.Time
	OriginalStepCount   int
	CompressedStepCount int
}

// BatchCompressionStats tracks statistics about batch compression of multiple trajectories.
type BatchCompressionStats struct {
	TotalOriginal       int
	TotalCompressed     int
	DuplicatesRemoved   int // exact + near duplicates
	AvgCompressionRatio float64
}

package convention

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VerifyResult holds the outcome of citation verification.
type VerifyResult struct {
	ConventionID string
	Valid        bool
	Reason       string
}

// VerifyConvention checks if the citation still matches the current codebase.
func (s *Store) VerifyConvention(_ context.Context, conv *Convention, projectRoot string) *VerifyResult {
	if conv.Citation == nil {
		return &VerifyResult{
			ConventionID: conv.ID,
			Valid:        true,
			Reason:       "citation matches",
		}
	}

	fullPath := filepath.Join(projectRoot, conv.Citation.File)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &VerifyResult{
				ConventionID: conv.ID,
				Valid:        false,
				Reason:       "file not found",
			}
		}
		return &VerifyResult{
			ConventionID: conv.ID,
			Valid:        false,
			Reason:       fmt.Sprintf("read error: %v", err),
		}
	}

	lines := splitLines(string(data))
	start := conv.Citation.StartLine
	end := conv.Citation.EndLine

	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end || start > len(lines) {
		return &VerifyResult{
			ConventionID: conv.ID,
			Valid:        false,
			Reason:       "content mismatch",
		}
	}

	cited := lines[start-1 : end]
	hash := ComputeContentHash(cited)

	if hash == conv.Citation.ContentHash {
		return &VerifyResult{
			ConventionID: conv.ID,
			Valid:        true,
			Reason:       "citation matches",
		}
	}
	return &VerifyResult{
		ConventionID: conv.ID,
		Valid:        false,
		Reason:       "content mismatch",
	}
}

// VerifyAll checks all conventions and returns results.
func (s *Store) VerifyAll(ctx context.Context, projectRoot string) []*VerifyResult {
	s.mu.RLock()
	convs := make([]*Convention, 0, len(s.convs))
	for _, c := range s.convs {
		convs = append(convs, c)
	}
	s.mu.RUnlock()

	var results []*VerifyResult
	for _, conv := range convs {
		result := s.VerifyConvention(ctx, conv, projectRoot)
		results = append(results, result)
	}
	return results
}

// PurgeInvalid removes conventions whose citations no longer match.
func (s *Store) PurgeInvalid(ctx context.Context, projectRoot string) (int, error) {
	results := s.VerifyAll(ctx, projectRoot)
	purged := 0
	for _, r := range results {
		if !r.Valid {
			if err := s.Delete(r.ConventionID); err != nil {
				return purged, fmt.Errorf("failed to purge %s: %w", r.ConventionID, err)
			}
			purged++
		}
	}
	return purged, nil
}

// Share makes a convention visible to another agent.
func (s *Store) Share(convID string, targetAgentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.convs[convID]; !ok {
		return fmt.Errorf("convention %q not found", convID)
	}

	sh, ok := s.shares[convID]
	if !ok {
		sh = &sharing{ConventionID: convID}
		s.shares[convID] = sh
	}

	for _, a := range sh.TargetAgents {
		if a == targetAgentID {
			return nil
		}
	}
	sh.TargetAgents = append(sh.TargetAgents, targetAgentID)
	return s.saveShares()
}

// SharedWith returns agents this convention is shared with.
func (s *Store) SharedWith(convID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sh, ok := s.shares[convID]
	if !ok {
		return nil
	}
	result := make([]string, len(sh.TargetAgents))
	copy(result, sh.TargetAgents)
	return result
}

// ReceiveShared imports a convention from another agent.
func (s *Store) ReceiveShared(conv *Convention) error {
	conv.Source = "agent"
	return s.Add(conv)
}

// splitLines splits text into individual lines preserving content.
func splitLines(text string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

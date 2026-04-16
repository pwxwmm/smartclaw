package github

import (
	"context"
	"os/exec"
	"testing"
)

func ghAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func TestNewClient(t *testing.T) {
	c := NewClient("/tmp/testrepo")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.repoDir != "/tmp/testrepo" {
		t.Fatalf("expected repoDir /tmp/testrepo, got %s", c.repoDir)
	}
}

func TestCheckInstalled(t *testing.T) {
	if !ghAvailable() {
		t.Skip("gh CLI not available, skipping CheckInstalled test")
	}

	c := NewClient(".")
	if err := c.CheckInstalled(context.Background()); err != nil {
		t.Fatalf("CheckInstalled failed: %v", err)
	}
}

func TestCheckInstalled_NoGH(t *testing.T) {
	if ghAvailable() {
		t.Skip("gh CLI is available, cannot test missing-gh scenario")
	}

	c := NewClient(".")
	err := c.CheckInstalled(context.Background())
	if err == nil {
		t.Fatal("expected error when gh is not installed")
	}
}

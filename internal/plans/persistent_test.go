package plans

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) (*PlanStore, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewPlanStore(dir)
	if err != nil {
		t.Fatalf("NewPlanStore: %v", err)
	}
	return store, dir
}

func TestNewPlanStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPlanStore(dir)
	if err != nil {
		t.Fatalf("NewPlanStore: %v", err)
	}

	plansDir := filepath.Join(dir, ".smartclaw", "plans")
	info, err := os.Stat(plansDir)
	if err != nil {
		t.Fatalf("plans dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("plans path is not a directory")
	}

	if store.plansDir != plansDir {
		t.Fatalf("plansDir = %q, want %q", store.plansDir, plansDir)
	}
}

func TestNewPlanStore_Idempotent(t *testing.T) {
	dir := t.TempDir()
	_, err := NewPlanStore(dir)
	if err != nil {
		t.Fatalf("first NewPlanStore: %v", err)
	}
	_, err = NewPlanStore(dir)
	if err != nil {
		t.Fatalf("second NewPlanStore: %v", err)
	}
}

func TestCreateAndGet(t *testing.T) {
	store, _ := newTestStore(t)

	p, err := store.Create("Test Plan", "1. Step one\n2. Step two", "/workspace")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if p.Title != "Test Plan" {
		t.Fatalf("Title = %q, want %q", p.Title, "Test Plan")
	}
	if p.Content != "1. Step one\n2. Step two" {
		t.Fatalf("Content = %q, want %q", p.Content, "1. Step one\n2. Step two")
	}
	if p.Status != StatusDraft {
		t.Fatalf("Status = %q, want %q", p.Status, StatusDraft)
	}
	if p.WorkspaceDir != "/workspace" {
		t.Fatalf("WorkspaceDir = %q, want %q", p.WorkspaceDir, "/workspace")
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero timestamps")
	}

	got, err := store.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != p.ID {
		t.Fatalf("got.ID = %q, want %q", got.ID, p.ID)
	}
	if got.Title != p.Title {
		t.Fatalf("got.Title = %q, want %q", got.Title, p.Title)
	}
	if got.Content != p.Content {
		t.Fatalf("got.Content = %q, want %q", got.Content, p.Content)
	}
	if got.Status != p.Status {
		t.Fatalf("got.Status = %q, want %q", got.Status, p.Status)
	}
	if got.WorkspaceDir != p.WorkspaceDir {
		t.Fatalf("got.WorkspaceDir = %q, want %q", got.WorkspaceDir, p.WorkspaceDir)
	}
}

func TestGet_NotFound(t *testing.T) {
	store, _ := newTestStore(t)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}
}

func TestList(t *testing.T) {
	store, _ := newTestStore(t)

	_, err := store.Create("First", "Content A", "/ws")
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	_, err = store.Create("Second", "Content B", "/ws")
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	plans, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("len(plans) = %d, want 2", len(plans))
	}

	if plans[0].CreatedAt.Before(plans[1].CreatedAt) {
		t.Fatal("expected plans sorted newest first")
	}
}

func TestList_Empty(t *testing.T) {
	store, _ := newTestStore(t)

	plans, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("len(plans) = %d, want 0", len(plans))
	}
}

func TestListByStatus(t *testing.T) {
	store, _ := newTestStore(t)

	p1, _ := store.Create("Draft Plan", "draft content", "/ws")
	p2, _ := store.Create("Will Be Active", "active content", "/ws")

	if err := store.SetStatus(p2.ID, StatusActive); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	drafts, err := store.ListByStatus(StatusDraft)
	if err != nil {
		t.Fatalf("ListByStatus draft: %v", err)
	}
	if len(drafts) != 1 || drafts[0].ID != p1.ID {
		t.Fatalf("drafts = %v, want plan %s", drafts, p1.ID)
	}

	actives, err := store.ListByStatus(StatusActive)
	if err != nil {
		t.Fatalf("ListByStatus active: %v", err)
	}
	if len(actives) != 1 || actives[0].ID != p2.ID {
		t.Fatalf("actives = %v, want plan %s", actives, p2.ID)
	}

	abandoned, err := store.ListByStatus(StatusAbandoned)
	if err != nil {
		t.Fatalf("ListByStatus abandoned: %v", err)
	}
	if len(abandoned) != 0 {
		t.Fatalf("len(abandoned) = %d, want 0", len(abandoned))
	}
}

func TestUpdate(t *testing.T) {
	store, _ := newTestStore(t)

	p, _ := store.Create("Original", "original content", "/ws")
	originalUpdatedAt := p.UpdatedAt

	if err := store.Update(p.ID, "updated content"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Get(p.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}

	if got.Content != "updated content" {
		t.Fatalf("Content = %q, want %q", got.Content, "updated content")
	}
	if got.Title != "Original" {
		t.Fatalf("Title = %q, want %q", got.Title, "Original")
	}
	if !got.UpdatedAt.After(originalUpdatedAt) {
		t.Fatal("expected UpdatedAt to be after original")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.Update("nonexistent", "content")
	if err == nil {
		t.Fatal("expected error for updating nonexistent plan")
	}
}

func TestSetStatus(t *testing.T) {
	store, _ := newTestStore(t)

	p, _ := store.Create("Plan", "content", "/ws")
	if p.Status != StatusDraft {
		t.Fatalf("initial Status = %q, want %q", p.Status, StatusDraft)
	}

	if err := store.SetStatus(p.ID, StatusActive); err != nil {
		t.Fatalf("SetStatus active: %v", err)
	}

	got, _ := store.Get(p.ID)
	if got.Status != StatusActive {
		t.Fatalf("Status = %q, want %q", got.Status, StatusActive)
	}

	if err := store.SetStatus(p.ID, StatusCompleted); err != nil {
		t.Fatalf("SetStatus completed: %v", err)
	}

	got, _ = store.Get(p.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompleted)
	}
}

func TestSetStatus_NotFound(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.SetStatus("nonexistent", StatusActive)
	if err == nil {
		t.Fatal("expected error for setting status on nonexistent plan")
	}
}

func TestDelete(t *testing.T) {
	store, _ := newTestStore(t)

	p, _ := store.Create("ToDelete", "content", "/ws")

	if err := store.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(p.ID)
	if err == nil {
		t.Fatal("expected error getting deleted plan")
	}
}

func TestDelete_NotFound(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent plan")
	}
}

func TestSearch(t *testing.T) {
	store, _ := newTestStore(t)

	store.Create("Refactor Auth Module", "1. Extract middleware\n2. Add token refresh", "/ws")
	store.Create("Fix CSS Bug", "Fix the overlapping header issue", "/ws")
	store.Create("Auth Tests", "Write tests for auth module", "/ws")

	results, err := store.Search("auth")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	for _, r := range results {
		if r.Title != "Refactor Auth Module" && r.Title != "Auth Tests" {
			t.Fatalf("unexpected result: %q", r.Title)
		}
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	store, _ := newTestStore(t)

	store.Create("UPPERCASE PLAN", "lowercase content", "/ws")

	results, err := store.Search("uppercase")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	results, err = store.Search("LOWERCASE")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestSearch_NoMatch(t *testing.T) {
	store, _ := newTestStore(t)

	store.Create("Some Plan", "Some content", "/ws")

	results, err := store.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

func TestPlanFileFormat_Roundtrip(t *testing.T) {
	store, dir := newTestStore(t)

	p, err := store.Create("Format Test", "## Plan\n\n1. Step one\n2. Step two", "/project")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	path := filepath.Join(dir, ".smartclaw", "plans", p.ID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if content[:4] != "---\n" {
		t.Fatal("file must start with YAML frontmatter delimiter")
	}

	got, err := store.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != p.ID {
		t.Fatalf("ID roundtrip: got %q, want %q", got.ID, p.ID)
	}
	if got.Title != p.Title {
		t.Fatalf("Title roundtrip: got %q, want %q", got.Title, p.Title)
	}
	if got.Content != p.Content {
		t.Fatalf("Content roundtrip: got %q, want %q", got.Content, p.Content)
	}
	if got.Status != p.Status {
		t.Fatalf("Status roundtrip: got %q, want %q", got.Status, p.Status)
	}
	if got.WorkspaceDir != p.WorkspaceDir {
		t.Fatalf("WorkspaceDir roundtrip: got %q, want %q", got.WorkspaceDir, p.WorkspaceDir)
	}
}

func TestPlan_WithTags(t *testing.T) {
	store, _ := newTestStore(t)

	p, _ := store.Create("Tagged Plan", "content", "/ws")
	p.Tags = []string{"refactor", "auth"}
	if err := store.save(p); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(got.Tags) != 2 || got.Tags[0] != "refactor" || got.Tags[1] != "auth" {
		t.Fatalf("Tags = %v, want [refactor auth]", got.Tags)
	}
}

func TestPlan_IDIsTimestampBased(t *testing.T) {
	store, _ := newTestStore(t)

	p, _ := store.Create("ID Test", "content", "/ws")

	if len(p.ID) < len("20060102-150405-000000") {
		t.Fatalf("ID = %q, expected timestamp format YYYYMMDD-HHMMSS", p.ID)
	}
}

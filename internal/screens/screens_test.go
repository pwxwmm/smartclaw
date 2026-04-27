package screens

import (
	"strings"
	"testing"
)

func TestNewScreen(t *testing.T) {
	t.Parallel()

	s := NewScreen("Test Title")
	if s.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", s.Title, "Test Title")
	}
	if len(s.Lines) != 0 {
		t.Errorf("Lines should be empty, got %d", len(s.Lines))
	}
	if s.CursorY != 0 {
		t.Errorf("CursorY = %d, want 0", s.CursorY)
	}
	if s.CursorX != 0 {
		t.Errorf("CursorX = %d, want 0", s.CursorX)
	}
}

func TestSetTitle(t *testing.T) {
	t.Parallel()

	s := NewScreen("Old Title")
	s.SetTitle("New Title")
	if s.Title != "New Title" {
		t.Errorf("Title = %q, want %q", s.Title, "New Title")
	}
}

func TestAddLine(t *testing.T) {
	t.Parallel()

	s := NewScreen("Test")
	s.AddLine("Line 1")
	s.AddLine("Line 2")

	if len(s.Lines) != 2 {
		t.Fatalf("Lines length = %d, want 2", len(s.Lines))
	}
	if s.Lines[0] != "Line 1" {
		t.Errorf("Lines[0] = %q, want %q", s.Lines[0], "Line 1")
	}
	if s.Lines[1] != "Line 2" {
		t.Errorf("Lines[1] = %q, want %q", s.Lines[1], "Line 2")
	}
}

func TestAddLines(t *testing.T) {
	t.Parallel()

	s := NewScreen("Test")
	s.AddLines([]string{"Line A", "Line B", "Line C"})

	if len(s.Lines) != 3 {
		t.Fatalf("Lines length = %d, want 3", len(s.Lines))
	}
}

func TestRender(t *testing.T) {
	s := NewScreen("Test")
	s.AddLine("Hello World")

	rendered := s.Render()
	if !strings.Contains(rendered, "Test") {
		t.Error("Render() should contain title")
	}
	if !strings.Contains(rendered, "Hello World") {
		t.Error("Render() should contain content line")
	}
	if !strings.Contains(rendered, "┌") {
		t.Error("Render() should contain header border")
	}
	if !strings.Contains(rendered, "└") {
		t.Error("Render() should contain footer border")
	}
}

func TestRender_EmptyScreen(t *testing.T) {
	s := NewScreen("Empty")

	rendered := s.Render()
	if !strings.Contains(rendered, "Empty") {
		t.Error("Render() of empty screen should contain title")
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	s := NewScreen("Test")
	clearSeq := s.Clear()
	if clearSeq != "\033[2J\033[H" {
		t.Errorf("Clear() = %q, want ANSI clear sequence", clearSeq)
	}
}

func TestNewMenuScreen(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	if m.Title != "Menu" {
		t.Errorf("Title = %q, want %q", m.Title, "Menu")
	}
	if len(m.Items) != 0 {
		t.Errorf("Items should be empty, got %d", len(m.Items))
	}
	if m.Selected != 0 {
		t.Errorf("Selected = %d, want 0", m.Selected)
	}
}

func TestMenuScreen_AddItem(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("Option 1")
	m.AddItem("Option 2")

	if len(m.Items) != 2 {
		t.Fatalf("Items length = %d, want 2", len(m.Items))
	}
	if m.Items[0] != "Option 1" {
		t.Errorf("Items[0] = %q, want %q", m.Items[0], "Option 1")
	}
}

func TestMenuScreen_MoveUp(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.AddItem("B")
	m.Selected = 1

	m.MoveUp()
	if m.Selected != 0 {
		t.Errorf("Selected after MoveUp = %d, want 0", m.Selected)
	}
}

func TestMenuScreen_MoveUp_AtTop(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.AddItem("B")
	m.Selected = 0

	m.MoveUp()
	if m.Selected != 0 {
		t.Errorf("Selected after MoveUp at top = %d, want 0 (should not go negative)", m.Selected)
	}
}

func TestMenuScreen_MoveDown(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.AddItem("B")
	m.AddItem("C")

	m.MoveDown()
	if m.Selected != 1 {
		t.Errorf("Selected after MoveDown = %d, want 1", m.Selected)
	}
}

func TestMenuScreen_MoveDown_AtBottom(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.AddItem("B")
	m.Selected = 1

	m.MoveDown()
	if m.Selected != 1 {
		t.Errorf("Selected after MoveDown at bottom = %d, want 1 (should not exceed last item)", m.Selected)
	}
}

func TestMenuScreen_Select(t *testing.T) {
	t.Parallel()

	selected := -1
	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.AddItem("B")
	m.Selected = 1
	m.OnSelect = func(idx int) {
		selected = idx
	}

	m.Select()
	if selected != 1 {
		t.Errorf("OnSelect called with %d, want 1", selected)
	}
}

func TestMenuScreen_Select_NilCallback(t *testing.T) {
	t.Parallel()

	m := NewMenuScreen("Menu")
	m.AddItem("A")
	m.OnSelect = nil

	m.Select()
}

func TestMenuScreen_Render(t *testing.T) {
	m := NewMenuScreen("Menu")
	m.AddItem("Option A")
	m.AddItem("Option B")

	rendered := m.Render()
	if !strings.Contains(rendered, "Menu") {
		t.Error("MenuScreen.Render() should contain title")
	}
	if !strings.Contains(rendered, "Option A") {
		t.Error("MenuScreen.Render() should contain first item")
	}
	if !strings.Contains(rendered, "Option B") {
		t.Error("MenuScreen.Render() should contain second item")
	}
}

func TestNewEditorScreen(t *testing.T) {
	t.Parallel()

	e := NewEditorScreen("Editor")
	if e.Title != "Editor" {
		t.Errorf("Title = %q, want %q", e.Title, "Editor")
	}
	if e.Content != "" {
		t.Errorf("Content should be empty, got %q", e.Content)
	}
	if e.Modified {
		t.Error("Modified should be false")
	}
}

func TestEditorScreen_SetContent(t *testing.T) {
	t.Parallel()

	e := NewEditorScreen("Editor")
	e.SetContent("package main\n\nfunc main() {}")

	if e.Content != "package main\n\nfunc main() {}" {
		t.Errorf("Content = %q, want set content", e.Content)
	}
	if !e.Modified {
		t.Error("Modified should be true after SetContent")
	}
}

func TestEditorScreen_GetContent(t *testing.T) {
	t.Parallel()

	e := NewEditorScreen("Editor")
	e.SetContent("test content")

	if got := e.GetContent(); got != "test content" {
		t.Errorf("GetContent() = %q, want %q", got, "test content")
	}
}

func TestEditorScreen_MarkSaved(t *testing.T) {
	t.Parallel()

	e := NewEditorScreen("Editor")
	e.SetContent("content")
	e.MarkSaved()

	if e.Modified {
		t.Error("Modified should be false after MarkSaved")
	}
}

func TestEditorScreen_Render(t *testing.T) {
	e := NewEditorScreen("Editor")
	e.SetContent("Line 1\nLine 2\nLine 3")

	rendered := e.Render()
	if !strings.Contains(rendered, "Editor") {
		t.Error("EditorScreen.Render() should contain title")
	}
	if !strings.Contains(rendered, "Line 1") {
		t.Error("EditorScreen.Render() should contain first line")
	}
	if !strings.Contains(rendered, "Line 3") {
		t.Error("EditorScreen.Render() should contain third line")
	}
}

func TestEditorScreen_Render_EmptyContent(t *testing.T) {
	e := NewEditorScreen("Editor")
	rendered := e.Render()
	if !strings.Contains(rendered, "Editor") {
		t.Error("EditorScreen.Render() with empty content should contain title")
	}
}

func TestScreen_ImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ Screen = &BaseScreen{}
	var _ Screen = &MenuScreen{}
	var _ Screen = &EditorScreen{}
}

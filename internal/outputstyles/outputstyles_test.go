package outputstyles

import "testing"

func TestGetStyle_Default(t *testing.T) {
	s := GetStyle("default")
	if s.Name != "default" {
		t.Errorf("Name = %q, want %q", s.Name, "default")
	}
}

func TestGetStyle_Minimal(t *testing.T) {
	s := GetStyle("minimal")
	if s.Name != "minimal" {
		t.Errorf("Name = %q, want %q", s.Name, "minimal")
	}
}

func TestGetStyle_Unicode(t *testing.T) {
	s := GetStyle("unicode")
	if s.Name != "unicode" {
		t.Errorf("Name = %q, want %q", s.Name, "unicode")
	}
	if !s.UseEmoji {
		t.Error("Unicode style should have UseEmoji = true")
	}
}

func TestGetStyle_Ascii(t *testing.T) {
	s := GetStyle("ascii")
	if s.Name != "ascii" {
		t.Errorf("Name = %q, want %q", s.Name, "ascii")
	}
}

func TestGetStyle_Unknown(t *testing.T) {
	s := GetStyle("nonexistent")
	if s.Name != "default" {
		t.Errorf("Unknown style should fallback to default, got %q", s.Name)
	}
}

func TestListStyles(t *testing.T) {
	names := ListStyles()
	if len(names) != 4 {
		t.Errorf("ListStyles returned %d styles, want 4", len(names))
	}

	expected := map[string]bool{
		"default": false,
		"minimal": false,
		"unicode": false,
		"ascii":   false,
	}
	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("Style %q not found in ListStyles", name)
		}
	}
}

func TestFormatMessage(t *testing.T) {
	style := OutputStyle{
		Name:   "test",
		Prefix: "[",
		Suffix: "]",
	}
	result := FormatMessage(style, "info", "hello world")
	if result != "[hello world]" {
		t.Errorf("FormatMessage = %q, want %q", result, "[hello world]")
	}
}

func TestFormatMessage_NoPrefixSuffix(t *testing.T) {
	style := GetStyle("default")
	result := FormatMessage(style, "info", "hello")
	if result != "hello" {
		t.Errorf("FormatMessage with no prefix/suffix = %q, want %q", result, "hello")
	}
}

func TestGetEmoji_Success(t *testing.T) {
	e := GetEmoji("success")
	if e != "✓" {
		t.Errorf("GetEmoji(success) = %q, want %q", e, "✓")
	}
}

func TestGetEmoji_Error(t *testing.T) {
	e := GetEmoji("error")
	if e != "✗" {
		t.Errorf("GetEmoji(error) = %q, want %q", e, "✗")
	}
}

func TestGetEmoji_Warning(t *testing.T) {
	e := GetEmoji("warning")
	if e != "⚠" {
		t.Errorf("GetEmoji(warning) = %q, want %q", e, "⚠")
	}
}

func TestGetEmoji_Info(t *testing.T) {
	e := GetEmoji("info")
	if e != "ℹ" {
		t.Errorf("GetEmoji(info) = %q, want %q", e, "ℹ")
	}
}

func TestGetEmoji_Unknown(t *testing.T) {
	e := GetEmoji("nonexistent")
	if e != "" {
		t.Errorf("GetEmoji(unknown) = %q, want empty string", e)
	}
}

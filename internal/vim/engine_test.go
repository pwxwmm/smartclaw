package vim

import "testing"

func TestNewVimEngine(t *testing.T) {
	v := NewVimEngine()
	if v.GetMode() != ModeNormal {
		t.Errorf("initial mode = %v, want %v", v.GetMode(), ModeNormal)
	}
	c := v.GetCursor()
	if c.Line != 0 || c.Column != 0 {
		t.Errorf("initial cursor = (%d,%d), want (0,0)", c.Line, c.Column)
	}
}

func TestNormalMode_Transitions(t *testing.T) {
	tests := []struct {
		key  string
		want Mode
	}{
		{"i", ModeInsert},
		{"v", ModeVisual},
		{":", ModeCommand},
		{"r", ModeReplace},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			v := NewVimEngine()
			v.ProcessKey(tt.key)
			if v.GetMode() != tt.want {
				t.Errorf("after %q: mode = %v, want %v", tt.key, v.GetMode(), tt.want)
			}
		})
	}
}

func TestNormalMode_Navigation(t *testing.T) {
	t.Run("h moves left", func(t *testing.T) {
		v := NewVimEngine()
		v.SetCursor(0, 5)
		v.ProcessKey("h")
		if c := v.GetCursor(); c.Column != 4 {
			t.Errorf("after h: column = %d, want 4", c.Column)
		}
	})

	t.Run("h does not go below 0", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("h")
		if c := v.GetCursor(); c.Column != 0 {
			t.Errorf("after h at 0: column = %d, want 0", c.Column)
		}
	})

	t.Run("j moves down", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("j")
		if c := v.GetCursor(); c.Line != 1 {
			t.Errorf("after j: line = %d, want 1", c.Line)
		}
	})

	t.Run("k moves up", func(t *testing.T) {
		v := NewVimEngine()
		v.SetCursor(3, 0)
		v.ProcessKey("k")
		if c := v.GetCursor(); c.Line != 2 {
			t.Errorf("after k: line = %d, want 2", c.Line)
		}
	})

	t.Run("k does not go below 0", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("k")
		if c := v.GetCursor(); c.Line != 0 {
			t.Errorf("after k at 0: line = %d, want 0", c.Line)
		}
	})

	t.Run("l moves right", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("l")
		if c := v.GetCursor(); c.Column != 1 {
			t.Errorf("after l: column = %d, want 1", c.Column)
		}
	})

	t.Run("0 moves to column 0", func(t *testing.T) {
		v := NewVimEngine()
		v.SetCursor(0, 10)
		v.ProcessKey("0")
		if c := v.GetCursor(); c.Column != 0 {
			t.Errorf("after 0: column = %d, want 0", c.Column)
		}
	})

	t.Run("$ moves to column 9999", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("$")
		if c := v.GetCursor(); c.Column != 9999 {
			t.Errorf("after $: column = %d, want 9999", c.Column)
		}
	})
}

func TestInsertMode(t *testing.T) {
	v := NewVimEngine()
	v.ProcessKey("i")
	if v.GetMode() != ModeInsert {
		t.Fatalf("mode = %v, want %v", v.GetMode(), ModeInsert)
	}
	v.ProcessKey("<Esc>")
	if v.GetMode() != ModeNormal {
		t.Errorf("after Esc: mode = %v, want %v", v.GetMode(), ModeNormal)
	}
}

func TestVisualMode(t *testing.T) {
	t.Run("Esc returns to Normal", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("v")
		v.ProcessKey("<Esc>")
		if v.GetMode() != ModeNormal {
			t.Errorf("mode = %v, want %v", v.GetMode(), ModeNormal)
		}
	})

	t.Run("y yanks and returns to Normal", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("v")
		v.ProcessKey("y")
		if v.GetMode() != ModeNormal {
			t.Errorf("mode = %v, want %v", v.GetMode(), ModeNormal)
		}
		if s := v.GetState(); s.Register != "yank" {
			t.Errorf("Register = %q, want %q", s.Register, "yank")
		}
	})

	t.Run("d deletes and returns to Normal", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey("v")
		v.ProcessKey("d")
		if v.GetMode() != ModeNormal {
			t.Errorf("mode = %v, want %v", v.GetMode(), ModeNormal)
		}
		if s := v.GetState(); s.Register != "delete" {
			t.Errorf("Register = %q, want %q", s.Register, "delete")
		}
	})
}

func TestCommandMode(t *testing.T) {
	t.Run("Esc returns to Normal", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey(":")
		v.ProcessKey("<Esc>")
		if v.GetMode() != ModeNormal {
			t.Errorf("mode = %v, want %v", v.GetMode(), ModeNormal)
		}
	})

	t.Run("Enter executes and returns to Normal", func(t *testing.T) {
		v := NewVimEngine()
		v.ProcessKey(":")
		v.ProcessKey("<Enter>")
		if v.GetMode() != ModeNormal {
			t.Errorf("mode = %v, want %v", v.GetMode(), ModeNormal)
		}
	})
}

func TestReplaceMode(t *testing.T) {
	v := NewVimEngine()
	v.ProcessKey("r")
	if v.GetMode() != ModeReplace {
		t.Fatalf("mode = %v, want %v", v.GetMode(), ModeReplace)
	}
	v.ProcessKey("<Esc>")
	if v.GetMode() != ModeNormal {
		t.Errorf("after Esc: mode = %v, want %v", v.GetMode(), ModeNormal)
	}
}

func TestMacro(t *testing.T) {
	v := NewVimEngine()
	v.StartRecording()
	v.ProcessKey("l")
	v.ProcessKey("l")
	v.ProcessKey("j")
	v.StopRecording()

	before := v.GetCursor()
	v.PlayMacro()
	after := v.GetCursor()

	if after.Column != before.Column+2 {
		t.Errorf("after PlayMacro: column = %d, want %d", after.Column, before.Column+2)
	}
	if after.Line != before.Line+1 {
		t.Errorf("after PlayMacro: line = %d, want %d", after.Line, before.Line+1)
	}
}

func TestMarks(t *testing.T) {
	v := NewVimEngine()
	v.SetMark("a", 42)
	pos, ok := v.GetMark("a")
	if !ok {
		t.Fatal("GetMark(a) not found")
	}
	if pos != 42 {
		t.Errorf("GetMark(a) = %d, want 42", pos)
	}

	_, ok = v.GetMark("z")
	if ok {
		t.Error("GetMark(z) should not exist")
	}
}

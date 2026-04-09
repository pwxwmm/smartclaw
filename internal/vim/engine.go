package vim

import (
	"strings"
	"sync"
)

type Mode string

const (
	ModeNormal  Mode = "normal"
	ModeInsert  Mode = "insert"
	ModeVisual  Mode = "visual"
	ModeCommand Mode = "command"
	ModeReplace Mode = "replace"
)

type Motion struct {
	Type     string
	Count    int
	Modifier string
}

type Operator struct {
	Name   string
	Motion *Motion
	Count  int
}

type TextObject struct {
	Type     string
	Modifier string
}

type VimState struct {
	Mode      Mode
	Count     int
	Register  string
	Search    string
	LastYank  string
	LastPaste string
	Mark      map[string]int
	Cursor    Cursor
}

type Cursor struct {
	Line   int
	Column int
}

type VimEngine struct {
	state     VimState
	keymaps   map[Mode]map[string]func()
	recording bool
	macro     []string
	mu        sync.Mutex
}

func NewVimEngine() *VimEngine {
	return &VimEngine{
		state: VimState{
			Mode:     ModeNormal,
			Count:    0,
			Register: "",
			Search:   "",
			Mark:     make(map[string]int),
			Cursor:   Cursor{Line: 0, Column: 0},
		},
		keymaps:   make(map[Mode]map[string]func()),
		recording: false,
		macro:     make([]string, 0),
	}
}

func (v *VimEngine) GetMode() Mode {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state.Mode
}

func (v *VimEngine) SetMode(mode Mode) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.state.Mode = mode
}

func (v *VimEngine) GetCursor() Cursor {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state.Cursor
}

func (v *VimEngine) SetCursor(line, column int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.state.Cursor.Line = line
	v.state.Cursor.Column = column
}

func (v *VimEngine) ProcessKey(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.recording {
		v.macro = append(v.macro, key)
	}

	switch v.state.Mode {
	case ModeNormal:
		v.processNormalMode(key)
	case ModeInsert:
		v.processInsertMode(key)
	case ModeVisual:
		v.processVisualMode(key)
	case ModeCommand:
		v.processCommandMode(key)
	case ModeReplace:
		v.processReplaceMode(key)
	}
}

func (v *VimEngine) processNormalMode(key string) {
	switch key {
	case "i":
		v.state.Mode = ModeInsert
	case "v":
		v.state.Mode = ModeVisual
	case ":":
		v.state.Mode = ModeCommand
	case "r":
		v.state.Mode = ModeReplace
	case "h":
		if v.state.Cursor.Column > 0 {
			v.state.Cursor.Column--
		}
	case "j":
		v.state.Cursor.Line++
	case "k":
		if v.state.Cursor.Line > 0 {
			v.state.Cursor.Line--
		}
	case "l":
		v.state.Cursor.Column++
	case "0":
		v.state.Cursor.Column = 0
	case "$":
		v.state.Cursor.Column = 9999
	case "w":
		v.state.Cursor.Column += 5
	case "b":
		if v.state.Cursor.Column > 5 {
			v.state.Cursor.Column -= 5
		}
	case "G":
		v.state.Cursor.Line = 9999
	case "g":
		v.state.Cursor.Line = 0
	case "d":
		v.state.Register = "delete"
	case "y":
		v.state.Register = "yank"
	case "p":
		v.state.Register = "paste"
	case "u":
		v.state.Register = "undo"
	case "/":
		v.state.Mode = ModeCommand
		v.state.Search = ""
	}
}

func (v *VimEngine) processInsertMode(key string) {
	switch key {
	case "<Esc>":
		v.state.Mode = ModeNormal
	}
}

func (v *VimEngine) processVisualMode(key string) {
	switch key {
	case "<Esc>":
		v.state.Mode = ModeNormal
	case "y":
		v.state.Register = "yank"
		v.state.Mode = ModeNormal
	case "d":
		v.state.Register = "delete"
		v.state.Mode = ModeNormal
	}
}

func (v *VimEngine) processCommandMode(key string) {
	switch key {
	case "<Esc>":
		v.state.Mode = ModeNormal
	case "<Enter>":
		v.executeCommand(v.state.Search)
		v.state.Mode = ModeNormal
	case "<Backspace>":
		if len(v.state.Search) > 0 {
			v.state.Search = v.state.Search[:len(v.state.Search)-1]
		}
	default:
		if len(key) == 1 {
			v.state.Search += key
		}
	}
}

func (v *VimEngine) processReplaceMode(key string) {
	if key == "<Esc>" {
		v.state.Mode = ModeNormal
	}
}

func (v *VimEngine) executeCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)

	if strings.HasPrefix(cmd, "/") {
		v.state.Search = cmd[1:]
		return
	}

	if strings.HasPrefix(cmd, "w") {
		return
	}

	if strings.HasPrefix(cmd, "q") {
		return
	}

	if cmd == "set number" {
		return
	}
}

func (v *VimEngine) StartRecording() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.recording = true
	v.macro = v.macro[:0]
}

func (v *VimEngine) StopRecording() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.recording = false
}

func (v *VimEngine) PlayMacro() {
	v.mu.Lock()
	macro := make([]string, len(v.macro))
	copy(macro, v.macro)
	v.mu.Unlock()

	for _, key := range macro {
		v.ProcessKey(key)
	}
}

func (v *VimEngine) GetState() VimState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state
}

func (v *VimEngine) SetMark(name string, position int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.state.Mark[name] = position
}

func (v *VimEngine) GetMark(name string) (int, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	pos, exists := v.state.Mark[name]
	return pos, exists
}

//go:build desktop
// +build desktop

package desktop

import "fmt"

type MenuItem struct {
	Label    string `json:"label"`
	Type     string `json:"type"`     // "normal", "separator", "checkbox"
	Disabled bool   `json:"disabled"`
	Callback string `json:"callback"` // method name on bound Go struct
}

type TrayManager struct {
	app       *App
	menuItems []MenuItem
	visible   bool
}

func NewTrayManager(app *App) *TrayManager {
	t := &TrayManager{app: app, visible: true}
	t.menuItems = t.defaultMenu()
	return t
}

func (t *TrayManager) defaultMenu() []MenuItem {
	return []MenuItem{
		{Label: "Show/Hide SmartClaw", Type: "normal", Callback: "ToggleWindow"},
		{Type: "separator"},
		{Label: "New Chat", Type: "normal", Callback: "NewChat"},
		{Label: "Toggle Agent", Type: "normal", Callback: "ToggleAgent"},
		{Type: "separator"},
		{Label: "Quit", Type: "normal", Callback: "Quit"},
	}
}

func (t *TrayManager) GetMenu() []MenuItem {
	return t.menuItems
}

// OnClick toggles the main window visibility (tray icon click on macOS, double-click on Windows/Linux).
func (t *TrayManager) OnClick() {
	t.visible = !t.visible
	if t.visible {
		t.app.ShowWindow()
	} else {
		t.app.HideWindow()
	}
}

func (t *TrayManager) ToggleWindow() { t.OnClick() }

func (t *TrayManager) NewChat() {
	if t.app != nil {
		t.app.Emit("tray:new-chat", nil)
	}
}

func (t *TrayManager) ToggleAgent() {
	if t.app != nil {
		t.app.Emit("tray:toggle-agent", nil)
	}
}

func (t *TrayManager) Quit() {
	if t.app != nil {
		t.app.Quit()
	}
}

func (t *TrayManager) SetMenu(items []MenuItem) { t.menuItems = items }

func (t *TrayManager) AddMenuItem(item MenuItem) { t.menuItems = append(t.menuItems, item) }

// HotkeyManager stores global hotkey→action mappings. Actual OS registration is done by Wails.
type HotkeyManager struct {
	bindings map[string]string // accelerator → action (e.g. "Ctrl+Shift+S" → "show/hide")
}

func NewHotkeyManager() *HotkeyManager {
	h := &HotkeyManager{bindings: make(map[string]string)}
	h.bindings["Ctrl+Shift+S"] = "show/hide"
	h.bindings["Ctrl+Shift+N"] = "new-chat"
	return h
}

// Register adds or replaces a hotkey binding. Key format: Modifier+Key (Wails accelerator syntax).
// Modifiers: Ctrl, Shift, Alt, Option, Super, Cmd. Keys: A-Z, 0-9, F1-F12, Space, Enter, Tab.
func (h *HotkeyManager) Register(key, action string) {
	h.bindings[key] = action
}

func (h *HotkeyManager) Unregister(key string) { delete(h.bindings, key) }

func (h *HotkeyManager) GetBindings() map[string]string {
	out := make(map[string]string, len(h.bindings))
	for k, v := range h.bindings {
		out[k] = v
	}
	return out
}

func (h *HotkeyManager) ActionForKey(key string) string { return h.bindings[key] }

// HandleHotkey dispatches a pressed hotkey to the appropriate App method.
func (h *HotkeyManager) HandleHotkey(app *App, key string) error {
	action, ok := h.bindings[key]
	if !ok {
		return fmt.Errorf("hotkey: no binding for %q", key)
	}

	switch action {
	case "show/hide":
		if tray := app.Tray(); tray != nil {
			tray.ToggleWindow()
		}
	case "new-chat":
		app.Emit("hotkey:new-chat", nil)
	default:
		app.Emit("hotkey:"+action, nil)
	}
	return nil
}

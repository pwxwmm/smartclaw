package keybindings

import (
	"fmt"
	"os"
)

type KeyBinding struct {
	Key      string
	Command  string
	Modifier string
}

type KeyBindings struct {
	bindings map[string]KeyBinding
}

var defaultBindings = map[string]KeyBinding{
	"ctrl+c": {Key: "c", Command: "copy", Modifier: "ctrl"},
	"ctrl+v": {Key: "v", Command: "paste", Modifier: "ctrl"},
	"ctrl+z": {Key: "z", Command: "undo", Modifier: "ctrl"},
	"ctrl+s": {Key: "s", Command: "save", Modifier: "ctrl"},
	"ctrl+p": {Key: "p", Command: "search", Modifier: "ctrl"},
	"ctrl+r": {Key: "r", Command: "refresh", Modifier: "ctrl"},
	"ctrl+/": {Key: "/", Command: "comment", Modifier: "ctrl"},
	"escape": {Key: "escape", Command: "cancel", Modifier: ""},
	"enter":  {Key: "enter", Command: "submit", Modifier: ""},
	"tab":    {Key: "tab", Command: "complete", Modifier: ""},
}

func NewKeyBindings() *KeyBindings {
	return &KeyBindings{
		bindings: make(map[string]KeyBinding),
	}
}

func (k *KeyBindings) Register(binding KeyBinding) {
	key := fmt.Sprintf("%s+%s", binding.Modifier, binding.Key)
	k.bindings[key] = binding
}

func (k *KeyBindings) Get(key string) (KeyBinding, bool) {
	b, ok := k.bindings[key]
	if !ok {
		b, ok = defaultBindings[key]
	}
	return b, ok
}

func (k *KeyBindings) List() []KeyBinding {
	result := make([]KeyBinding, 0, len(k.bindings))
	for _, b := range k.bindings {
		result = append(result, b)
	}
	return result
}

func LoadFromFile(path string) (*KeyBindings, error) {
	_, err := os.ReadFile(path)
	if err != nil {
		return NewKeyBindings(), nil
	}
	return NewKeyBindings(), nil
}

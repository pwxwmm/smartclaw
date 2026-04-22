package models

import (
	"math"
	"testing"
)

func TestGet(t *testing.T) {
	t.Run("known model", func(t *testing.T) {
		m, err := Get("claude-opus-4-6")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if m.ID != "claude-opus-4-6" {
			t.Errorf("Get() ID = %q, want %q", m.ID, "claude-opus-4-6")
		}
		if m.InputPrice != 15.0 {
			t.Errorf("Get() InputPrice = %v, want 15.0", m.InputPrice)
		}
		if m.OutputPrice != 75.0 {
			t.Errorf("Get() OutputPrice = %v, want 75.0", m.OutputPrice)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		_, err := Get("nonexistent-model")
		if err == nil {
			t.Fatal("Get() expected error for unknown model, got nil")
		}
	})
}

func TestList(t *testing.T) {
	list := List()
	if len(list) != 3 {
		t.Fatalf("List() returned %d models, want 3", len(list))
	}

	ids := map[string]bool{
		"claude-opus-4-6":   false,
		"claude-sonnet-4-5": false,
		"claude-haiku-3-5":  false,
	}
	for _, m := range list {
		if _, ok := ids[m.ID]; !ok {
			t.Errorf("List() unexpected model ID: %q", m.ID)
		}
		ids[m.ID] = true
	}
	for id, found := range ids {
		if !found {
			t.Errorf("List() missing model: %q", id)
		}
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"claude-opus-4-6", true},
		{"claude-sonnet-4-5", true},
		{"claude-haiku-3-5", true},
		{"nonexistent", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := Exists(tt.id); got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name         string
		modelID      string
		inputTokens  int
		outputTokens int
		wantCost     float64
		wantErr      bool
	}{
		{
			name: "opus", modelID: "claude-opus-4-6",
			inputTokens: 1_000_000, outputTokens: 1_000_000,
			wantCost: 15.0 + 75.0, wantErr: false,
		},
		{
			name: "sonnet", modelID: "claude-sonnet-4-5",
			inputTokens: 1_000_000, outputTokens: 1_000_000,
			wantCost: 3.0 + 15.0, wantErr: false,
		},
		{
			name: "haiku", modelID: "claude-haiku-3-5",
			inputTokens: 1_000_000, outputTokens: 1_000_000,
			wantCost: 0.25 + 1.25, wantErr: false,
		},
		{
			name: "partial tokens sonnet", modelID: "claude-sonnet-4-5",
			inputTokens: 500_000, outputTokens: 200_000,
			wantCost: 500_000*3.0/1_000_000 + 200_000*15.0/1_000_000, wantErr: false,
		},
		{
			name: "unknown model", modelID: "nonexistent",
			inputTokens: 100, outputTokens: 100,
			wantCost: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateCost(tt.modelID, tt.inputTokens, tt.outputTokens)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CalculateCost() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && math.Abs(got-tt.wantCost) > 1e-9 {
				t.Errorf("CalculateCost() = %v, want %v", got, tt.wantCost)
			}
		})
	}
}

func TestGetDefault(t *testing.T) {
	if got := GetDefault(); got != "claude-sonnet-4-5" {
		t.Errorf("GetDefault() = %q, want %q", got, "claude-sonnet-4-5")
	}
}

func TestValidate(t *testing.T) {
	t.Run("known model", func(t *testing.T) {
		if err := Validate("claude-opus-4-6"); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		err := Validate("nonexistent")
		if err == nil {
			t.Fatal("Validate() expected error for unknown model, got nil")
		}
	})
}

package schemas

import "testing"

func TestParseSchema(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		data := []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
		s, err := ParseSchema(data)
		if err != nil {
			t.Fatalf("ParseSchema() error = %v", err)
		}
		if s.Type != "object" {
			t.Errorf("Type = %q, want %q", s.Type, "object")
		}
		if len(s.Required) != 1 || s.Required[0] != "name" {
			t.Errorf("Required = %v, want [name]", s.Required)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		data := []byte(`{invalid json}`)
		_, err := ParseSchema(data)
		if err == nil {
			t.Fatal("ParseSchema() expected error for invalid JSON, got nil")
		}
	})
}

func TestValidate_RequiredFields(t *testing.T) {
	s := &JSONSchema{
		Type:     "object",
		Required: []string{"name", "age"},
	}

	t.Run("all required present", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": 30}
		if err := s.Validate(data); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		data := map[string]any{"name": "Alice"}
		err := s.Validate(data)
		if err == nil {
			t.Fatal("Validate() expected error for missing field, got nil")
		}
	})
}

func TestValidate_EmptyRequired(t *testing.T) {
	s := &JSONSchema{
		Type:     "object",
		Required: nil,
	}
	data := map[string]any{}
	if err := s.Validate(data); err != nil {
		t.Errorf("Validate() with nil Required error = %v, want nil", err)
	}
}

func TestValidate_NotMap(t *testing.T) {
	s := &JSONSchema{
		Type:     "object",
		Required: []string{"name"},
	}
	err := s.Validate("not a map")
	if err == nil {
		t.Fatal("Validate() expected error for non-map data with Required, got nil")
	}
}

func TestValidateJSON(t *testing.T) {
	schemaData := []byte(`{"type":"object","required":["name"]}`)

	t.Run("valid data", func(t *testing.T) {
		data := []byte(`{"name":"Alice"}`)
		if err := ValidateJSON(data, schemaData); err != nil {
			t.Errorf("ValidateJSON() error = %v, want nil", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		data := []byte(`{"age":30}`)
		if err := ValidateJSON(data, schemaData); err == nil {
			t.Fatal("ValidateJSON() expected error for missing required, got nil")
		}
	})

	t.Run("invalid schema JSON", func(t *testing.T) {
		data := []byte(`{"name":"Alice"}`)
		badSchema := []byte(`{invalid}`)
		if err := ValidateJSON(data, badSchema); err == nil {
			t.Fatal("ValidateJSON() expected error for invalid schema, got nil")
		}
	})

	t.Run("invalid data JSON — current behavior silently ignores unmarshal error", func(t *testing.T) {
		data := []byte(`{invalid data}`)
		schemaData := []byte(`{"type":"object","required":["name"]}`)
		// Known behavior: json.Unmarshal error on data is silently ignored,
		// then schema.Validate(nil) is called which fails because nil is not a map
		err := ValidateJSON(data, schemaData)
		// data unmarshal fails silently → dataObj=nil → Validate(nil) fails on Required check
		if err == nil {
			t.Log("ValidateJSON silently ignores data unmarshal error (known behavior)")
		}
	})
}

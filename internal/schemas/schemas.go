package schemas

import (
	"encoding/json"
	"fmt"
)

type Schema interface {
	Validate(data interface{}) error
}

type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]*JSONSchema `json:"properties"`
	Required   []string               `json:"required"`
}

func (s *JSONSchema) Validate(data interface{}) error {
	if s.Required != nil {
		m, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("data must be a map")
		}
		for _, key := range s.Required {
			if _, ok := m[key]; !ok {
				return fmt.Errorf("missing required field: %s", key)
			}
		}
	}
	return nil
}

func ParseSchema(data []byte) (*JSONSchema, error) {
	var s JSONSchema
	err := json.Unmarshal(data, &s)
	return &s, err
}

func ValidateJSON(data []byte, schemaData []byte) error {
	schema, err := ParseSchema(schemaData)
	if err != nil {
		return err
	}
	var dataObj interface{}
	json.Unmarshal(data, &dataObj)
	return schema.Validate(dataObj)
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &result, nil
}

func SaveJSON[T any](path string, data *T) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, bytes, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

package playbook

import (
	"context"
	"path/filepath"
	"testing"
)

func TestManagerSaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name:        "test-pb",
		Description: "a test playbook",
		Version:     "1.0",
		Steps: []Step{
			{ID: "step1", Name: "First", Action: "run_command", Command: "echo hello"},
		},
	}

	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := m.Load("test-pb")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Name != "test-pb" {
		t.Errorf("Name = %q, want %q", loaded.Name, "test-pb")
	}
	if loaded.Description != "a test playbook" {
		t.Errorf("Description = %q, want %q", loaded.Description, "a test playbook")
	}
	if loaded.Version != "1.0" {
		t.Errorf("Version = %q, want %q", loaded.Version, "1.0")
	}
	if len(loaded.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(loaded.Steps))
	}
	if loaded.Steps[0].ID != "step1" {
		t.Errorf("Steps[0].ID = %q, want %q", loaded.Steps[0].ID, "step1")
	}
}

func TestManagerList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb1 := &Playbook{Name: "pb-one", Steps: []Step{{ID: "s1", Action: "run_command", Command: "true"}}}
	pb2 := &Playbook{Name: "pb-two", Steps: []Step{{ID: "s2", Action: "run_command", Command: "true"}}}

	if err := m.Save(pb1); err != nil {
		t.Fatalf("Save(pb1) error: %v", err)
	}
	if err := m.Save(pb2); err != nil {
		t.Fatalf("Save(pb2) error: %v", err)
	}

	list, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len(List) = %d, want 2", len(list))
	}
}

func TestManagerDelete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{Name: "to-delete", Steps: []Step{{ID: "s1", Action: "run_command", Command: "true"}}}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := m.Delete("to-delete"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	list, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(List) after delete = %d, want 0", len(list))
	}
}

func TestManagerValidateValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "valid-pb",
		Steps: []Step{
			{ID: "s1", Name: "Step 1", Action: "run_command", Command: "echo hi"},
			{ID: "s2", Name: "Step 2", Action: "create_file", Find: "/tmp/test", Template: "content"},
		},
	}
	if err := m.Validate(pb); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestManagerValidateNoName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name:  "",
		Steps: []Step{{ID: "s1", Action: "run_command"}},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestManagerValidateNoSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{Name: "no-steps"}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for no steps")
	}
}

func TestManagerValidateInvalidAction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "bad-action",
		Steps: []Step{
			{ID: "s1", Action: "fly_to_moon"},
		},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestManagerValidateDuplicateStepID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "dup-step",
		Steps: []Step{
			{ID: "s1", Action: "run_command", Command: "true"},
			{ID: "s1", Action: "run_command", Command: "true"},
		},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for duplicate step ID")
	}
}

func TestManagerValidateInvalidNextStep(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "bad-next",
		Steps: []Step{
			{ID: "s1", Action: "run_command", Command: "true", NextStep: "nonexistent"},
		},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for unknown next_step")
	}
}

func TestManagerExecuteRunCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "run-cmd",
		Steps: []Step{
			{ID: "s1", Name: "Echo", Action: "run_command", Command: "echo hello"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	execCtx, err := m.Execute(context.Background(), "run-cmd", nil, nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if execCtx.Status != "completed" {
		t.Errorf("Status = %q, want %q", execCtx.Status, "completed")
	}
	sr, ok := execCtx.StepResults["s1"]
	if !ok {
		t.Fatal("missing step result for s1")
	}
	if !sr.Success {
		t.Errorf("s1 Success = false, Output = %q, Error = %q", sr.Output, sr.Error)
	}
}

func TestManagerExecuteCreateFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)
	outFile := filepath.Join(t.TempDir(), "output.txt")

	pb := &Playbook{
		Name: "create-f",
		Steps: []Step{
			{ID: "s1", Name: "Create", Action: "create_file", Find: outFile, Template: "hello world"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	execCtx, err := m.Execute(context.Background(), "create-f", nil, nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if execCtx.Status != "completed" {
		t.Errorf("Status = %q, want %q", execCtx.Status, "completed")
	}
	sr := execCtx.StepResults["s1"]
	if !sr.Success {
		t.Errorf("s1 Success = false, Error = %q", sr.Error)
	}
}

func TestManagerExecuteCondition(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "cond-pb",
		Steps: []Step{
			{ID: "s1", Name: "Run", Action: "run_command", Command: "true"},
			{ID: "s2", Name: "Check", Action: "condition", Condition: "on_success"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	execCtx, err := m.Execute(context.Background(), "cond-pb", nil, nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if execCtx.Status != "completed" {
		t.Errorf("Status = %q, want %q", execCtx.Status, "completed")
	}
	sr := execCtx.StepResults["s2"]
	if !sr.Success {
		t.Errorf("s2 (condition) Success = false, Output = %q, Error = %q", sr.Output, sr.Error)
	}
}

func TestManagerExecuteWithParams(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "params-pb",
		Params: []Param{
			{Name: "greeting", Type: "string", Required: true},
		},
		Steps: []Step{
			{ID: "s1", Name: "Greet", Action: "run_command", Command: "echo {{.greeting}}"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	execCtx, err := m.Execute(context.Background(), "params-pb", map[string]string{
		"greeting": "hello",
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if execCtx.Status != "completed" {
		t.Errorf("Status = %q, want %q", execCtx.Status, "completed")
	}
	sr := execCtx.StepResults["s1"]
	if !sr.Success {
		t.Errorf("s1 Success = false, Error = %q", sr.Error)
	}
}

func TestManagerExecuteMissingRequiredParam(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "req-param",
		Params: []Param{
			{Name: "name", Type: "string", Required: true},
		},
		Steps: []Step{
			{ID: "s1", Action: "run_command", Command: "echo {{.name}}"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	_, err := m.Execute(context.Background(), "req-param", nil, nil)
	if err == nil {
		t.Error("expected error for missing required param")
	}
}

func TestManagerExecuteOnStepCallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "callback-pb",
		Steps: []Step{
			{ID: "s1", Action: "run_command", Command: "echo a"},
			{ID: "s2", Action: "run_command", Command: "echo b"},
		},
	}
	if err := m.Save(pb); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	var results []*StepResult
	execCtx, err := m.Execute(context.Background(), "callback-pb", nil, func(sr *StepResult) {
		results = append(results, sr)
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("callback called %d times, want 2", len(results))
	}
	_ = execCtx
}

func TestManagerExecuteLoadNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	_, err := m.Execute(context.Background(), "nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for nonexistent playbook")
	}
}

func TestManagerValidateDuplicateParam(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "dup-param",
		Params: []Param{
			{Name: "x", Type: "string"},
			{Name: "x", Type: "string"},
		},
		Steps: []Step{{ID: "s1", Action: "run_command", Command: "true"}},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for duplicate param name")
	}
}

func TestManagerValidateInvalidParamType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "bad-param-type",
		Params: []Param{
			{Name: "x", Type: "float"},
		},
		Steps: []Step{{ID: "s1", Action: "run_command", Command: "true"}},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for invalid param type")
	}
}

func TestManagerValidateChoiceNoChoices(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(dir)

	pb := &Playbook{
		Name: "choice-no-opts",
		Params: []Param{
			{Name: "env", Type: "choice"},
		},
		Steps: []Step{{ID: "s1", Action: "run_command", Command: "true"}},
	}
	if err := m.Validate(pb); err == nil {
		t.Error("expected error for choice param with no choices")
	}
}

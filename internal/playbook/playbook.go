package playbook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"gopkg.in/yaml.v3"
)

// Playbook is a parameterized, reusable workflow template.
type Playbook struct {
	Name          string         `yaml:"name" json:"name"`
	Description   string         `yaml:"description" json:"description"`
	Version       string         `yaml:"version" json:"version"`
	Params        []Param        `yaml:"params" json:"params"`
	Steps         []Step         `yaml:"steps" json:"steps"`
	ApprovalGates []ApprovalGate `yaml:"approval_gates,omitempty" json:"approval_gates,omitempty"`
	Tags          []string       `yaml:"tags,omitempty" json:"tags,omitempty"`
	Author        string         `yaml:"author,omitempty" json:"author,omitempty"`
}

// Param defines a playbook parameter.
type Param struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Type        string   `yaml:"type" json:"type"`
	Default     string   `yaml:"default,omitempty" json:"default,omitempty"`
	Required    bool     `yaml:"required" json:"required"`
	Choices     []string `yaml:"choices,omitempty" json:"choices,omitempty"`
}

// Step defines a single step in the playbook.
type Step struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Action      string            `yaml:"action" json:"action"`
	Template    string            `yaml:"template,omitempty" json:"template,omitempty"`
	Find        string            `yaml:"find,omitempty" json:"find,omitempty"`
	Append      string            `yaml:"append,omitempty" json:"append,omitempty"`
	Command     string            `yaml:"command,omitempty" json:"command,omitempty"`
	Prompt      string            `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Condition   string            `yaml:"condition,omitempty" json:"condition,omitempty"`
	NextStep    string            `yaml:"next_step,omitempty" json:"next_step,omitempty"`
	OnFailure   string            `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
	MaxRetries  int               `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// ApprovalGate defines a point where human approval is required.
type ApprovalGate struct {
	AfterStep string `yaml:"after_step" json:"after_step"`
	Message   string `yaml:"message" json:"message"`
}

// ExecutionContext holds runtime state during playbook execution.
type ExecutionContext struct {
	Playbook    *Playbook
	Params      map[string]string
	StepResults map[string]*StepResult
	CurrentStep int
	Status      string
}

// StepResult holds the outcome of a single step.
type StepResult struct {
	StepID   string
	Success  bool
	Output   string
	Duration time.Duration
	Error    string
}

// Manager manages playbook CRUD and execution.
type Manager struct {
	dir string
	mu  sync.RWMutex
}

// NewManager creates a new Manager backed by the given directory.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

// Load reads a playbook YAML file by name.
func (m *Manager) Load(name string) (*Playbook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := filepath.Join(m.dir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read playbook %q: %w", name, err)
	}
	var pb Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("parse playbook %q: %w", name, err)
	}
	return &pb, nil
}

// Save writes a playbook to a YAML file.
func (m *Manager) Save(pb *Playbook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create playbook dir: %w", err)
	}
	data, err := yaml.Marshal(pb)
	if err != nil {
		return fmt.Errorf("marshal playbook: %w", err)
	}
	path := filepath.Join(m.dir, pb.Name+".yaml")
	return os.WriteFile(path, data, 0o644)
}

// List returns all playbooks in the storage directory.
func (m *Manager) List() ([]*Playbook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create playbook dir: %w", err)
	}
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("read playbook dir: %w", err)
	}
	var playbooks []*Playbook
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}
		var pb Playbook
		if err := yaml.Unmarshal(data, &pb); err != nil {
			continue
		}
		playbooks = append(playbooks, &pb)
	}
	return playbooks, nil
}

// Delete removes a playbook file by name.
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.dir, name+".yaml")
	return os.Remove(path)
}

// Validate checks a playbook for structural correctness.
func (m *Manager) Validate(pb *Playbook) error {
	if pb.Name == "" {
		return fmt.Errorf("playbook name is required")
	}
	if len(pb.Steps) == 0 {
		return fmt.Errorf("playbook must have at least one step")
	}

	paramNames := make(map[string]bool)
	for _, p := range pb.Params {
		if p.Name == "" {
			return fmt.Errorf("param name is required")
		}
		if paramNames[p.Name] {
			return fmt.Errorf("duplicate param name: %q", p.Name)
		}
		paramNames[p.Name] = true
		switch p.Type {
		case "string", "int", "bool", "choice":
		default:
			return fmt.Errorf("invalid param type %q for %q", p.Type, p.Name)
		}
		if p.Type == "choice" && len(p.Choices) == 0 {
			return fmt.Errorf("param %q is choice type but has no choices", p.Name)
		}
	}

	stepIDs := make(map[string]bool)
	for _, s := range pb.Steps {
		if s.ID == "" {
			return fmt.Errorf("step ID is required")
		}
		if stepIDs[s.ID] {
			return fmt.Errorf("duplicate step ID: %q", s.ID)
		}
		stepIDs[s.ID] = true
		switch s.Action {
		case "create_file", "edit_file", "run_command", "prompt", "condition":
		default:
			return fmt.Errorf("invalid step action %q for step %q", s.Action, s.ID)
		}
	}

	for _, s := range pb.Steps {
		if s.NextStep != "" && !stepIDs[s.NextStep] {
			return fmt.Errorf("step %q references unknown next_step %q", s.ID, s.NextStep)
		}
		if s.OnFailure != "" {
			switch s.OnFailure {
			case "abort", "skip", "retry":
			default:
				if !stepIDs[s.OnFailure] {
					return fmt.Errorf("step %q references unknown on_failure step %q", s.ID, s.OnFailure)
				}
			}
		}
	}

	for _, g := range pb.ApprovalGates {
		if !stepIDs[g.AfterStep] {
			return fmt.Errorf("approval gate references unknown step %q", g.AfterStep)
		}
	}

	return nil
}

// Execute runs a playbook by name with the given parameters.
func (m *Manager) Execute(ctx context.Context, name string, params map[string]string, onStep func(step *StepResult)) (*ExecutionContext, error) {
	pb, err := m.Load(name)
	if err != nil {
		return nil, err
	}
	if err := m.Validate(pb); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	resolved := make(map[string]string)
	for _, p := range pb.Params {
		val, ok := params[p.Name]
		if !ok {
			val = p.Default
		}
		if p.Required && val == "" {
			return nil, fmt.Errorf("required param %q is missing", p.Name)
		}
		if val != "" {
			switch p.Type {
			case "int":
				if _, err := strconv.Atoi(val); err != nil {
					return nil, fmt.Errorf("param %q must be int, got %q", p.Name, val)
				}
			case "bool":
				if _, err := strconv.ParseBool(val); err != nil {
					return nil, fmt.Errorf("param %q must be bool, got %q", p.Name, val)
				}
			case "choice":
				found := false
				for _, c := range p.Choices {
					if c == val {
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("param %q value %q not in choices %v", p.Name, val, p.Choices)
				}
			}
		}
		resolved[p.Name] = val
	}

	execCtx := &ExecutionContext{
		Playbook:    pb,
		Params:      resolved,
		StepResults: make(map[string]*StepResult),
		CurrentStep: 0,
		Status:      "running",
	}

	stepIndex := make(map[string]int)
	for i, s := range pb.Steps {
		stepIndex[s.ID] = i
	}

	render := func(tmpl string) string {
		t, err := template.New("").Parse(tmpl)
		if err != nil {
			return tmpl
		}
		data := make(map[string]interface{})
		for k, v := range resolved {
			data[k] = v
		}
		for stepID, sr := range execCtx.StepResults {
			data[stepID] = map[string]interface{}{
				"success": sr.Success,
				"output":  sr.Output,
				"error":   sr.Error,
			}
		}
		for _, s := range pb.Steps {
			for k, v := range s.Variables {
				data[k] = renderSimple(v, resolved)
			}
		}
		var buf strings.Builder
		if err := t.Execute(&buf, data); err != nil {
			return tmpl
		}
		return buf.String()
	}

	i := 0
	for i < len(pb.Steps) {
		select {
		case <-ctx.Done():
			execCtx.Status = "aborted"
			return execCtx, ctx.Err()
		default:
		}

		step := pb.Steps[i]
		execCtx.CurrentStep = i

		step.Command = render(step.Command)
		step.Template = render(step.Template)
		step.Find = render(step.Find)
		step.Append = render(step.Append)
		step.Prompt = render(step.Prompt)
		step.Condition = render(step.Condition)

		start := time.Now()
		result := &StepResult{StepID: step.ID}

		switch step.Action {
		case "condition":
			result = executeCondition(&step, execCtx, render)
		case "run_command":
			result = executeRunCommand(&step)
		case "create_file":
			result = executeCreateFile(&step)
		case "edit_file":
			result = executeEditFile(&step)
		case "prompt":
			result = executePrompt(&step)
		default:
			result.Success = true
			result.Output = fmt.Sprintf("step %q completed", step.ID)
		}

		result.Duration = time.Since(start)
		execCtx.StepResults[step.ID] = result

		if onStep != nil {
			onStep(result)
		}

		if !result.Success {
			switch step.OnFailure {
			case "abort":
				execCtx.Status = "failed"
				return execCtx, fmt.Errorf("step %q failed: %s", step.ID, result.Error)
			case "skip":
				i++
				continue
			case "retry":
				retries := step.MaxRetries
				if retries == 0 {
					retries = 1
				}
				retried := false
				for r := 0; r < retries; r++ {
					start = time.Now()
					retryResult := &StepResult{StepID: step.ID}
					if step.Action == "condition" {
						retryResult = executeCondition(&step, execCtx, render)
					} else if step.Action == "run_command" {
						retryResult = executeRunCommand(&step)
					} else if step.Action == "create_file" {
						retryResult = executeCreateFile(&step)
					} else if step.Action == "edit_file" {
						retryResult = executeEditFile(&step)
					} else if step.Action == "prompt" {
						retryResult = executePrompt(&step)
					} else {
						retryResult.Success = true
						retryResult.Output = fmt.Sprintf("action %q retried for step %q", step.Action, step.ID)
					}
					retryResult.Duration = time.Since(start)
					execCtx.StepResults[step.ID] = retryResult
					if onStep != nil {
						onStep(retryResult)
					}
					if retryResult.Success {
						retried = true
						break
					}
				}
				if !retried {
					execCtx.Status = "failed"
					return execCtx, fmt.Errorf("step %q failed after %d retries: %s", step.ID, retries, result.Error)
				}
			default:
				if step.OnFailure != "" {
					if idx, ok := stepIndex[step.OnFailure]; ok {
						i = idx
						continue
					}
				}
				execCtx.Status = "failed"
				return execCtx, fmt.Errorf("step %q failed: %s", step.ID, result.Error)
			}
		}

		for _, gate := range pb.ApprovalGates {
			if gate.AfterStep == step.ID {
				execCtx.Status = "paused"
				return execCtx, nil
			}
		}

		if step.NextStep != "" {
			if idx, ok := stepIndex[step.NextStep]; ok {
				i = idx
				continue
			}
		}

		i++
	}

	execCtx.Status = "completed"
	return execCtx, nil
}

var paramPattern = regexp.MustCompile(`\{\{\.(\w+)\}\}`)

func renderSimple(s string, params map[string]string) string {
	return paramPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := strings.Trim(match, "{}.")
		if val, ok := params[key]; ok {
			return val
		}
		return match
	})
}

func executeCondition(step *Step, execCtx *ExecutionContext, render func(string) string) *StepResult {
	result := &StepResult{StepID: step.ID}
	cond := strings.TrimSpace(step.Condition)
	if cond == "" {
		result.Success = true
		return result
	}

	switch cond {
	case "on_success":
		for _, sr := range execCtx.StepResults {
			if !sr.Success {
				result.Success = false
				result.Output = "previous step failed"
				return result
			}
		}
		result.Success = true
		result.Output = "all previous steps succeeded"
		return result
	case "on_failure":
		for _, sr := range execCtx.StepResults {
			if !sr.Success {
				result.Success = true
				result.Output = "previous step failure detected"
				return result
			}
		}
		result.Success = false
		result.Output = "no previous step failures"
		return result
	}

	if strings.Contains(cond, "==") {
		parts := strings.SplitN(cond, "==", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		leftResolved := render(left)
		result.Output = fmt.Sprintf("condition: %q == %q => %v", leftResolved, right, leftResolved == right)
		result.Success = leftResolved == right
		return result
	}

	if strings.Contains(cond, "!=") {
		parts := strings.SplitN(cond, "!=", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		leftResolved := render(left)
		result.Output = fmt.Sprintf("condition: %q != %q => %v", leftResolved, right, leftResolved != right)
		result.Success = leftResolved != right
		return result
	}

	result.Success = true
	result.Output = fmt.Sprintf("condition %q evaluated as true", cond)
	return result
}

func executeRunCommand(step *Step) *StepResult {
	result := &StepResult{StepID: step.ID}
	if step.Command == "" {
		result.Error = "no command specified"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		result.Error = fmt.Sprintf("%s: %s", err, string(output))
		result.Success = false
		return result
	}

	result.Success = true
	result.Output = string(output)
	return result
}

var defaultAPIClient *api.Client

// SetAPIClient sets the API client for prompt-type playbook steps.
func SetAPIClient(client *api.Client) {
	defaultAPIClient = client
}

func executeCreateFile(step *Step) *StepResult {
	result := &StepResult{StepID: step.ID}
	if step.Template == "" && step.Append == "" {
		result.Error = "no template or append content specified"
		return result
	}

	filePath := step.Find
	if filePath == "" {
		result.Error = "no file path specified in 'find' field"
		return result
	}

	content := step.Template
	if content == "" {
		content = step.Append
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		result.Error = fmt.Sprintf("create directory: %v", err)
		return result
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		result.Error = fmt.Sprintf("write file: %v", err)
		return result
	}

	result.Success = true
	result.Output = fmt.Sprintf("created file: %s (%d bytes)", filePath, len(content))
	return result
}

func executeEditFile(step *Step) *StepResult {
	result := &StepResult{StepID: step.ID}
	if step.Find == "" {
		result.Error = "no 'find' pattern specified for edit"
		return result
	}

	filePath := step.Find
	data, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("read file: %v", err)
		return result
	}

	content := string(data)
	if step.Append != "" && step.Template != "" {
		newContent := strings.Replace(content, step.Append, step.Template, 1)
		if newContent == content && step.Append != "" {
			result.Error = fmt.Sprintf("search string not found in %s", filePath)
			return result
		}
		content = newContent
	} else if step.Template != "" {
		content = step.Template
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		result.Error = fmt.Sprintf("write file: %v", err)
		return result
	}

	result.Success = true
	result.Output = fmt.Sprintf("edited file: %s (%d bytes)", filePath, len(content))
	return result
}

func executePrompt(step *Step) *StepResult {
	result := &StepResult{StepID: step.ID}
	if step.Prompt == "" {
		result.Error = "no prompt specified"
		return result
	}

	if defaultAPIClient == nil {
		result.Error = "no API client configured for prompt action"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := []api.MessageParam{
		{Role: "user", Content: step.Prompt},
	}

	resp, err := defaultAPIClient.CreateMessageCtx(ctx, messages, "You are a helpful coding assistant. Respond concisely.")
	if err != nil {
		result.Error = fmt.Sprintf("LLM call failed: %v", err)
		return result
	}

	if len(resp.Content) > 0 && resp.Content[0].Text != "" {
		result.Success = true
		result.Output = resp.Content[0].Text
		return result
	}

	result.Error = "empty response from LLM"
	return result
}

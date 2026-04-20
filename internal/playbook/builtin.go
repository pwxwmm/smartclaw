package playbook

// BuiltinPlaybooks returns a set of built-in playbook templates.
func BuiltinPlaybooks() []*Playbook {
	return []*Playbook{
		{
			Name:        "add-go-test",
			Description: "Creates a test file for a Go package with specified test functions",
			Version:     "1.0.0",
			Author:      "smartclaw",
			Tags:        []string{"go", "testing", "scaffold"},
			Params: []Param{
				{
					Name:        "package_path",
					Description: "Path to the Go package (e.g. internal/playbook)",
					Type:        "string",
					Required:    true,
				},
				{
					Name:        "test_functions",
					Description: "Comma-separated list of test function names (e.g. TestLoad,TestSave)",
					Type:        "string",
					Required:    true,
				},
			},
			Steps: []Step{
				{
					ID:          "check_pkg",
					Name:        "Check package exists",
					Description: "Verify the target package directory exists",
					Action:      "run_command",
					Command:     "test -d {{.package_path}}",
					OnFailure:   "abort",
				},
				{
					ID:          "create_test_file",
					Name:        "Create test file",
					Description: "Generate the _test.go file with test function stubs",
					Action:      "create_file",
					Template:    "{{.package_path}}/*_test.go",
					Variables: map[string]string{
						"pkg_name": "{{.package_path}}",
						"funcs":    "{{.test_functions}}",
					},
				},
				{
					ID:          "verify_compile",
					Name:        "Verify test compiles",
					Description: "Run go vet to ensure the test file compiles",
					Action:      "run_command",
					Command:     "go vet {{.package_path}}",
					OnFailure:   "abort",
					MaxRetries:  2,
				},
			},
			ApprovalGates: []ApprovalGate{
				{
					AfterStep: "create_test_file",
					Message:   "Review generated test file before verifying compilation",
				},
			},
		},
		{
			Name:        "fix-lint-error",
			Description: "Fixes a linting error in a source file",
			Version:     "1.0.0",
			Author:      "smartclaw",
			Tags:        []string{"lint", "fix", "quality"},
			Params: []Param{
				{
					Name:        "file",
					Description: "Path to the file with the lint error",
					Type:        "string",
					Required:    true,
				},
				{
					Name:        "line",
					Description: "Line number of the lint error",
					Type:        "int",
					Required:    true,
				},
				{
					Name:        "linter",
					Description: "Name of the linter reporting the error",
					Type:        "choice",
					Required:    true,
					Choices:     []string{"golint", "staticcheck", "govet", "errcheck", "gosec"},
				},
				{
					Name:        "message",
					Description: "The lint error message",
					Type:        "string",
					Required:    true,
				},
			},
			Steps: []Step{
				{
					ID:          "read_context",
					Name:        "Read file context",
					Description: "Read the file around the reported line for context",
					Action:      "run_command",
					Command:     "sed -n '$(({{.line}}-5)),p' {{.file}}",
					OnFailure:   "abort",
				},
				{
					ID:          "propose_fix",
					Name:        "Propose fix",
					Description: "Analyze the lint error and propose a fix",
					Action:      "prompt",
					Prompt:      "Fix {{.linter}} error at line {{.line}} in {{.file}}: {{.message}}",
				},
				{
					ID:          "apply_fix",
					Name:        "Apply fix",
					Description: "Apply the proposed fix to the file",
					Action:      "edit_file",
					Find:        "{{.message}}",
					Append:      "// fixed: {{.message}}",
				},
				{
					ID:          "re_lint",
					Name:        "Re-run linter",
					Description: "Verify the fix resolves the lint error",
					Action:      "run_command",
					Command:     "{{.linter}} {{.file}}",
					OnFailure:   "retry",
					MaxRetries:  3,
				},
			},
			ApprovalGates: []ApprovalGate{
				{
					AfterStep: "propose_fix",
					Message:   "Approve the proposed lint fix before applying",
				},
			},
		},
		{
			Name:        "migrate-endpoint",
			Description: "Migrates a REST endpoint from one format to another",
			Version:     "1.0.0",
			Author:      "smartclaw",
			Tags:        []string{"api", "migration", "rest"},
			Params: []Param{
				{
					Name:        "endpoint",
					Description: "The REST endpoint path (e.g. /api/v1/users)",
					Type:        "string",
					Required:    true,
				},
				{
					Name:        "from_format",
					Description: "Current format of the endpoint",
					Type:        "choice",
					Required:    true,
					Choices:     []string{"rest", "graphql", "grpc", "websocket"},
				},
				{
					Name:        "to_format",
					Description: "Target format to migrate to",
					Type:        "choice",
					Required:    true,
					Choices:     []string{"rest", "graphql", "grpc", "websocket"},
				},
			},
			Steps: []Step{
				{
					ID:          "find_handler",
					Name:        "Find endpoint handler",
					Description: "Locate the existing endpoint handler code",
					Action:      "run_command",
					Command:     "grep -rn '{{.endpoint}}' --include='*.go'",
					OnFailure:   "abort",
				},
				{
					ID:          "analyze_handler",
					Name:        "Analyze handler",
					Description: "Analyze the current handler signature and dependencies",
					Action:      "prompt",
					Prompt:      "Analyze the {{.from_format}} handler for {{.endpoint}} and plan migration to {{.to_format}}",
				},
				{
					ID:          "check_compat",
					Name:        "Check compatibility",
					Description: "Verify the migration is feasible",
					Action:      "condition",
					Condition:   "on_success",
					NextStep:    "create_new_handler",
				},
				{
					ID:          "create_new_handler",
					Name:        "Create new handler",
					Description: "Create the new format handler alongside the old one",
					Action:      "create_file",
					Template:    "handler_{{.to_format}}.go",
					Variables: map[string]string{
						"endpoint":   "{{.endpoint}}",
						"from_fmt":   "{{.from_format}}",
						"to_fmt":     "{{.to_format}}",
					},
				},
				{
					ID:          "update_routes",
					Name:        "Update route registration",
					Description: "Register the new handler and deprecate the old route",
					Action:      "edit_file",
					Find:        "{{.endpoint}}",
					Append:      "// migrated: {{.from_format}} -> {{.to_format}}",
				},
				{
					ID:          "verify_build",
					Name:        "Verify build",
					Description: "Ensure the project still compiles after migration",
					Action:      "run_command",
					Command:     "go build ./...",
					OnFailure:   "retry",
					MaxRetries:  2,
				},
			},
			ApprovalGates: []ApprovalGate{
				{
					AfterStep: "analyze_handler",
					Message:   "Review migration plan before creating new handler",
				},
				{
					AfterStep: "update_routes",
					Message:   "Confirm route changes before build verification",
				},
			},
		},
	}
}

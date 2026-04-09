package skills

var BundledSkillDefinitions = map[string]BundledSkill{
	"code-review": {
		Name:        "code-review",
		Description: "Review code changes with best practices and suggestions",
		Triggers:    []string{"/review", "/code-review"},
		Tools:       []string{"bash", "read_file", "grep", "ast_grep"},
		Tags:        []string{"code", "review", "quality"},
		Content: `# Code Review Skill

Review code changes with best practices and suggestions.

## Triggers
- /review
- /code-review

## Tools
- bash
- read_file
- grep
- ast_grep
- lsp

## Instructions

When asked to review code:

1. **Identify Scope**
   - Determine what files or changes need review
   - Use git diff to see recent changes
   - Focus on the most impactful areas

2. **Code Quality Analysis**
   - Check for code smells and anti-patterns
   - Verify consistent naming conventions
   - Look for duplicate code
   - Evaluate code complexity

3. **Best Practices**
   - Ensure proper error handling
   - Check for security vulnerabilities
   - Verify input validation
   - Review resource management

4. **Architecture Review**
   - Evaluate design patterns used
   - Check module boundaries
   - Review dependency management
   - Assess test coverage

5. **Provide Feedback**
   - Prioritize issues by severity
   - Provide actionable suggestions
   - Include code examples for fixes
   - Explain the reasoning

## Tags
code, review, quality, best-practices
`,
	},

	"git-expert": {
		Name:        "git-expert",
		Description: "Advanced git operations and workflow guidance",
		Triggers:    []string{"/git", "/git-*"},
		Tools:       []string{"bash"},
		Tags:        []string{"git", "version-control", "workflow"},
		Content: `# Git Expert Skill

Advanced git operations and workflow guidance.

## Triggers
- /git-status
- /git-diff
- /git-commit
- /git-branch
- /git-merge
- /git-rebase

## Tools
- bash

## Instructions

Help with git operations including:

1. **Branch Management**
   - Create, switch, and delete branches
   - Handle merge conflicts
   - Implement branching strategies (git flow, trunk-based)

2. **Commit Workflow**
   - Write meaningful commit messages
   - Amend and rebase commits
   - Interactive rebase for clean history

3. **Merge & Rebase**
   - Resolve conflicts intelligently
   - Choose appropriate strategy
   - Maintain clean commit history

4. **Repository Analysis**
   - Find bugs with git bisect
   - Track file history
   - Identify contributors

## Tags
git, version-control, workflow, branching
`,
	},

	"test-generator": {
		Name:        "test-generator",
		Description: "Generate comprehensive test suites",
		Triggers:    []string{"/test", "/generate-tests"},
		Tools:       []string{"bash", "read_file", "write_file", "grep"},
		Tags:        []string{"testing", "quality", "automation"},
		Content: `# Test Generator Skill

Generate comprehensive test suites.

## Triggers
- /test
- /generate-tests
- /test-coverage

## Tools
- bash
- read_file
- write_file
- grep
- ast_grep

## Instructions

Generate tests for code by:

1. **Analyze Structure**
   - Parse function signatures
   - Identify inputs and outputs
   - Map dependencies

2. **Identify Test Cases**
   - Normal cases
   - Edge cases
   - Error cases
   - Boundary conditions

3. **Generate Tests**
   - Unit tests for functions
   - Integration tests for modules
   - Mock external dependencies
   - Add assertions

4. **Ensure Coverage**
   - Aim for high code coverage
   - Test all code paths
   - Include negative tests

## Tags
testing, quality, automation, tdd
`,
	},

	"documentation": {
		Name:        "documentation",
		Description: "Generate and maintain documentation",
		Triggers:    []string{"/doc", "/document", "/readme"},
		Tools:       []string{"read_file", "write_file", "bash"},
		Tags:        []string{"docs", "communication", "clarity"},
		Content: `# Documentation Skill

Generate and maintain documentation.

## Triggers
- /doc
- /document
- /readme
- /api-docs

## Tools
- read_file
- write_file
- bash
- grep

## Instructions

Create documentation by:

1. **README Files**
   - Project overview
   - Installation instructions
   - Usage examples
   - Configuration options

2. **API Documentation**
   - Endpoint descriptions
   - Request/response schemas
   - Authentication requirements
   - Error codes

3. **Code Comments**
   - Function documentation
   - Complex logic explanations
   - Type annotations
   - Examples

4. **Usage Examples**
   - Quick start guides
   - Common use cases
   - Troubleshooting tips

## Tags
docs, communication, clarity, api
`,
	},

	"refactoring": {
		Name:        "refactoring",
		Description: "Safe code refactoring with tests",
		Triggers:    []string{"/refactor", "/restructure"},
		Tools:       []string{"bash", "read_file", "write_file", "edit_file", "grep", "lsp"},
		Tags:        []string{"refactoring", "quality", "clean-code"},
		Content: `# Refactoring Skill

Safe code refactoring with tests.

## Triggers
- /refactor
- /restructure
- /cleanup

## Tools
- bash
- read_file
- write_file
- edit_file
- grep
- lsp
- ast_grep

## Instructions

Refactor code safely by:

1. **Analysis**
   - Understand existing code
   - Identify code smells
   - Map dependencies
   - Check test coverage

2. **Preparation**
   - Ensure tests exist
   - Run tests to establish baseline
   - Create backup if needed

3. **Refactoring**
   - Apply small changes
   - Run tests after each change
   - Use automated tools
   - Maintain behavior

4. **Verification**
   - Run full test suite
   - Check for regressions
   - Review performance
   - Update documentation

## Tags
refactoring, quality, clean-code, SOLID
`,
	},

	"debugger": {
		Name:        "debugger",
		Description: "Debug and fix code issues",
		Triggers:    []string{"/debug", "/fix", "/troubleshoot"},
		Tools:       []string{"bash", "read_file", "grep", "lsp"},
		Tags:        []string{"debugging", "troubleshooting", "fixes"},
		Content: `# Debugger Skill

Debug and fix code issues.

## Triggers
- /debug
- /fix
- /troubleshoot
- /error

## Tools
- bash
- read_file
- grep
- lsp
- ast_grep

## Instructions

Debug code by:

1. **Reproduce Issue**
   - Gather error messages
   - Collect stack traces
   - Document reproduction steps

2. **Analyze**
   - Read relevant code
   - Use LSP for navigation
   - Check logs
   - Examine state

3. **Identify Root Cause**
   - Trace execution flow
   - Check assumptions
   - Validate inputs
   - Review recent changes

4. **Implement Fix**
   - Make minimal changes
   - Add tests for bug
   - Verify fix works
   - Prevent regression

## Tags
debugging, troubleshooting, fixes, errors
`,
	},

	"api-designer": {
		Name:        "api-designer",
		Description: "Design and implement APIs",
		Triggers:    []string{"/api", "/design-api", "/endpoint"},
		Tools:       []string{"read_file", "write_file", "bash"},
		Tags:        []string{"api", "design", "rest", "graphql"},
		Content: `# API Designer Skill

Design and implement APIs.

## Triggers
- /api
- /design-api
- /endpoint
- /schema

## Tools
- read_file
- write_file
- bash
- grep

## Instructions

Design APIs by:

1. **Define Requirements**
   - Identify resources
   - Define operations
   - Specify authentication
   - Plan versioning

2. **Design Endpoints**
   - Use RESTful conventions
   - Define URL patterns
   - Specify HTTP methods
   - Plan error responses

3. **Create Schemas**
   - Define request bodies
   - Define response shapes
   - Add validation rules
   - Document types

4. **Implement**
   - Create handlers
   - Add middleware
   - Implement validation
   - Add error handling

5. **Document**
   - Generate API docs
   - Add examples
   - Document auth
   - Include error codes

## Tags
api, design, rest, graphql, openapi
`,
	},

	"performance": {
		Name:        "performance",
		Description: "Analyze and optimize performance",
		Triggers:    []string{"/perf", "/optimize", "/profile"},
		Tools:       []string{"bash", "read_file", "grep"},
		Tags:        []string{"performance", "optimization", "profiling"},
		Content: `# Performance Skill

Analyze and optimize performance.

## Triggers
- /perf
- /optimize
- /profile
- /benchmark

## Tools
- bash
- read_file
- grep
- ast_grep

## Instructions

Optimize performance by:

1. **Profile**
   - Run benchmarks
   - Identify hot paths
   - Measure memory usage
   - Track allocations

2. **Identify Bottlenecks**
   - CPU-intensive operations
   - Memory leaks
   - I/O wait times
   - Database queries

3. **Optimize**
   - Cache results
   - Lazy loading
   - Batch operations
   - Use efficient algorithms

4. **Verify**
   - Re-run benchmarks
   - Compare before/after
   - Check memory impact
   - Ensure correctness

## Tags
performance, optimization, profiling, benchmarking
`,
	},

	"security": {
		Name:        "security",
		Description: "Security analysis and hardening",
		Triggers:    []string{"/security", "/audit", "/hardening"},
		Tools:       []string{"bash", "read_file", "grep"},
		Tags:        []string{"security", "audit", "hardening"},
		Content: `# Security Skill

Security analysis and hardening.

## Triggers
- /security
- /audit
- /hardening
- /vulnerability

## Tools
- bash
- read_file
- grep
- ast_grep

## Instructions

Analyze security by:

1. **Vulnerability Scan**
   - Check for known CVEs
   - Scan dependencies
   - Identify insecure patterns
   - Review configuration

2. **Code Review**
   - SQL injection risks
   - XSS vulnerabilities
   - CSRF protection
   - Input validation

3. **Authentication Review**
   - Password handling
   - Session management
   - Token security
   - OAuth implementation

4. **Authorization Check**
   - Access control
   - Permission checks
   - Role management
   - Resource isolation

## Tags
security, audit, hardening, owasp
`,
	},

	"deployment": {
		Name:        "deployment",
		Description: "Deploy and configure applications",
		Triggers:    []string{"/deploy", "/release", "/ship"},
		Tools:       []string{"bash", "read_file", "write_file"},
		Tags:        []string{"deployment", "devops", "release"},
		Content: `# Deployment Skill

Deploy and configure applications.

## Triggers
- /deploy
- /release
- /ship

## Tools
- bash
- read_file
- write_file

## Instructions

Handle deployments by:

1. **Pre-deployment**
   - Run tests
   - Build artifacts
   - Check configuration
   - Verify dependencies

2. **Deploy**
   - Create deployment configs
   - Deploy to environment
   - Run migrations
   - Update services

3. **Post-deployment**
   - Verify health checks
   - Monitor logs
   - Check metrics
   - Validate functionality

4. **Rollback Plan**
   - Document rollback steps
   - Keep previous version
   - Test rollback procedure

## Tags
deployment, devops, release, ci-cd
`,
	},

	"batch": {
		Name:        "batch",
		Description: "Execute operations in batch mode",
		Triggers:    []string{"/batch", "/bulk"},
		Tools:       []string{"bash", "read_file", "write_file"},
		Tags:        []string{"batch", "automation", "bulk"},
		Content: `# Batch Skill

Execute operations in batch mode.

## Triggers
- /batch
- /bulk
- /parallel

## Tools
- bash
- read_file
- write_file
- grep

## Instructions

Execute batch operations:

1. **Planning**
   - Identify batch targets
   - Group related operations
   - Plan execution order
   - Set up error handling

2. **Execution**
   - Process items in sequence or parallel
   - Track progress
   - Handle failures gracefully
   - Log results

3. **Reporting**
   - Summarize results
   - Report failures
   - Track timing
   - Generate output

## Tags
batch, automation, bulk, parallel
`,
	},

	"loop": {
		Name:        "loop",
		Description: "Create persistent execution loops",
		Triggers:    []string{"/loop", "/repeat", "/watch"},
		Tools:       []string{"bash", "read_file"},
		Tags:        []string{"loop", "automation", "watcher"},
		Content: `# Loop Skill

Create persistent execution loops.

## Triggers
- /loop
- /repeat
- /watch

## Tools
- bash
- read_file
- grep

## Instructions

Create execution loops:

1. **Setup**
   - Define loop condition
   - Set iteration interval
   - Configure exit criteria
   - Plan error handling

2. **Execution**
   - Run task repeatedly
   - Check conditions
   - Handle errors
   - Track state

3. **Monitoring**
   - Log progress
   - Track iterations
   - Monitor resources
   - Report status

## Tags
loop, automation, watcher, persistent
`,
	},

	"remember": {
		Name:        "remember",
		Description: "Store and recall information across sessions",
		Triggers:    []string{"/remember", "/recall", "/memory"},
		Tools:       []string{"read_file", "write_file"},
		Tags:        []string{"memory", "persistence", "context"},
		Content: `# Remember Skill

Store and recall information across sessions.

## Triggers
- /remember
- /recall
- /memory
- /forget

## Tools
- read_file
- write_file
- grep

## Instructions

Manage persistent memory:

1. **Store**
   - Save important information
   - Tag with keywords
   - Set expiration if needed
   - Organize by category

2. **Recall**
   - Search stored memories
   - Filter by tags
   - List recent items
   - Export data

3. **Manage**
   - Update entries
   - Delete obsolete
   - Merge duplicates
   - Archive old items

## Tags
memory, persistence, context, knowledge
`,
	},

	"verify": {
		Name:        "verify",
		Description: "Verify changes meet requirements",
		Triggers:    []string{"/verify", "/validate", "/check"},
		Tools:       []string{"bash", "read_file", "grep", "lsp"},
		Tags:        []string{"verification", "validation", "quality"},
		Content: `# Verify Skill

Verify changes meet requirements.

## Triggers
- /verify
- /validate
- /check

## Tools
- bash
- read_file
- grep
- lsp
- ast_grep

## Instructions

Verify code changes:

1. **Requirements Check**
   - List requirements
   - Map to implementations
   - Verify completeness
   - Check edge cases

2. **Behavioral Verification**
   - Run tests
   - Check outputs
   - Verify side effects
   - Test error handling

3. **Code Quality**
   - Lint checks
   - Type checks
   - Security scans
   - Performance benchmarks

4. **Documentation**
   - Update docs
   - Add examples
   - Document changes
   - Review comments

## Tags
verification, validation, quality, requirements
`,
	},

	"skillify": {
		Name:        "skillify",
		Description: "Convert code patterns into reusable skills",
		Triggers:    []string{"/skillify", "/make-skill"},
		Tools:       []string{"read_file", "write_file", "bash"},
		Tags:        []string{"skills", "patterns", "reusability"},
		Content: `# Skillify Skill

Convert code patterns into reusable skills.

## Triggers
- /skillify
- /make-skill
- /create-skill

## Tools
- read_file
- write_file
- bash
- grep

## Instructions

Create reusable skills:

1. **Identify Pattern**
   - Find reusable code
   - Extract common logic
   - Define parameters
   - Document usage

2. **Create Skill**
   - Write skill template
   - Add triggers
   - Define tools needed
   - Include examples

3. **Test**
   - Verify skill works
   - Test edge cases
   - Validate parameters
   - Check error handling

4. **Document**
   - Write usage guide
   - Add examples
   - Document parameters
   - Include tags

## Tags
skills, patterns, reusability, automation
`,
	},

	"simplify": {
		Name:        "simplify",
		Description: "Simplify complex code and logic",
		Triggers:    []string{"/simplify", "/clean", "/reduce"},
		Tools:       []string{"read_file", "write_file", "edit_file"},
		Tags:        []string{"simplification", "refactoring", "clean-code"},
		Content: `# Simplify Skill

Simplify complex code and logic.

## Triggers
- /simplify
- /clean
- /reduce

## Tools
- read_file
- write_file
- edit_file
- ast_grep

## Instructions

Simplify code:

1. **Identify Complexity**
   - Find complex functions
   - Locate nested logic
   - Spot duplicate code
   - Identify dead code

2. **Simplify**
   - Break down functions
   - Remove redundancy
   - Extract helpers
   - Use built-in features

3. **Verify**
   - Ensure behavior unchanged
   - Run tests
   - Check performance
   - Review readability

## Tags
simplification, refactoring, clean-code, complexity
`,
	},

	"stuck": {
		Name:        "stuck",
		Description: "Help when stuck on a problem",
		Triggers:    []string{"/stuck", "/help", "/unblock"},
		Tools:       []string{"bash", "read_file", "grep"},
		Tags:        []string{"help", "troubleshooting", "problem-solving"},
		Content: `# Stuck Skill

Help when stuck on a problem.

## Triggers
- /stuck
- /help
- /unblock

## Tools
- bash
- read_file
- grep
- web_fetch

## Instructions

Get unstuck:

1. **Analyze Situation**
   - Understand the problem
   - Review recent attempts
   - Check error messages
   - Examine context

2. **Explore Solutions**
   - Search documentation
   - Find similar issues
   - Check Stack Overflow
   - Review best practices

3. **Try Alternative Approaches**
   - Break down problem
   - Try minimal example
   - Use debugging tools
   - Ask for clarification

4. **Document Learning**
   - Record solution
   - Update notes
   - Share findings

## Tags
help, troubleshooting, problem-solving, learning
`,
	},

	"claude-api": {
		Name:        "claude-api",
		Description: "Work with Claude API directly",
		Triggers:    []string{"/claude-api", "/api-call"},
		Tools:       []string{"bash", "read_file", "write_file"},
		Tags:        []string{"api", "claude", "integration"},
		Content: `# Claude API Skill

Work with Claude API directly.

## Triggers
- /claude-api
- /api-call
- /anthropic

## Tools
- bash
- read_file
- write_file

## Instructions

Use Claude API:

1. **Setup**
   - Configure API key
   - Set up client
   - Choose model
   - Set parameters

2. **Make Requests**
   - Format messages
   - Handle streaming
   - Process responses
   - Handle errors

3. **Advanced Usage**
   - Use tools
   - Manage context
   - Handle images
   - Track usage

## Tags
api, claude, anthropic, integration
`,
	},

	"keybindings": {
		Name:        "keybindings",
		Description: "Manage and configure keybindings",
		Triggers:    []string{"/keybindings", "/keys", "/shortcuts"},
		Tools:       []string{"read_file", "write_file"},
		Tags:        []string{"keybindings", "configuration", "shortcuts"},
		Content: `# Keybindings Skill

Manage and configure keybindings.

## Triggers
- /keybindings
- /keys
- /shortcuts

## Tools
- read_file
- write_file

## Instructions

Manage keybindings:

1. **View Current**
   - List all keybindings
   - Show conflicts
   - Display categories

2. **Configure**
   - Add new bindings
   - Remove bindings
   - Update existing
   - Import/export

3. **Troubleshoot**
   - Find conflicts
   - Test bindings
   - Reset to defaults

## Tags
keybindings, configuration, shortcuts, vim
`,
	},

	"update-config": {
		Name:        "update-config",
		Description: "Update and manage configuration",
		Triggers:    []string{"/update-config", "/config", "/settings"},
		Tools:       []string{"read_file", "write_file"},
		Tags:        []string{"configuration", "settings", "management"},
		Content: `# Update Config Skill

Update and manage configuration.

## Triggers
- /update-config
- /config
- /settings

## Tools
- read_file
- write_file
- bash

## Instructions

Manage configuration:

1. **View Current**
   - Display settings
   - Show config file location
   - List all options

2. **Update**
   - Set individual options
   - Update multiple settings
   - Import configuration
   - Reset to defaults

3. **Validate**
   - Check configuration
   - Validate values
   - Test settings

## Tags
configuration, settings, management, preferences
`,
	},
}

type BundledSkill struct {
	Name        string
	Description string
	Triggers    []string
	Tools       []string
	Tags        []string
	Content     string
}

func GetBundledSkillNames() []string {
	names := make([]string, 0, len(BundledSkillDefinitions))
	for name := range BundledSkillDefinitions {
		names = append(names, name)
	}
	return names
}

func GetBundledSkill(name string) *BundledSkill {
	if skill, ok := BundledSkillDefinitions[name]; ok {
		return &skill
	}
	return nil
}

func GetBundledSkillsByTag(tag string) []*BundledSkill {
	var skills []*BundledSkill
	for _, skill := range BundledSkillDefinitions {
		for _, t := range skill.Tags {
			if t == tag {
				skills = append(skills, &skill)
				break
			}
		}
	}
	return skills
}

package github

import (
	"encoding/json"
	"testing"
)

func TestPRJSON_ToPR(t *testing.T) {
	input := prJSON{
		Number:      42,
		Title:       "Fix bug",
		State:       "open",
		HTMLURL:     "https://github.com/org/repo/pull/42",
		HeadRefName: "feature-branch",
		BaseRefName: "main",
		Author:      prAuthor{Login: "dev"},
	}
	pr := input.toPR()
	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Fix bug" {
		t.Errorf("Title = %q, want %q", pr.Title, "Fix bug")
	}
	if pr.State != "open" {
		t.Errorf("State = %q, want %q", pr.State, "open")
	}
	if pr.Head != "feature-branch" {
		t.Errorf("Head = %q, want %q", pr.Head, "feature-branch")
	}
	if pr.Base != "main" {
		t.Errorf("Base = %q, want %q", pr.Base, "main")
	}
	if pr.Author != "dev" {
		t.Errorf("Author = %q, want %q", pr.Author, "dev")
	}
}

func TestIssueJSON_ToIssue(t *testing.T) {
	input := issueJSON{
		Number:  7,
		Title:   "Bug report",
		State:   "open",
		HTMLURL: "https://github.com/org/repo/issues/7",
		Author:  issueAuthor{Login: "reporter"},
	}
	issue := input.toIssue()
	if issue.Number != 7 {
		t.Errorf("Number = %d, want 7", issue.Number)
	}
	if issue.Title != "Bug report" {
		t.Errorf("Title = %q, want %q", issue.Title, "Bug report")
	}
	if issue.Author != "reporter" {
		t.Errorf("Author = %q, want %q", issue.Author, "reporter")
	}
}

func TestAtoi_Valid(t *testing.T) {
	n, err := atoi("42")
	if err != nil {
		t.Fatalf("atoi(42) returned error: %v", err)
	}
	if n != 42 {
		t.Errorf("atoi(42) = %d, want 42", n)
	}
}

func TestAtoi_WithWhitespace(t *testing.T) {
	n, err := atoi("  99  ")
	if err != nil {
		t.Fatalf("atoi('  99  ') returned error: %v", err)
	}
	if n != 99 {
		t.Errorf("atoi('  99  ') = %d, want 99", n)
	}
}

func TestAtoi_Invalid(t *testing.T) {
	_, err := atoi("abc")
	if err == nil {
		t.Error("atoi('abc') should return error")
	}
}

func TestAtoi_Empty(t *testing.T) {
	_, err := atoi("")
	if err == nil {
		t.Error("atoi('') should return error")
	}
}

func TestParseCodeResults_Empty(t *testing.T) {
	results := parseCodeResults("")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestParseCodeResults_SingleLine(t *testing.T) {
	output := `{path:"main.go",repo:"org/repo",url:"https://github.com/org/repo/blob/main/main.go"}`
	results := parseCodeResults(output)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "main.go" {
		t.Errorf("Path = %q, want %q", results[0].Path, "main.go")
	}
	if results[0].Repo != "org/repo" {
		t.Errorf("Repo = %q, want %q", results[0].Repo, "org/repo")
	}
}

func TestParseCodeResults_MultipleLines(t *testing.T) {
	output := `{path:"a.go",repo:"org/repo",url:"https://github.com/org/repo/blob/main/a.go"}
{path:"b.go",repo:"org/repo",url:"https://github.com/org/repo/blob/main/b.go"}`
	results := parseCodeResults(output)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestParseCodeResultLine_NonJSONObject(t *testing.T) {
	result := parseCodeResultLine("not json")
	if result != nil {
		t.Error("expected nil for non-JSON line")
	}
}

func TestParseCodeResultLine_ShortLine(t *testing.T) {
	result := parseCodeResultLine("{}")
	if result != nil {
		t.Error("expected nil for empty object with no recognized fields")
	}
}

func TestRepoViewJSON_Parse(t *testing.T) {
	data := `{
		"owner": {"login": "myorg"},
		"name": "myrepo",
		"description": "A test repo",
		"url": "https://github.com/myorg/myrepo",
		"defaultBranchRef": {"name": "main"}
	}`
	var view repoViewJSON
	if err := json.Unmarshal([]byte(data), &view); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if view.Owner.Login != "myorg" {
		t.Errorf("owner login = %q, want %q", view.Owner.Login, "myorg")
	}
	if view.Name != "myrepo" {
		t.Errorf("name = %q, want %q", view.Name, "myrepo")
	}
	if view.DefaultBranchRef.Name != "main" {
		t.Errorf("default branch = %q, want %q", view.DefaultBranchRef.Name, "main")
	}
}

func TestPRJSON_Parse(t *testing.T) {
	data := `{
		"number": 10,
		"title": "Add feature",
		"state": "open",
		"url": "https://github.com/org/repo/pull/10",
		"headRefName": "feat",
		"baseRefName": "main",
		"author": {"login": "dev"}
	}`
	var item prJSON
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	pr := item.toPR()
	if pr.Number != 10 {
		t.Errorf("Number = %d, want 10", pr.Number)
	}
	if pr.Author != "dev" {
		t.Errorf("Author = %q, want %q", pr.Author, "dev")
	}
}

func TestPRListJSON_Parse(t *testing.T) {
	data := `[
		{"number":1,"title":"PR1","state":"open","url":"http://example.com/1","headRefName":"f1","baseRefName":"main","author":{"login":"a"}},
		{"number":2,"title":"PR2","state":"closed","url":"http://example.com/2","headRefName":"f2","baseRefName":"main","author":{"login":"b"}}
	]`
	var items []prJSON
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].toPR().Author != "a" {
		t.Errorf("first PR author = %q, want %q", items[0].toPR().Author, "a")
	}
}

func TestIssueJSON_Parse(t *testing.T) {
	data := `{
		"number": 5,
		"title": "Bug",
		"state": "open",
		"url": "https://github.com/org/repo/issues/5",
		"author": {"login": "user1"}
	}`
	var item issueJSON
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	issue := item.toIssue()
	if issue.Number != 5 {
		t.Errorf("Number = %d, want 5", issue.Number)
	}
}

func TestWorkflowJSON_Parse(t *testing.T) {
	data := `[{"id":1,"name":"CI","state":"active"},{"id":2,"name":"Deploy","state":"disabled"}]`
	var workflows []*Workflow
	if err := json.Unmarshal([]byte(data), &workflows); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(workflows))
	}
	if workflows[0].Name != "CI" {
		t.Errorf("workflow name = %q, want %q", workflows[0].Name, "CI")
	}
}

func TestWorkflowRunJSON_Parse(t *testing.T) {
	data := `[{"databaseId":100,"name":"CI","status":"completed","conclusion":"success","url":"http://example.com"}]`
	var runs []*WorkflowRun
	if err := json.Unmarshal([]byte(data), &runs); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ID != 100 {
		t.Errorf("run ID = %d, want 100", runs[0].ID)
	}
	if runs[0].Conclusion != "success" {
		t.Errorf("run conclusion = %q, want %q", runs[0].Conclusion, "success")
	}
}

func TestRepository_Struct(t *testing.T) {
	repo := Repository{
		Owner:         "myorg",
		Name:          "myrepo",
		FullName:      "myorg/myrepo",
		Description:   "A test repo",
		DefaultBranch: "main",
		HTMLURL:       "https://github.com/myorg/myrepo",
	}
	if repo.FullName != "myorg/myrepo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "myorg/myrepo")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestPROptions_Struct(t *testing.T) {
	opts := PROptions{
		Title:  "Fix something",
		Body:   "This PR fixes X",
		Head:   "fix-branch",
		Base:   "main",
		Draft:  true,
		Labels: []string{"bugfix", "urgent"},
	}
	if !opts.Draft {
		t.Error("Draft should be true")
	}
	if len(opts.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(opts.Labels))
	}
}

func TestIssueOptions_Struct(t *testing.T) {
	opts := IssueOptions{
		Title:  "Bug report",
		Body:   "Something is broken",
		Labels: []string{"bug"},
	}
	if opts.Title != "Bug report" {
		t.Errorf("Title = %q, want %q", opts.Title, "Bug report")
	}
}

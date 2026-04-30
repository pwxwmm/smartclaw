package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

type RecentProject struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	LastOpened  string `json:"lastOpened"`
}

var recentProjectsMu sync.Mutex

func recentProjectsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".smartclaw", "recent_projects.json")
}

func loadRecentProjects() ([]RecentProject, error) {
	path := recentProjectsPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RecentProject{}, nil
		}
		return nil, err
	}
	var projects []RecentProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func saveRecentProjects(projects []RecentProject) error {
	path := recentProjectsPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func addToRecentProjects(path string) {
	recentProjectsMu.Lock()
	defer recentProjectsMu.Unlock()

	projects, err := loadRecentProjects()
	if err != nil {
		projects = []RecentProject{}
	}

	name := filepath.Base(path)
	now := time.Now().Format(time.RFC3339)

	for i, p := range projects {
		if p.Path == path {
			projects = append(projects[:i], projects[i+1:]...)
			break
		}
	}

	entry := RecentProject{
		Path:       path,
		Name:       name,
		LastOpened: now,
	}
	projects = append([]RecentProject{entry}, projects...)

	if len(projects) > 10 {
		projects = projects[:10]
	}

	_ = saveRecentProjects(projects)
}

func (h *Handler) handleChangeProject(client *Client, msg WSMessage) {
	newPath := msg.Path
	if newPath == "" {
		h.sendError(client, "Path is required")
		return
	}

	if !filepath.IsAbs(newPath) {
		h.sendError(client, fmt.Sprintf("Absolute path required (got: %s). Example: /Users/jw/SOPA", newPath))
		return
	}

	absPath := filepath.Clean(newPath)

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			h.sendError(client, fmt.Sprintf("Path does not exist: %s", absPath))
		} else {
			h.sendError(client, fmt.Sprintf("Cannot access path: %v", err))
		}
		return
	}

	if !info.IsDir() {
		h.sendError(client, fmt.Sprintf("Path is not a directory: %s", absPath))
		return
	}

	testFile, err := os.OpenFile(filepath.Join(absPath, ".smartclaw_project_test"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Directory is not writable: %v", err))
		return
	}
	testFile.Close()
	os.Remove(filepath.Join(absPath, ".smartclaw_project_test"))

	h.mu.Lock()
	h.workDir = absPath
	h.mu.Unlock()

	tools.SetAllowedDirs([]string{absPath})

	go addToRecentProjects(absPath)

	tree, err := h.buildFileTree(absPath, 3)
	if err != nil {
		tree = []FileNode{}
	}

	h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
		Type: "file_tree",
		Tree: tree,
	}))

	projectName := filepath.Base(absPath)
	h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
		Type:    "project_changed",
		Path:    absPath,
		Message: projectName,
	}))
}

type DirEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
}

func (h *Handler) handleBrowseDirs(client *Client, msg WSMessage) {
	dirPath := msg.Path
	if dirPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			h.sendError(client, "Cannot determine home directory")
			return
		}
		dirPath = homeDir
	}

	if !filepath.IsAbs(dirPath) {
		h.sendError(client, "Absolute path required")
		return
	}

	dirPath = filepath.Clean(dirPath)

	info, err := os.Stat(dirPath)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Cannot access path: %s", dirPath))
		return
	}
	if !info.IsDir() {
		dirPath = filepath.Dir(dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Cannot read directory: %v", err))
		return
	}

	var dirs []DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}
		fullPath := filepath.Join(dirPath, entry.Name())
		dirs = append(dirs, DirEntry{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: true,
		})
	}

	if dirs == nil {
		dirs = []DirEntry{}
	}

	parent := ""
	if dirPath != "/" {
		parent = filepath.Dir(dirPath)
	}

	h.sendToClient(client, WSResponse{
		Type: "browse_dirs",
		Data: map[string]any{
			"path":    dirPath,
			"parent":  parent,
			"entries": dirs,
		},
	})
}

func (h *Handler) handleGetRecentProjects(client *Client) {
	recentProjectsMu.Lock()
	defer recentProjectsMu.Unlock()

	projects, err := loadRecentProjects()
	if err != nil {
		projects = []RecentProject{}
	}

	h.sendToClient(client, WSResponse{
		Type: "recent_projects",
		Data: projects,
	})
}

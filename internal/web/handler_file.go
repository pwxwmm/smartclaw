package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (h *Handler) handleFileOpen(client *Client, msg WSMessage) {
	path := filepath.Join(h.workDir, msg.Path)
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Clean(h.workDir)) {
		h.sendError(client, "Access denied: path outside work directory")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "file_content",
		Content: string(data),
	})
}

func (h *Handler) handleFileSave(client *Client, msg WSMessage) {
	path := filepath.Join(h.workDir, msg.Path)
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Clean(h.workDir)) {
		h.sendError(client, "Access denied: path outside work directory")
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	if err := os.WriteFile(path, []byte(msg.Content), 0644); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "done",
		Message: fmt.Sprintf("File saved: %s", msg.Path),
	})
}

func (h *Handler) handleFileTree(client *Client, msg WSMessage) {
	root := h.workDir
	if msg.Path != "" {
		root = filepath.Join(h.workDir, msg.Path)
	}

	tree, err := h.buildFileTree(root, 3)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to scan directory: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "file_tree",
		Tree: tree,
	})
}

func (h *Handler) handleGitStatus(client *Client, msg WSMessage) {
	statusMap, err := h.getGitStatus()
	if err != nil {
		h.sendToClient(client, WSResponse{
			Type: "git_status",
			Data: map[string]string{},
		})
		return
	}
	h.sendToClient(client, WSResponse{
		Type: "git_status",
		Data: statusMap,
	})
}

func (h *Handler) getGitStatus() (map[string]string, error) {
	gitDir := filepath.Join(h.workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = h.workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		statusCode := strings.TrimSpace(line[:2])
		path := line[3:]
		if path == "" {
			continue
		}
		result[path] = statusCode
	}
	return result, nil
}

func (h *Handler) buildFileTree(root string, maxDepth int) ([]FileNode, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".smartclaw" {
			continue
		}

		node := FileNode{Name: name}

		if entry.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, ".git": true,
				"dist": true, "build": true, "bin": true, "__pycache__": true,
			}
			if skipDirs[name] {
				node.Type = "dir"
				continue
			}

			node.Type = "dir"
			children, err := h.buildFileTree(filepath.Join(root, name), maxDepth-1)
			if err == nil {
				node.Children = children
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				node.Size = info.Size()
			}
			node.Type = "file"
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

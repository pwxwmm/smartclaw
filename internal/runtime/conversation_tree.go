package runtime

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var nodeCounter atomic.Int64

type MessageNode struct {
	ID       string
	Message  Message
	ParentID string
	Children []string
}

type ConversationTree struct {
	nodes    map[string]*MessageNode
	rootID   string
	headID   string
	branches map[string]string
	mu       sync.RWMutex
}

func NewConversationTree() *ConversationTree {
	return &ConversationTree{
		nodes:    make(map[string]*MessageNode),
		branches: make(map[string]string),
	}
}

func (ct *ConversationTree) AddMessage(msg Message) string {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	id := generateNodeID()

	node := &MessageNode{
		ID:      id,
		Message: msg,
	}

	if ct.headID != "" {
		node.ParentID = ct.headID
		if parent, ok := ct.nodes[ct.headID]; ok {
			parent.Children = append(parent.Children, id)
		}
	} else {
		ct.rootID = id
	}

	ct.nodes[id] = node
	ct.headID = id

	return id
}

func (ct *ConversationTree) Branch(fromNodeID string) (string, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, ok := ct.nodes[fromNodeID]; !ok {
		return "", fmt.Errorf("node %s not found", fromNodeID)
	}

	branchID := fmt.Sprintf("branch_%d", nodeCounter.Add(1))
	ct.branches[branchID] = fromNodeID

	return branchID, nil
}

func (ct *ConversationTree) Checkout(branchOrNodeID string) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if nodeID, ok := ct.branches[branchOrNodeID]; ok {
		ct.headID = nodeID
		return nil
	}

	if _, ok := ct.nodes[branchOrNodeID]; ok {
		ct.headID = branchOrNodeID
		return nil
	}

	return fmt.Errorf("branch or node %s not found", branchOrNodeID)
}

func (ct *ConversationTree) GetHeadID() string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.headID
}

func (ct *ConversationTree) GetLinearHistory() []Message {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	if ct.headID == "" {
		return nil
	}

	var path []Message
	visited := make(map[string]bool)
	current := ct.headID

	for current != "" {
		if visited[current] {
			break
		}
		visited[current] = true

		node, ok := ct.nodes[current]
		if !ok {
			break
		}
		path = append([]Message{node.Message}, path...)
		current = node.ParentID
	}

	return path
}

func (ct *ConversationTree) GetChildren(nodeID string) []string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	if node, ok := ct.nodes[nodeID]; ok {
		result := make([]string, len(node.Children))
		copy(result, node.Children)
		return result
	}
	return nil
}

func (ct *ConversationTree) GetBranches() map[string]string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make(map[string]string, len(ct.branches))
	for k, v := range ct.branches {
		result[k] = v
	}
	return result
}

func (ct *ConversationTree) GetNode(id string) (*MessageNode, bool) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	node, ok := ct.nodes[id]
	if !ok {
		return nil, false
	}
	copy := *node
	return &copy, true
}

func (ct *ConversationTree) Size() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.nodes)
}

func generateNodeID() string {
	return fmt.Sprintf("msg_%d", nodeCounter.Add(1))
}

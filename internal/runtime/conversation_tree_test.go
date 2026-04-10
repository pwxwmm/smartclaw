package runtime

import (
	"testing"
)

func TestConversationTree_AddAndGetLinear(t *testing.T) {
	tree := NewConversationTree()

	tree.AddMessage(Message{Role: "user", Content: "hello"})
	tree.AddMessage(Message{Role: "assistant", Content: "hi there"})
	tree.AddMessage(Message{Role: "user", Content: "how are you?"})

	history := tree.GetLinearHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	if history[0].Content != "hello" {
		t.Errorf("expected first message 'hello', got %v", history[0].Content)
	}
	if history[2].Content != "how are you?" {
		t.Errorf("expected third message 'how are you?', got %v", history[2].Content)
	}
}

func TestConversationTree_Branch(t *testing.T) {
	tree := NewConversationTree()

	tree.AddMessage(Message{Role: "user", Content: "hello"})
	nodeID2 := tree.AddMessage(Message{Role: "assistant", Content: "hi"})
	tree.AddMessage(Message{Role: "user", Content: "how are you?"})

	branchID, err := tree.Branch(nodeID2)
	if err != nil {
		t.Fatalf("branch failed: %v", err)
	}

	if branchID == "" {
		t.Error("expected non-empty branch ID")
	}

	branches := tree.GetBranches()
	if len(branches) != 1 {
		t.Errorf("expected 1 branch, got %d", len(branches))
	}
}

func TestConversationTree_Checkout(t *testing.T) {
	tree := NewConversationTree()

	tree.AddMessage(Message{Role: "user", Content: "hello"})
	nodeID2 := tree.AddMessage(Message{Role: "assistant", Content: "hi"})
	tree.AddMessage(Message{Role: "user", Content: "original path"})

	branchID, _ := tree.Branch(nodeID2)

	err := tree.Checkout(branchID)
	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	tree.AddMessage(Message{Role: "user", Content: "branched path"})

	history := tree.GetLinearHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 messages on branch, got %d", len(history))
	}

	lastMsg := history[len(history)-1]
	if lastMsg.Content != "branched path" {
		t.Errorf("expected 'branched path', got %v", lastMsg.Content)
	}
}

func TestConversationTree_CheckoutByNodeID(t *testing.T) {
	tree := NewConversationTree()

	tree.AddMessage(Message{Role: "user", Content: "msg1"})
	nodeID := tree.AddMessage(Message{Role: "assistant", Content: "msg2"})
	tree.AddMessage(Message{Role: "user", Content: "msg3"})

	err := tree.Checkout(nodeID)
	if err != nil {
		t.Fatalf("checkout by node ID failed: %v", err)
	}

	if tree.GetHeadID() != nodeID {
		t.Errorf("expected head ID %s, got %s", nodeID, tree.GetHeadID())
	}
}

func TestConversationTree_CheckoutNonExistent(t *testing.T) {
	tree := NewConversationTree()

	err := tree.Checkout("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent branch/node")
	}
}

func TestConversationTree_GetChildren(t *testing.T) {
	tree := NewConversationTree()

	nodeID1 := tree.AddMessage(Message{Role: "user", Content: "msg1"})
	tree.AddMessage(Message{Role: "assistant", Content: "msg2"})
	tree.AddMessage(Message{Role: "user", Content: "msg3"})

	children := tree.GetChildren(nodeID1)
	if len(children) != 1 {
		t.Errorf("expected 1 child for root, got %d", len(children))
	}
}

func TestConversationTree_EmptyTree(t *testing.T) {
	tree := NewConversationTree()

	history := tree.GetLinearHistory()
	if history != nil {
		t.Errorf("expected nil history for empty tree, got %v", history)
	}

	if tree.GetHeadID() != "" {
		t.Error("expected empty head ID for empty tree")
	}
}

func TestQueryState_TreeIntegration(t *testing.T) {
	state := NewQueryState()
	state.EnableTree()

	state.AddMessage(Message{Role: "user", Content: "hello"})
	state.AddMessage(Message{Role: "assistant", Content: "hi"})

	msgs := state.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestQueryState_LinearFallback(t *testing.T) {
	state := NewQueryState()

	state.AddMessage(Message{Role: "user", Content: "hello"})
	state.AddMessage(Message{Role: "assistant", Content: "hi"})

	msgs := state.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages from linear path, got %d", len(msgs))
	}
}

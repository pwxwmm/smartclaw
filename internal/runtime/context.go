package runtime

type ContextManager struct {
	messages  []Message
	maxTokens int
}

func NewContextManager(maxTokens int) *ContextManager {
	return &ContextManager{
		messages:  make([]Message, 0),
		maxTokens: maxTokens,
	}
}

func (cm *ContextManager) AddMessage(msg Message) {
	cm.messages = append(cm.messages, msg)
}

func (cm *ContextManager) GetMessages() []Message {
	return cm.messages
}

func (cm *ContextManager) Clear() {
	cm.messages = make([]Message, 0)
}

func (cm *ContextManager) Trim() {
	if ShouldCompact(cm.messages, cm.maxTokens) {
		cm.messages = Compact(cm.messages, cm.maxTokens)
	}
}

func (cm *ContextManager) TokenCount() int {
	return CountMessagesTokens(cm.messages)
}

package dap

import "encoding/json"

type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

type Breakpoint struct {
	ID        int     `json:"id"`
	Line      int     `json:"line,omitempty"`
	Source    *Source `json:"source,omitempty"`
	Verified  bool    `json:"verified"`
	Condition string  `json:"condition,omitempty"`
}

type StackFrame struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Source *Source `json:"source,omitempty"`
	Line   int     `json:"line"`
	Column int     `json:"column,omitempty"`
}

type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference,omitempty"`
}

type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive"`
}

type DAPRequest struct {
	Seq       int            `json:"seq"`
	Type      string         `json:"type"`
	Command   string         `json:"command"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type DAPResponse struct {
	Seq        int            `json:"seq"`
	Type       string         `json:"type"`
	Command    string         `json:"command"`
	RequestSeq int            `json:"request_seq"`
	Success    bool           `json:"success"`
	Body       map[string]any `json:"body,omitempty"`
	Message    string         `json:"message,omitempty"`
}

type DAPEvent struct {
	Seq   int            `json:"seq"`
	Type  string         `json:"type"`
	Event string         `json:"event"`
	Body  map[string]any `json:"body,omitempty"`
}

type DAPMessage struct {
	Seq        int            `json:"seq"`
	Type       string         `json:"type"`
	Command    string         `json:"command,omitempty"`
	Arguments  map[string]any `json:"arguments,omitempty"`
	Body       map[string]any `json:"body,omitempty"`
	Event      string         `json:"event,omitempty"`
	RequestSeq int            `json:"request_seq,omitempty"`
	Success    bool           `json:"success,omitempty"`
	Message    string         `json:"message,omitempty"`
}

func (r *DAPRequest) MarshalJSON() ([]byte, error) {
	type Alias DAPRequest
	return json.Marshal((*Alias)(r))
}

func (r *DAPRequest) UnmarshalJSON(data []byte) error {
	type Alias DAPRequest
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = DAPRequest(a)
	return nil
}

func (r *DAPResponse) MarshalJSON() ([]byte, error) {
	type Alias DAPResponse
	return json.Marshal((*Alias)(r))
}

func (r *DAPResponse) UnmarshalJSON(data []byte) error {
	type Alias DAPResponse
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = DAPResponse(a)
	return nil
}

func (e *DAPEvent) MarshalJSON() ([]byte, error) {
	type Alias DAPEvent
	return json.Marshal((*Alias)(e))
}

func (e *DAPEvent) UnmarshalJSON(data []byte) error {
	type Alias DAPEvent
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = DAPEvent(a)
	return nil
}

func (m *DAPMessage) MarshalJSON() ([]byte, error) {
	type Alias DAPMessage
	return json.Marshal((*Alias)(m))
}

func (m *DAPMessage) UnmarshalJSON(data []byte) error {
	type Alias DAPMessage
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = DAPMessage(a)
	return nil
}

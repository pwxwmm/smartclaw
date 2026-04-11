package tools

import "fmt"

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func ErrRequiredField(field string) *Error {
	return &Error{Code: "REQUIRED_FIELD", Message: field + " is required"}
}

func ErrNotImplemented(tool string) *Error {
	return &Error{Code: "NOT_IMPLEMENTED", Message: tool + " is not yet implemented"}
}

func ErrToolNotFound(tool string) *Error {
	return &Error{Code: "TOOL_NOT_FOUND", Message: "unknown tool: " + tool}
}

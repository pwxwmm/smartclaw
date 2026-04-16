package errors

import (
	"errors"
	"fmt"
)

type Category string

const (
	CategoryNetwork  Category = "network"
	CategoryAuth     Category = "auth"
	CategoryConfig   Category = "config"
	CategoryQuota    Category = "quota"
	CategoryTool     Category = "tool"
	CategoryInput    Category = "input"
	CategoryInternal Category = "internal"
	CategorySecurity Category = "security"
)

type AppError struct {
	Code      string
	Message   string
	Category  Category
	Retryable bool
	Err       error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code string, message string, opts ...Option) *AppError {
	e := &AppError{Code: code, Message: message}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func Wrap(err error, code string, message string, opts ...Option) *AppError {
	e := &AppError{Code: code, Message: message, Err: err}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type Option func(*AppError)

func WithCategory(c Category) Option {
	return func(e *AppError) { e.Category = c }
}

func WithRetryable(r bool) Option {
	return func(e *AppError) { e.Retryable = r }
}

func IsRetryable(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Retryable
	}
	return false
}

func GetCategory(err error) Category {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Category
	}
	return CategoryInternal
}

func GetCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return "UNKNOWN"
}

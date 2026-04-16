package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	err := New("TEST_CODE", "test message")

	if err.Code != "TEST_CODE" {
		t.Errorf("Code = %q, want %q", err.Code, "TEST_CODE")
	}
	if err.Message != "test message" {
		t.Errorf("Message = %q, want %q", err.Message, "test message")
	}
	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
	if err.Category != "" {
		t.Errorf("Category = %q, want empty", err.Category)
	}
	if err.Retryable {
		t.Errorf("Retryable = true, want false")
	}
}

func TestNewWithOptions(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg",
		WithCategory(CategoryNetwork),
		WithRetryable(true),
	)

	if err.Category != CategoryNetwork {
		t.Errorf("Category = %q, want %q", err.Category, CategoryNetwork)
	}
	if !err.Retryable {
		t.Errorf("Retryable = false, want true")
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("original error")
	err := Wrap(original, "WRAP_CODE", "wrapped message")

	if err.Code != "WRAP_CODE" {
		t.Errorf("Code = %q, want %q", err.Code, "WRAP_CODE")
	}
	if err.Message != "wrapped message" {
		t.Errorf("Message = %q, want %q", err.Message, "wrapped message")
	}
	if err.Err != original {
		t.Errorf("Err = %v, want %v", err.Err, original)
	}
}

func TestWrapWithOptions(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("base")
	err := Wrap(original, "CODE", "msg",
		WithCategory(CategoryAuth),
		WithRetryable(true),
	)

	if err.Category != CategoryAuth {
		t.Errorf("Category = %q, want %q", err.Category, CategoryAuth)
	}
	if !err.Retryable {
		t.Errorf("Retryable = false, want true")
	}
}

func TestAppError_ErrorWithoutWrapped(t *testing.T) {
	t.Parallel()

	err := New("E001", "something failed")
	got := err.Error()
	want := "[E001] something failed"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_ErrorWithWrapped(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("root cause")
	err := Wrap(original, "E002", "wrapped failure")
	got := err.Error()
	want := "[E002] wrapped failure: root cause"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("inner")
	err := Wrap(original, "CODE", "msg")

	unwrapped := err.Unwrap()
	if unwrapped != original {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, original)
	}
}

func TestAppError_UnwrapNil(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg")
	unwrapped := err.Unwrap()
	if unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestWrapChain_Unwrap(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base error")
	middle := Wrap(base, "MID", "middle")
	outer := Wrap(middle, "OUT", "outer")

	unwrapped := errors.Unwrap(outer)
	if unwrapped != middle {
		t.Errorf("first Unwrap = %v, want %v", unwrapped, middle)
	}

	unwrapped = errors.Unwrap(unwrapped)
	if unwrapped != base {
		t.Errorf("second Unwrap = %v, want %v", unwrapped, base)
	}
}

func TestErrorsIs_WithWrappedAppError(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base error")
	wrapped := Wrap(base, "CODE", "wrapped")

	if !errors.Is(wrapped, base) {
		t.Error("errors.Is(wrapped, base) = false, want true")
	}
}

func TestErrorsAs_WithAppError(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg", WithCategory(CategoryQuota))

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatal("errors.As failed to extract *AppError")
	}
	if appErr.Code != "CODE" {
		t.Errorf("Code = %q, want %q", appErr.Code, "CODE")
	}
	if appErr.Category != CategoryQuota {
		t.Errorf("Category = %q, want %q", appErr.Category, CategoryQuota)
	}
}

func TestIsRetryable_True(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg", WithRetryable(true))
	if !IsRetryable(err) {
		t.Error("IsRetryable() = false, want true")
	}
}

func TestIsRetryable_False(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg", WithRetryable(false))
	if IsRetryable(err) {
		t.Error("IsRetryable() = true, want false")
	}
}

func TestIsRetryable_DefaultFalse(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg")
	if IsRetryable(err) {
		t.Error("IsRetryable() = true, want false (default)")
	}
}

func TestIsRetryable_NonAppError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("plain error")
	if IsRetryable(err) {
		t.Error("IsRetryable(plain error) = true, want false")
	}
}

func TestIsRetryable_WrappedAppError(t *testing.T) {
	t.Parallel()

	inner := New("INNER", "retryable", WithRetryable(true))
	outer := fmt.Errorf("wrapped: %w", inner)
	if !IsRetryable(outer) {
		t.Error("IsRetryable(wrapped AppError) = false, want true")
	}
}

func TestGetCategory(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg", WithCategory(CategoryTool))
	if got := GetCategory(err); got != CategoryTool {
		t.Errorf("GetCategory() = %q, want %q", got, CategoryTool)
	}
}

func TestGetCategory_NonAppError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("plain error")
	if got := GetCategory(err); got != CategoryInternal {
		t.Errorf("GetCategory(plain error) = %q, want %q", got, CategoryInternal)
	}
}

func TestGetCategory_DefaultEmpty(t *testing.T) {
	t.Parallel()

	err := New("CODE", "msg")
	if got := GetCategory(err); got != "" {
		t.Errorf("GetCategory(no category set) = %q, want empty string", got)
	}
}

func TestGetCode(t *testing.T) {
	t.Parallel()

	err := New("MY_CODE", "msg")
	if got := GetCode(err); got != "MY_CODE" {
		t.Errorf("GetCode() = %q, want %q", got, "MY_CODE")
	}
}

func TestGetCode_NonAppError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("plain error")
	if got := GetCode(err); got != "UNKNOWN" {
		t.Errorf("GetCode(plain error) = %q, want %q", got, "UNKNOWN")
	}
}

func TestGetCode_WrappedAppError(t *testing.T) {
	t.Parallel()

	inner := New("INNER_CODE", "msg")
	outer := fmt.Errorf("wrapped: %w", inner)
	if got := GetCode(outer); got != "INNER_CODE" {
		t.Errorf("GetCode(wrapped) = %q, want %q", got, "INNER_CODE")
	}
}

func TestCategoryConstants(t *testing.T) {
	t.Parallel()

	categories := map[Category]string{
		CategoryNetwork:  "network",
		CategoryAuth:     "auth",
		CategoryConfig:   "config",
		CategoryQuota:    "quota",
		CategoryTool:     "tool",
		CategoryInput:    "input",
		CategoryInternal: "internal",
		CategorySecurity: "security",
	}

	for cat, want := range categories {
		if string(cat) != want {
			t.Errorf("Category %q = %q, want %q", cat, string(cat), want)
		}
	}

	if len(categories) != 8 {
		t.Errorf("expected 8 categories, got %d", len(categories))
	}
}

func TestWithCategory(t *testing.T) {
	t.Parallel()

	opt := WithCategory(CategoryConfig)
	err := &AppError{}
	opt(err)

	if err.Category != CategoryConfig {
		t.Errorf("Category = %q, want %q", err.Category, CategoryConfig)
	}
}

func TestWithRetryable(t *testing.T) {
	t.Parallel()

	opt := WithRetryable(true)
	err := &AppError{}
	opt(err)

	if !err.Retryable {
		t.Error("Retryable = false, want true")
	}

	opt2 := WithRetryable(false)
	err2 := &AppError{Retryable: true}
	opt2(err2)

	if err2.Retryable {
		t.Error("Retryable = true, want false")
	}
}

func TestNew_MultipleOptions(t *testing.T) {
	t.Parallel()

	err := New("MULTI", "multi option test",
		WithCategory(CategorySecurity),
		WithRetryable(true),
	)

	if err.Category != CategorySecurity {
		t.Errorf("Category = %q, want %q", err.Category, CategorySecurity)
	}
	if !err.Retryable {
		t.Error("Retryable = false, want true")
	}
	if err.Code != "MULTI" {
		t.Errorf("Code = %q, want %q", err.Code, "MULTI")
	}
	if err.Message != "multi option test" {
		t.Errorf("Message = %q, want %q", err.Message, "multi option test")
	}
}

func TestWrap_PreservesOriginalError(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("base")
	wrapped := Wrap(original, "W", "wrapped")

	if !errors.Is(wrapped, original) {
		t.Error("errors.Is cannot find original through Wrap")
	}
}

func TestNilAppError(t *testing.T) {
	t.Parallel()

	if IsRetryable(nil) {
		t.Error("IsRetryable(nil) = true, want false")
	}

	if got := GetCategory(nil); got != CategoryInternal {
		t.Errorf("GetCategory(nil) = %q, want %q", got, CategoryInternal)
	}

	if got := GetCode(nil); got != "UNKNOWN" {
		t.Errorf("GetCode(nil) = %q, want %q", got, "UNKNOWN")
	}
}

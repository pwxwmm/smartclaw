package tui

import (
	"fmt"
	"strings"
)

type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeAPI
	ErrorTypeConfig
	ErrorTypePermission
	ErrorTypeTimeout
	ErrorTypeQuota
	ErrorTypeUnknown
)

type SmartError struct {
	Type       ErrorType
	Message    string
	Suggestion string
	Retryable  bool
	Original   error
}

func (e *SmartError) Error() string {
	return e.Message
}

func ClassifyError(err error) *SmartError {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	switch {
	case strings.Contains(errMsgLower, "connection refused") ||
		strings.Contains(errMsgLower, "no such host") ||
		strings.Contains(errMsgLower, "network is unreachable") ||
		strings.Contains(errMsgLower, "timeout") ||
		strings.Contains(errMsgLower, "deadline exceeded"):
		return &SmartError{
			Type:       ErrorTypeNetwork,
			Message:    "无法连接到 API 服务器",
			Suggestion: "请检查网络连接，或尝试使用代理",
			Retryable:  true,
			Original:   err,
		}

	case strings.Contains(errMsgLower, "invalid api key") ||
		strings.Contains(errMsgLower, "authentication") ||
		strings.Contains(errMsgLower, "unauthorized"):
		return &SmartError{
			Type:       ErrorTypeAPI,
			Message:    "API 密钥无效",
			Suggestion: "运行 /set-api-key 重新设置密钥",
			Retryable:  false,
			Original:   err,
		}

	case strings.Contains(errMsgLower, "rate limit") ||
		strings.Contains(errMsgLower, "quota") ||
		strings.Contains(errMsgLower, "usage limit"):
		return &SmartError{
			Type:       ErrorTypeQuota,
			Message:    "API 配额已用尽",
			Suggestion: "升级套餐或等待配额重置",
			Retryable:  false,
			Original:   err,
		}

	case strings.Contains(errMsgLower, "permission denied") ||
		strings.Contains(errMsgLower, "access denied"):
		return &SmartError{
			Type:       ErrorTypePermission,
			Message:    "权限不足",
			Suggestion: "检查文件权限或 API 访问权限",
			Retryable:  false,
			Original:   err,
		}

	case strings.Contains(errMsgLower, "context deadline exceeded"):
		return &SmartError{
			Type:       ErrorTypeTimeout,
			Message:    "请求超时",
			Suggestion: "使用 /retry-with-timeout 增加超时时间重试",
			Retryable:  true,
			Original:   err,
		}

	case strings.Contains(errMsgLower, "no api key"):
		return &SmartError{
			Type:       ErrorTypeConfig,
			Message:    "未配置 API 密钥",
			Suggestion: "运行 /set-api-key 设置密钥，或设置环境变量 ANTHROPIC_API_KEY",
			Retryable:  false,
			Original:   err,
		}

	default:
		return &SmartError{
			Type:       ErrorTypeUnknown,
			Message:    fmt.Sprintf("发生错误: %s", err.Error()),
			Suggestion: "请查看详细错误信息，或联系支持",
			Retryable:  false,
			Original:   err,
		}
	}
}

func (e *SmartError) FormatError() string {
	var sb strings.Builder

	sb.WriteString("✗ ")
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	if e.Suggestion != "" {
		sb.WriteString("→ ")
		sb.WriteString(e.Suggestion)
		sb.WriteString("\n")
	}

	if e.Retryable {
		sb.WriteString("→ 输入 /retry 重试")
	}

	return sb.String()
}

func FormatSimpleError(message string) string {
	return fmt.Sprintf("✗ %s", message)
}

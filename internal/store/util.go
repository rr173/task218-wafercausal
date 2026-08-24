package store

import (
	"strings"

	"task218-wafercausal/internal/model"
)

// isUniqueViolation 判断错误是否为 SQLite 唯一约束冲突。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed")
}

// boolToInt 将布尔值转为 SQLite 整数。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// untrustedStatus 依据不可信标记返回对应状态。
func untrustedStatus(untrusted bool) string {
	if untrusted {
		return model.EventUntrusted
	}
	return model.EventValid
}

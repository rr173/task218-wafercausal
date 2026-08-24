package model

import "errors"

// 领域错误，service 层据此映射到 HTTP 状态码与业务语义。
var (
	// ErrNotFound 目标实体不存在。
	ErrNotFound = errors.New("not found")
	// ErrInvalid 输入不合法（缺失字段、坐标越界、时间倒退等）。
	ErrInvalid = errors.New("invalid input")
	// ErrConflict 状态机冲突或唯一约束冲突（重复事件、封存后修改等）。
	ErrConflict = errors.New("conflict")
	// ErrArchived 对已封存批次执行修改操作。
	ErrArchived = errors.New("batch archived")
	// ErrStateMachine 非法状态流转。
	ErrStateMachine = errors.New("illegal state transition")
	// ErrDuplicate 幂等键重复。
	ErrDuplicate = errors.New("duplicate record")
)

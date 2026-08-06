package run

import (
	"Qavor/internal/model/entity"
)

// 合法的状态迁移
var transitions = map[string]map[string]bool{
	entity.StatusPending: {
		entity.StatusRunning:   true, // 入队后被 Worker 取出
		entity.StatusCancelled: true, // 排队中取消
		entity.StatusFailed:    true, // 取出后校验失败
	},
	entity.StatusRunning: {
		entity.StatusInterrupted: true, // 工具审批暂停
		entity.StatusCompleted:   true,
		entity.StatusFailed:      true,
		entity.StatusCancelled:   true,
	},
	entity.StatusInterrupted: {
		entity.StatusRunning:   true, // resume 恢复执行
		entity.StatusCancelled: true, // 中断态取消
		entity.StatusFailed:    true,
	},
	// 终态不可迁移
	entity.StatusCompleted: {},
	entity.StatusFailed:    {},
	entity.StatusCancelled: {},
}

// CanTransit 校验 from → to 是否合法
func CanTransit(from, to string) bool {
	if to == from {
		return true
	}
	if next, ok := transitions[from]; ok {
		return next[to]
	}
	return false
}

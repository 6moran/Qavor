package repository

import (
	"errors"
	"time"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// AgentRunRepository AgentRun 仓储接口
type AgentRunRepository interface {
	Create(run *entity.AgentRun) error
	GetByID(id string) (*entity.AgentRun, error)
	GetByRequestID(requestID string) (*entity.AgentRun, error)
	Update(run *entity.AgentRun) error
	UpdateStatus(runID, status string, lastEventID string) error
	ListByThread(threadID string, offset, limit int) ([]entity.AgentRun, int64, error)
	ListByStatus(status string, offset, limit int) ([]entity.AgentRun, int64, error)
	ListSubagentThreadsByParent(parentConversationID uint) ([]entity.SubagentThread, error)
}

type agentRunRepository struct {
	db *gorm.DB
}

// NewAgentRunRepository 创建 AgentRun 仓储
func NewAgentRunRepository(db *gorm.DB) AgentRunRepository {
	return &agentRunRepository{db: db}
}

func (r *agentRunRepository) Create(run *entity.AgentRun) error {
	return r.db.Create(run).Error
}

func (r *agentRunRepository) GetByID(id string) (*entity.AgentRun, error) {
	var run entity.AgentRun
	err := r.db.First(&run, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *agentRunRepository) GetByRequestID(requestID string) (*entity.AgentRun, error) {
	var run entity.AgentRun
	err := r.db.First(&run, "request_id = ?", requestID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *agentRunRepository) Update(run *entity.AgentRun) error {
	return r.db.Save(run).Error
}

// UpdateStatus 更新状态与最后事件 ID，终态时写入 finished_at
func (r *agentRunRepository) UpdateStatus(runID, status, lastEventID string) error {
	updates := map[string]any{
		"status":        status,
		"last_event_id": lastEventID,
		"updated_at":    time.Now(),
	}
	switch status {
	case entity.StatusRunning:
		updates["started_at"] = time.Now()
	case entity.StatusCompleted, entity.StatusFailed, entity.StatusCancelled, entity.StatusInterrupted:
		updates["finished_at"] = time.Now()
	}
	return r.db.Model(&entity.AgentRun{}).Where("id = ?", runID).Updates(updates).Error
}

func (r *agentRunRepository) ListByThread(threadID string, offset, limit int) ([]entity.AgentRun, int64, error) {
	var runs []entity.AgentRun
	var total int64
	db := r.db.Model(&entity.AgentRun{}).Where("conversation_thread_id = ?", threadID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

func (r *agentRunRepository) ListByStatus(status string, offset, limit int) ([]entity.AgentRun, int64, error) {
	var runs []entity.AgentRun
	var total int64
	db := r.db.Model(&entity.AgentRun{}).Where("status = ?", status)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

// ListSubagentThreadsByParent 查询某父会话下的所有子智能体线程关系
func (r *agentRunRepository) ListSubagentThreadsByParent(parentConversationID uint) ([]entity.SubagentThread, error) {
	var threads []entity.SubagentThread
	if err := r.db.Where("parent_conversation_id = ?", parentConversationID).
		Order("created_at DESC").Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

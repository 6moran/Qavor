package entity

import "time"

// MessageFeedback 消息反馈实体
type MessageFeedback struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	MessageID uint      `gorm:"not null;index;comment:消息ID" json:"message_id"`
	UID       string    `gorm:"type:varchar(64);not null;index;comment:用户UID" json:"uid"`
	Rating    string    `gorm:"type:varchar(10);not null;comment:评分：like/dislike" json:"rating"`
	Reason    string    `gorm:"type:text;comment:不喜欢的原因" json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// 关联关系
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName 指定表名
func (MessageFeedback) TableName() string {
	return "message_feedbacks"
}

package models

import (
	"time"
)

// WhitelistedChannel 表示一个被加入白名单的频道
type WhitelistedChannel struct {
	ID          int64     `db:"id"`
	ChatID      int64     `db:"chat_id"`     // 群组ID
	ChannelID   int64     `db:"channel_id"`  // 频道ID
	AddedBy     int64     `db:"added_by"`    // 添加者ID
	AddedAt     time.Time `db:"added_at"`    // 添加时间
	Description string    `db:"description"` // 频道描述
}

// BlockedMessage 记录被删除的消息
type BlockedMessage struct {
	ID          int64     `db:"id"`
	ChatID      int64     `db:"chat_id"`      // 群组ID
	ChannelID   int64     `db:"channel_id"`   // 频道ID
	MessageID   int       `db:"message_id"`   // 消息ID
	BlockedAt   time.Time `db:"blocked_at"`   // 阻止时间
	MessageText string    `db:"message_text"` // 消息内容，可能为空
}

// BlockedMessageInfo 用于消息队列的简化结构
type BlockedMessageInfo struct {
	ChatID      int64  // 群组ID
	ChannelID   int64  // 频道ID
	MessageID   int    // 消息ID
	MessageText string // 消息内容，可能为空
}

// GroupSettings 存储群组的设置信息
type GroupSettings struct {
	ChatID       int64 `db:"chat_id"`        // 群组ID
	AdminOnly    bool  `db:"admin_only"`     // 是否只有管理员可以管理白名单
	LogChannelID int64 `db:"log_channel_id"` // 日志频道ID
	Enabled      bool  `db:"enabled"`        // 是否启用机器人
}

// ChannelApplication 存储频道申请信息
type ChannelApplication struct {
	ID              int64     `db:"id"`
	ChatID          int64     `db:"chat_id"`          // 群组ID
	ChannelID       int64     `db:"channel_id"`       // 频道ID
	UserID          int64     `db:"user_id"`          // 申请用户ID
	Reason          string    `db:"reason"`           // 申请理由
	AppliedAt       time.Time `db:"applied_at"`       // 申请时间
	Status          string    `db:"status"`           // 状态：pending, approved, rejected
	VerifiedChannel bool      `db:"verified_channel"` // 是否已验证频道所有权
	LastPromptDate  time.Time `db:"last_prompt_date"` // 最后一次提示日期
	PromptedToday   bool      `db:"prompted_today"`   // 今日是否已提示过
}

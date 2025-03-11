package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/anhe/tg-whitelist-bot/db/models"
	_ "modernc.org/sqlite"
)

// DB 封装数据库连接和操作
type DB struct {
	conn *sql.DB
}

// New 创建一个新的数据库连接
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 设置连接池参数
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	if _, err := conn.Exec(`PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA busy_timeout=10000;
		PRAGMA cache_size=10000;
		PRAGMA temp_store=MEMORY;
		PRAGMA mmap_size=30000000;
		PRAGMA page_size=4096;`); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.init(); err != nil {
		return nil, err
	}

	return db, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.conn.Close()
}

// init 初始化数据库表结构
func (db *DB) init() error {
	// 创建白名单频道表
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS whitelisted_channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			channel_id INTEGER NOT NULL,
			added_by INTEGER NOT NULL,
			added_at TIMESTAMP NOT NULL,
			description TEXT,
			UNIQUE(chat_id, channel_id)
		)
	`)
	if err != nil {
		return err
	}

	// 创建被阻止的消息表
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS blocked_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			channel_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			blocked_at TIMESTAMP NOT NULL,
			message_text TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 创建群组设置表
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS group_settings (
			chat_id INTEGER PRIMARY KEY,
			admin_only BOOLEAN NOT NULL DEFAULT 1,
			log_channel_id INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1
		)
	`)
	if err != nil {
		return err
	}

	// 创建频道申请表
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS channel_applications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			channel_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			reason TEXT,
			applied_at TIMESTAMP NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			verified_channel BOOLEAN NOT NULL DEFAULT 0,
			last_prompt_date DATE,
			prompted_today BOOLEAN NOT NULL DEFAULT 0,
			UNIQUE(chat_id, channel_id, user_id)
		)
	`)
	if err != nil {
		return err
	}

	// 创建用户状态表
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS user_states (
			user_id INTEGER PRIMARY KEY,
			state TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// 创建每日频道提示记录表
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS channel_daily_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			channel_id INTEGER NOT NULL,
			prompt_type TEXT NOT NULL,
			prompt_date DATE NOT NULL,
			UNIQUE(chat_id, channel_id, prompt_type, prompt_date)
		)
	`)
	if err != nil {
		return err
	}

	return err
}

// AddChannelToWhitelist 将频道添加到白名单
func (db *DB) AddChannelToWhitelist(chatID, channelID, addedBy int64, description string) error {
	_, err := db.conn.Exec(`
		INSERT INTO whitelisted_channels (chat_id, channel_id, added_by, added_at, description)
		VALUES (?, ?, ?, ?, ?)
	`, chatID, channelID, addedBy, time.Now(), description)
	return err
}

// RemoveChannelFromWhitelist 从白名单中移除频道
func (db *DB) RemoveChannelFromWhitelist(chatID, channelID int64) error {
	_, err := db.conn.Exec(`
		DELETE FROM whitelisted_channels
		WHERE chat_id = ? AND channel_id = ?
	`, chatID, channelID)
	return err
}

// IsChannelWhitelisted 检查频道是否在白名单中
func (db *DB) IsChannelWhitelisted(chatID, channelID int64) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM whitelisted_channels
		WHERE chat_id = ? AND channel_id = ?
	`, chatID, channelID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetWhitelistedChannels 获取群组的白名单频道列表
func (db *DB) GetWhitelistedChannels(chatID int64) ([]models.WhitelistedChannel, error) {
	rows, err := db.conn.Query(`
		SELECT id, chat_id, channel_id, added_by, added_at, description
		FROM whitelisted_channels
		WHERE chat_id = ?
		ORDER BY added_at DESC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.WhitelistedChannel
	for rows.Next() {
		var channel models.WhitelistedChannel
		var addedAt string
		err := rows.Scan(
			&channel.ID,
			&channel.ChatID,
			&channel.ChannelID,
			&channel.AddedBy,
			&addedAt,
			&channel.Description,
		)
		if err != nil {
			return nil, err
		}

		// 解析时间
		t, err := time.Parse("2006-01-02 15:04:05", addedAt)
		if err != nil {
			// 如果解析失败，使用当前时间
			t = time.Now()
		}
		channel.AddedAt = t

		channels = append(channels, channel)
	}

	return channels, nil
}

// LogBlockedMessage 记录被阻止的消息
func (db *DB) LogBlockedMessage(chatID, channelID int64, messageID int, messageText string) error {
	// 添加重试机制
	var err error
	for i := 0; i < 5; i++ {
		_, err = db.conn.Exec(`
			INSERT INTO blocked_messages (chat_id, channel_id, message_id, blocked_at, message_text)
			VALUES (?, ?, ?, ?, ?)
		`, chatID, channelID, messageID, time.Now(), messageText)

		if err == nil {
			return nil
		}

		// 如果是数据库忙，等待后重试，使用指数退避策略
		if strings.Contains(err.Error(), "database is locked") {
			backoffTime := time.Duration(100*((i+1)*(i+1))) * time.Millisecond
			time.Sleep(backoffTime)
			continue
		}

		// 其他错误直接返回
		return err
	}
	return err
}

// GetBlockedMessagesStats 获取被阻止消息的统计信息
func (db *DB) GetBlockedMessagesStats(chatID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM blocked_messages
		WHERE chat_id = ?
	`, chatID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetOrCreateGroupSettings 获取或创建群组设置
func (db *DB) GetOrCreateGroupSettings(chatID int64) (models.GroupSettings, error) {
	var settings models.GroupSettings

	// 尝试获取设置
	err := db.conn.QueryRow(`
		SELECT chat_id, admin_only, log_channel_id, enabled
		FROM group_settings
		WHERE chat_id = ?
	`, chatID).Scan(
		&settings.ChatID,
		&settings.AdminOnly,
		&settings.LogChannelID,
		&settings.Enabled,
	)

	// 如果不存在则创建
	if err == sql.ErrNoRows {
		settings = models.GroupSettings{
			ChatID:       chatID,
			AdminOnly:    true,
			LogChannelID: 0,
			Enabled:      true,
		}

		_, err := db.conn.Exec(`
			INSERT INTO group_settings (chat_id, admin_only, log_channel_id, enabled)
			VALUES (?, ?, ?, ?)
		`, settings.ChatID, settings.AdminOnly, settings.LogChannelID, settings.Enabled)
		if err != nil {
			return models.GroupSettings{}, err
		}

		return settings, nil
	}

	if err != nil {
		return models.GroupSettings{}, err
	}

	return settings, nil
}

// UpdateGroupSettings 更新群组设置
func (db *DB) UpdateGroupSettings(settings models.GroupSettings) error {
	_, err := db.conn.Exec(`
		UPDATE group_settings
		SET admin_only = ?, log_channel_id = ?, enabled = ?
		WHERE chat_id = ?
	`, settings.AdminOnly, settings.LogChannelID, settings.Enabled, settings.ChatID)
	return err
}

// CreateChannelApplication 创建频道申请
func (db *DB) CreateChannelApplication(chatID, channelID, userID int64, reason string) error {
	// 检查是否已存在该频道的申请记录
	var existingID int64
	var existingStatus string

	err := db.conn.QueryRow(`
		SELECT id, status
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ?
		ORDER BY applied_at DESC
		LIMIT 1
	`, chatID, channelID).Scan(&existingID, &existingStatus)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// 如果已存在申请且状态为pending，则返回错误
	if existingID > 0 && existingStatus == "pending" {
		return fmt.Errorf("该频道已有待审核的申请")
	}

	// 如果不存在申请或申请已被拒绝，创建/更新申请
	if existingID > 0 {
		// 使用现有记录ID更新
		_, err = db.conn.Exec(`
			UPDATE channel_applications
			SET user_id = ?, reason = ?, applied_at = ?, status = ?, verified_channel = 0
			WHERE id = ?
		`, userID, reason, time.Now(), "pending", existingID)
		return err
	} else {
		// 创建新记录
		_, err = db.conn.Exec(`
			INSERT INTO channel_applications (chat_id, channel_id, user_id, reason, applied_at, status)
			VALUES (?, ?, ?, ?, ?, ?)
		`, chatID, channelID, userID, reason, time.Now(), "pending")
		return err
	}
}

// GetChannelApplication 获取频道申请信息
func (db *DB) GetChannelApplication(chatID, channelID, userID int64) (models.ChannelApplication, error) {
	var app models.ChannelApplication
	var lastPromptDate sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, chat_id, channel_id, user_id, reason, applied_at, status, verified_channel, last_prompt_date
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ? AND user_id = ?
	`, chatID, channelID, userID).Scan(
		&app.ID, &app.ChatID, &app.ChannelID, &app.UserID,
		&app.Reason, &app.AppliedAt, &app.Status, &app.VerifiedChannel, &lastPromptDate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return models.ChannelApplication{}, nil
		}
		return models.ChannelApplication{}, err
	}

	// 处理可能为NULL的last_prompt_date
	if lastPromptDate.Valid {
		t, err := time.Parse("2006-01-02", lastPromptDate.String)
		if err == nil {
			app.LastPromptDate = t
		}
	}

	return app, nil
}

// UpdateChannelApplicationStatus 更新频道申请状态
func (db *DB) UpdateChannelApplicationStatus(chatID, channelID, userID int64, status string) error {
	_, err := db.conn.Exec(`
		UPDATE channel_applications
		SET status = ?
		WHERE chat_id = ? AND channel_id = ? AND user_id = ?
	`, status, chatID, channelID, userID)
	return err
}

// VerifyChannelOwnership 验证频道所有权
func (db *DB) VerifyChannelOwnership(chatID, channelID, userID int64) error {
	_, err := db.conn.Exec(`
		UPDATE channel_applications
		SET verified_channel = 1
		WHERE chat_id = ? AND channel_id = ? AND user_id = ?
	`, chatID, channelID, userID)
	return err
}

// UpdateLastPromptDate 更新最后提示日期，并重置 prompted_today 状态
func (db *DB) UpdateLastPromptDate(chatID, channelID int64) error {
	today := time.Now().Format("2006-01-02")

	_, err := db.conn.Exec(`
		UPDATE channel_applications 
		SET prompted_today = 0, last_prompt_date = ? 
		WHERE chat_id = ? AND channel_id = ? AND status = 'pending'
	`, today, chatID, channelID)

	return err
}

// GetPendingApplications 获取待处理的申请
func (db *DB) GetPendingApplications() ([]models.ChannelApplication, error) {
	rows, err := db.conn.Query(`
		SELECT id, chat_id, channel_id, user_id, reason, applied_at, status, verified_channel, last_prompt_date
		FROM channel_applications
		WHERE status = 'pending'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applications []models.ChannelApplication
	for rows.Next() {
		var app models.ChannelApplication
		var lastPromptDate sql.NullString

		err := rows.Scan(
			&app.ID, &app.ChatID, &app.ChannelID, &app.UserID,
			&app.Reason, &app.AppliedAt, &app.Status, &app.VerifiedChannel, &lastPromptDate,
		)
		if err != nil {
			return nil, err
		}

		// 处理可能为NULL的last_prompt_date
		if lastPromptDate.Valid {
			t, err := time.Parse("2006-01-02", lastPromptDate.String)
			if err == nil {
				app.LastPromptDate = t
			}
		}

		applications = append(applications, app)
	}

	return applications, nil
}

// GetChannelApplicationByDate 根据日期获取频道今日是否已提示过申请
func (db *DB) GetChannelApplicationByDate(chatID, channelID int64, date string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ? AND last_prompt_date = ?
	`, chatID, channelID, date).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// UpdateChannelApplicationUser 更新频道申请的用户ID
func (db *DB) UpdateChannelApplicationUser(chatID, channelID, userID int64) error {
	// 首先检查是否已经存在处于pending状态且已被认领的记录
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ? AND user_id != 0 AND user_id != ? AND status = 'pending'
	`, chatID, channelID, userID).Scan(&count)

	if err != nil {
		return err
	}

	// 如果已经存在被其他人认领过且处于pending状态的记录，则不允许再次认领
	if count > 0 {
		return fmt.Errorf("该频道申请已被认领")
	}

	// 查找最新的申请记录
	var applicationID int64
	err = db.conn.QueryRow(`
		SELECT id
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ? AND status = 'pending'
		ORDER BY applied_at DESC
		LIMIT 1
	`, chatID, channelID).Scan(&applicationID)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("未找到该频道的待处理申请")
		}
		return err
	}

	// 更新申请的用户ID
	_, err = db.conn.Exec(`
		UPDATE channel_applications
		SET user_id = ?
		WHERE id = ?
	`, userID, applicationID)

	return err
}

// SetUserState 设置用户状态
func (db *DB) SetUserState(userID int64, state string) error {
	_, err := db.conn.Exec(`
		INSERT INTO user_states (user_id, state, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		state = ?, updated_at = ?
	`, userID, state, time.Now(), state, time.Now())
	return err
}

// GetUserState 获取用户状态
func (db *DB) GetUserState(userID int64) (string, error) {
	var state string
	err := db.conn.QueryRow(`
		SELECT state FROM user_states
		WHERE user_id = ?
	`, userID).Scan(&state)

	if err == sql.ErrNoRows {
		return "", nil
	}
	return state, err
}

// ClearUserState 清除用户状态
func (db *DB) ClearUserState(userID int64) error {
	_, err := db.conn.Exec(`
		DELETE FROM user_states
		WHERE user_id = ?
	`, userID)
	return err
}

// UpdateChannelApplicationReason 更新频道申请的理由
func (db *DB) UpdateChannelApplicationReason(chatID, channelID int64, reason string) error {
	_, err := db.conn.Exec(`
		UPDATE channel_applications
		SET reason = ?
		WHERE chat_id = ? AND channel_id = ? AND status = 'pending'
	`, reason, chatID, channelID)
	return err
}

// GetPendingChannelApplication 获取指定频道的待处理申请
func (db *DB) GetPendingChannelApplication(chatID, channelID int64) (models.ChannelApplication, error) {
	var app models.ChannelApplication
	var lastPromptDate sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, chat_id, channel_id, user_id, reason, applied_at, status, verified_channel, last_prompt_date
		FROM channel_applications
		WHERE chat_id = ? AND channel_id = ? AND status = 'pending'
		ORDER BY applied_at DESC
		LIMIT 1
	`, chatID, channelID).Scan(
		&app.ID, &app.ChatID, &app.ChannelID, &app.UserID,
		&app.Reason, &app.AppliedAt, &app.Status, &app.VerifiedChannel, &lastPromptDate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return models.ChannelApplication{}, nil
		}
		return models.ChannelApplication{}, err
	}

	// 处理可能为NULL的last_prompt_date
	if lastPromptDate.Valid {
		t, err := time.Parse("2006-01-02", lastPromptDate.String)
		if err == nil {
			app.LastPromptDate = t
		}
	}

	return app, nil
}

// 定义提示类型常量
const (
	PromptTypeWhitelistWarning = "whitelist_warning" // 非白名单提示（需要申请）
	PromptTypePendingNotice    = "pending_notice"    // 待审核提示
)

// HasChannelDailyPrompt 检查指定频道在当天是否已经有过特定类型的提示
func (db *DB) HasChannelDailyPrompt(chatID, channelID int64, promptType string) (bool, error) {
	today := time.Now().Format("2006-01-02")

	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) 
		FROM channel_daily_prompts 
		WHERE chat_id = ? AND channel_id = ? AND prompt_type = ? AND prompt_date = ?
	`, chatID, channelID, promptType, today).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RecordChannelDailyPrompt 记录指定频道的每日提示
func (db *DB) RecordChannelDailyPrompt(chatID, channelID int64, promptType string) error {
	today := time.Now().Format("2006-01-02")

	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO channel_daily_prompts 
		(chat_id, channel_id, prompt_type, prompt_date) 
		VALUES (?, ?, ?, ?)
	`, chatID, channelID, promptType, today)

	return err
}

// HasPromptedToday 检查今天是否已经提示过，并返回提示状态
// 默认检查whitelist_warning类型的提示（非白名单提示）
func (db *DB) HasPromptedToday(chatID, channelID int64) (bool, error) {
	return db.HasChannelDailyPrompt(chatID, channelID, PromptTypeWhitelistWarning)
}

// HasPendingNoticeToday 检查今天是否已经提示过待审核通知
func (db *DB) HasPendingNoticeToday(chatID, channelID int64) (bool, error) {
	return db.HasChannelDailyPrompt(chatID, channelID, PromptTypePendingNotice)
}

// RecordPrompt 记录今天已经提示过（默认记录whitelist_warning类型）
func (db *DB) RecordPrompt(chatID, channelID int64) error {
	return db.RecordChannelDailyPrompt(chatID, channelID, PromptTypeWhitelistWarning)
}

// RecordPendingNotice 记录今天已经提示过待审核通知
func (db *DB) RecordPendingNotice(chatID, channelID int64) error {
	return db.RecordChannelDailyPrompt(chatID, channelID, PromptTypePendingNotice)
}

// ResetDailyPrompts 重置每日提示状态（每天凌晨调用）
func (db *DB) ResetDailyPrompts() error {
	// 删除非今日的提示记录
	today := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(`
		DELETE FROM channel_daily_prompts 
		WHERE prompt_date != ?
	`, today)

	// 同时重置application表中的提示状态（向后兼容）
	if err == nil {
		_, err = db.conn.Exec(`
			UPDATE channel_applications 
			SET prompted_today = 0 
			WHERE status = 'pending'
		`)
	}

	return err
}

// HasPendingApplication 检查是否有待处理的申请
func (db *DB) HasPendingApplication(chatID, channelID int64) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) 
		FROM channel_applications 
		WHERE chat_id = ? AND channel_id = ? AND status = 'pending'
	`, chatID, channelID).Scan(&count)

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BeginTx 开始一个数据库事务
func (db *DB) BeginTx() (*sql.Tx, error) {
	return db.conn.Begin()
}

// LogBlockedMessagesBatch 批量记录被阻止的消息
func (db *DB) LogBlockedMessagesBatch(tx *sql.Tx, messages []models.BlockedMessageInfo) bool {
	stmt, err := tx.Prepare(`
		INSERT INTO blocked_messages (chat_id, channel_id, message_id, blocked_at, message_text)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return false
	}
	defer stmt.Close()

	for _, msg := range messages {
		_, err := stmt.Exec(msg.ChatID, msg.ChannelID, msg.MessageID, time.Now(), msg.MessageText)
		if err != nil {
			return false
		}
	}

	return true
}

package handlers

import (
	"sync"

	"github.com/anhe/tg-whitelist-bot/config"
	"github.com/anhe/tg-whitelist-bot/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler 消息处理器
type Handler struct {
	Bot        *tgbotapi.BotAPI
	DB         *db.DB
	Config     *config.Config
	CommandMap map[string]func(message *tgbotapi.Message, args string) error

	// 消息队列和保护锁
	messageQueue     []blockedMessageInfo
	messageQueueLock sync.Mutex
}

// 被阻止的消息信息
type blockedMessageInfo struct {
	ChatID      int64
	ChannelID   int64
	MessageID   int
	MessageText string
}

// New 创建一个新的处理器
func New(bot *tgbotapi.BotAPI, db *db.DB, config *config.Config) *Handler {
	h := &Handler{
		Bot:              bot,
		DB:               db,
		Config:           config,
		CommandMap:       make(map[string]func(message *tgbotapi.Message, args string) error),
		messageQueue:     []blockedMessageInfo{},
		messageQueueLock: sync.Mutex{},
	}

	// 初始化命令映射
	h.CommandMap["start"] = h.HandleStart
	h.CommandMap["help"] = h.HandleHelp
	h.CommandMap["whitelist"] = h.HandleAddChannel
	h.CommandMap["wl"] = h.HandleAddChannel
	h.CommandMap["unwhitelist"] = h.HandleUnwhitelist
	h.CommandMap["unwl"] = h.HandleUnwhitelist
	h.CommandMap["list_channels"] = h.HandleListChannels
	h.CommandMap["stats"] = h.HandleStats
	h.CommandMap["enable"] = h.HandleEnable
	h.CommandMap["disable"] = h.HandleDisable
	h.CommandMap["settings"] = h.HandleSettings
	h.CommandMap["approve"] = h.HandleApprove
	h.CommandMap["reject"] = h.HandleReject
	h.CommandMap["apply"] = h.HandleApply
	h.CommandMap["claim"] = h.HandleClaim

	// 设置命令映射
	h.SetupCommands()

	// 启动批量处理goroutine
	go h.processMsgQueue()

	return h
}

// HandleUpdate 处理消息更新
func (h *Handler) HandleUpdate(update tgbotapi.Update) error {
	// 处理命令
	if update.Message != nil && update.Message.IsCommand() {
		return h.HandleCommand(update.Message)
	}

	// 处理频道发送的命令
	if update.ChannelPost != nil && update.ChannelPost.IsCommand() {
		return h.HandleCommand(update.ChannelPost)
	}

	// 处理普通消息
	if update.Message != nil {
		return h.HandleMessage(update.Message)
	}

	// 处理频道发送的普通消息
	if update.ChannelPost != nil {
		return h.HandleMessage(update.ChannelPost)
	}

	// 处理回调查询
	if update.CallbackQuery != nil {
		return h.HandleCallbackQuery(update.CallbackQuery)
	}

	return nil
}

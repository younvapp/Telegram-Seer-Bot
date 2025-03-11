package handlers

import (
	"fmt"

	"github.com/anhe/tg-whitelist-bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SetupCommands 设置机器人的命令列表
func (h *Handler) SetupCommands() {
	// 所有用户可见的命令
	publicCommands := []tgbotapi.BotCommand{
		{
			Command:     "help",
			Description: "显示帮助信息",
		},
		{
			Command:     "list_channels",
			Description: "列出白名单中的频道",
		},
		{
			Command:     "stats",
			Description: "显示统计信息",
		},
		{
			Command:     "apply",
			Description: "申请频道发言权限（需提供理由）",
		},
		{
			Command:     "claim",
			Description: "认领频道申请（由个人账号发送）",
		},
	}

	// 管理员可见的命令
	adminCommands := []tgbotapi.BotCommand{
		{
			Command:     "whitelist",
			Description: "将频道添加到白名单（简写：/wl）",
		},
		{
			Command:     "unwhitelist",
			Description: "将频道从白名单移除（简写：/unwl）",
		},
		{
			Command:     "enable",
			Description: "启用机器人",
		},
		{
			Command:     "disable",
			Description: "禁用机器人",
		},
		{
			Command:     "settings",
			Description: "配置群组设置",
		},
		{
			Command:     "approve",
			Description: "批准频道申请",
		},
		{
			Command:     "reject",
			Description: "拒绝频道申请",
		},
	}

	// 设置所有用户可见的命令
	setMyCommandsConfig := tgbotapi.NewSetMyCommands(publicCommands...)
	_, err := h.Bot.Request(setMyCommandsConfig)
	if err != nil {
		// 设置命令失败，仅记录错误
		fmt.Printf("设置公共命令失败: %s\n", err.Error())
	}

	// 尝试为管理员单独设置命令，如果API支持的话
	fullCommandList := append(publicCommands, adminCommands...)
	for _, adminID := range h.Config.AdminUsers {
		adminScope := tgbotapi.NewBotCommandScopeChat(adminID)
		adminCommandsConfig := tgbotapi.NewSetMyCommandsWithScope(adminScope, fullCommandList...)
		_, err := h.Bot.Request(adminCommandsConfig)
		if err != nil {
			// 如果API不支持针对用户的命令范围，仅记录错误
			fmt.Printf("为管理员 %d 设置命令失败: %s\n", adminID, err.Error())
		}
	}
}

// HandleCommand 处理命令消息
func (h *Handler) HandleCommand(message *tgbotapi.Message) error {
	// 获取命令和参数
	command := message.Command()
	args := message.CommandArguments()

	// 检查是否是频道消息
	if utils.IsChannelMessage(message) && message.Chat.Type != "private" {
		// 获取频道ID
		channelID := utils.GetChannelID(message)

		// 检查频道是否在白名单中
		isWhitelisted, err := h.DB.IsChannelWhitelisted(message.Chat.ID, channelID)
		if err != nil {
			return err
		}

		// 如果不在白名单中且不是apply命令，删除消息并返回
		if !isWhitelisted && command != "apply" {
			// 删除消息
			go h.deleteMessageWithTimeout(message.Chat.ID, message.MessageID)

			// 记录被阻止的消息
			go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)

			return nil
		}
	}

	// 特殊命令 /apply 和 /claim 无需艾特机器人也可使用
	if command == "apply" || command == "claim" {
		handler, exists := h.CommandMap[command]
		if exists {
			return handler(message, args)
		}
	}

	// 只处理艾特机器人的指令或私聊中的指令
	if message.Chat.Type != "private" {
		// 检查是否艾特了机器人
		if !utils.IsMentioningBot(message, h.Bot.Self.UserName) {
			return nil
		}
	}

	handler, exists := h.CommandMap[command]
	if !exists {
		return h.HandleUnknownCommand(message)
	}

	return handler(message, args)
}

// HandleUnknownCommand 处理未知命令
func (h *Handler) HandleUnknownCommand(message *tgbotapi.Message) error {
	text := "未知命令。使用 /help 查看所有可用命令。"
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err := h.Bot.Send(msg)
	return err
}

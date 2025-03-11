package handlers

import (
	"fmt"

	"github.com/anhe/tg-whitelist-bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleAddChannel 添加频道到白名单，支持回复消息或提供频道ID
func (h *Handler) HandleAddChannel(message *tgbotapi.Message, args string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查权限
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	isAdmin, err := utils.IsAdmin(h.Bot, message.Chat.ID, message.From.ID)
	if err != nil {
		return err
	}

	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)

	if !isGlobalAdmin && settings.AdminOnly && !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有群组管理员可以管理白名单")
		_, err := h.Bot.Send(msg)
		return err
	}

	var channelID int64

	// 检查是否是回复消息
	if message.ReplyToMessage != nil {
		// 检查回复的消息是否来自频道
		if !utils.IsChannelMessage(message.ReplyToMessage) {
			msg := tgbotapi.NewMessage(message.Chat.ID, "只能将频道添加到白名单，回复的消息不是来自频道")
			_, err := h.Bot.Send(msg)
			return err
		}

		// 获取频道ID
		channelID = utils.GetChannelID(message.ReplyToMessage)
	} else if args != "" {
		// 尝试从参数中解析频道ID
		var err error
		channelID, err = utils.ParseChannelID(args)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("无效的频道ID: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}
	} else {
		// 既没有回复消息也没有提供参数
		msg := tgbotapi.NewMessage(message.Chat.ID, "请回复一条频道消息或提供频道ID来将该频道添加到白名单")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查频道是否已在白名单中
	isWhitelisted, err := h.DB.IsChannelWhitelisted(message.Chat.ID, channelID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查频道白名单状态失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	if isWhitelisted {
		msg := tgbotapi.NewMessage(message.Chat.ID, "该频道已在白名单中")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 添加频道到白名单
	err = h.DB.AddChannelToWhitelist(message.Chat.ID, channelID, message.From.ID, "")
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("添加频道到白名单失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 获取频道名称
	channelName := h.getChannelName(channelID)

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已将频道「%s」添加到白名单", channelName))
	_, err = h.Bot.Send(msg)
	return err
}

// HandleUnwhitelist 移除频道白名单，支持回复消息或提供频道ID
func (h *Handler) HandleUnwhitelist(message *tgbotapi.Message, args string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查权限
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	isAdmin, err := utils.IsAdmin(h.Bot, message.Chat.ID, message.From.ID)
	if err != nil {
		return err
	}

	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)

	if !isGlobalAdmin && settings.AdminOnly && !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有群组管理员可以管理白名单")
		_, err := h.Bot.Send(msg)
		return err
	}

	var channelID int64

	// 检查是否是回复消息
	if message.ReplyToMessage != nil {
		// 检查回复的消息是否来自频道
		if !utils.IsChannelMessage(message.ReplyToMessage) {
			msg := tgbotapi.NewMessage(message.Chat.ID, "只能将频道从白名单移除，回复的消息不是来自频道")
			_, err := h.Bot.Send(msg)
			return err
		}

		// 获取频道ID
		channelID = utils.GetChannelID(message.ReplyToMessage)
	} else if args != "" {
		// 尝试从参数中解析频道ID
		var err error
		channelID, err = utils.ParseChannelID(args)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("无效的频道ID: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}
	} else {
		// 既没有回复消息也没有提供参数
		msg := tgbotapi.NewMessage(message.Chat.ID, "请回复一条频道消息或提供频道ID来将该频道从白名单移除")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查频道是否在白名单中
	isWhitelisted, err := h.DB.IsChannelWhitelisted(message.Chat.ID, channelID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查频道白名单状态失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	if !isWhitelisted {
		msg := tgbotapi.NewMessage(message.Chat.ID, "该频道不在白名单中")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 从白名单中移除频道
	err = h.DB.RemoveChannelFromWhitelist(message.Chat.ID, channelID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("从白名单移除频道失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 获取频道名称
	channelName := h.getChannelName(channelID)

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已将频道「%s」从白名单移除", channelName))
	_, err = h.Bot.Send(msg)
	return err
}

// HandleEnable 启用机器人
func (h *Handler) HandleEnable(message *tgbotapi.Message, _ string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查权限
	isAdmin, err := utils.IsAdmin(h.Bot, message.Chat.ID, message.From.ID)
	if err != nil {
		return err
	}

	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)

	if !isGlobalAdmin && !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有群组管理员可以使用此命令")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取群组设置
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	// 启用机器人
	settings.Enabled = true
	err = h.DB.UpdateGroupSettings(settings)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("启用机器人失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "机器人已启用")
	_, err = h.Bot.Send(msg)
	return err
}

// HandleDisable 禁用机器人
func (h *Handler) HandleDisable(message *tgbotapi.Message, _ string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查权限
	isAdmin, err := utils.IsAdmin(h.Bot, message.Chat.ID, message.From.ID)
	if err != nil {
		return err
	}

	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)

	if !isGlobalAdmin && !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有群组管理员可以使用此命令")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取群组设置
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	// 禁用机器人
	settings.Enabled = false
	err = h.DB.UpdateGroupSettings(settings)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("禁用机器人失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "机器人已禁用")
	_, err = h.Bot.Send(msg)
	return err
}

// HandleSettings 配置群组设置
func (h *Handler) HandleSettings(message *tgbotapi.Message, _ string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取群组设置
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	// 格式化设置信息
	enabledStatus := "启用"
	if !settings.Enabled {
		enabledStatus = "禁用"
	}

	adminOnlyStatus := "仅管理员"
	if !settings.AdminOnly {
		adminOnlyStatus = "所有成员"
	}

	logChannelText := "未设置"
	if settings.LogChannelID != 0 {
		logChannelText = fmt.Sprintf("%d", settings.LogChannelID)
	}

	text := fmt.Sprintf("⚙️ 当前设置:\n\n"+
		"状态: %s\n"+
		"管理权限: %s\n"+
		"日志频道: %s\n", enabledStatus, adminOnlyStatus, logChannelText)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err = h.Bot.Send(msg)
	return err
}

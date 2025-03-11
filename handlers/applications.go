package handlers

import (
	"fmt"

	"github.com/anhe/tg-whitelist-bot/db/models"
	"github.com/anhe/tg-whitelist-bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleApply 申请频道发言权限
func (h *Handler) HandleApply(message *tgbotapi.Message, args string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 检查是否是频道发送的消息
	if !utils.IsChannelMessage(message) {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能由频道直接发送")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取频道ID
	channelID := utils.GetChannelID(message)

	// 检查是否已有待处理的申请
	pendingApp, err := h.DB.GetPendingChannelApplication(message.Chat.ID, channelID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查频道申请状态失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 如果有待处理的申请
	if pendingApp.ID != 0 {
		// 检查今天是否已经提示过
		hasNoticed, err := h.DB.HasPendingNoticeToday(message.Chat.ID, channelID)
		if err != nil {
			return err
		}

		// 如果今天已经提示过，直接删除消息
		if hasNoticed {
			go h.deleteMessageWithTimeout(message.Chat.ID, message.MessageID)
			return nil
		}

		// 第一次或新的一天，提示"待审核"
		promptText := fmt.Sprintf("频道「%s」已有一个待处理的申请，请等待管理员审核。", h.getChannelName(channelID))
		promptMsg := tgbotapi.NewMessage(message.Chat.ID, promptText)
		if _, err := h.Bot.Send(promptMsg); err == nil {
			// 记录已提示过"待审核"
			h.DB.RecordPendingNotice(message.Chat.ID, channelID)
		}
		return nil
	}

	// 检查频道是否在白名单中
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

	// 获取频道名称或ID字符串
	channelName := "未知频道"
	if message.SenderChat != nil && message.SenderChat.Title != "" {
		channelName = message.SenderChat.Title
	} else {
		channelName = fmt.Sprintf("ID: %d", channelID)
	}

	// 创建申请 (userID设为0，表示是频道申请，尚未有个人账号认领)
	err = h.DB.CreateChannelApplication(message.Chat.ID, channelID, 0, args)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("申请频道发言权限失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 创建认领按钮
	claimButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("认领此申请", fmt.Sprintf("claim:%d:%d", message.Chat.ID, channelID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("私聊认领", fmt.Sprintf("https://t.me/%s?start=claim_%d_%d", h.Bot.Self.UserName, message.Chat.ID, channelID)),
		),
	)

	// 发送申请提示
	var verifyText string
	if args != "" {
		verifyText = fmt.Sprintf("频道「%s」正在申请发言权限。\n\n申请理由: %s\n\n如果您是此频道的所有者，请点击下方按钮认领此申请，管理员审核通过后即可发言。", channelName, args)
	} else {
		verifyText = fmt.Sprintf("频道「%s」正在申请发言权限。\n\n如果您是此频道的所有者，请点击下方按钮认领此申请，管理员审核通过后即可发言。", channelName)
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, verifyText)
	msg.ReplyMarkup = claimButton
	_, err = h.Bot.Send(msg)

	return err
}

// HandleClaim 认领频道申请
func (h *Handler) HandleClaim(message *tgbotapi.Message, args string) error {
	// 解析频道ID
	var channelID int64
	_, err := fmt.Sscanf(args, "%d", &channelID)
	if err != nil || channelID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "请提供有效的频道ID，格式：/claim 频道ID")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 查找该频道的申请
	applications, err := h.DB.GetPendingApplications()
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("查询申请失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	var targetApp models.ChannelApplication
	var targetChatID int64
	for _, app := range applications {
		if app.ChannelID == channelID && app.UserID == 0 { // UserID为0表示尚未认领
			targetApp = app
			targetChatID = app.ChatID
			break
		}
	}

	if targetApp.ID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "未找到该频道的待处理申请，或申请已被其他人认领")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取频道名称
	channelName := "未知频道"
	channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: targetApp.ChannelID,
		},
	})
	if err == nil && channelChat.Title != "" {
		channelName = channelChat.Title
	} else {
		channelName = fmt.Sprintf("ID: %d", targetApp.ChannelID)
	}

	// 获取群组名称
	groupName := "未知群组"
	groupChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: targetChatID,
		},
	})
	if err == nil && groupChat.Title != "" {
		groupName = groupChat.Title
	}

	// 获取用户信息
	userName := message.From.FirstName
	if message.From.LastName != "" {
		userName += " " + message.From.LastName
	}
	userInfo := fmt.Sprintf("%s (ID: %d)", userName, message.From.ID)
	if message.From.UserName != "" {
		userInfo += " @" + message.From.UserName
	}

	// 检查是否有申请理由
	if targetApp.Reason == "" {
		// 如果没有理由，且不是在私聊中，则提示用户需要私聊认领
		if message.Chat.Type != "private" {
			// 创建私聊认领按钮
			privateClaimButton := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("私聊认领并添加理由", fmt.Sprintf("https://t.me/%s?start=claim_%d_%d", h.Bot.Self.UserName, targetChatID, channelID)),
				),
			)

			// 发送提示消息
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"没有申请理由的申请只能通过私聊方式认领。\n\n请点击下方按钮前往私聊认领此申请并添加理由。")
			msg.ReplyMarkup = privateClaimButton
			_, err = h.Bot.Send(msg)
			return err
		}

		// 如果在私聊中，提示用户输入理由
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("您正在认领群组「%s」内频道「%s」的发言申请，但该申请没有提供理由。\n\n请回复您申请发言的理由，管理员将根据您的理由进行审核。",
				groupName, channelName))

		// 将用户状态设置为等待输入理由
		h.DB.SetUserState(message.From.ID, fmt.Sprintf("waiting_reason:%d:%d", targetChatID, channelID))

		_, err = h.Bot.Send(msg)
		return err
	}

	// 如果需要验证频道所有权
	if h.Config.RequireRealAccountVerification {
		// 创建确认按钮
		confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("确认认领", fmt.Sprintf("confirm_channel:%d:%d", targetChatID, targetApp.ChannelID)),
			),
		)
		// 发送确认消息
		confirmText := fmt.Sprintf("用户「%s」\n\n您是否确认认领频道「%s」(ID: %d) 的申请？\n\n请确保您是该频道的所有者，认领后管理员将收到您的申请并进行审核。", userInfo, channelName, targetApp.ChannelID)
		msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
		msg.ReplyMarkup = confirmKeyboard
		_, err = h.Bot.Send(msg)
		return err
	} else {
		// 不需要验证，直接更新申请
		err = h.DB.UpdateChannelApplicationUser(targetChatID, targetApp.ChannelID, message.From.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("认领申请失败: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}

		// 验证频道所有权
		err = h.DB.VerifyChannelOwnership(targetChatID, targetApp.ChannelID, message.From.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("验证频道所有权失败: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}

		// 获取频道名称
		channelName := "未知频道"
		channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
			ChatConfig: tgbotapi.ChatConfig{
				ChatID: targetApp.ChannelID,
			},
		})
		if err == nil && channelChat.Title != "" {
			channelName = channelChat.Title
		} else {
			channelName = fmt.Sprintf("ID: %d", targetApp.ChannelID)
		}

		// 通知管理员
		err = h.notifyAdminsAboutApplication(targetChatID, targetApp.ChannelID, message.From.ID, channelName, targetApp.Reason)
		if err != nil {
			return err
		}

		// 发送确认消息
		confirmText := fmt.Sprintf("您已成功认领频道「%s」的申请，管理员将尽快审核", channelName)
		msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
		_, err = h.Bot.Send(msg)

		// 获取用户信息
		userName := message.From.FirstName
		if message.From.LastName != "" {
			userName += " " + message.From.LastName
		}
		userInfo := fmt.Sprintf("%s (ID: %d)", userName, message.From.ID)
		if message.From.UserName != "" {
			userInfo += " @" + message.From.UserName
		}

		// 通知群组该频道已认领
		groupMsg := tgbotapi.NewMessage(targetChatID, fmt.Sprintf("频道「%s」的申请已被用户「%s」认领，管理员将尽快审核", channelName, userInfo))
		_, _ = h.Bot.Send(groupMsg)

		return err
	}
}

// HandleApprove 批准频道申请
func (h *Handler) HandleApprove(message *tgbotapi.Message, args string) error {
	// 只允许私聊使用
	if message.Chat.Type != "private" {
		return nil
	}

	// 检查是否是管理员
	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)
	if !isGlobalAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有管理员可以批准申请")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 解析频道ID
	var channelID int64
	_, err := fmt.Sscanf(args, "%d", &channelID)
	if err != nil || channelID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "请提供有效的频道ID，格式：/approve 频道ID")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取所有待处理的申请
	applications, err := h.DB.GetPendingApplications()
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取申请失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 查找对应的申请
	var targetApp models.ChannelApplication
	for _, app := range applications {
		if app.ChannelID == channelID && app.VerifiedChannel {
			targetApp = app
			break
		}
	}

	if targetApp.ID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "未找到该频道的待处理申请或该申请未经过认领验证")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 添加频道到白名单
	err = h.DB.AddChannelToWhitelist(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, targetApp.Reason)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("添加频道到白名单失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 更新申请状态
	err = h.DB.UpdateChannelApplicationStatus(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, "approved")
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("更新申请状态失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 获取频道名称
	channelName := "未知频道"
	channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: targetApp.ChannelID,
		},
	})
	if err == nil && channelChat.Title != "" {
		channelName = channelChat.Title
	} else {
		channelName = fmt.Sprintf("ID: %d", targetApp.ChannelID)
	}

	// 通知申请人
	notifyText := fmt.Sprintf("您对频道「%s」的发言申请已被批准", channelName)
	notifyMsg := tgbotapi.NewMessage(targetApp.UserID, notifyText)
	_, _ = h.Bot.Send(notifyMsg)

	// 通知群组
	groupNotifyText := fmt.Sprintf("频道「%s」的发言申请已被批准", channelName)
	groupMsg := tgbotapi.NewMessage(targetApp.ChatID, groupNotifyText)
	_, _ = h.Bot.Send(groupMsg)

	// 回复管理员
	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已批准频道「%s」的发言申请", channelName))
	_, err = h.Bot.Send(msg)
	return err
}

// HandleReject 拒绝频道申请
func (h *Handler) HandleReject(message *tgbotapi.Message, args string) error {
	// 只允许私聊使用
	if message.Chat.Type != "private" {
		return nil
	}

	// 检查是否是管理员
	isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, message.From.ID)
	if !isGlobalAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "只有管理员可以拒绝申请")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 解析频道ID
	var channelID int64
	_, err := fmt.Sscanf(args, "%d", &channelID)
	if err != nil || channelID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "请提供有效的频道ID，格式：/reject 频道ID")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取所有待处理的申请
	applications, err := h.DB.GetPendingApplications()
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取申请失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 查找对应的申请
	var targetApp models.ChannelApplication
	for _, app := range applications {
		if app.ChannelID == channelID && app.VerifiedChannel {
			targetApp = app
			break
		}
	}

	if targetApp.ID == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "未找到该频道的待处理申请或该申请未经过认领验证")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 更新申请状态
	err = h.DB.UpdateChannelApplicationStatus(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, "rejected")
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("更新申请状态失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 获取频道名称
	channelName := "未知频道"
	channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: targetApp.ChannelID,
		},
	})
	if err == nil && channelChat.Title != "" {
		channelName = channelChat.Title
	} else {
		channelName = fmt.Sprintf("ID: %d", targetApp.ChannelID)
	}

	// 通知申请人
	notifyText := fmt.Sprintf("您对频道「%s」的发言申请已被拒绝", channelName)
	notifyMsg := tgbotapi.NewMessage(targetApp.UserID, notifyText)
	_, _ = h.Bot.Send(notifyMsg)

	// 通知群组
	groupNotifyText := fmt.Sprintf("频道「%s」的发言申请已被拒绝", channelName)
	groupMsg := tgbotapi.NewMessage(targetApp.ChatID, groupNotifyText)
	_, _ = h.Bot.Send(groupMsg)

	// 回复管理员
	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已拒绝频道「%s」的发言申请", channelName))
	_, err = h.Bot.Send(msg)
	return err
}

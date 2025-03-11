package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// getChannelName 获取频道名称的辅助函数
func (h *Handler) getChannelName(channelID int64) string {
	channelName := "未知频道"
	channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: channelID,
		},
	})
	if err == nil && channelChat.Title != "" {
		channelName = channelChat.Title
	} else {
		channelName = fmt.Sprintf("ID: %d", channelID)
	}
	return channelName
}

// notifyAdminsAboutApplication 通知管理员有新的申请
func (h *Handler) notifyAdminsAboutApplication(chatID, channelID, userID int64, channelName, reason string) error {
	// 获取群组管理员
	admins, err := h.Bot.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	})
	if err != nil {
		return err
	}

	// 获取申请用户信息
	userChatConfig := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: userID,
		},
	}
	user, err := h.Bot.GetChat(userChatConfig)
	if err != nil {
		return err
	}

	// 获取群组信息
	chatConfig := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}
	chat, err := h.Bot.GetChat(chatConfig)
	if err != nil {
		return err
	}

	// 构建通知消息
	userName := user.FirstName
	if user.LastName != "" {
		userName += " " + user.LastName
	}
	if user.UserName != "" {
		userName += " (@" + user.UserName + ")"
	}

	notifyText := fmt.Sprintf("新的频道发言申请:\n\n"+
		"群组: %s\n"+
		"频道: %s (ID: %d)\n"+
		"申请人: %s\n"+
		"申请人ID: %d\n"+
		"申请理由: %s\n\n"+
		"请点击下方按钮批准或拒绝此申请",
		chat.Title, channelName, channelID, userName, userID, reason)

	// 创建确认/拒绝按钮
	approveButton := tgbotapi.NewInlineKeyboardButtonData("✅ 批准", fmt.Sprintf("approve:%d:%d", chatID, channelID))
	rejectButton := tgbotapi.NewInlineKeyboardButtonData("❌ 拒绝", fmt.Sprintf("reject:%d:%d", chatID, channelID))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(approveButton, rejectButton),
	)

	// 通知所有管理员
	for _, admin := range admins {
		if !admin.User.IsBot {
			msg := tgbotapi.NewMessage(admin.User.ID, notifyText)
			msg.ReplyMarkup = keyboard
			_, _ = h.Bot.Send(msg)
		}
	}

	// 通知全局管理员
	for _, adminID := range h.Config.AdminUsers {
		// 避免重复通知
		alreadyNotified := false
		for _, admin := range admins {
			if admin.User.ID == adminID {
				alreadyNotified = true
				break
			}
		}

		if !alreadyNotified {
			msg := tgbotapi.NewMessage(adminID, notifyText)
			msg.ReplyMarkup = keyboard
			_, _ = h.Bot.Send(msg)
		}
	}

	return nil
}

package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anhe/tg-whitelist-bot/db/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleStart 启动机器人
func (h *Handler) HandleStart(message *tgbotapi.Message, args string) error {
	// 检查是否是认领请求
	if strings.HasPrefix(args, "claim_") {
		parts := strings.Split(args, "_")
		if len(parts) == 3 {
			// 尝试解析群组ID和频道ID
			chatID, err1 := strconv.ParseInt(parts[1], 10, 64)
			channelID, err2 := strconv.ParseInt(parts[2], 10, 64)

			if err1 == nil && err2 == nil {
				// 获取申请信息
				applications, err := h.DB.GetPendingApplications()
				if err != nil {
					msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("查询申请失败: %s", err.Error()))
					_, _ = h.Bot.Send(msg)
					return err
				}

				var targetApp models.ChannelApplication
				for _, app := range applications {
					if app.ChatID == chatID && app.ChannelID == channelID && app.UserID == 0 {
						targetApp = app
						break
					}
				}

				if targetApp.ID == 0 {
					msg := tgbotapi.NewMessage(message.Chat.ID, "未找到该频道的待处理申请，或申请已被其他人认领")
					_, err := h.Bot.Send(msg)
					return err
				}

				// 获取群组名称
				groupName := "未知群组"
				groupChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
					ChatConfig: tgbotapi.ChatConfig{
						ChatID: chatID,
					},
				})
				if err == nil && groupChat.Title != "" {
					groupName = groupChat.Title
				}

				// 获取频道名称
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
					// 如果没有理由，提示用户输入理由
					msg := tgbotapi.NewMessage(message.Chat.ID,
						fmt.Sprintf("您正在认领群组「%s」内频道「%s」的发言申请，但该申请没有提供理由。\n\n请回复您申请发言的理由，管理员将根据您的理由进行审核。",
							groupName, channelName))

					// 将用户状态设置为等待输入理由
					h.DB.SetUserState(message.From.ID, fmt.Sprintf("waiting_reason:%d:%d", chatID, channelID))

					_, err = h.Bot.Send(msg)
					return err
				}

				// 创建确认按钮
				confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("确认认领", fmt.Sprintf("confirm_claim:%d:%d", chatID, channelID)),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("取消", "cancel_claim"),
					),
				)

				// 发送确认消息
				confirmText := fmt.Sprintf("用户「%s」\n\n您是否要认领群组「%s」内频道「%s」的发言申请？\n\n请确保您是该频道的所有者，认领后管理员将收到您的申请并进行审核。", userInfo, groupName, channelName)
				msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
				msg.ReplyMarkup = confirmKeyboard
				_, err = h.Bot.Send(msg)
				return err
			}
		}
	}

	// 常规开始消息
	text := "👋 你好！我是Telegram-Seer-Bot。\n\n" +
		"我可以帮助你管理群组中频道的消息，只允许白名单中的频道发言。\n\n" +
		"使用 /help 查看所有可用命令。"

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err := h.Bot.Send(msg)
	return err
}

// HandleHelp 显示帮助信息
func (h *Handler) HandleHelp(message *tgbotapi.Message, _ string) error {
	// 尝试使用普通文本格式，避免格式错误
	plainText := "📖 Telegram-Seer-Bot 使用帮助\n\n" +
		"基本命令:\n" +
		"/help - 显示帮助信息\n" +
		"/list_channels - 列出白名单中的频道\n" +
		"/stats - 显示统计信息\n\n" +
		"申请命令（由频道直接发送）:\n" +
		"/apply [理由] - 申请频道发言权限（必须提供理由才能在群内认领）\n\n" +
		"认领命令（由个人账号发送）:\n" +
		"/claim [频道ID] - 认领频道申请\n\n" +
		"管理员命令:\n" +
		"/whitelist 或 /wl - 将频道添加到白名单\n" +
		"/unwhitelist 或 /unwl - 将频道从白名单移除\n" +
		"/approve [频道ID] - 批准频道申请\n" +
		"/reject [频道ID] - 拒绝频道申请\n" +
		"/enable - 启用机器人\n" +
		"/disable - 禁用机器人\n" +
		"/settings - 配置群组设置\n\n" +
		"📌 项目地址: https://github.com/younvapp/Telegram-Seer-Bot"

	plainMsg := tgbotapi.NewMessage(message.Chat.ID, plainText)
	_, err := h.Bot.Send(plainMsg)
	return err
}

// HandleListChannels 列出白名单频道
func (h *Handler) HandleListChannels(message *tgbotapi.Message, _ string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取白名单频道列表
	channels, err := h.DB.GetWhitelistedChannels(message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取白名单频道失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	// 格式化频道列表
	var text string
	if len(channels) == 0 {
		text = "白名单中没有频道"
	} else {
		text = "📋 白名单频道列表:\n\n"
		for i, channel := range channels {
			addTime := channel.AddedAt.Format("2006-01-02 15:04:05")

			// 获取频道名称
			channelName := "未知频道"
			channelChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
				ChatConfig: tgbotapi.ChatConfig{
					ChatID: channel.ChannelID,
				},
			})

			if err == nil && channelChat.Title != "" {
				channelName = channelChat.Title
			}

			text += fmt.Sprintf("%d. 频道「%s」(ID: %d)\n    添加时间: %s\n",
				i+1, channelName, channel.ChannelID, addTime)

			if channel.Description != "" {
				text += fmt.Sprintf("    描述: %s\n", channel.Description)
			}

			// 添加分隔符
			if i < len(channels)-1 {
				text += "\n"
			}
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err = h.Bot.Send(msg)
	return err
}

// HandleStats 显示统计信息
func (h *Handler) HandleStats(message *tgbotapi.Message, _ string) error {
	// 只在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "此命令只能在群组中使用")
		_, err := h.Bot.Send(msg)
		return err
	}

	// 获取统计信息
	blockedCount, err := h.DB.GetBlockedMessagesStats(message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取统计信息失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	channelsList, err := h.DB.GetWhitelistedChannels(message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取白名单频道失败: %s", err.Error()))
		_, _ = h.Bot.Send(msg)
		return err
	}

	text := fmt.Sprintf("📊 统计信息:\n\n"+
		"白名单频道数量: %d\n"+
		"已阻止消息数量: %d\n", len(channelsList), blockedCount)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err = h.Bot.Send(msg)
	return err
}

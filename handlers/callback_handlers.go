package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anhe/tg-whitelist-bot/db/models"
	"github.com/anhe/tg-whitelist-bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCallbackQuery 处理回调查询
func (h *Handler) HandleCallbackQuery(query *tgbotapi.CallbackQuery) error {
	// 解析回调数据
	data := query.Data

	// 处理频道认领按钮点击
	if strings.HasPrefix(data, "claim:") {
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			return fmt.Errorf("无效的回调数据: %s", data)
		}

		// 解析群组ID和频道ID
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}

		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}

		// 获取申请信息
		applications, err := h.DB.GetPendingApplications()
		if err != nil {
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
			callback := tgbotapi.NewCallback(query.ID, "未找到该频道的待处理申请，或申请已被其他人认领")
			_, _ = h.Bot.Request(callback)
			return nil
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
		userName := query.From.FirstName
		if query.From.LastName != "" {
			userName += " " + query.From.LastName
		}
		userInfo := fmt.Sprintf("%s (ID: %d)", userName, query.From.ID)
		if query.From.UserName != "" {
			userInfo += " @" + query.From.UserName
		}

		// 如果没有申请理由，强制用户去私聊认领
		if targetApp.Reason == "" {
			callback := tgbotapi.NewCallback(query.ID, "此申请没有理由，只能通过私聊方式认领")
			_, _ = h.Bot.Request(callback)

			// 创建私聊按钮
			privateButton := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("去私聊认领并添加理由", fmt.Sprintf("https://t.me/%s?start=claim_%d_%d", h.Bot.Self.UserName, chatID, channelID)),
				),
			)

			// 发送私聊提示带按钮
			promptMsg := tgbotapi.NewMessage(query.Message.Chat.ID,
				fmt.Sprintf("⚠️ 此申请没有提供理由，无法在群内认领。\n\n申请必须提供理由才能使用群内认领功能。\n\n@%s 请点击下方按钮在私聊中认领并添加理由：",
					query.From.UserName))
			promptMsg.ReplyMarkup = privateButton
			_, _ = h.Bot.Send(promptMsg)
			return nil
		}

		// 如果是在群组中点击的认领按钮
		if query.Message.Chat.Type == "group" || query.Message.Chat.Type == "supergroup" {
			// 创建二次确认按钮
			confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("确认认领", fmt.Sprintf("confirm_claim:%d:%d", chatID, channelID)),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("取消", "cancel_claim"),
				),
			)

			// 发送二次确认消息
			confirmText := fmt.Sprintf("用户「%s」\n\n您是否确认认领频道「%s」的申请？\n\n请确保您是该频道的所有者，认领后管理员将收到您的申请并进行审核。", userInfo, channelName)
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				confirmText,
			)
			editMsg.ReplyMarkup = &confirmKeyboard
			_, err = h.Bot.Send(editMsg)
			if err != nil {
				return err
			}

			// 发送回调确认
			callback := tgbotapi.NewCallback(query.ID, "请确认是否认领此申请")
			_, err = h.Bot.Request(callback)
			return err
		} else {
			// 在私聊中，创建确认按钮
			confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("确认认领", fmt.Sprintf("confirm_channel:%d:%d", chatID, channelID)),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("取消", "cancel_claim"),
				),
			)

			// 发送确认消息
			confirmText := fmt.Sprintf("用户「%s」\n\n您是否确认认领频道「%s」的申请？\n\n请确保您是该频道的所有者，认领后管理员将收到您的申请并进行审核。", userInfo, channelName)

			// 在私聊中直接发送确认消息
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				confirmText,
			)
			editMsg.ReplyMarkup = &confirmKeyboard
			_, err = h.Bot.Send(editMsg)
		}

		return err
	} else if strings.HasPrefix(data, "confirm_claim:") {
		// 处理私聊中的认领确认
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			return fmt.Errorf("无效的回调数据: %s", data)
		}

		// 解析群组ID和频道ID
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}

		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}

		// 更新申请的用户ID
		err = h.DB.UpdateChannelApplicationUser(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "认领申请失败: "+err.Error())
			_, _ = h.Bot.Request(callback)

			// 更新消息
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				fmt.Sprintf("认领申请失败: %s", err.Error()),
			)
			_, _ = h.Bot.Send(editMsg)
			return err
		}

		// 验证频道所有权
		err = h.DB.VerifyChannelOwnership(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "验证频道所有权失败")
			_, _ = h.Bot.Request(callback)
			return err
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

		// 获取群组名称 - 删除未使用的变量
		groupChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
			ChatConfig: tgbotapi.ChatConfig{
				ChatID: chatID,
			},
		})
		var groupName string
		if err == nil && groupChat.Title != "" {
			groupName = groupChat.Title
		} else {
			groupName = "未知群组"
		}

		// 获取用户信息
		userName := query.From.FirstName
		if query.From.LastName != "" {
			userName += " " + query.From.LastName
		}
		userInfo := fmt.Sprintf("%s (ID: %d)", userName, query.From.ID)
		if query.From.UserName != "" {
			userInfo += " @" + query.From.UserName
		}

		// 获取申请信息
		app, err := h.DB.GetChannelApplication(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "获取申请信息失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 通知管理员
		err = h.notifyAdminsAboutApplication(chatID, channelID, query.From.ID, channelName, app.Reason)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "通知管理员失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 发送确认消息
		callback := tgbotapi.NewCallback(query.ID, "已确认您是频道所有者，申请已提交给管理员审核")
		_, err = h.Bot.Request(callback)
		if err != nil {
			return err
		}

		// 更新消息
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			fmt.Sprintf("您已成功认领群组「%s」内频道「%s」的申请，管理员将尽快审核", groupName, channelName),
		)
		_, err = h.Bot.Send(editMsg)

		return err
	} else if data == "cancel_claim" {
		// 处理取消认领
		callback := tgbotapi.NewCallback(query.ID, "已取消认领")
		_, err := h.Bot.Request(callback)
		if err != nil {
			return err
		}

		// 更新消息
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			"您已取消认领此频道申请",
		)
		_, err = h.Bot.Send(editMsg)
		return err
	} else if strings.HasPrefix(data, "confirm_channel:") {
		// 处理频道认领确认
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			return fmt.Errorf("无效的回调数据: %s", data)
		}

		// 解析群组ID和频道ID
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}

		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}

		// 更新申请的用户ID
		err = h.DB.UpdateChannelApplicationUser(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "认领申请失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 验证频道所有权
		err = h.DB.VerifyChannelOwnership(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "验证频道所有权失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 获取申请信息
		app, err := h.DB.GetChannelApplication(chatID, channelID, query.From.ID)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "获取申请信息失败")
			_, _ = h.Bot.Request(callback)
			return err
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
		userName := query.From.FirstName
		if query.From.LastName != "" {
			userName += " " + query.From.LastName
		}
		userInfo := fmt.Sprintf("%s (ID: %d)", userName, query.From.ID)
		if query.From.UserName != "" {
			userInfo += " @" + query.From.UserName
		}

		// 通知管理员
		err = h.notifyAdminsAboutApplication(chatID, channelID, query.From.ID, channelName, app.Reason)
		if err != nil {
			// 发送错误消息
			callback := tgbotapi.NewCallback(query.ID, "通知管理员失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 发送确认消息
		callback := tgbotapi.NewCallback(query.ID, "已确认您是频道所有者，申请已提交给管理员审核")
		_, err = h.Bot.Request(callback)
		if err != nil {
			return err
		}

		// 更新消息
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			fmt.Sprintf("您已成功认领频道「%s」的申请，管理员将尽快审核", channelName),
		)
		_, err = h.Bot.Send(editMsg)

		return err
	} else if strings.HasPrefix(data, "approve:") || strings.HasPrefix(data, "reject:") {
		// 处理批准/拒绝申请
		isApprove := strings.HasPrefix(data, "approve:")

		// 解析数据
		parts := strings.Split(data, ":")
		if len(parts) != 3 {
			return fmt.Errorf("无效的回调数据: %s", data)
		}

		// 解析群组ID和频道ID
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}

		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}

		// 检查是否是管理员
		isGlobalAdmin := utils.IsGlobalAdmin(h.Config.AdminUsers, query.From.ID)
		if !isGlobalAdmin {
			isAdmin, err := utils.IsAdmin(h.Bot, chatID, query.From.ID)
			if err != nil || !isAdmin {
				callback := tgbotapi.NewCallback(query.ID, "您没有权限执行此操作")
				_, _ = h.Bot.Request(callback)
				return nil
			}
		}

		// 获取所有待处理的申请
		applications, err := h.DB.GetPendingApplications()
		if err != nil {
			callback := tgbotapi.NewCallback(query.ID, "获取申请失败")
			_, _ = h.Bot.Request(callback)
			return err
		}

		// 查找对应的申请
		var targetApp models.ChannelApplication
		for _, app := range applications {
			if app.ChatID == chatID && app.ChannelID == channelID && app.VerifiedChannel {
				targetApp = app
				break
			}
		}

		if targetApp.ID == 0 {
			callback := tgbotapi.NewCallback(query.ID, "未找到该频道的待处理申请或该申请未经过认领验证")
			_, _ = h.Bot.Request(callback)
			return nil
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

		if isApprove {
			// 添加频道到白名单
			err = h.DB.AddChannelToWhitelist(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, targetApp.Reason)
			if err != nil {
				callback := tgbotapi.NewCallback(query.ID, "添加频道到白名单失败")
				_, _ = h.Bot.Request(callback)
				return err
			}

			// 更新申请状态
			err = h.DB.UpdateChannelApplicationStatus(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, "approved")
			if err != nil {
				callback := tgbotapi.NewCallback(query.ID, "更新申请状态失败")
				_, _ = h.Bot.Request(callback)
				return err
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
			callback := tgbotapi.NewCallback(query.ID, fmt.Sprintf("已批准频道「%s」的发言申请", channelName))
			_, err = h.Bot.Request(callback)

			// 更新消息
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				fmt.Sprintf("您已批准频道「%s」的发言申请", channelName),
			)
			_, _ = h.Bot.Send(editMsg)

			return err
		} else {
			// 更新申请状态
			err = h.DB.UpdateChannelApplicationStatus(targetApp.ChatID, targetApp.ChannelID, targetApp.UserID, "rejected")
			if err != nil {
				callback := tgbotapi.NewCallback(query.ID, "更新申请状态失败")
				_, _ = h.Bot.Request(callback)
				return err
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
			callback := tgbotapi.NewCallback(query.ID, fmt.Sprintf("已拒绝频道「%s」的发言申请", channelName))
			_, err = h.Bot.Request(callback)

			// 更新消息
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				fmt.Sprintf("您已拒绝频道「%s」的发言申请", channelName),
			)
			_, _ = h.Bot.Send(editMsg)

			return err
		}
	}

	return nil
}

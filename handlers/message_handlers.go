package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anhe/tg-whitelist-bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleMessage 处理普通消息
func (h *Handler) HandleMessage(message *tgbotapi.Message) error {
	// 处理私聊消息
	if message.Chat.Type == "private" {
		return h.handlePrivateMessage(message)
	}

	// 在群组中工作
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		return nil
	}

	// 检查群组设置
	settings, err := h.DB.GetOrCreateGroupSettings(message.Chat.ID)
	if err != nil {
		return err
	}

	// 如果机器人被禁用
	if !settings.Enabled {
		return nil
	}

	// 如果是频道消息
	if utils.IsChannelMessage(message) {
		channelID := utils.GetChannelID(message)

		// 检查频道是否在白名单中
		isWhitelisted, err := h.DB.IsChannelWhitelisted(message.Chat.ID, channelID)
		if err != nil {
			return err
		}

		// 如果不在白名单中
		if !isWhitelisted {
			// 删除消息
			go h.deleteMessageWithTimeout(message.Chat.ID, message.MessageID)

			// 如果是 /apply 命令，处理申请逻辑
			if message.Command() == "apply" {
				// 检查是否有待处理的申请
				pendingApp, err := h.DB.GetPendingChannelApplication(message.Chat.ID, channelID)
				if err != nil {
					return err
				}

				// 如果有待处理的申请
				if pendingApp.ID != 0 {
					// 检查是否已通知过"待审核"状态
					hasNoticed, err := h.DB.HasPendingNoticeToday(message.Chat.ID, channelID)
					if err != nil {
						return err
					}

					// 如果今天已经通知过一次"待审核"
					if hasNoticed {
						// 直接删除，不再提示
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

				// 如果没有待处理的申请，继续处理 apply 命令
				return h.HandleApply(message, message.CommandArguments())
			}

			// 对于所有非 /apply 的命令，直接删除并记录被阻止的消息
			if message.IsCommand() {
				go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
				return nil
			}

			// 对于非命令消息，检查今天是否已经提示过"需要申请"
			hasPrompted, _ := h.DB.HasPromptedToday(message.Chat.ID, channelID)

			// 如果今天已经提示过"需要申请"，直接删除不再提示
			if hasPrompted {
				// 记录被阻止的消息
				go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
				return nil
			}

			// 第一次提示"需要申请"
			channelName := h.getChannelName(channelID)
			promptText := fmt.Sprintf("频道「%s」未在白名单中，已删除消息。\n\n"+
				"频道可以直接发送 /apply + 申请理由 命令申请允许发言。", channelName)
			promptMsg := tgbotapi.NewMessage(message.Chat.ID, promptText)
			if _, err := h.Bot.Send(promptMsg); err == nil {
				// 记录已提示过"需要申请"
				h.DB.RecordPrompt(message.Chat.ID, channelID)
			}

			// 记录被阻止的消息
			go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
			return nil
		}
	}

	// 如果是 /apply 命令
	if message.Text == "/apply" || message.Text == "/apply@"+h.Bot.Self.UserName || strings.HasPrefix(message.Text, "/apply ") {
		// 获取参数（处理带理由的 /apply 命令）
		args := ""
		if strings.HasPrefix(message.Text, "/apply ") {
			args = strings.TrimPrefix(message.Text, "/apply ")
		} else if strings.HasPrefix(message.Text, "/apply@"+h.Bot.Self.UserName+" ") {
			args = strings.TrimPrefix(message.Text, "/apply@"+h.Bot.Self.UserName+" ")
		}

		// 检查是否已经有待处理的申请
		hasApp, err := h.DB.HasPendingApplication(message.Chat.ID, message.From.ID)
		if err != nil {
			return err
		}

		// 检查今天是否已经提示过待审核通知
		noticed, err := h.DB.HasPendingNoticeToday(message.Chat.ID, message.From.ID)
		if err != nil {
			return err
		}

		// 如果已经有待处理的申请且今天已经提示过，直接删除消息
		if hasApp && noticed {
			go h.deleteMessageWithTimeout(message.Chat.ID, message.MessageID)
			return nil
		}

		// 如果已经有待处理的申请但今天还没提示过，发送提示并记录
		if hasApp {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"您已经有一个待处理的申请了，请等待管理员审核！")
			msg.ReplyToMessageID = message.MessageID
			_, err = h.Bot.Send(msg)
			if err != nil {
				return err
			}

			// 记录今天已经提示过待审核通知
			err = h.DB.RecordPendingNotice(message.Chat.ID, message.From.ID)
			if err != nil {
				return err
			}

			return nil
		}

		// 如果没有待处理的申请，继续处理
		return h.HandleApply(message, args)
	}

	// 如果是频道消息，检查是否在白名单中
	if message.SenderChat != nil && message.SenderChat.Type == "channel" {
		// 获取频道ID
		channelID := message.SenderChat.ID

		// 检查是否在白名单中
		isWhitelisted, err := h.DB.IsChannelWhitelisted(message.Chat.ID, channelID)
		if err != nil {
			return err
		}

		// 如果不在白名单中
		if !isWhitelisted {
			// 删除消息
			go h.deleteMessageWithTimeout(message.Chat.ID, message.MessageID)

			// 如果是 /apply 命令
			if message.Command() == "apply" {
				args := message.CommandArguments()
				// 检查是否有待处理的申请
				pendingApp, err := h.DB.GetPendingChannelApplication(message.Chat.ID, channelID)
				if err == nil && pendingApp.ID != 0 {
					// 检查今天是否已经提示过"待审核"状态
					hasNoticed, _ := h.DB.HasPendingNoticeToday(message.Chat.ID, channelID)

					// 如果今天已经提示过一次"待审核"，直接删除不再提示
					if hasNoticed {
						return nil
					}

					// 否则，提示"待审核"
					channelName := h.getChannelName(channelID)
					promptText := fmt.Sprintf("频道「%s」已有一个待处理的申请，请等待管理员审核。", channelName)
					promptMsg := tgbotapi.NewMessage(message.Chat.ID, promptText)
					if _, err := h.Bot.Send(promptMsg); err == nil {
						// 记录已提示过"待审核"
						h.DB.RecordPendingNotice(message.Chat.ID, channelID)
					}
					return nil
				}

				// 如果没有待处理的申请，继续处理 apply 命令
				return h.HandleApply(message, args)
			}

			// 对于所有非/apply的命令，直接删除并记录被阻止的消息
			if message.IsCommand() {
				go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
				return nil
			}

			// 检查今天是否已经提示过"需要申请"
			hasPrompted, _ := h.DB.HasPromptedToday(message.Chat.ID, channelID)

			// 如果今天已经提示过，直接返回
			if hasPrompted {
				// 记录被阻止的消息
				go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
				return nil
			}

			// 获取频道名称
			channelName := h.getChannelName(channelID)

			// 对于其他消息，发送未在白名单的提示
			promptText := fmt.Sprintf("频道「%s」未在白名单中，已删除消息。\n\n"+
				"频道可以直接发送 /apply + 申请理由 命令申请允许发言。", channelName)
			promptMsg := tgbotapi.NewMessage(message.Chat.ID, promptText)
			if _, err := h.Bot.Send(promptMsg); err == nil {
				// 记录已提示过"需要申请"
				h.DB.RecordPrompt(message.Chat.ID, channelID)
			}

			// 记录被阻止的消息
			go h.addToMessageQueue(message.Chat.ID, channelID, message.MessageID, message.Text)
			return nil
		}
	}

	return nil
}

// handlePrivateMessage 处理私聊消息
func (h *Handler) handlePrivateMessage(message *tgbotapi.Message) error {
	// 获取用户当前状态
	state, err := h.DB.GetUserState(message.From.ID)
	if err != nil {
		return err
	}

	// 如果用户没有状态，忽略消息
	if state == "" {
		return nil
	}

	// 处理等待理由的状态
	if strings.HasPrefix(state, "waiting_reason:") {
		// 解析状态中的群组ID和频道ID
		parts := strings.Split(state, ":")
		if len(parts) != 3 {
			return fmt.Errorf("invalid state format: %s", state)
		}

		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}

		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return err
		}

		// 更新申请理由
		err = h.DB.UpdateChannelApplicationReason(chatID, channelID, message.Text)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("更新申请理由失败: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}

		// 更新申请用户ID
		err = h.DB.UpdateChannelApplicationUser(chatID, channelID, message.From.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("更新申请用户失败: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}

		// 验证频道所有权
		err = h.DB.VerifyChannelOwnership(chatID, channelID, message.From.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("验证频道所有权失败: %s", err.Error()))
			_, _ = h.Bot.Send(msg)
			return err
		}

		// 获取频道和群组名称
		channelName := h.getChannelName(channelID)

		groupName := "未知群组"
		groupChat, err := h.Bot.GetChat(tgbotapi.ChatInfoConfig{
			ChatConfig: tgbotapi.ChatConfig{
				ChatID: chatID,
			},
		})
		if err == nil && groupChat.Title != "" {
			groupName = groupChat.Title
		}

		// 发送成功消息
		successMsg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("您已成功认领群组「%s」内频道「%s」的发言申请！\n\n申请理由: %s\n\n管理员将尽快审核您的申请，请耐心等待。",
				groupName, channelName, message.Text))
		_, err = h.Bot.Send(successMsg)
		if err != nil {
			return err
		}

		// 通知管理员
		err = h.notifyAdminsAboutApplication(chatID, channelID, message.From.ID, channelName, message.Text)
		if err != nil {
			return err
		}

		// 清除用户状态
		return h.DB.ClearUserState(message.From.ID)
	}

	// 其他状态的处理可以在这里添加

	return nil
}

// deleteMessageWithTimeout 使用goroutine和超时机制删除消息
func (h *Handler) deleteMessageWithTimeout(chatID int64, messageID int) {
	// 创建删除消息请求
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)

	// 使用goroutine进行删除操作
	go func() {
		// 重试最多5次
		for i := 0; i < 5; i++ {
			_, err := h.Bot.Request(deleteMsg)
			if err == nil {
				// 删除成功，分开处理日志记录
				return
			}

			// 如果是限流错误，使用指数退避策略等待后重试
			if strings.Contains(err.Error(), "Too Many Requests") {
				backoffTime := time.Duration(500*((i+1)*(i+1))) * time.Millisecond
				time.Sleep(backoffTime)
				continue
			}

			// 对于其他错误，如果包含 Forbidden 或 Message to delete not found 则不重试
			if strings.Contains(err.Error(), "Forbidden") ||
				strings.Contains(err.Error(), "Message to delete not found") {
				return
			}

			// 其他错误记录并考虑重试
			fmt.Printf("删除消息失败 (尝试 %d/5): %s\n", i+1, err.Error())
			time.Sleep(time.Duration((i+1)*200) * time.Millisecond)
		}
	}()
}

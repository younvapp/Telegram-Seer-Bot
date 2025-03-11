package utils

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// IsAdmin 检查用户是否是群组管理员
func IsAdmin(bot *tgbotapi.BotAPI, chatID int64, userID int64) (bool, error) {
	admins, err := bot.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	})
	if err != nil {
		return false, err
	}

	for _, admin := range admins {
		if admin.User.ID == userID {
			return true, nil
		}
	}

	return false, nil
}

// IsGlobalAdmin 检查用户是否是全局管理员
func IsGlobalAdmin(adminUsers []int64, userID int64) bool {
	for _, admin := range adminUsers {
		if admin == userID {
			return true
		}
	}
	return false
}

// ParseChannelID 从命令参数解析频道ID
func ParseChannelID(args string) (int64, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return 0, fmt.Errorf("频道ID不能为空")
	}

	// 只移除可能的 @ 前缀，保留负号
	if strings.HasPrefix(args, "@") {
		args = args[1:]
	}

	channelID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的频道ID: %s", args)
	}

	return channelID, nil
}

// IsChannelMessage 检查消息是否来自频道
func IsChannelMessage(message *tgbotapi.Message) bool {
	return message.SenderChat != nil && message.SenderChat.Type == "channel"
}

// GetChannelID 获取消息的频道ID
func GetChannelID(message *tgbotapi.Message) int64 {
	if message.SenderChat != nil {
		return message.SenderChat.ID
	}
	return 0
}

// FormatChannelList 格式化频道列表为字符串
func FormatChannelList(channels []string) string {
	if len(channels) == 0 {
		return "白名单中没有频道"
	}

	return "白名单频道列表:\n" + strings.Join(channels, "\n")
}

// TruncateText 截断文本，确保不超过最大长度
func TruncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength-3] + "..."
}

// IsMentioningBot 检查消息是否艾特了机器人
func IsMentioningBot(message *tgbotapi.Message, botUsername string) bool {
	// 检查消息文本中是否包含@botUsername
	if message.Text != "" && strings.Contains(message.Text, "@"+botUsername) {
		return true
	}

	// 检查消息实体中是否有提及机器人
	for _, entity := range message.Entities {
		if entity.Type == "mention" {
			mention := message.Text[entity.Offset : entity.Offset+entity.Length]
			if mention == "@"+botUsername {
				return true
			}
		}
	}

	return false
}

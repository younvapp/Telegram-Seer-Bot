package handlers

import (
	"time"

	"github.com/anhe/tg-whitelist-bot/db/models"
)

// processMsgQueue 批量处理消息队列
func (h *Handler) processMsgQueue() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.flushMessageQueue()
	}
}

// flushMessageQueue 刷新消息队列，批量处理
func (h *Handler) flushMessageQueue() {
	h.messageQueueLock.Lock()

	// 如果队列为空，不处理
	if len(h.messageQueue) == 0 {
		h.messageQueueLock.Unlock()
		return
	}

	// 取出队列中的所有消息
	messages := h.messageQueue
	h.messageQueue = nil
	h.messageQueueLock.Unlock()

	// 转换消息格式
	dbMessages := make([]models.BlockedMessageInfo, len(messages))
	for i, msg := range messages {
		dbMessages[i] = models.BlockedMessageInfo{
			ChatID:      msg.ChatID,
			ChannelID:   msg.ChannelID,
			MessageID:   msg.MessageID,
			MessageText: msg.MessageText,
		}
	}

	// 开始事务处理
	tx, err := h.DB.BeginTx()
	if err != nil {
		// 如果开启事务失败，逐个处理消息
		for _, msg := range messages {
			_ = h.DB.LogBlockedMessage(msg.ChatID, msg.ChannelID, msg.MessageID, msg.MessageText)
		}
		return
	}

	// 批量插入
	success := h.DB.LogBlockedMessagesBatch(tx, dbMessages)

	// 如果批量处理成功，提交事务
	if success {
		_ = tx.Commit()
	} else {
		// 否则回滚并逐个处理
		_ = tx.Rollback()
		for _, msg := range messages {
			_ = h.DB.LogBlockedMessage(msg.ChatID, msg.ChannelID, msg.MessageID, msg.MessageText)
		}
	}
}

// addToMessageQueue 添加到消息队列
func (h *Handler) addToMessageQueue(chatID, channelID int64, messageID int, messageText string) {
	h.messageQueueLock.Lock()
	defer h.messageQueueLock.Unlock()

	h.messageQueue = append(h.messageQueue, blockedMessageInfo{
		ChatID:      chatID,
		ChannelID:   channelID,
		MessageID:   messageID,
		MessageText: messageText,
	})
}

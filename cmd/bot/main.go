package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anhe/tg-whitelist-bot/config"
	"github.com/anhe/tg-whitelist-bot/db"
	"github.com/anhe/tg-whitelist-bot/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// 启动每日零点重置提示状态的定时任务
func startDailyResetTask(database *db.DB) {
	go func() {
		for {
			// 计算距离下一个零点的时间
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			sleepDuration := nextMidnight.Sub(now)

			// 睡眠到下一个零点
			log.Printf("下一次重置提示状态将在 %v 后进行", sleepDuration)
			time.Sleep(sleepDuration)

			// 执行重置
			log.Println("正在重置每日提示状态...")
			if err := database.ResetDailyPrompts(); err != nil {
				log.Printf("重置每日提示状态失败: %v", err)
			} else {
				log.Println("重置每日提示状态成功")
			}
		}
	}()
}

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	database, err := db.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 启动每日零点重置提示状态的定时任务
	startDailyResetTask(database)

	// 初始化 Telegram Bot
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Fatalf("初始化 Telegram Bot 失败: %v", err)
	}

	// 设置调试模式
	bot.Debug = cfg.Debug
	log.Printf("授权账号 %s", bot.Self.UserName)

	// 创建消息处理器
	handler := handlers.New(bot, database, cfg)

	// 设置更新配置
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// 获取更新通道
	updates := bot.GetUpdatesChan(updateConfig)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("正在关闭...")
		bot.StopReceivingUpdates()
		os.Exit(0)
	}()

	// 处理更新
	for update := range updates {
		if err := handler.HandleUpdate(update); err != nil {
			log.Printf("处理更新失败: %v", err)
		}
	}
}

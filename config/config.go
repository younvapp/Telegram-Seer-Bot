package config

import (
	"encoding/json"
	"os"
)

// Config 存储机器人的配置信息
type Config struct {
	Token                          string  `json:"token"`                             // Telegram Bot Token
	DatabasePath                   string  `json:"database_path"`                     // SQLite数据库路径
	AdminUsers                     []int64 `json:"admin_users"`                       // 全局管理员用户ID列表
	Debug                          bool    `json:"debug"`                             // 是否启用调试模式
	RequireRealAccountVerification bool    `json:"require_real_account_verification"` // 是否需要真实账号验证
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	// 设置默认值
	if config.DatabasePath == "" {
		config.DatabasePath = "./whitelist.db"
	}

	return &config, nil
}

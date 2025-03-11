<div align="center">

# <img src="https://telegram.org/img/favicon.ico" width="30" height="30" style="vertical-align: text-bottom;"> Telegram-Seer-Bot

#### **简体中文** | [English](https://github.com/younvapp/Telegram-Seer-Bot/blob/main/README_EN.md)

Telegram 群组管理机器人，自动删除非白名单频道身份的发言。

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![GitHub stars](https://img.shields.io/github/stars/younvapp/Telegram-Seer-Bot?style=for-the-badge)
![GitHub issues](https://img.shields.io/github/issues/younvapp/Telegram-Seer-Bot?style=for-the-badge)
![GitHub pull requests](https://img.shields.io/github/issues-pr/younvapp/Telegram-Seer-Bot?style=for-the-badge)
![GitHub forks](https://img.shields.io/github/forks/younvapp/Telegram-Seer-Bot?style=for-the-badge)

</div>

<br>


## ⭐ 特性


1. **白名单机制**：机器人检查所有来自频道的消息，只允许白名单中的频道发言，自动删除非白名单频道消息。

2. **频道申请流程**：内置完整的频道申请、认领和审批流程，确保频道所有权验证。
   
3. **群组独立设置**：每个群组拥有独立的白名单和设置，互不干扰，满足不同群组的需求。

<br>

## 📕 使用说明

目前仍在完善中，仅推荐部署在个人群组。

### 配置机器人

1. 从 [@BotFather](https://t.me/BotFather) 获取 Telegram Bot Token
2. 创建配置文件 `config.json`，示例如下：

```json
{
  "token": "YOUR_TELEGRAM_BOT_TOKEN",
  "database_path": "./whitelist.db",
  "admin_users": [123456789, 987654321],
  "debug": false,
  "require_real_account_verification": true
}
```

#### 配置项说明

- `token`：（必填）Telegram Bot API令牌，从BotFather获取
- `database_path`：（可选）SQLite数据库文件路径，默认为"./whitelist.db"
- `admin_users`：（必填）全局管理员用户ID列表，这些用户可以在任何群组中管理机器人
- `debug`：（可选）是否启用调试模式，启用后会输出更多日志信息，默认为false
- `require_real_account_verification`：（可选）是否要求频道所有者进行真实账号验证，默认为true


### 部署机器人

```bash
# 下载依赖
go mod tidy

# 编译
go build -o Telegram-Seer-Bot cmd/bot/main.go

# 运行
./Telegram-Seer-Bot
```

### 使用机器人

将机器人添加到您的群组，并赋予管理员权限后，可以使用以下命令：

<br>

## ⌨️ 命令列表

### 基本命令（所有用户可用）

- `/start` - 启动机器人，也用于认领频道申请
- `/help` - 显示帮助信息
- `/list_channels` - 列出当前群组的白名单频道
- `/stats` - 显示当前群组的频道统计信息（总数、阻止次数等）
- `/apply [理由]` - 申请频道发言权限（必须提供理由才能在群内认领）
- `/claim` - 认领频道申请（由频道所有者的个人账号发送）

### 管理员命令

- `/whitelist` 或 `/wl` - 回复一条频道消息，将该频道添加到白名单
- `/unwhitelist` 或 `/unwl` - 回复一条频道消息，将该频道从白名单移除
- `/settings` - 查看/修改当前群组的设置
- `/approve` - 批准频道申请（回复申请消息或提供申请ID）
- `/reject` - 拒绝频道申请（回复申请消息或提供申请ID）

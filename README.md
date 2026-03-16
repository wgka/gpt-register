# codex-register

一个带 Web UI 的自动注册/批量注册工具（Go 后端 + Vue 前端），并可选集成 Telegram 机器人在 TG 里触发注册并实时回传日志。

## 本地运行

1) 安装依赖：Go（按 `go.mod`）+ Node.js 20

2) 配置环境变量

- 复制 `.env.example` 为 `.env`，按需填写

3) 构建前端并运行服务

```bash
cd frontend
npm ci
npm run build

cd ..
go run .
```

启动后访问终端打印的地址（默认 `http://127.0.0.1:8080`，端口占用会自动换到下一个）。

## Telegram 机器人（可选）

只要在 `.env` 里设置 `TELEGRAM_BOT_TOKEN`，执行 `go run .` 时会自动同进程启动机器人。

TG 命令：

- `/register 20 5`（数量=20，并发=5）
- `/cancel <batch_id>`

建议配置白名单：

- `.env`：`TELEGRAM_ALLOWED_CHAT_IDS=123,456`

## GitHub Actions 自动打包

推送 tag（`v*`）会自动构建并发布 Release：

- Windows x64：`codex-register-windows-x64.exe`
- Linux x64：`codex-register-linux-x64`

工作流文件：`.github/workflows/build.yml`


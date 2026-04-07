# gpt-register

一个带 Web UI 的自动注册/批量注册工具（Go 后端 + Vue 前端），并可选集成 Telegram 机器人在 TG 里触发注册并实时回传日志。

## 本地运行

1) 安装依赖：Go（按 `go.mod`）+ Node.js 20

2) 配置环境变量

- 复制 `.env.example` 为 `.env`，按需填写
- 注册邮箱默认走临时邮箱 API，可通过 `TEMP_MAIL_API_BASE_URL` 覆盖地址
- 代理配置二选一：固定代理填 `APP_PROXY_URL`；动态代理池接口才填 `APP_PROXY_API_URL`
- 本地 Clash/7897 这类代理端口不要填到 `APP_PROXY_API_URL`，否则程序会把它当 JSON 接口请求
- 授权链路可通过 `APP_AUTH_MODE` 选择：
  - `chatgpt_web`：ChatGPT 网页登录链路
  - `codex_cli`：Codex/CLI 风格 OAuth 参数（PKCE + `http://localhost:1455/auth/callback`）

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

package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"codex-register/internal/runtime"
)

type Service struct {
	tasks *runtime.TaskManager
	bot   *tgbotapi.BotAPI

	allowedChats map[int64]struct{}
}

func Start(ctx context.Context, tasks *runtime.TaskManager) error {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return nil
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("init telegram bot: %w", err)
	}
	bot.Debug = strings.EqualFold(strings.TrimSpace(os.Getenv("TELEGRAM_DEBUG")), "true")

	s := &Service{
		tasks:        tasks,
		bot:          bot,
		allowedChats: parseAllowedChatIDs(os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS")),
	}
	if len(s.allowedChats) == 0 {
		log.Println("telegram-bot: warning: TELEGRAM_ALLOWED_CHAT_IDS not set; all chats are allowed")
	}

	log.Printf("telegram-bot: authorized as @%s", bot.Self.UserName)
	return s.run(ctx)
}

func (s *Service) run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := s.bot.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return errors.New("telegram updates channel closed")
			}
			if update.Message == nil {
				continue
			}
			chatID := update.Message.Chat.ID
			if !isChatAllowed(s.allowedChats, chatID) {
				_ = s.reply(chatID, "未授权的 chat_id")
				continue
			}

			text := strings.TrimSpace(update.Message.Text)
			if text == "" {
				continue
			}

			switch {
			case strings.HasPrefix(text, "/start"), strings.HasPrefix(text, "/help"):
				_ = s.reply(chatID, helpText())
			case strings.HasPrefix(text, "/register"):
				go func() {
					if err := s.handleRegister(context.Background(), chatID, text); err != nil {
						_ = s.reply(chatID, "启动失败: "+err.Error())
					}
				}()
			case strings.HasPrefix(text, "/cancel"):
				if err := s.handleCancel(chatID, text); err != nil {
					_ = s.reply(chatID, "取消失败: "+err.Error())
				}
			default:
				_ = s.reply(chatID, "未知命令，发送 /help 查看用法")
			}
		}
	}
}

func helpText() string {
	return strings.TrimSpace(`
可用命令：
/register 20 5
/register count=20 concurrency=5
/cancel <batch_id>

环境变量：
TELEGRAM_BOT_TOKEN：机器人 token（必填）
TELEGRAM_ALLOWED_CHAT_IDS：允许的 chat_id（逗号分隔，可选；不填则允许所有）
TELEGRAM_DEBUG：true/false（可选）
`)
}

func (s *Service) handleRegister(ctx context.Context, chatID int64, text string) error {
	args := strings.Fields(text)
	count, concurrency, err := parseRegisterArgs(args[1:])
	if err != nil {
		return err
	}

	req := runtime.BatchRequest{
		Count:            count,
		Concurrency:      concurrency,
		IntervalMin:      5,
		IntervalMax:      15,
		Proxy:            "",
		EmailServiceType: "meteormail",
	}

	msgID, err := s.send(chatID, fmt.Sprintf("已接收：数量=%d 并发=%d\n准备启动…", req.Count, req.Concurrency))
	if err != nil {
		return err
	}

	batchID, _, err := s.tasks.StartBatch(ctx, req)
	if err != nil {
		return err
	}

	_ = s.edit(chatID, msgID, fmt.Sprintf("已启动 batch：%s\n正在订阅日志…", batchID))
	return s.streamBatch(ctx, chatID, msgID, batchID)
}

func (s *Service) handleCancel(chatID int64, text string) error {
	args := strings.Fields(text)
	if len(args) < 2 {
		return errors.New("用法：/cancel <batch_id>")
	}
	batchID := strings.TrimSpace(args[1])
	if batchID == "" {
		return errors.New("batch_id 不能为空")
	}
	if !s.tasks.CancelBatch(batchID) {
		return errors.New("batch 不存在或已结束")
	}
	_ = s.reply(chatID, "取消请求已提交: "+batchID)
	return nil
}

func parseRegisterArgs(args []string) (int, int, error) {
	// 支持两种写法：
	// 1) /register 20 5
	// 2) /register count=20 concurrency=5
	count := 1
	concurrency := 1

	// 纯数字参数优先
	nums := make([]int, 0, 2)
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.Contains(a, "=") {
			continue
		}
		n, err := strconv.Atoi(a)
		if err != nil {
			continue
		}
		nums = append(nums, n)
		if len(nums) >= 2 {
			break
		}
	}
	if len(nums) >= 1 {
		count = nums[0]
	}
	if len(nums) >= 2 {
		concurrency = nums[1]
	}

	// key=value 覆盖
	kv := parseKVArgs(args)
	if v, ok := kv["count"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			count = n
		}
	}
	if v, ok := kv["concurrency"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			concurrency = n
		}
	}

	if count < 1 {
		return 0, 0, errors.New("count 必须 >= 1，例如：/register 20 5")
	}
	if concurrency < 1 {
		return 0, 0, errors.New("concurrency 必须 >= 1，例如：/register 20 5")
	}
	if concurrency > count {
		concurrency = count
	}
	return count, concurrency, nil
}

func (s *Service) streamBatch(ctx context.Context, chatID int64, msgID int, batchID string) error {
	ch := s.tasks.SubscribeBatch(batchID)
	defer s.tasks.UnsubscribeBatch(batchID, ch)

	lines := make([]string, 0, 128)
	var mu sync.Mutex
	status := "running"
	summary := ""

	ticker := time.NewTicker(1200 * time.Millisecond)
	defer ticker.Stop()

	flush := func(force bool) {
		mu.Lock()
		defer mu.Unlock()
		text := renderLogMessage(batchID, status, summary, lines)
		if !force && strings.TrimSpace(text) == "" {
			return
		}
		_ = s.edit(chatID, msgID, text)
	}

	for {
		select {
		case <-ctx.Done():
			flush(true)
			return ctx.Err()
		case <-ticker.C:
			flush(false)
		case ev, ok := <-ch:
			if !ok {
				flush(true)
				return errors.New("batch event channel closed")
			}

			mu.Lock()
			switch ev.Type {
			case "log":
				if strings.TrimSpace(ev.Message) != "" {
					lines = append(lines, ev.Message)
				}
			case "status":
				if strings.TrimSpace(ev.Status) != "" {
					status = ev.Status
				}
				if ev.Extra != nil {
					summary = renderSummary(ev.Extra)
				}
				if strings.TrimSpace(ev.Message) != "" {
					lines = append(lines, ev.Message)
				}
			}
			if len(lines) > 60 {
				lines = lines[len(lines)-60:]
			}
			done := isTerminalStatus(status)
			mu.Unlock()

			if done {
				flush(true)
				return nil
			}
		}
	}
}

func renderLogMessage(batchID, status, summary string, lines []string) string {
	var b strings.Builder
	b.WriteString("batch: ")
	b.WriteString(batchID)
	b.WriteString("\nstatus: ")
	if strings.TrimSpace(status) == "" {
		b.WriteString("-")
	} else {
		b.WriteString(status)
	}
	if strings.TrimSpace(summary) != "" {
		b.WriteString("\n")
		b.WriteString(summary)
	}
	b.WriteString("\n\n")
	if len(lines) == 0 {
		b.WriteString("暂无日志")
		return b.String()
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	out := b.String()
	if len(out) > 3900 {
		out = out[len(out)-3900:]
	}
	return out
}

func renderSummary(extra map[string]any) string {
	get := func(key string) string {
		v, ok := extra[key]
		if !ok || v == nil {
			return ""
		}
		switch t := v.(type) {
		case float64:
			return strconv.Itoa(int(t))
		case int:
			return strconv.Itoa(t)
		case string:
			return strings.TrimSpace(t)
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	total := get("total")
	completed := get("completed")
	success := get("success")
	failed := get("failed")
	concurrency := get("concurrency")
	if total == "" && completed == "" && success == "" && failed == "" {
		return ""
	}
	parts := make([]string, 0, 5)
	if total != "" {
		parts = append(parts, "total="+total)
	}
	if completed != "" {
		parts = append(parts, "completed="+completed)
	}
	if success != "" {
		parts = append(parts, "success="+success)
	}
	if failed != "" {
		parts = append(parts, "failed="+failed)
	}
	if concurrency != "" {
		parts = append(parts, "concurrency="+concurrency)
	}
	return strings.Join(parts, " ")
}

func isTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "cancelled", "failed":
		return true
	default:
		return false
	}
}

func parseKVArgs(args []string) map[string]string {
	out := map[string]string{}
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		key, val, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}

func getInt(kv map[string]string, key string, def int) int {
	raw := strings.TrimSpace(kv[key])
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func parseAllowedChatIDs(raw string) map[int64]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[int64]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func isChatAllowed(allowed map[int64]struct{}, chatID int64) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[chatID]
	return ok
}

func (s *Service) reply(chatID int64, text string) error {
	_, err := s.bot.Send(tgbotapi.NewMessage(chatID, text))
	return err
}

func (s *Service) send(chatID int64, text string) (int, error) {
	msg, err := s.bot.Send(tgbotapi.NewMessage(chatID, text))
	if err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

func (s *Service) edit(chatID int64, msgID int, text string) error {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	_, err := s.bot.Send(edit)
	return err
}

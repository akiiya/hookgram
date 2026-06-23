package render

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

type WebhookView struct {
	Title      string
	Content    string
	Format     string
	Level      string
	Source     string
	TokenAlias string
	Fields     map[string]string
	CreatedAt  time.Time
}

type Message struct {
	Text      string
	ParseMode string
	Fallback  string
}

const maxTelegramChars = 3900

func WebhookMessage(view WebhookView) Message {
	format := normalizeFormat(view.Format)
	switch format {
	case "markdown":
		fallback := plainWebhook(view)
		return Message{
			Text:      markdownWebhook(view),
			ParseMode: "MarkdownV2",
			Fallback:  fallback,
		}
	case "html":
		fallback := plainWebhook(view)
		return Message{
			Text:      htmlWebhook(view),
			ParseMode: "HTML",
			Fallback:  fallback,
		}
	default:
		text := plainWebhook(view)
		return Message{Text: text, Fallback: text}
	}
}

func ConstrainTelegramMessage(message Message) Message {
	message.Text = truncateWithNotice(message.Text, message.ParseMode)
	message.Fallback = truncateWithNotice(message.Fallback, "")
	return message
}

func plainWebhook(view WebhookView) string {
	var b strings.Builder
	b.WriteString("Hookgram 通知\n\n")
	if view.Title != "" {
		b.WriteString("标题：")
		b.WriteString(view.Title)
		b.WriteString("\n")
	}
	if view.TokenAlias != "" {
		b.WriteString("Token：")
		b.WriteString(view.TokenAlias)
		b.WriteString("\n")
	}
	b.WriteString("级别：")
	b.WriteString(normalizeLevel(view.Level))
	b.WriteString("\n")
	if strings.TrimSpace(view.Source) != "" {
		b.WriteString("来源：")
		b.WriteString(view.Source)
		b.WriteString("\n")
	}
	b.WriteString("时间：")
	b.WriteString(view.CreatedAt.Format("2006-01-02 15:04:05"))
	b.WriteString("\n\n内容：\n")
	if strings.TrimSpace(view.Content) == "" {
		b.WriteString("（空内容）")
	} else {
		b.WriteString(view.Content)
	}
	appendFields(&b, view.Fields)
	return b.String()
}

func markdownWebhook(view WebhookView) string {
	var b strings.Builder
	b.WriteString("*Hookgram 通知*\n\n")
	if view.Title != "" {
		b.WriteString("*标题：*")
		b.WriteString(escapeMarkdownV2(view.Title))
		b.WriteString("\n")
	}
	if view.TokenAlias != "" {
		b.WriteString("*Token：*")
		b.WriteString(escapeMarkdownV2(view.TokenAlias))
		b.WriteString("\n")
	}
	b.WriteString("*级别：*")
	b.WriteString(escapeMarkdownV2(normalizeLevel(view.Level)))
	b.WriteString("\n")
	if strings.TrimSpace(view.Source) != "" {
		b.WriteString("*来源：*")
		b.WriteString(escapeMarkdownV2(view.Source))
		b.WriteString("\n")
	}
	b.WriteString("*时间：*")
	b.WriteString(escapeMarkdownV2(view.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString("\n\n*内容：*\n")
	if strings.TrimSpace(view.Content) == "" {
		b.WriteString("（空内容）")
	} else {
		b.WriteString(view.Content)
	}
	appendMarkdownFields(&b, view.Fields)
	return b.String()
}

func htmlWebhook(view WebhookView) string {
	var b strings.Builder
	b.WriteString("<b>Hookgram 通知</b>\n\n")
	if view.Title != "" {
		b.WriteString("<b>标题：</b>")
		b.WriteString(html.EscapeString(view.Title))
		b.WriteString("\n")
	}
	if view.TokenAlias != "" {
		b.WriteString("<b>Token：</b>")
		b.WriteString(html.EscapeString(view.TokenAlias))
		b.WriteString("\n")
	}
	b.WriteString("<b>级别：</b>")
	b.WriteString(html.EscapeString(normalizeLevel(view.Level)))
	b.WriteString("\n")
	if strings.TrimSpace(view.Source) != "" {
		b.WriteString("<b>来源：</b>")
		b.WriteString(html.EscapeString(view.Source))
		b.WriteString("\n")
	}
	b.WriteString("<b>时间：</b>")
	b.WriteString(html.EscapeString(view.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString("\n\n<b>内容：</b>\n")
	if strings.TrimSpace(view.Content) == "" {
		b.WriteString("（空内容）")
	} else {
		b.WriteString(view.Content)
	}
	appendHTMLFields(&b, view.Fields)
	return b.String()
}

func appendFields(b *strings.Builder, fields map[string]string) {
	if len(fields) == 0 {
		return
	}
	b.WriteString("\n\n附加信息：\n")
	for _, key := range sortedKeys(fields) {
		b.WriteString(key)
		b.WriteString("：")
		b.WriteString(fields[key])
		b.WriteString("\n")
	}
}

func appendMarkdownFields(b *strings.Builder, fields map[string]string) {
	if len(fields) == 0 {
		return
	}
	b.WriteString("\n\n*附加信息：*\n")
	for _, key := range sortedKeys(fields) {
		b.WriteString(escapeMarkdownV2(key))
		b.WriteString("：")
		b.WriteString(escapeMarkdownV2(fields[key]))
		b.WriteString("\n")
	}
}

func appendHTMLFields(b *strings.Builder, fields map[string]string) {
	if len(fields) == 0 {
		return
	}
	b.WriteString("\n\n<b>附加信息：</b>\n")
	for _, key := range sortedKeys(fields) {
		b.WriteString(html.EscapeString(key))
		b.WriteString("：")
		b.WriteString(html.EscapeString(fields[key]))
		b.WriteString("\n")
	}
}

func sortedKeys(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "markdown", "html", "plain":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return "plain"
	}
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "success", "warning", "error", "info":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "info"
	}
}

func escapeMarkdownV2(input string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(input)
}

func truncateWithNotice(input, parseMode string) string {
	if runeLen(input) <= maxTelegramChars {
		return input
	}
	notice := "\n\n（内容过长，已截断）"
	switch parseMode {
	case "MarkdownV2":
		notice = "\n\n" + escapeMarkdownV2("（内容过长，已截断）")
	case "HTML":
		notice = "\n\n" + html.EscapeString("（内容过长，已截断）")
	}
	limit := maxTelegramChars - runeLen(notice)
	if limit < 0 {
		limit = maxTelegramChars
	}
	return firstRunes(input, limit) + notice
}

func runeLen(input string) int {
	return len([]rune(input))
}

func firstRunes(input string, limit int) string {
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}

func CommandHelp(baseURL string) string {
	return fmt.Sprintf(`Hookgram 命令

/list
查看当前所有 Webhook Token。

/add [别名]
创建新的 Webhook Token。

/del <别名或token前缀>
删除自己的 Token。

/rename <旧别名> <新别名>
修改 Token 别名。

/url <别名>
查看 Webhook 地址说明。系统不保存完整 Token，丢失后请重新创建。

/usage <别名>
查看 Token 使用统计。

当前服务地址：%s`, baseURL)
}

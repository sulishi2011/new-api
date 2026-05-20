package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// WebhookPayload webhook 通知的负载数据
type WebhookPayload struct {
	Type      string        `json:"type"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Values    []interface{} `json:"values,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

type feishuWebhookPayload struct {
	Timestamp string                `json:"timestamp,omitempty"`
	Sign      string                `json:"sign,omitempty"`
	MsgType   string                `json:"msg_type"`
	Content   *feishuWebhookContent `json:"content,omitempty"`
	Card      *feishuWebhookCard    `json:"card,omitempty"`
}

type feishuWebhookContent struct {
	Text string `json:"text"`
}

type feishuWebhookCard struct {
	Config   feishuWebhookCardConfig    `json:"config"`
	Header   feishuWebhookCardHeader    `json:"header"`
	Elements []feishuWebhookCardElement `json:"elements"`
}

type feishuWebhookCardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type feishuWebhookCardHeader struct {
	Template string            `json:"template,omitempty"`
	Title    feishuWebhookText `json:"title"`
}

type feishuWebhookCardElement struct {
	Tag    string               `json:"tag"`
	Text   *feishuWebhookText   `json:"text,omitempty"`
	Fields []feishuWebhookField `json:"fields,omitempty"`
}

type feishuWebhookField struct {
	IsShort bool              `json:"is_short"`
	Text    feishuWebhookText `json:"text"`
}

type feishuWebhookText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type feishuWebhookResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

const (
	feishuFieldValueMaxRunes  = 80
	feishuDetailValueMaxRunes = 240
)

// generateSignature 生成 webhook 签名
func generateSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func generateFeishuSignature(timestamp string, secret string) string {
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func isFeishuWebhookURL(webhookURL string) bool {
	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "open.feishu.cn" && host != "open.larksuite.com" {
		return false
	}
	return strings.HasPrefix(parsed.EscapedPath(), "/open-apis/bot/v2/hook/") ||
		strings.HasPrefix(parsed.EscapedPath(), "/open-apis/bot/hook/")
}

func renderNotifyContent(data dto.Notify) string {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	return content
}

func buildGenericWebhookPayload(data dto.Notify, content string) ([]byte, error) {
	payload := WebhookPayload{
		Type:      data.Type,
		Title:     data.Title,
		Content:   content,
		Values:    data.Values,
		Timestamp: time.Now().Unix(),
	}
	return common.Marshal(payload)
}

func buildFeishuWebhookPayload(secret string, data dto.Notify, content string) ([]byte, error) {
	payload := feishuWebhookPayload{
		MsgType: "interactive",
		Card:    buildFeishuWebhookCard(data, content),
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload.Timestamp = timestamp
		payload.Sign = generateFeishuSignature(timestamp, secret)
	}
	return common.Marshal(payload)
}

func buildFeishuWebhookCard(data dto.Notify, content string) *feishuWebhookCard {
	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = notifyTypeLabel(data.Type)
	}

	card := &feishuWebhookCard{
		Config: feishuWebhookCardConfig{
			WideScreenMode: true,
		},
		Header: feishuWebhookCardHeader{
			Template: feishuCardTemplate(data.Type),
			Title: feishuWebhookText{
				Tag:     "plain_text",
				Content: title,
			},
		},
		Elements: []feishuWebhookCardElement{},
	}

	compactFields, detailFields := splitFeishuFields(data.Fields)
	compactFields = append([]dto.NotifyField{
		{Label: "时间", Value: time.Now().Format("01-02 15:04:05 MST")},
	}, compactFields...)
	if len(compactFields) > 0 {
		card.Elements = append(card.Elements, feishuSummaryDiv(compactFields))
	}
	for _, field := range detailFields {
		if len(card.Elements) > 0 {
			card.Elements = append(card.Elements, feishuHr())
		}
		card.Elements = append(card.Elements, feishuDetailDiv(field))
	}
	if len(data.Fields) == 0 && strings.TrimSpace(content) != "" {
		if len(card.Elements) > 0 {
			card.Elements = append(card.Elements, feishuHr())
		}
		card.Elements = append(card.Elements, feishuMarkdownDiv(fmt.Sprintf(
			"**详情**\n%s",
			compactFeishuValue(content, feishuDetailValueMaxRunes),
		)))
	}
	return card
}

func splitFeishuFields(fields []dto.NotifyField) ([]dto.NotifyField, []dto.NotifyField) {
	compactFields := make([]dto.NotifyField, 0, len(fields))
	detailFields := make([]dto.NotifyField, 0, len(fields))
	for _, field := range fields {
		switch strings.TrimSpace(field.Label) {
		case "错误", "原因":
			detailFields = append(detailFields, field)
		case "通道":
			continue
		default:
			compactFields = append(compactFields, field)
		}
	}
	return compactFields, detailFields
}

func feishuSummaryDiv(fields []dto.NotifyField) feishuWebhookCardElement {
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		label := strings.TrimSpace(field.Label)
		if label == "" {
			continue
		}
		items = append(items, fmt.Sprintf("**%s** %s", compactFeishuLabel(label), compactFeishuValue(field.Value, feishuFieldValueMaxRunes)))
	}
	if len(items) == 0 {
		return feishuMarkdownDiv("-")
	}
	lines := make([]string, 0, (len(items)+2)/3)
	for start := 0; start < len(items); start += 3 {
		end := start + 3
		if end > len(items) {
			end = len(items)
		}
		lines = append(lines, strings.Join(items[start:end], "   "))
	}
	return feishuMarkdownDiv(strings.Join(lines, "\n"))
}

func feishuDetailDiv(field dto.NotifyField) feishuWebhookCardElement {
	label := strings.TrimSpace(field.Label)
	value := compactFeishuValue(field.Value, feishuDetailValueMaxRunes)
	return feishuMarkdownDiv(fmt.Sprintf("**%s**\n%s", label, value))
}

func feishuMarkdownDiv(content string) feishuWebhookCardElement {
	return feishuWebhookCardElement{
		Tag: "div",
		Text: &feishuWebhookText{
			Tag:     "lark_md",
			Content: content,
		},
	}
}

func feishuHr() feishuWebhookCardElement {
	return feishuWebhookCardElement{Tag: "hr"}
}

func feishuCardTemplate(notifyType string) string {
	switch notifyType {
	case dto.NotifyTypeChannelRequestFailure:
		return "red"
	case dto.NotifyTypeChannelDisabled, dto.NotifyTypeQuotaExceed:
		return "orange"
	case dto.NotifyTypeChannelTest:
		return "green"
	default:
		return "blue"
	}
}

func notifyTypeLabel(notifyType string) string {
	switch notifyType {
	case dto.NotifyTypeChannelRequestFailure:
		return "请求失败"
	case dto.NotifyTypeChannelDisabled:
		return "渠道禁用"
	case dto.NotifyTypeQuotaExceed:
		return "额度告警"
	case dto.NotifyTypeChannelUpdate:
		return "渠道更新"
	case dto.NotifyTypeChannelTest:
		return "渠道测试"
	default:
		return notifyType
	}
}

func trimFeishuValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func compactFeishuValue(value string, maxRunes int) string {
	value = trimFeishuValue(strings.Join(strings.Fields(value), " "))
	if value == "-" || maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func compactFeishuLabel(label string) string {
	switch label {
	case "请求路径":
		return "路径"
	case "重试序号":
		return "重试"
	default:
		return label
	}
}

func checkWebhookResponse(resp *http.Response, feishu bool) error {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook request failed with status code: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if !feishu || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	var feishuResp feishuWebhookResponse
	if err := common.Unmarshal(body, &feishuResp); err != nil {
		return fmt.Errorf("failed to parse feishu webhook response: %v", err)
	}
	if feishuResp.Code != 0 {
		return fmt.Errorf("feishu webhook request failed with code %d: %s", feishuResp.Code, feishuResp.Msg)
	}
	return nil
}

// SendWebhookNotify 发送 webhook 通知
func SendWebhookNotify(webhookURL string, secret string, data dto.Notify) error {
	// 处理占位符
	content := renderNotifyContent(data)
	secret = strings.TrimSpace(secret)
	feishu := isFeishuWebhookURL(webhookURL)

	// 序列化负载
	var payloadBytes []byte
	var err error
	if feishu {
		payloadBytes, err = buildFeishuWebhookPayload(secret, data, content)
	} else {
		payloadBytes, err = buildGenericWebhookPayload(data, content)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %v", err)
	}

	// 创建 HTTP 请求
	var req *http.Request
	var resp *http.Response

	if system_setting.EnableWorker() {
		// 构建worker请求数据
		workerReq := &WorkerRequest{
			URL:    webhookURL,
			Key:    system_setting.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: payloadBytes,
		}

		// 如果有secret，添加签名到headers
		if secret != "" && !feishu {
			signature := generateSignature(secret, payloadBytes)
			workerReq.Headers["X-Webhook-Signature"] = signature
			workerReq.Headers["Authorization"] = "Bearer " + secret
		}

		resp, err = DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send webhook request through worker: %v", err)
		}
		defer resp.Body.Close()
		return checkWebhookResponse(resp, feishu)
	} else {
		// SSRF防护：验证Webhook URL（非Worker模式）
		fetchSetting := system_setting.GetFetchSetting()
		if err := common.ValidateURLWithFetchSetting(webhookURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
			return fmt.Errorf("request reject: %v", err)
		}

		req, err = http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return fmt.Errorf("failed to create webhook request: %v", err)
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")

		// 如果有 secret，生成签名
		if secret != "" && !feishu {
			signature := generateSignature(secret, payloadBytes)
			req.Header.Set("X-Webhook-Signature", signature)
		}

		// 发送请求
		client := GetHttpClient()
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send webhook request: %v", err)
		}
		defer resp.Body.Close()
		return checkWebhookResponse(resp, feishu)
	}
}

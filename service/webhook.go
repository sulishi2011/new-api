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
	Timestamp string               `json:"timestamp,omitempty"`
	Sign      string               `json:"sign,omitempty"`
	MsgType   string               `json:"msg_type"`
	Content   feishuWebhookContent `json:"content"`
}

type feishuWebhookContent struct {
	Text string `json:"text"`
}

type feishuWebhookResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

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
	text := strings.TrimSpace(data.Title)
	if strings.TrimSpace(content) != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.TrimSpace(content)
	}

	payload := feishuWebhookPayload{
		MsgType: "text",
		Content: feishuWebhookContent{Text: text},
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload.Timestamp = timestamp
		payload.Sign = generateFeishuSignature(timestamp, secret)
	}
	return common.Marshal(payload)
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

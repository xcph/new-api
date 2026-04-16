// Package moonshot 提供 Moonshot Kimi OpenAPI 的轻量客户端（含官方 web-search Formula）。
package moonshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL 与 Python 示例 kimi-search.py 一致：国内 Key 通常只接受 .cn 端点。
	DefaultBaseURL = "https://api.moonshot.cn/v1"
	WebSearchFormula = "moonshot/web-search:latest"
	// DefaultBuiltinModel 对应 Python 中 chat.completions 使用的 model（builtin $web_search）。
	DefaultBuiltinModel = "kimi-k2-turbo-preview"
	// DefaultFormulaChatModel 用于 Formula tools + Chat 流程。
	DefaultFormulaChatModel = "kimi-k2.5"
	defaultUserAgent = "xcph-kimi-search-go/1.0"
)

// Client 调用 Kimi HTTP API（兼容 OpenAI Chat Completions + Formula 端点）。
type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	UserAgent  string
	ChatModel  string
}

// New 使用 API Key 创建客户端；baseURL 为空则使用 DefaultBaseURL。
func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKey:    apiKey,
		HTTP:      &http.Client{Timeout: 120 * time.Second},
		UserAgent: defaultUserAgent,
		// ChatModel 为空时：内置搜索用 DefaultBuiltinModel，Formula 对话用 DefaultFormulaChatModel。
		ChatModel: "",
	}
}

func (c *Client) authHeader() string {
	return "Bearer " + c.APIKey
}

// --- Formula: tools ---

// ToolsListResponse 对应 GET /formulas/{uri}/tools。
type ToolsListResponse struct {
	Object string          `json:"object"`
	Tools  json.RawMessage `json:"tools"`
}

// GetFormulaTools 拉取指定 Formula 的 tools 定义（可直接填入 chat completions 的 tools 字段）。
func (c *Client) GetFormulaTools(formulaURI string) (json.RawMessage, error) {
	url := c.BaseURL + "/formulas/" + formulaURI + "/tools"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get tools %s: %s: %s", url, resp.Status, truncate(body, 512))
	}
	var out ToolsListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

// --- Formula: fiber（直接执行 web_search 等）---

// FiberRequest POST /formulas/{uri}/fibers。
type FiberRequest struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串（与官方文档一致）
}

// FiberResponse 为 Fiber 调用返回的简化结构。
// 失败时 error 可能是字符串，也可能是 { "message", "type" } 对象（与 OpenAI 风格一致）。
type FiberResponse struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Status    string          `json:"status"`
	Formula   string          `json:"formula"`
	Context   json.RawMessage `json:"context"`
	Error     json.RawMessage `json:"error,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type fiberContext struct {
	Input           string `json:"input"`
	Output          string `json:"output"`
	EncryptedOutput string `json:"encrypted_output"`
}

// CallFiber 执行 Formula 中的函数（例如 web_search）。成功时优先返回 output，否则返回 encrypted_output。
func (c *Client) CallFiber(formulaURI, functionName, argumentsJSON string) (string, *FiberResponse, error) {
	url := c.BaseURL + "/formulas/" + formulaURI + "/fibers"
	payload := FiberRequest{Name: functionName, Arguments: argumentsJSON}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	var fr FiberResponse
	fr.Raw = append(json.RawMessage(nil), body...)
	if err := json.Unmarshal(body, &fr); err != nil {
		return "", nil, fmt.Errorf("decode fiber response: %w; body=%s", err, truncate(body, 512))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := formatFlexibleAPIError(fr.Error)
		if msg == "" {
			msg = truncate(body, 512)
		}
		return "", &fr, fmt.Errorf("fiber %s: %s: %s", url, resp.Status, msg)
	}
	if fr.Status != "succeeded" {
		if msg := formatFlexibleAPIError(fr.Error); msg != "" {
			return "", &fr, fmt.Errorf("fiber status=%s: %s", fr.Status, msg)
		}
		return "", &fr, fmt.Errorf("fiber status=%s", fr.Status)
	}

	var ctx fiberContext
	if err := json.Unmarshal(fr.Context, &ctx); err == nil {
		if ctx.Output != "" {
			return ctx.Output, &fr, nil
		}
		if ctx.EncryptedOutput != "" {
			return ctx.EncryptedOutput, &fr, nil
		}
	}
	return string(fr.Context), &fr, nil
}

// WebSearch 使用 Chat Completions 的 builtin_function「$web_search」，行为与 py/scripts/kimi-search.py 一致
//（非 Formula /fibers）。同一 MOONSHOT_API_KEY 请配合 DefaultBaseURL（api.moonshot.cn）使用。
func (c *Client) WebSearch(query string) (string, error) {
	return c.searchBuiltinWebSearch(query)
}

// WebSearchFiber 直接调用 Formula 的 web_search（POST .../fibers）。部分区域或 Key 仅支持内置搜索，
// 若需与国内 Python 脚本一致，请使用 WebSearch。
func (c *Client) WebSearchFiber(query string) (string, error) {
	args, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return "", err
	}
	out, _, err := c.CallFiber(WebSearchFormula, "web_search", string(args))
	return out, err
}

// searchBuiltinWebSearch 对齐 Python：tools 为 builtin_function $web_search，工具结果回传模型解析出的 arguments。
func (c *Client) searchBuiltinWebSearch(userQuery string) (string, error) {
	model := c.ChatModel
	if model == "" {
		model = DefaultBuiltinModel
	}
	tools := []map[string]any{
		{
			"type": "builtin_function",
			"function": map[string]any{
				"name": "$web_search",
			},
		},
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return "", err
	}

	msgs := []map[string]any{
		{"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手。"},
		{"role": "user", "content": userQuery},
	}

	for {
		reqBody := map[string]any{
			"model":       model,
			"messages":    msgs,
			"tools":       json.RawMessage(toolsJSON),
			"temperature": 0.6,
		}
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", c.authHeader())
		req.Header.Set("Content-Type", "application/json")
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return "", err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("chat completions: %s: %s", resp.Status, truncate(body, 1024))
		}

		var cr chatResponse
		if err := json.Unmarshal(body, &cr); err != nil {
			return "", err
		}
		if cr.Error != nil && cr.Error.Message != "" {
			return "", fmt.Errorf("api error: %s", cr.Error.Message)
		}
		if len(cr.Choices) == 0 {
			return "", fmt.Errorf("empty choices")
		}
		ch := cr.Choices[0]
		finish := ch.FinishReason

		if finish != "tool_calls" {
			if ch.Message.Content != nil {
				return *ch.Message.Content, nil
			}
			return "", nil
		}

		// 与 OpenAI Message 结构对齐：整段 assistant（含 tool_calls）原样追加，便于下一轮请求。
		assistant := map[string]any{
			"role": ch.Message.Role,
		}
		if ch.Message.Content != nil {
			assistant["content"] = *ch.Message.Content
		} else {
			assistant["content"] = nil
		}
		if len(ch.Message.ToolCalls) > 0 {
			assistant["tool_calls"] = ch.Message.ToolCalls
		}
		msgs = append(msgs, assistant)

		for _, tc := range ch.Message.ToolCalls {
			fn, ok := tc["function"].(map[string]any)
			if !ok {
				return "", fmt.Errorf("tool_calls: missing function")
			}
			name, _ := fn["name"].(string)
			argStr, _ := fn["arguments"].(string)
			var toolResult any
			if argStr != "" {
				if err := json.Unmarshal([]byte(argStr), &toolResult); err != nil {
					toolResult = argStr
				}
			}
			var content string
			switch name {
			case "$web_search":
				b, err := json.Marshal(toolResult)
				if err != nil {
					return "", err
				}
				content = string(b)
			default:
				content = `{"error":"unknown tool ` + name + `"}`
			}
			id, _ := tc["id"].(string)
			toolMsg := map[string]any{
				"role":         "tool",
				"tool_call_id": id,
				"name":         name,
				"content":      content,
			}
			msgs = append(msgs, toolMsg)
		}
	}
}

// --- Chat completions（带 tool_calls 循环）---

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []map[string]any `json:"messages"`
	Tools    json.RawMessage  `json:"tools,omitempty"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role         string           `json:"role"`
			Content      *string          `json:"content"` // null 常见于带 tool_calls 的 assistant
			ToolCalls    []map[string]any `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ChatWithWebSearch 使用 web-search Formula 的工具定义发起对话，并在本地执行 tool_calls（含 web_search）。
func (c *Client) ChatWithWebSearch(systemPrompt, userMsg string) (string, error) {
	tools, err := c.GetFormulaTools(WebSearchFormula)
	if err != nil {
		return "", err
	}
	model := c.ChatModel
	if model == "" {
		model = DefaultFormulaChatModel
	}
	msgs := []map[string]any{}
	if systemPrompt != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": systemPrompt})
	}
	msgs = append(msgs, map[string]any{"role": "user", "content": userMsg})

	toolToURI := map[string]string{"web_search": WebSearchFormula}

	for {
		reqBody := chatRequest{Model: model, Messages: msgs, Tools: tools}
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", c.authHeader())
		req.Header.Set("Content-Type", "application/json")
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return "", err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("chat completions: %s: %s", resp.Status, truncate(body, 1024))
		}

		var cr chatResponse
		if err := json.Unmarshal(body, &cr); err != nil {
			return "", err
		}
		if cr.Error != nil && cr.Error.Message != "" {
			return "", fmt.Errorf("api error: %s", cr.Error.Message)
		}
		if len(cr.Choices) == 0 {
			return "", fmt.Errorf("empty choices")
		}
		ch := cr.Choices[0]
		finish := ch.FinishReason
		msg := ch.Message

		if finish != "tool_calls" || len(msg.ToolCalls) == 0 {
			if msg.Content != nil {
				return *msg.Content, nil
			}
			return "", nil
		}

		// 追加 assistant（含 tool_calls）
		assistant := map[string]any{
			"role": "assistant",
		}
		if msg.Content != nil {
			assistant["content"] = *msg.Content
		} else {
			assistant["content"] = nil
		}
		if len(msg.ToolCalls) > 0 {
			assistant["tool_calls"] = msg.ToolCalls
		}
		msgs = append(msgs, assistant)

		for _, tc := range msg.ToolCalls {
			id, _ := tc["id"].(string)
			fn, _ := tc["function"].(map[string]any)
			name, _ := fn["name"].(string)
			argStr, _ := fn["arguments"].(string)
			uri := toolToURI[name]
			if uri == "" {
				return "", fmt.Errorf("unknown tool %q", name)
			}
			result, _, err := c.CallFiber(uri, name, argStr)
			if err != nil {
				return "", fmt.Errorf("tool %s: %w", name, err)
			}
			msgs = append(msgs, map[string]any{
				"role":         "tool",
				"tool_call_id": id,
				"content":      result,
			})
		}
	}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// formatFlexibleAPIError 解析 error 字段（字符串或 {message,type} 对象）。
func formatFlexibleAPIError(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s
	}
	var obj struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Message != "" {
		if obj.Type != "" {
			return obj.Message + " (" + obj.Type + ")"
		}
		return obj.Message
	}
	return string(raw)
}

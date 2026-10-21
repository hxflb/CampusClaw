package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func Call(ctx context.Context, baseURL, token, method string, params any) (json.RawMessage, error) {
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		payload["params"] = params
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/mcp"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &Error{Message: "连不上 MCP 服务"}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Status: http.StatusUnauthorized, Message: "令牌被拒绝"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{Status: resp.StatusCode, Message: "MCP 服务拒绝了请求"}
	}
	var parsed struct {
		Result json.RawMessage `json:"result"`
		Err    *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &Error{Message: "MCP 响应无法解析"}
	}
	if parsed.Err != nil {
		msg := strings.TrimSpace(parsed.Err.Message)
		if msg == "" {
			msg = "MCP 调用失败"
		}
		return nil, &Error{Message: msg}
	}
	return parsed.Result, nil
}

func ListTools(ctx context.Context, baseURL, token string) ([]Tool, error) {
	if _, err := Call(ctx, baseURL, token, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "campusclaw", "version": "1.0.0"},
	}); err != nil {
		return nil, err
	}
	raw, err := Call(ctx, baseURL, token, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var listed struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, &Error{Message: "工具清单无法解析"}
	}
	out := make([]Tool, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		name := strings.TrimSpace(tool.Name)
		if !validToolName(name) {
			continue
		}
		out = append(out, Tool{Name: name, Description: clip(tool.Description, 240), InputSchema: tool.InputSchema})
	}
	return out, nil
}

// CallTool 按模型给出的参数执行一次 tools/call。参数由模型填写，宿主先用同一份 inputSchema 校验过。
func CallTool(ctx context.Context, baseURL, token, name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	raw, err := Call(ctx, baseURL, token, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", &Error{Message: "工具结果无法解析"}
	}
	var parts []string
	for _, item := range result.Content {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, strings.TrimSpace(item.Text))
		}
	}
	if len(parts) == 0 {
		return "", &Error{Message: "工具没有返回文本"}
	}
	return strings.Join(parts, "\n"), nil
}

func validToolName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

func StatusText(err error) string {
	if err == nil {
		return ""
	}
	if mcp, ok := err.(*Error); ok && mcp.Message != "" {
		return mcp.Message
	}
	return fmt.Sprintf("%v", err)
}

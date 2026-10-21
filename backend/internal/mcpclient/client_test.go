package mcpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAndCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body), "initialize"):
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}`)
		case strings.Contains(string(body), "tools/list"):
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"formula_hint","description":"提示","inputSchema":{"type":"object","properties":{"question":{"type":"string"}}}}]}}`)
		default:
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"先通分"}]}}`)
		}
	}))
	defer srv.Close()

	tools, err := ListTools(context.Background(), srv.URL, "secret")
	if err != nil || len(tools) != 1 || tools[0].Name != "formula_hint" {
		t.Fatalf("%v %+v", err, tools)
	}
	if !strings.Contains(string(tools[0].InputSchema), `"question"`) {
		t.Fatalf("inputSchema 没取回来: %s", tools[0].InputSchema)
	}
	text, err := CallTool(context.Background(), srv.URL, "secret", "formula_hint", map[string]any{"question": "分数"})
	if err != nil || text != "先通分" {
		t.Fatalf("%v %s", err, text)
	}
	if _, err := ListTools(context.Background(), srv.URL, "nope"); err == nil || !strings.Contains(err.Error(), "令牌") {
		t.Fatalf("%v", err)
	}
}

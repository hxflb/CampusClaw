package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToolsRequireTokenAndReturnText(t *testing.T) {
	token = "test-token"
	defer func() { token = "" }()

	health := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}).ServeHTTP(rec, health)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("ok")) {
		t.Fatalf("health %d %s", rec.Code, rec.Body.String())
	}

	denied := postMCP(t, "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if denied.Code != 401 {
		t.Fatalf("denied %d", denied.Code)
	}

	listed := postMCP(t, token, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if listed.Code != 200 || !bytes.Contains(listed.Body.Bytes(), []byte("formula_hint")) || !bytes.Contains(listed.Body.Bytes(), []byte("unit_convert")) || !bytes.Contains(listed.Body.Bytes(), []byte("weather_query")) {
		t.Fatalf("list %s", listed.Body.String())
	}

	called := postMCP(t, token, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"formula_hint","arguments":{"question":"这道分数题怎么通分"}}}`)
	if called.Code != 200 || !bytes.Contains(called.Body.Bytes(), []byte("分母")) {
		t.Fatalf("call %s", called.Body.String())
	}

	bad := postMCP(t, token, `not-json`)
	var payload map[string]any
	_ = json.Unmarshal(bad.Body.Bytes(), &payload)
	if bad.Code != 400 || payload["error"] == nil {
		t.Fatalf("bad request %d %s", bad.Code, bad.Body.String())
	}
}

func TestUnitConvert(t *testing.T) {
	text := unitConvert("3厘米等于多少米", nil)
	if text != "3 厘米 = 0.03 米" {
		t.Fatalf("%s", text)
	}
}

var token string

func postMCP(t *testing.T, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	if !authorized(req, token) {
		writeJSON(rec, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return rec
	}
	handleMCP(rec, req)
	return rec
}

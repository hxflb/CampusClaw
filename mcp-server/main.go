package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	token := os.Getenv("MCP_AUTH_TOKEN")
	addr := os.Getenv("MCP_LISTEN")
	if addr == "" {
		addr = ":8090"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		handleMCP(w, r)
	})
	log.Printf("mcp listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if strings.HasPrefix(strings.ToLower(got), "bearer ") {
		got = strings.TrimSpace(got[7:])
	}
	return got != "" && got == token
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil || req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		writeRPC(w, http.StatusBadRequest, nil, &rpcError{Code: -32600, Message: "非法的 JSON-RPC 请求"})
		return
	}
	switch req.Method {
	case "initialize":
		writeRPC(w, http.StatusOK, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "campusclaw-mcp", "version": "1.0.0"},
		})
	case "tools/list":
		writeRPC(w, http.StatusOK, req.ID, map[string]any{"tools": toolDefs()})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) != nil {
			writeRPC(w, http.StatusOK, req.ID, &rpcError{Code: -32602, Message: "工具参数无法解析"})
			return
		}
		text, ok := callTool(params.Name, params.Arguments)
		if !ok {
			writeRPC(w, http.StatusOK, req.ID, &rpcError{Code: -32602, Message: "没有这个工具"})
			return
		}
		writeRPC(w, http.StatusOK, req.ID, map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
			"isError": false,
		})
	default:
		writeRPC(w, http.StatusOK, req.ID, &rpcError{Code: -32601, Message: "不支持的方法"})
	}
}

func toolDefs() []map[string]any {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]string{"type": "string", "description": "学生的原问题"},
		},
	}
	weatherSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]string{"type": "string", "description": "学生的原问题"},
			"city":     map[string]string{"type": "string", "description": "要查询的城市"},
		},
	}
	return []map[string]any{
		{"name": "formula_hint", "description": "根据题目给出下一步公式提示，不计算最终得数", "inputSchema": schema},
		{"name": "unit_convert", "description": "在米、克、升及其常见单位之间换算", "inputSchema": schema},
		{"name": "weather_query", "description": "查询指定城市的当前天气，返回现象和气温", "inputSchema": weatherSchema},
	}
}

func callTool(name string, args map[string]any) (string, bool) {
	question, _ := args["question"].(string)
	switch name {
	case "formula_hint":
		return formulaHint(question), true
	case "unit_convert":
		return unitConvert(question, args), true
	case "weather_query":
		return weatherQuery(question, args), true
	default:
		return "", false
	}
}

func formulaHint(question string) string {
	switch {
	case strings.Contains(question, "分数") || strings.Contains(question, "通分"):
		return "先把分母化成相同的数，再比较分子。这里只提示下一步，不给出计算结果。"
	case strings.Contains(question, "方程") || strings.Contains(question, "未知数"):
		return "先把含未知数的项留在等号一边，已知数移到另一边。这里只提示下一步，不给出未知数的值。"
	case strings.Contains(question, "面积"):
		return "先确认图形，再选用对应的面积关系。不要直接写出最后的面积数。"
	default:
		return "先写出已知量和所求量，再选择一个关系式。不要直接给出最终得数。"
	}
}

var amountPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(千米|厘米|毫米|米|千克|克|毫升|升|km|cm|mm|kg|ml|m|g|l|L)`)

func unitConvert(question string, args map[string]any) string {
	value, from, to, ok := amountsFromArgs(args)
	if !ok {
		value, from, to, ok = amountsFromText(question)
	}
	if !ok {
		return "没有识别出要换算的数值和单位。请写明数字、原单位和目标单位，例如 3厘米等于多少米。"
	}
	if from.family != to.family {
		return "这两个单位不能直接换算。"
	}
	base := value * from.toBase
	out := base / to.toBase
	return trimFloat(value) + " " + from.label + " = " + trimFloat(out) + " " + to.label
}

type unit struct {
	label  string
	family string
	toBase float64
}

func amountsFromArgs(args map[string]any) (float64, unit, unit, bool) {
	raw, ok := args["value"]
	if !ok {
		return 0, unit{}, unit{}, false
	}
	var value float64
	switch n := raw.(type) {
	case float64:
		value = n
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, unit{}, unit{}, false
		}
		value = parsed
	default:
		return 0, unit{}, unit{}, false
	}
	fromName, _ := args["from"].(string)
	toName, _ := args["to"].(string)
	from, okFrom := lookupUnit(fromName)
	to, okTo := lookupUnit(toName)
	if !okFrom || !okTo {
		return 0, unit{}, unit{}, false
	}
	return value, from, to, true
}

func amountsFromText(question string) (float64, unit, unit, bool) {
	found := amountPattern.FindAllStringSubmatch(question, 2)
	if len(found) == 0 {
		return 0, unit{}, unit{}, false
	}
	value, err := strconv.ParseFloat(found[0][1], 64)
	if err != nil {
		return 0, unit{}, unit{}, false
	}
	from, ok := lookupUnit(found[0][2])
	if !ok {
		return 0, unit{}, unit{}, false
	}
	target := foundTarget(question)
	if target == "" && len(found) > 1 {
		target = found[1][2]
	}
	to, ok := lookupUnit(target)
	if !ok {
		return 0, unit{}, unit{}, false
	}
	return value, from, to, true
}

func foundTarget(question string) string {
	for _, name := range []string{"千米", "厘米", "毫米", "千克", "毫升", "米", "克", "升", "km", "cm", "mm", "kg", "ml", "m", "g", "l"} {
		if strings.Contains(question, "多少"+name) || strings.Contains(question, "几"+name) {
			return name
		}
	}
	return ""
}

func lookupUnit(name string) (unit, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mm", "毫米":
		return unit{"毫米", "length", 1}, true
	case "cm", "厘米":
		return unit{"厘米", "length", 10}, true
	case "m", "米":
		return unit{"米", "length", 1000}, true
	case "km", "千米":
		return unit{"千米", "length", 1000000}, true
	case "g", "克":
		return unit{"克", "mass", 1}, true
	case "kg", "千克":
		return unit{"千克", "mass", 1000}, true
	case "ml", "毫升":
		return unit{"毫升", "volume", 1}, true
	case "l", "升":
		return unit{"升", "volume", 1000}, true
	default:
		return unit{}, false
	}
}

func trimFloat(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeRPC(w http.ResponseWriter, status int, id json.RawMessage, payload any) {
	body := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage("null")}
	if len(id) > 0 {
		body["id"] = id
	}
	if errBody, ok := payload.(*rpcError); ok {
		body["error"] = errBody
	} else {
		body["result"] = payload
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"campusclaw/internal/mcpclient"
	"campusclaw/internal/tutor"
)

var (
	errToolDenied = errors.New("mcp_tool_denied")
	serverNameRE  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`)
)

type skillTool struct {
	Ref    string
	Tool   string
	Server string
}

type mcpServerRow struct {
	ID        int
	Name      string
	BaseURL   string
	Token     string
	Connected bool
}

type mcpToolRow struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

func (s *Server) listMcpServers(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	rows, err := s.db.Query(`SELECT id, name, base_url, token, connected FROM mcp_servers WHERE class_id=? ORDER BY id`, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var row mcpServerRow
		var connected int
		if err := rows.Scan(&row.ID, &row.Name, &row.BaseURL, &row.Token, &connected); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		items = append(items, serverJSON(row, connected == 1, s.mcpTools(row.ID), s.hasMcpToken(row)))
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

// 令牌只写不读：任何响应里都没有原文，只有 has_token 这个布尔。
func (s *Server) hasMcpToken(row mcpServerRow) bool {
	return strings.TrimSpace(row.Token) != "" || strings.TrimSpace(s.cfg.McpToken) != ""
}

func serverJSON(row mcpServerRow, connected bool, tools []mcpToolRow, hasToken bool) map[string]any {
	// 断开后对外清单清空；最近一次发现结果只留作内部快照。
	shown := []map[string]any{}
	if connected {
		for _, tool := range tools {
			shown = append(shown, map[string]any{"name": tool.Name, "description": tool.Description})
		}
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "base_url": row.BaseURL,
		"connected": connected, "has_token": hasToken, "tools": shown,
	}
}

func (s *Server) createMcpServer(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	var body struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
		ClassID int    `json:"class_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	_ = body.ClassID
	name := strings.TrimSpace(body.Name)
	base := strings.TrimSpace(body.BaseURL)
	if !serverNameRE.MatchString(name) || !validBaseURL(base) {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	res, err := s.db.Exec(`INSERT INTO mcp_servers (class_id, name, base_url, token, connected) VALUES (?, ?, ?, ?, 0)`,
		u.ClassID, name, base, strings.TrimSpace(body.Token))
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, 400, map[string]string{"error": "name_taken"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	id, _ := res.LastInsertId()
	row := mcpServerRow{ID: int(id), Name: name, BaseURL: base, Token: strings.TrimSpace(body.Token)}
	writeJSON(w, 201, serverJSON(row, false, nil, s.hasMcpToken(row)))
}

func (s *Server) connectMcpServer(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	row, ok := s.ownMcpServer(w, r, u)
	if !ok {
		return
	}
	token := strings.TrimSpace(row.Token)
	if token == "" {
		token = strings.TrimSpace(s.cfg.McpToken)
	}
	if token == "" {
		writeJSON(w, 502, map[string]string{"error": "mcp_unavailable", "message": "服务端还没有配置 MCP 令牌"})
		return
	}
	tools, err := mcpclient.ListTools(r.Context(), row.BaseURL, token)
	if err != nil {
		// 握手或发现失败：停在未连接，把提示交给调用者。
		_, _ = s.db.Exec(`UPDATE mcp_servers SET connected=0 WHERE id=? AND class_id=?`, row.ID, u.ClassID)
		writeJSON(w, 502, map[string]string{"error": "mcp_unavailable", "message": mcpclient.StatusText(err)})
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if _, err := tx.Exec(`DELETE FROM mcp_tools WHERE server_id=?`, row.ID); err != nil {
		tx.Rollback()
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	stored := make([]mcpToolRow, 0, len(tools))
	for _, tool := range tools {
		schema := string(tool.InputSchema)
		if _, err := tx.Exec(`INSERT INTO mcp_tools (server_id, name, description, input_schema) VALUES (?, ?, ?, ?)`,
			row.ID, tool.Name, tool.Description, schema); err != nil {
			tx.Rollback()
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		stored = append(stored, mcpToolRow{Name: tool.Name, Description: tool.Description, Schema: tool.InputSchema})
	}
	if _, err := tx.Exec(`UPDATE mcp_servers SET connected=1 WHERE id=? AND class_id=?`, row.ID, u.ClassID); err != nil {
		tx.Rollback()
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, serverJSON(row, true, stored, s.hasMcpToken(row)))
}

func (s *Server) disconnectMcpServer(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	row, ok := s.ownMcpServer(w, r, u)
	if !ok {
		return
	}
	if _, err := s.db.Exec(`UPDATE mcp_servers SET connected=0 WHERE id=? AND class_id=?`, row.ID, u.ClassID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, serverJSON(row, false, s.mcpTools(row.ID), s.hasMcpToken(row)))
}

func (s *Server) deleteMcpServer(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	row, ok := s.ownMcpServer(w, r, u)
	if !ok {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM mcp_servers WHERE id=? AND class_id=?`, row.ID, u.ClassID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": row.ID})
}

func (s *Server) listMcpAudit(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	if !s.teacherOnly(w, u) {
		return
	}
	rows, err := s.db.Query(`SELECT a.tool_ref, a.status, a.round_no, a.conversation_id, users.username, a.created_at
		FROM mcp_audit a JOIN users ON users.id=a.user_id
		WHERE a.class_id=? ORDER BY a.id DESC LIMIT 100`, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var tool, status, username string
		var roundNo int
		var convID sql.NullInt64
		var created sql.NullTime
		if err := rows.Scan(&tool, &status, &roundNo, &convID, &username, &created); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		stamp := ""
		if created.Valid {
			stamp = created.Time.Format("2006-01-02 15:04:05")
		}
		item := map[string]any{
			"tool": tool, "status": status, "round": roundNo,
			"username": username, "created_at": stamp,
		}
		if convID.Valid {
			item["conversation_id"] = convID.Int64
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) teacherOnly(w http.ResponseWriter, u sessionUser) bool {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return false
	}
	return true
}

func (s *Server) ownMcpServer(w http.ResponseWriter, r *http.Request, u sessionUser) (mcpServerRow, bool) {
	var row mcpServerRow
	var connected int
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return row, false
	}
	err = s.db.QueryRow(`SELECT id, name, base_url, token, connected FROM mcp_servers WHERE id=? AND class_id=?`, id, u.ClassID).
		Scan(&row.ID, &row.Name, &row.BaseURL, &row.Token, &connected)
	if err == sql.ErrNoRows {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return row, false
	}
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return row, false
	}
	row.Connected = connected == 1
	return row, true
}

func (s *Server) mcpServerByName(classID int, name string) (mcpServerRow, error) {
	var row mcpServerRow
	var connected int
	err := s.db.QueryRow(`SELECT id, name, base_url, token, connected FROM mcp_servers WHERE class_id=? AND name=?`, classID, name).
		Scan(&row.ID, &row.Name, &row.BaseURL, &row.Token, &connected)
	row.Connected = connected == 1
	return row, err
}

func (s *Server) mcpTools(serverID int) []mcpToolRow {
	rows, err := s.db.Query(`SELECT name, description, COALESCE(input_schema,'') FROM mcp_tools WHERE server_id=? ORDER BY name`, serverID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []mcpToolRow{}
	for rows.Next() {
		var row mcpToolRow
		var schema string
		if err := rows.Scan(&row.Name, &row.Description, &schema); err != nil {
			return out
		}
		row.Schema = json.RawMessage(schema)
		out = append(out, row)
	}
	return out
}

// toolAllowed 用于保存技能时校验引用：服务器属于本班、已连接，且工具在最近一次发现结果里。
func (s *Server) toolAllowed(classID int, tool string) error {
	if tool == "knowledge_search" {
		return nil
	}
	name, server, ok := strings.Cut(tool, "@")
	if !ok {
		return errToolDenied
	}
	var serverID, connected int
	err := s.db.QueryRow(`SELECT id, connected FROM mcp_servers WHERE class_id=? AND name=?`, classID, server).Scan(&serverID, &connected)
	if err == sql.ErrNoRows || connected != 1 {
		return errToolDenied
	}
	if err != nil {
		return err
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mcp_tools WHERE server_id=? AND name=?`, serverID, name).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return errToolDenied
	}
	return nil
}

func (s *Server) ensureTool(w http.ResponseWriter, classID int, tool string) bool {
	err := s.toolAllowed(classID, tool)
	if err == nil {
		return true
	}
	if errors.Is(err, errToolDenied) {
		writeJSON(w, 400, map[string]string{"error": "mcp_tool_denied"})
		return false
	}
	writeJSON(w, 503, map[string]string{"error": "unavailable"})
	return false
}

// —— 调用循环的宿主侧 ——

type toolBinding struct {
	ref    string
	server string
	name   string
	spec   tutor.ToolSpec
}

type toolSession struct {
	s      *Server
	u      sessionUser
	convID int64
	budget int
	order  []string
	byName map[string]toolBinding
}

// newToolSession 把本次选中技能引用的工具变成候选集。声明三项取自发现结果（快照），
// 因此服务器断开后模型仍可能提出调用，由执行环节判「被拒」并留记录。
func (s *Server) newToolSession(u sessionUser, convID int64, tools []skillTool) *toolSession {
	budget := s.cfg.MaxToolCalls
	if budget <= 0 {
		budget = 4
	}
	sess := &toolSession{s: s, u: u, convID: convID, budget: budget, byName: map[string]toolBinding{}}
	for _, st := range tools {
		row, err := s.mcpServerByName(u.ClassID, st.Server)
		if err != nil {
			continue
		}
		for _, tool := range s.mcpTools(row.ID) {
			if tool.Name != st.Tool {
				continue
			}
			if _, seen := sess.byName[tool.Name]; seen {
				continue
			}
			sess.byName[tool.Name] = toolBinding{
				ref: st.Ref, server: st.Server, name: tool.Name,
				spec: tutor.ToolSpec{Ref: st.Ref, Name: tool.Name, Description: tool.Description, Schema: tool.Schema},
			}
			sess.order = append(sess.order, tool.Name)
		}
	}
	return sess
}

func (t *toolSession) Specs() []tutor.ToolSpec {
	if t == nil || t.budget <= 0 || len(t.order) == 0 {
		return nil
	}
	out := make([]tutor.ToolSpec, 0, len(t.order))
	for _, name := range t.order {
		out = append(out, t.byName[name].spec)
	}
	return out
}

func (t *toolSession) Execute(ctx context.Context, round int, calls []tutor.ToolCall) []tutor.CallRecord {
	out := make([]tutor.CallRecord, 0, len(calls))
	for _, call := range calls {
		out = append(out, t.run(ctx, round, call))
	}
	return out
}

func (t *toolSession) run(ctx context.Context, round int, call tutor.ToolCall) tutor.CallRecord {
	binding, ok := t.byName[call.Function.Name]
	if !ok {
		return tutor.CallRecord{Ref: call.Function.Name, Status: "denied", Text: "调用被宿主拒绝：这个工具不在本次候选集里。"}
	}
	rec := tutor.CallRecord{Ref: binding.ref, Status: "denied"}
	if t.budget <= 0 {
		rec.Text = "调用被宿主拒绝：本次提问的调用次数已达上限。"
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	t.budget--
	args, err := decodeToolArgs(call.Function.Arguments)
	if err != nil {
		rec.Status, rec.Text = "error", "工具调用失败：参数不是合法的 JSON 对象。"
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	if err := validateToolArgs(binding.spec.Schema, args); err != nil {
		rec.Status, rec.Text = "error", "工具调用失败："+err.Error()
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	row, err := t.s.mcpServerByName(t.u.ClassID, binding.server)
	if err == sql.ErrNoRows {
		// 服务器已不存在：这次失败无法归属，只体现在应答里。
		rec.Text = "调用被宿主拒绝：该工具所在服务器已不存在。"
		return rec
	}
	if err != nil {
		rec.Status, rec.Text = "error", "工具调用失败：服务端读取失败。"
		return rec
	}
	if !row.Connected {
		rec.Text = "调用被宿主拒绝：该服务器当前未连接。"
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	token := strings.TrimSpace(row.Token)
	if token == "" {
		token = strings.TrimSpace(t.s.cfg.McpToken)
	}
	if token == "" {
		rec.Text = "调用被宿主拒绝：服务端没有配置调用令牌。"
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	text, callErr := mcpclient.CallTool(ctx, row.BaseURL, token, binding.name, args)
	if callErr != nil {
		rec.Status = "error"
		if mcp, ok := callErr.(*mcpclient.Error); ok && mcp.Status == http.StatusUnauthorized {
			rec.Status = "denied"
		}
		rec.Text = "工具调用失败：" + mcpclient.StatusText(callErr)
		t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
		return rec
	}
	if blocked, note := scanToolText(text); blocked {
		text = note
	}
	rec.Status, rec.Text = "ok", text
	t.s.writeMcpAudit(t.u, t.convID, round, binding.ref, rec.Status)
	return rec
}

func (s *Server) writeMcpAudit(u sessionUser, convID int64, round int, ref, status string) {
	_, err := s.db.Exec(`INSERT INTO mcp_audit (class_id, user_id, conversation_id, round_no, tool_ref, status) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ClassID, u.UserID, convID, round, ref, status)
	if err != nil {
		log.Printf("mcp audit: %v", err)
	}
}

func decodeToolArgs(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	return args, nil
}

type toolParam struct {
	Type string `json:"type"`
}

type toolSchema struct {
	Properties map[string]toolParam `json:"properties"`
	Required   []string             `json:"required"`
}

// validateToolArgs 用发现结果里的 inputSchema 校验模型填的参数，与模型看到的是同一份结构。
func validateToolArgs(schema json.RawMessage, args map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	var s toolSchema
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil
	}
	for _, key := range s.Required {
		v, ok := args[key]
		if !ok || v == nil {
			return fmt.Errorf("缺少必填参数 %s", key)
		}
		if str, ok := v.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("参数 %s 不能为空", key)
		}
	}
	for key, prop := range s.Properties {
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch prop.Type {
		case "string":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("参数 %s 应为字符串", key)
			}
		case "number":
			if _, ok := v.(float64); !ok {
				return fmt.Errorf("参数 %s 应为数字", key)
			}
		case "integer":
			f, ok := v.(float64)
			if !ok || f != math.Trunc(f) {
				return fmt.Errorf("参数 %s 应为整数", key)
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("参数 %s 应为布尔值", key)
			}
		}
	}
	return nil
}

// 已知的指令式句式。命中即整条丢弃：被改过的文本可能自相矛盾，删一句后剩下的仍可能是构造过的。
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?(the\s+)?(previous|prior|above)\s+instructions?`),
	regexp.MustCompile(`(?i)system\s+prompt`),
	regexp.MustCompile(`忽略(之前|以上|上面|先前|前面)的?(所有)?(指令|要求|提示|设定)`),
	regexp.MustCompile(`(不要|别)(告诉|告知|让)用户`),
	regexp.MustCompile(`(把|将)(以下|下面|这些)?内容(发送|转发|上传|提交)(到|至)`),
	regexp.MustCompile(`你现在是|从现在起你是|现在开始你是`),
}

const blockedToolNote = "（工具返回的内容触发了安全检查，已整条丢弃。）"

func scanToolText(text string) (bool, string) {
	for _, re := range injectionPatterns {
		if re.MatchString(text) {
			return true, blockedToolNote
		}
	}
	return false, ""
}

func validBaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != "" && parsed.Hostname() != ""
}

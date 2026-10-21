package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"campusclaw/internal/retrieve"
	"campusclaw/internal/tutor"
)

func (s *Server) getPrompt(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	body, err := s.classPrompt(u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"body": body, "prompt_sha256": tutor.PromptDigest(body)})
}

func (s *Server) putPrompt(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	var body struct {
		Body    string `json:"body"`
		ClassID int    `json:"class_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	text := strings.TrimSpace(body.Body)
	if text == "" || len([]rune(text)) > 4000 {
		writeJSON(w, 400, map[string]string{"error": "empty_prompt"})
		return
	}
	_, err := s.db.Exec(`INSERT INTO class_prompts (class_id, body, updated_by) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE body=VALUES(body), updated_by=VALUES(updated_by)`, u.ClassID, text, u.UserID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "prompt_sha256": tutor.PromptDigest(text)})
}

func (s *Server) getSkills(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	on, err := s.skillEnabled(u.ClassID, tutor.SkillGuide)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"items": []map[string]any{{
		"key": tutor.SkillGuide, "name": "解题引导", "enabled": on,
	}}})
}

func (s *Server) putSkill(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if r.PathValue("key") != tutor.SkillGuide {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
		ClassID int   `json:"class_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	if body.Enabled == nil {
		writeJSON(w, 400, map[string]string{"error": "empty_skill"})
		return
	}
	en := 0
	if *body.Enabled {
		en = 1
	}
	_, err := s.db.Exec(`INSERT INTO class_skills (class_id, skill_key, enabled, updated_by) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE enabled=VALUES(enabled), updated_by=VALUES(updated_by)`,
		u.ClassID, tutor.SkillGuide, en, u.UserID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "enabled": *body.Enabled})
}

func (s *Server) listConversations(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	rows, err := s.db.Query(`SELECT c.id, c.title, c.agent_id, c.created_at
		FROM conversations c WHERE c.user_id=? AND c.class_id=?
		ORDER BY COALESCE(c.updated_at, c.created_at) DESC, c.id DESC`, u.UserID, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var title string
		var agentID sql.NullInt64
		var created sql.NullTime
		if err := rows.Scan(&id, &title, &agentID, &created); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		stamp := ""
		if created.Valid {
			stamp = created.Time.Format("2006-01-02 15:04:05")
		}
		if strings.TrimSpace(title) == "" {
			title = "新的提问"
		}
		agent := 0
		if agentID.Valid {
			agent = int(agentID.Int64)
		}
		items = append(items, map[string]any{
			"id": id, "title": title, "agent_id": agent, "created_at": stamp, "preview": title,
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request, u sessionUser) {
	var body struct {
		ClassID int `json:"class_id"`
		AgentID int `json:"agent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	var enabled int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM class_agents WHERE class_id=? AND enabled=1`, u.ClassID).Scan(&enabled); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if enabled == 0 {
		writeJSON(w, 400, map[string]string{"error": "no_agent"})
		return
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM class_agents WHERE id=? AND class_id=? AND enabled=1`, body.AgentID, u.ClassID).Scan(&n); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if n != 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	res, err := s.db.Exec(`INSERT INTO conversations (class_id, user_id, agent_id, title, updated_at) VALUES (?, ?, ?, '新的提问', NOW())`, u.ClassID, u.UserID, body.AgentID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 200, map[string]any{"id": id, "agent_id": body.AgentID, "title": "新的提问"})
}

func (s *Server) setConversationAgent(w http.ResponseWriter, r *http.Request, u sessionUser) {
	id, ok := s.ownConversation(r, u)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		AgentID int `json:"agent_id"`
		ClassID int `json:"class_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	var messages int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=?`, id).Scan(&messages); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if messages > 0 {
		writeJSON(w, 409, map[string]string{"error": "started"})
		return
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM class_agents WHERE id=? AND class_id=? AND enabled=1`, body.AgentID, u.ClassID).Scan(&n); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if n != 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if _, err := s.db.Exec(`UPDATE conversations SET agent_id=? WHERE id=? AND user_id=? AND class_id=?`, body.AgentID, id, u.UserID, u.ClassID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "agent_id": body.AgentID})
}

func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request, u sessionUser) {
	id, ok := s.ownConversation(r, u)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	res, err := s.db.Exec(`DELETE FROM conversations WHERE id=? AND user_id=? AND class_id=?`, id, u.UserID, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request, u sessionUser) {
	id, ok := s.ownConversation(r, u)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	var title string
	var agentID sql.NullInt64
	if err := s.db.QueryRow(`SELECT title, agent_id FROM conversations WHERE id=?`, id).Scan(&title, &agentID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	rows, err := s.db.Query(`SELECT id, role, content, citations_json, skill_name FROM conversation_messages WHERE conversation_id=? ORDER BY id`, id)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	msgs := []map[string]any{}
	for rows.Next() {
		var mid int64
		var role, content string
		var raw, skillName sql.NullString
		if err := rows.Scan(&mid, &role, &content, &raw, &skillName); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		item := map[string]any{"id": mid, "role": role, "content": content}
		if skillName.Valid {
			item["skill_name"] = skillName.String
		}
		if role == "assistant" {
			cites := []tutor.Citation{}
			if raw.Valid && raw.String != "" {
				_ = json.Unmarshal([]byte(raw.String), &cites)
			}
			if cites == nil {
				cites = []tutor.Citation{}
			}
			item["citations"] = cites
		}
		msgs = append(msgs, item)
	}
	agent := 0
	if agentID.Valid {
		agent = int(agentID.Int64)
	}
	writeJSON(w, 200, map[string]any{"id": id, "title": title, "agent_id": agent, "messages": msgs})
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request, u sessionUser) {
	// 1 取会话：按编号与当前用户读取，再比对班级
	id, ok := s.ownConversation(r, u)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		Query    string           `json:"query"`
		System   string           `json:"system"`
		ClassID  int              `json:"class_id"`
		AgentID  int              `json:"agent_id"`
		Messages []map[string]any `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_, _, _, _ = body.System, body.ClassID, body.AgentID, body.Messages
	q := strings.TrimSpace(body.Query)
	if q == "" || len([]rune(q)) > 2000 {
		writeJSON(w, 400, map[string]string{"error": "empty_query"})
		return
	}
	// 2 取智能体：读会话选定的智能体，再比对班级
	ag, err := s.conversationAgent(u, id)
	if err == sql.ErrNoRows {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	// 3 取候选集：只取该智能体绑定且已启用的技能
	skills, err := s.routableSkills(u.ClassID, ag.SkillIDs)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	// 4 选技能：候选集内按适用条件打分，至多一个
	picked, okSkill := tutor.PickSkill(q, skills)
	subject := ""
	skillName, skillBody := "", ""
	var tools []skillTool
	if okSkill {
		skillName, skillBody = picked.Name, picked.Body
		subject = picked.Subject
		if name, server, ok := strings.Cut(picked.Tool, "@"); ok && name != "" && server != "" {
			tools = []skillTool{{Ref: picked.Tool, Tool: name, Server: server}}
		}
	}
	// 5 记问题：先落库，后面的检索失败也不会丢掉这一轮提问
	if _, err := s.db.Exec(`INSERT INTO conversation_messages (conversation_id, role, content) VALUES (?, 'user', ?)`, id, q); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if _, err := s.touchConversation(id, q); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	// 6、7 检索并筛切片：只在绑定材料内取前 4 条，再去掉学科不符与空正文
	hits, err := s.loadTutorHits(u.ClassID, q, ag.MaterialIDs, subject)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "vector_unavailable"})
		return
	}
	// 工具候选集来自本次选中的技能；声明三项取自发现结果
	toolSess := s.newToolSession(u, id, tools)
	prompt := ag.Prompt
	systemPrompt := tutor.BuildMessages(prompt, skillName, skillBody, hits, nil, q)[0].Content
	cites := tutor.Citations(hits)
	guideOn := strings.Contains(skillName, "解题引导")
	skillMeta := map[string]any{"key": "", "name": "", "enabled": false}
	if okSkill {
		skillMeta = map[string]any{"key": tutor.SkillGuide, "name": skillName, "enabled": guideOn}
	}
	// 8 推 meta：状态、技能与引用列表先到；读历史与组消息都排在这一帧之后
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := writeSSE(w, "meta", map[string]any{
		"status":        "ok",
		"skill":         skillMeta,
		"citations":     cites,
		"hit_count":     len(hits),
		"prompt_sha256": tutor.PromptDigest(systemPrompt),
	}); err != nil {
		return
	}
	// 9 组消息：读最近 6 条历史，角色与技能进系统消息，资料进用户消息
	hist, err := s.chatHistory(id)
	if err != nil {
		_ = writeSSE(w, "error", map[string]string{"error": "unavailable"})
		return
	}
	if n := len(hist); n > 0 && hist[n-1].Role == "user" {
		hist = hist[:n-1]
	}
	// 10 生成：模型提议调用、宿主执行并回传，逐轮推 tool_calls 与 delta
	emitCalls := func(round int, calls []tutor.CallRecord) error {
		return writeSSE(w, "tool_calls", map[string]any{"round": round, "calls": calls})
	}
	out, err := tutor.Stream(r.Context(), s.cfg, tutor.Input{
		Prompt: prompt, SkillName: skillName, SkillBody: skillBody, Question: q, Hits: hits, History: hist,
	}, toolSess, func(text string) error {
		return writeSSE(w, "delta", map[string]string{"text": text})
	}, emitCalls)
	if out.Answer == "" {
		_ = writeSSE(w, "error", map[string]string{"error": "stream_failed"})
		return
	}
	// 11 收尾：助手消息连同当次证据落库，再推 done
	raw, _ := json.Marshal(out.Citations)
	skillInt := 0
	if guideOn {
		skillInt = 1
	}
	res, dbErr := s.db.Exec(`INSERT INTO conversation_messages
		(conversation_id, role, content, citations_json, prompt_snapshot, skill_enabled, skill_name)
		VALUES (?, 'assistant', ?, ?, ?, ?, ?)`, id, out.Answer, string(raw), systemPrompt, skillInt, skillName)
	if dbErr != nil {
		_ = writeSSE(w, "error", map[string]string{"error": "save_failed"})
		return
	}
	mid, _ := res.LastInsertId()
	_ = writeSSE(w, "done", map[string]any{
		"answer": out.Answer, "message_id": mid, "model_called": out.ModelCalled,
	})
	_ = err
}

func (s *Server) classPrompt(classID int) (string, error) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM class_prompts WHERE class_id=?`, classID).Scan(&body)
	if err == sql.ErrNoRows {
		return tutor.DefaultPrompt, nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return tutor.DefaultPrompt, nil
	}
	return body, nil
}

func (s *Server) skillEnabled(classID int, key string) (bool, error) {
	var en int
	err := s.db.QueryRow(`SELECT enabled FROM class_skills WHERE class_id=? AND skill_key=?`, classID, key).Scan(&en)
	if err == sql.ErrNoRows {
		return true, nil
	}
	return en == 1, err
}

func (s *Server) ownConversation(r *http.Request, u sessionUser) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	var n int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE id=? AND user_id=? AND class_id=?`, id, u.UserID, u.ClassID).Scan(&n)
	return id, err == nil && n == 1
}

func keepCheckedMaterials(hits []retrieve.Hit, materialIDs []int) []retrieve.Hit {
	if len(materialIDs) == 0 {
		return []retrieve.Hit{}
	}
	allow := map[int]bool{}
	for _, id := range materialIDs {
		allow[id] = true
	}
	filtered := make([]retrieve.Hit, 0, len(hits))
	for _, h := range hits {
		if allow[h.MaterialID] {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func (s *Server) conversationAgent(u sessionUser, convID int64) (classAgent, error) {
	var agentID sql.NullInt64
	err := s.db.QueryRow(`SELECT agent_id FROM conversations WHERE id=? AND user_id=? AND class_id=?`, convID, u.UserID, u.ClassID).Scan(&agentID)
	if err != nil {
		return classAgent{}, err
	}
	if !agentID.Valid || agentID.Int64 <= 0 {
		ag, err := s.resolveAgent(u, 0)
		if err != nil {
			return classAgent{}, err
		}
		if _, err := s.db.Exec(`UPDATE conversations SET agent_id=? WHERE id=? AND agent_id IS NULL`, ag.ID, convID); err != nil {
			return classAgent{}, err
		}
		return ag, nil
	}
	return s.loadAgent(u.ClassID, int(agentID.Int64))
}

func (s *Server) loadAgent(classID, id int) (classAgent, error) {
	var row classAgent
	var en int
	err := s.db.QueryRow(`SELECT id, name, prompt, enabled FROM class_agents WHERE id=? AND class_id=?`, id, classID).
		Scan(&row.ID, &row.Name, &row.Prompt, &en)
	if err != nil {
		return classAgent{}, err
	}
	row.Enabled = en == 1
	row.SkillIDs, row.MaterialIDs, err = s.agentLinks(row.ID)
	if err != nil {
		return classAgent{}, err
	}
	row.Guide, row.SkillNote, row.SkillBodies, row.Tools, err = s.skillEffect(classID, row.SkillIDs)
	return row, err
}

func (s *Server) touchConversation(id int64, question string) (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=? AND role='user'`, id).Scan(&n); err != nil {
		return 0, err
	}
	if n == 1 {
		title := strings.TrimSpace(question)
		rs := []rune(title)
		if len(rs) > 24 {
			title = string(rs[:24])
		}
		if title == "" {
			title = "新的提问"
		}
		_, err := s.db.Exec(`UPDATE conversations SET title=?, updated_at=NOW() WHERE id=?`, title, id)
		return n, err
	}
	_, err := s.db.Exec(`UPDATE conversations SET updated_at=NOW() WHERE id=?`, id)
	return n, err
}

func (s *Server) loadTutorHits(classID int, question string, materialIDs []int, subject string) ([]tutor.Hit, error) {
	if len(materialIDs) == 0 {
		return []tutor.Hit{}, nil
	}
	hits, _, err := retrieve.Hybrid(s.db, s.idx.Qdrant(), s.idx.Embedder(), classID, question)
	if err != nil {
		return nil, err
	}
	out := make([]tutor.Hit, 0, 4)
	for _, h := range selectHits(hits, materialIDs, subject) {
		var text string
		_ = s.db.QueryRow(`SELECT chunk_text FROM knowledge_chunks WHERE id=? AND class_id=?`, h.ChunkID, classID).Scan(&text)
		if strings.TrimSpace(text) == "" {
			text = h.Excerpt
		}
		if strings.TrimSpace(text) == "" {
			continue // 空正文切片不进编号，避免 [n] 指向一条没有内容的资料
		}
		out = append(out, tutor.Hit{
			MaterialID: h.MaterialID, Title: h.Title, ChunkID: h.ChunkID,
			ChunkIndex: h.ChunkIndex, CharStart: h.CharStart, CharEnd: h.CharEnd, Text: text,
		})
	}
	return out, nil
}

// selectHits 按 answer.html 的第 6、7 步：先只留绑定材料并取前 4 条，再按学科筛。
func selectHits(hits []retrieve.Hit, materialIDs []int, subject string) []retrieve.Hit {
	kept := keepCheckedMaterials(hits, materialIDs)
	if len(kept) > 4 {
		kept = kept[:4]
	}
	return keepSubject(kept, subject)
}

func keepSubject(hits []retrieve.Hit, subject string) []retrieve.Hit {
	subject = strings.ToLower(strings.TrimSpace(subject))
	if subject == "" {
		return hits
	}
	out := make([]retrieve.Hit, 0, len(hits))
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.Title), subject) {
			out = append(out, h)
		}
	}
	return out
}

func (s *Server) chatHistory(convID int64) ([]tutor.Turn, error) {
	rows, err := s.db.Query(`SELECT role, content FROM conversation_messages WHERE conversation_id=? ORDER BY id`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []tutor.Turn
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		turns = append(turns, tutor.Turn{Role: role, Content: content})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tutor.ContextWindow(turns), nil
}

func writeSSE(w http.ResponseWriter, event string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func clipRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

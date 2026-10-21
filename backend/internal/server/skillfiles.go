package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"campusclaw/internal/skillfile"
	"campusclaw/internal/tutor"
)

func (s *Server) listSkillFiles(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	q := `SELECT id, name, trigger_text, subject, tool, enabled, body, source FROM skill_files WHERE class_id=?`
	if u.Role != "teacher" {
		q += ` AND enabled=1`
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, err := scanSkill(rows)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) createSkillFile(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	doc, ok := readSkillSource(w, r)
	if !ok {
		return
	}
	if !s.ensureTool(w, u.ClassID, doc.Tool) {
		return
	}
	res, err := s.db.Exec(`INSERT INTO skill_files (class_id, name, trigger_text, subject, tool, enabled, body, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, u.ClassID, doc.Name, doc.Trigger, doc.Subject, doc.Tool, boolInt(doc.Enabled), doc.Body, doc.Source)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, 400, map[string]string{"error": "name_taken"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 200, skillJSON(int(id), doc))
}

func (s *Server) updateSkillFile(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id, ok := s.ownSkill(w, r, u)
	if !ok {
		return
	}
	doc, ok := readSkillSource(w, r)
	if !ok {
		return
	}
	if !s.ensureTool(w, u.ClassID, doc.Tool) {
		return
	}
	_, err := s.db.Exec(`UPDATE skill_files SET name=?, trigger_text=?, subject=?, tool=?, enabled=?, body=?, source=? WHERE id=? AND class_id=?`,
		doc.Name, doc.Trigger, doc.Subject, doc.Tool, boolInt(doc.Enabled), doc.Body, doc.Source, id, u.ClassID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, 400, map[string]string{"error": "name_taken"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, skillJSON(id, doc))
}

func (s *Server) setSkillFileEnabled(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id, ok := s.ownSkill(w, r, u)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
		ClassID int   `json:"class_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	_ = body.ClassID
	var source string
	if err := s.db.QueryRow(`SELECT source FROM skill_files WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&source); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	source = flipEnabledLine(source, *body.Enabled)
	if _, err := s.db.Exec(`UPDATE skill_files SET enabled=?, source=? WHERE id=? AND class_id=?`, boolInt(*body.Enabled), source, id, u.ClassID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id, "enabled": *body.Enabled, "source": source})
}

func flipEnabledLine(source string, on bool) string {
	from, to := "启用：是", "启用：否"
	if on {
		from, to = "启用：否", "启用：是"
	}
	return strings.Replace(source, from, to, 1)
}

func (s *Server) deleteSkillFile(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id, ok := s.ownSkill(w, r, u)
	if !ok {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM skill_files WHERE id=? AND class_id=?`, id, u.ClassID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) ownSkill(w http.ResponseWriter, r *http.Request, u sessionUser) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return 0, false
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM skill_files WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&n); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return 0, false
	}
	if n != 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return 0, false
	}
	return id, true
}

func readSkillSource(w http.ResponseWriter, r *http.Request) (skillfile.Doc, bool) {
	var body struct {
		Source  string `json:"source"`
		ClassID int    `json:"class_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return skillfile.Doc{}, false
	}
	_ = body.ClassID
	doc, ok := skillfile.Parse(body.Source)
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "bad_skill"})
		return skillfile.Doc{}, false
	}
	return doc, true
}

type skillScanner interface {
	Scan(dest ...any) error
}

func scanSkill(rows skillScanner) (map[string]any, error) {
	var id, en int
	var name, trigger, subject, tool, body, source string
	if err := rows.Scan(&id, &name, &trigger, &subject, &tool, &en, &body, &source); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "name": name, "trigger": trigger, "subject": subject, "tool": tool,
		"enabled": en == 1, "body": body, "source": source,
	}, nil
}

func skillJSON(id int, doc skillfile.Doc) map[string]any {
	return map[string]any{
		"id": id, "name": doc.Name, "trigger": doc.Trigger, "subject": doc.Subject, "tool": doc.Tool,
		"enabled": doc.Enabled, "body": doc.Body, "source": doc.Source,
	}
}

type skillBody struct {
	Name string
	Body string
}

func (s *Server) skillEffect(classID int, ids []int) (bool, string, []skillBody, []skillTool, error) {
	if len(ids) == 0 {
		return false, "", nil, nil, nil
	}
	placeholders := make([]string, len(ids))
	args := []any{classID}
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT id, name, trigger_text, subject, tool, enabled, body FROM skill_files WHERE class_id=? AND id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return false, "", nil, nil, err
	}
	defer rows.Close()
	type skillNote struct {
		name, trigger, subject, tool, body string
		enabled                            bool
	}
	found := map[int]skillNote{}
	for rows.Next() {
		var id, en int
		var item skillNote
		if err := rows.Scan(&id, &item.name, &item.trigger, &item.subject, &item.tool, &en, &item.body); err != nil {
			return false, "", nil, nil, err
		}
		item.enabled = en == 1
		found[id] = item
	}
	guide := false
	var notes []string
	var bodies []skillBody
	var tools []skillTool
	for _, id := range ids {
		item, ok := found[id]
		if !ok || !item.enabled {
			continue
		}
		if strings.Contains(item.name, "解题引导") {
			guide = true
		}
		if name, server, ok := strings.Cut(item.tool, "@"); ok && name != "" && server != "" {
			tools = append(tools, skillTool{Ref: item.tool, Tool: name, Server: server})
		}
		notes = append(notes, "技能「"+item.name+"」\n适用条件："+item.trigger+"\n资料："+item.subject)
		if body := strings.TrimSpace(item.body); body != "" {
			bodies = append(bodies, skillBody{Name: item.name, Body: body})
		}
	}
	if len(notes) == 0 {
		return guide, "", bodies, tools, nil
	}
	catalog := "已启用技能只列出名称、适用条件和资料，正文未附上。\n\n" + strings.Join(notes, "\n\n")
	return guide, catalog, bodies, tools, nil
}

func (s *Server) routableSkills(classID int, ids []int) ([]tutor.SkillCandidate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := []any{classID}
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT id, name, trigger_text, subject, body, tool FROM skill_files
		WHERE class_id=? AND enabled=1 AND id IN (`+strings.Join(placeholders, ",")+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tutor.SkillCandidate
	for rows.Next() {
		var item tutor.SkillCandidate
		if err := rows.Scan(&item.ID, &item.Name, &item.Trigger, &item.Subject, &item.Body, &item.Tool); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

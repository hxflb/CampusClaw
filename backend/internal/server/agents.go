package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type classAgent struct {
	ID          int
	Name        string
	Prompt      string
	Enabled     bool
	SkillIDs    []int
	MaterialIDs []int
	Guide       bool
	SkillNote   string
	SkillBodies []skillBody
	Tools       []skillTool
}

func (s *Server) listAgents(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	q := `SELECT id, name, prompt, enabled FROM class_agents WHERE class_id=?`
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
		var id, en int
		var name, prompt string
		if err := rows.Scan(&id, &name, &prompt, &en); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		skills, materials, err := s.agentLinks(id)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		guide, _, _, _, err := s.skillEffect(u.ClassID, skills)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		items = append(items, agentJSON(id, name, prompt, en == 1, skills, materials, guide))
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	in, ok := s.readAgentBody(w, r, u)
	if !ok {
		return
	}
	res, err := s.db.Exec(`INSERT INTO class_agents (class_id, name, prompt, enabled) VALUES (?, ?, ?, ?)`,
		u.ClassID, in.Name, in.Prompt, boolInt(in.Enabled))
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, 400, map[string]string{"error": "name_taken"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	id, _ := res.LastInsertId()
	if err := s.replaceAgentLinks(int(id), in.SkillIDs, in.MaterialIDs); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	guide, _, _, _, _ := s.skillEffect(u.ClassID, in.SkillIDs)
	writeJSON(w, 200, agentJSON(int(id), in.Name, in.Prompt, in.Enabled, in.SkillIDs, in.MaterialIDs, guide))
}

func (s *Server) updateAgent(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM class_agents WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&n); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if n != 1 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	in, ok := s.readAgentBody(w, r, u)
	if !ok {
		return
	}
	_, err = s.db.Exec(`UPDATE class_agents SET name=?, prompt=?, enabled=? WHERE id=? AND class_id=?`,
		in.Name, in.Prompt, boolInt(in.Enabled), id, u.ClassID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, 400, map[string]string{"error": "name_taken"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if err := s.replaceAgentLinks(id, in.SkillIDs, in.MaterialIDs); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	guide, _, _, _, _ := s.skillEffect(u.ClassID, in.SkillIDs)
	writeJSON(w, 200, agentJSON(id, in.Name, in.Prompt, in.Enabled, in.SkillIDs, in.MaterialIDs, guide))
}

func (s *Server) listStudents(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	rows, err := s.db.Query(`SELECT username FROM users WHERE role='student' AND class_id=? ORDER BY username`, u.ClassID)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		items = append(items, map[string]string{"username": name})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

type agentInput struct {
	Name        string
	Prompt      string
	Enabled     bool
	SkillIDs    []int
	MaterialIDs []int
}

func (s *Server) readAgentBody(w http.ResponseWriter, r *http.Request, u sessionUser) (agentInput, bool) {
	var body struct {
		Name        string `json:"name"`
		Prompt      string `json:"prompt"`
		Enabled     *bool  `json:"enabled"`
		SkillIDs    []int  `json:"skill_ids"`
		MaterialIDs []int  `json:"material_ids"`
		ClassID     int    `json:"class_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return agentInput{}, false
	}
	_ = body.ClassID
	name := strings.TrimSpace(body.Name)
	prompt := strings.TrimSpace(body.Prompt)
	if name == "" || len([]rune(name)) > 64 || len([]rune(prompt)) > 2000 || body.Enabled == nil {
		writeJSON(w, 400, map[string]string{"error": "bad_request"})
		return agentInput{}, false
	}
	skills := body.SkillIDs
	if skills == nil {
		skills = []int{}
	}
	for _, id := range skills {
		var n int
		err := s.db.QueryRow(`SELECT COUNT(*) FROM skill_files WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&n)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return agentInput{}, false
		}
		if n != 1 {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return agentInput{}, false
		}
	}
	mats := body.MaterialIDs
	if mats == nil {
		mats = []int{}
	}
	for _, id := range mats {
		var n int
		err := s.db.QueryRow(`SELECT COUNT(*) FROM materials WHERE id=? AND class_id=?`, id, u.ClassID).Scan(&n)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "unavailable"})
			return agentInput{}, false
		}
		if n != 1 {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return agentInput{}, false
		}
	}
	return agentInput{Name: name, Prompt: prompt, Enabled: *body.Enabled, SkillIDs: skills, MaterialIDs: mats}, true
}

func (s *Server) replaceAgentLinks(id int, skills []int, materials []int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM class_agent_skill_files WHERE agent_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM class_agent_materials WHERE agent_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	seenSkill := map[int]bool{}
	for _, sid := range skills {
		if seenSkill[sid] {
			continue
		}
		seenSkill[sid] = true
		if _, err := tx.Exec(`INSERT INTO class_agent_skill_files (agent_id, skill_id) VALUES (?, ?)`, id, sid); err != nil {
			tx.Rollback()
			return err
		}
	}
	seenMat := map[int]bool{}
	for _, mid := range materials {
		if seenMat[mid] {
			continue
		}
		seenMat[mid] = true
		if _, err := tx.Exec(`INSERT INTO class_agent_materials (agent_id, material_id) VALUES (?, ?)`, id, mid); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Server) agentLinks(id int) ([]int, []int, error) {
	skills := []int{}
	rows, err := s.db.Query(`SELECT skill_id FROM class_agent_skill_files WHERE agent_id=? ORDER BY skill_id`, id)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var key int
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, nil, err
		}
		skills = append(skills, key)
	}
	rows.Close()
	materials := []int{}
	rows, err = s.db.Query(`SELECT material_id FROM class_agent_materials WHERE agent_id=? ORDER BY material_id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int
		if err := rows.Scan(&mid); err != nil {
			return nil, nil, err
		}
		materials = append(materials, mid)
	}
	return skills, materials, nil
}

func (s *Server) resolveAgent(u sessionUser, requested int) (classAgent, error) {
	var row classAgent
	var en int
	var err error
	if requested > 0 {
		err = s.db.QueryRow(`SELECT id, name, prompt, enabled FROM class_agents WHERE id=? AND class_id=?`, requested, u.ClassID).
			Scan(&row.ID, &row.Name, &row.Prompt, &en)
		if err == nil && u.Role != "teacher" && en != 1 {
			return classAgent{}, sql.ErrNoRows
		}
	} else {
		err = s.db.QueryRow(`SELECT id, name, prompt, enabled FROM class_agents WHERE class_id=? AND enabled=1 ORDER BY id LIMIT 1`, u.ClassID).
			Scan(&row.ID, &row.Name, &row.Prompt, &en)
	}
	if err != nil {
		return classAgent{}, err
	}
	row.Enabled = en == 1
	row.SkillIDs, row.MaterialIDs, err = s.agentLinks(row.ID)
	if err != nil {
		return classAgent{}, err
	}
	row.Guide, row.SkillNote, row.SkillBodies, row.Tools, err = s.skillEffect(u.ClassID, row.SkillIDs)
	return row, err
}

func agentJSON(id int, name, prompt string, enabled bool, skills []int, materials []int, guide bool) map[string]any {
	if skills == nil {
		skills = []int{}
	}
	if materials == nil {
		materials = []int{}
	}
	return map[string]any{
		"id": id, "name": name, "prompt": prompt, "enabled": enabled,
		"skill_ids": skills, "material_ids": materials, "guide": guide,
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

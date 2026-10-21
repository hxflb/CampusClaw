package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"campusclaw/internal/config"
	"campusclaw/internal/indexx"
	"campusclaw/internal/token"
)

type Server struct {
	cfg   *config.Config
	db    *sql.DB
	idx   *indexx.Service
	dummy []byte
	fail  sync.Map
}

type failState struct {
	n     int
	until time.Time
}

type sessionUser struct {
	UserID  int
	Role    string
	ClassID int
	Name    string
}

func New(cfg *config.Config, db *sql.DB, idx *indexx.Service) *Server {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("timing-pad"), bcrypt.DefaultCost)
	return &Server{cfg: cfg, db: db, idx: idx, dummy: dummy}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.require(s.me))
	mux.HandleFunc("GET /api/classes", s.require(s.classes))
	mux.HandleFunc("GET /api/materials", s.require(s.listMaterials))
	mux.HandleFunc("GET /api/materials/{id}", s.require(s.getMaterial))
	mux.HandleFunc("GET /api/materials/{id}/file", s.require(s.fileMaterial))
	mux.HandleFunc("POST /api/materials", s.require(s.upload))
	mux.HandleFunc("PUT /api/materials/{id}", s.require(s.update))
	mux.HandleFunc("DELETE /api/materials/{id}", s.require(s.remove))
	mux.HandleFunc("POST /api/materials/{id}/reindex", s.require(s.reindex))
	mux.HandleFunc("POST /api/search", s.require(s.search))
	mux.HandleFunc("POST /api/ask", s.require(s.ask))
	mux.HandleFunc("GET /api/prompt", s.require(s.getPrompt))
	mux.HandleFunc("PUT /api/prompt", s.require(s.putPrompt))
	mux.HandleFunc("GET /api/skills", s.require(s.getSkills))
	mux.HandleFunc("PUT /api/skills/{key}", s.require(s.putSkill))
	mux.HandleFunc("GET /api/skill-files", s.require(s.listSkillFiles))
	mux.HandleFunc("POST /api/skill-files", s.require(s.createSkillFile))
	mux.HandleFunc("PUT /api/skill-files/{id}", s.require(s.updateSkillFile))
	mux.HandleFunc("PUT /api/skill-files/{id}/enabled", s.require(s.setSkillFileEnabled))
	mux.HandleFunc("DELETE /api/skill-files/{id}", s.require(s.deleteSkillFile))
	mux.HandleFunc("GET /api/agents", s.require(s.listAgents))
	mux.HandleFunc("POST /api/agents", s.require(s.createAgent))
	mux.HandleFunc("PUT /api/agents/{id}", s.require(s.updateAgent))
	mux.HandleFunc("GET /api/students", s.require(s.listStudents))
	mux.HandleFunc("GET /api/conversations", s.require(s.listConversations))
	mux.HandleFunc("POST /api/conversations", s.require(s.createConversation))
	mux.HandleFunc("GET /api/conversations/{id}", s.require(s.getConversation))
	mux.HandleFunc("DELETE /api/conversations/{id}", s.require(s.deleteConversation))
	mux.HandleFunc("PUT /api/conversations/{id}/agent", s.require(s.setConversationAgent))
	mux.HandleFunc("POST /api/conversations/{id}/messages", s.require(s.postMessage))
	mux.HandleFunc("GET /api/mcp/servers", s.require(s.listMcpServers))
	mux.HandleFunc("POST /api/mcp/servers", s.require(s.createMcpServer))
	mux.HandleFunc("POST /api/mcp/servers/{id}/connect", s.require(s.connectMcpServer))
	mux.HandleFunc("POST /api/mcp/servers/{id}/disconnect", s.require(s.disconnectMcpServer))
	mux.HandleFunc("DELETE /api/mcp/servers/{id}", s.require(s.deleteMcpServer))
	mux.HandleFunc("GET /api/mcp/audit", s.require(s.listMcpAudit))
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	key := strings.ToLower(strings.TrimSpace(body.Username)) + "|" + clientIP(r)
	if s.locked(key) {
		s.failLogin(w)
		return
	}
	var id int
	var username, hash, role string
	var classID int
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, class_id FROM users WHERE username=?`, strings.TrimSpace(body.Username)).
		Scan(&id, &username, &hash, &role, &classID)
	cmp := hash
	if err != nil {
		cmp = string(s.dummy)
	}
	if bcrypt.CompareHashAndPassword([]byte(cmp), []byte(body.Password)) != nil || err != nil {
		s.bumpFail(key)
		s.failLogin(w)
		return
	}
	s.fail.Delete(key)
	if old := bearerToken(r); old != "" {
		if c, err := token.Parse(s.cfg.SessionSecret, old, time.Now()); err == nil {
			_, _ = s.db.Exec(`DELETE FROM sessions WHERE id=? AND user_id=?`, c.JTI, c.UserID)
		}
	}
	now := time.Now()
	exp := now.Add(time.Duration(s.cfg.SessionTTLHours) * time.Hour)
	jti := newID()
	signed, err := token.Sign(s.cfg.SessionSecret, token.Claims{
		UserID: id, Username: username, Role: role, ClassID: classID,
		JTI: jti, Iat: now.Unix(), Exp: exp.Unix(),
	})
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if _, err := s.db.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`, jti, id, exp); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	clearSID(w)
	writeJSON(w, 200, map[string]any{"ok": true, "role": role, "class_id": classID, "token": signed})
}

func (s *Server) failLogin(w http.ResponseWriter) {
	writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if raw := bearerToken(r); raw != "" {
		if c, err := token.Parse(s.cfg.SessionSecret, raw, time.Now()); err == nil {
			_, _ = s.db.Exec(`DELETE FROM sessions WHERE id=? AND user_id=?`, c.JTI, c.UserID)
		}
	}
	clearSID(w)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	writeJSON(w, 200, map[string]any{"user_id": u.UserID, "username": u.Name, "role": u.Role, "class_id": u.ClassID})
}

func (s *Server) classes(w http.ResponseWriter, _ *http.Request, u sessionUser) {
	q := `SELECT id, name FROM classes ORDER BY id`
	if u.Role != "teacher" {
		q = `SELECT id, name FROM classes WHERE id=? ORDER BY id`
	}
	var rows *sql.Rows
	var err error
	if u.Role != "teacher" {
		rows, err = s.db.Query(q, u.ClassID)
	} else {
		rows, err = s.db.Query(q)
	}
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var name string
		_ = rows.Scan(&id, &name)
		out = append(out, map[string]any{"id": id, "name": name})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) listMaterials(w http.ResponseWriter, r *http.Request, u sessionUser) {
	cid := u.ClassID
	if u.Role == "teacher" {
		if raw := r.URL.Query().Get("class_id"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && s.classExists(n) {
				cid = n
			}
		}
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sqlStr := `SELECT m.id, m.title, m.class_id, m.created_at, u.username
		FROM materials m JOIN users u ON u.id=m.uploaded_by
		WHERE m.class_id=?`
	args := []any{cid}
	if q != "" {
		sqlStr += ` AND m.title LIKE ?`
		args = append(args, "%"+q+"%")
	}
	sqlStr += ` ORDER BY m.id DESC`
	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, classID int
		var title, uploader string
		var created time.Time
		_ = rows.Scan(&id, &title, &classID, &created, &uploader)
		items = append(items, map[string]any{
			"id": id, "title": title, "class_id": classID,
			"uploader": uploader, "created_at": created.Format("2006-01-02 15:04:05"),
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) getMaterial(w http.ResponseWriter, r *http.Request, u sessionUser) {
	row, ok := s.fetchMaterial(r)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if u.Role != "teacher" && row.classID != u.ClassID {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": row.id, "title": row.title, "class_id": row.classID,
		"body_text": row.body, "created_at": row.created,
	})
}

func (s *Server) fileMaterial(w http.ResponseWriter, r *http.Request, u sessionUser) {
	row, ok := s.fetchMaterial(r)
	if !ok || (u.Role != "teacher" && row.classID != u.ClassID) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	http.ServeFile(w, r, row.path)
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+1024)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		writeJSON(w, 413, map[string]string{"error": "too_large"})
		return
	}
	cid := u.ClassID
	if raw := r.FormValue("class_id"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && s.classExists(n) {
			cid = n
		}
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "missing_file"})
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext != ".txt" && ext != ".md" {
		writeJSON(w, 400, map[string]string{"error": "unsupported_type"})
		return
	}
	data, err := io.ReadAll(f)
	if err != nil || len(bytesTrim(data)) == 0 || !utf8.Valid(data) {
		writeJSON(w, 400, map[string]string{"error": "invalid_file"})
		return
	}
	body := strings.TrimSpace(string(data))
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(hdr.Filename), ext)
	}
	name := newID() + ext
	dest := filepath.Join(s.cfg.UploadDir, name)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		writeJSON(w, 500, map[string]string{"error": "store_failed"})
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		_ = os.Remove(dest)
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	res, err := tx.Exec(`INSERT INTO materials (title, class_id, file_path, uploaded_by) VALUES (?, ?, ?, ?)`,
		title, cid, dest, u.UserID)
	if err != nil {
		tx.Rollback()
		_ = os.Remove(dest)
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	mid, _ := res.LastInsertId()
	if _, err := tx.Exec(`INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)`,
		mid, cid, body); err != nil {
		tx.Rollback()
		_ = os.Remove(dest)
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if _, err := tx.Exec(`INSERT IGNORE INTO class_agent_materials (agent_id, material_id)
		SELECT id, ? FROM class_agents WHERE class_id=?`, mid, cid); err != nil {
		tx.Rollback()
		_ = os.Remove(dest)
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(dest)
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, 201, map[string]any{"material_id": mid, "title": title, "class_id": cid})
	opts := indexx.ParseOpts(
		r.FormValue("strategy"), r.FormValue("delimiter"),
		atoiForm(r.FormValue("max_chars")), atoiForm(r.FormValue("overlap_pct")),
		r.FormValue("strip_urls") == "1", r.FormValue("collapse_ws") == "1",
	)
	_ = s.idx.IndexMaterial(context.Background(), int(mid), opts)
}

func (s *Server) update(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	row, ok := s.fetchMaterial(r)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+1024)
	_ = r.ParseMultipartForm(s.cfg.MaxUploadBytes)
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	cid := row.classID
	if raw := r.FormValue("class_id"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && s.classExists(n) {
			cid = n
		}
	}
	path := row.path
	if f, hdr, err := r.FormFile("file"); err == nil && strings.TrimSpace(hdr.Filename) != "" {
		defer f.Close()
		ext := strings.ToLower(filepath.Ext(hdr.Filename))
		if ext != ".txt" && ext != ".md" {
			writeJSON(w, 400, map[string]string{"error": "unsupported_type"})
			return
		}
		data, err := io.ReadAll(f)
		if err != nil || !utf8.Valid(data) || len(bytesTrim(data)) == 0 {
			writeJSON(w, 400, map[string]string{"error": "invalid_file"})
			return
		}
		body = strings.TrimSpace(string(data))
		dest := filepath.Join(s.cfg.UploadDir, newID()+ext)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			writeJSON(w, 500, map[string]string{"error": "store_failed"})
			return
		}
		path = dest
	}
	if title == "" || body == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid"})
		return
	}
	tx, _ := s.db.Begin()
	if _, err := tx.Exec(`UPDATE materials SET title=?, class_id=?, file_path=? WHERE id=?`, title, cid, path, row.id); err != nil {
		tx.Rollback()
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if _, err := tx.Exec(`UPDATE knowledge_entries SET class_id=?, body_text=? WHERE material_id=?`, cid, body, row.id); err != nil {
		tx.Rollback()
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if path != row.path {
		_ = os.Remove(row.path)
	}
	writeJSON(w, 200, map[string]any{"material_id": row.id, "title": title, "class_id": cid})
	opts := indexx.ParseOpts(
		r.FormValue("strategy"), r.FormValue("delimiter"),
		atoiForm(r.FormValue("max_chars")), atoiForm(r.FormValue("overlap_pct")),
		r.FormValue("strip_urls") == "1", r.FormValue("collapse_ws") == "1",
	)
	_ = s.idx.IndexMaterial(context.Background(), row.id, opts)
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	row, ok := s.fetchMaterial(r)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	_ = s.idx.Qdrant().DeleteByMaterial(row.id)
	tx, _ := s.db.Begin()
	_, _ = tx.Exec(`DELETE FROM knowledge_chunks WHERE material_id=?`, row.id)
	_, _ = tx.Exec(`DELETE FROM knowledge_entries WHERE material_id=?`, row.id)
	_, _ = tx.Exec(`DELETE FROM materials WHERE id=?`, row.id)
	if err := tx.Commit(); err != nil {
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	_ = os.Remove(row.path)
	writeJSON(w, 200, map[string]any{"deleted": row.id})
}

type matRow struct {
	id, classID                int
	title, path, body, created string
}

func (s *Server) fetchMaterial(r *http.Request) (matRow, bool) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var row matRow
	var created time.Time
	err := s.db.QueryRow(`SELECT m.id, m.title, m.class_id, m.file_path, m.created_at, COALESCE(k.body_text,'')
		FROM materials m LEFT JOIN knowledge_entries k ON k.material_id=m.id WHERE m.id=?`, id).
		Scan(&row.id, &row.title, &row.classID, &row.path, &created, &row.body)
	if err != nil {
		return row, false
	}
	row.created = created.Format("2006-01-02 15:04:05")
	return row, true
}

func (s *Server) classExists(id int) bool {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM classes WHERE id=?`, id).Scan(&n)
	return n == 1
}

func (s *Server) require(next func(http.ResponseWriter, *http.Request, sessionUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		claims, err := token.Parse(s.cfg.SessionSecret, raw, time.Now())
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		var u sessionUser
		err = s.db.QueryRow(`SELECT u.id, u.username, u.role, u.class_id
			FROM sessions s JOIN users u ON u.id=s.user_id
			WHERE s.id=? AND s.user_id=? AND s.expires_at > NOW()`, claims.JTI, claims.UserID).
			Scan(&u.UserID, &u.Name, &u.Role, &u.ClassID)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r, u)
	}
}

func (s *Server) locked(key string) bool {
	v, ok := s.fail.Load(key)
	if !ok {
		return false
	}
	st := v.(failState)
	return time.Now().Before(st.until)
}

func (s *Server) bumpFail(key string) {
	v, _ := s.fail.LoadOrStore(key, failState{})
	st := v.(failState)
	st.n++
	if st.n >= s.cfg.FailLimit {
		st.until = time.Now().Add(time.Duration(s.cfg.LockSeconds) * time.Second)
		st.n = 0
	}
	s.fail.Store(key, st)
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

func clearSID(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "sid", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

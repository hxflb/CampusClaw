package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"campusclaw/internal/chat"
	"campusclaw/internal/indexx"
	"campusclaw/internal/retrieve"
)

func atoiForm(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, u sessionUser) {
	var body struct {
		Query   string `json:"query"`
		Mode    string `json:"mode"`
		ClassID int    `json:"class_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	q := strings.TrimSpace(body.Query)
	if q == "" {
		writeJSON(w, 400, map[string]string{"error": "empty_query"})
		return
	}
	mode := body.Mode
	if mode == "" {
		mode = "hybrid"
	}
	cid := u.ClassID
	var hits []retrieve.Hit
	var qv []float32
	var err error
	switch mode {
	case "keyword":
		hits, err = retrieve.Keyword(s.db, cid, q, 20)
	case "vector":
		hits, qv, err = retrieve.Vector(s.db, s.idx.Qdrant(), s.idx.Embedder(), cid, q, 20)
	default:
		mode = "hybrid"
		hits, qv, err = retrieve.Hybrid(s.db, s.idx.Qdrant(), s.idx.Embedder(), cid, q)
	}
	if err != nil {
		if mode != "keyword" {
			writeJSON(w, 503, map[string]string{"error": "vector_unavailable"})
			return
		}
		writeJSON(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if hits == nil {
		hits = []retrieve.Hit{}
	}
	out := map[string]any{"mode": mode, "hits": hits}
	if mode != "keyword" && qv != nil {
		out["query_vector"] = qv
	}
	writeJSON(w, 200, out)
}

func (s *Server) ask(w http.ResponseWriter, r *http.Request, u sessionUser) {
	var body struct {
		Query    string           `json:"query"`
		Messages []map[string]any `json:"messages"`
		System   string           `json:"system"`
		ClassID  int              `json:"class_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.ClassID
	_ = body.System
	q := strings.TrimSpace(body.Query)
	if q == "" && len(body.Messages) > 0 {
		for i := len(body.Messages) - 1; i >= 0; i-- {
			role, _ := body.Messages[i]["role"].(string)
			if role == "system" {
				continue
			}
			if c, ok := body.Messages[i]["content"].(string); ok {
				q = strings.TrimSpace(c)
				break
			}
		}
	}
	if q == "" {
		writeJSON(w, 400, map[string]string{"error": "empty_query"})
		return
	}
	hits, _, err := retrieve.Hybrid(s.db, s.idx.Qdrant(), s.idx.Embedder(), u.ClassID, q)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "vector_unavailable"})
		return
	}
	if len(hits) == 0 {
		writeJSON(w, 200, map[string]any{
			"answer":    "资料中未找到相关内容",
			"citations": []any{},
		})
		return
	}
	if len(hits) > 4 {
		hits = hits[:4]
	}
	chits := make([]chat.Hit, 0, len(hits))
	for _, h := range hits {
		var text string
		_ = s.db.QueryRow(`SELECT chunk_text FROM knowledge_chunks WHERE id=? AND class_id=?`, h.ChunkID, u.ClassID).Scan(&text)
		if text == "" {
			text = h.Excerpt
		}
		chits = append(chits, chat.Hit{
			MaterialID: h.MaterialID, Title: h.Title, ChunkID: h.ChunkID,
			ChunkIndex: h.ChunkIndex, Text: text,
		})
	}
	var prior []chat.Turn
	for _, m := range body.Messages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "system" || strings.TrimSpace(content) == "" {
			continue
		}
		if role != "user" && role != "assistant" {
			continue
		}
		prior = append(prior, chat.Turn{Role: role, Content: strings.TrimSpace(content)})
	}
	if n := len(prior); n > 0 && prior[n-1].Role == "user" && prior[n-1].Content == q {
		prior = prior[:n-1]
	}
	ans, cites := chat.Answer(s.cfg, q, chits, prior)
	writeJSON(w, 200, map[string]any{"answer": ans, "citations": cites})
}

func (s *Server) reindex(w http.ResponseWriter, r *http.Request, u sessionUser) {
	if u.Role != "teacher" {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	row, ok := s.fetchMaterial(r)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		Strategy    string `json:"strategy"`
		Delimiter   string `json:"delimiter"`
		MaxChars    int    `json:"max_chars"`
		OverlapPct  int    `json:"overlap_pct"`
		StripURLs   bool   `json:"strip_urls"`
		CollapseWS  bool   `json:"collapse_ws"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	opts := indexx.ParseOpts(body.Strategy, body.Delimiter, body.MaxChars, body.OverlapPct, body.StripURLs, body.CollapseWS)
	if err := s.idx.IndexMaterial(r.Context(), row.id, opts); err != nil {
		writeJSON(w, 503, map[string]string{"error": "index_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "material_id": row.id, "strategy": opts.Strategy})
}

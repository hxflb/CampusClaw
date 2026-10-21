package retrieve

import (
	"database/sql"
	"sort"
	"strings"

	"campusclaw/internal/embedder"
	"campusclaw/internal/qdrant"
)

type Hit struct {
	ChunkID      int64    `json:"chunk_id"`
	MaterialID   int      `json:"material_id"`
	Title        string   `json:"material_title"`
	ChunkIndex   int      `json:"chunk_index"`
	CharStart    int      `json:"char_start"`
	CharEnd      int      `json:"char_end"`
	Excerpt      string   `json:"excerpt"`
	KeywordScore *float64 `json:"keyword_score,omitempty"`
	KeywordRank  *int     `json:"keyword_rank,omitempty"`
	VectorScore  *float64 `json:"vector_score,omitempty"`
	VectorRank   *int     `json:"vector_rank,omitempty"`
	RRFScore     *float64 `json:"rrf_score,omitempty"`
}

func Keyword(db *sql.DB, classID int, query string, limit int) ([]Hit, error) {
	q := strings.TrimSpace(query)
	rows, err := db.Query(`SELECT c.id, c.material_id, c.chunk_index, c.chunk_text, c.char_start, c.char_end, m.title,
		MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE) AS score
		FROM knowledge_chunks c JOIN materials m ON m.id=c.material_id
		WHERE c.class_id=? AND c.index_status='ready'
		  AND MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE)
		ORDER BY score DESC LIMIT ?`, q, classID, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		var score float64
		var text string
		if err := rows.Scan(&h.ChunkID, &h.MaterialID, &h.ChunkIndex, &text, &h.CharStart, &h.CharEnd, &h.Title, &score); err != nil {
			return nil, err
		}
		h.Excerpt = clip(text, 220)
		sc, rk := score, len(hits)+1
		h.KeywordScore, h.KeywordRank = &sc, &rk
		hits = append(hits, h)
	}
	return hits, nil
}

func Vector(db *sql.DB, qd *qdrant.Client, emb *embedder.Client, classID int, query string, limit int) ([]Hit, []float32, error) {
	vec, err := emb.Embed(query)
	if err != nil {
		return nil, nil, err
	}
	raw, err := qd.Search(vec, classID, limit, 0.35)
	if err != nil {
		return nil, vec, err
	}
	if len(raw) == 0 {
		return nil, vec, nil
	}
	ids := make([]int64, len(raw))
	scoreBy := map[int64]float64{}
	rankBy := map[int64]int{}
	for i, r := range raw {
		ids[i] = r.ID
		scoreBy[r.ID] = r.Score
		rankBy[r.ID] = i + 1
	}
	hits, err := loadHits(db, classID, ids)
	if err != nil {
		return nil, vec, err
	}
	for i := range hits {
		sc := scoreBy[hits[i].ChunkID]
		rk := rankBy[hits[i].ChunkID]
		hits[i].VectorScore, hits[i].VectorRank = &sc, &rk
	}
	sort.Slice(hits, func(i, j int) bool {
		return *hits[i].VectorScore > *hits[j].VectorScore
	})
	return hits, vec, nil
}

func keepStrongKeywords(hits []Hit) []Hit {
	if len(hits) == 0 || hits[0].KeywordScore == nil || *hits[0].KeywordScore <= 0 {
		return hits
	}
	floor := *hits[0].KeywordScore * 0.4
	n := 0
	for n < len(hits) && hits[n].KeywordScore != nil && *hits[n].KeywordScore >= floor {
		n++
	}
	return hits[:n]
}

func Hybrid(db *sql.DB, qd *qdrant.Client, emb *embedder.Client, classID int, query string) ([]Hit, []float32, error) {
	kw, err := Keyword(db, classID, query, 20)
	if err != nil {
		return nil, nil, err
	}
	kw = keepStrongKeywords(kw)
	vecHits, qv, err := Vector(db, qd, emb, classID, query, 20)
	if err != nil {
		return nil, qv, err
	}
	const k = 60.0
	type acc struct {
		h Hit
		s float64
	}
	m := map[int64]*acc{}
	add := func(list []Hit, which string) {
		for _, h := range list {
			a := m[h.ChunkID]
			if a == nil {
				cp := h
				a = &acc{h: cp}
				m[h.ChunkID] = a
			}
			if which == "kw" && h.KeywordRank != nil {
				a.s += 1.0 / (k + float64(*h.KeywordRank))
				a.h.KeywordScore, a.h.KeywordRank = h.KeywordScore, h.KeywordRank
			}
			if which == "vec" && h.VectorRank != nil {
				a.s += 1.0 / (k + float64(*h.VectorRank))
				a.h.VectorScore, a.h.VectorRank = h.VectorScore, h.VectorRank
			}
			if a.h.Title == "" {
				a.h = h
			}
		}
	}
	add(kw, "kw")
	add(vecHits, "vec")
	out := make([]Hit, 0, len(m))
	for _, a := range m {
		sc := a.s
		a.h.RRFScore = &sc
		out = append(out, a.h)
	}
	sort.Slice(out, func(i, j int) bool { return *out[i].RRFScore > *out[j].RRFScore })
	if len(out) > 20 {
		out = out[:20]
	}
	return out, qv, nil
}

func loadHits(db *sql.DB, classID int, ids []int64) ([]Hit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, classID)
	rows, err := db.Query(`SELECT c.id, c.material_id, c.chunk_index, c.chunk_text, c.char_start, c.char_end, m.title
		FROM knowledge_chunks c JOIN materials m ON m.id=c.material_id
		WHERE c.id IN (`+ph+`) AND c.class_id=? AND c.index_status='ready'`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		var text string
		if err := rows.Scan(&h.ChunkID, &h.MaterialID, &h.ChunkIndex, &text, &h.CharStart, &h.CharEnd, &h.Title); err != nil {
			return nil, err
		}
		h.Excerpt = clip(text, 220)
		hits = append(hits, h)
	}
	return hits, nil
}

func clip(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

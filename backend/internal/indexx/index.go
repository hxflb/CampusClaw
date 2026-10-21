package indexx

import (
	"context"
	"database/sql"
	"fmt"

	"campusclaw/internal/chunker"
	"campusclaw/internal/config"
	"campusclaw/internal/embedder"
	"campusclaw/internal/qdrant"
)

type Service struct {
	db   *sql.DB
	cfg  *config.Config
	emb  *embedder.Client
	qd   *qdrant.Client
}

func New(db *sql.DB, cfg *config.Config, emb *embedder.Client, qd *qdrant.Client) *Service {
	return &Service{db: db, cfg: cfg, emb: emb, qd: qd}
}

func (s *Service) Qdrant() *qdrant.Client { return s.qd }
func (s *Service) Embedder() *embedder.Client { return s.emb }

func ParseOpts(strategy, delim string, max, overlapPct int, strip, collapse bool) chunker.Opts {
	if strategy == "" {
		strategy = "auto"
	}
	ov := 0
	if max > 0 {
		ov = max * overlapPct / 100
	}
	return chunker.Opts{
		Strategy:     strategy,
		MaxRunes:     max,
		OverlapRunes: ov,
		Delim:        delim,
		StripURLs:    strip,
		CollapseWS:   collapse,
	}
}

func (s *Service) IndexMaterial(ctx context.Context, materialID int, opts chunker.Opts) error {
	_ = s.qd.DeleteByMaterial(materialID)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE material_id=?`, materialID)
	var entryID, classID int
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT id, class_id, body_text FROM knowledge_entries WHERE material_id=? ORDER BY id DESC LIMIT 1`, materialID).
		Scan(&entryID, &classID, &body)
	if err != nil {
		return err
	}
	chunks := chunker.Split(body, opts)
	if len(chunks) == 0 {
		return nil
	}
	for _, ch := range chunks {
		res, err := s.db.ExecContext(ctx, `INSERT INTO knowledge_chunks
			(knowledge_entry_id, material_id, class_id, chunk_index, chunk_text, char_start, char_end, index_status, strategy)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			entryID, materialID, classID, ch.Index, ch.Text, ch.Start, ch.End, "pending", opts.Strategy)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		vec, err := s.emb.Embed(ch.Text)
		if err != nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE id=?`, id)
			continue
		}
		payload := map[string]any{
			"class_id": classID, "material_id": materialID,
			"knowledge_entry_id": entryID, "chunk_id": id, "chunk_index": ch.Index,
		}
		if err := s.qd.Upsert(id, vec, payload); err != nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE id=?`, id)
			continue
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='ready' WHERE id=?`, id)
	}
	return nil
}

func (s *Service) Reembed(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, class_id, material_id, knowledge_entry_id, chunk_index, chunk_text FROM knowledge_chunks ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id, classID, materialID, entryID, idx int
		text                                  string
	}
	var items []row
	for rows.Next() {
		var it row
		if err := rows.Scan(&it.id, &it.classID, &it.materialID, &it.entryID, &it.idx, &it.text); err != nil {
			return err
		}
		items = append(items, it)
	}
	for _, it := range items {
		vec, err := s.emb.Embed(it.text)
		if err != nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE id=?`, it.id)
			fmt.Println("reembed", it.id, err)
			continue
		}
		payload := map[string]any{
			"class_id": it.classID, "material_id": it.materialID,
			"knowledge_entry_id": it.entryID, "chunk_id": it.id, "chunk_index": it.idx,
		}
		if err := s.qd.Upsert(int64(it.id), vec, payload); err != nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE id=?`, it.id)
			fmt.Println("reembed", it.id, err)
			continue
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='ready' WHERE id=?`, it.id)
	}
	return nil
}

func (s *Service) Backfill(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT m.id FROM materials m
		LEFT JOIN knowledge_chunks c ON c.material_id=m.id
		WHERE c.id IS NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := s.IndexMaterial(ctx, id, chunker.Opts{Strategy: "auto"}); err != nil {
			fmt.Println("backfill", id, err)
		}
	}
	return nil
}

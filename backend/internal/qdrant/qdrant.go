package qdrant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base   string
	http   *http.Client
	coll   string
	dim    int
}

func New(base string, dim int) *Client {
	return &Client{base: trim(base), http: &http.Client{Timeout: 15 * time.Second}, coll: "campusclaw_chunks", dim: dim}
}

func (c *Client) Ready() error {
	res, err := c.http.Get(c.base + "/readyz")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("qdrant %d", res.StatusCode)
	}
	return nil
}

func (c *Client) Ensure() (bool, error) {
	res, err := c.http.Get(c.base + "/collections/" + c.coll)
	if err != nil {
		return false, err
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return false, c.create()
	}
	if res.StatusCode >= 300 {
		return false, fmt.Errorf("collection %d", res.StatusCode)
	}
	size := parseVectorSize(raw)
	if size <= 0 {
		return false, fmt.Errorf("collection vector size unknown")
	}
	if size == c.dim {
		return false, nil
	}
	del, err := http.NewRequest(http.MethodDelete, c.base+"/collections/"+c.coll, nil)
	if err != nil {
		return false, err
	}
	dres, err := c.http.Do(del)
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, dres.Body)
	dres.Body.Close()
	if dres.StatusCode >= 300 {
		return false, fmt.Errorf("delete collection %d", dres.StatusCode)
	}
	if err := c.create(); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) create() error {
	body, _ := json.Marshal(map[string]any{
		"vectors": map[string]any{"size": c.dim, "distance": "Cosine"},
	})
	req, _ := http.NewRequest(http.MethodPut, c.base+"/collections/"+c.coll, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 300 && res.StatusCode != http.StatusConflict {
		return fmt.Errorf("ensure collection %d", res.StatusCode)
	}
	return nil
}

func parseVectorSize(raw []byte) int {
	var out struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors json.RawMessage `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return 0
	}
	var simple struct {
		Size int `json:"size"`
	}
	if json.Unmarshal(out.Result.Config.Params.Vectors, &simple) == nil && simple.Size > 0 {
		return simple.Size
	}
	var named map[string]struct {
		Size int `json:"size"`
	}
	if json.Unmarshal(out.Result.Config.Params.Vectors, &named) != nil {
		return 0
	}
	for _, v := range named {
		if v.Size > 0 {
			return v.Size
		}
	}
	return 0
}

func (c *Client) Upsert(id int64, vec []float32, payload map[string]any) error {
	body, _ := json.Marshal(map[string]any{
		"points": []map[string]any{{"id": id, "vector": vec, "payload": payload}},
	})
	req, _ := http.NewRequest(http.MethodPut, c.base+"/collections/"+c.coll+"/points?wait=true", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("qdrant upsert %d", res.StatusCode)
	}
	return nil
}

func (c *Client) DeleteByMaterial(materialID int) error {
	body, _ := json.Marshal(map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{{"key": "material_id", "match": map[string]any{"value": materialID}}},
		},
	})
	return c.postJSON("/collections/"+c.coll+"/points/delete?wait=true", body)
}

type Hit struct {
	ID    int64
	Score float64
}

func (c *Client) Search(vec []float32, classID int, limit int, minScore float64) ([]Hit, error) {
	body, _ := json.Marshal(map[string]any{
		"vector":          vec,
		"limit":           limit,
		"score_threshold": minScore,
		"with_payload":    false,
		"filter": map[string]any{
			"must": []map[string]any{{"key": "class_id", "match": map[string]any{"value": classID}}},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, c.base+"/collections/"+c.coll+"/points/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("search %d: %s", res.StatusCode, raw)
	}
	var out struct {
		Result []struct {
			ID    json.Number `json:"id"`
			Score float64     `json:"score"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(out.Result))
	for _, r := range out.Result {
		id, _ := r.ID.Int64()
		hits = append(hits, Hit{ID: id, Score: r.Score})
	}
	return hits, nil
}

func (c *Client) postJSON(path string, body []byte) error {
	req, _ := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("qdrant post %d", res.StatusCode)
	}
	return nil
}

func trim(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

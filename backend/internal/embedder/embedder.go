package embedder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"campusclaw/internal/config"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
	hasAPI bool
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: 30 * time.Second},
		hasAPI: strings.TrimSpace(cfg.EmbedBaseURL) != "" && strings.TrimSpace(cfg.EmbedAPIKey) != "",
	}
}

func (c *Client) UsesGateway() bool { return c.hasAPI }

func (c *Client) Embed(text string) ([]float32, error) {
	if c.hasAPI {
		return c.remote(text)
	}
	return local(text, c.cfg.VectorSize), nil
}

func (c *Client) remote(text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": c.cfg.EmbedModel, "input": text})
	req, err := http.NewRequest(http.MethodPost, config.OpenAIURL(c.cfg.EmbedBaseURL, "/embeddings"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.EmbedAPIKey)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("embed %d: %s", res.StatusCode, raw)
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) == 0 {
		return nil, fmt.Errorf("embed parse")
	}
	vec := l2(out.Data[0].Embedding)
	if c.cfg.VectorSize > 0 && len(vec) != c.cfg.VectorSize {
		return nil, fmt.Errorf("embed dim %d", len(vec))
	}
	return vec, nil
}

func local(text string, dim int) []float32 {
	if dim <= 0 {
		dim = 384
	}
	v := make([]float32, dim)
	rs := []rune(strings.ToLower(text))
	for i := 0; i+1 < len(rs); i++ {
		h := fnv.New32a()
		_, _ = h.Write([]byte(string(rs[i : i+2])))
		v[int(h.Sum32())%dim] += 1
	}
	if len(rs) == 1 {
		h := fnv.New32a()
		_, _ = h.Write([]byte(string(rs)))
		v[int(h.Sum32())%dim] += 1
	}
	return l2(v)
}

func l2(v []float32) []float32 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	n := math.Sqrt(s)
	if n == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / n)
	}
	return out
}

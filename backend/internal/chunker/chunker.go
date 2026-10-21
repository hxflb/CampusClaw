package chunker

import (
	"regexp"
	"strings"
)

type Chunk struct {
	Index int
	Text  string
	Start int
	End   int
}

type Opts struct {
	Strategy     string
	MaxRunes     int
	OverlapRunes int
	Delim        string
	StripURLs    bool
	CollapseWS   bool
}

var urlRe = regexp.MustCompile(`https?://\S+|[\w.+-]+@[\w-]+\.[\w.-]+`)

func Prepare(body string, o Opts) string {
	s := body
	if o.Strategy == "custom" && o.StripURLs {
		s = urlRe.ReplaceAllString(s, " ")
	}
	if o.Strategy == "custom" && o.CollapseWS {
		s = strings.Join(strings.Fields(s), " ")
	}
	return s
}

func Split(body string, o Opts) []Chunk {
	if o.Strategy == "" {
		o.Strategy = "auto"
	}
	src := Prepare(body, o)
	switch o.Strategy {
	case "hierarchy":
		max, ov := windowSize(o.MaxRunes, o.OverlapRunes, true)
		return splitHierarchy(src, max, ov)
	case "custom":
		max, ov := windowSize(o.MaxRunes, o.OverlapRunes, false)
		return window(src, max, ov, o.Delim)
	default:
		return window(src, 800, 80, "")
	}
}

func windowSize(max, ov int, hierarchy bool) (int, int) {
	if hierarchy && max <= 0 {
		max = 800
		ov = 80
	}
	if max < 100 {
		max = 100
	}
	if max > 2000 {
		max = 2000
	}
	if ov < 0 {
		ov = 0
	}
	if ov > max/2 {
		ov = max / 2
	}
	return max, ov
}

func splitHierarchy(src string, max, ov int) []Chunk {
	lines := strings.Split(src, "\n")
	var parts []string
	var cur strings.Builder
	flush := func() {
		t := strings.TrimSpace(cur.String())
		if t != "" {
			parts = append(parts, t)
		}
		cur.Reset()
	}
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "# ") || strings.HasPrefix(trim, "## ") || strings.HasPrefix(trim, "### ") {
			flush()
		}
		cur.WriteString(ln)
		cur.WriteByte('\n')
	}
	flush()
	if len(parts) == 0 {
		return window(src, max, ov, "")
	}
	var out []Chunk
	off := 0
	runes := []rune(src)
	for _, p := range parts {
		pr := []rune(p)
		idx := indexRunes(runes, pr, off)
		if idx < 0 {
			idx = off
		}
		if len(pr) > max {
			sub := window(p, max, ov, "")
			for _, s := range sub {
				s.Start += idx
				s.End += idx
				s.Index = len(out)
				out = append(out, s)
			}
		} else {
			out = append(out, Chunk{Index: len(out), Text: p, Start: idx, End: idx + len(pr)})
		}
		off = idx + len(pr)
	}
	return out
}

func indexRunes(hay, needle []rune, from int) int {
	if from < 0 {
		from = 0
	}
	n := len(needle)
	if n == 0 {
		return from
	}
	for i := from; i+n <= len(hay); i++ {
		ok := true
		for j := 0; j < n; j++ {
			if hay[i+j] != needle[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func window(src string, max, overlap int, delim string) []Chunk {
	rs := []rune(src)
	if len(rs) == 0 {
		return nil
	}
	if overlap >= max {
		overlap = max / 10
	}
	var out []Chunk
	i := 0
	for i < len(rs) {
		end := i + max
		if end > len(rs) {
			end = len(rs)
		} else {
			end = breakAt(rs, i, end, delim)
		}
		if end <= i {
			end = i + max
			if end > len(rs) {
				end = len(rs)
			}
		}
		text := strings.TrimSpace(string(rs[i:end]))
		if text != "" {
			out = append(out, Chunk{Index: len(out), Text: text, Start: i, End: end})
		}
		if end >= len(rs) {
			break
		}
		next := end - overlap
		if next <= i {
			next = end
		}
		i = next
	}
	return out
}

func breakAt(rs []rune, start, end int, delim string) int {
	seg := string(rs[start:end])
	prio := []string{"\n\n", "\n", "。", "！", "？", ".", "!", "?"}
	if delim == "blank" {
		prio = []string{"\n\n"}
	} else if delim == "newline" {
		prio = []string{"\n"}
	} else if delim == "period" {
		prio = []string{"。", "."}
	}
	minKeep := len(seg) / 5
	for _, d := range prio {
		p := strings.LastIndex(seg, d)
		if p >= minKeep {
			return start + len([]rune(seg[:p])) + len([]rune(d))
		}
	}
	return end
}

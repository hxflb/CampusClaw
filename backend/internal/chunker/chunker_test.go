package chunker

import (
	"strings"
	"testing"
)

func TestHierarchyUsesGivenLength(t *testing.T) {
	body := "# 标题\n" + strings.Repeat("字", 500)
	chunks := Split(body, Opts{Strategy: "hierarchy", MaxRunes: 200, OverlapRunes: 20})
	if len(chunks) < 3 {
		t.Fatalf("long chapter should split, got %d", len(chunks))
	}
	for _, c := range chunks {
		if n := len([]rune(c.Text)); n > 200 {
			t.Fatalf("chunk length %d exceeds 200", n)
		}
	}
}

func TestHierarchyDefaultsTo800(t *testing.T) {
	body := "# 标题\n" + strings.Repeat("字", 900)
	chunks := Split(body, Opts{Strategy: "hierarchy"})
	if len(chunks) < 2 {
		t.Fatalf("default max should split a 900-rune chapter, got %d", len(chunks))
	}
	for _, c := range chunks {
		if n := len([]rune(c.Text)); n > 800 {
			t.Fatalf("chunk length %d exceeds 800", n)
		}
	}
}

func TestHierarchyKeepsShortChapters(t *testing.T) {
	chunks := Split("# 甲\n短文\n## 乙\n另一段", Opts{Strategy: "hierarchy", MaxRunes: 800, OverlapRunes: 80})
	if len(chunks) != 2 {
		t.Fatalf("two short chapters, got %d", len(chunks))
	}
}

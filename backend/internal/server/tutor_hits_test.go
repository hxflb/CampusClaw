package server

import (
	"testing"

	"campusclaw/internal/retrieve"
)

func TestKeepCheckedMaterials(t *testing.T) {
	hits := []retrieve.Hit{{MaterialID: 1}, {MaterialID: 2}, {MaterialID: 3}}
	got := keepCheckedMaterials(hits, []int{2})
	if len(got) != 1 || got[0].MaterialID != 2 {
		t.Fatalf("checked filter = %+v", got)
	}
	if len(keepCheckedMaterials(hits, nil)) != 0 {
		t.Fatal("no checked materials must not fall back to the whole class")
	}
	titled := []retrieve.Hit{{Title: "数学自学笔记"}, {Title: "语文自学笔记"}}
	gotSubject := keepSubject(titled, "数学")
	if len(gotSubject) != 1 || gotSubject[0].Title != "数学自学笔记" {
		t.Fatalf("%+v", gotSubject)
	}
	if len(keepSubject(titled, "")) != 2 {
		t.Fatal("empty subject must keep every hit")
	}
}

func TestSelectHitsTakesTopFourThenFiltersSubject(t *testing.T) {
	note := func(title string) retrieve.Hit { return retrieve.Hit{MaterialID: 1, Title: title} }
	hits := []retrieve.Hit{
		note("语文笔记"), note("语文笔记"), note("语文笔记"), note("语文笔记"), note("数学笔记"),
	}
	if got := selectHits(hits, []int{1}, "数学"); len(got) != 0 {
		t.Fatalf("subject filter must run after the top-4 cut: %+v", got)
	}
	mixed := []retrieve.Hit{note("语文笔记"), note("数学笔记")}
	got := selectHits(mixed, []int{1}, "数学")
	if len(got) != 1 || got[0].Title != "数学笔记" {
		t.Fatalf("%+v", got)
	}
	if len(selectHits(hits, []int{2}, "")) != 0 {
		t.Fatal("unbound materials must be dropped")
	}
	if len(selectHits(hits, []int{1}, "")) != 4 {
		t.Fatalf("cap = %d", len(selectHits(hits, []int{1}, "")))
	}
}

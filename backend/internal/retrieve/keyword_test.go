package retrieve

import "testing"

func TestKeepStrongKeywords(t *testing.T) {
	score := func(v float64) *float64 { return &v }
	hits := []Hit{
		{MaterialID: 8, KeywordScore: score(1.91)},
		{MaterialID: 17, KeywordScore: score(0.55)},
		{MaterialID: 10, KeywordScore: score(0.01)},
	}
	got := keepStrongKeywords(hits)
	if len(got) != 1 || got[0].MaterialID != 8 {
		t.Fatalf("got %+v", got)
	}
	only := keepStrongKeywords([]Hit{{MaterialID: 17, KeywordScore: score(0.55)}})
	if len(only) != 1 {
		t.Fatalf("single weak hit should stay, got %d", len(only))
	}
}

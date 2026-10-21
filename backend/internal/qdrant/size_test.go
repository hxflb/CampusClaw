package qdrant

import "testing"

func TestParseVectorSize(t *testing.T) {
	plain := []byte(`{"result":{"config":{"params":{"vectors":{"size":2048,"distance":"Cosine"}}}}}`)
	if got := parseVectorSize(plain); got != 2048 {
		t.Fatalf("plain %d", got)
	}
	named := []byte(`{"result":{"config":{"params":{"vectors":{"body":{"size":384,"distance":"Cosine"}}}}}}`)
	if got := parseVectorSize(named); got != 384 {
		t.Fatalf("named %d", got)
	}
	if got := parseVectorSize([]byte(`{}`)); got != 0 {
		t.Fatalf("empty %d", got)
	}
}

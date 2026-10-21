package config

import "testing"

func TestOpenAIURL(t *testing.T) {
	root := "https://ai-gateway.devops.hello1023.com"
	withV1 := root + "/v1"
	if got := OpenAIURL(root, "/chat/completions"); got != withV1+"/chat/completions" {
		t.Fatalf("root: %s", got)
	}
	if got := OpenAIURL(withV1+"/", "/embeddings"); got != withV1+"/embeddings" {
		t.Fatalf("v1: %s", got)
	}
	if got := OpenAIURL(withV1, "/chat/completions"); got != withV1+"/chat/completions" || containsTwice(got) {
		t.Fatalf("doubled: %s", got)
	}
}

func containsTwice(s string) bool {
	return len(s) > 0 && (count(s, "/v1/v1") > 0)
}

func count(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

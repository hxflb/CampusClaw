package token

import (
	"strings"
	"testing"
	"time"
)

func sample() Claims {
	return Claims{
		UserID: 2, Username: "student_a1", Role: "student", ClassID: 1,
		JTI: "abc", Iat: 1_700_000_000, Exp: 1_700_003_600,
	}
}

func TestSignParse(t *testing.T) {
	raw, err := Sign("secret", sample())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(raw, ".") != 2 {
		t.Fatalf("token %s", raw)
	}
	got, err := Parse("secret", raw, time.Unix(1_700_000_100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "student_a1" || got.Role != "student" || got.ClassID != 1 || got.UserID != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestRejectTamperAndAlg(t *testing.T) {
	raw, err := Sign("secret", sample())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(raw, ".")
	mid := []byte(parts[1])
	if mid[1] == 'A' {
		mid[1] = 'B'
	} else {
		mid[1] = 'A'
	}
	if _, err := Parse("secret", parts[0]+"."+string(mid)+"."+parts[2], time.Unix(1_700_000_100, 0)); err == nil {
		t.Fatal("tampered payload accepted")
	}
	if _, err := Parse("other", raw, time.Unix(1_700_000_100, 0)); err == nil {
		t.Fatal("wrong secret accepted")
	}
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + parts[1] + "."
	if _, err := Parse("secret", none, time.Unix(1_700_000_100, 0)); err == nil {
		t.Fatal("alg none accepted")
	}
	if _, err := Parse("secret", raw, time.Unix(1_700_003_600, 0)); err == nil {
		t.Fatal("expired token accepted")
	}
}

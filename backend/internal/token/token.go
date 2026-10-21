package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid token")

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	ClassID  int    `json:"class_id"`
	JTI      string `json:"jti"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

func Sign(secret string, c Claims) (string, error) {
	if secret == "" || c.UserID == 0 || c.JTI == "" || c.Exp <= c.Iat {
		return "", ErrInvalid
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func Parse(secret, raw string, now time.Time) (Claims, error) {
	var c Claims
	parts := strings.Split(raw, ".")
	if secret == "" || len(parts) != 3 {
		return c, ErrInvalid
	}
	headRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, ErrInvalid
	}
	var head struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if json.Unmarshal(headRaw, &head) != nil || head.Alg != "HS256" || head.Typ != "JWT" {
		return c, ErrInvalid
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return c, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &c) != nil {
		return c, ErrInvalid
	}
	if c.UserID == 0 || c.JTI == "" || c.Exp <= now.Unix() {
		return Claims{}, ErrInvalid
	}
	return c, nil
}

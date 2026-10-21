package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SessionSecret   string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	SeedPassword    string
	UploadDir       string
	MaxUploadBytes  int64
	SessionTTLHours int
	FailLimit       int
	LockSeconds     int
	ListenAddr      string
	QdrantURL       string
	VectorSize      int
	EmbedBaseURL    string
	EmbedAPIKey     string
	EmbedModel      string
	ChatBaseURL     string
	ChatAPIKey      string
	ChatModel       string
	McpToken        string
	MaxToolRounds   int
	MaxToolCalls    int
}

func Load() (*Config, error) {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	seed := os.Getenv("SEED_PASSWORD")
	if seed == "" {
		return nil, fmt.Errorf("SEED_PASSWORD is required")
	}
	user := getenv("DB_USER", "")
	if user == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}
	name := getenv("DB_NAME", "")
	if name == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}
	maxB, _ := strconv.ParseInt(getenv("MAX_UPLOAD_BYTES", "2097152"), 10, 64)
	ttl, _ := strconv.Atoi(getenv("SESSION_TTL_HOURS", "24"))
	fl, _ := strconv.Atoi(getenv("LOGIN_FAIL_LIMIT", "8"))
	lk, _ := strconv.Atoi(getenv("LOGIN_LOCK_SECONDS", "60"))
	return &Config{
		SessionSecret:   secret,
		DBHost:          getenv("DB_HOST", "db"),
		DBPort:          getenv("DB_PORT", "3306"),
		DBUser:          user,
		DBPassword:      dbPass,
		DBName:          name,
		SeedPassword:    seed,
		UploadDir:       getenv("UPLOAD_DIR", "/data/uploads"),
		MaxUploadBytes:  maxB,
		SessionTTLHours: ttl,
		FailLimit:       fl,
		LockSeconds:     lk,
		ListenAddr:      getenv("LISTEN_ADDR", ":8080"),
		QdrantURL:       getenv("QDRANT_URL", "http://qdrant:6333"),
		VectorSize:      vectorSize(),
		EmbedBaseURL:    os.Getenv("EMBEDDING_BASE_URL"),
		EmbedAPIKey:     os.Getenv("EMBEDDING_API_KEY"),
		EmbedModel:      getenv("EMBEDDING_MODEL", "text-embedding-3-small"),
		ChatBaseURL:     os.Getenv("CHAT_BASE_URL"),
		ChatAPIKey:      os.Getenv("CHAT_API_KEY"),
		ChatModel:       getenv("CHAT_MODEL", "gpt-4o-mini"),
		McpToken:        os.Getenv("MCP_AUTH_TOKEN"),
		MaxToolRounds:   positiveInt("MCP_MAX_ROUNDS", 3),
		MaxToolCalls:    positiveInt("MCP_MAX_CALLS", 4),
	}, nil
}

func positiveInt(key string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func vectorSize() int {
	if v := strings.TrimSpace(os.Getenv("EMBEDDING_DIM")); v != "" {
		return atoi(v)
	}
	return atoi(getenv("VECTOR_SIZE", "384"))
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// OpenAIURL joins a course-gateway base with an OpenAI path such as "/chat/completions".
// A base of either "https://host" or "https://host/v1" becomes "https://host/v1" + suffix.
func OpenAIURL(base, suffix string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(b), "/v1") {
		return b + suffix
	}
	return b + "/v1" + suffix
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

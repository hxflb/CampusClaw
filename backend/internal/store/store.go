package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"campusclaw/internal/config"
	"campusclaw/internal/skillfile"
	"campusclaw/internal/tutor"
)

func Open(cfg *config.Config) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < 40; i++ {
		db, err = sql.Open("mysql", cfg.DSN())
		if err == nil {
			if ping := db.Ping(); ping == nil {
				return db, nil
			} else {
				err = ping
			}
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("db not ready: %w", err)
}

func Init(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS classes (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL UNIQUE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(64) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role ENUM('teacher','student') NOT NULL,
			class_id INT NOT NULL,
			INDEX idx_users_class (class_id),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id CHAR(64) PRIMARY KEY,
			user_id INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS materials (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			class_id INT NOT NULL,
			file_path VARCHAR(512) NOT NULL,
			uploaded_by INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_mat_class (class_id),
			FOREIGN KEY (class_id) REFERENCES classes(id),
			FOREIGN KEY (uploaded_by) REFERENCES users(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INT AUTO_INCREMENT PRIMARY KEY,
			material_id INT NOT NULL,
			class_id INT NOT NULL,
			body_text MEDIUMTEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_ke_class (class_id),
			FOREIGN KEY (material_id) REFERENCES materials(id),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS knowledge_chunks (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			knowledge_entry_id INT NOT NULL,
			material_id INT NOT NULL,
			class_id INT NOT NULL,
			chunk_index INT NOT NULL,
			chunk_text MEDIUMTEXT NOT NULL,
			char_start INT NOT NULL,
			char_end INT NOT NULL,
			index_status VARCHAR(16) NOT NULL DEFAULT 'pending',
			strategy VARCHAR(16) NOT NULL DEFAULT 'auto',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_chunk_class (class_id, index_status),
			INDEX idx_chunk_mat (material_id),
			FULLTEXT INDEX ft_chunk (chunk_text) WITH PARSER ngram,
			FOREIGN KEY (knowledge_entry_id) REFERENCES knowledge_entries(id) ON DELETE CASCADE,
			FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS lectures (
			id INT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			INDEX idx_lec_class (class_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS homework (
			id INT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			INDEX idx_hw_class (class_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS assistants (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS skills (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(64) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_prompts (
			class_id INT PRIMARY KEY,
			body TEXT NOT NULL,
			updated_by INT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_skills (
			class_id INT NOT NULL,
			skill_key VARCHAR(32) NOT NULL,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			updated_by INT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (class_id, skill_key),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			user_id INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_conv_owner (user_id, id),
			FOREIGN KEY (class_id) REFERENCES classes(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_agents (
			id INT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			name VARCHAR(64) NOT NULL,
			prompt TEXT NOT NULL,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			UNIQUE KEY uniq_class_agent_name (class_id, name),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS skill_files (
			id INT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			name VARCHAR(32) NOT NULL,
			trigger_text VARCHAR(120) NOT NULL,
			subject VARCHAR(32) NOT NULL,
			tool VARCHAR(32) NOT NULL,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			body TEXT NOT NULL,
			source TEXT NOT NULL,
			UNIQUE KEY uniq_skill_file_name (class_id, name),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_agent_skill_files (
			agent_id INT NOT NULL,
			skill_id INT NOT NULL,
			PRIMARY KEY (agent_id, skill_id),
			FOREIGN KEY (agent_id) REFERENCES class_agents(id) ON DELETE CASCADE,
			FOREIGN KEY (skill_id) REFERENCES skill_files(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_agent_skills (
			agent_id INT NOT NULL,
			skill_key VARCHAR(32) NOT NULL,
			PRIMARY KEY (agent_id, skill_key),
			FOREIGN KEY (agent_id) REFERENCES class_agents(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS class_agent_materials (
			agent_id INT NOT NULL,
			material_id INT NOT NULL,
			PRIMARY KEY (agent_id, material_id),
			FOREIGN KEY (agent_id) REFERENCES class_agents(id) ON DELETE CASCADE,
			FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversation_messages (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			conversation_id BIGINT NOT NULL,
			role ENUM('user','assistant') NOT NULL,
			content MEDIUMTEXT NOT NULL,
			citations_json MEDIUMTEXT NULL,
			prompt_snapshot TEXT NULL,
			skill_enabled TINYINT(1) NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_cmsg (conversation_id, id),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id INT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			name VARCHAR(32) NOT NULL,
			base_url VARCHAR(255) NOT NULL,
			token VARCHAR(512) NOT NULL DEFAULT '',
			connected TINYINT(1) NOT NULL DEFAULT 0,
			UNIQUE KEY uniq_mcp_server (class_id, name),
			FOREIGN KEY (class_id) REFERENCES classes(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS mcp_tools (
			server_id INT NOT NULL,
			name VARCHAR(32) NOT NULL,
			description VARCHAR(255) NOT NULL,
			input_schema MEDIUMTEXT NULL,
			PRIMARY KEY (server_id, name),
			FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS mcp_audit (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			class_id INT NOT NULL,
			user_id INT NOT NULL,
			conversation_id BIGINT NULL,
			round_no INT NOT NULL DEFAULT 0,
			tool_ref VARCHAR(80) NOT NULL,
			status VARCHAR(16) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_mcp_audit_class (class_id, id),
			FOREIGN KEY (class_id) REFERENCES classes(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN agent_id INT NULL`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN title VARCHAR(64) NOT NULL DEFAULT '新的提问'`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN updated_at DATETIME NULL`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE conversation_messages ADD COLUMN skill_name VARCHAR(32) NULL`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE skill_files MODIFY tool VARCHAR(64) NOT NULL`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE mcp_servers ADD COLUMN token VARCHAR(512) NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE mcp_tools ADD COLUMN input_schema MEDIUMTEXT NULL`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE mcp_audit ADD COLUMN conversation_id BIGINT NULL`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE mcp_audit ADD COLUMN round_no INT NOT NULL DEFAULT 0`); err != nil && !strings.Contains(err.Error(), "Duplicate column") {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE conversations SET updated_at=created_at WHERE updated_at IS NULL`); err != nil {
		return err
	}
	if err := backfillTitles(ctx, db); err != nil {
		return err
	}
	return seed(ctx, db, cfg)
}

func backfillTitles(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT c.id, (
		SELECT m.content FROM conversation_messages m
		WHERE m.conversation_id=c.id AND m.role='user' ORDER BY m.id LIMIT 1
	) FROM conversations c WHERE c.title='新的提问'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id      int64
		content sql.NullString
	}
	var pending []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.content); err != nil {
			return err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range pending {
		if !item.content.Valid {
			continue
		}
		title := clipTitle(item.content.String)
		if title == "" || title == "新的提问" {
			continue
		}
		if _, err := db.ExecContext(ctx, `UPDATE conversations SET title=? WHERE id=? AND title='新的提问'`, title, item.id); err != nil {
			return err
		}
	}
	return nil
}

func clipTitle(s string) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) > 24 {
		rs = rs[:24]
	}
	return string(rs)
}

func seed(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO classes (id, name) VALUES (1, 'A 班'), (2, 'B 班')`); err != nil {
		return err
	}
	users := []struct {
		u, role string
		cid     int
	}{
		{"teacher_a", "teacher", 1},
		{"student_a1", "student", 1},
		{"student_b1", "student", 2},
	}
	for _, u := range users {
		_, err := db.ExecContext(ctx, `INSERT IGNORE INTO users (username, password_hash, role, class_id) VALUES (?, ?, ?, ?)`,
			u.u, string(hash), u.role, u.cid)
		if err != nil {
			return err
		}
	}
	if err := seedTutor(ctx, db); err != nil {
		return err
	}
	if err := seedAgents(ctx, db); err != nil {
		return err
	}
	if err := seedSkillFiles(ctx, db); err != nil {
		return err
	}
	if err := seedWeatherSkill(ctx, db); err != nil {
		return err
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM materials`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var teacherID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE username='teacher_a'`).Scan(&teacherID); err != nil {
		return err
	}
	// 标题带上种子技能的学科「数学」：第 7 步按标题筛学科，标题不含学科会让命中全被筛掉。
	seeds := []struct {
		title, body string
		cid         int
		file        string
	}{
		{"A 班春季教研材料（数学）", "A 班春季教研材料：本周作文讲评要点。", 1, "seed-a.txt"},
		{"B 班春季教研材料（数学）", "B 班春季教研材料：本周阅读理解训练。", 2, "seed-b.txt"},
	}
	for _, s := range seeds {
		path := cfg.UploadDir + "/" + s.file
		if err := os.WriteFile(path, []byte(s.body+"\n"), 0o644); err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO materials (title, class_id, file_path, uploaded_by) VALUES (?, ?, ?, ?)`,
			s.title, s.cid, path, teacherID)
		if err != nil {
			tx.Rollback()
			return err
		}
		mid, _ := res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)`,
			mid, s.cid, s.body); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func seedAgents(ctx context.Context, db *sql.DB) error {
	for _, cid := range []int{1, 2} {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM class_agents WHERE class_id=?`, cid).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		var prompt string
		if err := db.QueryRowContext(ctx, `SELECT body FROM class_prompts WHERE class_id=?`, cid).Scan(&prompt); err != nil {
			prompt = tutor.DefaultPrompt
		}
		res, err := db.ExecContext(ctx, `INSERT INTO class_agents (class_id, name, prompt, enabled) VALUES (?, ?, ?, 1)`, cid, "默认解题助手", prompt)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		var en int
		if err := db.QueryRowContext(ctx, `SELECT enabled FROM class_skills WHERE class_id=? AND skill_key=?`, cid, tutor.SkillGuide).Scan(&en); err != nil || en == 1 {
			if _, err := db.ExecContext(ctx, `INSERT INTO class_agent_skills (agent_id, skill_key) VALUES (?, ?)`, id, tutor.SkillGuide); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedSkillFiles(ctx context.Context, db *sql.DB) error {
	doc, ok := skillfile.Parse(skillfile.Example)
	if !ok {
		return fmt.Errorf("seed skill file")
	}
	for _, cid := range []int{1, 2} {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skill_files WHERE class_id=?`, cid).Scan(&n); err != nil {
			return err
		}
		var id int64
		if n == 0 {
			res, err := db.ExecContext(ctx, `INSERT INTO skill_files (class_id, name, trigger_text, subject, tool, enabled, body, source)
				VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, cid, doc.Name, doc.Trigger, doc.Subject, doc.Tool, doc.Body, doc.Source)
			if err != nil {
				return err
			}
			id, _ = res.LastInsertId()
		} else {
			if err := db.QueryRowContext(ctx, `SELECT id FROM skill_files WHERE class_id=? AND name=?`, cid, doc.Name).Scan(&id); err != nil {
				continue
			}
		}
		if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO class_agent_skill_files (agent_id, skill_id)
			SELECT s.agent_id, ? FROM class_agent_skills s
			JOIN class_agents a ON a.id=s.agent_id
			WHERE a.class_id=? AND s.skill_key=?`, id, cid, tutor.SkillGuide); err != nil {
			return err
		}
	}
	return nil
}

func seedWeatherSkill(ctx context.Context, db *sql.DB) error {
	const source = "名称：天气查询\n适用条件：天气,气温,下雨,温度\n资料：\n工具：weather_query@campus-tools\n启用：是\n\n学生在问某个城市现在的天气。只根据工具返回说明现象和气温，不要编造数字。\n"
	doc, ok := skillfile.Parse(source)
	if !ok {
		return fmt.Errorf("weather skill")
	}
	for _, cid := range []int{1, 2} {
		var id int64
		err := db.QueryRowContext(ctx, `SELECT id FROM skill_files WHERE class_id=? AND name=?`, cid, doc.Name).Scan(&id)
		if err == sql.ErrNoRows {
			res, err := db.ExecContext(ctx, `INSERT INTO skill_files (class_id, name, trigger_text, subject, tool, enabled, body, source)
				VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, cid, doc.Name, doc.Trigger, doc.Subject, doc.Tool, doc.Body, doc.Source)
			if err != nil {
				return err
			}
			id, _ = res.LastInsertId()
		} else if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO class_agent_skill_files (agent_id, skill_id)
			SELECT id, ? FROM class_agents WHERE class_id=? AND name=?`, id, cid, "默认解题助手"); err != nil {
			return err
		}
	}
	return nil
}

func seedTutor(ctx context.Context, db *sql.DB) error {
	for _, cid := range []int{1, 2} {
		if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO class_prompts (class_id, body) VALUES (?, ?)`, cid, tutor.DefaultPrompt); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO class_skills (class_id, skill_key, enabled) VALUES (?, ?, 1)`, cid, tutor.SkillGuide); err != nil {
			return err
		}
	}
	return nil
}

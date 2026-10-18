package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// User 用户（Web 版身份：自增 ID + 登录id）
type User struct {
	ID        int      `json:"userID"`
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	MMR       int      `json:"mmr"`
	Roles     []int    `json:"roles"`
	Signature string   `json:"signature"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
	IsAdmin   bool     `json:"isAdmin,omitempty"`
	IsSuper   bool     `json:"isSuper,omitempty"`
}

// AdminsConfig 管理员名单（名单制：超管 + 管理员）
type AdminsConfig struct {
	SuperAdmin int   `json:"superAdmin"`
	Admins     []int `json:"admins"`
}

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，避免锁冲突
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  nickname TEXT NOT NULL,
  mmr INTEGER NOT NULL DEFAULT 0,
  roles TEXT NOT NULL DEFAULT '[]',
  signature TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateUser(username, passwordHash, nickname string, now int64) (*User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, nickname, mmr, roles, signature, created_at, updated_at) VALUES (?,?,?,0,'[]','',?,?)`,
		username, passwordHash, nickname, now, now,
	)
	if err != nil {
		return nil, err // 唯一约束冲突
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(int(id))
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var roles string
	var hash string
	if err := row.Scan(&u.ID, &u.Username, &hash, &u.Nickname, &u.MMR, &roles, &u.Signature, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(roles), &u.Roles)
	return &u, nil
}

func (s *Store) GetUserByID(id int) (*User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, nickname, mmr, roles, signature, created_at, updated_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, nickname, mmr, roles, signature, created_at, updated_at FROM users WHERE username=?`, username)
	return scanUser(row)
}

func (s *Store) GetPasswordHashByUsername(username string) (int, string, error) {
	var id int
	var hash string
	err := s.db.QueryRow(`SELECT id, password_hash FROM users WHERE username=?`, username).Scan(&id, &hash)
	return id, hash, err
}

// NicknameTaken 昵称是否被其他人占用（排除 selfID）
func (s *Store) NicknameTaken(nickname string, selfID int) (bool, error) {
	var cnt int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE nickname=? AND id<>?`, nickname, selfID).Scan(&cnt)
	return cnt > 0, err
}

func (s *Store) UpdateProfile(id int, nickname string, mmr int, roles []int, signature string, now int64) error {
	rolesJSON, _ := json.Marshal(roles)
	_, err := s.db.Exec(
		`UPDATE users SET nickname=?, mmr=?, roles=?, signature=?, updated_at=? WHERE id=?`,
		nickname, mmr, string(rolesJSON), signature, now, id,
	)
	return err
}

func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, nickname, mmr, roles, signature, created_at, updated_at FROM users ORDER BY nickname ASC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ---------- 管理员名单 ----------

func (s *Store) GetAdminsConfig() (*AdminsConfig, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key='admins'`).Scan(&value)
	if err == sql.ErrNoRows {
		return &AdminsConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := &AdminsConfig{}
	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return &AdminsConfig{}, nil
	}
	return cfg, nil
}

func (s *Store) SetAdminsConfig(cfg *AdminsConfig) error {
	value, _ := json.Marshal(cfg)
	_, err := s.db.Exec(
		`INSERT INTO config (key, value) VALUES ('admins', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		string(value),
	)
	return err
}

func (s *Store) IsAdmin(uid int, cfg *AdminsConfig) bool {
	if cfg.SuperAdmin == uid {
		return true
	}
	for _, id := range cfg.Admins {
		if id == uid {
			return true
		}
	}
	return false
}

func (s *Store) IsSuper(uid int, cfg *AdminsConfig) bool {
	return cfg.SuperAdmin == uid
}

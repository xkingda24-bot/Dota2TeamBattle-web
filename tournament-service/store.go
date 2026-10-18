package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// Registration 报名记录
type Registration struct {
	UserID    int      `json:"userID"`
	Nickname  string   `json:"nickname"`
	Role      string   `json:"role"` // captain | player | waitlist
	TS        int64    `json:"ts"`
	MMR       int      `json:"mmr"`
	Roles     []int    `json:"roles"`
	Signature string   `json:"signature"`
	TeamName  string   `json:"teamName,omitempty"`
}

// CaptainInfo 队长信息（选人顺序中的队长）
type CaptainInfo struct {
	UserID    int      `json:"userID"`
	Nickname  string   `json:"nickname"`
	TeamName  string   `json:"teamName,omitempty"`
	MMR       int      `json:"mmr"`
	Roles     []int    `json:"roles"`
	Signature string   `json:"signature"`
}

// TeamMember 队伍成员
type TeamMember struct {
	UserID    int      `json:"userID"`
	Nickname  string   `json:"nickname"`
	MMR       int      `json:"mmr"`
	Roles     []int    `json:"roles"`
	Signature string   `json:"signature"`
}

// Team 队伍
type Team struct {
	Captain CaptainInfo `json:"captain"`
	Players []TeamMember `json:"players"`
}

// Draft 选人状态
type Draft struct {
	Turn  int      `json:"turn"`
	Picks [][]int  `json:"picks"` // picks[i] = 第 i 队已选队员 userID 列表
}

// Tournament 比赛（Web 版：openid → userID）
type Tournament struct {
	ID            int64          `json:"id"`
	Name          string         `json:"name"`
	Time          string         `json:"time"`
	Secret        string         `json:"secret"`
	Mode          string         `json:"mode"` // captain | auto
	N             int            `json:"N"`
	K             int            `json:"K"`
	WaitLimit     int            `json:"waitLimit"`
	Status        string         `json:"status"` // registration | draft | battle | finished
	Registrations []Registration `json:"registrations"`
	CaptainOrder  []CaptainInfo  `json:"captainOrder"`
	Draft         *Draft         `json:"draft"`
	Teams         []Team         `json:"teams"`
	Seeding       []int          `json:"seeding"`
	Matches       []*Match       `json:"matches"`
	Champion      *ChampionInfo  `json:"champion"`
	CreatedBy     int            `json:"createdBy"`
	CreatorName   string         `json:"creatorName"`
	CreatedAt     int64          `json:"createdAt"`
	ArchivedAt    *int64         `json:"archivedAt"`
	UpdatedAt     int64          `json:"updatedAt"`
}

// ChampionInfo 冠军信息
type ChampionInfo struct {
	UserID int    `json:"userID"`
	Name   string `json:"name"`
}

type Store struct {
	db *sql.DB
}

func (s *Store) Close() error {
	return s.db.Close()
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tournaments (id INTEGER PRIMARY KEY AUTOINCREMENT, data TEXT NOT NULL);`); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Create(t *Tournament) (int64, error) {
	data, _ := json.Marshal(t)
	res, err := s.db.Exec(`INSERT INTO tournaments (data) VALUES (?)`, string(data))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	t.ID = id
	// 重新写入带 ID 的完整 JSON，保证反序列化后 ID 一致（后续 Save 依赖 WHERE id=?）
	data2, _ := json.Marshal(t)
	if _, err := s.db.Exec(`UPDATE tournaments SET data=? WHERE id=?`, string(data2), id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) Get(id int64) (*Tournament, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM tournaments WHERE id=?`, id).Scan(&data)
	if err != nil {
		return nil, err
	}
	t := &Tournament{}
	if err := json.Unmarshal([]byte(data), t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) Save(t *Tournament) error {
	data, _ := json.Marshal(t)
	_, err := s.db.Exec(`UPDATE tournaments SET data=? WHERE id=?`, string(data), t.ID)
	return err
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tournaments WHERE id=?`, id)
	return err
}

// ListAll 全量加载（按创建时间倒序）。比赛数量级很小，Go 侧过滤足够。
func (s *Store) ListAll() ([]*Tournament, error) {
	rows, err := s.db.Query(`SELECT id, data FROM tournaments ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Tournament{}
	for rows.Next() {
		var id int64
		var data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		t := &Tournament{}
		if err := json.Unmarshal([]byte(data), t); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// AdminsConfig 管理员名单（从 auth-service 内部接口获取）
type AdminsConfig struct {
	SuperAdmin int   `json:"superAdmin"`
	Admins     []int `json:"admins"`
}

type AdminClient struct {
	url       string
	key       string
	mu        sync.Mutex
	cache     *AdminsConfig
	cacheTime time.Time
}

func NewAdminClient(url, key string) *AdminClient {
	return &AdminClient{url: url, key: key}
}

func (c *AdminClient) Get() (*AdminsConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache != nil && time.Since(c.cacheTime) < 10*time.Second {
		return c.cache, nil
	}
	req, err := http.NewRequest("GET", c.url+"/internal/admins", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", c.key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	cfg := &AdminsConfig{}
	if err := json.Unmarshal(body, cfg); err != nil {
		return nil, err
	}
	c.cache = cfg
	c.cacheTime = time.Now()
	return cfg, nil
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (c *AdminClient) IsAdmin(uid int) bool {
	cfg, err := c.Get()
	if err != nil {
		return false
	}
	return cfg.SuperAdmin == uid || containsInt(cfg.Admins, uid)
}

func (c *AdminClient) IsSuper(uid int) bool {
	cfg, err := c.Get()
	if err != nil {
		return false
	}
	return cfg.SuperAdmin == uid
}

// UserInfo 内部用户资料
type UserInfo struct {
	UserID    int    `json:"userID"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	MMR       int    `json:"mmr"`
	Roles     []int  `json:"roles"`
	Signature string `json:"signature"`
}

func (c *AdminClient) GetUser(uid int) (*UserInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/internal/user?id=%d", c.url, uid), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", c.key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	u := &UserInfo{}
	if err := json.Unmarshal(body, u); err != nil {
		return nil, err
	}
	return u, nil
}

// App 应用
type App struct {
	store       *Store
	adminClient *AdminClient
	locks       sync.Map // tournamentID -> *sync.Mutex（单实例部署，内存锁保证选人/结果并发安全）
	now         func() int64
}

func (a *App) lock(id int64) *sync.Mutex {
	v, _ := a.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func main() {
	addr := getenv("TOURNAMENT_ADDR", "127.0.0.1:8082")
	dbPath := getenv("TOURNAMENT_DB", "/opt/dota2/data/tournament.db")
	authURL := getenv("AUTH_INTERNAL_URL", "http://127.0.0.1:8081")
	internalKey := getenv("INTERNAL_KEY", "dev-internal-key")

	store, err := OpenStore(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer store.Close()

	app := &App{
		store:       store,
		adminClient: NewAdminClient(authURL, internalKey),
		now:         func() int64 { return time.Now().UnixMilli() },
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tournament/home", app.wrap(app.home))
	mux.HandleFunc("GET /api/tournament/{id}", app.wrap(app.tournament))
	mux.HandleFunc("POST /api/tournament", app.wrap(app.create))
	mux.HandleFunc("POST /api/tournament/{id}/config", app.wrap(app.updateConfig))
	mux.HandleFunc("POST /api/tournament/{id}/check-secret", app.wrap(app.checkSecret))
	mux.HandleFunc("POST /api/tournament/{id}/register", app.wrap(app.register))
	mux.HandleFunc("POST /api/tournament/{id}/cancel", app.wrap(app.cancelRegister))
	mux.HandleFunc("POST /api/tournament/{id}/remove-reg", app.wrap(app.removeRegistration))
	mux.HandleFunc("POST /api/tournament/{id}/move-reg", app.wrap(app.moveRegistration))
	mux.HandleFunc("POST /api/tournament/{id}/team-name", app.wrap(app.setTeamName))
	mux.HandleFunc("POST /api/tournament/{id}/start-draft", app.wrap(app.startDraft))
	mux.HandleFunc("POST /api/tournament/{id}/pick", app.wrap(app.pickPlayer))
	mux.HandleFunc("POST /api/tournament/{id}/reset-draft", app.wrap(app.resetDraft))
	mux.HandleFunc("POST /api/tournament/{id}/seeding", app.wrap(app.setSeeding))
	mux.HandleFunc("POST /api/tournament/{id}/shuffle-seeding", app.wrap(app.shuffleSeeding))
	mux.HandleFunc("POST /api/tournament/{id}/start-bracket", app.wrap(app.startBracket))
	mux.HandleFunc("POST /api/tournament/{id}/stage-rollback", app.wrap(app.stageRollback))
	mux.HandleFunc("POST /api/tournament/{id}/bo", app.wrap(app.setMatchBo))
	mux.HandleFunc("POST /api/tournament/{id}/result", app.wrap(app.setMatchResult))
	mux.HandleFunc("POST /api/tournament/{id}/reset-result", app.wrap(app.resetMatchResult))
	mux.HandleFunc("POST /api/tournament/{id}/finish", app.wrap(app.finishTournament))
	mux.HandleFunc("DELETE /api/tournament/{id}", app.wrap(app.deleteTournament))
	mux.HandleFunc("POST /api/tournament/{id}/auto-group", app.wrap(app.autoGroup))
	mux.HandleFunc("POST /api/tournament/{id}/confirm-group", app.wrap(app.confirmGroup))
	mux.HandleFunc("POST /api/tournament/{id}/reset-group", app.wrap(app.resetGroup))
	mux.HandleFunc("POST /api/tournament/{id}/seed-test", app.wrap(app.seedTest))

	log.Printf("tournament-service 监听 %s (db=%s)", addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// wrap 解析身份头并包装 handler（身份由网关注入，本服务不直接面对公网）
func (a *App) wrap(h func(http.ResponseWriter, *http.Request, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := 0
		hdr := r.Header.Get("X-User-Id")
		if hdr != "" {
			for _, ch := range hdr {
				if ch < '0' || ch > '9' {
					uid = 0
					break
				}
				uid = uid*10 + int(ch-'0')
			}
		}
		h(w, r, uid)
	}
}

// pathID 读取路径中的 {id}
func (a *App) pathID(r *http.Request) (int64, bool) {
	id := int64(0)
	raw := r.PathValue("id")
	if raw == "" {
		return 0, false
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		id = id*10 + int64(ch-'0')
	}
	return id, id > 0
}

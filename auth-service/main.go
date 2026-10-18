package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := getenv("AUTH_ADDR", "127.0.0.1:8081")
	dbPath := getenv("AUTH_DB", "/opt/dota2/data/auth.db")
	secret := []byte(getenv("JWT_SECRET", "dev-secret-change-me"))
	internalKey := getenv("INTERNAL_KEY", "dev-internal-key")

	store, err := OpenStore(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer store.Close()

	app := &App{store: store, secret: secret, now: func() int64 { return time.Now().Unix() }}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", app.registerUser)
	mux.HandleFunc("POST /api/auth/login", app.loginUser)
	mux.HandleFunc("GET /api/auth/me", app.authMiddleware(app.me))
	mux.HandleFunc("POST /api/auth/profile", app.authMiddleware(app.updateProfile))
	mux.HandleFunc("GET /api/auth/staff", app.authMiddleware(app.getStaff))
	mux.HandleFunc("POST /api/auth/init-super", app.authMiddleware(app.initSuperAdmin))
	mux.HandleFunc("POST /api/auth/add-admin", app.authMiddleware(app.addAdmin))
	mux.HandleFunc("POST /api/auth/remove-admin", app.authMiddleware(app.removeAdmin))
	mux.HandleFunc("GET /api/auth/users", app.authMiddleware(app.listUsers))
	// 内部接口（仅 tournament-service 经内网调用，校验共享密钥）
	mux.HandleFunc("GET /internal/admins", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Key") != internalKey {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		app.internalAdmins(w, r)
	})
	mux.HandleFunc("GET /internal/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Key") != internalKey {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		app.internalUser(w, r)
	})

	log.Printf("auth-service 监听 %s (db=%s)", addr, dbPath)
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

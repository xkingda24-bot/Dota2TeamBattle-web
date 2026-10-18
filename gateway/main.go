package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed web
var webFS embed.FS

// ---------- JWT (HS256, 与 auth-service 同算法同密钥) ----------

type Claims struct {
	UID      int    `json:"uid"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

func parseJWT(secret []byte, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("无效的令牌格式")
	}
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("无效的令牌签名")
	}
	if !hmac.Equal(expected, actual) {
		return nil, errors.New("令牌签名无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	c := &Claims{}
	if err := json.Unmarshal(payload, c); err != nil {
		return nil, err
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return nil, errors.New("令牌已过期")
	}
	if c.UID <= 0 {
		return nil, errors.New("令牌缺少用户信息")
	}
	return c, nil
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustProxy(target string) *httputil.ReverseProxy {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("代理目标无效 %s: %v", target, err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}

func main() {
	addr := getenv("GATEWAY_ADDR", "0.0.0.0:8080")
	authTarget := getenv("GATEWAY_AUTH_URL", "http://127.0.0.1:8081")
	tourTarget := getenv("GATEWAY_TOUR_URL", "http://127.0.0.1:8082")
	secret := []byte(getenv("JWT_SECRET", "dev-secret-change-me"))

	authProxy := mustProxy(authTarget)
	tourProxy := mustProxy(tourTarget)

	// tournament 路径统一处理：解析 JWT -> 注入 X-User-Id -> 转发
	tourHandler := func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseJWT(secret, bearer(r))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录或登录已失效"})
			return
		}
		r.Header.Set("X-User-Id", strconv.Itoa(claims.UID))
		r.Header.Del("Authorization")
		tourProxy.ServeHTTP(w, r)
	}

	mux := http.NewServeMux()
	// 前端静态资源（加防缓存头，避免浏览器缓存旧版页面）
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("前端资源加载失败: %v", err)
	}
	fileHandler := http.FileServer(http.FS(sub))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		fileHandler.ServeHTTP(w, r)
	}))
	// 认证服务原样转发（保留 Authorization 头，由 auth-service 自行校验）
	mux.HandleFunc("/api/auth/", func(w http.ResponseWriter, r *http.Request) {
		authProxy.ServeHTTP(w, r)
	})
	// 赛事服务（需登录）
	mux.HandleFunc("/api/tournament", tourHandler)
	mux.HandleFunc("/api/tournament/", tourHandler)

	log.Printf("gateway 监听 %s (auth=%s tournament=%s)", addr, authTarget, tourTarget)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

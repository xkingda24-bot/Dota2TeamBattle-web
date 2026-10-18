package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ---------- JWT (HS256, 自实现，无额外依赖) ----------

type Claims struct {
	UID      int    `json:"uid"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

func signJWT(secret []byte, c Claims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return sigInput + "." + sig, nil
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

// ---------- 响应工具 ----------

type resp struct {
	Error string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v interface{}) { writeJSON(w, http.StatusOK, v) }

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, resp{Error: msg})
}

// ---------- App ----------

type App struct {
	store  *Store
	secret []byte
	now    func() int64
}

func (a *App) loginUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	id, hash, err := a.store.GetPasswordHashByUsername(req.Username)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	user, err := a.store.GetUserByID(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	token, err := signJWT(a.secret, Claims{UID: id, Username: req.Username, Exp: a.now() + 7*24*3600})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	cfg, _ := a.store.GetAdminsConfig()
	user.IsAdmin = a.store.IsAdmin(id, cfg)
	user.IsSuper = a.store.IsSuper(id, cfg)
	writeOK(w, map[string]interface{}{"token": token, "user": user})
}

func (a *App) registerUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if !validUsername(req.Username) {
		writeErr(w, http.StatusBadRequest, "用户名需 3-32 位字母/数字/下划线")
		return
	}
	if len(req.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "密码至少 6 位")
		return
	}
	if req.Nickname == "" || len([]rune(req.Nickname)) > 20 {
		writeErr(w, http.StatusBadRequest, "昵称需 1-20 个字")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	now := a.now()
	user, err := a.store.CreateUser(req.Username, string(hash), req.Nickname, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeErr(w, http.StatusConflict, "用户名已被使用")
		} else {
			writeErr(w, http.StatusInternalServerError, "服务异常")
		}
		return
	}
	token, err := signJWT(a.secret, Claims{UID: user.ID, Username: user.Username, Exp: now + 7*24*3600})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{"token": token, "user": user})
}

func validUsername(u string) bool {
	if len(u) < 3 || len(u) > 32 {
		return false
	}
	for _, ch := range u {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '_' {
			return false
		}
	}
	return true
}

// currentUser 返回当前用户（带管理员标记）
func (a *App) currentUser(uid int) (*User, error) {
	u, err := a.store.GetUserByID(uid)
	if err != nil {
		return nil, err
	}
	cfg, _ := a.store.GetAdminsConfig()
	u.IsAdmin = a.store.IsAdmin(uid, cfg)
	u.IsSuper = a.store.IsSuper(uid, cfg)
	return u, nil
}

func (a *App) me(w http.ResponseWriter, r *http.Request, uid int) {
	u, err := a.currentUser(uid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	writeOK(w, u)
}

// 更新资料：昵称/天梯分/常玩位置/签名（天梯分与位置必填）
func (a *App) updateProfile(w http.ResponseWriter, r *http.Request, uid int) {
	var req struct {
		Nickname  string `json:"nickname"`
		MMR       *int   `json:"mmr"`
		Roles     []int  `json:"roles"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Signature = strings.TrimSpace(req.Signature)
	if req.Nickname == "" {
		writeErr(w, http.StatusBadRequest, "昵称不能为空")
		return
	}
	if len([]rune(req.Nickname)) > 20 {
		writeErr(w, http.StatusBadRequest, "昵称最多 20 个字")
		return
	}
	if req.MMR == nil {
		writeErr(w, http.StatusBadRequest, "请填写天梯分")
		return
	}
	if *req.MMR < 1 || *req.MMR > 20000 {
		writeErr(w, http.StatusBadRequest, "天梯分需在 1-20000 之间")
		return
	}
	roles := []int{}
	seen := map[int]bool{}
	for _, v := range req.Roles {
		if v >= 1 && v <= 5 && !seen[v] {
			seen[v] = true
			roles = append(roles, v)
		}
	}
	if len(roles) == 0 {
		writeErr(w, http.StatusBadRequest, "请至少选择一个常玩位置")
		return
	}
	if len([]rune(req.Signature)) > 15 {
		writeErr(w, http.StatusBadRequest, "个人签名最多 15 个字")
		return
	}
	taken, err := a.store.NicknameTaken(req.Nickname, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	if taken {
		writeErr(w, http.StatusConflict, "该昵称已被使用，请换一个")
		return
	}
	if err := a.store.UpdateProfile(uid, req.Nickname, *req.MMR, roles, req.Signature, a.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	u, err := a.currentUser(uid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	writeOK(w, u)
}

// ---------- 管理员 ----------

func (a *App) getStaff(w http.ResponseWriter, r *http.Request, uid int) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	ids := []int{cfg.SuperAdmin}
	ids = append(ids, cfg.Admins...)
	members := []*User{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		u, err := a.store.GetUserByID(id)
		if err != nil {
			continue
		}
		members = append(members, u)
	}
	u, _ := a.currentUser(uid)
	writeOK(w, map[string]interface{}{
		"user":           u,
		"superAdmin":     superOf(cfg, members),
		"admins":         nonSuperOf(cfg, members),
		"isAdmin":        a.store.IsAdmin(uid, cfg),
		"isSuper":        a.store.IsSuper(uid, cfg),
		"hasSuper":       cfg.SuperAdmin > 0,
	})
}

func superOf(cfg *AdminsConfig, members []*User) *User {
	for _, m := range members {
		if m.ID == cfg.SuperAdmin {
			return m
		}
	}
	return nil
}

func nonSuperOf(cfg *AdminsConfig, members []*User) []*User {
	out := []*User{}
	for _, m := range members {
		if m.ID != cfg.SuperAdmin {
			out = append(out, m)
		}
	}
	return out
}

func (a *App) initSuperAdmin(w http.ResponseWriter, r *http.Request, uid int) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	if cfg.SuperAdmin > 0 {
		writeErr(w, http.StatusConflict, "最高管理员已初始化")
		return
	}
	if err := a.store.SetAdminsConfig(&AdminsConfig{SuperAdmin: uid, Admins: []int{}}); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{"isSuper": true})
}

func (a *App) addAdmin(w http.ResponseWriter, r *http.Request, uid int) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	if !a.store.IsSuper(uid, cfg) {
		writeErr(w, http.StatusForbidden, "只有最高管理员可以添加管理员")
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	target, err := a.store.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		writeErr(w, http.StatusNotFound, "未找到该用户")
		return
	}
	if target.ID == uid {
		writeErr(w, http.StatusBadRequest, "你已经是最高管理员")
		return
	}
	for _, id := range cfg.Admins {
		if id == target.ID {
			writeErr(w, http.StatusConflict, "该用户已是管理员")
			return
		}
	}
	cfg.Admins = append(cfg.Admins, target.ID)
	if err := a.store.SetAdminsConfig(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

func (a *App) removeAdmin(w http.ResponseWriter, r *http.Request, uid int) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	if !a.store.IsSuper(uid, cfg) {
		writeErr(w, http.StatusForbidden, "只有最高管理员可以移除管理员")
		return
	}
	var req struct {
		UserID int `json:"userID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.UserID == uid {
		writeErr(w, http.StatusBadRequest, "不能移除自己（最高管理员）")
		return
	}
	admins := []int{}
	removed := false
	for _, id := range cfg.Admins {
		if id == req.UserID {
			removed = true
			continue
		}
		admins = append(admins, id)
	}
	if !removed {
		writeErr(w, http.StatusNotFound, "该用户不是管理员")
		return
	}
	cfg.Admins = admins
	if err := a.store.SetAdminsConfig(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

func (a *App) listUsers(w http.ResponseWriter, r *http.Request, uid int) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	if !a.store.IsSuper(uid, cfg) {
		writeErr(w, http.StatusForbidden, "只有最高管理员可以查看用户列表")
		return
	}
	excluded := map[int]bool{cfg.SuperAdmin: true}
	for _, id := range cfg.Admins {
		excluded[id] = true
	}
	users, err := a.store.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	out := []*User{}
	for _, u := range users {
		if excluded[u.ID] {
			continue
		}
		out = append(out, u)
	}
	writeOK(w, map[string]interface{}{"users": out})
}

// internalAdmins 供 tournament-service 内部查询管理员名单
func (a *App) internalAdmins(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.store.GetAdminsConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, cfg)
}

// internalUser 供 tournament-service 内部查询用户资料（报名校验用）
func (a *App) internalUser(w http.ResponseWriter, r *http.Request) {
	id := 0
	raw := r.URL.Query().Get("id")
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			id = 0
			break
		}
		id = id*10 + int(ch-'0')
	}
	u, err := a.store.GetUserByID(id)
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	writeOK(w, map[string]interface{}{
		"userID": u.ID, "username": u.Username, "nickname": u.Nickname,
		"mmr": u.MMR, "roles": u.Roles, "signature": u.Signature,
	})
}

func (a *App) authMiddleware(h func(http.ResponseWriter, *http.Request, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "未登录")
			return
		}
		c, err := parseJWT(a.secret, strings.TrimPrefix(hdr, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		h(w, r, c.UID)
	}
}

var _ = fmt.Sprintf

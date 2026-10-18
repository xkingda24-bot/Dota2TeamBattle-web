package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

const (
	MAX_N   = 16
	MAX_K   = 5
	BO_LIST = "BO1,BO3,BO5"
)

// ---------- 响应工具 ----------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v interface{}) { writeJSON(w, http.StatusOK, v) }

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func shuffleInts(arr []int) []int {
	a := make([]int, len(arr))
	copy(a, arr)
	for i := len(a) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
	return a
}

func shuffleRegs(arr []Registration) []Registration {
	a := make([]Registration, len(arr))
	copy(a, arr)
	for i := len(a) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
	return a
}

func queryInt(r *http.Request, key string, def int64) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// ---------- 对阵解析（对齐云函数 resolveBracket） ----------

// ResolvedMatch 带展示信息的比赛
type ResolvedMatch struct {
	*Match
	TeamAInfo *TeamRef `json:"teamAInfo"`
	TeamBInfo *TeamRef `json:"teamBInfo"`
	DisplayA  string   `json:"displayA"`
	DisplayB  string   `json:"displayB"`
}

// TeamRef 实际队伍引用
type TeamRef struct {
	UserID int    `json:"userID"`
	Name   string `json:"name"`
}

func resolveBracket(t *Tournament) []*ResolvedMatch {
	w := map[int]*TeamRef{} // matchId -> winner team
	l := map[int]*TeamRef{} // matchId -> loser team

	resolveSrc := func(src *TeamSource) *TeamRef {
		if src == nil {
			return nil
		}
		if src.Type == "team" {
			return &TeamRef{UserID: parseTeamUID(src.Openid), Name: src.Name}
		}
		if src.Slot == "winner" {
			return w[src.MatchID]
		}
		return l[src.MatchID]
	}

	out := []*ResolvedMatch{}
	for _, m := range t.Matches {
		rm := &ResolvedMatch{Match: m}
		a := resolveSrc(m.TeamA)
		b := resolveSrc(m.TeamB)
		rm.TeamAInfo = a
		rm.TeamBInfo = b
		rm.DisplayA = "待定"
		rm.DisplayB = "待定"
		if a != nil {
			rm.DisplayA = a.Name
		}
		if b != nil {
			rm.DisplayB = b.Name
		}
		if m.Status == "done" && m.Winner != "" {
			if a != nil && teamUIDStr(a.UserID) == m.Winner {
				w[m.ID] = a
				l[m.ID] = b
			} else {
				w[m.ID] = b
				l[m.ID] = a
			}
		}
		out = append(out, rm)
	}
	return out
}

// 队伍标识：openid 字段统一存 "U<userID>"（bracket 生成的队伍标识）
func teamUIDStr(uid int) string { return "U" + strconv.Itoa(uid) }

func parseTeamUID(s string) int {
	if !strings.HasPrefix(s, "U") {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(s, "U"))
	return n
}

// ---------- 组装比赛详情 ----------

type DraftInfo struct {
	Turn            int              `json:"turn"`
	TotalPicks      int              `json:"totalPicks"`
	Done            bool             `json:"done"`
	Round           int              `json:"round"`
	CapIdx          int              `json:"capIdx"`
	CurrentCaptain  *CaptainInfo     `json:"currentCaptain"`
	CaptainOrder    []CaptainInfo    `json:"captainOrder"`
	Picks           []DraftTeamPicks `json:"picks"`
	RemainingPlayer []Registration   `json:"remainingPlayers"`
}

type DraftTeamPicks struct {
	TeamIndex int           `json:"teamIndex"`
	Captain   *CaptainInfo  `json:"captain"`
	Players   []Registration `json:"players"`
}

func composeTournament(t *Tournament, myUID int, isAdmin, isSuper bool) map[string]interface{} {
	mode := "captain"
	if t.Mode == "auto" {
		mode = "auto"
	}
	captainTotal := t.N
	if mode == "auto" {
		captainTotal = 0
	}
	playerTotal := t.N * t.K
	waitLimit := t.WaitLimit

	captains := []Registration{}
	players := []Registration{}
	waitlist := []Registration{}
	for _, r := range t.Registrations {
		switch r.Role {
		case "captain":
			captains = append(captains, r)
		case "player":
			players = append(players, r)
		case "waitlist":
			waitlist = append(waitlist, r)
		}
	}
	var myReg *Registration
	for i := range t.Registrations {
		if t.Registrations[i].UserID == myUID {
			reg := t.Registrations[i]
			myReg = &reg
			break
		}
	}

	// 选人信息
	var draftInfo *DraftInfo
	if mode == "captain" && t.Draft != nil && (t.Status == "draft" || t.Status == "battle") {
		playerRegMap := map[int]Registration{}
		for _, p := range players {
			playerRegMap[p.UserID] = p
		}
		turn := t.Draft.Turn
		totalPicks := t.N * t.K
		round := turn/t.N + 1
		inRoundIdx := turn % t.N
		capIdx := inRoundIdx
		if round%2 == 0 {
			capIdx = t.N - 1 - inRoundIdx
		}
		var currentCaptain *CaptainInfo
		if turn < totalPicks && len(t.CaptainOrder) > capIdx {
			cc := t.CaptainOrder[capIdx]
			currentCaptain = &cc
		}
		picked := map[int]bool{}
		for _, arr := range t.Draft.Picks {
			for _, p := range arr {
				picked[p] = true
			}
		}
		remaining := []Registration{}
		for _, p := range players {
			if !picked[p.UserID] {
				remaining = append(remaining, p)
			}
		}
		picks := []DraftTeamPicks{}
		for i, arr := range t.Draft.Picks {
			var cap *CaptainInfo
			if i < len(t.CaptainOrder) {
				c := t.CaptainOrder[i]
				cap = &c
			}
			pl := []Registration{}
			for _, pid := range arr {
				if r, ok := playerRegMap[pid]; ok {
					pl = append(pl, r)
				} else {
					pl = append(pl, Registration{UserID: pid, Nickname: "队员"})
				}
			}
			picks = append(picks, DraftTeamPicks{TeamIndex: i, Captain: cap, Players: pl})
		}
		draftInfo = &DraftInfo{
			Turn: turn, TotalPicks: totalPicks, Done: turn >= totalPicks,
			Round: round, CapIdx: capIdx, CurrentCaptain: currentCaptain,
			CaptainOrder: t.CaptainOrder, Picks: picks, RemainingPlayer: remaining,
		}
	}

	// 队伍信息
	teams := t.Teams
	if teams == nil && len(t.CaptainOrder) > 0 && t.Draft != nil {
		teams = []Team{}
		for i, cap := range t.CaptainOrder {
			pl := []TeamMember{}
			if i < len(t.Draft.Picks) {
				for _, pid := range t.Draft.Picks[i] {
					pl = append(pl, TeamMember{UserID: pid, Nickname: "队员"})
				}
			}
			teams = append(teams, Team{Captain: cap, Players: pl})
		}
	}

	secret := ""
	if isAdmin {
		secret = t.Secret
	}

	return map[string]interface{}{
		"id": t.ID, "name": t.Name, "time": t.Time, "mode": mode,
		"N": t.N, "K": t.K, "status": t.Status, "champion": t.Champion,
		"creatorName": t.CreatorName, "secret": secret,
		"myUserID": myUID, "isAdmin": isAdmin, "isSuper": isSuper,
		"captainFilled": len(captains), "captainTotal": captainTotal,
		"playerFilled": len(players), "playerTotal": playerTotal,
		"waitlistFilled": len(waitlist), "waitLimit": waitLimit,
		"registrations": t.Registrations, "myReg": myReg,
		"draftInfo": draftInfo, "teams": teams, "seeding": t.Seeding,
		"matches": resolveBracket(t),
		"createdAt": t.CreatedAt, "archivedAt": t.ArchivedAt,
	}
}

func summarize(t *Tournament, myUID int) map[string]interface{} {
	mode := "captain"
	if t.Mode == "auto" {
		mode = "auto"
	}
	caps := 0
	players := 0
	meReg := false
	for _, r := range t.Registrations {
		switch r.Role {
		case "captain":
			caps++
		case "player":
			players++
		}
		if r.UserID == myUID {
			meReg = true
		}
	}
	captainTotal := t.N
	if mode == "auto" {
		captainTotal = 0
	}
	return map[string]interface{}{
		"id": t.ID, "name": t.Name, "time": t.Time, "mode": mode,
		"N": t.N, "K": t.K, "status": t.Status,
		"captainFilled": caps, "captainTotal": captainTotal,
		"playerFilled": players, "playerTotal": t.N * t.K,
		"champion": t.Champion, "creatorName": t.CreatorName,
		"createdAt": t.CreatedAt, "archivedAt": t.ArchivedAt,
		"meRegistered": meReg,
	}
}

// ---------- handlers ----------

func (a *App) getTournament(id int64) (*Tournament, error) {
	return a.store.Get(id)
}

/* 主页：今天 + 历史 + 我的身份 */
func (a *App) home(w http.ResponseWriter, r *http.Request, uid int) {
	todayStart := queryInt(r, "todayStart", 0)
	user, _ := a.adminClient.GetUser(uid)
	cfg, _ := a.adminClient.Get()
	isAdmin := false
	isSuper := false
	if cfg != nil {
		isAdmin = cfg.SuperAdmin == uid || containsInt(cfg.Admins, uid)
		isSuper = cfg.SuperAdmin == uid
	}
	hasSuper := false
	if cfg != nil {
		hasSuper = cfg.SuperAdmin > 0
	}
	all, err := a.store.ListAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	todayRaw := []map[string]interface{}{}
	olderRaw := []map[string]interface{}{}
	for _, t := range all {
		s := summarize(t, uid)
		// 今日比赛 = 今天创建且未归档（CreatedAt 为 Unix 毫秒，与前端 todayStart 同单位）
		if todayStart > 0 && t.CreatedAt >= todayStart && t.ArchivedAt == nil && len(todayRaw) < 30 {
			todayRaw = append(todayRaw, s)
			continue
		}
		if len(olderRaw) < 50 {
			olderRaw = append(olderRaw, s)
		}
	}
	writeOK(w, map[string]interface{}{
		"user": user, "isAdmin": isAdmin, "isSuper": isSuper, "hasSuper": hasSuper,
		"today": todayRaw, "older": olderRaw,
	})
}

/* 创建比赛（管理员） */
func (a *App) create(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "只有管理员可以创建比赛")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Time      string `json:"time"`
		Secret    string `json:"secret"`
		Mode      string `json:"mode"`
		N         int    `json:"N"`
		K         int    `json:"K"`
		WaitLimit int    `json:"waitLimit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Secret = strings.TrimSpace(req.Secret)
	req.Time = strings.TrimSpace(req.Time)
	mode := "captain"
	if req.Mode == "auto" {
		mode = "auto"
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "请填写比赛名称")
		return
	}
	if req.Secret == "" {
		writeErr(w, http.StatusBadRequest, "请设置比赛密码")
		return
	}
	if len([]rune(req.Secret)) > 20 {
		writeErr(w, http.StatusBadRequest, "比赛密码最多 20 个字")
		return
	}
	if req.N < 2 || req.N > MAX_N {
		writeErr(w, http.StatusBadRequest, "队伍数量需在 2-16 之间")
		return
	}
	if req.K < 1 || req.K > MAX_K {
		writeErr(w, http.StatusBadRequest, "每队人数需在 1-5 之间")
		return
	}
	if req.WaitLimit < 0 || req.WaitLimit > 20 {
		writeErr(w, http.StatusBadRequest, "替补数量需在 0-20 之间")
		return
	}
	me, _ := a.adminClient.GetUser(uid)
	creatorName := ""
	if me != nil {
		creatorName = me.Nickname
	}
	now := a.now()
	t := &Tournament{
		Name: req.Name, Time: req.Time, Secret: req.Secret, Mode: mode,
		N: req.N, K: req.K, WaitLimit: req.WaitLimit,
		Status: "registration", Registrations: []Registration{},
		CaptainOrder: []CaptainInfo{}, Draft: nil, Teams: []Team{},
		Seeding: []int{}, Matches: []*Match{}, Champion: nil,
		CreatedBy: uid, CreatorName: creatorName, CreatedAt: now,
	}
	id, err := a.store.Create(t)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	t.ID = id
	writeOK(w, map[string]interface{}{"id": id})
}

/* 修改报名阶段配置（管理员） */
func (a *App) updateConfig(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "只有管理员可以修改配置")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "只有报名阶段可以修改配置")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Time      string `json:"time"`
		Secret    string `json:"secret"`
		Mode      string `json:"mode"`
		N         int    `json:"N"`
		K         int    `json:"K"`
		WaitLimit int    `json:"waitLimit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Secret = strings.TrimSpace(req.Secret)
	req.Time = strings.TrimSpace(req.Time)
	mode := "captain"
	if req.Mode == "auto" {
		mode = "auto"
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "请填写比赛名称")
		return
	}
	if req.Secret == "" {
		writeErr(w, http.StatusBadRequest, "请设置比赛密码")
		return
	}
	if len([]rune(req.Secret)) > 20 {
		writeErr(w, http.StatusBadRequest, "比赛密码最多 20 个字")
		return
	}
	if req.N < 2 || req.N > MAX_N {
		writeErr(w, http.StatusBadRequest, "队伍数量需在 2-16 之间")
		return
	}
	if req.K < 1 || req.K > MAX_K {
		writeErr(w, http.StatusBadRequest, "每队人数需在 1-5 之间")
		return
	}
	if req.WaitLimit < 0 || req.WaitLimit > 20 {
		writeErr(w, http.StatusBadRequest, "替补数量需在 0-20 之间")
		return
	}
	captains, players, waitlist := 0, 0, 0
	for _, reg := range t.Registrations {
		switch reg.Role {
		case "captain":
			captains++
		case "player":
			players++
		case "waitlist":
			waitlist++
		}
	}
	if mode == "auto" {
		if captains > 0 {
			writeErr(w, http.StatusBadRequest, "已有队长报名，不能切换为系统分组模式（请先移除队长报名）")
			return
		}
		if req.N*req.K < players {
			writeErr(w, http.StatusBadRequest, "队员名额不能小于已报名人数")
			return
		}
	} else {
		if req.N < captains {
			writeErr(w, http.StatusBadRequest, "已有队长报名，队长数不能小于当前报名数")
			return
		}
		if req.N*req.K < players {
			writeErr(w, http.StatusBadRequest, "队员名额不能小于已报名人数")
			return
		}
	}
	if req.WaitLimit < waitlist {
		writeErr(w, http.StatusBadRequest, "替补名额不能小于已报名替补人数")
		return
	}
	t.Name, t.Time, t.Secret, t.Mode, t.N, t.K, t.WaitLimit = req.Name, req.Time, req.Secret, mode, req.N, req.K, req.WaitLimit
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{})
}

/* 比赛密码校验 */
func (a *App) checkSecret(w http.ResponseWriter, r *http.Request, uid int) {
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	var req struct {
		Secret string `json:"secret"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	writeOK(w, map[string]interface{}{"correct": req.Secret == t.Secret})
}

/* 报名（需比赛密码；资料必填） */
func (a *App) register(w http.ResponseWriter, r *http.Request, uid int) {
	if uid <= 0 {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	me, err := a.adminClient.GetUser(uid)
	if err != nil || me == nil {
		writeErr(w, http.StatusUnauthorized, "请先设置昵称")
		return
	}
	if me.Nickname == "" {
		writeErr(w, http.StatusBadRequest, "请先设置昵称")
		return
	}
	if len(me.Roles) == 0 {
		writeErr(w, http.StatusBadRequest, "请先在「我的资料」选择常玩位置后再报名")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "当前不在报名阶段")
		return
	}
	var req struct {
		Role   string `json:"role"`
		Secret string `json:"secret"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Secret != t.Secret {
		writeErr(w, http.StatusBadRequest, "比赛密码错误")
		return
	}
	for _, reg := range t.Registrations {
		if reg.UserID == uid {
			writeErr(w, http.StatusBadRequest, "你已经报名过了")
			return
		}
	}
	captains := []Registration{}
	players := []Registration{}
	waitlist := []Registration{}
	for _, reg := range t.Registrations {
		switch reg.Role {
		case "captain":
			captains = append(captains, reg)
		case "player":
			players = append(players, reg)
		case "waitlist":
			waitlist = append(waitlist, reg)
		}
	}
	role := "player"
	auto := false
	if req.Role == "captain" {
		if t.Mode == "auto" {
			writeErr(w, http.StatusBadRequest, "本场比赛为系统分组模式，请报名队员或替补")
			return
		}
		role = "captain"
		if len(captains) >= t.N {
			writeErr(w, http.StatusBadRequest, "队长名额已满")
			return
		}
	} else if req.Role == "waitlist" {
		role = "waitlist"
		if t.WaitLimit <= 0 {
			writeErr(w, http.StatusBadRequest, "本场比赛未设置替补")
			return
		}
		if len(waitlist) >= t.WaitLimit {
			writeErr(w, http.StatusBadRequest, "替补名额已满")
			return
		}
	} else {
		if len(players) >= t.N*t.K {
			if t.WaitLimit <= 0 {
				writeErr(w, http.StatusBadRequest, "队员名额已满")
				return
			}
			if len(waitlist) >= t.WaitLimit {
				writeErr(w, http.StatusBadRequest, "队员名额已满，替补也已满")
				return
			}
			role = "waitlist"
			auto = true
		}
	}
	reg := Registration{
		UserID: uid, Nickname: me.Nickname, Role: role, TS: a.now(),
		MMR: me.MMR, Roles: me.Roles, Signature: me.Signature,
	}
	if role == "captain" {
		reg.TeamName = ""
	}
	t.Registrations = append(t.Registrations, reg)
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{"role": role, "auto": auto})
}

/* 退报（报名阶段自己取消） */
func (a *App) cancelRegister(w http.ResponseWriter, r *http.Request, uid int) {
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "报名已锁定，请联系管理员")
		return
	}
	before := len(t.Registrations)
	filtered := []Registration{}
	for _, reg := range t.Registrations {
		if reg.UserID != uid {
			filtered = append(filtered, reg)
		}
	}
	if len(filtered) == before {
		writeErr(w, http.StatusBadRequest, "你尚未报名")
		return
	}
	t.Registrations = filtered
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 移除报名（管理员） */
func (a *App) removeRegistration(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	var req struct {
		UserID int `json:"userID"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	before := len(t.Registrations)
	filtered := []Registration{}
	for _, reg := range t.Registrations {
		if reg.UserID != req.UserID {
			filtered = append(filtered, reg)
		}
	}
	if len(filtered) == before {
		writeErr(w, http.StatusNotFound, "未找到该报名")
		return
	}
	t.Registrations = filtered
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 报名阶段队员/替补互转（管理员） */
func (a *App) moveRegistration(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "只有报名阶段可以调整名单")
		return
	}
	var req struct {
		UserID int    `json:"userID"`
		ToRole string `json:"toRole"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	toRole := "player"
	if req.ToRole == "waitlist" {
		toRole = "waitlist"
	}
	idx := -1
	for i := range t.Registrations {
		if t.Registrations[i].UserID == req.UserID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "未找到该报名")
		return
	}
	cur := t.Registrations[idx]
	if cur.Role != "player" && cur.Role != "waitlist" {
		writeErr(w, http.StatusBadRequest, "只能调整队员/替补")
		return
	}
	if cur.Role == toRole {
		writeErr(w, http.StatusBadRequest, "该用户已在目标名单中")
		return
	}
	players, waitlist := 0, 0
	for _, reg := range t.Registrations {
		if reg.Role == "player" {
			players++
		} else if reg.Role == "waitlist" {
			waitlist++
		}
	}
	if toRole == "player" {
		if players >= t.N*t.K {
			writeErr(w, http.StatusBadRequest, "队员名额已满，无法移入")
			return
		}
	} else {
		if t.WaitLimit <= 0 {
			writeErr(w, http.StatusBadRequest, "本场比赛未设置替补")
			return
		}
		if waitlist >= t.WaitLimit {
			writeErr(w, http.StatusBadRequest, "替补名额已满")
			return
		}
	}
	t.Registrations[idx].Role = toRole
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 设置队伍名称（队长本人/组长本人或管理员） */
/* 预设队伍图标（Dota2 相关 + 趣味图标，前后端一致） */
var teamIcons = []string{"⚔️", "🛡️", "🐉", "🐺", "🦇", "🔥", "❄️", "⚡", "💀", "👑", "🏹", "🗡️", "🪓", "🐍", "☠️", "👹", "🧙", "🧝"}

func inTeamIcons(s string) bool {
	for _, i := range teamIcons {
		if i == s {
			return true
		}
	}
	return false
}

func (a *App) setTeamName(w http.ResponseWriter, r *http.Request, uid int) {
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	var req struct {
		UserID   int    `json:"userID"`
		TeamName string `json:"teamName"`
		TeamIcon string `json:"teamIcon"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	teamName := strings.TrimSpace(req.TeamName)
	if len([]rune(teamName)) > 10 {
		writeErr(w, http.StatusBadRequest, "队伍名称最多 10 个字")
		return
	}
	teamIcon := strings.TrimSpace(req.TeamIcon)
	if teamIcon != "" && !inTeamIcons(teamIcon) {
		writeErr(w, http.StatusBadRequest, "图标不在支持范围内")
		return
	}
	targetUID := req.UserID
	if targetUID == 0 {
		targetUID = uid
	}
	isAdminFlag := a.adminClient.IsAdmin(uid)
	if targetUID != uid && !isAdminFlag {
		writeErr(w, http.StatusForbidden, "只有队长本人或管理员可以设置队伍名称")
		return
	}
	if t.Mode == "auto" {
		if t.Status != "draft" && t.Status != "battle" {
			writeErr(w, http.StatusBadRequest, "只有分组完成后可以设置队伍名称")
			return
		}
		found := false
		for i := range t.Teams {
			if t.Teams[i].Captain.UserID == targetUID {
				t.Teams[i].Captain.TeamName = teamName
				t.Teams[i].Captain.TeamIcon = teamIcon
				found = true
				break
			}
		}
		if !found {
			writeErr(w, http.StatusBadRequest, "只能由组长（本队天梯分最高者）或管理员设置队伍名称")
			return
		}
	} else {
		if t.Status != "registration" && t.Status != "draft" && t.Status != "battle" {
			writeErr(w, http.StatusBadRequest, "只有报名或选人阶段可以设置队伍名称")
			return
		}
		found := false
		for i := range t.Registrations {
			if t.Registrations[i].UserID == targetUID && t.Registrations[i].Role == "captain" {
				t.Registrations[i].TeamName = teamName
				t.Registrations[i].TeamIcon = teamIcon
				found = true
				break
			}
		}
		for i := range t.CaptainOrder {
			if t.CaptainOrder[i].UserID == targetUID {
				t.CaptainOrder[i].TeamName = teamName
				t.CaptainOrder[i].TeamIcon = teamIcon
				break
			}
		}
		for i := range t.Teams {
			if t.Teams[i].Captain.UserID == targetUID {
				t.Teams[i].Captain.TeamName = teamName
				t.Teams[i].Captain.TeamIcon = teamIcon
				break
			}
		}
		if !found {
			writeErr(w, http.StatusBadRequest, "只能为已报名的队长设置队伍名称")
			return
		}
	}
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 结束并归档（管理员）：要求已有冠军 */
func (a *App) finishTournament(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.ArchivedAt != nil {
		writeErr(w, http.StatusBadRequest, "比赛已归档")
		return
	}
	if t.Champion == nil {
		writeErr(w, http.StatusBadRequest, "比赛还未结束（未产生冠军），不能归档")
		return
	}
	now := a.now()
	t.Status = "finished"
	t.ArchivedAt = &now
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 删除比赛（管理员） */
func (a *App) deleteTournament(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if _, err := a.getTournament(id); err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	a.lock(id).Lock()
	defer a.lock(id).Unlock()
	if err := a.store.Delete(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 比赛详情 */
func (a *App) tournament(w http.ResponseWriter, r *http.Request, uid int) {
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	isAdmin := a.adminClient.IsAdmin(uid)
	isSuper := a.adminClient.IsSuper(uid)
	writeOK(w, composeTournament(t, uid, isAdmin, isSuper))
}

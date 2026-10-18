package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ---------- 选人 ----------

/* 开始选人（管理员）：随机排序队长，初始化蛇形轮次 */
func (a *App) startDraft(w http.ResponseWriter, r *http.Request, uid int) {
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
		writeErr(w, http.StatusBadRequest, "当前不在报名阶段")
		return
	}
	captains := []Registration{}
	players := []Registration{}
	for _, reg := range t.Registrations {
		switch reg.Role {
		case "captain":
			captains = append(captains, reg)
		case "player":
			players = append(players, reg)
		}
	}
	if len(captains) < t.N {
		writeErr(w, http.StatusBadRequest, "队长尚未报满（"+strconv.Itoa(len(captains))+"/"+strconv.Itoa(t.N)+"）")
		return
	}
	if len(players) < t.N*t.K {
		writeErr(w, http.StatusBadRequest, "队员尚未报满（"+strconv.Itoa(len(players))+"/"+strconv.Itoa(t.N*t.K)+"）")
		return
	}
	order := []CaptainInfo{}
	for _, c := range shuffleRegs(captains) {
		order = append(order, CaptainInfo{
			UserID: c.UserID, Nickname: c.Nickname, TeamName: c.TeamName, TeamIcon: c.TeamIcon,
			MMR: c.MMR, Roles: c.Roles, Signature: c.Signature,
		})
	}
	picks := make([][]int, len(order))
	t.Status = "draft"
	t.CaptainOrder = order
	t.Draft = &Draft{Turn: 0, Picks: picks}
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 队长选人 / 管理员代选 */
func (a *App) pickPlayer(w http.ResponseWriter, r *http.Request, uid int) {
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	a.lock(id).Lock()
	defer a.lock(id).Unlock()

	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "当前不在选人阶段")
		return
	}
	var req struct {
		PlayerUserID int `json:"playerUserID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlayerUserID == 0 {
		writeErr(w, http.StatusBadRequest, "缺少被选队员")
		return
	}
	isPlayer := false
	for _, reg := range t.Registrations {
		if reg.UserID == req.PlayerUserID && reg.Role == "player" {
			isPlayer = true
			break
		}
	}
	if !isPlayer {
		writeErr(w, http.StatusBadRequest, "该用户不是本次比赛队员")
		return
	}
	for _, arr := range t.Draft.Picks {
		for _, p := range arr {
			if p == req.PlayerUserID {
				writeErr(w, http.StatusBadRequest, "该队员已被选择")
				return
			}
		}
	}
	turn := t.Draft.Turn
	totalPicks := t.N * t.K
	if turn >= totalPicks {
		writeErr(w, http.StatusBadRequest, "选人已经完成")
		return
	}
	round := turn/t.N + 1
	inRoundIdx := turn % t.N
	capIdx := inRoundIdx
	if round%2 == 0 {
		capIdx = t.N - 1 - inRoundIdx
	}
	if capIdx >= len(t.CaptainOrder) {
		writeErr(w, http.StatusBadRequest, "队长信息缺失，请刷新")
		return
	}
	currentCaptain := t.CaptainOrder[capIdx]
	isAdminFlag := a.adminClient.IsAdmin(uid)
	if uid != currentCaptain.UserID && !isAdminFlag {
		writeErr(w, http.StatusForbidden, "还没轮到你选人（当前是"+currentCaptain.Nickname+"）")
		return
	}
	t.Draft.Picks[capIdx] = append(t.Draft.Picks[capIdx], req.PlayerUserID)
	t.Draft.Turn = turn + 1
	done := turn+1 >= totalPicks
	t.UpdatedAt = a.now()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{"picked": req.PlayerUserID, "teamIndex": capIdx, "done": done})
}

/* 重置选人（管理员） */
func (a *App) resetDraft(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "当前不在选人阶段")
		return
	}
	captains := []Registration{}
	for _, reg := range t.Registrations {
		if reg.Role == "captain" {
			captains = append(captains, reg)
		}
	}
	order := []CaptainInfo{}
	for _, c := range shuffleRegs(captains) {
		order = append(order, CaptainInfo{
			UserID: c.UserID, Nickname: c.Nickname, TeamName: c.TeamName, TeamIcon: c.TeamIcon,
			MMR: c.MMR, Roles: c.Roles, Signature: c.Signature,
		})
	}
	t.CaptainOrder = order
	t.Draft = &Draft{Turn: 0, Picks: make([][]int, len(order))}
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 调整胜者组第一轮对阵落位（管理员） */
func (a *App) setSeeding(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "当前不在生成对阵阶段")
		return
	}
	if t.Mode != "auto" {
		totalPicks := t.N * t.K
		if t.Draft == nil || t.Draft.Turn < totalPicks {
			writeErr(w, http.StatusBadRequest, "选人还未完成")
			return
		}
	}
	var req struct {
		Seeding []int `json:"seeding"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.Seeding) != t.N {
		writeErr(w, http.StatusBadRequest, "对阵落位数据不正确")
		return
	}
	seen := map[int]bool{}
	for _, v := range req.Seeding {
		if v < 0 || v >= t.N || seen[v] {
			writeErr(w, http.StatusBadRequest, "对阵落位数据不正确")
			return
		}
		seen[v] = true
	}
	t.Seeding = req.Seeding
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 随机对阵落位（管理员） */
func (a *App) shuffleSeeding(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "当前不在生成对阵阶段")
		return
	}
	base := make([]int, t.N)
	for i := range base {
		base[i] = i
	}
	t.Seeding = shuffleInts(base)
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 生成对阵、进入对战阶段（管理员） */
func (a *App) startBracket(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "当前不在生成对阵阶段")
		return
	}
	var teams []Team
	if t.Mode == "auto" {
		teams = t.Teams
		if len(teams) == 0 {
			writeErr(w, http.StatusBadRequest, "队伍尚未生成（请先完成系统分组）")
			return
		}
	} else {
		totalPicks := t.N * t.K
		if t.Draft == nil || t.Draft.Turn < totalPicks {
			writeErr(w, http.StatusBadRequest, "选人还未完成")
			return
		}
		playerRegMap := map[int]Registration{}
		for _, reg := range t.Registrations {
			if reg.Role == "player" {
				playerRegMap[reg.UserID] = reg
			}
		}
		teams = []Team{}
		for i, cap := range t.CaptainOrder {
			pl := []TeamMember{}
			if i < len(t.Draft.Picks) {
				for _, p := range t.Draft.Picks[i] {
					if reg, ok := playerRegMap[p]; ok {
						pl = append(pl, TeamMember{UserID: reg.UserID, Nickname: reg.Nickname, MMR: reg.MMR, Roles: reg.Roles, Signature: reg.Signature})
					} else {
						pl = append(pl, TeamMember{UserID: p, Nickname: "队员"})
					}
				}
			}
			teams = append(teams, Team{Captain: cap, Players: pl})
		}
	}
	seeding := t.Seeding
	if len(seeding) != t.N {
		seeding = shuffleInts(makeSeq(t.N))
	}
	ordered := make([]Team, 0, t.N)
	for _, idx := range seeding {
		ordered = append(ordered, teams[idx])
	}
	bracketTeams := []BracketTeam{}
	for _, team := range ordered {
		name := strings.TrimSpace(team.Captain.TeamName)
		if name == "" {
			name = team.Captain.Nickname + "战队"
		}
		bracketTeams = append(bracketTeams, BracketTeam{Openid: teamUIDStr(team.Captain.UserID), Name: name, CaptainOpenid: team.Captain.UserID})
	}
	matches, err := BuildBracket(bracketTeams)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "生成对阵失败："+err.Error())
		return
	}
	t.Status = "battle"
	t.Teams = teams
	t.Seeding = seeding
	t.Matches = matches
	t.Champion = nil
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

func makeSeq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

/* 配置比赛赛制 BO（管理员） */
func (a *App) setMatchBo(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Status != "battle" {
		writeErr(w, http.StatusBadRequest, "当前不在对战阶段")
		return
	}
	if t.ArchivedAt != nil {
		writeErr(w, http.StatusBadRequest, "比赛已归档，不能修改赛制")
		return
	}
	var req struct {
		MatchID int    `json:"matchId"`
		Bo      string `json:"bo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	bo := ""
	if req.Bo == "BO1" || req.Bo == "BO3" || req.Bo == "BO5" {
		bo = req.Bo
	}
	if bo == "" {
		writeErr(w, http.StatusBadRequest, "赛制只能是 BO1 / BO3 / BO5")
		return
	}
	idx := -1
	for i, m := range t.Matches {
		if m.ID == req.MatchID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Matches[idx].Status == "done" {
		writeErr(w, http.StatusBadRequest, "比赛已结束，不能修改赛制")
		return
	}
	t.Matches[idx].Bo = bo
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 记录比赛结果（管理员）：自动晋级/淘汰/产生冠军 */
func (a *App) setMatchResult(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	a.lock(id).Lock()
	defer a.lock(id).Unlock()

	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Status != "battle" {
		writeErr(w, http.StatusBadRequest, "当前不在对战阶段")
		return
	}
	if t.ArchivedAt != nil {
		writeErr(w, http.StatusBadRequest, "比赛已归档，不能修改结果")
		return
	}
	var req struct {
		MatchID int    `json:"matchId"`
		Winner  string `json:"winner"`
		ScoreA  *int   `json:"scoreA"`
		ScoreB  *int   `json:"scoreB"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	idx := -1
	for i, m := range t.Matches {
		if m.ID == req.MatchID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	m := t.Matches[idx]
	if m.Status == "done" {
		writeErr(w, http.StatusBadRequest, "该场已有结果，请先修正再重记")
		return
	}
	resolved := resolveBracket(t)
	if idx >= len(resolved) || resolved[idx].TeamAInfo == nil || resolved[idx].TeamBInfo == nil {
		writeErr(w, http.StatusBadRequest, "本场对阵尚未完整（有待定队伍）")
		return
	}
	rm := resolved[idx]
	winnerA := rm.TeamAInfo.UserID == parseTeamUID(req.Winner) || teamUIDStr(rm.TeamAInfo.UserID) == req.Winner
	winnerB := rm.TeamBInfo.UserID == parseTeamUID(req.Winner) || teamUIDStr(rm.TeamBInfo.UserID) == req.Winner
	if !winnerA && !winnerB {
		writeErr(w, http.StatusBadRequest, "胜方必须是本场比赛的两支队伍之一")
		return
	}
	winnerIsA := winnerA
	scoreA := req.ScoreA
	scoreB := req.ScoreB
	if m.Bo == "BO1" {
		one, zero := 1, 0
		if winnerIsA {
			scoreA, scoreB = &one, &zero
		} else {
			scoreA, scoreB = &zero, &one
		}
	} else {
		winScore := 2
		if m.Bo == "BO5" {
			winScore = 3
		}
		if scoreA == nil || scoreB == nil {
			s1, s0 := winScore, 0
			if winnerIsA {
				scoreA, scoreB = &s1, &s0
			} else {
				scoreA, scoreB = &s0, &s1
			}
		} else {
			if *scoreA < 0 || *scoreB < 0 || *scoreA > winScore || *scoreB > winScore {
				writeErr(w, http.StatusBadRequest, "比分不合法（单方最高 "+strconv.Itoa(winScore)+"）")
				return
			}
			if *scoreA == *scoreB {
				writeErr(w, http.StatusBadRequest, "比分不能打平")
				return
			}
			if (*scoreA > *scoreB) != winnerIsA {
				writeErr(w, http.StatusBadRequest, "比分与胜方不一致")
				return
			}
		}
	}
	m.Status = "done"
	m.Winner = req.Winner
	m.ScoreA = scoreA
	m.ScoreB = scoreB

	winTeam := rm.TeamAInfo
	if !winnerIsA {
		winTeam = rm.TeamBInfo
	}
	status := t.Status
	champion := t.Champion
	if m.IsChampionMatch {
		champion = &ChampionInfo{UserID: winTeam.UserID, Name: winTeam.Name}
		status = "finished"
	}
	t.Champion = champion
	t.Status = status
	t.UpdatedAt = a.now()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, map[string]interface{}{
		"champion": nilString(champion), "finished": status == "finished",
	})
}

func nilString(c *ChampionInfo) interface{} {
	if c == nil {
		return nil
	}
	return c.Name
}

/* 修正比赛结果（管理员）：级联重置受影响的后续比赛 */
func (a *App) resetMatchResult(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "需要管理员权限")
		return
	}
	id, ok := a.pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	a.lock(id).Lock()
	defer a.lock(id).Unlock()

	t, err := a.getTournament(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.ArchivedAt != nil {
		writeErr(w, http.StatusBadRequest, "比赛已归档，不能修正结果")
		return
	}
	var req struct {
		MatchID int `json:"matchId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	idx := -1
	for i, m := range t.Matches {
		if m.ID == req.MatchID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeErr(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if t.Matches[idx].Status != "done" {
		writeErr(w, http.StatusBadRequest, "该场比赛还没有结果")
		return
	}
	// BFS 收集受影响场次
	resetIDs := map[int]bool{}
	queue := []int{req.MatchID}
	for len(queue) > 0 {
		curID := queue[0]
		queue = queue[1:]
		if resetIDs[curID] {
			continue
		}
		resetIDs[curID] = true
		var cur *Match
		for _, m := range t.Matches {
			if m.ID == curID {
				cur = m
				break
			}
		}
		if cur != nil && cur.Status == "done" {
			if cur.Next != nil {
				queue = append(queue, cur.Next.MatchID)
			}
			if cur.LoserNext != nil {
				queue = append(queue, cur.LoserNext.MatchID)
			}
		}
	}
	for _, m := range t.Matches {
		if resetIDs[m.ID] {
			m.Status = "pending"
			m.Winner = ""
			m.ScoreA = nil
			m.ScoreB = nil
		}
	}
	status := t.Status
	if t.Champion != nil {
		for _, m := range t.Matches {
			if resetIDs[m.ID] && m.IsChampionMatch {
				status = "battle"
				break
			}
		}
	}
	if status == "battle" {
		t.Champion = nil
		t.ArchivedAt = nil
	}
	t.Status = status
	t.UpdatedAt = a.now()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

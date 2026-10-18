package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
)

// ---------- 系统分组算法（对齐云函数 runAutoGroup） ----------

type GroupOut struct {
	Idx         int          `json:"idx"`
	Captain     *Registration `json:"captain"`
	AvgMmr      int          `json:"avgMmr"`
	MissingRoles []int        `json:"missingRoles"`
	Members     []Registration `json:"members"`
}

type AutoGroupResult struct {
	Groups    []GroupOut `json:"groups"`
	MaxDiff   int        `json:"maxDiff"`
	MaxDiffOk bool       `json:"maxDiffOk"`
}

func runAutoGroup(players []Registration, N, K int) AutoGroupResult {
	has := func(p Registration, r int) bool {
		for _, v := range p.Roles {
			if v == r {
				return true
			}
		}
		return false
	}
	missingRoles := func(g []Registration) []int {
		hasRole := map[int]bool{}
		for _, p := range g {
			for _, r := range p.Roles {
				hasRole[r] = true
			}
		}
		m := []int{}
		for r := 1; r <= 5; r++ {
			if !hasRole[r] {
				m = append(m, r)
			}
		}
		return m
	}
	avg := func(g []Registration) float64 {
		if len(g) == 0 {
			return 0
		}
		sum := 0
		for _, p := range g {
			sum += p.MMR
		}
		return float64(sum) / float64(len(g))
	}

	groups := make([][]Registration, N)
	pool := make([]Registration, len(players))
	copy(pool, players)

	// 1. 位置代表：每组依次拿 1-5 号位人选（mmr 与组均值最接近者）
	for r := 1; r <= 5; r++ {
		for gi := 0; gi < N; gi++ {
			if len(groups[gi]) >= K {
				continue
			}
			cand := []Registration{}
			for _, p := range pool {
				if has(p, r) {
					cand = append(cand, p)
				}
			}
			if len(cand) == 0 {
				continue
			}
			curAvg := avg(groups[gi])
			pickIdx := 0
			bestD := 1e18
			for i, p := range cand {
				d := float64(p.MMR) - curAvg
				if d < 0 {
					d = -d
				}
				if d < bestD {
					bestD = d
					pickIdx = i
				}
			}
			pick := cand[pickIdx]
			groups[gi] = append(groups[gi], pick)
			for i, p := range pool {
				if p.UserID == pick.UserID {
					pool = append(pool[:i], pool[i+1:]...)
					break
				}
			}
		}
	}

	// 2. 补位：每组补到 K 人，优先补该组缺失位置的人
	for gi := 0; gi < N; gi++ {
		for len(groups[gi]) < K && len(pool) > 0 {
			miss := missingRoles(groups[gi])
			pick := (*Registration)(nil)
			for _, r := range miss {
				for i, p := range pool {
					if has(p, r) {
						pc := pool[i]
						pick = &pc
						break
					}
				}
				if pick != nil {
					break
				}
			}
			if pick == nil {
				pc := pool[0]
				pick = &pc
			}
			groups[gi] = append(groups[gi], *pick)
			for i, p := range pool {
				if p.UserID == pick.UserID {
					pool = append(pool[:i], pool[i+1:]...)
					break
				}
			}
		}
	}

	// 3. 爬山优化：随机交换，缺位更少优先、同缺位时均分差更小才接受
	score := func() (int, float64) {
		maxD := 0.0
		missN := 0
		for _, g := range groups {
			av := avg(g)
			for _, g2 := range groups {
				d := avg(g2) - av
				if d < 0 {
					d = -d
				}
				if d > maxD {
					maxD = d
				}
			}
			missN += len(missingRoles(g))
		}
		return missN, maxD
	}
	bestMissN, bestMaxD := score()
	best := make([][]Registration, N)
	for i, g := range groups {
		best[i] = append([]Registration{}, g...)
	}
	for iter := 0; iter < 4000; iter++ {
		gi := rand.Intn(N)
		gj := rand.Intn(N)
		if gj == gi {
			gj = (gj + 1) % N
		}
		ai := rand.Intn(len(groups[gi]))
		bj := rand.Intn(len(groups[gj]))
		groups[gi][ai], groups[gj][bj] = groups[gj][bj], groups[gi][ai]
		missN, maxD := score()
		if missN < bestMissN || (missN == bestMissN && maxD < bestMaxD) {
			bestMissN, bestMaxD = missN, maxD
			for i, g := range groups {
				best[i] = append([]Registration{}, g...)
			}
		} else {
			groups[gi][ai], groups[gj][bj] = groups[gj][bj], groups[gi][ai]
		}
	}
	groups = best

	// 4. 组装输出：每组按 mmr 降序，组长 = 最高者
	out := []GroupOut{}
	maxDiff := 0.0
	for gi := 0; gi < N; gi++ {
		g := append([]Registration{}, groups[gi]...)
		sort.Slice(g, func(a, b int) bool { return g[a].MMR > g[b].MMR })
		var cap *Registration
		if len(g) > 0 {
			c := g[0]
			cap = &c
		}
		out = append(out, GroupOut{
			Idx: gi, Captain: cap, AvgMmr: int(avg(g) + 0.5),
			MissingRoles: missingRoles(g), Members: g,
		})
	}
	for _, g := range groups {
		av := avg(g)
		for _, g2 := range groups {
			d := avg(g2) - av
			if d < 0 {
				d = -d
			}
			if d > maxDiff {
				maxDiff = d
			}
		}
	}
	return AutoGroupResult{Groups: out, MaxDiff: int(maxDiff + 0.5), MaxDiffOk: maxDiff <= 300}
}

// ---------- handlers ----------

/* 系统分组：预览（不落库） */
func (a *App) autoGroup(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Mode != "auto" {
		writeErr(w, http.StatusBadRequest, "本场比赛不是系统分组模式")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "当前不在报名阶段")
		return
	}
	players := []Registration{}
	for _, reg := range t.Registrations {
		if reg.Role == "player" {
			players = append(players, reg)
		}
	}
	need := t.N * t.K
	if len(players) < need {
		writeErr(w, http.StatusBadRequest, "队员尚未报满（"+itoa(len(players))+"/"+itoa(need)+"），报满后才能分组")
		return
	}
	writeOK(w, runAutoGroup(players, t.N, t.K))
}

/* 系统分组：确认并写入队伍（服务端重算，不信任客户端） */
func (a *App) confirmGroup(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Mode != "auto" {
		writeErr(w, http.StatusBadRequest, "本场比赛不是系统分组模式")
		return
	}
	if t.Status != "registration" {
		writeErr(w, http.StatusBadRequest, "当前不在报名阶段")
		return
	}
	players := []Registration{}
	playerByID := map[int]Registration{}
	for _, reg := range t.Registrations {
		if reg.Role == "player" {
			players = append(players, reg)
			playerByID[reg.UserID] = reg
		}
	}
	need := t.N * t.K
	if len(players) < need {
		writeErr(w, http.StatusBadRequest, "队员尚未报满，报满后才能分组")
		return
	}
	var req struct {
		Groups [][]int `json:"groups"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var groups [][]Registration
	if len(req.Groups) > 0 {
		if len(req.Groups) != t.N {
			writeErr(w, http.StatusBadRequest, "分组数据不正确（组数不符）")
			return
		}
		seen := map[int]bool{}
		total := 0
		for gi, arr := range req.Groups {
			if len(arr) != t.K {
				writeErr(w, http.StatusBadRequest, "第 "+itoa(gi+1)+" 组人数不正确（需恰好 "+itoa(t.K)+" 人）")
				return
			}
			grp := []Registration{}
			for _, oid := range arr {
				if seen[oid] {
					writeErr(w, http.StatusBadRequest, "分组数据不正确（重复队员）")
					return
				}
				p, exists := playerByID[oid]
				if !exists {
					writeErr(w, http.StatusBadRequest, "分组数据不正确（含非报名队员）")
					return
				}
				seen[oid] = true
				total++
				grp = append(grp, p)
			}
			sort.Slice(grp, func(a, b int) bool { return grp[a].MMR > grp[b].MMR })
			groups = append(groups, grp)
		}
		if total != len(players) {
			writeErr(w, http.StatusBadRequest, "还有队员未分组")
			return
		}
	} else {
		res := runAutoGroup(players, t.N, t.K)
		for _, g := range res.Groups {
			groups = append(groups, g.Members)
		}
	}
	teams := []Team{}
	for _, members := range groups {
		captain := members[0]
		pl := []TeamMember{}
		for _, m := range members[1:] {
			pl = append(pl, TeamMember{UserID: m.UserID, Nickname: m.Nickname, MMR: m.MMR, Roles: m.Roles, Signature: m.Signature})
		}
		teams = append(teams, Team{
			Captain: CaptainInfo{UserID: captain.UserID, Nickname: captain.Nickname, TeamName: "", MMR: captain.MMR, Roles: captain.Roles, Signature: captain.Signature},
			Players: pl,
		})
	}
	t.Status = "draft"
	t.Teams = teams
	t.CaptainOrder = []CaptainInfo{}
	t.Draft = nil
	t.Seeding = []int{}
	t.UpdatedAt = a.now()
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	writeOK(w, true)
}

/* 系统分组：返回报名阶段重新分组（管理员，未生成对阵前） */
func (a *App) resetGroup(w http.ResponseWriter, r *http.Request, uid int) {
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
	if t.Mode != "auto" {
		writeErr(w, http.StatusBadRequest, "仅系统分组模式支持重新分组")
		return
	}
	if t.Status != "draft" {
		writeErr(w, http.StatusBadRequest, "只有分组完成后（未生成对阵前）可以重新分组")
		return
	}
	t.Status = "registration"
	t.Teams = []Team{}
	t.Seeding = []int{}
	t.Matches = []*Match{}
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

func itoa(n int) string { return strconv.Itoa(n) }

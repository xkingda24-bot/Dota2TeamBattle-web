package main

import (
	"math/rand"
	"net/http"
	"strconv"
)

var vCaps = []struct {
	Nickname  string
	MMR       int
	Roles     []int
	Signature string
}{
	{"天府一号", 8800, []int{1}, "梦想是打TI"},
	{"剑气纵横", 8300, []int{2}, "中单霸主"},
	{"顽皮虎", 7900, []int{3}, "一号位大腿"},
	{"竹林听风", 7500, []int{4}, "四号位游走"},
	{"清欢渡", 7100, []int{5}, "五号位守护神"},
	{"星夜骑士", 6800, []int{1, 3}, "手残党慎选"},
	{"雾都游侠", 6500, []int{2, 5}, "主打一个陪伴"},
}

var vNames = []string{
	"幽鬼之刃", "虚空假面", "蓝猫淘气", "白牛冲锋", "神谕者", "潮汐猎人", "影魔登场", "屠夫钩子",
	"赏金猎人", "冰女宝宝", "火女莉娜", "风行天下", "白虎月神", "猴子猴孙", "剑圣无敌", "矮人火枪",
	"双头食人", "沙王地刺", "巫妖亡语", "术士乱斗", "先知树语", "撼地神牛", "主宰无敌", "昆卡船长",
	"巨魔战将", "刚背兽王", "谜团黑洞", "宙斯雷霆", "美杜莎之眼", "幻影长矛", "敌法师之怒", "痛苦女王",
}

var vSigs = []string{"稳住别浪", "认真打每一把", "求个大腿带", "菜鸡互啄", "一波推平", "五号位保人", "手残但努力", "主打一个快乐"}

func pickRoleSet() []int {
	n := 1 + rand.Intn(3)
	pool := []int{1, 2, 3, 4, 5}
	for i := len(pool) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		pool[i], pool[j] = pool[j], pool[i]
	}
	out := pool[:n]
	sortInts(out)
	return out
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func pickStr(arr []string) string { return arr[rand.Intn(len(arr))] }

func vName(i int) string {
	idx := i % len(vNames)
	if i < len(vNames) {
		return vNames[idx]
	}
	return vNames[idx] + "·" + strconv.Itoa(i/len(vNames)+1)
}

func mkVRoles(base int) []int {
	extra := pickRoleSet()
	out := []int{base}
	for _, r := range extra {
		if r != base {
			out = append(out, r)
			break
		}
	}
	sortInts(out)
	return out
}

/* 生成虚拟测试用户并报名（管理员；用于自测完整流程） */
func (a *App) seedTest(w http.ResponseWriter, r *http.Request, uid int) {
	if !a.adminClient.IsAdmin(uid) {
		writeErr(w, http.StatusForbidden, "仅管理员可生成测试数据")
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
		writeErr(w, http.StatusBadRequest, "当前不在报名阶段（测试数据只能在报名阶段注入）")
		return
	}
	now := a.now()

	if t.Mode == "auto" {
		need := t.N * t.K
		waitLimit := t.WaitLimit
		curPlayers, curWait := 0, 0
		existing := map[int]bool{}
		for _, reg := range t.Registrations {
			existing[reg.UserID] = true
			if reg.Role == "player" {
				curPlayers++
			} else if reg.Role == "waitlist" {
				curWait++
			}
		}
		toAdd := []Registration{}
		if curPlayers < need {
			for i := 0; i < need-curPlayers; i++ {
				base := i%5 + 1
				reg := Registration{
					UserID: -(100 + i), Nickname: vName(i), Role: "player", TS: now,
					MMR: 4000 + rand.Intn(5000), Roles: mkVRoles(base), Signature: pickStr(vSigs),
				}
				if !existing[reg.UserID] {
					toAdd = append(toAdd, reg)
				}
			}
		}
		if waitLimit > 0 && curWait < waitLimit {
			wc := waitLimit - curWait
			if wc > 5 {
				wc = 5
			}
			for i := 0; i < wc; i++ {
				base := (i+17)%5 + 1
				reg := Registration{
					UserID: -(200 + i), Nickname: vName(need + i), Role: "waitlist", TS: now,
					MMR: 3500 + rand.Intn(4500), Roles: mkVRoles(base), Signature: pickStr(vSigs),
				}
				if !existing[reg.UserID] {
					toAdd = append(toAdd, reg)
				}
			}
		}
		if len(toAdd) == 0 {
			writeErr(w, http.StatusBadRequest, "队员与替补均已满，无需注入虚拟用户")
			return
		}
		t.Registrations = append(t.Registrations, toAdd...)
		t.UpdatedAt = now
		a.lock(t.ID).Lock()
		defer a.lock(t.ID).Unlock()
		if err := a.store.Save(t); err != nil {
			writeErr(w, http.StatusInternalServerError, "服务异常")
			return
		}
		playerFilled, waitFilled := 0, 0
		for _, reg := range t.Registrations {
			if reg.Role == "player" {
				playerFilled++
			} else if reg.Role == "waitlist" {
				waitFilled++
			}
		}
		writeOK(w, map[string]interface{}{
			"added": len(toAdd), "playerFilled": playerFilled, "playerTotal": need,
			"waitlistFilled": waitFilled, "waitLimit": waitLimit, "auto": true,
		})
		return
	}

	// 队长选人模式
	existing := map[int]bool{}
	for _, reg := range t.Registrations {
		existing[reg.UserID] = true
	}
	toAdd := []Registration{}
	for i, c := range vCaps {
		reg := Registration{
			UserID: -(10 + i), Nickname: c.Nickname, Role: "captain", TS: now,
			MMR: c.MMR, Roles: c.Roles, Signature: c.Signature, TeamName: "",
		}
		if !existing[reg.UserID] {
			toAdd = append(toAdd, reg)
		}
	}
	for i := 0; i < len(vNames); i++ {
		reg := Registration{
			UserID: -(100 + i), Nickname: vNames[i], Role: "player", TS: now,
			MMR: 4000 + rand.Intn(5000), Roles: pickRoleSet(), Signature: pickStr(vSigs),
		}
		if !existing[reg.UserID] {
			toAdd = append(toAdd, reg)
		}
	}
	if len(toAdd) == 0 {
		writeErr(w, http.StatusBadRequest, "虚拟用户已全部报名过了（如需重新注入，请先删除本场比赛再重建）")
		return
	}
	t.Registrations = append(t.Registrations, toAdd...)
	t.UpdatedAt = now
	a.lock(t.ID).Lock()
	defer a.lock(t.ID).Unlock()
	if err := a.store.Save(t); err != nil {
		writeErr(w, http.StatusInternalServerError, "服务异常")
		return
	}
	captainFilled, playerFilled := 0, 0
	for _, reg := range t.Registrations {
		if reg.Role == "captain" {
			captainFilled++
		} else if reg.Role == "player" {
			playerFilled++
		}
	}
	writeOK(w, map[string]interface{}{
		"added": len(toAdd), "captainFilled": captainFilled, "playerFilled": playerFilled,
	})
}

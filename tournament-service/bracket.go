package main

import (
	"fmt"
)

// TeamSource 表示一场比赛的队伍来源：种子队伍或上游比赛结果
type TeamSource struct {
	Type    string `json:"type"` // "team" | "match"
	Openid  string `json:"openid,omitempty"`
	Name    string `json:"name,omitempty"`
	MatchID int    `json:"matchId,omitempty"`
	Slot    string `json:"slot,omitempty"` // "winner" | "loser"
}

// NextLink 指向下游比赛
type NextLink struct {
	MatchID int    `json:"matchId"`
	Slot    string `json:"slot"` // "A" | "B"
}

// Match 一场比赛（TI 双败赛制）
type Match struct {
	ID              int         `json:"id"`
	Stage           string      `json:"stage"` // UB1/UB2.../UBF/LB1.../LBF/GF
	Label           string      `json:"label"`
	TeamA           *TeamSource `json:"teamA"`
	TeamB           *TeamSource `json:"teamB"`
	Bo              string      `json:"bo"`
	Status          string      `json:"status"` // pending | done
	Winner          string      `json:"winner"`
	ScoreA          *int        `json:"scoreA"`
	ScoreB          *int        `json:"scoreB"`
	Next            *NextLink   `json:"next"`
	LoserNext       *NextLink   `json:"loserNext"`
	IsChampionMatch bool        `json:"isChampionMatch"`
}

// BracketTeam 种子队伍
type BracketTeam struct {
	Openid        string `json:"openid"`
	Name          string `json:"name"`
	CaptainOpenid int    `json:"captainOpenid,omitempty"`
}

var cnNum = [...]string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二", "十三", "十四", "十五", "十六"}

func cn(n int) string {
	if n >= 1 && n <= 16 {
		return cnNum[n]
	}
	return fmt.Sprintf("%d", n)
}

// BuildBracket 生成 TI 双败赛程（无总决赛重置，非 2 的幂自动插轮空）
func BuildBracket(teams []BracketTeam) ([]*Match, error) {
	N := len(teams)
	if N < 2 {
		return nil, fmt.Errorf("队伍数量不足（至少 2 支队伍）")
	}

	matches := []*Match{}
	byID := map[int]*Match{}
	cid := 0
	newID := func() int { cid++; return cid }

	teamSource := func(t BracketTeam) *TeamSource {
		return &TeamSource{Type: "team", Openid: t.Openid, Name: t.Name}
	}
	matchSource := func(matchID int, slot string) *TeamSource {
		return &TeamSource{Type: "match", MatchID: matchID, Slot: slot}
	}

	placeIntoMatch := func(m *Match, src *TeamSource, slot string) {
		if slot == "A" {
			m.TeamA = src
		} else {
			m.TeamB = src
		}
		if src != nil && src.Type == "match" {
			feeder := byID[src.MatchID]
			if feeder == nil {
				return
			}
			if src.Slot == "winner" {
				feeder.Next = &NextLink{MatchID: m.ID, Slot: slot}
			} else {
				feeder.LoserNext = &NextLink{MatchID: m.ID, Slot: slot}
			}
		}
	}

	pair := func(sources []*TeamSource, stage, label string) (ms []*Match, adv []*TeamSource) {
		ms = []*Match{}
		adv = []*TeamSource{}
		for i := 0; i+1 < len(sources); i += 2 {
			id := newID()
			m := &Match{ID: id, Stage: stage, Label: label, Bo: "BO1", Status: "pending"}
			placeIntoMatch(m, sources[i], "A")
			placeIntoMatch(m, sources[i+1], "B")
			ms = append(ms, m)
			byID[id] = m
			matches = append(matches, m)
			adv = append(adv, matchSource(id, "winner"))
		}
		if len(sources)%2 == 1 { // 轮空直接晋级
			adv = append(adv, sources[len(sources)-1])
		}
		return ms, adv
	}

	// ---------- 胜者组 ----------
	type ubRound struct {
		round int
		ms    []*Match
	}
	ubRounds := []ubRound{}
	ubSources := []*TeamSource{}
	for _, t := range teams {
		ubSources = append(ubSources, teamSource(t))
	}
	roundNo := 1
	for len(ubSources) > 1 {
		isFinal := len(ubSources) == 2
		stage := "UB" + fmt.Sprintf("%d", roundNo)
		label := "胜者组第" + cn(roundNo) + "轮"
		if roundNo == 1 {
			stage = "UB1"
		}
		if isFinal {
			stage = "UBF"
			label = "胜者组决赛"
		}
		ms, adv := pair(ubSources, stage, label)
		ubRounds = append(ubRounds, ubRound{round: roundNo, ms: ms})
		ubSources = adv
		roundNo++
	}
	p := len(ubRounds)

	// 只有 2 支队伍：胜者组决赛即总决赛
	if p == 1 {
		only := ubRounds[0].ms[0]
		only.Stage = "GF"
		only.Label = "总决赛"
		only.IsChampionMatch = true
		return matches, nil
	}

	// ---------- 败者组 ----------
	lbSurvivors := []*TeamSource{}
	for _, m := range ubRounds[0].ms {
		lbSurvivors = append(lbSurvivors, matchSource(m.ID, "loser"))
	}
	totalLbRounds := 2*p - 2
	for j := 1; j <= totalLbRounds; j++ {
		participants := lbSurvivors
		if j%2 == 0 {
			// 偶数轮接入下一轮胜者组的败者（交错配对避免同组自打）
			ubIdx := j / 2
			ubLosers := []*TeamSource{}
			for _, m := range ubRounds[ubIdx].ms {
				ubLosers = append(ubLosers, matchSource(m.ID, "loser"))
			}
			merged := []*TeamSource{}
			ln := len(lbSurvivors)
			if len(ubLosers) > ln {
				ln = len(ubLosers)
			}
			for i := 0; i < ln; i++ {
				if i < len(lbSurvivors) {
					merged = append(merged, lbSurvivors[i])
				}
				if i < len(ubLosers) {
					merged = append(merged, ubLosers[i])
				}
			}
			participants = merged
		}
		isLast := j == totalLbRounds
		stage := "LB" + fmt.Sprintf("%d", j)
		label := "败者组第" + cn(j) + "轮"
		if isLast {
			stage = "LBF"
			label = "败者组决赛"
		}
		_, adv := pair(participants, stage, label)
		lbSurvivors = adv
	}

	// ---------- 总决赛 ----------
	gfID := newID()
	gf := &Match{ID: gfID, Stage: "GF", Label: "总决赛", Bo: "BO1", Status: "pending", IsChampionMatch: true}
	placeIntoMatch(gf, ubSources[0], "A")   // 胜者组冠军
	placeIntoMatch(gf, lbSurvivors[0], "B") // 败者组冠军
	byID[gfID] = gf
	matches = append(matches, gf)

	return matches, nil
}

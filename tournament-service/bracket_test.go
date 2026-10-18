package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBuildBracketAllSizes(t *testing.T) {
	for N := 2; N <= 16; N++ {
		teams := make([]BracketTeam, N)
		for i := 0; i < N; i++ {
			teams[i] = BracketTeam{Openid: fmt.Sprintf("T%d", i), Name: fmt.Sprintf("队伍%d", i)}
		}
		ms, err := BuildBracket(teams)
		if err != nil {
			t.Fatalf("N=%d err: %v", N, err)
		}

		// 恰一场冠军赛
		gfCount := 0
		for _, m := range ms {
			if m.IsChampionMatch {
				gfCount++
			}
		}
		if gfCount != 1 {
			t.Fatalf("N=%d 冠军赛场次=%d, want 1", N, gfCount)
		}

		// 总场次：N>=3 时 2N-2；N=2 时 1
		want := 2*N - 2
		if N == 2 {
			want = 1
		}
		if len(ms) != want {
			t.Fatalf("N=%d 场次=%d, want %d", N, len(ms), want)
		}

		// 种子队源出现次数 = N（每支队伍恰好作为种子出现一次）
		teamCount := 0
		for _, m := range ms {
			for _, s := range []*TeamSource{m.TeamA, m.TeamB} {
				if s != nil && s.Type == "team" {
					teamCount++
				}
			}
		}
		if teamCount != N {
			t.Fatalf("N=%d 种子源=%d, want %d", N, teamCount, N)
		}

		// 引用合法：match 源引用已存在的、编号更小的比赛
		byID := map[int]*Match{}
		for _, m := range ms {
			byID[m.ID] = m
		}
		for _, m := range ms {
			for _, s := range []*TeamSource{m.TeamA, m.TeamB} {
				if s != nil && s.Type == "match" {
					f, ok := byID[s.MatchID]
					if !ok {
						t.Fatalf("N=%d match %d 引用不存在的比赛 %d", N, m.ID, s.MatchID)
					}
					if f.ID >= m.ID {
						t.Fatalf("N=%d match %d 引用顺序错误: %d >= %d", N, m.ID, f.ID, m.ID)
					}
					// 上游比赛必须设置了对应的输出接线
					if s.Slot == "winner" && (f.Next == nil || f.Next.MatchID != m.ID) {
						t.Fatalf("N=%d match %d 的胜者去向未接线到 %d", N, f.ID, m.ID)
					}
					if s.Slot == "loser" && (f.LoserNext == nil || f.LoserNext.MatchID != m.ID) {
						t.Fatalf("N=%d match %d 的败者去向未接线到 %d", N, f.ID, m.ID)
					}
				}
			}
		}

		// 冠军赛无下游
		for _, m := range ms {
			if m.IsChampionMatch && (m.Next != nil || m.LoserNext != nil) {
				t.Fatalf("N=%d 冠军赛不应有下游接线", N)
			}
		}

		// 模拟随机推进：无死锁，全部场次最终完成
		simulateProgression(t, N, ms)
	}
}

// simulateProgression 按拓扑序随机定胜负推进全部场次，验证赛程无死锁
func simulateProgression(t *testing.T, N int, ms []*Match) {
	rng := rand.New(rand.NewSource(int64(N) * 7919))
	byID := map[int]*Match{}
	for _, m := range ms {
		byID[m.ID] = m
	}
	status := map[int]string{}
	for _, m := range ms {
		status[m.ID] = m.Status
	}
	resolved := map[int][2]string{} // matchID -> [A队openid, B队openid]

	resolveSrc := func(s *TeamSource) (string, bool) {
		if s == nil {
			return "", false
		}
		if s.Type == "team" {
			return s.Openid, true
		}
		r, ok := resolved[s.MatchID]
		if !ok {
			return "", false
		}
		if s.Slot == "winner" {
			return r[0], true // A 位；胜者存于 [0] 由下方赋值
		}
		return r[1], true
	}

	progress := true
	for progress {
		progress = false
		for _, m := range ms {
			if status[m.ID] == "done" {
				continue
			}
			a, okA := resolveSrc(m.TeamA)
			b, okB := resolveSrc(m.TeamB)
			if !okA || !okB {
				continue
			}
			// 可玩：随机定胜负，胜者放 A 位表示
			var winner string
			if rng.Intn(2) == 0 {
				winner = a
			} else {
				winner = b
			}
			// resolved 里 [0]=胜者 openid, [1]=败者 openid
			loser := b
			if winner == b {
				loser = a
			}
			resolved[m.ID] = [2]string{winner, loser}
			m.Winner = winner
			m.Status = "done"
			status[m.ID] = "done"
			progress = true
		}
	}

	done := 0
	for _, m := range ms {
		if status[m.ID] != "done" {
			t.Fatalf("N=%d 比赛 %d(%s) 无法推进完成", N, m.ID, m.Label)
		}
		if m.IsChampionMatch {
			done++
		}
	}
	if done != 1 {
		t.Fatalf("N=%d 完成的冠军赛=%d, want 1", N, done)
	}
}

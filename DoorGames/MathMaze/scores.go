package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ScoreEntry is one recorded score event.
type ScoreEntry struct {
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	Score     int       `json:"score"`
	Overall   int       `json:"overall"` // cumulative session overall at save
	When      time.Time `json:"when"`
}

// PlayerBest tracks per-player highs.
type PlayerBest struct {
	Name         string         `json:"name"`
	HighestLevel int            `json:"highest_level"`
	LevelBest    map[string]int `json:"level_best"` // level number as string -> best score
	BestOverall  int            `json:"best_overall"`
}

// ScoreStore persists MathMaze high/low scores.
type ScoreStore struct {
	Players []PlayerBest `json:"players"`
	Runs    []ScoreEntry `json:"runs"` // historical runs for top/lowest lists
	path    string
}

func LoadScores(dataDir string) *ScoreStore {
	path := filepath.Join(dataDir, "scores.json")
	s := &ScoreStore{path: path, Runs: nil, Players: nil}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	s.path = path
	return s
}

func (s *ScoreStore) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *ScoreStore) player(name string) *PlayerBest {
	for i := range s.Players {
		if strings.EqualFold(s.Players[i].Name, name) {
			return &s.Players[i]
		}
	}
	s.Players = append(s.Players, PlayerBest{
		Name:      name,
		LevelBest: map[string]int{},
	})
	return &s.Players[len(s.Players)-1]
}

func levelKey(level int) string { return fmt.Sprintf("%d", level) }

// BestForLevel returns this player's best score on a level (0 if none).
func (s *ScoreStore) BestForLevel(name string, level int) int {
	p := s.player(name)
	if p.LevelBest == nil {
		return 0
	}
	return p.LevelBest[levelKey(level)]
}

// HighestLevel returns player's highest level achieved.
func (s *ScoreStore) HighestLevel(name string) int {
	return s.player(name).HighestLevel
}

// RecordSession updates store when a player quits or finishes.
// levelScores is map[level]score for this session; maxLevel is furthest level reached (1-based).
// overall is sum of level scores this session.
func (s *ScoreStore) RecordSession(name string, levelScores map[int]int, maxLevel, overall int) {
	p := s.player(name)
	if p.LevelBest == nil {
		p.LevelBest = map[string]int{}
	}
	if maxLevel > p.HighestLevel {
		p.HighestLevel = maxLevel
	}
	if overall > p.BestOverall {
		p.BestOverall = overall
	}
	now := time.Now()
	for level, score := range levelScores {
		lk := levelKey(level)
		if score > p.LevelBest[lk] {
			p.LevelBest[lk] = score
		}
		s.Runs = append(s.Runs, ScoreEntry{
			Name: name, Level: level, Score: score, Overall: overall, When: now,
		})
	}
	// Cap runs history
	if len(s.Runs) > 500 {
		s.Runs = s.Runs[len(s.Runs)-500:]
	}
}

type namedScore struct {
	Name  string
	Score int
	Level int
}

func topOverall(runs []ScoreEntry, players []PlayerBest, n int) []namedScore {
	// Prefer player best_overall
	var list []namedScore
	for _, p := range players {
		if p.BestOverall > 0 {
			list = append(list, namedScore{Name: p.Name, Score: p.BestOverall})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func topHighestLevel(players []PlayerBest, n int) []namedScore {
	var list []namedScore
	for _, p := range players {
		if p.HighestLevel > 0 {
			list = append(list, namedScore{Name: p.Name, Level: p.HighestLevel, Score: p.HighestLevel})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Level != list[j].Level {
			return list[i].Level > list[j].Level
		}
		return list[i].Name < list[j].Name
	})
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func topPerLevel(players []PlayerBest, level, n int) []namedScore {
	var list []namedScore
	lk := levelKey(level)
	for _, p := range players {
		if p.LevelBest == nil {
			continue
		}
		if sc, ok := p.LevelBest[lk]; ok {
			list = append(list, namedScore{Name: p.Name, Score: sc, Level: level})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func lowestPerLevel(runs []ScoreEntry, level, n int) []namedScore {
	var list []namedScore
	seen := map[string]int{} // best (lowest) per name for this level from runs
	for _, r := range runs {
		if r.Level != level {
			continue
		}
		prev, ok := seen[r.Name]
		if !ok || r.Score < prev {
			seen[r.Name] = r.Score
		}
	}
	for name, sc := range seen {
		list = append(list, namedScore{Name: name, Score: sc, Level: level})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score < list[j].Score })
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func levelsPresent(players []PlayerBest, runs []ScoreEntry) []int {
	set := map[int]bool{}
	for _, p := range players {
		for lv := range p.LevelBest {
			var n int
			fmt.Sscanf(lv, "%d", &n)
			if n > 0 {
				set[n] = true
			}
		}
	}
	for _, r := range runs {
		set[r.Level] = true
	}
	var out []int
	for lv := range set {
		out = append(out, lv)
	}
	sort.Ints(out)
	return out
}

// WriteBulletin writes MATHMAZE.ANS top scores bulletin.
func (s *ScoreStore) WriteBulletin(path string, topN int) error {
	if topN <= 0 {
		topN = 10
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("\x1b[2J\x1b[H")
	b.WriteString(fmt.Sprintf("\x1b[1m\x1b[96m=[ \x1b[97mMathMaze Top Scores\x1b[96m ]=\x1b[0m  \x1b[90mv%s\x1b[0m\r\n\r\n", Version))

	b.WriteString("\x1b[93mHighest Level Achieved\x1b[0m\r\n")
	hl := topHighestLevel(s.Players, topN)
	if len(hl) == 0 {
		b.WriteString("  (none yet)\r\n")
	}
	for i, e := range hl {
		b.WriteString(fmt.Sprintf("  %2d. %-20s  Level %d\r\n", i+1, trunc(e.Name, 20), e.Level))
	}
	b.WriteString("\r\n")

	b.WriteString("\x1b[93mOverall Top Scores\x1b[0m\r\n")
	ov := topOverall(s.Runs, s.Players, topN)
	if len(ov) == 0 {
		b.WriteString("  (none yet)\r\n")
	}
	for i, e := range ov {
		b.WriteString(fmt.Sprintf("  %2d. %-20s  %5d\r\n", i+1, trunc(e.Name, 20), e.Score))
	}
	b.WriteString("\r\n")

	levels := levelsPresent(s.Players, s.Runs)
	b.WriteString("\x1b[93mTop Scores Per Level\x1b[0m\r\n")
	if len(levels) == 0 {
		b.WriteString("  (none yet)\r\n")
	}
	for _, lv := range levels {
		b.WriteString(fmt.Sprintf("\x1b[96m  Level %d\x1b[0m\r\n", lv))
		tops := topPerLevel(s.Players, lv, topN)
		if len(tops) == 0 {
			b.WriteString("    (none)\r\n")
			continue
		}
		for i, e := range tops {
			b.WriteString(fmt.Sprintf("    %2d. %-18s  %5d\r\n", i+1, trunc(e.Name, 18), e.Score))
		}
	}
	b.WriteString("\r\n")

	b.WriteString("\x1b[91mLowest Scores Per Level\x1b[0m\r\n")
	if len(levels) == 0 {
		b.WriteString("  (none yet)\r\n")
	}
	for _, lv := range levels {
		b.WriteString(fmt.Sprintf("\x1b[96m  Level %d\x1b[0m\r\n", lv))
		lows := lowestPerLevel(s.Runs, lv, topN)
		if len(lows) == 0 {
			// fall back to level best (treat as only scores)
			lows = topPerLevel(s.Players, lv, topN)
			sort.Slice(lows, func(i, j int) bool { return lows[i].Score < lows[j].Score })
			if len(lows) > topN {
				lows = lows[:topN]
			}
		}
		for i, e := range lows {
			b.WriteString(fmt.Sprintf("    %2d. %-18s  %5d\r\n", i+1, trunc(e.Name, 18), e.Score))
		}
	}
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("\x1b[90m  MathMaze v%s — generated %s\x1b[0m\r\n", Version, time.Now().Format(time.RFC3339)))
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

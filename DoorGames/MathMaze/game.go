package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Game is one MathMaze play session.
type Game struct {
	term   *Terminal
	cfg    Config
	scores *ScoreStore
	player string
	rng    *rand.Rand

	level        int
	levelScore   int
	overall      int
	levelScores  map[int]int
	maxLevelSeen int

	maze *Maze
	px   int
	py   int

	// quiz state — number-key multiple choice (1–4)
	quizActive  bool
	quizQ       Question
	quizChoices []int // index 0..3 = keys 1..4
	quizCorrect int   // 0-based index of correct choice
	quizTargetX int
	quizTargetY int
	statusMsg   string
}

// Run plays until quit or disconnect.
func Run(term *Terminal, cfg Config, scores *ScoreStore, player string) {
	g := &Game{
		term:         term,
		cfg:          cfg,
		scores:       scores,
		player:       player,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		level:        1,
		levelScores:  map[int]int{},
		maxLevelSeen: 1,
	}
	g.startLevel()
	for {
		g.draw()
		key, err := term.ReadKey()
		if err != nil || key == KeyQuit {
			g.finish()
			return
		}
		if g.quizActive {
			g.handleQuiz(key)
			continue
		}
		g.handleMove(key)
	}
}

func (g *Game) mazeSize() (w, h int) {
	grow := (g.level - 1) / 2
	w = g.cfg.BaseWidth + grow*2
	h = g.cfg.BaseHeight + grow*2
	if w > 21 {
		w = 21
	}
	if h > 15 {
		h = 15
	}
	return w, h
}

func (g *Game) startLevel() {
	w, h := g.mazeSize()
	g.maze = NewMaze(w, h, g.rng)
	g.px, g.py = g.maze.StartX, g.maze.StartY
	g.levelScore = 0
	g.quizActive = false
	g.maze.GateLabels = nil
	g.statusMsg = fmt.Sprintf("Level %d — reach E. Wrong answer -2, correct +5. Q=quit", g.level)
	if g.level > g.maxLevelSeen {
		g.maxLevelSeen = g.level
	}
}

func (g *Game) bestHUD() int {
	return g.scores.BestForLevel(g.player, g.level)
}

func (g *Game) draw() {
	t := g.term
	t.Clear()
	t.Print(color(cBrightCyan, "=[ ")+color(cBrightWhite, "MathMaze "+Version)+color(cBrightCyan, " ]=")+"\r\n")
	best := g.bestHUD()
	t.Printf("%s  Level %s  Score %s  Best %s  Overall %s\r\n",
		color(cBrightYellow, g.player),
		color(cBrightWhite, fmt.Sprintf("%d", g.level)),
		color(cBrightGreen, fmt.Sprintf("%d", g.levelScore)),
		color(cCyan, fmt.Sprintf("%d", best)),
		color(cYellow, fmt.Sprintf("%d", g.overall)),
	)
	if g.quizActive {
		t.Print(color(cBrightBlack, "Room locked — press 1-4 to answer   Q: quit") + "\r\n\r\n")
	} else {
		t.Print(color(cBrightBlack, "Arrows: move   Q: quit") + "\r\n\r\n")
	}
	t.Print(g.maze.Render(g.px, g.py))
	t.Print("\r\n")
	if g.quizActive {
		t.Print(color(cBrightYellow, "Problem: ") + color(cBrightWhite, g.quizQ.Prompt) + "\r\n")
		parts := make([]string, 0, 4)
		for i, ans := range g.quizChoices {
			parts = append(parts, fmt.Sprintf("%s=%d",
				color(cBrightCyan, fmt.Sprintf("%d", i+1)), ans))
		}
		t.Print(strings.Join(parts, "   ") + "\r\n")
	}
	if g.statusMsg != "" {
		t.Print("\r\n" + g.statusMsg + "\r\n")
	}
}

func (g *Game) handleMove(key int) {
	d, ok := KeyFromArrow(key)
	if !ok {
		return
	}
	nx, ny, open := g.maze.Neighbor(g.px, g.py, d)
	if !open {
		g.statusMsg = "Wall."
		return
	}
	if g.maze.IsSolved(nx, ny) {
		g.px, g.py = nx, ny
		g.statusMsg = ""
		g.checkLevelComplete()
		return
	}
	// Locked room — answer a maths question before entering
	g.beginQuiz(nx, ny)
}

func (g *Game) beginQuiz(tx, ty int) {
	g.quizActive = true
	g.quizTargetX, g.quizTargetY = tx, ty
	g.reshuffleQuiz()
	g.statusMsg = "Press 1-4 for the correct answer to enter the room."
}

func (g *Game) reshuffleQuiz() {
	g.quizQ = NewQuestion(g.level, g.rng)
	g.quizChoices = Distractors(g.quizQ.Answer, 4, g.rng)
	g.quizCorrect = -1
	for i, a := range g.quizChoices {
		if a == g.quizQ.Answer {
			g.quizCorrect = i
			break
		}
	}
	if g.quizCorrect < 0 {
		g.quizChoices[0] = g.quizQ.Answer
		g.quizCorrect = 0
		g.rng.Shuffle(len(g.quizChoices), func(i, j int) {
			g.quizChoices[i], g.quizChoices[j] = g.quizChoices[j], g.quizChoices[i]
		})
		for i, a := range g.quizChoices {
			if a == g.quizQ.Answer {
				g.quizCorrect = i
				break
			}
		}
	}
	g.maze.GateLabels = nil
}

func (g *Game) handleQuiz(key int) {
	n := DigitFromKey(key)
	if n == 0 {
		g.statusMsg = "Press 1, 2, 3, or 4."
		return
	}
	choice := n - 1
	if choice == g.quizCorrect {
		g.levelScore += 5
		g.overall += 5
		g.maze.MarkSolved(g.quizTargetX, g.quizTargetY)
		g.px, g.py = g.quizTargetX, g.quizTargetY
		g.quizActive = false
		g.quizChoices = nil
		g.statusMsg = color(cBrightGreen, "Correct! +5")
		g.checkLevelComplete()
		return
	}
	if g.levelScore >= 2 {
		g.levelScore -= 2
		g.overall -= 2
		if g.overall < 0 {
			g.overall = 0
		}
	} else {
		g.overall -= g.levelScore
		if g.overall < 0 {
			g.overall = 0
		}
		g.levelScore = 0
	}
	g.statusMsg = color(cBrightRed, "Wrong! -2 — new question")
	g.reshuffleQuiz()
}

func (g *Game) checkLevelComplete() {
	if g.px != g.maze.EndX || g.py != g.maze.EndY {
		return
	}
	g.levelScores[g.level] = g.levelScore
	g.statusMsg = color(cBrightGreen, fmt.Sprintf("Level %d complete! Score %d", g.level, g.levelScore))
	g.draw()
	g.term.Print("\r\nPress any key for next level...\r\n")
	_, _ = g.term.ReadKey()
	g.level++
	if g.level > g.maxLevelSeen {
		g.maxLevelSeen = g.level
	}
	g.startLevel()
}

func (g *Game) finish() {
	if g.levelScore > 0 || g.levelScores[g.level] == 0 {
		g.levelScores[g.level] = g.levelScore
	}
	g.scores.RecordSession(g.player, g.levelScores, g.maxLevelSeen, g.overall)
	_ = g.scores.Save()
	_ = g.scores.WriteBulletin(g.cfg.BulletinPath, g.cfg.TopN)

	g.term.Clear()
	g.term.Printf("%s\r\n", color(cBrightCyan, "Thanks for playing MathMaze "+Version+"!"))
	g.term.Printf("Player: %s  Overall: %d  Highest level: %d\r\n", g.player, g.overall, g.maxLevelSeen)
	g.term.Print(color(cBrightBlack, "Scores saved. Returning to BBS...\r\n"))
}

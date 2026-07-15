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

	// quiz state
	quizActive   bool
	quizQ        Question
	quizAnswers  map[int]int  // direction -> answer value
	quizCorrectD int          // direction whose answer is correct
	quizTargetX  int
	quizTargetY  int
	statusMsg    string
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
	g.statusMsg = fmt.Sprintf("Level %d — reach E. Wrong gate -2, correct +5. Q=quit", g.level)
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
	t.Print(color(cBrightBlack, "Arrows: move / pick gate   Q: quit") + "\r\n\r\n")
	t.Print(g.maze.Render(g.px, g.py))
	t.Print("\r\n")
	if g.quizActive {
		t.Print(color(cBrightYellow, "Problem: ") + color(cBrightWhite, g.quizQ.Prompt) + "\r\n")
		t.Print(color(cCyan, "Gates: "))
		parts := []string{}
		for _, d := range []int{DirN, DirE, DirS, DirW} {
			if ans, ok := g.quizAnswers[d]; ok {
				mark := ""
				if g.maze.Cells[g.py][g.px]&d == 0 {
					mark = "*" // ghost gate (closed wall)
				}
				parts = append(parts, fmt.Sprintf("%s%s=%d", dirName[d], mark, ans))
			}
		}
		t.Print(strings.Join(parts, "  ") + "\r\n")
		t.Print(color(cBrightBlack, "(* = decoy; arrow that way is wrong)") + "\r\n")
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
	// Locked room — start quiz with correct answer on this exit
	g.beginQuiz(d, nx, ny)
}

func (g *Game) beginQuiz(toward int, tx, ty int) {
	g.quizActive = true
	g.quizTargetX, g.quizTargetY = tx, ty
	g.quizCorrectD = toward
	g.reshuffleQuiz()
	g.statusMsg = "Choose the gate with the correct answer!"
}

func (g *Game) reshuffleQuiz() {
	g.quizQ = NewQuestion(g.level, g.rng)
	open := g.maze.OpenDirs(g.px, g.py)
	// Ensure the intended direction is among labeled exits
	need := len(open)
	if need < 3 {
		need = 3
	}
	if need > 4 {
		need = 4
	}
	answers := Distractors(g.quizQ.Answer, need, g.rng)

	g.quizAnswers = map[int]int{}
	g.maze.GateLabels = map[int]string{}

	// Place correct answer on quizCorrectD (must be open)
	correctAssigned := false
	var wrongPool []int
	for _, a := range answers {
		if a == g.quizQ.Answer && !correctAssigned {
			continue // assign later
		}
		wrongPool = append(wrongPool, a)
	}
	// Ensure correct is in answers
	hasCorrect := false
	for _, a := range answers {
		if a == g.quizQ.Answer {
			hasCorrect = true
			break
		}
	}
	if !hasCorrect {
		answers[0] = g.quizQ.Answer
	}

	g.quizAnswers[g.quizCorrectD] = g.quizQ.Answer
	g.maze.GateLabels[g.quizCorrectD] = LabelForAnswer(g.quizQ.Answer)
	correctAssigned = true

	// Assign wrong answers to other open dirs
	wi := 0
	for _, d := range open {
		if d == g.quizCorrectD {
			continue
		}
		if wi >= len(wrongPool) {
			wrongPool = append(wrongPool, g.quizQ.Answer+wi+3)
		}
		g.quizAnswers[d] = wrongPool[wi]
		g.maze.GateLabels[d] = LabelForAnswer(wrongPool[wi])
		wi++
	}
	// Ghost wrong answers on closed dirs until we have ≥3 choices
	if len(g.quizAnswers) < 3 {
		for _, d := range []int{DirN, DirE, DirS, DirW} {
			if _, ok := g.quizAnswers[d]; ok {
				continue
			}
			if wi >= len(wrongPool) {
				wrongPool = append(wrongPool, g.quizQ.Answer+10+wi)
			}
			g.quizAnswers[d] = wrongPool[wi]
			g.maze.GateLabels[d] = LabelForAnswer(wrongPool[wi])
			wi++
			if len(g.quizAnswers) >= 3 {
				break
			}
		}
	}
	_ = correctAssigned
}

func (g *Game) handleQuiz(key int) {
	d, ok := KeyFromArrow(key)
	if !ok {
		return
	}
	ans, labeled := g.quizAnswers[d]
	if !labeled {
		g.statusMsg = "No gate that way."
		return
	}
	if ans == g.quizQ.Answer && d == g.quizCorrectD {
		g.levelScore += 5
		g.overall += 5
		g.maze.MarkSolved(g.quizTargetX, g.quizTargetY)
		g.px, g.py = g.quizTargetX, g.quizTargetY
		g.quizActive = false
		g.maze.GateLabels = nil
		g.quizAnswers = nil
		g.statusMsg = color(cBrightGreen, "Correct! +5")
		g.checkLevelComplete()
		return
	}
	// Wrong (including correct value on wrong ghost dir, or wrong value)
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
	// Persist current level score if any progress
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

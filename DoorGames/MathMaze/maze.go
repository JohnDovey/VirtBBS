package main

import (
	"math/rand"
	"strings"
)

// Directions
const (
	DirN = 1 << iota
	DirS
	DirE
	DirW
)

var dirDelta = map[int][2]int{
	DirN: {0, -1},
	DirS: {0, 1},
	DirE: {1, 0},
	DirW: {-1, 0},
}

var dirOpposite = map[int]int{
	DirN: DirS,
	DirS: DirN,
	DirE: DirW,
	DirW: DirE,
}

var dirName = map[int]string{
	DirN: "N",
	DirS: "S",
	DirE: "E",
	DirW: "W",
}

// Maze is a perfect maze of cells with open walls as bitflags.
type Maze struct {
	W, H       int
	Cells      [][]int
	StartX     int
	StartY     int
	EndX       int
	EndY       int
	Solved     map[[2]int]bool
	GateLabels map[int]string // direction -> answer label text
}

// NewMaze generates a perfect maze with recursive backtracker.
func NewMaze(w, h int, rng *rand.Rand) *Maze {
	if w%2 == 0 {
		w++
	}
	if h%2 == 0 {
		h++
	}
	if w < 5 {
		w = 5
	}
	if h < 5 {
		h = 5
	}
	m := &Maze{
		W:      w,
		H:      h,
		Cells:  make([][]int, h),
		Solved: make(map[[2]int]bool),
	}
	for y := 0; y < h; y++ {
		m.Cells[y] = make([]int, w)
	}

	type pt struct{ x, y int }
	stack := []pt{{0, 0}}
	visited := make([][]bool, h)
	for y := 0; y < h; y++ {
		visited[y] = make([]bool, w)
	}
	visited[0][0] = true

	dirs := []int{DirN, DirS, DirE, DirW}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		var neighbors []int
		for _, d := range dirs {
			dx, dy := dirDelta[d][0], dirDelta[d][1]
			nx, ny := cur.x+dx, cur.y+dy
			if nx >= 0 && nx < w && ny >= 0 && ny < h && !visited[ny][nx] {
				neighbors = append(neighbors, d)
			}
		}
		if len(neighbors) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		d := neighbors[rng.Intn(len(neighbors))]
		dx, dy := dirDelta[d][0], dirDelta[d][1]
		nx, ny := cur.x+dx, cur.y+dy
		m.Cells[cur.y][cur.x] |= d
		m.Cells[ny][nx] |= dirOpposite[d]
		visited[ny][nx] = true
		stack = append(stack, pt{nx, ny})
	}

	m.StartX, m.StartY = 0, 0
	corners := [][2]int{{w - 1, h - 1}, {w - 1, 0}, {0, h - 1}}
	best := corners[0]
	bestDist := -1
	for _, c := range corners {
		d := bfsDist(m, 0, 0, c[0], c[1])
		if d > bestDist {
			bestDist = d
			best = c
		}
	}
	m.EndX, m.EndY = best[0], best[1]
	m.Solved[[2]int{m.StartX, m.StartY}] = true
	return m
}

func bfsDist(m *Maze, sx, sy, tx, ty int) int {
	type qn struct{ x, y, d int }
	q := []qn{{sx, sy, 0}}
	seen := map[[2]int]bool{{sx, sy}: true}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if cur.x == tx && cur.y == ty {
			return cur.d
		}
		for _, d := range []int{DirN, DirS, DirE, DirW} {
			if m.Cells[cur.y][cur.x]&d == 0 {
				continue
			}
			nx, ny := cur.x+dirDelta[d][0], cur.y+dirDelta[d][1]
			k := [2]int{nx, ny}
			if seen[k] {
				continue
			}
			seen[k] = true
			q = append(q, qn{nx, ny, cur.d + 1})
		}
	}
	return -1
}

// OpenDirs returns directions with passages from (x,y).
func (m *Maze) OpenDirs(x, y int) []int {
	var out []int
	for _, d := range []int{DirN, DirE, DirS, DirW} {
		if m.Cells[y][x]&d != 0 {
			out = append(out, d)
		}
	}
	return out
}

// Neighbor returns the cell reached by direction d, if open.
func (m *Maze) Neighbor(x, y, d int) (nx, ny int, ok bool) {
	if m.Cells[y][x]&d == 0 {
		return 0, 0, false
	}
	nx = x + dirDelta[d][0]
	ny = y + dirDelta[d][1]
	if nx < 0 || nx >= m.W || ny < 0 || ny >= m.H {
		return 0, 0, false
	}
	return nx, ny, true
}

func (m *Maze) IsSolved(x, y int) bool {
	return m.Solved[[2]int{x, y}]
}

func (m *Maze) MarkSolved(x, y int) {
	m.Solved[[2]int{x, y}] = true
}

func KeyFromArrow(key int) (int, bool) {
	switch key {
	case KeyUp:
		return DirN, true
	case KeyDown:
		return DirS, true
	case KeyLeft:
		return DirW, true
	case KeyRight:
		return DirE, true
	}
	return 0, false
}

// Render draws an ASCII maze; gate answer labels appear on open passages from the player.
func (m *Maze) Render(px, py int) string {
	rows := m.H*2 + 1
	cols := m.W*4 + 1
	grid := make([][]rune, rows)
	for y := 0; y < rows; y++ {
		grid[y] = make([]rune, cols)
		for x := 0; x < cols; x++ {
			grid[y][x] = ' '
		}
	}

	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			cx, cy := x*4+2, y*2+1
			grid[cy-1][cx-2] = '+'
			grid[cy-1][cx+2] = '+'
			grid[cy+1][cx-2] = '+'
			grid[cy+1][cx+2] = '+'
			if m.Cells[y][x]&DirN == 0 {
				grid[cy-1][cx-1], grid[cy-1][cx], grid[cy-1][cx+1] = '-', '-', '-'
			}
			if m.Cells[y][x]&DirS == 0 {
				grid[cy+1][cx-1], grid[cy+1][cx], grid[cy+1][cx+1] = '-', '-', '-'
			}
			if m.Cells[y][x]&DirW == 0 {
				grid[cy][cx-2] = '|'
			}
			if m.Cells[y][x]&DirE == 0 {
				grid[cy][cx+2] = '|'
			}
			ch := ' '
			switch {
			case x == px && y == py:
				ch = '@'
			case x == m.StartX && y == m.StartY:
				ch = 'S'
			case x == m.EndX && y == m.EndY:
				ch = 'E'
			case m.IsSolved(x, y):
				ch = '.'
			}
			grid[cy][cx] = ch
		}
	}

	// Multi-char gate labels on passages (overwrite 1–3 cells)
	if m.GateLabels != nil {
		cx, cy := px*4+2, py*2+1
		for d, lab := range m.GateLabels {
			if lab == "" {
				continue
			}
			r := []rune(lab)
			place := func(x, y int, rr rune) {
				if y >= 0 && y < rows && x >= 0 && x < cols {
					grid[y][x] = rr
				}
			}
			switch d {
			case DirN:
				for i, rr := range r {
					if i > 2 {
						break
					}
					place(cx-1+i, cy-1, rr)
				}
			case DirS:
				for i, rr := range r {
					if i > 2 {
						break
					}
					place(cx-1+i, cy+1, rr)
				}
			case DirW:
				place(cx-1, cy, r[0])
			case DirE:
				place(cx+1, cy, r[0])
			}
		}
	}

	var b strings.Builder
	for y := 0; y < rows; y++ {
		b.WriteString("  ")
		for _, r := range grid[y] {
			switch r {
			case '@':
				b.WriteString(color(cBrightGreen, "@"))
			case 'S':
				b.WriteString(color(cBrightCyan, "S"))
			case 'E':
				b.WriteString(color(cMagenta, "E"))
			default:
				if m.isGateRune(r) {
					b.WriteString(color(cBrightYellow, string(r)))
				} else {
					b.WriteRune(r)
				}
			}
		}
		b.WriteString("\r\n")
	}
	return b.String()
}

func (m *Maze) isGateRune(r rune) bool {
	if m.GateLabels == nil {
		return false
	}
	if (r >= '0' && r <= '9') || r == '-' {
		for _, lab := range m.GateLabels {
			for _, lr := range lab {
				if lr == r {
					return true
				}
			}
		}
	}
	return false
}

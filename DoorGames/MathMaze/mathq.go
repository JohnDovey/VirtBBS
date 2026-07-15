package main

import (
	"fmt"
	"math/rand"
)

// Question is a maths problem with integer answer.
type Question struct {
	Prompt string
	Answer int
}

// NewQuestion generates a problem scaled by level (1+).
func NewQuestion(level int, rng *rand.Rand) Question {
	if level < 1 {
		level = 1
	}
	switch {
	case level <= 2:
		return addSub(level, rng)
	case level <= 4:
		if rng.Intn(2) == 0 {
			return mulEasy(level, rng)
		}
		return addSub(level+1, rng)
	default:
		switch rng.Intn(4) {
		case 0:
			return mulHard(level, rng)
		case 1:
			return twoStep(level, rng)
		case 2:
			return addSub(level+2, rng)
		default:
			return mulEasy(level, rng)
		}
	}
}

func addSub(level int, rng *rand.Rand) Question {
	max := 10 + level*5
	a := rng.Intn(max) + 1
	b := rng.Intn(max) + 1
	if rng.Intn(2) == 0 {
		return Question{Prompt: fmt.Sprintf("%d + %d = ?", a, b), Answer: a + b}
	}
	if a < b {
		a, b = b, a
	}
	return Question{Prompt: fmt.Sprintf("%d - %d = ?", a, b), Answer: a - b}
}

func mulEasy(level int, rng *rand.Rand) Question {
	a := rng.Intn(5+level) + 2
	b := rng.Intn(5+level) + 2
	return Question{Prompt: fmt.Sprintf("%d × %d = ?", a, b), Answer: a * b}
}

func mulHard(level int, rng *rand.Rand) Question {
	a := rng.Intn(8+level) + 3
	b := rng.Intn(8+level) + 3
	return Question{Prompt: fmt.Sprintf("%d × %d = ?", a, b), Answer: a * b}
}

func twoStep(level int, rng *rand.Rand) Question {
	a := rng.Intn(6+level) + 2
	b := rng.Intn(6+level) + 2
	c := rng.Intn(8+level) + 1
	if rng.Intn(2) == 0 {
		ans := a*b + c
		return Question{Prompt: fmt.Sprintf("%d × %d + %d = ?", a, b, c), Answer: ans}
	}
	ans := a*b - c
	if ans < 0 {
		c = rng.Intn(a*b) + 0
		ans = a*b - c
	}
	return Question{Prompt: fmt.Sprintf("%d × %d - %d = ?", a, b, c), Answer: ans}
}

// Distractors builds unique wrong answers near the correct one, plus correct.
// Returns a shuffled slice of length n (including correct).
func Distractors(correct int, n int, rng *rand.Rand) []int {
	if n < 2 {
		n = 2
	}
	seen := map[int]bool{correct: true}
	out := []int{correct}
	offsets := []int{1, -1, 2, -2, 3, -3, 4, -4, 5, 10, -5, 6, -6, 7, 8, 9, 11, 12}
	rng.Shuffle(len(offsets), func(i, j int) { offsets[i], offsets[j] = offsets[j], offsets[i] })
	for _, off := range offsets {
		if len(out) >= n {
			break
		}
		v := correct + off
		if v < 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	for len(out) < n {
		v := correct + rng.Intn(21) - 10
		if v < 0 || seen[v] {
			v = correct + len(out) + 1
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// LabelForAnswer short label for maze display (fits in 1–3 chars ideally).
func LabelForAnswer(v int) string {
	s := fmt.Sprintf("%d", v)
	if len(s) > 3 {
		return s[:3]
	}
	return s
}

package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDoorSYS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DOOR.SYS")
	content := "COM1:\r\n38400,N,8,1\r\n0\r\n1\r\n38400\r\nY\r\nY\r\nY\r\nY\r\nAlice Wonder\r\nCity\r\n\r\n\r\n\r\n50\r\n1\r\n01/01/26\r\n3600\r\n60\r\n0\r\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseDoorSYS(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.UserName != "Alice Wonder" {
		t.Fatalf("got name %q", s.UserName)
	}
	if !s.ANSI {
		t.Fatal("expected ANSI")
	}
}

func TestMazeGeneration(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	m := NewMaze(9, 7, rng)
	if m.W < 5 || m.H < 5 {
		t.Fatalf("size %dx%d", m.W, m.H)
	}
	if !m.IsSolved(m.StartX, m.StartY) {
		t.Fatal("start should be solved")
	}
	if bfsDist(m, m.StartX, m.StartY, m.EndX, m.EndY) < 0 {
		t.Fatal("end not reachable")
	}
}

func TestQuestionAndScores(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	q := NewQuestion(1, rng)
	if q.Prompt == "" {
		t.Fatal("empty prompt")
	}
	ds := Distractors(q.Answer, 3, rng)
	if len(ds) != 3 {
		t.Fatalf("got %d distractors", len(ds))
	}
	found := false
	for _, d := range ds {
		if d == q.Answer {
			found = true
		}
	}
	if !found {
		t.Fatal("correct answer missing from distractors")
	}

	dir := t.TempDir()
	store := LoadScores(dir)
	store.RecordSession("Bob", map[int]int{1: 15, 2: 10}, 2, 25)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "out.ANS")
	if err := store.WriteBulletin(path, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

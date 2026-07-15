package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	sauceFieldTitle = iota
	sauceFieldAuthor
	sauceFieldGroup
	sauceFieldDate
	sauceFieldDataType
	sauceFieldFileType
	sauceFieldWidth
	sauceFieldHeight
	sauceFieldFlags
	sauceFieldComments // list start; selection indexes into comment lines with offset
	sauceFieldCount
)

func (e *Editor) runSauceUI() {
	s := e.sauce
	if !s.Present {
		s = NewSauce()
		s.TInfo1 = uint16(e.canvas.Cols)
		s.TInfo2 = uint16(e.canvas.Rows)
	}
	// Work on a copy
	work := s
	if work.CommentLines == nil {
		work.CommentLines = []string{}
	}
	field := 0
	comntIdx := 0
	dirtyLocal := false

	for {
		e.drawSauceScreen(work, field, comntIdx, dirtyLocal)
		ev, err := e.term.ReadEvent()
		if err != nil {
			return
		}
		switch ev.Kind {
		case KeyEsc:
			if dirtyLocal {
				e.term.MoveTo(e.termRows(), 1)
				e.term.Print("\x1b[K Discard SAUCE changes? [y/N] ")
				yev, _ := e.term.ReadEvent()
				if yev.Kind == KeyRune && (yev.Rune == 'y' || yev.Rune == 'Y') {
					return
				}
				continue
			}
			return
		case KeyCtrlS:
			e.sauce = work
			e.sauce.Present = true
			e.sauce.Comments = uint8(len(e.sauce.CommentLines))
			e.dirty = true
			e.status = "SAUCE applied"
			return
		case KeyUp:
			if field == sauceFieldComments {
				if comntIdx > 0 {
					comntIdx--
				} else {
					field = sauceFieldFlags
				}
			} else if field > 0 {
				field--
			}
		case KeyDown, KeyEnter:
			if field < sauceFieldComments {
				if ev.Kind == KeyEnter {
					e.editSauceField(&work, field, &dirtyLocal)
				} else {
					field++
					if field == sauceFieldComments {
						comntIdx = 0
					}
				}
			} else {
				if ev.Kind == KeyEnter {
					e.editCommentLine(&work, comntIdx, &dirtyLocal)
				} else if comntIdx+1 < len(work.CommentLines) {
					comntIdx++
				} else if comntIdx+1 == len(work.CommentLines) || len(work.CommentLines) == 0 {
					// stay / allow insert via key
				}
			}
		case KeyRune:
			switch ev.Rune {
			case 'i', 'I':
				if field == sauceFieldComments && len(work.CommentLines) < maxComments {
					work.CommentLines = append(work.CommentLines, "")
					if len(work.CommentLines) == 1 {
						comntIdx = 0
					} else {
						comntIdx = len(work.CommentLines) - 1
					}
					dirtyLocal = true
					e.editCommentLine(&work, comntIdx, &dirtyLocal)
				}
			case 'd', 'D':
				if field == sauceFieldComments && len(work.CommentLines) > 0 && comntIdx < len(work.CommentLines) {
					work.CommentLines = append(work.CommentLines[:comntIdx], work.CommentLines[comntIdx+1:]...)
					if comntIdx >= len(work.CommentLines) && comntIdx > 0 {
						comntIdx--
					}
					dirtyLocal = true
				}
			case 'x', 'X':
				// Clear SAUCE on apply
				e.term.MoveTo(e.termRows(), 1)
				e.term.Print("\x1b[K Remove SAUCE on next save? [y/N] ")
				yev, _ := e.term.ReadEvent()
				if yev.Kind == KeyRune && (yev.Rune == 'y' || yev.Rune == 'Y') {
					e.sauce = Sauce{Present: false}
					e.dirty = true
					e.status = "SAUCE will be omitted on save"
					return
				}
			case '?':
				// ignore
			}
		}
	}
}

func (e *Editor) drawSauceScreen(s Sauce, field, comntIdx int, dirtyLocal bool) {
	cols, rows := e.term.Size()
	e.term.Clear()
	e.term.MoveTo(1, 1)
	e.term.Print("\x1b[1;44;97m")
	title := " SAUCE / COMNT editor "
	if dirtyLocal {
		title += "* "
	}
	e.term.Print(padRight(title, cols))
	e.term.Print("\x1b[0m")

	labels := []string{
		"Title   (35)",
		"Author  (20)",
		"Group   (20)",
		"Date CCYYMMDD",
		"DataType",
		"FileType",
		"Width (TInfo1)",
		"Height(TInfo2)",
		"Flags",
	}
	values := []string{
		s.Title,
		s.Author,
		s.Group,
		s.Date,
		fmt.Sprintf("%d", s.DataType),
		fmt.Sprintf("%d", s.FileType),
		fmt.Sprintf("%d", s.TInfo1),
		fmt.Sprintf("%d", s.TInfo2),
		fmt.Sprintf("%d", s.Flags),
	}
	for i := 0; i < len(labels); i++ {
		row := 3 + i
		e.term.MoveTo(row, 2)
		marker := "  "
		if field == i {
			marker = "> "
			e.term.Print("\x1b[1;37;44m")
		}
		e.term.Printf("%s%-16s %s", marker, labels[i], values[i])
		e.term.Print("\x1b[0m\x1b[K")
	}

	comntRow := 3 + len(labels) + 1
	e.term.MoveTo(comntRow, 2)
	if field == sauceFieldComments {
		e.term.Print("\x1b[1;37;44m")
	}
	e.term.Printf("Comments (%d / %d)  [Enter edit] [I insert] [D delete]", len(s.CommentLines), maxComments)
	e.term.Print("\x1b[0m\x1b[K")

	listTop := comntRow + 1
	maxShow := rows - listTop - 2
	if maxShow < 3 {
		maxShow = 3
	}
	start := 0
	if comntIdx >= maxShow {
		start = comntIdx - maxShow + 1
	}
	for i := 0; i < maxShow; i++ {
		idx := start + i
		e.term.MoveTo(listTop+i, 2)
		if idx >= len(s.CommentLines) {
			e.term.Print("\x1b[K")
			continue
		}
		line := s.CommentLines[idx]
		if len(line) > 64 {
			line = line[:64]
		}
		prefix := fmt.Sprintf("%3d: ", idx+1)
		if field == sauceFieldComments && idx == comntIdx {
			e.term.Print("\x1b[1;30;46m")
			e.term.Print(prefix + padRight(line, 64))
			e.term.Print("\x1b[0m")
		} else {
			e.term.Print(prefix + padRight(line, 64))
		}
		e.term.Print("\x1b[K")
	}

	e.term.MoveTo(rows, 1)
	e.term.Print("\x1b[1;43;30m")
	help := " Enter=edit  Ctrl+S=apply  X=remove SAUCE  Esc=cancel "
	e.term.Print(padRight(help, cols))
	e.term.Print("\x1b[0m")
}

func (e *Editor) editSauceField(s *Sauce, field int, dirty *bool) {
	cols, _ := e.term.Size()
	e.term.MoveTo(e.termRows()-1, 1)
	e.term.Print("\x1b[K Value: ")
	val := e.term.PromptLine("")
	if val == "" && field != sauceFieldTitle && field != sauceFieldAuthor && field != sauceFieldGroup {
		return
	}
	*dirty = true
	switch field {
	case sauceFieldTitle:
		s.Title = truncate(val, 35)
	case sauceFieldAuthor:
		s.Author = truncate(val, 20)
	case sauceFieldGroup:
		s.Group = truncate(val, 20)
	case sauceFieldDate:
		s.Date = truncate(val, 8)
	case sauceFieldDataType:
		if n, err := strconv.Atoi(val); err == nil {
			s.DataType = uint8(n)
		}
	case sauceFieldFileType:
		if n, err := strconv.Atoi(val); err == nil {
			s.FileType = uint8(n)
		}
	case sauceFieldWidth:
		if n, err := strconv.Atoi(val); err == nil {
			s.TInfo1 = uint16(n)
		}
	case sauceFieldHeight:
		if n, err := strconv.Atoi(val); err == nil {
			s.TInfo2 = uint16(n)
		}
	case sauceFieldFlags:
		if n, err := strconv.Atoi(val); err == nil {
			s.Flags = uint8(n)
		}
	}
	_ = cols
}

func (e *Editor) editCommentLine(s *Sauce, idx int, dirty *bool) {
	if idx < 0 || idx >= len(s.CommentLines) {
		return
	}
	e.term.MoveTo(e.termRows()-1, 1)
	e.term.Printf("\x1b[K Comment %d (max 64): ", idx+1)
	val := e.term.PromptLine("")
	if len(val) > comntLineLen {
		val = val[:comntLineLen]
	}
	s.CommentLines[idx] = val
	*dirty = true
}

func (e *Editor) termRows() int {
	_, r := e.term.Size()
	return r
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func padRight(s string, n int) string {
	// pad by display width approximating rune count
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return string(r) + strings.Repeat(" ", n-len(r))
}

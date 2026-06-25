// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.1  2026-06-24  Initial implementation
// ============================================================================

package ppl

// Node is implemented by all AST node types.
type Node interface{ nodeTag() }

// ── Statements ────────────────────────────────────────────────────────────────

type Program struct {
	Stmts []Node
}

func (p *Program) nodeTag() {}

type DimStmt struct {
	Name string
	Type TypeCode
	Dims []Expr // array dimensions; nil = scalar
	Line int
}

func (d *DimStmt) nodeTag() {}

type AssignStmt struct {
	Name  string
	Index []Expr // nil = scalar
	Value Expr
	Line  int
}

func (a *AssignStmt) nodeTag() {}

type IfStmt struct {
	Cond     Expr
	Then     []Node
	ElseIfs  []ElseIf
	Else     []Node
	Line     int
}

func (i *IfStmt) nodeTag() {}

type ElseIf struct {
	Cond  Expr
	Stmts []Node
}

type WhileStmt struct {
	Cond  Expr
	Body  []Node
	Line  int
}

func (w *WhileStmt) nodeTag() {}

type ForStmt struct {
	Var   string
	Start Expr
	Stop  Expr
	Step  Expr
	Body  []Node
	Line  int
}

func (f *ForStmt) nodeTag() {}

type GotoStmt  struct{ Label string; Line int }
type GosubStmt struct{ Label string; Line int }
type ReturnStmt struct{ Line int }
type LabelStmt  struct{ Name string; Line int }
type EndStmt    struct{ Line int }
type QuitStmt   struct{ Line int }

func (g *GotoStmt) nodeTag()   {}
func (g *GosubStmt) nodeTag()  {}
func (r *ReturnStmt) nodeTag() {}
func (l *LabelStmt) nodeTag()  {}
func (e *EndStmt) nodeTag()    {}
func (q *QuitStmt) nodeTag()   {}

// CallStmt covers both built-in statements (PRINT, GETUSER, etc.)
// and user-defined procedure calls.
type CallStmt struct {
	Name string
	Args []Expr
	Line int
}

func (c *CallStmt) nodeTag() {}

type ProcDecl struct {
	Name   string
	Params []Param
	Body   []Node
	Line   int
}

func (p *ProcDecl) nodeTag() {}

type FuncDecl struct {
	Name       string
	Params     []Param
	ReturnType TypeCode
	Body       []Node
	Line       int
}

func (f *FuncDecl) nodeTag() {}

type Param struct {
	Name string
	Type TypeCode
}

// ── Expressions ───────────────────────────────────────────────────────────────

type Expr interface{ exprTag() }

type IntLit  struct{ Val int64 }
type RealLit struct{ Val float64 }
type StrLit  struct{ Val string }
type BoolLit struct{ Val bool }

func (i *IntLit) exprTag()  {}
func (r *RealLit) exprTag() {}
func (s *StrLit) exprTag()  {}
func (b *BoolLit) exprTag() {}

type VarExpr struct {
	Name  string
	Index []Expr
}

func (v *VarExpr) exprTag() {}

type BinExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (b *BinExpr) exprTag() {}

type UnaryExpr struct {
	Op  string
	Val Expr
}

func (u *UnaryExpr) exprTag() {}

type CallExpr struct {
	Name string
	Args []Expr
}

func (c *CallExpr) exprTag() {}

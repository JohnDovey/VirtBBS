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

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser converts a token stream into an AST.
type Parser struct {
	toks []Token
	pos  int
}

func NewParser(toks []Token) *Parser {
	return &Parser{toks: toks}
}

func (p *Parser) peek() Token {
	for p.pos < len(p.toks) && p.toks[p.pos].Kind == TokNewline {
		p.pos++
	}
	if p.pos >= len(p.toks) {
		return Token{Kind: TokEOF}
	}
	return p.toks[p.pos]
}

func (p *Parser) peekRaw() Token {
	if p.pos >= len(p.toks) {
		return Token{Kind: TokEOF}
	}
	return p.toks[p.pos]
}

func (p *Parser) next() Token {
	t := p.peek()
	p.pos++
	return t
}

func (p *Parser) skipNewlines() {
	for p.pos < len(p.toks) && p.toks[p.pos].Kind == TokNewline {
		p.pos++
	}
}

func (p *Parser) expectIdent(name string) error {
	t := p.next()
	if t.Kind != TokIdent || strings.ToUpper(t.Text) != strings.ToUpper(name) {
		return fmt.Errorf("line %d: expected %q, got %q", t.Line, name, t.Text)
	}
	return nil
}

// Parse parses the full program.
func (p *Parser) Parse() (*Program, error) {
	prog := &Program{}
	for p.peek().Kind != TokEOF {
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			prog.Stmts = append(prog.Stmts, stmt)
		}
	}
	return prog, nil
}

func (p *Parser) parseStmts(until ...string) ([]Node, error) {
	var stmts []Node
	for {
		p.skipNewlines()
		t := p.peek()
		if t.Kind == TokEOF {
			break
		}
		if t.Kind == TokIdent {
			up := strings.ToUpper(t.Text)
			for _, u := range until {
				if up == u {
					return stmts, nil
				}
			}
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts, nil
}

func (p *Parser) parseStmt() (Node, error) {
	p.skipNewlines()
	t := p.peek()

	if t.Kind == TokEOF {
		return nil, nil
	}

	// Label: IDENT :
	if t.Kind == TokIdent && p.pos+1 < len(p.toks) {
		// peek ahead past current
		next := p.toks[p.pos+1]
		if next.Kind == TokColon {
			p.next() // consume ident
			p.next() // consume :
			return &LabelStmt{Name: t.Text, Line: t.Line}, nil
		}
	}

	// Hash label: #LABEL
	if t.Kind == TokHash {
		p.next()
		return &LabelStmt{Name: t.Text, Line: t.Line}, nil
	}

	if t.Kind != TokIdent {
		p.next() // skip unknown token
		return nil, nil
	}

	p.next() // consume keyword/ident
	line := t.Line
	kw := strings.ToUpper(t.Text)

	switch kw {
	case "DIM":
		return p.parseDim(line)
	case "IF":
		return p.parseIf(line)
	case "WHILE":
		return p.parseWhile(line)
	case "FOR":
		return p.parseFor(line)
	case "GOTO":
		label := p.next()
		return &GotoStmt{Label: label.Text, Line: line}, nil
	case "GOSUB":
		label := p.next()
		return &GosubStmt{Label: label.Text, Line: line}, nil
	case "RETURN":
		return &ReturnStmt{Line: line}, nil
	case "END", "STOP":
		return &EndStmt{Line: line}, nil
	case "QUIT":
		return &QuitStmt{Line: line}, nil
	case "LET":
		return p.parseAssign(line)
	case "PROCEDURE", "PROC":
		return p.parseProcDecl(line)
	case "FUNCTION", "FUNC":
		return p.parseFuncDecl(line)
	case "BOOLEAN", "INTEGER", "STRING", "REAL", "DATE", "TIME", "MONEY", "BIGSTR":
		// Variable declaration without DIM keyword
		return p.parseVarDecl(kw, line)
	default:
		// Either assignment (VARNAME = expr) or built-in/procedure call
		// Check if next is = → assignment
		if p.peekRaw().Kind == TokEq {
			p.next() // consume =
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &AssignStmt{Name: kw, Value: val, Line: line}, nil
		}
		// Array assignment: NAME(idx) = expr
		if p.peekRaw().Kind == TokLParen {
			p.next() // (
			idx, err := p.parseExprList()
			if err != nil {
				return nil, err
			}
			p.next() // )
			if p.peekRaw().Kind == TokEq {
				p.next() // =
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				return &AssignStmt{Name: kw, Index: idx, Value: val, Line: line}, nil
			}
			// It's a call with paren args
			return &CallStmt{Name: kw, Args: idx, Line: line}, nil
		}
		// Built-in call or procedure call with space-delimited args
		args, err := p.parseArgList()
		if err != nil {
			return nil, err
		}
		return &CallStmt{Name: kw, Args: args, Line: line}, nil
	}
}

func (p *Parser) parseDim(line int) (Node, error) {
	name := p.next().Text
	var dims []Expr
	if p.peekRaw().Kind == TokLParen {
		p.next() // (
		var err error
		dims, err = p.parseExprList()
		if err != nil {
			return nil, err
		}
		p.next() // )
	}
	// Optional type annotation: AS TYPE
	tc := TypeString
	if p.peekRaw().Kind == TokIdent && strings.ToUpper(p.peekRaw().Text) == "AS" {
		p.next() // AS
		typTok := p.next()
		tc = typeCodeFromName(typTok.Text)
	}
	return &DimStmt{Name: strings.ToUpper(name), Type: tc, Dims: dims, Line: line}, nil
}

func (p *Parser) parseVarDecl(typeName string, line int) (Node, error) {
	name := strings.ToUpper(p.next().Text)
	tc := typeCodeFromName(typeName)
	var dims []Expr
	if p.peekRaw().Kind == TokLParen {
		p.next()
		var err error
		dims, err = p.parseExprList()
		if err != nil {
			return nil, err
		}
		p.next()
	}
	return &DimStmt{Name: name, Type: tc, Dims: dims, Line: line}, nil
}

func (p *Parser) parseAssign(line int) (Node, error) {
	name := strings.ToUpper(p.next().Text)
	var idx []Expr
	if p.peekRaw().Kind == TokLParen {
		p.next()
		var err error
		idx, err = p.parseExprList()
		if err != nil {
			return nil, err
		}
		p.next()
	}
	if err := p.expectIdent("="); err != nil {
		// try TokEq
		if p.peekRaw().Kind != TokEq {
			return nil, fmt.Errorf("line %d: expected = in LET", line)
		}
		p.next()
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &AssignStmt{Name: name, Index: idx, Value: val, Line: line}, nil
}

func (p *Parser) parseIf(line int) (Node, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// Optional THEN keyword
	if p.peekRaw().Kind == TokIdent && strings.ToUpper(p.peekRaw().Text) == "THEN" {
		p.next()
	}
	// Single-line IF: IF cond THEN stmt
	if p.peekRaw().Kind != TokNewline && p.peekRaw().Kind != TokEOF {
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		return &IfStmt{Cond: cond, Then: []Node{stmt}, Line: line}, nil
	}

	// Block IF
	then, err := p.parseStmts("ELSE", "ELSEIF", "ENDIF")
	if err != nil {
		return nil, err
	}
	node := &IfStmt{Cond: cond, Then: then, Line: line}

	for p.peek().Kind == TokIdent && strings.ToUpper(p.peek().Text) == "ELSEIF" {
		p.next()
		eicond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peekRaw().Kind == TokIdent && strings.ToUpper(p.peekRaw().Text) == "THEN" {
			p.next()
		}
		eistmts, err := p.parseStmts("ELSE", "ELSEIF", "ENDIF")
		if err != nil {
			return nil, err
		}
		node.ElseIfs = append(node.ElseIfs, ElseIf{Cond: eicond, Stmts: eistmts})
	}

	if p.peek().Kind == TokIdent && strings.ToUpper(p.peek().Text) == "ELSE" {
		p.next()
		node.Else, err = p.parseStmts("ENDIF")
		if err != nil {
			return nil, err
		}
	}
	p.skipNewlines()
	_ = p.next() // ENDIF
	return node, nil
}

func (p *Parser) parseWhile(line int) (Node, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStmts("WEND")
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	p.next() // WEND
	return &WhileStmt{Cond: cond, Body: body, Line: line}, nil
}

func (p *Parser) parseFor(line int) (Node, error) {
	varName := strings.ToUpper(p.next().Text)
	p.next() // =
	start, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.next() // TO
	stop, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var step Expr = &IntLit{Val: 1}
	if p.peekRaw().Kind == TokIdent && strings.ToUpper(p.peekRaw().Text) == "STEP" {
		p.next()
		step, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseStmts("NEXT")
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	p.next() // NEXT
	// optional: NEXT varName
	if p.peekRaw().Kind == TokIdent {
		p.next()
	}
	return &ForStmt{Var: varName, Start: start, Stop: stop, Step: step, Body: body, Line: line}, nil
}

func (p *Parser) parseProcDecl(line int) (Node, error) {
	name := strings.ToUpper(p.next().Text)
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStmts("ENDPROC", "END")
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	p.next() // ENDPROC or END
	return &ProcDecl{Name: name, Params: params, Body: body, Line: line}, nil
}

func (p *Parser) parseFuncDecl(line int) (Node, error) {
	name := strings.ToUpper(p.next().Text)
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	retType := TypeInteger
	if p.peekRaw().Kind == TokIdent && strings.ToUpper(p.peekRaw().Text) == "AS" {
		p.next()
		retType = typeCodeFromName(p.next().Text)
	}
	body, err := p.parseStmts("ENDFUNC", "END")
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	p.next() // ENDFUNC
	return &FuncDecl{Name: name, Params: params, ReturnType: retType, Body: body, Line: line}, nil
}

func (p *Parser) parseParams() ([]Param, error) {
	if p.peekRaw().Kind != TokLParen {
		return nil, nil
	}
	p.next() // (
	var params []Param
	for p.peekRaw().Kind != TokRParen && p.peekRaw().Kind != TokEOF {
		typTok := p.next()
		nameTok := p.next()
		params = append(params, Param{Name: strings.ToUpper(nameTok.Text), Type: typeCodeFromName(typTok.Text)})
		if p.peekRaw().Kind == TokComma {
			p.next()
		}
	}
	p.next() // )
	return params, nil
}

// ── Expression parsing (Pratt parser) ─────────────────────────────────────────

func (p *Parser) parseArgList() ([]Expr, error) {
	var args []Expr
	for p.peekRaw().Kind != TokNewline && p.peekRaw().Kind != TokEOF && p.peekRaw().Kind != TokColon {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peekRaw().Kind == TokComma {
			p.next()
		} else {
			break
		}
	}
	return args, nil
}

func (p *Parser) parseExprList() ([]Expr, error) {
	var exprs []Expr
	for p.peekRaw().Kind != TokRParen && p.peekRaw().Kind != TokEOF {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, e)
		if p.peekRaw().Kind == TokComma {
			p.next()
		} else {
			break
		}
	}
	return exprs, nil
}

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseBinary(0)
}

func precedence(op string) int {
	switch op {
	case "OR", "||":
		return 1
	case "AND", "&&":
		return 2
	case "=", "<>", "!=", "<", "<=", ">", ">=":
		return 3
	case "+", "-":
		return 4
	case "*", "/", "MOD", "%":
		return 5
	case "^":
		return 6
	}
	return 0
}

func (p *Parser) tokOp() (string, bool) {
	t := p.peekRaw()
	switch t.Kind {
	case TokPlus:
		return "+", true
	case TokMinus:
		return "-", true
	case TokStar:
		return "*", true
	case TokSlash:
		return "/", true
	case TokCaret:
		return "^", true
	case TokPercent:
		return "MOD", true
	case TokEq:
		return "=", true
	case TokNeq:
		return "<>", true
	case TokLt:
		return "<", true
	case TokLte:
		return "<=", true
	case TokGt:
		return ">", true
	case TokGte:
		return ">=", true
	case TokAmpAmp:
		return "AND", true
	case TokPipePipe:
		return "OR", true
	case TokIdent:
		up := strings.ToUpper(t.Text)
		if up == "AND" || up == "OR" || up == "MOD" {
			return up, true
		}
	}
	return "", false
}

func (p *Parser) parseBinary(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := p.tokOp()
		if !ok {
			break
		}
		prec := precedence(op)
		if prec <= minPrec {
			break
		}
		p.next() // consume operator
		right, err := p.parseBinary(prec)
		if err != nil {
			return nil, err
		}
		left = &BinExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Expr, error) {
	t := p.peekRaw()
	if t.Kind == TokMinus {
		p.next()
		val, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Val: val}, nil
	}
	if t.Kind == TokBang || (t.Kind == TokIdent && strings.ToUpper(t.Text) == "NOT") {
		p.next()
		val, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Val: val}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expr, error) {
	t := p.peekRaw()

	switch t.Kind {
	case TokInteger:
		p.next()
		n, _ := strconv.ParseInt(t.Text, 10, 64)
		return &IntLit{Val: n}, nil

	case TokReal:
		p.next()
		f, _ := strconv.ParseFloat(t.Text, 64)
		return &RealLit{Val: f}, nil

	case TokString:
		p.next()
		return &StrLit{Val: t.Text}, nil

	case TokLParen:
		p.next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		p.next() // )
		return e, nil

	case TokIdent:
		p.next()
		name := strings.ToUpper(t.Text)
		if name == "TRUE" {
			return &BoolLit{Val: true}, nil
		}
		if name == "FALSE" {
			return &BoolLit{Val: false}, nil
		}
		// Function call or array index
		if p.peekRaw().Kind == TokLParen {
			p.next() // (
			args, err := p.parseExprList()
			if err != nil {
				return nil, err
			}
			p.next() // )
			// Distinguish built-in function vs array access later in interpreter
			return &CallExpr{Name: name, Args: args}, nil
		}
		return &VarExpr{Name: name}, nil
	}

	return &IntLit{Val: 0}, nil
}

func typeCodeFromName(name string) TypeCode {
	switch strings.ToUpper(name) {
	case "BOOLEAN":
		return TypeBoolean
	case "INTEGER":
		return TypeInteger
	case "REAL":
		return TypeReal
	case "MONEY":
		return TypeMoney
	case "DATE":
		return TypeDate
	case "TIME":
		return TypeTime
	case "BIGSTR":
		return TypeBigStr
	default:
		return TypeString
	}
}

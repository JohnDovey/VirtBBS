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

// Package ppl implements the PCBoard Programming Language (PPL) interpreter.
// VirtBBS executes .PPS source files directly (tree-walking interpreter) and
// can also load/save a VirtBBS-native unencrypted .PPE bytecode format.
//
// Language reference: PCBoard Developer's Guide, PPL 3.x specification.
package ppl

import "fmt"

// Type codes mirror the PCBoard vtXXX constants.
type TypeCode int

const (
	TypeBoolean   TypeCode = 0
	TypeInteger   TypeCode = 4
	TypeMoney     TypeCode = 5
	TypeReal      TypeCode = 6
	TypeString    TypeCode = 7
	TypeBigStr    TypeCode = 13
	TypeDate      TypeCode = 2
	TypeTime      TypeCode = 8
	TypeUnsigned  TypeCode = 1
)

// Value holds a PPL runtime value. PPL is dynamically coercible —
// any type can be converted to any other with defined rules.
type Value struct {
	typ TypeCode
	// Unified storage
	inum int64   // Boolean, Integer, Unsigned, Money, Date, Time
	fnum float64 // Real
	str  string  // String, BigStr
}

var (
	True  = Value{typ: TypeBoolean, inum: 1}
	False = Value{typ: TypeBoolean, inum: 0}
	Zero  = Value{typ: TypeInteger, inum: 0}
	Empty = Value{typ: TypeString, str: ""}
)

func IntVal(n int64) Value   { return Value{typ: TypeInteger, inum: n} }
func StrVal(s string) Value  { return Value{typ: TypeString, str: s} }
func BoolVal(b bool) Value {
	if b {
		return True
	}
	return False
}
func RealVal(f float64) Value { return Value{typ: TypeReal, fnum: f} }

// ToInt coerces a Value to int64.
func (v Value) ToInt() int64 {
	switch v.typ {
	case TypeBoolean, TypeInteger, TypeUnsigned, TypeMoney, TypeDate, TypeTime:
		return v.inum
	case TypeReal:
		return int64(v.fnum)
	case TypeString, TypeBigStr:
		var n int64
		fmt.Sscanf(v.str, "%d", &n)
		return n
	}
	return 0
}

// ToFloat coerces a Value to float64.
func (v Value) ToFloat() float64 {
	switch v.typ {
	case TypeReal:
		return v.fnum
	case TypeString, TypeBigStr:
		var f float64
		fmt.Sscanf(v.str, "%f", &f)
		return f
	}
	return float64(v.ToInt())
}

// ToString coerces a Value to string.
func (v Value) ToString() string {
	switch v.typ {
	case TypeString, TypeBigStr:
		return v.str
	case TypeBoolean:
		if v.inum != 0 {
			return "TRUE"
		}
		return "FALSE"
	case TypeReal:
		return fmt.Sprintf("%g", v.fnum)
	default:
		return fmt.Sprintf("%d", v.inum)
	}
}

// ToBool coerces a Value to bool.
func (v Value) ToBool() bool {
	switch v.typ {
	case TypeBoolean:
		return v.inum != 0
	case TypeString, TypeBigStr:
		return v.str != "" && v.str != "FALSE" && v.str != "0"
	case TypeReal:
		return v.fnum != 0
	default:
		return v.inum != 0
	}
}

func (v Value) Type() TypeCode { return v.typ }

func (v Value) String() string { return v.ToString() }

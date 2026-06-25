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

// Package pcbformat decodes PCBoard 15.x binary file formats.
package pcbformat

import (
	"math"
)

// Float4ToFloat64 converts a 4-byte PCBoard BASIC single-precision float to float64.
// PCBoard stores many numeric values (message numbers, byte counts) as 4-byte
// IEEE 754 single-precision floats in little-endian byte order, matching the
// Microsoft BASIC MKS$/CVS format used in the original DOS implementation.
func Float4ToFloat64(b [4]byte) float64 {
	bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return float64(math.Float32frombits(bits))
}

// Float64ToFloat4 converts a float64 to PCBoard's 4-byte BASIC single-precision format.
func Float64ToFloat4(f float64) [4]byte {
	bits := math.Float32bits(float32(f))
	return [4]byte{
		byte(bits),
		byte(bits >> 8),
		byte(bits >> 16),
		byte(bits >> 24),
	}
}

// Float4ToInt converts a PCBoard 4-byte float directly to an integer.
func Float4ToInt(b [4]byte) int {
	return int(Float4ToFloat64(b))
}

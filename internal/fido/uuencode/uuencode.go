// Package uuencode implements classic Unix uuencode/uudecode for FidoNet
// message-embedded file attachments (begin … end blocks).
package uuencode

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// DecodedFile is one file extracted from a message body.
type DecodedFile struct {
	Filename string
	Mode     uint32
	Data     []byte
}

// Decode scans body for uuencode blocks and returns decoded files plus the
// body with those blocks removed.
func Decode(body string) (files []DecodedFile, cleanBody string, err error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\r"), "\r")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\n")
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trimmed), "begin ") {
			out = append(out, line)
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 3 {
			out = append(out, line)
			continue
		}
		mode, parseErr := strconv.ParseUint(parts[1], 8, 32)
		if parseErr != nil {
			return nil, "", fmt.Errorf("uuencode: invalid mode in %q", trimmed)
		}
		name := parts[2]
		var data []byte
		i++
		for i < len(lines) {
			enc := strings.TrimRight(lines[i], "\n")
			i++
			if strings.TrimSpace(enc) == "end" || strings.TrimSpace(enc) == "`" {
				break
			}
			if enc == "" {
				continue
			}
			chunk, decErr := decodeLine(enc)
			if decErr != nil {
				return nil, "", decErr
			}
			data = append(data, chunk...)
		}
		files = append(files, DecodedFile{
			Filename: name,
			Mode:     uint32(mode),
			Data:     data,
		})
	}
	return files, strings.Join(out, "\r"), nil
}

// Encode returns a uuencode block for data with the given filename.
func Encode(data []byte, filename string) string {
	if filename == "" {
		filename = "file.dat"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "begin 644 %s\r", filename)
	for off := 0; off < len(data) || (off == 0 && len(data) == 0); {
		n := 45
		if off+n > len(data) {
			n = len(data) - off
		}
		if n == 0 && off > 0 {
			break
		}
		chunk := data[off : off+n]
		sb.WriteString(encodeLine(chunk))
		sb.WriteString("\r")
		off += n
		if len(data) == 0 {
			break
		}
	}
	sb.WriteString("end\r")
	return sb.String()
}

func encodeLine(in []byte) string {
	var buf bytes.Buffer
	buf.WriteByte(byte(len(in) + 32))
	for i := 0; i < len(in); i += 3 {
		var b0, b1, b2 byte
		b0 = in[i]
		if i+1 < len(in) {
			b1 = in[i+1]
		}
		if i+2 < len(in) {
			b2 = in[i+2]
		}
		buf.WriteByte(byte(((b0 >> 2) & 0x3F) + 32))
		buf.WriteByte(byte((((b0 & 0x03) << 4) | ((b1 >> 4) & 0x0F)) + 32))
		buf.WriteByte(byte((((b1 & 0x0F) << 2) | ((b2 >> 6) & 0x03)) + 32))
		buf.WriteByte(byte((b2 & 0x3F) + 32))
	}
	return buf.String()
}

func decodeLine(line string) ([]byte, error) {
	if line == "" {
		return nil, nil
	}
	if line[0] < 32 {
		return nil, fmt.Errorf("uuencode: bad line length")
	}
	count := int(line[0]) - 32
	if count < 0 {
		return nil, fmt.Errorf("uuencode: negative line length")
	}
	payload := line[1:]
	out := make([]byte, 0, count)
	for i := 0; len(out) < count && i+3 < len(payload); i += 4 {
		if i+3 >= len(payload) {
			break
		}
		c0 := payload[i] - 32
		c1 := payload[i+1] - 32
		c2 := payload[i+2] - 32
		c3 := payload[i+3] - 32
		out = append(out, byte((c0<<2)|((c1>>4)&0x03)))
		if len(out) < count {
			out = append(out, byte((c1<<4)|((c2>>2)&0x0F)))
		}
		if len(out) < count {
			out = append(out, byte((c2<<6)|(c3&0x3F)))
		}
	}
	if len(out) > count {
		out = out[:count]
	}
	return out, nil
}

// AppendToBody appends uuencoded blocks after message text.
func AppendToBody(body string, files []DecodedFile) string {
	if len(files) == 0 {
		return body
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimRight(body, "\r\n"))
	sb.WriteString("\r\n\r\n")
	for _, f := range files {
		sb.WriteString(Encode(f.Data, f.Filename))
		sb.WriteString("\r")
	}
	return sb.String()
}

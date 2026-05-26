package toml

import "unicode/utf8"

func countRun(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func lineCol(s string, pos int) (int, int) {
	line, col := 1, 1
	for i := 0; i < len(s) && i < pos; {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i += size
	}
	return line, col
}

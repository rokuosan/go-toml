package toml

import "fmt"

type parseError struct {
	input string
	pos   int
	msg   string
}

func (e *parseError) Error() string {
	line, col := lineCol(e.input, e.pos)
	return fmt.Sprintf("toml: %s at line %d, column %d", e.msg, line, col)
}

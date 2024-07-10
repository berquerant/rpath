package rpath

import (
	"errors"
	"fmt"
	"io"

	"github.com/berquerant/ybase"
)

type Queryer interface {
	// Query finds the path of the element present at specified position.
	Query(r io.Reader, p *Position) (*Result, error)
}

type Position struct {
	// Line number, 1-based.
	Line int `json:"line"`
	// Column number, 1-based.
	Column int `json:"column"`
	// Offset of document, 0-based.
	Offset int `json:"offset"`
}

func (p *Position) Fill(b ybase.Bytes) error {
	if p.Line < 1 || p.Column < 1 {
		l, c, ok := b.LineColumn(p.Offset)
		if !ok {
			return fmt.Errorf("%w: out of range offset %d", ErrNotFound, p.Offset)
		}
		p.Line, p.Column = l, c
		return nil
	}

	offset, ok := b.Offset(p.Line, p.Column)
	if !ok {
		return fmt.Errorf("%w: out of range line %d column %d", ErrNotFound, p.Line, p.Column)
	}
	p.Offset = offset
	return nil
}

type Result struct {
	Position *Position `json:"pos"`
	Path     string    `json:"path"`
	Left     any       `json:"left"`
	Right    any       `json:"right"`
	Meta     any       `json:"meta"`
}

func inRange(target, left, right int) bool {
	return left <= target && target <= right
}

var (
	ErrNotFound = errors.New("NotFound")
)

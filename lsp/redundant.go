package lsp

import "fmt"

type position struct {
	l int
	s int
}

type RedundantLine interface {
	Line() int
	Subline() int
	String() string
}

type RedundantLabelLine struct {
	p    position
	name string
}

func (l RedundantLabelLine) Line() int {
	return l.p.l
}

func (l RedundantLabelLine) Subline() int {
	return l.p.s
}

func (l RedundantLabelLine) String() string {
	return fmt.Sprintf("Remove label '%s'", l.name)
}

type RedundantBLine struct {
	p position
}

func (l RedundantBLine) Line() int {
	return l.p.l
}

func (l RedundantBLine) Subline() int {
	return l.p.s
}

func (l RedundantBLine) String() string {
	return "Remove b call"
}

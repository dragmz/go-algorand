package lsp

import (
	"fmt"
)

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

type Linter struct {
	l [][]Op

	reds []RedundantLine
}

func (l *Linter) forEachForward(line, sub int, cb func(line int, sub int) bool) {
	first := true
	for i := line; i < len(l.l); i++ {
		if first {
			first = false
		} else {
			sub = 0
		}
		for j := sub; j < len(l.l[i]); j++ {
			if !cb(i, j) {
				return
			}
		}
	}
}

func (l *Linter) getLabelsUsers() map[string][]position {
	used := map[string][]position{}

	for i, lo := range l.l {
		for j, o := range lo {
			switch o2 := o.(type) {
			case usesLabels:
				for _, l := range o2.Labels() {
					used[l.Name] = append(used[l.Name], position{l: i, s: j})
				}
			}
		}
	}

	return used
}

func (l *Linter) getAllLabels() map[string][]position {
	all := map[string][]position{}

	for i, lo := range l.l {
		for j, o := range lo {
			switch o := o.(type) {
			case *LabelExpr:
				all[o.Name] = append(all[o.Name], position{l: i, s: j})
			}
		}
	}

	return all
}

type lintRule interface {
	Run(l *Linter)
}

type UnusedLabelsRule struct{}

func (r UnusedLabelsRule) Run(l *Linter) {
	used := l.getLabelsUsers()
	for name, positions := range l.getAllLabels() {
		if len(used[name]) == 0 {
			for _, p := range positions {
				l.reds = append(l.reds, &RedundantLabelLine{p: p, name: name})
			}
		}
	}
}

type CheckBranchJustBeforeLabelRule struct{}

func (r CheckBranchJustBeforeLabelRule) Run(l *Linter) {
	l.forEachForward(0, 0, func(i, j int) bool {
		o := l.l[i][j]
		switch o := o.(type) {
		case *BExpr:
			l.forEachForward(i, j+1, func(i2, j2 int) bool {
				o2 := l.l[i2][j2]
				switch o2 := o2.(type) {
				case *LabelExpr:
					if o2.Name == o.Label.Name {
						l.reds = append(l.reds, RedundantBLine{p: position{l: i, s: j}})
						return true
					}
					return false
				case Nop:
				default:
					return false
				}
				return true
			})
		}
		return true
	})
}

var LintRules = []lintRule{
	UnusedLabelsRule{},
	CheckBranchJustBeforeLabelRule{},
}

func (l *Linter) Lint() {
	for _, r := range LintRules {
		r.Run(l)
	}
}

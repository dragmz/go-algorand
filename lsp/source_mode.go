package lsp

import (
	"strings"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func sourceModeForLSP(lines []logic.SourceLine) logic.RunMode {
	for _, line := range lines {
		if line.Comment != nil && strings.TrimSpace(line.Comment.Text) == "#pragma mode logicsig" {
			return logic.ModeSig
		}
	}
	return logic.ModeApp
}

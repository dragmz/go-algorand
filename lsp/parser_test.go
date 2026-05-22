package lsp

import (
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
)

func TestDocs(t *testing.T) {
	i, ok := logic.ToolOpcodeForTools("txn", 1, logic.ModeApp)

	if !ok {
		t.Error("txn not found")
	}

	if i.Name != "txn" {
		t.Error("unexpected name")
	}
}

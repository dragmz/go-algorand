package lsp

import (
	"testing"
)

func TestDocs(t *testing.T) {
	i, ok := Ops.Get(OpContext{
		Name:    "txn",
		Version: 9,
	})

	if !ok {
		t.Error("txn not found")
	}

	if i.Name != "txn" {
		t.Error("unexpected name")
	}
}

package lsp

import (
	"fmt"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/stretchr/testify/assert"
)

type testRange struct {
	sl int
	sc int
	el int
	ec int
}

func (r testRange) StartLine() int {
	return r.sl
}

func (r testRange) StartCharacter() int {
	return r.sc
}

func (r testRange) EndLine() int {
	return r.el
}

func (r testRange) EndCharacter() int {
	return r.ec
}

func TestProcessEmpty(t *testing.T) {
	res := Process("")

	assert.Equal(t, logic.ModeApp, res.Mode)
	assert.Equal(t, uint64(1), res.Version)

	assert.Equal(t, 0, len(res.SourceIndex.MissingReferences))
	assert.Equal(t, 0, len(res.SourceIndex.Redundants))
	assert.Equal(t, 0, len(res.SourceIndex.RefCounts))
	assert.Equal(t, 0, len(res.SourceLines))
	assert.Equal(t, 0, len(res.SourceProgram.Operations))
	assert.Equal(t, 0, len(res.SourceProgram.RequiredVersions))
	assert.Equal(t, 0, len(res.SourceProgram.TokenClasses))
	assert.Equal(t, 0, len(res.SourceIndex.References))
	assert.Equal(t, 0, len(res.SourceIndex.Symbols))
}

func TestRedundantLabelLine(t *testing.T) {
	res := Process("test_label:")

	assert.Len(t, res.SourceIndex.Redundants, 1)

	r := res.SourceIndex.Redundants[0]

	assert.Equal(t, 0, r.Line)
	assert.Equal(t, "Remove label 'test_label'", r.Message)
}

func TestRedundantBCallLine(t *testing.T) {
	res := Process("b a\na:")

	if len(res.SourceIndex.Redundants) != 1 {
		t.Error("len mismatch")
	}

	r := res.SourceIndex.Redundants[0]

	assert.Equal(t, 0, r.Line)
	assert.Equal(t, "Remove b call", r.Message)
}

func TestIntArgVals(t *testing.T) {
	res := Process("int ")

	arg, _, ok := logic.SourceToolArgAtForTools(res.SourceLines, res.SourceProgram, 0, res.sourceColumn(0, 4))
	if !assert.True(t, ok) {
		return
	}
	vals := res.ArgVals(arg)

	m := map[string]bool{}
	for _, v := range vals {
		m[v.Name] = true
	}

	if _, ok := m["DeleteApplication"]; !ok {
		t.Error("missing DeleteApplication")
	}
}

func TestSourceIdentifierAt(t *testing.T) {
	res := Process(`test_label:
test_label2:
b test_label
`)

	type test struct {
		i Range
		o string
	}

	tests := []test{
		{testRange{}, "test_label"},
		{testRange{1, 1, 1, 1}, "test_label2"},
		{testRange{2, 2, 2, 2}, "test_label"},
	}

	for i, test := range tests {
		column := res.sourceColumn(test.i.StartLine(), test.i.StartCharacter())
		identifier, ok := logic.SourceIdentifierAtForTools(res.SourceIndex, test.i.StartLine(), column)
		if assert.True(t, ok, fmt.Sprintf("test #%d", i)) {
			assert.Equal(t, test.o, identifier.Name, fmt.Sprintf("test #%d", i))
		}
	}
}

func TestOverlaps(t *testing.T) {
	type test struct {
		a Range
		b Range

		o bool
	}

	tests := []test{
		{testRange{1, 1, 1, 2}, testRange{0, 3, 1, 3}, true},

		{testRange{1, 1, 1, 2}, testRange{0, 0, 1, 0}, false},
		{testRange{1, 1, 1, 2}, testRange{0, 2, 1, 2}, true},

		{testRange{1, 1, 1, 2}, testRange{1, 0, 1, 0}, false},
		{testRange{0, 5, 0, 11}, testRange{0, 0, 1, 0}, true},

		{testRange{}, testRange{}, true},
		{testRange{}, testRange{0, 1, 0, 1}, false},
		{testRange{0, 1, 0, 1}, testRange{0, 1, 0, 1}, true},
		{testRange{1, 1, 1, 1}, testRange{1, 1, 1, 1}, true},
		{testRange{1, 1, 1, 2}, testRange{1, 1, 1, 1}, true},
		{testRange{1, 1, 1, 2}, testRange{1, 2, 1, 2}, true},
		{testRange{1, 1, 1, 2}, testRange{1, 3, 1, 3}, false},
		{testRange{1, 1, 1, 2}, testRange{0, 1, 1, 1}, true},
		{testRange{1, 1, 1, 2}, testRange{0, 1, 0, 1}, false},
		{testRange{1, 1, 1, 2}, testRange{0, 2, 0, 2}, false},
		{testRange{1, 1, 1, 2}, testRange{0, 0, 0, 0}, false},
		{testRange{1, 1, 1, 2}, testRange{0, 3, 0, 3}, false},
		{testRange{1, 1, 1, 2}, testRange{2, 1, 2, 1}, false},
		{testRange{1, 1, 1, 2}, testRange{2, 2, 2, 2}, false},
		{testRange{1, 1, 1, 2}, testRange{2, 0, 2, 0}, false},
		{testRange{1, 1, 1, 2}, testRange{2, 3, 2, 3}, false},
	}

	for i, test := range tests {
		o := Overlaps(test.a, test.b)
		assert.Equal(t, test.o, o, fmt.Sprintf("test #%d", i))
	}
}

func TestInlayHints(t *testing.T) {
	tests := []string{
		"byte 0x3031",
		"byte 0x3031\n",
	}

	for i, ts := range tests {
		name := fmt.Sprintf("test #%d", i)

		res := Process(ts)
		ihs := logic.SourceInlayHintsForTools(res.SourceLines, res.SourceProgram)

		var decoded []logic.SourceInlayHint
		for _, hint := range ihs {
			if hint.Kind == logic.SourceInlayHintDecoded {
				decoded = append(decoded, hint)
			}
		}
		if !assert.Equal(t, 1, len(decoded), name) {
			return
		}
		assert.Equal(t, "01", decoded[0].Label, name)
	}
}

func TestRefCounts(t *testing.T) {
	res := Process("b a\nb a\nb a\na:")
	assert.Equal(t, 3, res.SourceIndex.RefCounts["a"])
}

func TestVersion(t *testing.T) {
	res := Process("#pragma version 8")
	assert.Equal(t, uint64(8), res.Version)
}

func TestRequiredVersion(t *testing.T) {
	res := Process("box_create")
	assert.Len(t, res.SourceProgram.RequiredVersions, 1)

	v := res.SourceProgram.RequiredVersions[0]
	assert.Equal(t, uint64(8), v.Version)
}

func TestInvalidByteInt(t *testing.T) {
	res := Process(`#pragma version 8
	byte \"test\"int 123
	int 1
	int 2`)

	assert.Len(t, res.SourceLines, 4)
	assert.Len(t, res.SourceProgram.Operations, 4)
}

func TestSemicolon(t *testing.T) {
	res := Process("int 1; int 2")
	assert.Len(t, res.SourceLines, 1)

	assert.Len(t, res.SourceLines[0].Statements, 2)
	assert.Len(t, res.SourceLines[0].Statements[0].Tokens, 2)
	assert.Len(t, res.SourceLines[0].Statements[1].Tokens, 2)

	assert.Len(t, res.SourceLines[0].Tokens, 5)
}

func TestSemicolonEmptySubs(t *testing.T) {
	res := Process("int 1;")
	assert.Len(t, res.SourceLines[0].Statements, 2)
}

func TestMultiSemicolon(t *testing.T) {
	res := Process(";;;")
	assert.Len(t, res.SourceLines, 1)

	assert.Len(t, res.SourceLines[0].Statements, 4)
	assert.Len(t, res.SourceLines[0].Statements[0].Tokens, 0)
	assert.Len(t, res.SourceLines[0].Statements[1].Tokens, 0)
	assert.Len(t, res.SourceLines[0].Statements[2].Tokens, 0)
	assert.Len(t, res.SourceLines[0].Statements[3].Tokens, 0)

	assert.Len(t, res.SourceLines[0].Tokens, 3)
}

func TestAssemblerSourceTokenizationRegressions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		tokens []string
	}{
		{
			name:   "base64 with comment marker",
			source: `byte base64(ABC//==) // comment`,
			tokens: []string{"byte", "base64(ABC//==)"},
		},
		{
			name:   "string with comment marker",
			source: `byte "foo bar // not a comment" // comment`,
			tokens: []string{"byte", `"foo bar // not a comment"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := Process(test.source)
			if !assert.Len(t, res.SourceLines, 1) {
				return
			}
			if !assert.Len(t, res.SourceLines[0].Statements, 1) {
				return
			}
			var got []string
			for _, token := range res.SourceLines[0].Statements[0].Tokens {
				got = append(got, token.Text)
			}
			assert.Equal(t, test.tokens, got)
		})
	}
}

func TestGithubIssueVsCodeTeal3Regression(t *testing.T) {
	Process(`int 1 /
	b a`)
}

func TestBranchToSameLine(t *testing.T) {
	Process("a:;b a")
}

func TestLogicSigMode(t *testing.T) {
	res := Process(`//#pragma mode logicsig`)

	assert.Equal(t, logic.ModeSig, res.Mode)
}

func TestAppMode(t *testing.T) {
	req := Process(``)
	assert.Equal(t, logic.ModeApp, req.Mode)
}

func getAvailableOps(source string) []string {
	res := Process(source)

	available := []string{}

	for _, item := range res.AvailableOps() {
		available = append(available, item.Name)
	}

	return available
}

func TestCompletion(t *testing.T) {
	defaultVApp := getAvailableOps(``)

	assert.NotContains(t, defaultVApp, "args")
	assert.NotContains(t, defaultVApp, "addw")
	assert.Contains(t, defaultVApp, "err")

	v2App := getAvailableOps(`#pragma version 2`)

	assert.Contains(t, v2App, "addw")

	defaultVSig := getAvailableOps(`//#pragma mode logicsig`)
	v4Sig := getAvailableOps(`#pragma version 4; //#pragma mode logicsig`)

	assert.NotContains(t, v4Sig, "args")

	assert.NotContains(t, defaultVSig, "args")

	v5Sig := getAvailableOps(`#pragma version 5; //#pragma mode logicsig`)

	assert.Contains(t, v5Sig, "args")
}

func TestPragmaTypetrackTrue(t *testing.T) {
	res := Process(`#pragma typetrack true`)

	assert.Len(t, sourceTokensByClass(res, logic.SourceTokenClassMacro), 2)
	assert.Equal(t, 1, len(sourceTokensByClass(res, logic.SourceTokenClassBool)))
}

func TestPragmaTypetrackFalse(t *testing.T) {
	res := Process(`#pragma typetrack false`)

	assert.Len(t, sourceTokensByClass(res, logic.SourceTokenClassMacro), 2)
	assert.Equal(t, 1, len(sourceTokensByClass(res, logic.SourceTokenClassBool)))
}

func TestPragmaTypetrackInvalid(t *testing.T) {
	res := Process(`#pragma typetrack test`)
	assert.Empty(t, sourceTokensByClass(res, logic.SourceTokenClassBool))
}

func TestDefineRef(t *testing.T) {
	res := Process(`#pragma version 8
	#define VALUE 123
	VALUE`)

	assert.Len(t, res.SourceIndex.Symbols, 1)
	assert.Equal(t, "VALUE", res.SourceIndex.Symbols[0].Name)
	assert.Equal(t, 1, res.SourceIndex.Symbols[0].Line)

	assert.Len(t, res.SourceIndex.References, 1)
	assert.Equal(t, "VALUE", res.SourceIndex.References[0].Name)
	assert.Equal(t, 2, res.SourceIndex.References[0].Line)
}

func TestDefineValueRef(t *testing.T) {
	res := Process(`#pragma version 8
	#define VALUE 123
	int VALUE`)

	assert.Len(t, res.SourceIndex.Symbols, 1)
	assert.Equal(t, "VALUE", res.SourceIndex.Symbols[0].Name)
	assert.Equal(t, 1, res.SourceIndex.Symbols[0].Line)

	assert.Len(t, res.SourceIndex.References, 1)
	assert.Equal(t, "VALUE", res.SourceIndex.References[0].Name)
	assert.Equal(t, 2, res.SourceIndex.References[0].Line)
}

func TestStringDoesConflictWithDefine(t *testing.T) {
	res := Process(`#pragma version 8
	#define VALUE "test"
	byte "VALUE"`)

	assert.Len(t, sourceTokensByClass(res, logic.SourceTokenClassString), 1)
	assert.Len(t, res.SourceIndex.Symbols, 1)
	assert.Empty(t, res.SourceIndex.References)
}

func TestEmojiLabelAndBranch(t *testing.T) {
	emojiCases := []string{
		"👍",
		"👍👍",
		"👋👋",
		"🤝",
		"🧑🏽‍🔧",    // person with skin tone + ZWJ
		"👩‍👩‍👧‍👦", // family (ZWJ sequence)
		"🏳️‍🌈",    // flag (ZWJ + VS16)
		"🇺🇸",      // regional indicator pair (flag)
	}

	for _, name := range emojiCases {
		name := name
		t.Run(name, func(t *testing.T) {
			src := name + ":\n" + "b " + name + "\n" + "byte \"" + name + "\"\n"

			res := Process(src)

			// symbol detected
			if !assert.Len(t, res.SourceIndex.Symbols, 1) {
				return
			}
			sym := res.SourceIndex.Symbols[0]
			assert.Equal(t, name, sym.Name)

			// branch reference detected
			refs := logic.SourceReferencesByNameForTools(res.SourceIndex, name)
			if !assert.Len(t, refs, 1) {
				return
			}
			assert.Equal(t, name, refs[0].Name)

			// string token captured separately
			strings := sourceTokensByClass(res, logic.SourceTokenClassString)
			if !assert.Len(t, strings, 1) {
				return
			}
			assert.Equal(t, "\""+name+"\"", strings[0].Text)

			// positions: source index columns are byte offsets.
			assert.Equal(t, len(name+":"), sym.EndColumn)
		})
	}
}

func sourceTokensByClass(res *ProcessResult, kind logic.SourceTokenClassKind) []logic.SourceToken {
	var tokens []logic.SourceToken
	for _, class := range res.SourceProgram.TokenClasses {
		if class.Kind == kind {
			tokens = append(tokens, class.Token)
		}
	}
	return tokens
}

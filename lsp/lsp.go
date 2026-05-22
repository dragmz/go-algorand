package lsp

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"strings"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/pkg/errors"
)

const (
	semanticTokenKeyword  = 0
	semanticTokenString   = 1
	semanticTokenComment  = 2
	semanticTokenMethod   = 3
	semanticTokenMacro    = 4
	semanticTokenValue    = 5
	semanticTokenNumber   = 6
	semanticTokenOperator = 7
	semanticTokenFunction = 8
)

type lspDoc struct {
	s   string
	res *logic.SourceAnalysisResult
}

func (d *lspDoc) Update(s string) {
	d.s = s
	d.res = nil
}

func (d *lspDoc) Results() *logic.SourceAnalysisResult {
	if d.res == nil {
		d.res = Process(d.s)
	}

	return d.res
}

type lsp struct {
	id int

	config tealConfig

	docs     map[string]*lspDoc
	shutdown bool

	exit     bool
	exitCode int

	tp *textproto.Reader
	w  *bufio.Writer

	debug *bufio.Writer
}

type LspOption func(l *lsp) error

func WithDebug(w io.Writer) LspOption {
	return func(l *lsp) error {
		l.debug = bufio.NewWriter(w)
		return nil
	}
}

func New(r io.Reader, w io.Writer, opts ...LspOption) (*lsp, error) {
	l := &lsp{
		tp:   textproto.NewReader(bufio.NewReader(r)),
		w:    bufio.NewWriter(w),
		docs: map[string]*lspDoc{},
		config: tealConfig{
			SemanticTokens: true,
			InlayNamed:     true,
			InlayDecoded:   true,
			LensRefs:       true,
			ProgramSize:    true,
			PcLens:         false,
			PcInlay:        false,
		},
	}

	for _, opt := range opts {
		err := opt(l)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set lsp option")
		}
	}

	return l, nil
}

type jsonRpcRequest struct {
	JsonRpc string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	ErrorCodeServerNotInitialized = -32002
	ErrorCodeRequestFailed        = -32803
	ErrorCodeServerCancelled      = -32802
	ErrorCodeContentModified      = -32801
	ErrorCodeRequestCancelled     = -32800
)

type jsonRpcHeader struct {
	JsonRpc string `json:"jsonrpc"`

	Id interface{} `json:"id"`

	Method string `json:"method"`

	Result interface{} `json:"result,omitempty"`
	Error  interface{} `json:"error,omitempty"`
}

type jsonRpcResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Id      interface{} `json:"id"`
}

type lspFullDocumentDiagnosticReport struct {
	Kind  string          `json:"kind"`
	Items []LspDiagnostic `json:"items"`
}

type lspRequest[T any] struct {
	Params T `json:"params"`
}

type lspDiagnosticProvider struct {
	InterFileDependencies bool `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
}

type lspCompletionProviderItem struct {
	LabelDetailsSupport *bool `json:"labelDetailsSupport,omitempty"`
}

type lspCompletionItemLabelDetails struct {
	Detail      string `json:"detail,omitempty"`
	Description string `json:"description,omitempty"`
}

type lspCompletionItem struct {
	Label        string                         `json:"label"`
	LabelDetails *lspCompletionItemLabelDetails `json:"labelDetails,omitempty"`
	Kind         *int                           `json:"kind,omitempty"`
	Detail       string                         `json:"detail,omitempty"`

	// string | MarkupContent
	Documentation interface{} `json:"documentation,omitempty"`

	Deprecated          *bool         `json:"deprecated,omitempty"`
	Preselect           *bool         `json:"preselect,omitempty"`
	SortText            string        `json:"sortText,omitempty"`
	FilterText          string        `json:"filterText,omitempty"`
	InsertText          string        `json:"insertText,omitempty"`
	InsertTextFormat    *int          `json:"insertTextFormat,omitempty"`
	InsertTextMode      *int          `json:"insertTextMode,omitempty"`
	TextEdit            *lspTextEdit  `json:"textEdit,omitempty"`
	TextEditText        string        `json:"textEditText,omitempty"`
	AdditionalTextEdits []lspTextEdit `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string      `json:"commitCharacters,omitempty"`
	Command             *LspCommand   `json:"command,omitempty"`
	Data                interface{}   `json:"data,omitempty"`
}

type lspCompletionList struct {
	IsIncomplete bool                `json:"isIncomplete"`
	Items        []lspCompletionItem `json:"items"`
}

type lspCompletionProvider struct {
	TriggerCharacters   []string                   `json:"triggerCharacters,omitempty"`
	AllCommitCharacters []string                   `json:"allCommitCharacters,omitempty"`
	ResolveProvider     *bool                      `json:"resolveProvider,omitempty"`
	CompletionItem      *lspCompletionProviderItem `json:"completionItem,omitempty"`
}

type lspExecuteCommandProvider struct {
	Commands []string `json:"commands"`
}

type lspError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type lspSemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type lspSemanticTokensProvider struct {
	Legend lspSemanticTokensLegend `json:"legend"`
	Range  *bool                   `json:"range"`
	Full   *bool                   `json:"full"`
}

type lspCodeLensProvider struct {
	ResolveProvider *bool `json:"resolveProvider,omitempty"`
}

type lspSemanticTokens struct {
	Data []uint32 `json:"data"`
}

type lspSignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

type lspParameterInformation struct {
	Label string `json:"label"`

	// string | [uinteger, uinteger]
	Documentation interface{} `json:"documentation,omitempty"`
}

type lspSignatureInformation struct {
	Label string `json:"label"`

	//string | Markupcontent
	Documentation interface{} `json:"documentation,omitempty"`

	Parameters      []lspParameterInformation `json:"parameters,omitempty"`
	ActiveParameter *int                      `json:"activeParameter,omitempty"`
}

type lspSignatureHelp struct {
	Signatures []lspSignatureInformation `json:"signatures,omitempty"`
}

type lspServerCapabilities struct {
	TextDocumentSync          *int                       `json:"textDocumentSync,omitempty"`
	DiagnosticProvider        *lspDiagnosticProvider     `json:"diagnosticProvider,omitempty"`
	CompletionProvider        *lspCompletionProvider     `json:"completionProvider,omitempty"`
	DocumentSymbolProvider    *bool                      `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider        *bool                      `json:"codeActionProvider,omitempty"`
	ExecuteCommandProvider    *lspExecuteCommandProvider `json:"executeCommandProvider,omitempty"`
	RenameProvider            *lspRenameOptions          `json:"renameProvider,omitempty"`
	ColorProvider             *bool                      `json:"colorProvider,omitempty"`
	DocumentHighlightProvider *bool                      `json:"documentHighlightProvider,omitempty"`
	SemanticTokensProvider    *lspSemanticTokensProvider `json:"semanticTokensProvider,omitempty"`
	DefinitionProvider        *bool                      `json:"definitionProvider,omitempty"`
	HoverProvider             *bool                      `json:"hoverProvider,omitempty"`
	SignatureHelpProvider     *lspSignatureHelpOptions   `json:"signatureHelpProvider,omitempty"`
	InlayHintProvider         *bool                      `json:"inlayHintProvider,omitempty"`
	CodeLensProvider          *lspCodeLensProvider       `json:"codeLensProvider,omitempty"`
}

type lspInitializeResult struct {
	Capabilities *lspServerCapabilities `json:"capabilities"`
}

type LspSymbolKind int

const (
	LspSymbolKindFile          = 1
	LspSymbolKindModule        = 2
	LspSymbolKindNamespace     = 3
	LspSymbolKindPackage       = 4
	LspSymbolKindClass         = 5
	LspSymbolKindMethod        = 6
	LspSymbolKindProperty      = 7
	LspSymbolKindField         = 8
	LspSymbolKindConstructor   = 9
	LspSymbolKindEnum          = 10
	LspSymbolKindInterface     = 11
	LspSymbolKindFunction      = 12
	LspSymbolKindVariable      = 13
	LspSymbolKindConstant      = 14
	LspSymbolKindString        = 15
	LspSymbolKindNumber        = 16
	LspSymbolKindBoolean       = 17
	LspSymbolKindArray         = 18
	LspSymbolKindObject        = 19
	LspSymbolKindKey           = 20
	LspSymbolKindNull          = 21
	LspSymbolKindEnumMember    = 22
	LspSymbolKindStruct        = 23
	LspSymbolKindEvent         = 24
	LspSymbolKindOperator      = 25
	LspSymbolKindTypeParameter = 26
)

type LspDocumentSymbol struct {
	Name           string        `json:"name"`
	Kind           LspSymbolKind `json:"kind"`
	Range          LspRange      `json:"range"`
	SelectionRange LspRange      `json:"selectionRange"`
}

type lspInitializeClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type tealInitializationOptions struct {
	SemanticTokens *bool `json:"semanticTokens,omitempty"`
	InlayNamed     *bool `json:"inlayNamed,omitempty"`
	InlayDecoded   *bool `json:"inlayDecoded,omitempty"`
	LensRefs       *bool `json:"lensRefs,omitempty"`
	PcLens         *bool `json:"pcLens,omitempty"`
	PcInlay        *bool `json:"pcInlay,omitempty"`
	ProgramSize    *bool `json:"programSize,omitempty"`
}

type tealConfig struct {
	SemanticTokens bool
	InlayNamed     bool
	InlayDecoded   bool
	LensRefs       bool
	PcLens         bool
	PcInlay        bool
	ProgramSize    bool
}

type lspCompletionCompletionItemClientCapabilities struct {
	SnippetSupport          *bool `json:"snippetSupport,omitempty"`
	CommitCharactersSupport *bool `json:"commitCharactersSupport,omitempty"`
}

type lspCompletionTagSupportClientCapabilities struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type lspCompletionInsertTextModeSupportClientCapabilities struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type lspCompletionClientCapabilities struct {
	DynamicRegistration   *bool                                                 `json:"dynamicRegistration,omitempty"`
	CompletionItem        *lspCompletionCompletionItemClientCapabilities        `json:"completionItem,omitempty"`
	DocumentationFormat   []string                                              `json:"documentationFormat,omitempty"`
	DeprecatedSupport     *bool                                                 `json:"deprecatedSupport,omitempty"`
	PreselectSupport      *bool                                                 `json:"preselectSupport,omitempty"`
	TagSupport            *lspCompletionTagSupportClientCapabilities            `json:"tagSupport,omitempty"`
	InsertReplaceSupport  *bool                                                 `json:"insertReplaceSupport,omitempty"`
	ResolveSupport        []string                                              `json:"resolveSupport,omitempty"`
	InsertTextModeSupport *lspCompletionInsertTextModeSupportClientCapabilities `json:"insertTextModeSupport,omitempty"`
	LabelDetailsSupport   *bool                                                 `json:"labelDetailsSupport,omitempty"`
}

type lspTextDocumentCompletionListClientCapabilities struct {
	ItemDefaults []string `json:"itemDefaults,omitempty"`
}

type lspTextDocumentClientCapabilities struct {
	Completion         *lspCompletionClientCapabilities                 `json:"completion"`
	CompletionItemKind []int                                            `json:"completionItemKind,omitempty"`
	ContextSupport     *bool                                            `json:"contextSupport,omitempty"`
	InsertTextMode     *int                                             `json:"insertTextMode,omitempty"`
	CompletionList     *lspTextDocumentCompletionListClientCapabilities `json:"completionList,omitempty"`
}

type lspClientCapabilities struct {
	TextDocument *lspTextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type lspInitializeRequestParams struct {
	ProcessId             int                        `json:"processId"`
	ClientInfo            *lspInitializeClientInfo   `json:"clientInfo"`
	InitializationOptions *tealInitializationOptions `json:"initializationOptions,omitempty"`
	Capabilities          lspClientCapabilities      `json:"capabilities"`
}

type lspDidOpenTextDocument struct {
	Uri     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type lspDidOpenParams struct {
	TextDocument *lspDidOpenTextDocument `json:"textDocument"`
}

type lspDidChangeTextDocument struct {
	Uri     string `json:"uri"`
	Version int    `json:"version"`
}

type lspContentChange struct {
	Text string `json:"text"`
}

type lspDidChangeParams struct {
	TextDocument   *lspDidChangeTextDocument `json:"textDocument"`
	ContentChanges []*lspContentChange       `json:"contentChanges"`
}

type lspDidSaveTextDocument struct {
	Uri string `json:"uri"`
}

type lspDidSaveParams struct {
	TextDocument *lspDidSaveTextDocument `json:"textDocument"`
}

type lspDocumentSymbolTextDocument struct {
	Uri string `json:"uri"`
}

type lspDocumentSymbolParams struct {
	TextDocument *lspDocumentSymbolTextDocument `json:"textDocument"`
}

type tealGenerateSourcemapCommandArgs struct {
	Uri string `json:"uri"`
}

type tealGenerateSourcemapCommandResult struct {
	SourceMap logic.SourceMap `json:"sourcemap"`
}

type tealDecompileCommandArgs struct {
	Bytecode string `json:"bytecode"`
}

type tealDecompileCommandResult struct {
	Teal string `json:"teal"`
}

type tealGotoPcCommandArgs struct {
	Uri string `json:"uri"`
	Pc  int    `json:"pc"`
}

type tealUpdateVersion struct {
	Uri     string `json:"uri"`
	Version uint64 `json:"version"`
}

type tealReplaceValueCommandArgs struct {
	Uri   string   `json:"uri"`
	Range LspRange `json:"range"`
	Value string   `json:"name"`
}

type tealCreateLabelCommandArgs struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

type tealRemoveLabelCommandArgs struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

type tealRemoveCallCommandArgs struct {
	Uri       string `json:"uri"`
	Line      int    `json:"line"`
	Statement int    `json:"statement"`
}

type lspWorkspaceExecuteCommandHeader struct {
	Command string `json:"command"`
}

type lspWorkspaceExecuteCommandBodyArguments[T any] struct {
	Arguments T `json:"arguments"`
}

type lspWorkspaceExecuteCommandBody[T any] struct {
	Params lspWorkspaceExecuteCommandBodyArguments[T] `json:"params"`
}

type lspDiagnosticRequestTextDocument struct {
	Uri string `json:"uri"`
}

type lspDiagnosticRequestParams struct {
	TextDocument *lspDiagnosticRequestTextDocument `json:"textDocument"`
}

type lspCodeActionTextDocument struct {
	Uri string `json:"uri"`
}

type lspCodeActionContext struct {
	Diagnosticts []LspDiagnostic `json:"diagnostics"`
	Only         []string        `json:"only,omitempty"`
	TriggerKind  *int            `json:"triggerKind,omitempty"`
}

type lspCodeActionRequestParams struct {
	TextDocument lspCodeActionTextDocument `json:"textDocument"`
	Range        LspRange                  `json:"range"`
	Context      lspCodeActionContext      `json:"context"`
}

type lspRenameRequestTextDocument struct {
	Uri string `json:"uri"`
}

type lspRenameRequestParams struct {
	TextDocument  lspRenameRequestTextDocument `json:"textDocument"`
	Position      LspPosition
	NewName       string      `json:"newName"`
	WorkDoneToken interface{} `json:"workDoneToken,omitempty"`
}

type lspPrepareRenameRequestTextDocument struct {
	Uri string `json:"uri"`
}

type lspPrepareRenameRequestParams struct {
	TextDocument  lspPrepareRenameRequestTextDocument `json:"textDocument"`
	Position      LspPosition
	WorkDoneToken interface{} `json:"workDoneToken,omitempty"`
}

type lspDocumentColorRequestTextDocument struct {
	Uri string `json:"uri"`
}

type lspColor struct {
	R float32 `json:"r"`
	G float32 `json:"g"`
	B float32 `json:"b"`
	A float32 `json:"a"`
}

type lspColorInformation struct {
	Range LspRange `json:"range"`
	Color lspColor `json:"color"`
}

type lspDocumentColorRequestParams struct {
	TextDocument lspDocumentColorRequestTextDocument `json:"textDocument"`
}

type lspPrepareRenameResponse struct {
	Range       LspRange `json:"range"`
	Placeholder string   `json:"placeholder"`
}

type LspCommand struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

type lspTextEdit struct {
	Range   LspRange `json:"range"`
	NewText string   `json:"newText"`
}

type lspOptionalVersionedTextDocumentIdentifier struct {
	Uri     string `json:"uri"`
	Version *int   `json:"version"`
}

type lspTextDocumentEdit struct {
	TextDocument lspOptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []lspTextEdit                              `json:"edits"`
}

type lspWorkspaceEdit struct {
	Changes         map[string][]lspTextEdit `json:"changes,omitempty"`
	DocumentChanges []lspTextDocumentEdit    `json:"documentChanges,omitempty"`
}

type lspWorkspaceApplyEditRequestParams struct {
	Label string           `json:"label,omitempty"`
	Edit  lspWorkspaceEdit `json:"edit"`
}

type lspShowMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

type lspCodeAction struct {
	Title       string            `json:"title"`
	Kind        *string           `json:"kind,omitempty"`
	Diagnostics []LspDiagnostic   `json:"diagnostics,omitempty"`
	IsPreferred *bool             `json:"isPreferred,omitempty"`
	Edit        *lspWorkspaceEdit `json:"edit,omitempty"`
	Command     *LspCommand       `json:"command,omitempty"`
}

type lspDidCloseTextDocument struct {
	Uri string `json:"uri"`
}

type lspDidCloseRequestParams struct {
	TextDocument *lspDidCloseTextDocument `json:"textDocument"`
}

type LspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func (p LspPosition) StartLine() int {
	return p.Line
}

func (p LspPosition) StartCharacter() int {
	return p.Character
}

func (p LspPosition) EndLine() int {
	return p.Line
}

func (p LspPosition) EndCharacter() int {
	return p.Character
}

type LspRange struct {
	Start LspPosition `json:"start"`
	End   LspPosition `json:"end"`
}

func (r LspRange) StartLine() int {
	return r.Start.Line
}

func (r LspRange) StartCharacter() int {
	return r.Start.Character
}

func (r LspRange) EndLine() int {
	return r.End.Line
}

func (r LspRange) EndCharacter() int {
	return r.End.Character
}

type LspDiagnostic struct {
	Range    LspRange `json:"range"`
	Severity *int     `json:"severity,omitempty"`
	Message  string   `json:"message"`
}

type lspNotification struct {
	JsonRpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type lspProgressParams struct {
	Token interface{} `json:"token"`
	Value interface{} `json:"value"`
}

type lspWorkDoneProgressBegin struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Cancellable *bool  `json:"cancellable,omitempty"`
	Message     string `json:"message,omitempty"`
	Percentage  *int   `json:"percentage,omitempty"`
}

type lspWorkDoneProgressReport struct {
	Kind        string `json:"kind"`
	Cancellable *bool  `json:"cancellable,omitempty"`
	Message     string `json:"message,omitempty"`
	Percentage  *int   `json:"percentage,omitempty"`
}

type lspWorkDoneProgressEnd struct {
	Kind    string `json:"kind"`
	Message string `json:"message,omitempty"`
}

type lspRenameOptions struct {
	PrepareProvider  *bool `json:"prepareProvider,omitempty"`
	WorkDoneProgress *bool `json:"workDoneProgress,omitempty"`
}

type lspDocumentHighlightRequestTextDocument struct {
	Uri string `json:"uri"`
}

type lspDocumentHighlightRequestParams struct {
	TextDocument lspDocumentHighlightRequestTextDocument `json:"textDocument"`
	Position     LspPosition                             `json:"position"`
}

type lspDocumentHighlight struct {
	Range LspRange `json:"range"`
	Kind  *int     `json:"kind"`
}

type lspTextDocumentIdentifier struct {
	Uri string `json:"uri"`
}

type lspSemanticTokensFullRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
}

type lspCompletionRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     LspPosition               `json:"position"`
}

type lspDefinitionRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     LspPosition               `json:"position"`
}

type lspLocation struct {
	Uri   string   `json:"uri"`
	Range LspRange `json:"range"`
}

type lspMarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspHover struct {
	Contents lspMarkupContent `json:"contents"`
	Range    LspRange         `json:"range,omitempty"`
}

type lspHoverRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     LspPosition               `json:"position"`
}

type lspSignatureHelpRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     LspPosition               `json:"position"`
}

type lspInlayHintRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Range        LspRange                  `json:"range,omitempty"`
}

type LspInlayHint struct {
	Position    LspPosition `json:"position"`
	Label       string      `json:"label"`
	Kind        *int        `json:"kind,omitempty"`
	Tooltip     string      `json:"tooltip,omitempty"`
	PaddingLeft *bool       `json:"paddingLeft,omitempty"`
}

type lspCodeLensRequestParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
}

type LspCodeLens struct {
	Range   LspRange    `json:"range"`
	Command *LspCommand `json:"command,omitempty"`
	Data    any         `json:"data,omitempty"`
}

// notifications
type lspDidChange lspRequest[*lspDidChangeParams]
type lspDidOpen lspRequest[*lspDidOpenParams]
type lspDidSave lspRequest[*lspDidSaveParams]

// requests
type lspDocumentSymbolRequest lspRequest[*lspDocumentSymbolParams]
type lspWorkspaceExecuteCommand lspRequest[*lspWorkspaceExecuteCommandHeader]
type lspDiagnosticRequest lspRequest[*lspDiagnosticRequestParams]
type lspCodeActionRequest lspRequest[*lspCodeActionRequestParams]
type lspRenameRequest lspRequest[*lspRenameRequestParams]
type lspPrepareRenameRequest lspRequest[*lspPrepareRenameRequestParams]
type lspDocumentColorRequest lspRequest[*lspDocumentColorRequestParams]
type lspDidCloseRequest lspRequest[*lspDidCloseRequestParams]
type lspDocumentHighlightRequest lspRequest[*lspDocumentHighlightRequestParams]
type lspSemanticTokensFullRequest lspRequest[*lspSemanticTokensFullRequestParams]
type lspCompletionRequest lspRequest[*lspCompletionRequestParams]
type lspDefinitionRequest lspRequest[*lspDefinitionRequestParams]
type lspHoverRequest lspRequest[*lspHoverRequestParams]
type lspSignatureHelpRequest lspRequest[*lspSignatureHelpRequestParams]
type lspInlayHintRequest lspRequest[*lspInlayHintRequestParams]
type lspInitializeRequest lspRequest[*lspInitializeRequestParams]
type lspCodeLensRequest lspRequest[*lspCodeLensRequestParams]

func readInto(b []byte, v interface{}) error {
	err := json.Unmarshal(b, v)
	if err != nil {
		return err
	}

	return nil
}

func (l *lsp) reply(id interface{}, result interface{}, err interface{}) error {
	return l.write(jsonRpcResponse{
		JsonRpc: "2.0",
		Id:      id,
		Result:  result,
		Error:   err,
	})
}

func (l *lsp) fail(id interface{}, err interface{}) error {
	return l.reply(id, nil, err)
}

func (l *lsp) success(id interface{}, result interface{}) error {
	return l.reply(id, result, nil)
}

func read[T any](b []byte) (T, error) {
	var v T

	err := readInto(b, &v)
	if err != nil {
		return v, err
	}

	return v, nil
}

func (l *lsp) request(method string, params interface{}) error {
	l.id++
	return l.write(jsonRpcRequest{
		JsonRpc: "2.0",
		Id:      strconv.Itoa(l.id),
		Method:  method,
		Params:  params,
	})
}

func (l *lsp) notify(method string, params interface{}) error {
	return l.write(lspNotification{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (l *lsp) notifyProgress(token interface{}, value interface{}) error {
	if token == nil {
		return nil
	}
	return l.notify("$/progress", lspProgressParams{
		Token: token,
		Value: value,
	})
}

func (l *lsp) reportProgressBegin(token interface{}, title string, message string) error {
	return l.notifyProgress(token, lspWorkDoneProgressBegin{
		Kind:    "begin",
		Title:   title,
		Message: message,
	})
}

func (l *lsp) reportProgressEnd(token interface{}, message string) error {
	return l.notifyProgress(token, lspWorkDoneProgressEnd{
		Kind:    "end",
		Message: message,
	})
}

func (l *lsp) prepare(uri string) (*lspDoc, *logic.SourceAnalysisResult, error) {
	doc := l.docs[uri]
	if doc == nil {
		return nil, nil, errors.New("doc not found")
	}

	return doc, doc.Results(), nil
}

func (l *lsp) handle(h jsonRpcHeader, b []byte) error {

	if h.Result != nil {
		// TODO: handle success
		return nil
	}

	if h.Error != nil {
		// TODO: handle failure
		return nil
	}

	if h.Method == "" {
		// TODO: handle response
		return nil
	}

	switch h.Method { // notifications
	case "initialized":

	case "exit":
		l.exit = true
		if !l.shutdown {
			l.exitCode = 1
		}

	case "textDocument/didOpen":
		req, err := read[lspDidOpen](b)
		if err != nil {
			return err
		}

		doc := l.docs[req.Params.TextDocument.Uri]
		if doc == nil {
			doc = &lspDoc{}
			l.docs[req.Params.TextDocument.Uri] = doc
		}

		doc.Update(req.Params.TextDocument.Text)

	case "textDocument/didChange":
		req, err := read[lspDidChange](b)
		if err != nil {
			return err
		}

		for _, ch := range req.Params.ContentChanges {
			doc := l.docs[req.Params.TextDocument.Uri]
			if doc == nil {
				return errors.New("doc not found")
			}

			doc.Update(ch.Text)
		}

	case "textDocument/didSave":
		_, err := read[lspDidSave](b)
		if err != nil {
			return err
		}

		// TODO: handle save

	case "textDocument/didClose":
		req, err := read[lspDidCloseRequest](b)
		if err != nil {
			return err
		}

		delete(l.docs, req.Params.TextDocument.Uri)

	default: // requests

		if l.shutdown {
			return errors.New("cannot process requests - server is shut down")
		}

		switch h.Method {
		case "shutdown":
			l.shutdown = true
			return l.success(h.Id, nil)

		case "$/cancelRequest":

		case "workspace/executeCommand":
			req, err := read[lspWorkspaceExecuteCommand](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to parse request: %v", err),
				})
			}

			switch req.Params.Command {
			case "teal.sourcemap.generate":
				var body lspWorkspaceExecuteCommandBody[tealGenerateSourcemapCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				_, res, err := l.prepare(body.Params.Arguments.Uri)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to prepare document: %v", err),
					})
				}

				if res.Err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to assemble document: %v", res.Err),
					})
				}

				sm, ok := logic.SourceMapForTools(*res, []string{body.Params.Arguments.Uri})
				if !ok {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: "no opstream",
					})
				}
				return l.success(h.Id, tealGenerateSourcemapCommandResult{
					SourceMap: sm,
				})

			case "teal.decompile":
				var body lspWorkspaceExecuteCommandBody[tealDecompileCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				bs, err := base64.StdEncoding.DecodeString(body.Params.Arguments.Bytecode)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: fmt.Sprintf("failed to decode bytecode: %v", err),
					})
				}

				teal, err := logic.Disassemble(bs)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to disassemble bytecode: %v", err),
					})
				}

				result := tealDecompileCommandResult{
					Teal: teal,
				}

				return l.success(h.Id, result)

			case "teal.pc.resolve":
				var body lspWorkspaceExecuteCommandBody[tealGotoPcCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				_, res, err := l.prepare(body.Params.Arguments.Uri)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to prepare document: %v", err),
					})
				}

				if res.Err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to assemble document: %v", res.Err),
					})
				}

				pos, ok := logic.SourcePositionForProgramCounterForTools(*res, body.Params.Arguments.Pc)
				if !ok {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: "pc not found",
					})
				}

				return l.success(h.Id, LspPosition{
					Line: pos.Line, Character: pos.Column,
				})

			case "teal.version.update":
				var body lspWorkspaceExecuteCommandBody[[]tealUpdateVersion]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				args := body.Params.Arguments
				if len(args) != 1 {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("unexpected number of args").Error(),
					})
				}

				arg := args[0]

				doc := l.docs[arg.Uri]
				if doc == nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: errors.New("doc not found").Error(),
					})
				}

				res := doc.Results()
				edits := []lspTextEdit{
					sourceEditToLSP(res.Lines, logic.SourceEditUpdateVersionForTools(res.Index, arg.Version)),
				}

				err = l.request("workspace/applyEdit", lspWorkspaceApplyEditRequestParams{
					Label: "Update version",
					Edit: lspWorkspaceEdit{
						DocumentChanges: []lspTextDocumentEdit{
							{
								TextDocument: lspOptionalVersionedTextDocumentIdentifier{
									Uri: arg.Uri,
								},
								Edits: edits,
							},
						},
					},
				})

				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to apply edit: %v", err),
					})
				}

				return l.success(h.Id, nil)

			case "teal.value.replace":
				var body lspWorkspaceExecuteCommandBody[[]tealReplaceValueCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				args := body.Params.Arguments
				if len(args) != 1 {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("unexpected number of args").Error(),
					})
				}

				arg := args[0]

				doc := l.docs[arg.Uri]
				if doc == nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: errors.New("doc not found").Error(),
					})
				}

				edits := []lspTextEdit{
					{
						Range:   arg.Range,
						NewText: arg.Value,
					},
				}

				err = l.request("workspace/applyEdit", lspWorkspaceApplyEditRequestParams{
					Label: "Replace with named value",
					Edit: lspWorkspaceEdit{
						DocumentChanges: []lspTextDocumentEdit{
							{
								TextDocument: lspOptionalVersionedTextDocumentIdentifier{
									Uri: arg.Uri,
								},
								Edits: edits,
							},
						},
					},
				})

				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to apply edit: %v", err),
					})
				}

				return l.success(h.Id, nil)
			case "teal.call.remove":
				var body lspWorkspaceExecuteCommandBody[[]tealRemoveCallCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				args := body.Params.Arguments
				if len(args) != 1 {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("unexpected number of args").Error(),
					})
				}

				arg := args[0]

				doc := l.docs[arg.Uri]
				if doc == nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: errors.New("doc not found").Error(),
					})
				}

				line := arg.Line
				statement := arg.Statement
				res := doc.Results()
				edit, ok := logic.SourceEditRemoveStatementForTools(res.Lines, line, statement)
				if !ok {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("statement not found").Error(),
					})
				}

				edits := []lspTextEdit{sourceEditToLSP(res.Lines, edit)}

				err = l.request("workspace/applyEdit", lspWorkspaceApplyEditRequestParams{
					Label: "Remove call",
					Edit: lspWorkspaceEdit{
						DocumentChanges: []lspTextDocumentEdit{
							{
								TextDocument: lspOptionalVersionedTextDocumentIdentifier{
									Uri: arg.Uri,
								},
								Edits: edits,
							},
						},
					},
				})

				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to apply edit: %v", err),
					})
				}

				return l.success(h.Id, nil)
			case "teal.label.remove":
				var body lspWorkspaceExecuteCommandBody[[]tealRemoveLabelCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				args := body.Params.Arguments
				if len(args) != 1 {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("unexpected number of args").Error(),
					})
				}

				arg := args[0]

				_, res, err := l.prepare(arg.Uri)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to prepare document: %v", err),
					})
				}

				name := arg.Name

				edits := sourceEditsToLSP(res.Lines, logic.SourceEditsRemoveSymbolForTools(res.Index, body.Params.Arguments[0].Name))

				err = l.request("workspace/applyEdit", lspWorkspaceApplyEditRequestParams{
					Label: fmt.Sprintf("Remove label: %s", name),
					Edit: lspWorkspaceEdit{
						DocumentChanges: []lspTextDocumentEdit{
							{
								TextDocument: lspOptionalVersionedTextDocumentIdentifier{
									Uri: arg.Uri,
								},
								Edits: edits,
							},
						},
					},
				})

				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to apply edit: %v", err),
					})
				}

				return l.success(h.Id, nil)

			case "teal.label.create":
				var body lspWorkspaceExecuteCommandBody[[]tealCreateLabelCommandArgs]
				err := readInto(b, &body)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeParseError,
						Message: fmt.Sprintf("failed to read request body: %v", err),
					})
				}

				args := body.Params.Arguments
				if len(args) != 1 {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeInvalidParams,
						Message: errors.New("unexpected number of args").Error(),
					})
				}

				arg := args[0]

				_, res, err := l.prepare(arg.Uri)
				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to prepare document: %v", err),
					})
				}

				name := arg.Name

				err = l.request("workspace/applyEdit", lspWorkspaceApplyEditRequestParams{
					Label: fmt.Sprintf("Create label: %s", name),
					Edit: lspWorkspaceEdit{
						DocumentChanges: []lspTextDocumentEdit{
							{
								TextDocument: lspOptionalVersionedTextDocumentIdentifier{
									Uri: arg.Uri,
								},
								Edits: sourceEditsToLSP(res.Lines, []logic.SourceEdit{logic.SourceEditCreateLabelForTools(res.Lines, name)}),
							},
						},
					},
				})

				if err != nil {
					return l.fail(h.Id, lspError{
						Code:    ErrorCodeRequestFailed,
						Message: fmt.Sprintf("failed to apply edit: %v", err),
					})
				}

				return l.success(h.Id, nil)

			default:
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeMethodNotFound,
					Message: fmt.Sprintf("unknown command: %s", req.Params.Command),
				})
			}

		case "textDocument/prepareRename":
			req, err := read[lspPrepareRenameRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			err = l.reportProgressBegin(req.Params.WorkDoneToken, "Preparing Rename", "Checking symbol for rename")
			if err != nil {
				l.trace(fmt.Sprintf("Failed to report progress begin: %s", err))
			}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err != nil {
				l.reportProgressEnd(req.Params.WorkDoneToken, "Prepare rename failed")
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeRequestFailed,
					Message: fmt.Sprintf("failed to prepare document: %v", err),
				})
			}

			column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
			rename, ok := sourcePrepareRename(*res, req.Params.Position.Line, column)
			if ok {
				err = l.reportProgressEnd(req.Params.WorkDoneToken, "Symbol ready for rename")
				if err != nil {
					l.trace(fmt.Sprintf("Failed to report progress end: %s", err))
				}

				return l.success(h.Id, lspPrepareRenameResponse{
					Range:       sourceRangeToLSP(res.Lines, rename.Range),
					Placeholder: rename.Placeholder,
				})
			}

			err = l.reportProgressEnd(req.Params.WorkDoneToken, "No symbol found for rename")
			if err != nil {
				l.trace(fmt.Sprintf("Failed to report progress end: %s", err))
			}

			return l.success(h.Id, nil)

		case "textDocument/rename":
			req, err := read[lspRenameRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			err = l.reportProgressBegin(req.Params.WorkDoneToken, "Renaming Symbol", fmt.Sprintf("Renaming to '%s'", req.Params.NewName))
			if err != nil {
				l.trace(fmt.Sprintf("Failed to report progress begin: %s", err))
			}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err != nil {
				l.reportProgressEnd(req.Params.WorkDoneToken, "Rename failed")
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeRequestFailed,
					Message: fmt.Sprintf("failed to prepare document: %v", err),
				})
			}

			chs := []lspTextEdit{}

			column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
			chs = append(chs, sourceEditsToLSP(res.Lines, sourceRenameEdits(*res, req.Params.Position.Line, column, req.Params.NewName))...)

			message := fmt.Sprintf("Renamed %d locations", len(chs))
			err = l.reportProgressEnd(req.Params.WorkDoneToken, message)
			if err != nil {
				l.trace(fmt.Sprintf("Failed to report progress end: %s", err))
			}

			return l.success(h.Id, lspWorkspaceEdit{
				Changes: map[string][]lspTextEdit{
					req.Params.TextDocument.Uri: chs,
				},
			})

		case "textDocument/codeLens":
			req, err := read[lspCodeLensRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			var cls []LspCodeLens

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				for _, lens := range sourceCodeLenses(*res) {
					if sourceCodeLensEnabled(l.config, lens) {
						cls = append(cls, sourceCodeLensToLSP(res.Lines, lens))
					}
				}
			}

			return l.success(h.Id, cls)

		case "textDocument/inlayHint":
			req, err := read[lspInlayHintRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			ihs := []LspInlayHint{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				for _, inlay := range sourceInlays(*res) {
					if sourceInlayEnabled(l.config, inlay) && sourceInlayInRange(res.Lines, inlay, req.Params.Range) {
						ihs = append(ihs, sourceInlayToLSP(res.Lines, inlay))
					}
				}
			}
			return l.success(h.Id, ihs)

		case "textDocument/completion":
			req, err := read[lspCompletionRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}
			ccs := []lspCompletionItem{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
				ccs = sourceCompletionsAtToLSP(*res, req.Params.Position.Line, column)
			}

			if len(ccs) == 0 {
				ccs = append(ccs, lspCompletionItem{
					Label: "",
				})
			}

			return l.success(h.Id, ccs)

		case "textDocument/hover":
			req, err := read[lspHoverRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			var c interface{} = struct{}{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
				hover, ok := logic.SourceHoverForTools(*res, req.Params.Position.Line, column)
				if ok {
					c = lspHover{
						Contents: lspMarkupContent{
							Kind:  "plaintext",
							Value: hover.Text,
						},
					}
				}
			}

			return l.success(h.Id, c)

		case "textDocument/definition":
			req, err := read[lspDefinitionRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			ls := []lspLocation{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
				for _, rg := range sourceDefinitions(*res, req.Params.Position.Line, column) {
					ls = append(ls, lspLocation{
						Uri:   req.Params.TextDocument.Uri,
						Range: sourceRangeToLSP(res.Lines, rg),
					})
				}
			}

			return l.success(h.Id, ls)

		case "textDocument/signatureHelp":
			req, err := read[lspSignatureHelpRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			var sh interface{} = struct{}{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
				help, ok := logic.SourceSignatureHelpForTools(*res, req.Params.Position.Line, column)
				if ok {
					active := new(int)
					*active = help.ActiveParameter

					var doc interface{}
					if help.Docs != "" {
						doc = lspMarkupContent{
							Kind:  "markdown",
							Value: help.Docs,
						}
					}

					ps := []lspParameterInformation{}
					for _, parameter := range help.Parameters {
						ps = append(ps, lspParameterInformation{
							Label: parameter,
						})
					}

					sh = &lspSignatureHelp{
						Signatures: []lspSignatureInformation{
							{
								Label:           help.Label,
								Documentation:   doc,
								Parameters:      ps,
								ActiveParameter: active,
							},
						},
					}
				}
			}

			return l.success(h.Id, sh)

		case "textDocument/codeAction":
			req, err := read[lspCodeActionRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read request body: %v", err),
				})
			}

			cas := []lspCodeAction{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				rg := sourceRangeFromLSP(res.Lines, req.Params.Range)
				for _, action := range logic.SourceActionsForTools(res.Lines, res.Index, res.Program, rg) {
					cas = append(cas, sourceActionToLSP(req.Params.TextDocument.Uri, res.Lines, action))
				}
			}

			return l.success(h.Id, cas)
		case "textDocument/diagnostic":
			req, err := read[lspDiagnosticRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read diagnostic request: %s", err),
				})
			}

			ds := []LspDiagnostic{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				ds = sourceDiagnosticsToLSP(*res, l.config.ProgramSize)
			}

			return l.success(h.Id, lspFullDocumentDiagnosticReport{
				Kind:  "full",
				Items: ds,
			})

		case "textDocument/documentHighlight":
			req, err := read[lspDocumentHighlightRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read document highlight request: %s", err),
				})
			}

			hs := []lspDocumentHighlight{}

			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				column := sourceColumn(res.Lines, req.Params.Position.Line, req.Params.Position.Character)
				for _, highlight := range sourceHighlights(*res, req.Params.Position.Line, column) {
					hs = append(hs, sourceHighlightToLSP(res.Lines, highlight))
				}
			}

			return l.success(h.Id, hs)
		case "textDocument/documentSymbol":
			req, err := read[lspDocumentSymbolRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read document symbol request: %s", err),
				})
			}

			syms := []LspDocumentSymbol{}
			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				for _, symbol := range sourceDocumentSymbols(*res) {
					syms = append(syms, sourceDocumentSymbolToLSP(res.Lines, symbol))
				}
			}
			return l.success(h.Id, syms)

		case "textDocument/semanticTokens/full":
			req, err := read[lspSemanticTokensFullRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read semantic tokens full request: %s", err),
				})
			}

			st := SemanticTokens{}
			_, res, err := l.prepare(req.Params.TextDocument.Uri)
			if err == nil {
				for _, token := range logic.SourceSemanticTokensForTools(*res) {
					st = append(st, sourceSemanticTokenToLSP(res.Lines, token))
				}
			}

			data := st.Encode()

			return l.success(h.Id, lspSemanticTokens{
				Data: data,
			})

		case "initialize":
			req, err := read[lspInitializeRequest](b)
			if err != nil {
				return l.fail(h.Id, lspError{
					Code:    ErrorCodeParseError,
					Message: fmt.Sprintf("failed to read initialize request: %s", err),
				})
			}

			if req.Params != nil {
				if req.Params.InitializationOptions != nil {
					if req.Params.InitializationOptions.SemanticTokens != nil {
						l.config.SemanticTokens = *req.Params.InitializationOptions.SemanticTokens
					}
					if req.Params.InitializationOptions.InlayNamed != nil {
						l.config.InlayNamed = *req.Params.InitializationOptions.InlayNamed
					}
					if req.Params.InitializationOptions.InlayDecoded != nil {
						l.config.InlayDecoded = *req.Params.InitializationOptions.InlayDecoded
					}
					if req.Params.InitializationOptions.LensRefs != nil {
						l.config.LensRefs = *req.Params.InitializationOptions.LensRefs
					}
					if req.Params.InitializationOptions.PcInlay != nil {
						l.config.PcInlay = *req.Params.InitializationOptions.PcInlay
					}
					if req.Params.InitializationOptions.PcLens != nil {
						l.config.PcLens = *req.Params.InitializationOptions.PcLens
					}
					if req.Params.InitializationOptions.ProgramSize != nil {
						l.config.ProgramSize = *req.Params.InitializationOptions.ProgramSize
					}
				}
			}

			sync := new(int)
			*sync = 1

			definition := new(bool)
			*definition = true

			symbol := new(bool)
			*symbol = true

			action := new(bool)
			*action = true

			rename := new(bool)
			*rename = true

			highlight := new(bool)
			*highlight = true

			fullSemantic := new(bool)
			*fullSemantic = true

			hover := new(bool)
			*hover = true

			inlayHint := new(bool)
			if l.config.InlayNamed || l.config.InlayDecoded {
				*inlayHint = true
			}

			var semanticTokensProvider *lspSemanticTokensProvider

			if l.config.SemanticTokens {
				semanticTokensProvider = &lspSemanticTokensProvider{
					Full: fullSemantic,
					Legend: lspSemanticTokensLegend{
						TokenTypes:     []string{"keyword", "string", "comment", "method", "macro", "value", "number", "operator", "function"},
						TokenModifiers: []string{},
					},
				}
			}

			return l.success(h.Id, lspInitializeResult{
				Capabilities: &lspServerCapabilities{
					TextDocumentSync:          sync,
					DocumentHighlightProvider: highlight,
					DiagnosticProvider:        &lspDiagnosticProvider{},
					DocumentSymbolProvider:    symbol,
					CodeActionProvider:        action,
					ExecuteCommandProvider: &lspExecuteCommandProvider{
						Commands: []string{
							"teal.sourcemap.generate",
							"teal.decompile",
							"teal.label.create",
							"teal.label.remove",
							"teal.value.replace",
							"teal.call.remove",
							"teal.version.update",
							"teal.pc.resolve",
						},
					},
					RenameProvider: &lspRenameOptions{
						PrepareProvider:  rename,
						WorkDoneProgress: rename,
					},
					SemanticTokensProvider: semanticTokensProvider,
					CompletionProvider: &lspCompletionProvider{
						TriggerCharacters: []string{" "},
					},
					DefinitionProvider:    definition,
					HoverProvider:         hover,
					SignatureHelpProvider: &lspSignatureHelpOptions{},
					InlayHintProvider:     inlayHint,
					CodeLensProvider:      &lspCodeLensProvider{},
				},
			})
		default:
			return errors.New("unknown method")
		}
	}

	return nil
}

type sourceCodeLensKind int

const (
	sourceCodeLensReferenceCount sourceCodeLensKind = iota
	sourceCodeLensProgramCounter
)

type sourceCodeLens struct {
	Kind           sourceCodeLensKind
	Range          logic.SourceRange
	ReferenceCount int
	ProgramCounter int
}

type sourceInlayKind int

const (
	sourceInlayNamedValue sourceInlayKind = iota
	sourceInlayDecodedValue
	sourceInlayProgramCounter
)

type sourceInlay struct {
	Kind           sourceInlayKind
	Position       logic.SourcePosition
	Range          logic.SourceRange
	Label          string
	ProgramCounter int
}

type sourcePrepareRenameResult struct {
	Range       logic.SourceRange
	Placeholder string
}

type sourceHighlight struct {
	Range logic.SourceRange
}

type sourceDocumentSymbol struct {
	Name           string
	Range          logic.SourceRange
	SelectionRange logic.SourceRange
}

func sourcePrepareRename(result logic.SourceAnalysisResult, line int, column int) (sourcePrepareRenameResult, bool) {
	identifier, ok := logic.SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return sourcePrepareRenameResult{}, false
	}
	if identifier.Symbol != nil {
		return sourcePrepareRenameResult{
			Range:       logic.SourceSymbolNameRangeForTools(*identifier.Symbol),
			Placeholder: identifier.Name,
		}, true
	}
	if identifier.Reference != nil {
		return sourcePrepareRenameResult{
			Range:       logic.SourceReferenceRangeForTools(*identifier.Reference),
			Placeholder: identifier.Name,
		}, true
	}
	return sourcePrepareRenameResult{}, false
}

func sourceRenameEdits(result logic.SourceAnalysisResult, line int, column int, newName string) []logic.SourceEdit {
	identifier, ok := logic.SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	return logic.SourceEditsRenameSymbolForTools(result.Index, identifier.Name, newName)
}

func sourceDefinitions(result logic.SourceAnalysisResult, line int, column int) []logic.SourceRange {
	ref, ok := logic.SourceReferenceAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	var ranges []logic.SourceRange
	for _, symbol := range logic.SourceSymbolsByNameForTools(result.Index, ref.Name) {
		ranges = append(ranges, logic.SourceSymbolNameRangeForTools(symbol))
	}
	return ranges
}

func sourceHighlights(result logic.SourceAnalysisResult, line int, column int) []sourceHighlight {
	identifier, ok := logic.SourceIdentifierAtForTools(result.Index, line, column)
	if !ok {
		return nil
	}
	var highlights []sourceHighlight
	for _, symbol := range logic.SourceSymbolsByNameForTools(result.Index, identifier.Name) {
		highlights = append(highlights, sourceHighlight{Range: logic.SourceSymbolNameRangeForTools(symbol)})
	}
	for _, ref := range logic.SourceReferencesByNameForTools(result.Index, identifier.Name) {
		highlights = append(highlights, sourceHighlight{Range: logic.SourceReferenceRangeForTools(ref)})
	}
	return highlights
}

func sourceDocumentSymbols(result logic.SourceAnalysisResult) []sourceDocumentSymbol {
	symbols := make([]sourceDocumentSymbol, 0, len(result.Index.Symbols))
	for _, symbol := range result.Index.Symbols {
		if symbol.Name == "" {
			continue
		}
		symbols = append(symbols, sourceDocumentSymbol{
			Name:           symbol.Name,
			Range:          logic.SourceSymbolRangeForTools(symbol),
			SelectionRange: logic.SourceSymbolNameRangeForTools(symbol),
		})
	}
	return symbols
}

func sourceCodeLenses(result logic.SourceAnalysisResult) []sourceCodeLens {
	var lenses []sourceCodeLens

	for _, symbol := range result.Index.Symbols {
		count := result.Index.RefCounts[symbol.Name]
		if count == 0 {
			continue
		}
		lenses = append(lenses, sourceCodeLens{
			Kind: sourceCodeLensReferenceCount,
			Range: logic.SourceRange{
				Line:    symbol.Line,
				EndLine: symbol.Line,
			},
			ReferenceCount: count,
		})
	}

	for _, pc := range sourceProgramCounters(result) {
		loc := result.OpStream.OffsetToSource[pc]
		lenses = append(lenses, sourceCodeLens{
			Kind:           sourceCodeLensProgramCounter,
			Range:          sourceRangeFromLocation(loc),
			ProgramCounter: pc,
		})
	}

	return lenses
}

func sourceInlays(result logic.SourceAnalysisResult) []sourceInlay {
	var inlays []sourceInlay

	for _, hint := range logic.SourceInlayHintsForTools(result.Lines, result.Program) {
		inlay := sourceInlay{
			Position: logic.SourcePosition{
				Line:   hint.Token.Line,
				Column: hint.Token.EndColumn,
			},
			Range: sourceRangeFromToken(hint.Token),
			Label: hint.Label,
		}
		switch hint.Kind {
		case logic.SourceInlayHintNamed:
			inlay.Kind = sourceInlayNamedValue
		case logic.SourceInlayHintDecoded:
			inlay.Kind = sourceInlayDecodedValue
		default:
			continue
		}
		inlays = append(inlays, inlay)
	}

	for _, pc := range sourceProgramCounters(result) {
		loc := result.OpStream.OffsetToSource[pc]
		inlays = append(inlays, sourceInlay{
			Kind: sourceInlayProgramCounter,
			Position: logic.SourcePosition{
				Line:   loc.Line,
				Column: loc.Column,
			},
			Range:          sourceRangeFromLocation(loc),
			ProgramCounter: pc,
		})
	}

	return inlays
}

func sourceProgramCounters(result logic.SourceAnalysisResult) []int {
	if result.OpStream == nil || result.OpStream.OffsetToSource == nil {
		return nil
	}
	pcs := make([]int, 0, len(result.OpStream.OffsetToSource))
	for pc := range result.OpStream.OffsetToSource {
		pcs = append(pcs, pc)
	}
	sort.Ints(pcs)
	return pcs
}

func sourceRangeFromLocation(loc logic.SourceLocation) logic.SourceRange {
	return logic.SourceRange{
		Line:      loc.Line,
		Column:    loc.Column,
		EndLine:   loc.Line,
		EndColumn: loc.Column,
	}
}

func sourceRangeFromToken(token logic.SourceToken) logic.SourceRange {
	return logic.SourceRange{
		Line:      token.Line,
		Column:    token.Column,
		EndLine:   token.Line,
		EndColumn: token.EndColumn,
	}
}

func sourceActionToLSP(uri string, lines []logic.SourceLine, action logic.SourceAction) lspCodeAction {
	kind := "quickfix"
	edit := workspaceEditFromSourceEdits(uri, lines, action.Edits)
	return lspCodeAction{
		Title: action.Title,
		Kind:  &kind,
		Edit:  &edit,
	}
}

func sourceCodeLensEnabled(config tealConfig, lens sourceCodeLens) bool {
	switch lens.Kind {
	case sourceCodeLensReferenceCount:
		return config.LensRefs
	case sourceCodeLensProgramCounter:
		return config.PcLens
	default:
		return false
	}
}

func sourceCodeLensToLSP(lines []logic.SourceLine, lens sourceCodeLens) LspCodeLens {
	title := ""
	switch lens.Kind {
	case sourceCodeLensReferenceCount:
		title = fmt.Sprintf("refs: %d", lens.ReferenceCount)
	case sourceCodeLensProgramCounter:
		title = fmt.Sprintf("pc: %d", lens.ProgramCounter)
	default:
	}
	return LspCodeLens{
		Range: sourceRangeToLSP(lines, lens.Range),
		Command: &LspCommand{
			Title: title,
		},
	}
}

func sourceInlayEnabled(config tealConfig, inlay sourceInlay) bool {
	switch inlay.Kind {
	case sourceInlayNamedValue:
		return config.InlayNamed
	case sourceInlayDecodedValue:
		return config.InlayDecoded
	case sourceInlayProgramCounter:
		return config.PcInlay
	default:
		return false
	}
}

func sourceInlayInRange(lines []logic.SourceLine, inlay sourceInlay, rg LspRange) bool {
	return Overlaps(sourceRangeToLSP(lines, inlay.Range), rg)
}

func sourceInlayToLSP(lines []logic.SourceLine, inlay sourceInlay) LspInlayHint {
	parameter := new(int)
	*parameter = 2
	padding := new(bool)
	*padding = true

	label := inlay.Label
	if inlay.Kind == sourceInlayProgramCounter {
		label = fmt.Sprintf("pc: %d", inlay.ProgramCounter)
	}

	hint := LspInlayHint{
		Position:    sourcePositionToLSP(lines, inlay.Position),
		Label:       label,
		Kind:        parameter,
		PaddingLeft: padding,
	}
	if inlay.Kind == sourceInlayProgramCounter {
		hint.PaddingLeft = nil
	}
	return hint
}

func sourceCompletionsAtToLSP(result logic.SourceAnalysisResult, line int, column int) []lspCompletionItem {
	completions := sourceCompletionsToLSP(logic.SourceCompletionsForTools(result, line, column))
	ctx := logic.SourceCompletionContextForTools(result.Lines, result.Program, line, column)
	if ctx.Mode == logic.SourceCompletionOpcode {
		completions = append(sourceSnippetCompletionsToLSP(), completions...)
	}
	return completions
}

func sourceCompletionsToLSP(items []logic.SourceCompletionItem) []lspCompletionItem {
	completions := make([]lspCompletionItem, 0, len(items))
	for _, item := range items {
		completions = append(completions, sourceCompletionToLSP(item))
	}
	return completions
}

func sourceCompletionToLSP(item logic.SourceCompletionItem) lspCompletionItem {
	operator := new(int)
	*operator = 25
	snippetFormat := new(int)
	*snippetFormat = 2

	switch item.Kind {
	case logic.SourceCompletionItemDefine:
		return lspCompletionItem{
			Label:      item.Label,
			Kind:       operator,
			InsertText: item.Label,
		}
	case logic.SourceCompletionItemOpcode:
		var insert string
		var format *int
		if len(item.Args) > 0 {
			var placeholders string
			for i, arg := range item.Args {
				if i > 0 {
					placeholders += " "
				}
				placeholders += fmt.Sprintf("${%d:%s}", i+1, arg.Name)
			}
			insert = fmt.Sprintf("%s %s", item.Label, placeholders)
			format = snippetFormat
		}
		return lspCompletionItem{
			Label: item.Label,
			Documentation: lspMarkupContent{
				Kind:  "markdown",
				Value: item.Docs,
			},
			Kind:             operator,
			InsertText:       insert,
			InsertTextFormat: format,
			LabelDetails: &lspCompletionItemLabelDetails{
				Description: fmt.Sprintf("v%d", item.Version),
				Detail:      " " + item.ArgsSignature,
			},
		}
	default:
		var details *lspCompletionItemLabelDetails
		if item.HasValue {
			details = &lspCompletionItemLabelDetails{
				Detail: fmt.Sprintf(" = %d", item.Value),
			}
		} else if item.Signature != "" {
			details = &lspCompletionItemLabelDetails{
				Detail: fmt.Sprintf(" %s", item.Signature),
			}
		}
		return lspCompletionItem{
			LabelDetails: details,
			Label:        item.Label,
			Documentation: lspMarkupContent{
				Kind:  "markdown",
				Value: item.Docs,
			},
		}
	}
}

func sourceSnippetCompletionsToLSP() []lspCompletionItem {
	snippet := 15
	snippetFormat := new(int)
	*snippetFormat = 2
	return []lspCompletionItem{
		{
			Label:            "soc",
			Kind:             &snippet,
			Detail:           "switch on OnCompletion",
			InsertText:       sourceOnCompletionSwitchSnippet(),
			InsertTextFormat: snippetFormat,
		},
		{
			Label:            "func",
			Kind:             &snippet,
			Detail:           "create subroutine",
			InsertText:       "${1:sub}:\r\n\r\n\tproto ${2:0} ${3:0}\r\n\t${4}\r\n\tretsub\r\n",
			InsertTextFormat: snippetFormat,
		},
	}
}

func sourceOnCompletionSwitchSnippet() string {
	var at string
	var bt string
	for i, name := range logic.OnCompletionNames {
		if i > 0 {
			at += " "
		}

		field := fmt.Sprintf("${%d:%s}", i+1, strings.ToLower(name))

		at += field
		bt += fmt.Sprintf("%s:\n", field)

		if i < len(logic.OnCompletionNames)-1 {
			bt += fmt.Sprintf("b ${%d:then}\n", len(logic.OnCompletionNames)+2)
		}

		bt += "\n"
	}

	bt += fmt.Sprintf("${%d:then}:\n$%d", len(logic.OnCompletionNames)+2, len(logic.OnCompletionNames)+3)
	return fmt.Sprintf("txn OnCompletion\nswitch %s\n%s", at, bt)
}

func workspaceEditFromSourceEdits(uri string, lines []logic.SourceLine, edits []logic.SourceEdit) lspWorkspaceEdit {
	return lspWorkspaceEdit{
		DocumentChanges: []lspTextDocumentEdit{
			{
				TextDocument: lspOptionalVersionedTextDocumentIdentifier{
					Uri: uri,
				},
				Edits: sourceEditsToLSP(lines, edits),
			},
		},
	}
}

func sourceEditsToLSP(lines []logic.SourceLine, edits []logic.SourceEdit) []lspTextEdit {
	lspEdits := make([]lspTextEdit, 0, len(edits))
	for _, edit := range edits {
		lspEdits = append(lspEdits, sourceEditToLSP(lines, edit))
	}
	return lspEdits
}

func sourceEditToLSP(lines []logic.SourceLine, edit logic.SourceEdit) lspTextEdit {
	return lspTextEdit{
		Range: LspRange{
			Start: LspPosition{
				Line:      edit.Line,
				Character: sourceEditUTF16Column(lines, edit.Line, edit.Column),
			},
			End: LspPosition{
				Line:      edit.EndLine,
				Character: sourceEditUTF16Column(lines, edit.EndLine, edit.EndColumn),
			},
		},
		NewText: edit.NewText,
	}
}

func sourceRangeFromLSP(lines []logic.SourceLine, rg LspRange) logic.SourceRange {
	return logic.SourceRange{
		Line:      rg.Start.Line,
		Column:    sourceRangeByteColumn(lines, rg.Start.Line, rg.Start.Character),
		EndLine:   rg.End.Line,
		EndColumn: sourceRangeByteColumn(lines, rg.End.Line, rg.End.Character),
	}
}

func sourceEditUTF16Column(lines []logic.SourceLine, line int, column int) int {
	if line < 0 || line >= len(lines) {
		return column
	}
	return utf16ColumnFromByte(lines[line].Text, column)
}

func sourceRangeByteColumn(lines []logic.SourceLine, line int, character int) int {
	if line < 0 || line >= len(lines) {
		return character
	}
	return byteColumnFromUTF16Column(lines[line].Text, character)
}

// utf16LenString returns the number of UTF-16 code units required to
// represent the provided string. LSP positions are defined in UTF-16
// code units and we must provide ranges in those units.
func utf16LenString(s string) int {
	cnt := 0
	for _, r := range s {
		if r > 0xFFFF {
			cnt += 2
		} else {
			cnt++
		}
	}
	return cnt
}

func sourceRangeToLSP(lines []logic.SourceLine, rg logic.SourceRange) LspRange {
	return LspRange{
		Start: LspPosition{
			Line:      rg.Line,
			Character: sourceEditUTF16Column(lines, rg.Line, rg.Column),
		},
		End: LspPosition{
			Line:      rg.EndLine,
			Character: sourceEditUTF16Column(lines, rg.EndLine, rg.EndColumn),
		},
	}
}

func sourcePositionToLSP(lines []logic.SourceLine, pos logic.SourcePosition) LspPosition {
	return LspPosition{
		Line:      pos.Line,
		Character: sourceEditUTF16Column(lines, pos.Line, pos.Column),
	}
}

func sourceSemanticTokenToLSP(lines []logic.SourceLine, token logic.SourceSemanticToken) SemanticToken {
	rg := sourceRangeToLSP(lines, token.Range)
	return SemanticToken{
		Line:      rg.Start.Line,
		Index:     rg.Start.Character,
		Length:    rg.End.Character - rg.Start.Character,
		Type:      sourceSemanticTokenTypeToLSP(token.Kind),
		Modifiers: 0,
	}
}

func sourceSemanticTokenTypeToLSP(kind logic.SourceSemanticTokenKind) int {
	switch kind {
	case logic.SourceSemanticMacro:
		return semanticTokenMacro
	case logic.SourceSemanticBool:
		return semanticTokenValue
	case logic.SourceSemanticNumber:
		return semanticTokenNumber
	case logic.SourceSemanticString:
		return semanticTokenString
	case logic.SourceSemanticKeyword:
		return semanticTokenKeyword
	case logic.SourceSemanticComment:
		return semanticTokenComment
	case logic.SourceSemanticSymbol:
		return semanticTokenMethod
	case logic.SourceSemanticReference:
		return semanticTokenString
	default:
		return semanticTokenKeyword
	}
}

func sourceDocumentSymbolToLSP(lines []logic.SourceLine, symbol sourceDocumentSymbol) LspDocumentSymbol {
	return LspDocumentSymbol{
		Name:           symbol.Name,
		Kind:           LspSymbolKindMethod,
		Range:          sourceRangeToLSP(lines, symbol.Range),
		SelectionRange: sourceRangeToLSP(lines, symbol.SelectionRange),
	}
}

func sourceHighlightToLSP(lines []logic.SourceLine, highlight sourceHighlight) lspDocumentHighlight {
	return lspDocumentHighlight{
		Range: sourceRangeToLSP(lines, highlight.Range),
		Kind:  &symbolHighlightKind,
	}
}

var symbolHighlightKind = 1

func sourceDiagnosticsToLSP(result logic.SourceAnalysisResult, programSize bool) []LspDiagnostic {
	ds := make([]LspDiagnostic, 0, len(result.Diagnostics)+1)
	for _, diagnostic := range result.Diagnostics {
		ds = append(ds, sourceDiagnosticToLSP(result.Lines, diagnostic))
	}
	if programSize && result.Err == nil && result.OpStream != nil {
		ds = append(ds, sourceProgramSizeDiagnosticToLSP(len(result.OpStream.Program)))
	}
	return ds
}

func sourceProgramSizeDiagnosticToLSP(size int) LspDiagnostic {
	sev := int(DiagInfo)
	return LspDiagnostic{
		Range: LspRange{
			Start: LspPosition{},
			End:   LspPosition{},
		},
		Severity: &sev,
		Message:  fmt.Sprintf("Program size: %d", size),
	}
}

func sourceDiagnosticToLSP(lines []logic.SourceLine, diagnostic logic.SourceDiagnostic) LspDiagnostic {
	sev := int(sourceDiagnosticSeverityToLSP(diagnostic.Severity))
	lineText := ""
	if diagnostic.Line >= 0 && diagnostic.Line < len(lines) {
		lineText = lines[diagnostic.Line].Text
	}

	return LspDiagnostic{
		Range: LspRange{
			Start: LspPosition{
				Line:      diagnostic.Line,
				Character: utf16ColumnFromByte(lineText, diagnostic.Column),
			},
			End: LspPosition{
				Line:      diagnostic.Line,
				Character: utf16ColumnFromByte(lineText, diagnostic.EndColumn),
			},
		},
		Severity: &sev,
		Message:  diagnostic.Message,
	}
}

func sourceDiagnosticSeverityToLSP(severity logic.SourceDiagnosticSeverity) DiagnosticSeverity {
	switch severity {
	case logic.SourceDiagnosticWarning:
		return DiagWarn
	default:
		return DiagErr
	}
}

func (l *lsp) write(v interface{}) error {
	rb, err := json.Marshal(v)
	if err != nil {
		return errors.Wrap(err, "failed to marshal response")
	}

	l.trace(fmt.Sprintf("OUT: %s", string(rb)))

	h := http.Header{}
	h.Set("Content-Length", strconv.Itoa(len(rb)))

	err = h.Write(l.w)
	if err != nil {
		return errors.Wrap(err, "failed to write response headers")
	}

	_, err = l.w.Write([]byte("\r\n"))
	if err != nil {
		return errors.Wrap(err, "failed to write")
	}

	_, err = l.w.Write(rb)
	if err != nil {
		return errors.Wrap(err, "failed to write response body")
	}

	err = l.w.Flush()
	if err != nil {
		return errors.Wrap(err, "failed to flush")
	}

	return nil
}

func (l *lsp) trace(s string) {
	if l.debug == nil {
		return
	}

	l.debug.WriteString(s)
	l.debug.WriteString("\n")

	l.debug.Flush()
}

func (l *lsp) Run() (int, error) {
	l.trace("TEAL LSP running..")
	defer func() {
		l.trace("TEAL LSP exited.")
	}()

	for !l.exit {
		err := func() error {
			mh, err := l.tp.ReadMIMEHeader()
			if err != nil {
				return errors.Wrap(err, "failed to read request headers")
			}

			h := http.Header(mh)

			length, err := strconv.Atoi(h.Get("Content-Length"))
			if err != nil {
				return errors.Wrap(err, "failed to parse content length")
			}

			data := make([]byte, length)
			_, err = io.ReadFull(l.tp.R, data)
			if err != nil {
				return errors.Wrap(err, "failed to read content body")
			}

			l.trace(fmt.Sprintf("IN: %s", string(data)))

			var jh jsonRpcHeader
			err = json.Unmarshal(data, &jh)
			if err != nil {
				return errors.Wrap(err, "failed to unmarshal json rpc header")
			}

			err = l.handle(jh, data)
			if err != nil {
				return errors.Wrap(err, "failed to handle request")
			}

			return nil
		}()

		if err != nil {
			l.trace(fmt.Sprintf("ERR: %s", err))

			if errors.Is(err, io.EOF) {
				break
			}
		}
	}

	return l.exitCode, nil
}

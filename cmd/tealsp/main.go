package main

import (
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/dragmz/teal"
	"github.com/dragmz/teal/dbg"
	"github.com/dragmz/teal/lsp"
	"github.com/pkg/errors"
)

type lspArgs struct {
	Debug string

	Addr string
	Net  string
}

type dbgArgs struct {
	Debug      string
	Config     string
	Algod      string
	AlgodToken string
	Replay     string
}

func runLsp(a lspArgs) (int, error) {
	var r io.Reader
	var w io.Writer

	if a.Addr != "" && a.Net != "" {
		c, err := net.Dial(a.Net, a.Addr)
		if err != nil {
			return -1, errors.Wrap(err, "failed to connect to the client")
		}

		r = c
		w = c
	} else {
		r = os.Stdin
		w = os.Stdout
	}

	var opts []lsp.LspOption
	if a.Debug != "" {
		f, err := os.Create(a.Debug)
		if err != nil {
			return -2, errors.Wrap(err, "failed to create debug output file")
		}

		opts = append(opts, lsp.WithDebug(f))
	}

	opts = append(opts, lsp.WithPrepareDiagnosticsHandler(func(source string) []lsp.LspDiagnostic {
		var res []lsp.LspDiagnostic

		ops, err := logic.AssembleString(source)

		if err != nil {
			if len(ops.Errors) == 0 && len(ops.Warnings) == 0 {
				if err != nil {
					sev := teal.DiagErr
					res = append(res, lsp.LspDiagnostic{
						Range: lsp.LspRange{
							Start: lsp.LspPosition{
								Line:      0,
								Character: 0,
							},
							End: lsp.LspPosition{
								Line:      0,
								Character: 0,
							},
						},
						Severity: &sev,
						Message:  err.Error(),
					})
				}
			}
		}

		for _, e := range ops.Errors {
			l := e.Line
			c := e.Column

			if l != 0 {
				l--
			}

			if c != 0 {
				c--
			}

			sev := teal.DiagErr
			res = append(res, lsp.LspDiagnostic{
				Range: lsp.LspRange{
					Start: lsp.LspPosition{
						Line:      l,
						Character: c,
					},
					End: lsp.LspPosition{
						Line:      l,
						Character: c,
					},
				},
				Severity: &sev,
				Message:  e.Unwrap().Error(),
			})
		}

		for _, w := range ops.Warnings {
			sev := teal.DiagWarn
			res = append(res, lsp.LspDiagnostic{
				Range: lsp.LspRange{
					Start: lsp.LspPosition{
						Line:      0,
						Character: 0,
					},
					End: lsp.LspPosition{
						Line:      0,
						Character: 0,
					},
				},
				Severity: &sev,
				Message:  w.Error(),
			})
		}

		return res
	}))

	l, err := lsp.New(r, w, opts...)
	if err != nil {
		return -3, errors.Wrap(err, "failed to create lsp")
	}

	return l.Run()
}

func runDbg(a dbgArgs) (int, error) {
	r := os.Stdin
	w := os.Stdout

	var opts []dbg.DbgOption
	if a.Debug != "" {
		f, err := os.Create(a.Debug)
		if err != nil {
			return -2, errors.Wrap(err, "failed to create debug output file")
		}

		opts = append(opts, dbg.WithDebug(f))
	}

	if a.Config != "" {
		bs, err := os.ReadFile(a.Config)
		if err != nil {
			return -3, errors.Wrap(err, "failed to read config file")
		}

		var cfg dbg.DbgConfig
		if err := json.Unmarshal(bs, &cfg); err != nil {
			return -4, errors.Wrap(err, "failed to unmarshal config file")
		}

		opts = append(opts, dbg.WithConfig(cfg))
	}

	if a.Algod != "" {
		opts = append(opts, dbg.WithAlgod(a.Algod, a.AlgodToken))
	}

	if a.Replay != "" {
		bs, err := os.ReadFile(a.Replay)
		if err != nil {
			return -5, errors.Wrap(err, "failed to read replay file")
		}

		var rpl models.SimulateResponse
		if err := json.Unmarshal(bs, &rpl); err != nil {
			return -6, errors.Wrap(err, "failed to unmarshal replay file")
		}

		opts = append(opts, dbg.WithReplay(rpl))
	}

	l, err := dbg.New(r, w, opts...)
	if err != nil {
		return -3, errors.Wrap(err, "failed to create dbg")
	}

	return l.Run()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dbg" {
		var a dbgArgs

		flag.StringVar(&a.Debug, "debug", "", "debug file path")
		flag.StringVar(&a.Config, "config", "", "config file path")
		flag.StringVar(&a.Algod, "algod", "https://testnet-api.algonode.cloud", "algod endpoint")
		flag.StringVar(&a.AlgodToken, "algod-token", "", "algod token")
		flag.StringVar(&a.Replay, "replay", "", "replay file path")

		copy(os.Args[1:], os.Args[2:])
		os.Args = os.Args[:len(os.Args)-1]

		flag.Parse()

		code, err := runDbg(a)
		if err != nil {
			panic(err)
		}
		os.Exit(code)
	} else {
		var a lspArgs

		flag.StringVar(&a.Net, "net", "tcp", "client network")
		flag.StringVar(&a.Addr, "addr", "", "client address")
		flag.StringVar(&a.Debug, "debug", "", "debug file path")

		flag.Parse()

		code, err := runLsp(a)
		if err != nil {
			panic(err)
		}

		os.Exit(code)
	}
}

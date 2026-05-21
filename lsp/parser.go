package lsp

import (
	"strconv"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/pkg/errors"
)

func readInt8(s string) (int8, error) {
	v, err := strconv.ParseInt(s, 10, 8)
	if err != nil {
		return 0, err
	}

	return int8(v), nil
}

func readBool(s string) (bool, error) {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return false, err
	}

	return v, nil
}

func readUint8(s string) (uint8, error) {
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, err
	}

	return uint8(v), nil
}

func readAssetHoldingField(v uint64, s string) (logic.AssetHoldingField, bool, error) {
	spec, ok := logic.AssetHoldingFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.AssetHoldingField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.AssetHoldingField(value), false, nil
}

func readVrfVerifyField(v uint64, s string) (logic.VrfStandard, bool, error) {
	spec, ok := logic.VrfStandardSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.VrfStandard(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.VrfStandard(value), false, nil
}

func readBlockField(v uint64, s string) (logic.BlockField, bool, error) {
	spec, ok := logic.BlockFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.BlockField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.BlockField(value), false, nil
}

func readVoterParams(v uint64, s string) (logic.VoterParamsField, bool, error) {
	spec, ok := logic.VoterParamsFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.VoterParamsField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.VoterParamsField(value), false, nil
}

func readMimcField(v uint64, s string) (logic.MimcConfig, bool, error) {
	spec, ok := logic.MimcConfigSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.MimcConfig(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse mimc field")
	}

	return logic.MimcConfig(value), false, nil
}

func readVoterParamsField(v uint64, s string) (logic.VoterParamsField, bool, error) {
	spec, ok := logic.VoterParamsFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.VoterParamsField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.VoterParamsField(value), false, nil
}

func readAcctParams(v uint64, s string) (logic.AcctParamsField, bool, error) {
	spec, ok := logic.AcctParamsFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.AcctParamsField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.AcctParamsField(value), false, nil
}

func readAppParamsField(v uint64, s string) (logic.AppParamsField, bool, error) {
	spec, ok := logic.AppParamsFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.AppParamsField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.AppParamsField(value), false, nil
}

func readAssetParamsField(v uint64, s string) (logic.AssetParamsField, bool, error) {
	spec, ok := logic.AssetParamsFieldSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.AssetParamsField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.AssetParamsField(value), false, nil
}

func readGlobalField(v uint64, s string, mode logic.RunMode) (logic.GlobalField, bool, error) {
	spec, ok := logic.GlobalFieldSpecByName()[s]
	if ok {
		if !spec.Mode().Any() {
			if spec.Mode() != mode {
				return 0, true, errors.Errorf("not available in this mode (need: %s, got: %s)", spec.Mode(), mode)
			}
		}

		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.GlobalField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.GlobalField(value), false, nil
}

func readEcGroupField(v uint64, s string) (logic.EcGroup, bool, error) {
	spec, ok := logic.EcGroupSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.EcGroup(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.EcGroup(value), false, nil
}

func readBase64EncodingField(v uint64, s string) (logic.Base64Encoding, bool, error) {
	spec, ok := logic.Base64EncodingSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.Base64Encoding(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.Base64Encoding(value), false, nil
}

func readJsonRefField(v uint64, s string) (logic.JSONRefType, bool, error) {
	spec, ok := logic.JsonRefSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.JSONRefType(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.JSONRefType(value), false, nil
}

func readTxnField(c fieldContext, v uint64, s string, m logic.RunMode) (logic.TxnField, bool, error) {
	spec, ok := logic.TxnFieldSpecByName()[s]
	if ok {
		if spec.Effects() && m == logic.ModeSig {
			return 0, true, errors.Errorf("not available in this mode (need: %s, got: %s)", logic.ModeApp, m)
		}

		switch c {
		case txnaFieldContext:
			if !spec.Array() {
				return 0, true, errors.New("not available in array context")
			}
		}
		var needed uint64

		switch c {
		case txnFieldContext:
			needed = spec.Version()
		case itxnFieldContext:
			if spec.ItxVersion() == 0 {
				return 0, true, errors.New("not available for internal transactions")
			}
			needed = spec.ItxVersion()
		}

		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.TxnField(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.TxnField(value), false, nil
}

func readInt(a *arguments) (uint64, error) {
	val, err := strconv.ParseUint(a.Text(), 0, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}

func readConstInt(a *arguments) (uint64, error) {
	i, ok := logic.TxnTypeMap()[a.Text()]
	if ok {
		return i, nil
	}

	oc, ok := logic.OnCompletionMap()[a.Text()]
	if ok {
		return oc, nil
	}

	val, err := strconv.ParseUint(a.Text(), 0, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}

func readEcdsaCurveIndex(v uint64, s string) (logic.EcdsaCurve, bool, error) {
	spec, ok := logic.EcdsaCurveSpecByName()[s]
	if ok {
		needed := spec.Version()
		if needed > v {
			return 0, true, errors.Errorf("not available in this version (need >= %d, got: %d)", needed, v)
		}

		return logic.EcdsaCurve(spec.Field()), true, nil
	}

	value, err := readUint8(s)
	if err != nil {
		return 0, false, errors.Wrap(err, "failed to parse txn field")
	}

	return logic.EcdsaCurve(value), false, nil
}

type arguments struct {
	ts []Token
	i  int
}

func (a *arguments) Scan() bool {
	if a.i <= len(a.ts) {
		a.i++
		return a.i <= len(a.ts)
	}

	return false
}

func (a *arguments) Prev() Token {
	if a.i > 1 {
		return a.ts[a.i-2]
	}

	return Token{}
}

func (a *arguments) Curr() Token {
	if a.i > 0 && a.i <= len(a.ts) {
		return a.ts[a.i-1]
	}

	return Token{}
}

func (a *arguments) Text() string {
	return a.Curr().String()
}

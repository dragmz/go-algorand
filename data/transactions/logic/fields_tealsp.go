// Copyright (C) 2019-2025 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package logic

type MimcConfigSpec = mimcConfigSpec

func (fs mimcConfigSpec) FieldString() string {
	return fs.field.String()
}

type TxnFieldSpec = txnFieldSpec

func (fs txnFieldSpec) FieldString() string {
	return fs.field.String()
}

type GlobalFieldSpec = globalFieldSpec

func (fs globalFieldSpec) FieldString() string {
	return fs.field.String()
}

type EcdsaCurveSpec = ecdsaCurveSpec

func (fs ecdsaCurveSpec) FieldString() string {
	return fs.field.String()
}

type EcGroupSpec = ecGroupSpec

func (fs ecGroupSpec) FieldString() string {
	return fs.field.String()
}

type Base64EncodingSpec = base64EncodingSpec

func (fs base64EncodingSpec) FieldString() string {
	return fs.field.String()
}

type JsonRefSpec = jsonRefSpec

func (fs jsonRefSpec) FieldString() string {
	return fs.field.String()
}

type VrfStandardSpec = vrfStandardSpec

func (fs vrfStandardSpec) FieldString() string {
	return fs.field.String()
}

type BlockFieldSpec = blockFieldSpec

func (fs blockFieldSpec) FieldString() string {
	return fs.field.String()
}

type AssetHoldingFieldSpec = assetHoldingFieldSpec

func (fs assetHoldingFieldSpec) FieldString() string {
	return fs.field.String()
}

type AssetParamsFieldSpec = assetParamsFieldSpec

func (fs assetParamsFieldSpec) FieldString() string {
	return fs.field.String()
}

type AppParamsFieldSpec = appParamsFieldSpec

func (fs appParamsFieldSpec) FieldString() string {
	return fs.field.String()
}

type AcctParamsFieldSpec = acctParamsFieldSpec

func (fs acctParamsFieldSpec) FieldString() string {
	return fs.field.String()
}

type VoterParamsFieldSpec = voterParamsFieldSpec

func (fs voterParamsFieldSpec) FieldString() string {
	return fs.field.String()
}

var VoterParamsFieldSpecByField = voterParamsFieldSpecByField
var VoterParamsFieldSpecByName = voterParamsFieldSpecByName

var AcctParamsFieldSpecByField = acctParamsFieldSpecByField
var AcctParamsFieldSpecByName = acctParamsFieldSpecByName

var AppParamsFieldSpecByField = appParamsFieldSpecByField
var AppParamsFieldSpecByName = appParamsFieldSpecByName

var AssetParamsFieldSpecByField = assetParamsFieldSpecByField
var AssetParamsFieldSpecByName = assetParamsFieldSpecByName

var AssetHoldingFieldSpecByField = assetHoldingFieldSpecByField
var AssetHoldingFieldSpecByName = assetHoldingFieldSpecByName

var BlockFieldSpecByField = blockFieldSpecByField
var BlockFieldSpecByName = blockFieldSpecByName

var VrfStandardSpecByField = vrfStandardSpecByField
var VrfStandardSpecByName = vrfStandardSpecByName

var JsonRefSpecByField = jsonRefSpecByField
var JsonRefSpecByName = jsonRefSpecByName

var Base64EncodingSpecByField = base64EncodingSpecByField
var Base64EncodingSpecByName = base64EncodingSpecByName

var EcGroupSpecByField = ecGroupSpecByField
var EcGroupSpecByName = ecGroupSpecByName

var EcdsaCurveSpecByField = ecdsaCurveSpecByField
var EcdsaCurveSpecByName = ecdsaCurveSpecByName

var GlobalFieldSpecByField = globalFieldSpecByField
var GlobalFieldSpecByName = globalFieldSpecByName

func (fs globalFieldSpec) Mode() RunMode {
	return fs.mode
}

var TxnFieldSpecByField = txnFieldSpecByField
var TxnFieldSpecByName = txnFieldSpecByName

func (fs txnFieldSpec) Array() bool {
	return fs.array
}

func (fs txnFieldSpec) ItxVersion() uint64 {
	return fs.itxVersion
}

func (fs txnFieldSpec) Effects() bool {
	return fs.effects
}

var MimcConfigSpecByField = mimcConfigSpecByField
var MimcConfigSpecByName = mimcConfigSpecByName

var TxnTypeMap = txnTypeMap
var OnCompletionMap = onCompletionMap

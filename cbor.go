// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the github.com/fxamacker/cbor whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// MIT License
//
// Copyright (c) 2019-present Faye Amacker
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cborpatch

import (
	"strconv"

	"github.com/fxamacker/cbor/v2"
)

// Predefined CBORTypes.
const (
	CBORTypePositiveInt CBORType = 0x00
	CBORTypeNegativeInt CBORType = 0x20
	CBORTypeByteString  CBORType = 0x40
	CBORTypeTextString  CBORType = 0x60
	CBORTypeArray       CBORType = 0x80
	CBORTypeMap         CBORType = 0xa0
	CBORTypeTag         CBORType = 0xc0
	CBORTypePrimitives  CBORType = 0xe0
	CBORTypeInvalid     CBORType = 0xff
)

var (
	rawCBORNull  = []byte{0xf6}
	rawCBORArray = []byte{0x80}
	rawCBORMap   = []byte{0xa0}
)

var (
	decMode, _ = cbor.DecOptions{
		DupMapKey:   cbor.DupMapKeyQuiet,
		IndefLength: cbor.IndefLengthForbidden,
		UTF8:        cbor.UTF8DecodeInvalid,
	}.DecMode()

	encMode, _ = cbor.EncOptions{
		Sort:          cbor.SortLengthFirst,
		Time:          cbor.TimeRFC3339Nano,
		ShortestFloat: cbor.ShortestFloat16,
		NaNConvert:    cbor.NaNConvert7e00,
		InfConvert:    cbor.InfConvertFloat16,
		IndefLength:   cbor.IndefLengthForbidden,
		BigIntConvert: cbor.BigIntConvertNone,
	}.EncMode()

	cborUnmarshal = decMode.Unmarshal
	cborValid     = decMode.Valid
	cborMarshal   = encMode.Marshal
)

// SetCBOR set the underlying global CBOR Marshal and Unmarshal functions.
// The default is cbor.CanonicalEncOptions's Marshal and default cbor.Unmarshal.
//
//	func init() {
//		var EncMode, _ = cbor.CanonicalEncOptions().EncMode()
//		var DecMode, _ = cbor.DecOptions{
//			DupMapKey:   cbor.DupMapKeyQuiet,
//			IndefLength: cbor.IndefLengthForbidden,
//		}.DecMode()
//
//		cborpatch.SetCBOR(EncMode.Marshal, DecMode.Unmarshal)
//	}
func SetCBOR(
	marshal func(v interface{}) ([]byte, error),
	unmarshal func(data []byte, v interface{}) error,
) {
	cborMarshal = marshal
	cborUnmarshal = unmarshal
}

// RawMessage is a raw encoded CBOR value.
type RawMessage = cbor.RawMessage

// CBORType is the type of a raw encoded CBOR value.
type CBORType uint8

// String returns a string representation of CBORType.
func (t CBORType) String() string {
	switch t {
	case CBORTypePositiveInt:
		return "positive integer"
	case CBORTypeNegativeInt:
		return "negative integer"
	case CBORTypeByteString:
		return "byte string"
	case CBORTypeTextString:
		return "UTF-8 text string"
	case CBORTypeArray:
		return "array"
	case CBORTypeMap:
		return "map"
	case CBORTypeTag:
		return "tag"
	case CBORTypePrimitives:
		return "primitives"
	default:
		return "invalid type " + strconv.Itoa(int(t))
	}
}

// ReadCBORType returns the type of a raw encoded CBOR value.
func ReadCBORType(data []byte) CBORType {
	switch {
	case len(data) == 0:
		return CBORTypeInvalid
	default:
		return CBORType(data[0] & 0xe0)
	}
}

func MustMarshal(val interface{}) []byte {
	data, err := cborMarshal(val)
	if err != nil {
		panic(err)
	}
	return data
}

// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type Op int

const (
	OpReserved Op = iota
	OpAdd
	OpRemove
	OpReplace
	OpMove
	OpCopy
	OpTest
)

// String returns a string representation of the Op.
func (op Op) String() string {
	switch op {
	default:
		return fmt.Sprintf("reserved(%d)", op)
	case OpAdd:
		return "add"
	case OpRemove:
		return "remove"
	case OpReplace:
		return "replace"
	case OpMove:
		return "move"
	case OpCopy:
		return "copy"
	case OpTest:
		return "test"
	}
}

// Operation is a single CBOR-Patch step, such as a single 'add' operation.
type Operation struct {
	Op    Op         `cbor:"1,keyasint"`
	From  Path       `cbor:"2,keyasint,omitempty"`
	Path  Path       `cbor:"3,keyasint"`
	Value RawMessage `cbor:"4,keyasint,omitempty"`
}

func (o *Operation) Valid() error {
	if o == nil {
		return errors.New("nil operation")
	}

	switch o.Op {
	default:
		return fmt.Errorf("invalid operation: %d", o.Op)

	case OpAdd:
		if o.From != nil {
			return errors.New(`"from" must be nil for "add" operation`)
		}

	case OpRemove:
		if o.From != nil {
			return errors.New(`"from" must be nil for "remove" operation`)
		}
		if o.Value != nil {
			return errors.New(`"value" must be nil for "remove" operation`)
		}

	case OpReplace:
		if o.From != nil {
			return errors.New(`"from" must be nil for "replace" operation`)
		}

	case OpMove:
		if o.From == nil {
			return errors.New(`"from" must be non-nil for "move" operation`)
		}
		if o.Path == nil {
			return errors.New(`"path" must be non-nil for "move" operation`)
		}
		if o.Value != nil {
			return errors.New(`"value" must be nil for "move" operation`)
		}

	case OpCopy:
		if o.From == nil {
			return errors.New(`"from" must be non-nil for "copy" operation`)
		}
		if o.Path == nil {
			return errors.New(`"path" must be non-nil for "copy" operation`)
		}
		if o.Value != nil {
			return errors.New(`"from" must be nil for "copy" operation`)
		}

	case OpTest:
		if o.From != nil {
			return errors.New(`"from" must be nil for "test" operation`)
		}
	}

	return nil
}

func (op Op) Operation(from, path []any, value any) (*Operation, error) {
	o := &Operation{Op: op}
	var err error
	if from != nil {
		if o.From, err = PathFrom(from...); err != nil {
			return nil, err
		}
	}

	if path != nil {
		if o.Path, err = PathFrom(path...); err != nil {
			return nil, err
		}
	}

	if value != nil {
		if o.Value, err = cborMarshal(value); err != nil {
			return nil, err
		}
	}

	if err = o.Valid(); err != nil {
		return nil, err
	}
	return o, nil
}

type Path []rawKey

func PathFrom(keys ...any) (Path, error) {
	path := make(Path, len(keys))
	for i, key := range keys {
		data, err := cborMarshal(key)
		if err != nil {
			return nil, err
		}
		rk := rawKey(data)
		if !rk.isValid() {
			return nil, fmt.Errorf("%q can not be used as map key", ReadCBORType(data).String())
		}
		path[i] = rk
	}
	return path, nil
}

// String returns the Path as diagnostic notation (TODO).
func (p Path) String() string {
	if p == nil {
		return "null"
	}

	slice := make([]any, len(p))
	for i, k := range p {
		var v any
		if err := cborUnmarshal([]byte(k), &v); err != nil {
			v = fmt.Sprintf("h'%x'", []byte(k))
		}
		if ReadCBORType([]byte(k)) == CBORTypeByteString {
			v = fmt.Sprintf("h'%x'", v)
		}

		slice[i] = v
	}

	data, _ := json.Marshal(slice)
	return string(data)
}

func (p Path) withIndex(i int) Path {
	return p.withKey(rawKey(MustMarshal(i)))
}

func (p Path) withKey(key rawKey) Path {
	np := make(Path, len(p)+1)
	copy(np, p)
	np[len(np)-1] = key
	return np
}

// rawKey is a raw encoded CBOR value for map key.
type rawKey string

var minus = rawKey([]byte{0x61, 0x2d}) // "-"

func (k rawKey) isMinus() bool {
	return k == minus
}

func (k rawKey) isIndex() bool {
	if k.isMinus() {
		return true
	}

	if ty := ReadCBORType([]byte(k)); ty == CBORTypePositiveInt || ty == CBORTypeNegativeInt {
		return true
	}
	return false
}

func (k rawKey) isValid() bool {
	switch ty := ReadCBORType([]byte(k)); ty {
	default:
		return false

	case CBORTypePositiveInt, CBORTypeNegativeInt, CBORTypeTextString, CBORTypeByteString:
		return true
	}
}

func (k rawKey) toInt() (int, error) {
	if k.isMinus() {
		return -1, nil
	}

	var i int
	if err := cborUnmarshal([]byte(k), &i); err != nil {
		return -1, err
	}

	return i, nil
}

func (k rawKey) String() string {
	data := []byte(k)

	switch ReadCBORType(data) {
	case CBORTypePositiveInt:
		var v uint64
		if err := cborUnmarshal(data, &v); err == nil {
			return strconv.FormatUint(v, 10)
		}

	case CBORTypeNegativeInt:
		var v int64
		if err := cborUnmarshal(data, &v); err == nil {
			return strconv.FormatInt(v, 10)
		}

	case CBORTypeTextString:
		var v string
		if err := cborUnmarshal(data, &v); err == nil {
			return rfc6901Encoder.Replace(v)
		}

	case CBORTypeByteString:
		var v ByteString
		if err := cborUnmarshal(data, &v); err == nil {
			return base64.RawURLEncoding.EncodeToString(v.Bytes())
		}
	}

	return base64.RawURLEncoding.EncodeToString(data)
}

// MarshalCBOR returns m or CBOR nil if m is nil.
func (k rawKey) MarshalCBOR() ([]byte, error) {
	if len(k) == 0 {
		return []byte{60}, nil
	}
	return []byte(k), nil
}

// UnmarshalCBOR creates a copy of data and saves to *k.
func (k *rawKey) UnmarshalCBOR(data []byte) error {
	if k == nil {
		return errors.New("nil rawKey")
	}

	*k = rawKey(data)
	if !k.isValid() {
		return fmt.Errorf("%q can not be used as map key", ReadCBORType([]byte(*k)).String())
	}
	return nil
}

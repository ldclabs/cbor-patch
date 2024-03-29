// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"bytes"
	"errors"
	"fmt"
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
		return fmt.Errorf("invalid operation %q", o.Op)

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

type Path []RawKey

func PathFrom(keys ...any) (Path, error) {
	path := make(Path, len(keys))
	for i, key := range keys {
		data, err := cborMarshal(key)
		if err != nil {
			return nil, err
		}

		rk := RawKey(data)
		if err = rk.Valid(); err != nil {
			return nil, err
		}
		path[i] = rk
	}
	return path, nil
}

func PathMustFrom(keys ...any) Path {
	path, err := PathFrom(keys...)
	if err != nil {
		panic(err)
	}
	return path
}

// String returns the Path as CBOR diagnostic notation.
func (p Path) String() string {
	if p == nil {
		return "null"
	}

	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i, k := range p {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(k.String())
	}
	buf.WriteByte(']')
	return buf.String()
}

func (p Path) withIndex(i int) Path {
	return p.WithKey(RawKey(MustMarshal(i)))
}

func (p Path) WithKey(key RawKey) Path {
	np := make(Path, len(p)+1)
	copy(np, p)
	np[len(np)-1] = key
	return np
}

// rawKey is a raw encoded CBOR value for map key.
type RawKey string

var minus = RawKey([]byte{0x61, 0x2d}) // "-"

func (k RawKey) isMinus() bool {
	return k == minus
}

func (k RawKey) isIndex() bool {
	if k.isMinus() {
		return true
	}
	switch ReadCBORType([]byte(k)) {
	default:
		return false

	case CBORTypePositiveInt, CBORTypeNegativeInt:
		return true
	}
}

func (k RawKey) Valid() error {
	switch t := ReadCBORType([]byte(k)); t {
	default:
		return fmt.Errorf("%q can not be used as map key", t)

	case CBORTypePositiveInt, CBORTypeNegativeInt, CBORTypeTextString, CBORTypeByteString:
		return cborValid([]byte(k))
	}
}

func (k RawKey) toInt() (int, error) {
	if k.isMinus() {
		return -1, nil
	}

	var i int
	if err := cborUnmarshal([]byte(k), &i); err != nil {
		return -1, err
	}

	return i, nil
}

func (k RawKey) Bytes() []byte {
	return []byte(k)
}

func (k RawKey) Equal(other RawKey) bool {
	return k == other
}

func (k RawKey) Is(other any) bool {
	if data, err := cborMarshal(other); err == nil {
		return k.Equal(RawKey(data))
	}
	return false
}

// String returns the rawKey as CBOR diagnostic notation.
func (k RawKey) String() string {
	return Diagify([]byte(k))
}

// Key returns a string notation as JSON Object key.
func (k RawKey) Key() string {
	str := k.String()
	if len(str) > 1 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return str
}

// MarshalCBOR returns m or CBOR nil if m is nil.
func (k RawKey) MarshalCBOR() ([]byte, error) {
	if len(k) == 0 {
		return []byte{60}, nil
	}
	return []byte(k), nil
}

// UnmarshalCBOR creates a copy of data and saves to *k.
func (k *RawKey) UnmarshalCBOR(data []byte) error {
	if k == nil {
		return errors.New("nil RawKey")
	}

	*k = RawKey(data)
	return k.Valid()
}

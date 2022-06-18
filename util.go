// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import "encoding/json"

// FromJSON converts a JSON-encoded data to a CBOR-encoded data with a optional value as struct container.
// If v is not nil, it will decode data into v and then encode v to CBOR-encoded data.
func FromJSON(doc []byte, v interface{}) ([]byte, error) {
	if len(doc) == 0 {
		return doc, nil
	}

	if v == nil {
		var val interface{}
		v = &val
	}
	if err := json.Unmarshal(doc, v); err != nil {
		return nil, err
	}
	return cborMarshal(v)
}

// MustFromJSON converts a JSON-encoded string to a CBOR-encoded data.
// It will panic if converting failed.
func MustFromJSON(doc string) []byte {
	data, err := FromJSON([]byte(doc), nil)
	if err != nil {
		panic(err)
	}
	return data
}

// ToJSON converts a CBOR-encoded data to a JSON-encoded data with a optional value as struct container.
// If v is not nil, it will decode data into v and then encode v to JSON-encoded data.
func ToJSON(doc []byte, v interface{}) ([]byte, error) {
	if len(doc) == 0 {
		return doc, nil
	}

	if v != nil {
		if err := cborUnmarshal(doc, v); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	}

	return json.Marshal(NewNode(doc))
}

// MustToJSON converts a CBOR-encoded data to a JSON-encoded string.
// It will panic if converting failed.
func MustToJSON(doc []byte) string {
	data, err := ToJSON(doc, nil)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// Equal indicates if 2 CBOR documents have the same structural equality.
func Equal(a, b []byte) bool {
	return NewNode(a).equal(NewNode(b))
}

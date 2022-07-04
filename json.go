// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// FromJSON converts a JSON-encoded data to a CBOR-encoded data with a optional value as struct container.
// If v is not nil, it will decode data into v and then encode v to CBOR-encoded data.
// If v is nil, it will decode data with the following rules:
//
//     JSON booleans decode to bool.
//     JSON positive integers decode to uint64 (big.Int if value overflows).
//     JSON negative integers decode to int64 (big.Int if value overflows).
//     JSON floating points decode to float64.
//     JSON text strings decode to string.
//     JSON arrays decode to []interface{}.
//     JSON objects decode to map[string]interface{}.
//     JSON null decode to nil.
//
func FromJSON(doc []byte, v interface{}) ([]byte, error) {
	if len(doc) == 0 {
		return doc, nil
	}

	var err error
	if v == nil {
		if !json.Valid(doc) {
			return nil, fmt.Errorf("invalid JSON document")
		}

		dec := json.NewDecoder(bytes.NewReader(doc))
		dec.UseNumber()
		if v, err = readJSONValue(dec); err != nil {
			return nil, err
		}

	} else if err = json.Unmarshal(doc, v); err != nil {
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

func readJSONKey(dec *json.Decoder) (string, error) {
	t, err := dec.Token()
	if err != nil {
		return "", err
	}

	if key, ok := t.(string); ok {
		return key, nil
	}
	return "", fmt.Errorf("expected a string as key, got token %v", t)
}

func readJSONValue(dec *json.Decoder) (interface{}, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			obj := make(map[string]interface{})

			for dec.More() {
				key, err := readJSONKey(dec)
				if err != nil {
					return nil, err
				}
				val, err := readJSONValue(dec)
				if err != nil {
					return nil, err
				}
				obj[key] = val
			}
			// read '}'
			if _, err = dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil

		case '[':
			arr := make([]interface{}, 0)

			for dec.More() {
				val, err := readJSONValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			// read ']'
			if _, err = dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil

		default:
			return nil, fmt.Errorf("unexpected token %v", v)
		}

	case json.Number:
		return convertNumber(v)

	default:
		return v, nil
	}
}

func convertNumber(n json.Number) (interface{}, error) {
	switch s := string(n); {
	case strings.IndexRune(s, '.') != -1:
		return n.Float64()
	default:
		neg := s[0] == '-'
		if neg {
			s = s[1:]
		}

		u, err := strconv.ParseUint(s, 10, 64)
		if err == nil {
			switch {
			case !neg:
				return u, nil

			case u <= math.MaxInt64+1:
				return -int64(u), nil

			default:
				i := new(big.Int).SetUint64(u)
				return i.Neg(i), nil
			}
		}

		if errors.Unwrap(err) != strconv.ErrRange {
			return nil, err
		}

		i := new(big.Int)
		if _, ok := i.SetString(s, 10); !ok {
			return nil, fmt.Errorf("invalid number %q", string(n))
		}

		if neg {
			i.Neg(i)
		}
		return i, nil
	}
}

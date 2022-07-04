// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"
)

func TestConvertNumber(t *testing.T) {
	float64Cases := []struct {
		in  string
		out float64
	}{
		{"0.0", float64(0)},
		{"0.1", float64(0.1)},
		{"1.2", float64(1.2)},
		{"-1.2", float64(-1.2)},
		{"1.2e10", float64(1.2e10)},
		{"-1.2e10", float64(-1.2e10)},
		{"2.71828182845904523536028747135266249775724709369995957496696763e100",
			float64(2.71828182845904523536028747135266249775724709369995957496696763e100)},
		{"-2.71828182845904523536028747135266249775724709369995957496696763e100",
			float64(-2.71828182845904523536028747135266249775724709369995957496696763e100)},
	}
	for _, c := range float64Cases {
		got, err := convertNumber(json.Number(c.in))
		if err != nil {
			t.Errorf("convertNumber(%q) error: %v", c.in, err)
		}
		if got != c.out {
			t.Errorf("convertNumber(%q) = %v, want %v", c.in, got, c.out)
		}
	}

	uint64Cases := []struct {
		in  string
		out uint64
	}{
		{"0", uint64(0)},
		{"1", uint64(1)},
		{"20000", uint64(20000)},
		{"18446744073709551615", math.MaxUint64},
	}
	for _, c := range uint64Cases {
		got, err := convertNumber(json.Number(c.in))
		if err != nil {
			t.Errorf("convertNumber(%q) error: %v", c.in, err)
		}
		if got != c.out {
			t.Errorf("convertNumber(%q) = %v, want %v", c.in, got, c.out)
		}
	}

	int64Cases := []struct {
		in  string
		out int64
	}{
		{"-1", int64(-1)},
		{"-2", int64(-2)},
		{"-20000", int64(-20000)},
		{"-9223372036854775808", math.MinInt64},
	}
	for _, c := range int64Cases {
		got, err := convertNumber(json.Number(c.in))
		if err != nil {
			t.Errorf("convertNumber(%q) error: %v", c.in, err)
		}
		if got != c.out {
			t.Errorf("convertNumber(%q) = %v, want %v", c.in, got, c.out)
		}
	}

	bigInt1 := new(big.Int).SetUint64(1)
	bigIntCases := []struct {
		in  string
		out *big.Int
	}{
		{"18446744073709551616", new(big.Int).Add(new(big.Int).SetUint64(math.MaxUint64), bigInt1)},
		{"-9223372036854775809", new(big.Int).Sub(new(big.Int).SetInt64(math.MinInt64), bigInt1)},
	}
	for _, c := range bigIntCases {
		got, err := convertNumber(json.Number(c.in))
		if err != nil {
			t.Errorf("convertNumber(%q) error: %v", c.in, err)
		}
		if b, ok := got.(*big.Int); !ok || b.Cmp(c.out) != 0 {
			t.Errorf("convertNumber(%q) = %v, want %v", c.in, got, c.out)
		}
	}
}

# CBOR-Patch
[![ci](https://github.com/ldclabs/cbor-patch/actions/workflows/ci.yml/badge.svg)](https://github.com/ldclabs/cbor-patch/actions?query=workflow%3Aci)
[![codecov](https://codecov.io/gh/ldclabs/cbor-patch/branch/main/graph/badge.svg?token=2G1SE83FY5)](https://codecov.io/gh/ldclabs/cbor-patch)

`cborpatch` is a library which provides functionality for applying
[RFC6902 JSON patches](https://datatracker.ietf.org/doc/html/rfc6902) on [CBOR](https://datatracker.ietf.org/doc/html/rfc8949).

## Documentation

[Go-Documentation](https://pkg.go.dev/github.com/ldclabs/cbor-patch)

CBOR-Patch only supports string keys in a map.

## Import

```go
// package cborpatch
import "github.com/ldclabs/cbor-patch"
```


## Examples

### Create and apply a CBOR Patch

```go
package main

import (
	"fmt"

	cborpatch "github.com/ldclabs/cbor-patch"
)

func main() {
	original := cborpatch.MustFromJSON(`{"name": "John", "age": 24, "height": 3.21}`)
	fmt.Printf("%x\n", original)
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	patchDoc := cborpatch.MustFromJSON(`[
		{"op": "replace", "path": "/name", "value": "Jane"},
		{"op": "remove", "path": "/height"}
	]`)

	patch, err := cborpatch.NewPatch(patchDoc)
	if err != nil {
		panic(err)
	}
	modified, err := patch.Apply(original)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", modified)
	// a263616765f94e00646e616d65644a616e65
	fmt.Printf("%s\n", cborpatch.MustToJSON(modified))
	// {"age":24,"name":"Jane"}
}
```

### Create a Node and apply more Patchs

```go
package main

import (
	"fmt"

	cborpatch "github.com/ldclabs/cbor-patch"
)

func main() {
	original := cborpatch.MustFromJSON(`{"name": "John", "age": 24, "height": 3.21}`)
	fmt.Printf("%x\n", original)
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	patchDoc0 := cborpatch.MustFromJSON(`[
		{"op": "replace", "path": "/name", "value": "Jane"},
		{"op": "remove", "path": "/height"}
	]`)
	patchDoc1 := cborpatch.MustFromJSON(`[
		{"op": "replace", "path": "/age", "value": 25}
	]`)

	node := cborpatch.NewNode(original)
	patch, err := cborpatch.NewPatch(patchDoc0)
	if err != nil {
		panic(err)
	}
	err = node.Patch(patch, nil)
	if err != nil {
		panic(err)
	}
	modified, err := node.MarshalCBOR()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", modified)
	// a263616765f94e00646e616d65644a616e65
	modified, err = node.MarshalJSON()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", string(modified))
	// {"age":24,"name":"Jane"}

	patch, err = cborpatch.NewPatch(patchDoc1)
	if err != nil {
		panic(err)
	}
	err = node.Patch(patch, nil)
	if err != nil {
		panic(err)
	}
	modified, err = node.MarshalCBOR()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", modified)
	// a263616765f94e40646e616d65644a616e65
	modified, err = node.MarshalJSON()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", string(modified))
	// {"age":25,"name":"Jane"}
}
```

### Get value by path

```go
package main

import (
	"fmt"

	cborpatch "github.com/ldclabs/cbor-patch"
)

func main() {
	doc := cborpatch.MustFromJSON(`{
		"baz": "qux",
		"foo": [ "a", 2, "c" ]
	}`)
	node := cborpatch.NewNode(doc)

	value, err := node.GetValue("/foo/0", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", value)
	// 6161
	fmt.Printf("%s\n", cborpatch.MustToJSON(value))
	// "a"
}
```

### Find children by test operations

```go
package main

import (
	"fmt"

	cborpatch "github.com/ldclabs/cbor-patch"
)

func main() {
	doc := cborpatch.MustFromJSON(`["root", ["p",
		["span", {"data-type": "text"},
			["span", {"data-type": "leaf"}, "Hello 1"],
			["span", {"data-type": "leaf"}, "Hello 2"],
			["span", {"data-type": "leaf"}, "Hello 3"],
			["span", {"data-type": null}, "Hello 4"]
		]
	]]`)

	node := cborpatch.NewNode(doc)
	tests := cborpatch.PVs{
		{"/0", cborpatch.MustFromJSON(`"span"`)},
		{"/1/data-type", cborpatch.MustFromJSON(`"leaf"`)},
	}

	result, err := node.FindChildren(tests, nil)
	if err != nil {
		panic(err)
	}
	for _, r := range result {
		fmt.Printf("Path: \"%s\", Value: %x, JSON: %s\n", r.Path, r.Value, cborpatch.MustToJSON(r.Value))
	}

	// Output:
	// Path: "/1/1/2", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2031, JSON: ["span",{"data-type":"leaf"},"Hello 1"]
	// Path: "/1/1/3", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2032, JSON: ["span",{"data-type":"leaf"},"Hello 2"]
	// Path: "/1/1/4", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2033, JSON: ["span",{"data-type":"leaf"},"Hello 3"]
}
```
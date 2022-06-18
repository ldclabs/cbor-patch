// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import "fmt"

func ExamplePatch_Apply() {
	original := MustFromJSON(`{"name": "John", "age": 24, "height": 3.21}`)
	fmt.Printf("%x\n", original)
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	patchDoc := MustFromJSON(`[
		{"op": "replace", "path": "/name", "value": "Jane"},
		{"op": "remove", "path": "/height"}
	]`)

	patch, err := NewPatch(patchDoc)
	if err != nil {
		panic(err)
	}
	modified, err := patch.Apply(original)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", modified)
	// a263616765f94e00646e616d65644a616e65
	fmt.Printf("%s\n", MustToJSON(modified))
	// {"age":24,"name":"Jane"}

	// Output:
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	// a263616765f94e00646e616d65644a616e65
	// {"age":24,"name":"Jane"}
}

func ExampleNode_Patch() {
	original := MustFromJSON(`{"name": "John", "age": 24, "height": 3.21}`)
	fmt.Printf("%x\n", original)
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	patchDoc0 := MustFromJSON(`[
		{"op": "replace", "path": "/name", "value": "Jane"},
		{"op": "remove", "path": "/height"}
	]`)
	patchDoc1 := MustFromJSON(`[
		{"op": "replace", "path": "/age", "value": 25}
	]`)

	node := NewNode(original)
	patch, err := NewPatch(patchDoc0)
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

	patch, err = NewPatch(patchDoc1)
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

	// Output:
	// a363616765f94e00646e616d65644a6f686e66686569676874fb4009ae147ae147ae
	// a263616765f94e00646e616d65644a616e65
	// {"age":24,"name":"Jane"}
	// a263616765f94e40646e616d65644a616e65
	// {"age":25,"name":"Jane"}
}

func ExampleNode_GetValue() {
	doc := MustFromJSON(`{
		"baz": "qux",
		"foo": [ "a", 2, "c" ]
	}`)
	node := NewNode(doc)

	value, err := node.GetValue("/foo/0", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", value)
	// 6161
	fmt.Printf("%s\n", MustToJSON(value))
	// "a"

	// Output:
	// 6161
	// "a"
}

func ExampleNode_FindChildren() {
	doc := MustFromJSON(`["root", ["p",
		["span", {"data-type": "text"},
			["span", {"data-type": "leaf"}, "Hello 1"],
			["span", {"data-type": "leaf"}, "Hello 2"],
			["span", {"data-type": "leaf"}, "Hello 3"],
			["span", {"data-type": null}, "Hello 4"]
		]
	]]`)

	node := NewNode(doc)
	tests := PVs{
		{"/0", MustFromJSON(`"span"`)},
		{"/1/data-type", MustFromJSON(`"leaf"`)},
	}

	result, err := node.FindChildren(tests, nil)
	if err != nil {
		panic(err)
	}
	for _, r := range result {
		fmt.Printf("Path: \"%s\", Value: %x, JSON: %s\n", r.Path, r.Value, MustToJSON(r.Value))
	}

	// Output:
	// Path: "/1/1/2", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2031, JSON: ["span",{"data-type":"leaf"},"Hello 1"]
	// Path: "/1/1/3", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2032, JSON: ["span",{"data-type":"leaf"},"Hello 2"]
	// Path: "/1/1/4", Value: 83647370616ea169646174612d74797065646c6561666748656c6c6f2033, JSON: ["span",{"data-type":"leaf"},"Hello 3"]
}

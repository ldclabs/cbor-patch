// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type GetValueCase struct {
	doc, path string
	result    []byte
	err       string
}

var GetValueCases = []GetValueCase{
	{
		`{ "baz": "qux" }`,
		"/baz",
		[]byte(`"qux"`),
		"",
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		"/foo/0",
		[]byte(`"a"`),
		"",
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		"/foo/1",
		[]byte(`2`),
		"",
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c", {"baz": null} ]
	  }`,
		"/foo/3/baz",
		[]byte(`null`),
		"",
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c", {"baz": null}, null ]
	  }`,
		"/foo/4",
		[]byte(`null`),
		"",
	},
	{
		`{ "foo": {} }`,
		"/foo",
		[]byte(`{}`),
		"",
	},
	{
		`{ "foo": [ ] }`,
		"/foo",
		[]byte(`[]`),
		"",
	},
	{
		`{ "foo": null }`,
		"/foo",
		[]byte(`null`),
		"",
	},
	{
		`{ "baz/foo": "qux" }`,
		"/baz~1foo",
		[]byte(`"qux"`),
		"",
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		"/fooo",
		nil,
		`unable to get nonexistent key "fooo", missing value`,
	},
}

func TestGetValueByPath(t *testing.T) {
	assert := assert.New(t)

	for _, c := range GetValueCases {
		res, err := GetValueByPath(MustFromJSON(c.doc), c.path)
		switch {
		case c.err != "":
			assert.ErrorContains(err, c.err,
				"Testing failed when it should have error for [%s]: expected [%s], got [%v]",
				string(c.doc), c.err, err)
		case err != nil:
			assert.NoError(err,
				"Testing failed when it should have passed for [%s]: %v",
				string(c.doc), err)
		default:
			assert.Equal(string(c.result), MustToJSON(res),
				"Testing failed for [%s]: expected [%s], got [%s]",
				string(c.doc), string(c.result), string(res))
		}
	}
}

func TestGetValueByCBORPath(t *testing.T) {
	assert := assert.New(t)

	obj := map[any]any{
		"baz": "qux",
		1:     1,
		-1:    -1,
		true:  false,
		// [4]byte{0, 0, 0, 0}: []byte{0, 0, 0, 0},
		string([]byte{0, 0, 0, 0}): map[string]string{
			string([]byte{0, 0, 0, 0}): string([]byte{0, 0, 0, 0}),
		},
	}
	data, err := cborMarshal(obj)
	assert.NoError(err)

	val, err := GetValueByPath(data, "/baz")
	assert.NoError(err)
	assert.Equal(MustMarshal("qux"), val)
}

type FindChildrenCase struct {
	doc    []byte
	tests  []*PV
	result []*PV
}

var FindChildrenCases = []FindChildrenCase{
	{
		MustFromJSON(`{ "baz": "qux" }`),
		[]*PV{{"/baz", MustFromJSON(`"qux"`)}},
		[]*PV{{"", MustFromJSON(`{"baz": "qux"}`)}},
	},
	{
		MustFromJSON(`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`),
		[]*PV{{"/foo/0", MustFromJSON(`"a"`)}},
		[]*PV{{"", MustFromJSON(`{
				"baz": "qux",
				"foo": [ "a", 2, "c" ]
			}`),
		}},
	},
	{
		MustFromJSON(`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`),
		[]*PV{{"/1", MustFromJSON(`2`)}},
		[]*PV{{"/foo", MustFromJSON(`[ "a", 2, "c" ]`)}},
	},
	{
		MustFromJSON(`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`),
		[]*PV{{"/fooo", nil}},
		[]*PV{},
	},
	{
		MustFromJSON(`{ "foo": {} }`),
		[]*PV{{"/foo", MustFromJSON(`{}`)}},
		[]*PV{{"", MustFromJSON(`{ "foo": {} }`)}},
	},
	{
		MustFromJSON(`{ "foo": [ ] }`),
		[]*PV{{"/foo", MustFromJSON(`[]`)}},
		[]*PV{{"", MustFromJSON(`{ "foo": [ ] }`)}},
	},
	{
		MustFromJSON(`{ "foo": null }`),
		[]*PV{{"/foo", nil}},
		[]*PV{{"", MustFromJSON(`{ "foo": null }`)}},
	},
	{
		MustFromJSON(`{ "foo": null }`),
		[]*PV{{"/foo", MustFromJSON("")}},
		[]*PV{{"", MustFromJSON(`{ "foo": null }`)}},
	},
	{
		MustFromJSON(`{ "foo": null }`),
		[]*PV{{"/foo", MustFromJSON("null")}},
		[]*PV{{"", MustFromJSON(`{ "foo": null }`)}},
	},
	{
		MustFromJSON(`{ "foo": "" }`),
		[]*PV{{"/foo", MustFromJSON(`""`)}},
		[]*PV{{"", MustFromJSON(`{ "foo": "" }`)}},
	},
	{
		MustFromJSON(`{ "baz/foo": "qux" }`),
		[]*PV{{"/baz~1foo", MustFromJSON(`"qux"`)}},
		[]*PV{{"", MustFromJSON(`{ "baz/foo": "qux" }`)}},
	},
	{
		MustFromJSON(`{ "baz/foo": [ "qux" ] }`),
		[]*PV{{"/0", MustFromJSON(`"qux"`)}},
		[]*PV{{"/baz~1foo", MustFromJSON(`["qux"]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`),
		[]*PV{{"/0", MustFromJSON(`"object"`)}},
		[]*PV{
			{"/1", MustFromJSON(`["object", { "id": "id1" }]`)},
			{"/2", MustFromJSON(`["object", { "id": "id2" }]`)},
		},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`),
		[]*PV{{"/1/id", MustFromJSON(`"id1"`)}},
		[]*PV{{"/1", MustFromJSON(`["object", { "id": "id1" }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`),
		[]*PV{{"/1", MustFromJSON(`{ "id": "id1" }`)}},
		[]*PV{{"/1", MustFromJSON(`["object", { "id": "id1" }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`),
		[]*PV{{"/1/id", MustFromJSON(`""`)}},
		[]*PV{{"/1", MustFromJSON(`["object", { "id": "" }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`),
		[]*PV{{"/1/id", MustFromJSON(`null`)}},
		[]*PV{{"/2", MustFromJSON(`["object", { "id": null }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`),
		[]*PV{{"/1/id", MustFromJSON(`null`)}},
		[]*PV{{"/2", MustFromJSON(`["object", { "id": null }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`),
		[]*PV{{"/1/id", MustFromJSON(`""`)}},
		[]*PV{{"/1", MustFromJSON(`["object", { "id": "" }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`),
		[]*PV{
			{"/0", MustFromJSON(`"object2"`)},
			{"/1/id", MustFromJSON(`null`)},
		},
		[]*PV{{"/2", MustFromJSON(`["object2", { "id": null }]`)}},
	},
	{
		MustFromJSON(`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`),
		[]*PV{
			{"/0", MustFromJSON(`"root"`)},
			{"/1/0", MustFromJSON(`"object1"`)},
			{"/1/1/id", MustFromJSON(`""`)},
		},
		[]*PV{{"", MustFromJSON(`[
				"root",
				["object1", { "id": "" }],
				["object2", { "id": null }]
			]`)},
		},
	},
	{
		MustFromJSON(`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`),
		[]*PV{
			{"/0", MustFromJSON(`"root"`)},
			{"/1/0", MustFromJSON(`"object1"`)},
			{"/1/1/id", MustFromJSON(`""`)},
			{"/2", MustFromJSON(`["object2", { "id": null }]`)},
		},
		[]*PV{
			{"", MustFromJSON(`[
				"root",
				["object1", { "id": "" }],
				["object2", { "id": null }]
			]`)},
		},
	},
	{
		MustFromJSON(`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`),
		[]*PV{{"/0", MustFromJSON(`"span"`)}, {"/1/data-type", MustFromJSON(`"leaf"`)}},
		[]*PV{
			{"/1/1/2", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 1"]`)},
			{"/1/1/3", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 2"]`)},
			{"/1/1/4", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 3"]`)},
		},
	},
	{
		MustFromJSON(`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`),
		[]*PV{{"/0", MustFromJSON(`"span"`)}, {"/1/data-type", nil}},
		[]*PV{{"/1/1/5", MustFromJSON(`["span", {"data-type": null}, "Hello 4"]`)}},
	},
	{
		MustFromJSON(`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`),
		[]*PV{{"/0", MustFromJSON(`"span"`)}},
		[]*PV{
			{"/1/1", MustFromJSON(`["span", {"data-type": "text"},
			["span", {"data-type": "leaf"}, "Hello 1"],
			["span", {"data-type": "leaf"}, "Hello 2"],
			["span", {"data-type": "leaf"}, "Hello 3"],
			["span", {"data-type": null}, "Hello 4"]]`)},
			{"/1/1/2", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 1"]`)},
			{"/1/1/3", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 2"]`)},
			{"/1/1/4", MustFromJSON(`["span", {"data-type": "leaf"}, "Hello 3"]`)},
			{"/1/1/5", MustFromJSON(`["span", {"data-type": null}, "Hello 4"]`)},
		},
	},
}

func TestFindChildren(t *testing.T) {
	assert := assert.New(t)

	for i, c := range FindChildrenCases {
		res, err := NewNode(c.doc).FindChildren(c.tests, nil)
		assert.NoError(err,
			"Testing failed when case %d should have passed: %s",
			i, err)

		assert.Equal(len(c.result), len(res),
			"Testing failed for case %d, %v: expected %d, got %d",
			i, MustToJSON(c.doc), len(c.result), len(res))

		for j := range res {
			assert.Equal(c.result[j].Path, res[j].Path,
				"Testing failed for case %d, %v: expected path [%s], got [%s]",
				i, string(c.doc), c.result[j].Path, res[j].Path)

			assert.Equal(c.result[j].Value, res[j].Value,
				"Testing failed for case %d, %v: expected [%s], got [%s]",
				i, MustToJSON(c.doc), MustToJSON(c.result[j].Value), MustToJSON(res[j].Value))
		}
	}
}

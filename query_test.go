// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type GetValueByPathCase struct {
	doc, path string
	result    []byte
	err       string
}

var GetValueByPathCases = []GetValueByPathCase{
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
		`unable to get nonexistent key "fooo": missing value`,
	},
}

func TestGetValueByPath(t *testing.T) {
	for _, c := range GetValueByPathCases {
		res, err := GetValueByPath(mustFromJSONData(c.doc), c.path, nil)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("Testing failed when it should have error for [%s]: expected [%s], got [%v]",
					string(c.doc), c.err, err)
			}
		} else if err != nil {
			t.Errorf("Testing failed when it should have passed for [%s]: %v", string(c.doc), err)
		} else {
			res = mustToJSONData(res)
			if !bytes.Equal(res, c.result) {
				t.Errorf("Testing failed for [%s]: expected [%s], got [%s]", string(c.doc), string(c.result), string(res))
			}
		}
	}
}

type jsonFilter map[string]json.RawMessage
type jsonNode struct {
	Path  string
	Value json.RawMessage
}
type FindChildrenByFiltersCase struct {
	doc     string
	filters []jsonFilter
	result  []*jsonNode
}

func (fc *FindChildrenByFiltersCase) Filters() []Filter {
	fs := make([]Filter, len(fc.filters))
	for i, f := range fc.filters {
		fs[i] = make(Filter, len(f))
		for k, v := range f {
			fs[i][k] = mustFromJSONData(string(v))
		}
	}
	return fs
}

var FindChildrenByFiltersCases = []FindChildrenByFiltersCase{
	{
		`{ "baz": "qux" }`,
		[]jsonFilter{{"/baz": []byte(`"qux"`)}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{"baz": "qux"}`),
		}},
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		[]jsonFilter{{"/foo/0": []byte(`"a"`)}},
		[]*jsonNode{{
			Path: "",
			Value: []byte(`{
				"baz": "qux",
				"foo": [ "a", 2, "c" ]
			}`),
		}},
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		[]jsonFilter{{"/1": []byte(`2`)}},
		[]*jsonNode{{
			Path:  "/foo",
			Value: []byte(`[ "a", 2, "c" ]`),
		}},
	},
	{
		`{
	    "baz": "qux",
	    "foo": [ "a", 2, "c" ]
	  }`,
		[]jsonFilter{{"/fooo": nil}},
		[]*jsonNode{},
	},
	{
		`{ "foo": {} }`,
		[]jsonFilter{{"/foo": []byte(`{}`)}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": {} }`),
		}},
	},
	{
		`{ "foo": [ ] }`,
		[]jsonFilter{{"/foo": []byte(`[]`)}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": [ ] }`),
		}},
	},
	{
		`{ "foo": null }`,
		[]jsonFilter{{"/foo": nil}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": null }`),
		}},
	},
	{
		`{ "foo": null }`,
		[]jsonFilter{{"/foo": []byte("")}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": null }`),
		}},
	},
	{
		`{ "foo": null }`,
		[]jsonFilter{{"/foo": []byte("null")}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": null }`),
		}},
	},
	{
		`{ "foo": "" }`,
		[]jsonFilter{{"/foo": []byte(`""`)}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "foo": "" }`),
		}},
	},
	{
		`{ "baz/foo": "qux" }`,
		[]jsonFilter{{"/baz~1foo": []byte(`"qux"`)}},
		[]*jsonNode{{
			Path:  "",
			Value: []byte(`{ "baz/foo": "qux" }`),
		}},
	},
	{
		`{ "baz/foo": [ "qux" ] }`,
		[]jsonFilter{{"/0": []byte(`"qux"`)}},
		[]*jsonNode{{
			Path:  "/baz~1foo",
			Value: []byte(`["qux"]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`,
		[]jsonFilter{{"/0": []byte(`"object"`)}},
		[]*jsonNode{{
			Path:  "/1",
			Value: []byte(`["object", { "id": "id1" }]`),
		}, {
			Path:  "/2",
			Value: []byte(`["object", { "id": "id2" }]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`,
		[]jsonFilter{{"/1/id": []byte(`"id1"`)}},
		[]*jsonNode{{
			Path:  "/1",
			Value: []byte(`["object", { "id": "id1" }]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "id1" }],
			["object", { "id": "id2" }]
		]`,
		[]jsonFilter{{"/1": []byte(`{ "id": "id1" }`)}},
		[]*jsonNode{{
			Path:  "/1",
			Value: []byte(`["object", { "id": "id1" }]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`,
		[]jsonFilter{{"/1/id": []byte(`""`)}},
		[]*jsonNode{{
			Path:  "/1",
			Value: []byte(`["object", { "id": "" }]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`,
		[]jsonFilter{{"/1/id": []byte(`null`)}},
		[]*jsonNode{{
			Path:  "/2",
			Value: []byte(`["object", { "id": null }]`),
		}},
	},
	{
		`[
			"root",
			["object", { "id": "" }],
			["object", { "id": null }]
		]`,
		[]jsonFilter{{"/1/id": []byte(`null`)}, {"/1/id": []byte(`""`)}},
		[]*jsonNode{{
			Path:  "/2",
			Value: []byte(`["object", { "id": null }]`),
		}, {
			Path:  "/1",
			Value: []byte(`["object", { "id": "" }]`),
		}},
	},
	{
		`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`,
		[]jsonFilter{{"/0": []byte(`"object2"`), "/1/id": []byte(`null`)}},
		[]*jsonNode{{
			Path:  "/2",
			Value: []byte(`["object2", { "id": null }]`),
		}},
	},
	{
		`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`,
		[]jsonFilter{{"/0": []byte(`"root"`), "/1/0": []byte(`"object1"`), "/1/1/id": []byte(`""`)}},
		[]*jsonNode{{
			Path: "",
			Value: []byte(`[
				"root",
				["object1", { "id": "" }],
				["object2", { "id": null }]
			]`),
		}},
	},
	{
		`[
			"root",
			["object1", { "id": "" }],
			["object2", { "id": null }]
		]`,
		[]jsonFilter{{"/0": []byte(`"root"`), "/1/0": []byte(`"object1"`), "/1/1/id": []byte(`""`), "/2": []byte(`["object2", { "id": null }]`)}},
		[]*jsonNode{{
			Path: "",
			Value: []byte(`[
				"root",
				["object1", { "id": "" }],
				["object2", { "id": null }]
			]`),
		}},
	},
	{
		`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`,
		[]jsonFilter{{"/0": []byte(`"span"`), "/1/data-type": []byte(`"leaf"`)}},
		[]*jsonNode{{
			Path:  "/1/1/2",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 1"]`),
		}, {
			Path:  "/1/1/3",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 2"]`),
		}, {
			Path:  "/1/1/4",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 3"]`),
		}},
	},
	{
		`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`,
		[]jsonFilter{{"/0": []byte(`"span"`), "/1/data-type": nil}},
		[]*jsonNode{{
			Path:  "/1/1/5",
			Value: []byte(`["span", {"data-type": null}, "Hello 4"]`),
		}},
	},
	{
		`["root", ["p",
			["span", {"data-type": "text"},
				["span", {"data-type": "leaf"}, "Hello 1"],
				["span", {"data-type": "leaf"}, "Hello 2"],
				["span", {"data-type": "leaf"}, "Hello 3"],
				["span", {"data-type": null}, "Hello 4"]
			]
		]]`,
		[]jsonFilter{{"/0": []byte(`"span"`)}},
		[]*jsonNode{{
			Path: "/1/1",
			Value: []byte(`["span", {"data-type": "text"},
			["span", {"data-type": "leaf"}, "Hello 1"],
			["span", {"data-type": "leaf"}, "Hello 2"],
			["span", {"data-type": "leaf"}, "Hello 3"],
			["span", {"data-type": null}, "Hello 4"]
		]`),
		}, {
			Path:  "/1/1/2",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 1"]`),
		}, {
			Path:  "/1/1/3",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 2"]`),
		}, {
			Path:  "/1/1/4",
			Value: []byte(`["span", {"data-type": "leaf"}, "Hello 3"]`),
		}, {
			Path:  "/1/1/5",
			Value: []byte(`["span", {"data-type": null}, "Hello 4"]`),
		}},
	},
}

func TestFindChildrenByQuery(t *testing.T) {
	for i, c := range FindChildrenByFiltersCases {
		res, err := FindChildrenByFilters(mustFromJSONData(c.doc), c.Filters(), nil)

		if err != nil {
			t.Errorf("Testing failed when case %d should have passed: %s", i, err)
		} else {
			if len(res) != len(c.result) {
				t.Errorf("Testing failed for case %d, %v, %s: expected %#v, got %#v", i, string(c.doc), c.filters, c.result, res)
			}
			for j := range res {
				if c.result[j].Path != res[j].Path {
					t.Errorf("Testing failed for case %d, %v, %s: expected path [%s], got [%s]", i, string(c.doc), c.filters, c.result[j].Path, res[j].Path)
				} else if !Equal(mustFromJSONData(string(c.result[j].Value)), res[j].Value) {
					t.Errorf("Testing failed for case %d, %v, %s: expected path [%s], got [%s]", i, string(c.doc), c.filters, string(c.result[j].Value), string(mustToJSONData(res[j].Value)))
				}
			}
		}
	}
}

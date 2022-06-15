// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cborpatch

import (
	"fmt"
	"strconv"
	"strings"
)

func GetValueByPath(doc []byte, path string, options *ApplyOptions) ([]byte, error) {
	if len(doc) == 0 {
		return doc, nil
	}

	var pd container
	switch t := ReadCBORType(doc); t {
	case CBORTypeMap:
		pd = &partialDoc{}
	case CBORTypeArray:
		pd = &partialArray{}
	default:
		return nil, fmt.Errorf("unexpected document type: %s", t.String())
	}

	err := cborUnmarshal(doc, pd)
	if err != nil {
		return nil, err
	}

	if options == nil {
		options = NewApplyOptions()
	}
	con, key := findObject(&pd, path, options)
	if con == nil {
		return nil, fmt.Errorf("get value by %s error: %v", strconv.Quote(path), ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil {
		return nil, fmt.Errorf("get value by %s error: %v", strconv.Quote(path), err)
	}
	return val.MarshalCBOR()
}

type ChildNode struct {
	Path  string     `cbor:"path"`
	Value RawMessage `cbor:"value"`
	node  *lazyNode
}

type Filter map[string]RawMessage

func FindChildrenByFilters(doc []byte, filters []Filter, options *ApplyOptions) (nodes []*ChildNode, err error) {
	if len(filters) == 0 {
		return
	}
	if options == nil {
		options = NewApplyOptions()
	}
	dn := newLazyNode(newRawMessage(doc))
	for _, filter := range filters {
		keys := make([]string, 0, len(filter))
		for querypath := range filter {
			keys = append(keys, querypath)
		}
		if len(keys) == 0 {
			continue
		}

		subpaths, err := toSubpaths(keys[0])
		if err != nil {
			return nil, err
		}
		value := filter[keys[0]]
		v := newLazyNode(&value)
		keys = keys[1:]

		ns, err := findChildNodes(dn, v, "", subpaths, options)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			subpaths, err := toSubpaths(key)
			if err != nil {
				return nil, err
			}
			value := filter[key]
			v := newLazyNode(&value)
			_ns := make([]*ChildNode, 0, len(ns))
			for _, n := range ns {
				if assertObject(n.node, subpaths, v, options) {
					_ns = append(_ns, n)
				}
			}
			ns = _ns
			if len(ns) == 0 {
				break
			}
		}
		nodes = append(nodes, ns...)
	}
	return
}

func toSubpaths(s string) ([]string, error) {
	subpaths := strings.Split(s, "/")
	if len(subpaths) < 2 || subpaths[0] != "" {
		return nil, fmt.Errorf("invalid query path: %s", s)
	}
	return subpaths[1:], nil
}

func findChildNodes(
	node, value *lazyNode, parentpath string, subpaths []string, options *ApplyOptions,
) (res []*ChildNode, err error) {

	node.intoContainer()
	if node.which == eOther {
		return
	}

	if assertObject(node, subpaths, value, options) {
		res = append(res, &ChildNode{Path: parentpath, Value: *node.raw, node: node})
	}

	if node.which == eAry {
		for i, n := range node.ary {
			if n == nil {
				continue
			}
			r, e := findChildNodes(n, value, fmt.Sprintf("%s/%d", parentpath, i), subpaths, options)
			if e != nil {
				return nil, e
			}
			if len(r) > 0 {
				res = append(res, r...)
			}
		}
	} else {
		for k, n := range node.doc.obj {
			if n == nil {
				continue
			}
			r, e := findChildNodes(n, value, fmt.Sprintf("%s/%s", parentpath, encodePatchKey(k)), subpaths, options)
			if e != nil {
				return nil, e
			}
			if len(r) > 0 {
				res = append(res, r...)
			}
		}
	}
	return
}

func assertObject(node *lazyNode, subpaths []string, value *lazyNode, options *ApplyOptions) bool {
	last := len(subpaths) - 1
	doc := node.intoContainer()
	if doc == nil {
		return false
	}

	for i, part := range subpaths {
		next, ok := doc.get(decodePatchKey(part), options)
		if ok != nil {
			return false
		}
		if i == last {
			if next == nil {
				return value.isNull()
			}
			return next.equal(value)
		}

		if next == nil {
			return false
		}

		doc = next.intoContainer()
		if doc == nil {
			return false
		}
	}
	return false
}

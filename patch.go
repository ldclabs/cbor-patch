// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
//
// This file is a derived work, based on the github.com/evanphx/json-patch whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright (c) 2014, Evan Phoenix
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
// * Neither the name of the Evan Phoenix nor the names of its contributors
//   may be used to endorse or promote products derived from this software
//   without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package cborpatch

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// SupportNegativeIndices decides whether to support non-standard practice of
	// allowing negative indices to mean indices starting at the end of an array.
	// Default to true.
	SupportNegativeIndices bool = true
	// AccumulatedCopySizeLimit limits the total size increase in bytes caused by
	// "copy" operations in a patch.
	AccumulatedCopySizeLimit int64 = 0
)

var (
	ErrMissing      = errors.New("missing value")
	ErrUnknownType  = errors.New("unknown object type")
	ErrInvalid      = errors.New("invalid node detected")
	ErrInvalidIndex = errors.New("invalid index referenced")
)

const (
	eRaw = iota
	eDoc
	eAry
	eOther
)

// Equal indicates if 2 CBOR documents have the same structural equality.
func Equal(a, b []byte) bool {
	return NewNode(a).Equal(NewNode(b))
}

// Operation is a single CBOR-Patch step, such as a single 'add' operation.
type Operation struct {
	Op    string     `cbor:"op"`
	Path  string     `cbor:"path"`
	From  string     `cbor:"from,omitempty"`
	Value RawMessage `cbor:"value,omitempty"`
}

// Patch is an ordered collection of Operations.
type Patch []Operation

// Options specifies options for calls to ApplyWithOptions.
// Use NewOptions to obtain default values for Options.
type Options struct {
	// SupportNegativeIndices decides whether to support non-standard practice of
	// allowing negative indices to mean indices starting at the end of an array.
	// Default to true.
	SupportNegativeIndices bool
	// AccumulatedCopySizeLimit limits the total size increase in bytes caused by
	// "copy" operations in a patch.
	AccumulatedCopySizeLimit int64
	// AllowMissingPathOnRemove indicates whether to fail "remove" operations when the target path is missing.
	// Default to false.
	AllowMissingPathOnRemove bool
	// EnsurePathExistsOnAdd instructs cbor-patch to recursively create the missing parts of path on "add" operation.
	// Default to false.
	EnsurePathExistsOnAdd bool
}

// NewOptions creates a default set of options for calls to ApplyWithOptions.
func NewOptions() *Options {
	return &Options{
		SupportNegativeIndices:   SupportNegativeIndices,
		AccumulatedCopySizeLimit: AccumulatedCopySizeLimit,
		AllowMissingPathOnRemove: false,
		EnsurePathExistsOnAdd:    false,
	}
}

// NewPatch decodes the passed CBOR document as an RFC 6902 patch.
func NewPatch(doc []byte) (Patch, error) {
	var p Patch

	err := cborUnmarshal(doc, &p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Apply mutates a CBOR document according to the patch, and returns the new document.
func (p Patch) Apply(doc []byte) ([]byte, error) {
	return p.ApplyWithOptions(doc, NewOptions())
}

// ApplyWithOptions mutates a CBOR document according to the patch and the passed in Options.
// It returns the new document.
func (p Patch) ApplyWithOptions(doc []byte, options *Options) ([]byte, error) {
	node := NewNode(doc)
	if err := node.Patch(p, options); err != nil {
		return nil, err
	}
	return node.MarshalCBOR()
}

// Node represents a lazy parsing CBOR document.
type Node struct {
	raw   *RawMessage
	doc   *partialDoc
	ary   partialArray
	ty    CBORType
	which int
}

// NewNode returns a new Node with the given raw encoded CBOR document.
// A nil or empty raw document is equal to CBOR null.
func NewNode(doc RawMessage) *Node {
	if len(doc) == 0 {
		doc = rawCBORNull
	}
	raw := make(RawMessage, len(doc))
	copy(raw, doc)
	return &Node{raw: &raw, ty: CBORTypePrimitives}
}

// String returns a string representation of the node.
func (n *Node) String() string {
	if n.raw == nil || isNull(*n.raw) {
		return "<nil>"
	}
	var v interface{}
	if err := cborUnmarshal(*n.raw, &v); err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	return fmt.Sprintf("%v", v)
}

// Patch applies the given patch to the node.
// It only supports string keys in a map node.
func (n *Node) Patch(p Patch, options *Options) error {
	pd, err := n.intoContainer()
	switch {
	case err != nil:
		return fmt.Errorf("unexpected node %q, %v", n.String(), err)
	case pd == nil:
		return fmt.Errorf("unexpected node %q", n.String())
	}

	if options == nil {
		options = NewOptions()
	}
	var accumulatedCopySize int64
	for _, op := range p {
		switch op.Op {
		case "add":
			err = p.add(&pd, op, options)
		case "remove":
			err = p.remove(&pd, op, options)
		case "replace":
			err = p.replace(&pd, op, options)
		case "move":
			err = p.move(&pd, op, options)
		case "test":
			err = p.test(&pd, op, options)
		case "copy":
			err = p.copy(&pd, op, &accumulatedCopySize, options)
		default:
			err = fmt.Errorf("unexpected operation %q", op.Op)
		}

		if err != nil {
			return err
		}
	}
	switch n.which {
	case eDoc:
		n.doc = pd.(*partialDoc)
	case eAry:
		n.ary = *(pd.(*partialArray))
	}
	return nil
}

// MarshalCBOR implements the cbor.Marshaler interface.
func (n *Node) MarshalCBOR() ([]byte, error) {
	switch n.which {
	case eRaw, eOther:
		return cborMarshal(n.raw)
	case eDoc:
		return cborMarshal(n.doc)
	case eAry:
		return cborMarshal(n.ary)
	default:
		return nil, ErrUnknownType
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (n *Node) MarshalJSON() ([]byte, error) {
	n.intoContainer()

	switch n.which {
	case eOther:
		if n.raw == nil {
			return json.Marshal(nil)
		}
		var val interface{}
		if err := cborUnmarshal(*n.raw, &val); err != nil {
			return nil, err
		}
		return json.Marshal(val)
	case eDoc:
		return json.Marshal(n.doc)
	case eAry:
		return json.Marshal(n.ary)
	default:
		return nil, ErrUnknownType
	}
}

// UnmarshalCBOR implements the cbor.Unmarshaler interface.
func (n *Node) UnmarshalCBOR(data []byte) error {
	if n == nil {
		return errors.New("unexpected node, nil pointer")
	}
	if n.raw == nil {
		raw := make(RawMessage, len(data))
		n.raw = &raw
	}
	*n.raw = append((*n.raw)[0:0], data...)
	n.which = eRaw
	n.ty = CBORTypePrimitives
	return nil
}

type container interface {
	get(key interface{}, options *Options) (*Node, error)
	set(key interface{}, val *Node, options *Options) error
	add(key interface{}, val *Node, options *Options) error
	remove(key interface{}, options *Options) error
}

type partialDoc struct {
	obj map[interface{}]*Node
}

type partialArray []*Node

func (d *partialDoc) MarshalCBOR() ([]byte, error) {
	return cborMarshal(d.obj)
}

func (d *partialDoc) MarshalJSON() ([]byte, error) {
	obj := make(map[string]*Node, len(d.obj))
	for k := range d.obj {
		obj[EncodePatchKey(k)] = d.obj[k]
	}
	return json.Marshal(obj)
}

func (d *partialDoc) UnmarshalCBOR(data []byte) error {
	return cborUnmarshal(data, &d.obj)
}

func (d *partialDoc) set(key interface{}, val *Node, options *Options) error {
	d.obj[key] = val
	return nil
}

func (d *partialDoc) add(key interface{}, val *Node, options *Options) error {
	return d.set(key, val, options)
}

func (d *partialDoc) get(key interface{}, options *Options) (*Node, error) {
	v, ok := d.obj[key]
	if !ok {
		return nil, fmt.Errorf("unable to get nonexistent key %q, %v", EncodePatchKey(key), ErrMissing)
	}
	if v == nil {
		v = NewNode(rawCBORNull)
	}
	return v, nil
}

func (d *partialDoc) remove(key interface{}, options *Options) error {
	_, ok := d.obj[key]
	if !ok {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("unable to remove nonexistent key %q, %v", EncodePatchKey(key), ErrMissing)
	}
	delete(d.obj, key)
	return nil
}

// set should only be used to implement the "replace" operation, so "key" must
// be an already existing index in "d".
func (d *partialArray) set(key interface{}, val *Node, options *Options) error {
	idx, err := decodeArrayIdx(key)
	if err != nil {
		return err
	}

	sz := len(*d)
	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	(*d)[idx] = val
	return nil
}

func (d *partialArray) add(key interface{}, val *Node, options *Options) error {
	if k, ok := key.(string); ok && k == "-" {
		*d = append(*d, val)
		return nil
	}

	idx, err := decodeArrayIdx(key)
	if err != nil {
		return err
	}

	sz := len(*d) + 1
	if idx >= sz {
		return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
	}

	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	cur := *d
	ary := make([]*Node, sz)
	copy(ary[0:idx], cur[0:idx])
	ary[idx] = val
	copy(ary[idx+1:], cur[idx:])

	*d = ary
	return nil
}

func (d *partialArray) get(key interface{}, options *Options) (*Node, error) {
	idx, err := decodeArrayIdx(key)
	if err != nil {
		return nil, err
	}

	sz := len(*d)
	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return nil, fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	if idx >= sz {
		return nil, fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
	}
	v := (*d)[idx]
	if v == nil {
		v = NewNode(rawCBORNull)
	}
	return v, nil
}

func (d *partialArray) remove(key interface{}, options *Options) error {
	idx, err := decodeArrayIdx(key)
	if err != nil {
		return err
	}

	sz := len(*d)
	if idx >= sz {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
	}

	if idx < 0 {
		if !options.SupportNegativeIndices {
			return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
		}
		if idx < -sz {
			if options.AllowMissingPathOnRemove {
				return nil
			}
			return fmt.Errorf("unable to access invalid index %s, %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	cur := *d
	ary := make([]*Node, sz-1)
	copy(ary[0:idx], cur[0:idx])
	copy(ary[idx:], cur[idx+1:])

	*d = ary
	return nil
}

func (n *Node) intoContainer() (container, error) {
	switch n.which {
	case eDoc:
		return n.doc, nil
	case eAry:
		return &n.ary, nil
	case eOther:
		return nil, ErrInvalid
	}

	n.which = eOther
	if n.raw == nil {
		return nil, ErrInvalid
	}

	n.ty = ReadCBORType(*n.raw)
	switch n.ty {
	case CBORTypeMap:
		if err := cborUnmarshal(*n.raw, &n.doc); err != nil {
			return nil, err
		}
		n.which = eDoc
		return n.doc, nil
	case CBORTypeArray:
		if err := cborUnmarshal(*n.raw, &n.ary); err != nil {
			return nil, err
		}
		n.which = eAry
		return &n.ary, nil
	}
	return nil, ErrInvalid
}

func (n *Node) isNull() bool {
	if n.raw == nil {
		return true
	}
	return isNull(*n.raw)
}

// Equal indicates if 2 CBOR Nodes have the same structural equality.
func (n *Node) Equal(o *Node) bool {
	n.intoContainer()
	if n.which == eOther {
		if o.which == eDoc || o.which == eAry {
			return false
		}

		return bytes.Equal(*n.raw, *o.raw)
	}

	o.intoContainer()
	if n.which != o.which {
		return false
	}

	if n.which == eDoc {
		if len(n.doc.obj) != len(o.doc.obj) {
			return false
		}

		for k, v := range n.doc.obj {
			ov, ok := o.doc.obj[k]

			if !ok {
				return false
			}

			if (v == nil) != (ov == nil) {
				return false
			}

			if v == nil && ov == nil {
				continue
			}

			if !v.Equal(ov) {
				return false
			}
		}

		return true
	}

	if len(n.ary) != len(o.ary) {
		return false
	}

	for idx, val := range n.ary {
		if !val.Equal(o.ary[idx]) {
			return false
		}
	}

	return true
}

func (p Patch) add(doc *container, op Operation, options *Options) error {
	if options.EnsurePathExistsOnAdd {
		if err := ensurePathExists(doc, op.Path, options); err != nil {
			return err
		}
	}

	con, key := findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("add operation does not apply for %q, %v", op.Path, ErrMissing)
	}

	if err := con.add(key, NewNode(op.Value), options); err != nil {
		return fmt.Errorf("add operation does not apply for %q, %v", op.Path, err)
	}

	return nil
}

func (p Patch) remove(doc *container, op Operation, options *Options) error {
	con, key := findObject(doc, op.Path, options)
	if con == nil {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("remove operation does not apply for %q, %v", op.Path, ErrMissing)
	}

	if err := con.remove(key, options); err != nil {
		return fmt.Errorf("remove operation does not apply for %q, %v", op.Path, err)
	}
	return nil
}

func (p Patch) replace(doc *container, op Operation, options *Options) error {
	if op.Path == "" {
		val := NewNode(op.Value)
		val.intoContainer()

		switch val.which {
		case eAry:
			*doc = &val.ary
		case eDoc:
			*doc = val.doc
		case eOther:
			return errors.New("replace operation hit impossible case")
		}

		return nil
	}

	con, key := findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("replace operation does not apply for %q, %v", op.Path, ErrMissing)
	}

	_, ok := con.get(key, options)
	if ok != nil {
		return fmt.Errorf("replace operation does not apply for %q, %v", op.Path, ErrMissing)
	}

	if err := con.set(key, NewNode(op.Value), options); err != nil {
		return fmt.Errorf("replace operation does not apply for %q, %v", op.Path, err)
	}
	return nil
}

func (p Patch) move(doc *container, op Operation, options *Options) error {
	con, key := findObject(doc, op.From, options)
	if con == nil {
		return fmt.Errorf("move operation does not apply for from %q, %v", op.From, ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil {
		return fmt.Errorf("move operation does not apply for from %q, %v", op.From, err)
	}

	if err = con.remove(key, options); err != nil {
		return fmt.Errorf("move operation does not apply for from %q, %v", op.From, err)
	}

	con, key = findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("move operation does not apply for path %q, %v", op.Path, ErrMissing)
	}

	if err = con.add(key, val, options); err != nil {
		return fmt.Errorf("move operation does not apply for path %q, %v", op.Path, err)
	}
	return nil
}

func (p Patch) test(doc *container, op Operation, options *Options) error {
	if op.Path == "" {
		var self Node

		switch sv := (*doc).(type) {
		case *partialDoc:
			self.doc = sv
			self.which = eDoc
		case *partialArray:
			self.ary = *sv
			self.which = eAry
		}

		if self.Equal(NewNode(op.Value)) {
			return nil
		}

		return fmt.Errorf("test operation for path %q failed, not equal", op.Path)
	}

	con, key := findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("test operation for path %q failed, %v", op.Path, ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil && !strings.Contains(err.Error(), ErrMissing.Error()) {
		return fmt.Errorf("test operation for path %q failed, %v", op.Path, err)
	}

	if val == nil || val.isNull() {
		if isNull(op.Value) {
			return nil
		}
		return fmt.Errorf("test operation for path %q failed, expected %q, got nil",
			op.Path, NewNode(op.Value).String())

	} else if op.Value == nil {
		return fmt.Errorf("test operation for path %q failed, expected nil, got %q",
			op.Path, val.String())
	}

	if val.Equal(NewNode(op.Value)) {
		return nil
	}

	return fmt.Errorf("test operation for path %q failed, expected %q, got %q",
		op.Path, NewNode(op.Value).String(), val.String())
}

func (p Patch) copy(doc *container, op Operation, accumulatedCopySize *int64, options *Options) error {
	con, key := findObject(doc, op.From, options)

	if con == nil {
		return fmt.Errorf("copy operation does not apply for from path %q, %v", op.From, ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil {
		return fmt.Errorf("copy operation does not apply for from path %q, %v", op.From, err)
	}

	con, key = findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("copy operation does not apply for path %q, %v", op.Path, ErrMissing)
	}

	valCopy, sz, err := deepCopy(val)
	if err != nil {
		return fmt.Errorf("copy operation does not apply for path %q while performing deep copy, %v", op.Path, err)
	}

	(*accumulatedCopySize) += int64(sz)
	if options.AccumulatedCopySizeLimit > 0 && *accumulatedCopySize > options.AccumulatedCopySizeLimit {
		return NewAccumulatedCopySizeError(options.AccumulatedCopySizeLimit, *accumulatedCopySize)
	}

	err = con.add(key, valCopy, options)
	if err != nil {
		return fmt.Errorf("copy operation does not apply for path %s while adding value during copy, %v",
			op.Path, err)
	}

	return nil
}

func findObject(pd *container, path string, options *Options) (container, interface{}) {
	doc := *pd

	split := strings.Split(path, "/")
	if len(split) < 2 {
		return nil, ""
	}

	parts := split[1 : len(split)-1]
	key := split[len(split)-1]

	for _, part := range parts {
		next, ok := doc.get(DecodePatchKey(part), options)
		if next == nil || ok != nil {
			return nil, ""
		}
		doc, _ = next.intoContainer()
		if doc == nil {
			return nil, ""
		}
	}
	return doc, DecodePatchKey(key)
}

// Given a document and a path to a key, walk the path and create all missing elements
// creating objects and arrays as needed.
func ensurePathExists(pd *container, path string, options *Options) error {
	var err error
	var arrIndex int

	doc := *pd
	split := strings.Split(path, "/")
	if len(split) < 2 {
		return nil
	}

	parts := split[1:]
	for pi, part := range parts {
		// Have we reached the key part of the path?
		// If yes, we're done.
		if pi == len(parts)-1 {
			return nil
		}

		target, ok := doc.get(DecodePatchKey(part), options)
		if target == nil || ok != nil {
			// If the current container is an array which has fewer elements than our target index,
			// pad the current container with nulls.
			if arrIndex, err = strconv.Atoi(part); err == nil {
				pa, ok := doc.(*partialArray)

				if ok && arrIndex >= len(*pa)+1 {
					// Pad the array with null values up to the required index.
					for i := len(*pa); i <= arrIndex-1; i++ {
						doc.add(strconv.Itoa(i), NewNode(rawCBORNull), options)
					}
				}
			}

			// Check if the next part is a numeric index or "-".
			// If yes, then create an array, otherwise, create an object.
			if arrIndex, err = strconv.Atoi(parts[pi+1]); err == nil || parts[pi+1] == "-" {
				if arrIndex < 0 {
					if !options.SupportNegativeIndices {
						return fmt.Errorf("unable to ensure path for invalid index %d, %v",
							arrIndex, ErrInvalidIndex)
					}

					if arrIndex < -1 {
						return fmt.Errorf("unable to ensure path for invalid index %d, %v",
							arrIndex, ErrInvalidIndex)
					}

					arrIndex = 0
				}

				node := NewNode(rawCBORArray)
				doc.add(part, node, options)
				doc, _ = node.intoContainer()

				// Pad the new array with null values up to the required index.
				for i := 0; i < arrIndex; i++ {
					doc.add(strconv.Itoa(i), NewNode(rawCBORNull), options)
				}
			} else {
				node := NewNode(rawCBORMap)

				doc.add(part, node, options)
				doc, _ = node.intoContainer()
			}
		} else {
			doc, err = target.intoContainer()
			if doc == nil {
				return fmt.Errorf("unable to ensure path for invalid target %q, %v", target.String(), err)
			}
		}
	}

	return nil
}

func deepCopy(src *Node) (*Node, int, error) {
	if src == nil {
		return nil, 0, nil
	}
	a, err := src.MarshalCBOR()
	if err != nil {
		return nil, 0, err
	}
	sz := len(a)
	return NewNode(a), sz, nil
}

func isNull(data RawMessage) bool {
	if l := len(data); l == 0 || l == 1 && (data[0] == 0xf6 || data[0] == 0xf7) {
		return true
	}
	return false
}

func decodeArrayIdx(key interface{}) (int, error) {
	switch v := key.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return -1, fmt.Errorf("%v was not a proper array index, type %T", v, v)
	}
}

// From http://tools.ietf.org/html/rfc6901#section-4 :
//
// Evaluation of each reference token begins by decoding any escaped
// character sequence. This is performed by first transforming any
// occurrence of the sequence '~1' to '/', and then transforming any
// occurrence of the sequence '~0' to '~'.
var (
	rfc6901Decoder = strings.NewReplacer("~1", "/", "~0", "~")
	rfc6901Encoder = strings.NewReplacer("/", "~1", "~", "~0")
)

// DecodePatchKey decodes a interface{} key from a RFC6901 compliant string.
// The key with a prefix of '~x' will be decoded to a interface with hex decode and CBOR Unmarshal.
// Otherwise it will transform any '~1' to '/',
// and then transform any '~0' to '~'.
// See http://tools.ietf.org/html/rfc6901#section-4 for details.
func DecodePatchKey(key string) interface{} {
	if strings.HasPrefix(key, "~x") {
		if data, err := hex.DecodeString(key[2:]); err == nil {
			if ReadCBORType(data).ValidKey() {
				var v interface{}
				if err = cborUnmarshal(data, &v); err == nil {
					return v
				}
			}
		}
	}

	return rfc6901Decoder.Replace(key)
}

// EncodePatchKey encodes a interface{} key to a RFC6901 compliant string.
// If the key is a valid utf8 string, it will transform any '/' to '~1',
// and then transform any '~' to '~0'.
// Otherwise it will encode the key with CBOR Marshal and hex encode and prefix it with '~x'.
func EncodePatchKey(key interface{}) string {
	if str, ok := key.(string); ok {
		return rfc6901Encoder.Replace(str)
	}

	data, err := cborMarshal(key)
	if t := ReadCBORType(data); !t.ValidKey() && err == nil {
		err = fmt.Errorf("%q can't be key", t.String())
	}

	if err == nil {
		return "~x" + hex.EncodeToString(data)
	}

	// we should not reach here
	return fmt.Sprintf("EncodePatchKey(%v) error: %v", key, err)
}

// AccumulatedCopySizeError is an error type returned when the accumulated size
// increase caused by copy operations in a patch operation has exceeded the
// limit.
type AccumulatedCopySizeError struct {
	limit       int64
	accumulated int64
}

// NewAccumulatedCopySizeError returns an AccumulatedCopySizeError.
func NewAccumulatedCopySizeError(l, a int64) *AccumulatedCopySizeError {
	return &AccumulatedCopySizeError{limit: l, accumulated: a}
}

// Error implements the error interface.
func (a *AccumulatedCopySizeError) Error() string {
	return fmt.Sprintf(
		"unable to copy, the accumulated size increase of copy is %d, exceeding the limit %d",
		a.accumulated, a.limit)
}

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
	ErrTestFailed   = errors.New("test failed")
	ErrMissing      = errors.New("missing value")
	ErrUnknownType  = errors.New("unknown object type")
	ErrInvalidIndex = errors.New("invalid index referenced")
)

const (
	eRaw = iota
	eDoc
	eAry
	eOther
)

// Operation is a single CBOR-Patch step, such as a single 'add' operation.
type Operation struct {
	Op    string      `cbor:"op"`
	Path  string      `cbor:"path"`
	From  string      `cbor:"from,omitempty"`
	Value *RawMessage `cbor:"value,omitempty"`
}

// Patch is an ordered collection of Operations.
type Patch []Operation

// ApplyOptions specifies options for calls to ApplyWithOptions.
// Use NewApplyOptions to obtain default values for ApplyOptions.
type ApplyOptions struct {
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

// NewApplyOptions creates a default set of options for calls to ApplyWithOptions.
func NewApplyOptions() *ApplyOptions {
	return &ApplyOptions{
		SupportNegativeIndices:   SupportNegativeIndices,
		AccumulatedCopySizeLimit: AccumulatedCopySizeLimit,
		AllowMissingPathOnRemove: false,
		EnsurePathExistsOnAdd:    false,
	}
}

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

	return json.Marshal(newLazyNode(newRawMessage(doc)))
}

// Equal indicates if 2 CBOR documents have the same structural equality.
func Equal(a, b []byte) bool {
	la := newLazyNode(newRawMessage(a))
	lb := newLazyNode(newRawMessage(b))

	return la.equal(lb)
}

// DecodePatch decodes the passed CBOR document as an RFC 6902 patch.
func DecodePatch(buf []byte) (Patch, error) {
	var p Patch

	err := cborUnmarshal(buf, &p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Apply mutates a CBOR document according to the patch, and returns the new
// document.
func (p Patch) Apply(doc []byte) ([]byte, error) {
	return p.ApplyWithOptions(doc, NewApplyOptions())
}

// ApplyWithOptions mutates a CBOR document according to the patch and the passed in ApplyOptions.
// It returns the new document.
func (p Patch) ApplyWithOptions(doc []byte, options *ApplyOptions) ([]byte, error) {
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
		return nil, fmt.Errorf("unexpected document: %s", strconv.Quote(t.String()))
	}

	err := cborUnmarshal(doc, pd)
	if err != nil {
		return nil, err
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
			err = fmt.Errorf("unexpected operation: %s", strconv.Quote(op.Op))
		}

		if err != nil {
			return nil, err
		}
	}

	return cborMarshal(pd)
}

type container interface {
	get(key string, options *ApplyOptions) (*lazyNode, error)
	set(key string, val *lazyNode, options *ApplyOptions) error
	add(key string, val *lazyNode, options *ApplyOptions) error
	remove(key string, options *ApplyOptions) error
}

type partialDoc struct {
	obj map[string]*lazyNode
}

type partialArray []*lazyNode

type lazyNode struct {
	raw   *RawMessage
	doc   *partialDoc
	ary   partialArray
	ty    CBORType
	which int
}

func newLazyNode(raw *RawMessage) *lazyNode {
	if raw == nil || len(*raw) == 0 {
		raw = newRawMessage(rawCBORNull)
	}
	return &lazyNode{raw: raw, doc: nil, ary: nil, ty: CBORTypePrimitives, which: eRaw}
}

func newRawMessage(buf []byte) *RawMessage {
	ra := make(RawMessage, len(buf))
	copy(ra, buf)
	return &ra
}

func (n *lazyNode) MarshalCBOR() ([]byte, error) {
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

func (n *lazyNode) MarshalJSON() ([]byte, error) {
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

func (n *lazyNode) UnmarshalCBOR(data []byte) error {
	n.raw = newRawMessage(data)
	n.which = eRaw
	n.ty = CBORTypePrimitives
	return nil
}

func (d *partialDoc) MarshalCBOR() ([]byte, error) {
	return cborMarshal(d.obj)
}

func (d *partialDoc) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.obj)
}

func (d *partialDoc) UnmarshalCBOR(data []byte) error {
	return cborUnmarshal(data, &d.obj)
}

func (d *partialDoc) set(key string, val *lazyNode, options *ApplyOptions) error {
	d.obj[key] = val
	return nil
}

func (d *partialDoc) add(key string, val *lazyNode, options *ApplyOptions) error {
	return d.set(key, val, options)
}

func (d *partialDoc) get(key string, options *ApplyOptions) (*lazyNode, error) {
	v, ok := d.obj[key]
	if !ok {
		return nil, fmt.Errorf("unable to get nonexistent key %s: %v", strconv.Quote(key), ErrMissing)
	}
	if v == nil {
		v = newLazyNode(newRawMessage(rawCBORNull))
	}
	return v, nil
}

func (d *partialDoc) remove(key string, options *ApplyOptions) error {
	_, ok := d.obj[key]
	if !ok {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("unable to remove nonexistent key %s: %v", strconv.Quote(key), ErrMissing)
	}
	delete(d.obj, key)
	return nil
}

// set should only be used to implement the "replace" operation, so "key" must
// be an already existing index in "d".
func (d *partialArray) set(key string, val *lazyNode, options *ApplyOptions) error {
	idx, err := strconv.Atoi(key)
	if err != nil {
		return err
	}

	sz := len(*d)
	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	(*d)[idx] = val
	return nil
}

func (d *partialArray) add(key string, val *lazyNode, options *ApplyOptions) error {
	if key == "-" {
		*d = append(*d, val)
		return nil
	}

	idx, err := strconv.Atoi(key)
	if err != nil {
		return fmt.Errorf("value was not a proper array index %s: %v", key, err)
	}

	sz := len(*d) + 1
	if idx >= sz {
		return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
	}

	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	cur := *d
	ary := make([]*lazyNode, sz)
	copy(ary[0:idx], cur[0:idx])
	ary[idx] = val
	copy(ary[idx+1:], cur[idx:])

	*d = ary
	return nil
}

func (d *partialArray) get(key string, options *ApplyOptions) (*lazyNode, error) {
	idx, err := strconv.Atoi(key)
	if err != nil {
		return nil, err
	}

	sz := len(*d)
	if idx < 0 {
		if !options.SupportNegativeIndices || idx < -sz {
			return nil, fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	if idx >= sz {
		return nil, fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
	}
	v := (*d)[idx]
	if v == nil {
		v = newLazyNode(newRawMessage(rawCBORNull))
	}
	return v, nil
}

func (d *partialArray) remove(key string, options *ApplyOptions) error {
	idx, err := strconv.Atoi(key)
	if err != nil {
		return err
	}

	sz := len(*d)
	if idx >= sz {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
	}

	if idx < 0 {
		if !options.SupportNegativeIndices {
			return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
		}
		if idx < -sz {
			if options.AllowMissingPathOnRemove {
				return nil
			}
			return fmt.Errorf("unable to access invalid index %s: %v", key, ErrInvalidIndex)
		}
		idx += sz
	}

	cur := *d
	ary := make([]*lazyNode, sz-1)
	copy(ary[0:idx], cur[0:idx])
	copy(ary[idx:], cur[idx+1:])

	*d = ary
	return nil
}

func (n *lazyNode) intoContainer() container {
	switch n.which {
	case eDoc:
		return n.doc
	case eAry:
		return &n.ary
	case eOther:
		return nil
	}

	n.which = eOther
	if n.raw == nil {
		return nil
	}

	n.ty = ReadCBORType(*n.raw)
	switch n.ty {
	case CBORTypeMap:
		if err := cborUnmarshal(*n.raw, &n.doc); err != nil {
			return nil
		}
		n.which = eDoc
		return n.doc
	case CBORTypeArray:
		if err := cborUnmarshal(*n.raw, &n.ary); err != nil {
			return nil
		}
		n.which = eAry
		return &n.ary
	}
	return nil
}

func (n *lazyNode) isNull() bool {
	if n.raw == nil || len(*n.raw) == 0 {
		return true
	}
	if r := *n.raw; len(r) == 1 && (r[0] == 0xf6 || r[0] == 0xf7) {
		return true
	}
	return false
}

func (n *lazyNode) equal(o *lazyNode) bool {
	if n.which == eRaw {
		n.intoContainer()
	}

	if n.which == eOther {
		if o.which == eDoc || o.which == eAry {
			return false
		}

		return bytes.Equal(*n.raw, *o.raw)
	}

	if o.which == eRaw {
		o.intoContainer()
	}

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

			if !v.equal(ov) {
				return false
			}
		}

		return true
	}

	if len(n.ary) != len(o.ary) {
		return false
	}

	for idx, val := range n.ary {
		if !val.equal(o.ary[idx]) {
			return false
		}
	}

	return true
}

func (p Patch) add(doc *container, op Operation, options *ApplyOptions) error {
	if options.EnsurePathExistsOnAdd {
		if err := ensurePathExists(doc, op.Path, options); err != nil {
			return err
		}
	}

	con, key := findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("add operation does not apply for %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	if err := con.add(key, newLazyNode(op.Value), options); err != nil {
		return fmt.Errorf("add operation does not apply for %s: %v", strconv.Quote(op.Path), err)
	}

	return nil
}

func (p Patch) remove(doc *container, op Operation, options *ApplyOptions) error {
	con, key := findObject(doc, op.Path, options)
	if con == nil {
		if options.AllowMissingPathOnRemove {
			return nil
		}
		return fmt.Errorf("remove operation does not apply for %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	if err := con.remove(key, options); err != nil {
		return fmt.Errorf("remove operation does not apply for %s: %v", strconv.Quote(op.Path), err)
	}
	return nil
}

func (p Patch) replace(doc *container, op Operation, options *ApplyOptions) error {
	if op.Path == "" {
		val := newLazyNode(op.Value)
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
		return fmt.Errorf("replace operation does not apply for %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	_, ok := con.get(key, options)
	if ok != nil {
		return fmt.Errorf("remove operation does not apply for %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	if err := con.set(key, newLazyNode(op.Value), options); err != nil {
		return fmt.Errorf("remove operation does not apply for %s: %v", strconv.Quote(op.Path), err)
	}
	return nil
}

func (p Patch) move(doc *container, op Operation, options *ApplyOptions) error {
	con, key := findObject(doc, op.From, options)
	if con == nil {
		return fmt.Errorf("move operation does not apply for from path %s: %v", strconv.Quote(op.From), ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil {
		return fmt.Errorf("move operation does not apply for from path %s: %v", strconv.Quote(op.From), err)
	}

	if err = con.remove(key, options); err != nil {
		return fmt.Errorf("move operation does not apply for from path %s: %v", strconv.Quote(op.From), err)
	}

	con, key = findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("move operation does not apply for path %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	if err = con.add(key, val, options); err != nil {
		return fmt.Errorf("move operation does not apply for path %s: %v", strconv.Quote(op.Path), err)
	}
	return nil
}

func (p Patch) test(doc *container, op Operation, options *ApplyOptions) error {
	if op.Path == "" {
		var self lazyNode

		switch sv := (*doc).(type) {
		case *partialDoc:
			self.doc = sv
			self.which = eDoc
		case *partialArray:
			self.ary = *sv
			self.which = eAry
		}

		if self.equal(newLazyNode(op.Value)) {
			return nil
		}

		return fmt.Errorf("testing value %s failed: %v", strconv.Quote(op.Path), ErrTestFailed)
	}

	con, key := findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("testing value %s failed: %s", strconv.Quote(op.Path), ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil && !strings.Contains(err.Error(), ErrMissing.Error()) {
		return fmt.Errorf("testing value %s failed: %s", strconv.Quote(op.Path), err)
	}

	if val == nil || val.isNull() {
		if op.Value == nil {
			return nil
		}
		return fmt.Errorf("testing value %s failed: %v", strconv.Quote(op.Path), ErrTestFailed)
	} else if op.Value == nil {
		return fmt.Errorf("testing value %s failed: %v", strconv.Quote(op.Path), ErrTestFailed)
	}

	if val.equal(newLazyNode(op.Value)) {
		return nil
	}

	return fmt.Errorf("testing value %s failed: %v", strconv.Quote(op.Path), ErrTestFailed)
}

func (p Patch) copy(doc *container, op Operation, accumulatedCopySize *int64, options *ApplyOptions) error {
	con, key := findObject(doc, op.From, options)

	if con == nil {
		return fmt.Errorf("copy operation does not apply for from path %s: %v", strconv.Quote(op.From), ErrMissing)
	}

	val, err := con.get(key, options)
	if err != nil {
		return fmt.Errorf("copy operation does not apply for from path %s: %v", strconv.Quote(op.From), err)
	}

	con, key = findObject(doc, op.Path, options)
	if con == nil {
		return fmt.Errorf("copy operation does not apply for path %s: %v", strconv.Quote(op.Path), ErrMissing)
	}

	valCopy, sz, err := deepCopy(val)
	if err != nil {
		return fmt.Errorf("copy operation does not apply, error while performing deep copy: %v", err)
	}

	(*accumulatedCopySize) += int64(sz)
	if options.AccumulatedCopySizeLimit > 0 && *accumulatedCopySize > options.AccumulatedCopySizeLimit {
		return NewAccumulatedCopySizeError(options.AccumulatedCopySizeLimit, *accumulatedCopySize)
	}

	err = con.add(key, valCopy, options)
	if err != nil {
		return fmt.Errorf("copy operation does not apply, error while adding value during copy: %v", err)
	}

	return nil
}

func findObject(pd *container, path string, options *ApplyOptions) (container, string) {
	doc := *pd

	split := strings.Split(path, "/")
	if len(split) < 2 {
		return nil, ""
	}

	parts := split[1 : len(split)-1]
	key := split[len(split)-1]

	for _, part := range parts {
		next, ok := doc.get(decodePatchKey(part), options)
		if next == nil || ok != nil {
			return nil, ""
		}
		doc = next.intoContainer()
		if doc == nil {
			return nil, ""
		}
	}
	return doc, decodePatchKey(key)
}

// Given a document and a path to a key, walk the path and create all missing elements
// creating objects and arrays as needed.
func ensurePathExists(pd *container, path string, options *ApplyOptions) error {
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

		target, ok := doc.get(decodePatchKey(part), options)
		if target == nil || ok != nil {
			// If the current container is an array which has fewer elements than our target index,
			// pad the current container with nulls.
			if arrIndex, err = strconv.Atoi(part); err == nil {
				pa, ok := doc.(*partialArray)

				if ok && arrIndex >= len(*pa)+1 {
					// Pad the array with null values up to the required index.
					for i := len(*pa); i <= arrIndex-1; i++ {
						doc.add(strconv.Itoa(i), newLazyNode(newRawMessage(rawCBORNull)), options)
					}
				}
			}

			// Check if the next part is a numeric index or "-".
			// If yes, then create an array, otherwise, create an object.
			if arrIndex, err = strconv.Atoi(parts[pi+1]); err == nil || parts[pi+1] == "-" {
				if arrIndex < 0 {
					if !options.SupportNegativeIndices {
						return fmt.Errorf("unable to ensure path for invalid index %d: %v", arrIndex, ErrInvalidIndex)
					}

					if arrIndex < -1 {
						return fmt.Errorf("unable to ensure path for invalid index %d: %v", arrIndex, ErrInvalidIndex)
					}

					arrIndex = 0
				}

				newNode := newLazyNode(newRawMessage(rawCBORArray))
				doc.add(part, newNode, options)
				doc = newNode.intoContainer()

				// Pad the new array with null values up to the required index.
				for i := 0; i < arrIndex; i++ {
					doc.add(strconv.Itoa(i), newLazyNode(newRawMessage(rawCBORNull)), options)
				}
			} else {
				newNode := newLazyNode(newRawMessage(rawCBORMap))

				doc.add(part, newNode, options)
				doc = newNode.intoContainer()
			}
		} else {
			doc = target.intoContainer()
			if doc == nil {
				return fmt.Errorf("unable to ensure path for invalid target type value %s: %v",
					target.ty.String(), ErrUnknownType)
			}
		}
	}

	return nil
}

func deepCopy(src *lazyNode) (*lazyNode, int, error) {
	if src == nil {
		return nil, 0, nil
	}
	a, err := src.MarshalCBOR()
	if err != nil {
		return nil, 0, err
	}
	sz := len(a)
	return newLazyNode(newRawMessage(a)), sz, nil
}

// From http://tools.ietf.org/html/rfc6901#section-4 :
//
// Evaluation of each reference token begins by decoding any escaped
// character sequence.  This is performed by first transforming any
// occurrence of the sequence '~1' to '/', and then transforming any
// occurrence of the sequence '~0' to '~'.
var (
	rfc6901Decoder = strings.NewReplacer("~1", "/", "~0", "~")
	rfc6901Encoder = strings.NewReplacer("/", "~1", "~", "~0")
)

func decodePatchKey(k string) string {
	return rfc6901Decoder.Replace(k)
}

func encodePatchKey(k string) string {
	return rfc6901Encoder.Replace(k)
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
		"unable to complete the copy, the accumulated size increase of copy is %d, exceeding the limit %d",
		a.accumulated, a.limit)
}

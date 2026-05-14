/*
 * Copyright (c) 2021-2026 Marcin Gasperowicz <xnooga@gmail.com>
 * SPDX-License-Identifier: MIT
 */

package vm

import (
	"sort"
	"strings"
	"unsafe"
)

// theReifiedType is the singleton ValueType for all *Reified instances.
// Class identity across reify forms is intentionally not part of the contract.
type theReifiedType struct{}

func (t *theReifiedType) String() string     { return t.Name() }
func (t *theReifiedType) Type() ValueType    { return TypeType }
func (t *theReifiedType) Unbox() interface{} { return nil }
func (t *theReifiedType) Name() string       { return "let-go.lang.Reified" }
func (t *theReifiedType) Box(bare interface{}) (Value, error) {
	return NIL, NewTypeError(bare, "can't be boxed as", t)
}

var ReifiedType *theReifiedType = &theReifiedType{}

// Reified is the value produced by (reify ...). It carries instance-owned
// protocol implementations rather than registering them on Protocol.impls.
//
// The impls map is built once at construction and never mutated, so dispatch
// is safe for concurrent reads. Closure captures live in the Fn values and
// are reclaimed when the Reified is GC'd.
type Reified struct {
	impls map[*Protocol]map[Symbol]Fn
	meta  Value
}

// NewReified creates a Reified. The caller must not mutate impls afterward.
func NewReified(impls map[*Protocol]map[Symbol]Fn) *Reified {
	return &Reified{impls: impls}
}

func (r *Reified) Type() ValueType    { return ReifiedType }
func (r *Reified) Unbox() interface{} { return r }

// Equals: identity. Matches every other anonymous-instance value type
// across JVM, CLJS, and let-go's existing DTypeInstance.
func (r *Reified) Equals(other Value) bool {
	o, ok := other.(*Reified)
	return ok && r == o
}

// Hash: identity-based, mixed through Murmur3 finalizer.
func (r *Reified) Hash() uint32 {
	return mixFinish(uint32(uintptr(unsafe.Pointer(r))))
}

// String: opaque, deterministic. Protocol names sorted for stable output.
func (r *Reified) String() string {
	names := make([]string, 0, len(r.impls))
	for p := range r.impls {
		names = append(names, p.Name())
	}
	sort.Strings(names)
	b := &strings.Builder{}
	b.WriteString("#<reified")
	for _, n := range names {
		b.WriteRune(' ')
		b.WriteString(n)
	}
	b.WriteString(">")
	return b.String()
}

// --- IMeta ---

func (r *Reified) Meta() Value {
	if r.meta == nil {
		return NIL
	}
	return r.meta
}

// WithMeta returns a new *Reified with the same impls and new metadata.
// The new value has distinct identity from the original.
func (r *Reified) WithMeta(m Value) Value {
	return &Reified{impls: r.impls, meta: m}
}

// --- Dispatch helpers (called from Protocol.Lookup / Satisfies) ---

// Claims reports whether this Reified claims to satisfy p.
func (r *Reified) Claims(p *Protocol) bool {
	_, ok := r.impls[p]
	return ok
}

// Impl returns the impl of method m on this Reified for protocol p.
// Returns (nil, false) if either p is not claimed or m is not implemented.
func (r *Reified) Impl(p *Protocol, m Symbol) (Fn, bool) {
	methods, ok := r.impls[p]
	if !ok {
		return nil, false
	}
	fn, ok := methods[m]
	return fn, ok
}

/*
 * Copyright (c) 2021 Marcin Gasperowicz <xnooga@gmail.com>
 * SPDX-License-Identifier: MIT
 */

package vm

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Var is a thread-safe mutable reference to a root value, plus a dynamic
// binding stack. Root access (Deref's root path, SetRoot, Invoke, Arity,
// AlterRoot) is synchronized via a generation counter and mutex, mirroring
// Atom.Swap. Read-heavy paths use RLock; mutations and AlterRoot's CAS use
// Lock.
//
// The dynamic binding stack (bindings) is intentionally NOT synchronized
// here — under concurrent goroutines, racing append/index can produce
// fatal Go runtime panics (corrupted slice header, index out of range),
// not just lost updates. The binding model needs a per-execution-context
// redesign (see feat/dynamic-var-binding-conveyance) before it can be
// safely shared across goroutines; locking the slice in place would mask
// the bug rather than fix it.
type Var struct {
	mu          sync.RWMutex
	root        Value
	gen         uint64
	bindings    []Value // legacy global stack — slated for removal once the
	                    // Execution-scoped binding model (see exec.go and
	                    // backlog/decisions/decision-1) is fully wired through
	                    // the bytecode VM. Until then both paths coexist.
	threadBound atomic.Bool // sticky: flipped true the first time this var is
	                        // dynamically bound on any Execution. Lets the
	                        // common deref skip the binding-frame lookup.
	nsref     *Namespace
	ns        string
	name      string
	isMacro   bool
	isDynamic bool
	isPrivate bool
}

func NewVar(nsref *Namespace, ns string, name string) *Var {
	return &Var{
		nsref: nsref,
		ns:    ns,
		name:  name,
		root:  NIL,
	}
}

// rootSnapshot returns the current root under the read lock. Callers that
// only need to read the root (Deref's root branch, Invoke, Arity) should
// use this rather than touching v.root directly.
func (v *Var) rootSnapshot() Value {
	v.mu.RLock()
	r := v.root
	v.mu.RUnlock()
	return r
}

func (v *Var) Invoke(values []Value) (Value, error) {
	r := v.rootSnapshot()
	f, ok := r.(Fn)
	if !ok {
		return NIL, fmt.Errorf("%v root does not implement Fn", r)
	}
	return f.Invoke(values)
}

func (v *Var) Arity() int {
	f, ok := v.rootSnapshot().(Fn)
	if !ok {
		return 0
	}
	return f.Arity()
}

func (v *Var) SetRoot(val Value) *Var {
	v.mu.Lock()
	v.root = val
	v.gen++
	v.mu.Unlock()
	return v
}

func (v *Var) Deref() Value {
	if len(v.bindings) > 0 {
		return v.bindings[len(v.bindings)-1]
	}
	return v.rootSnapshot()
}

// DerefIn returns the value of v in the given Execution: the innermost
// binding-frame value if any, else the root. This is the
// Execution-scoped reader that the bytecode VM will consult once the
// Frame.exec wiring lands (currently OP_LOAD_VAR uses Deref instead).
//
// Skips the binding-frame walk entirely if no Execution has ever bound
// this var dynamically (the threadBound fast-path).
func (v *Var) DerefIn(exec *Execution) Value {
	if !v.threadBound.Load() {
		return v.rootSnapshot()
	}
	if t := exec.Lookup(v); t != nil {
		return t.Value()
	}
	return v.rootSnapshot()
}

// SetIn updates the binding for v in the given Execution's nearest
// frame. Returns an error if v is not bound in this Execution, or if
// the binding's owner is a different Execution. This is the
// Execution-scoped set! that the bytecode VM will use once
// integrated.
func (v *Var) SetIn(exec *Execution, val Value) error {
	return exec.SetBound(v, val)
}

// markThreadBound flips the sticky threadBound flag. Called by
// Execution.PushBinding when this var first acquires a dynamic binding.
func (v *Var) markThreadBound() {
	v.threadBound.Store(true)
}

// AlterRoot atomically applies fn to the current root and stores the
// result. Mirrors Atom.Swap: fn runs outside the lock, a generation
// counter detects concurrent mutation, and on conflict the read/apply is
// retried with the fresh value. Bypasses the dynamic binding stack — root
// semantics, per Clojure's alter-var-root.
func (v *Var) AlterRoot(fn Fn, args []Value) (Value, error) {
	for {
		v.mu.RLock()
		old := v.root
		oldGen := v.gen
		v.mu.RUnlock()

		result, err := fn.Invoke(append([]Value{old}, args...))
		if err != nil {
			return NIL, err
		}

		v.mu.Lock()
		if v.gen == oldGen {
			v.root = result
			v.gen++
			v.mu.Unlock()
			return result, nil
		}
		v.mu.Unlock()
		// generation moved — another goroutine altered root; retry
	}
}

// PushBinding pushes a dynamic binding value. NOT goroutine-safe — see
// the type-level comment on Var.
func (v *Var) PushBinding(val Value) {
	v.bindings = append(v.bindings, val)
}

// PopBinding removes the most recent dynamic binding. NOT goroutine-safe
// — see the type-level comment on Var.
func (v *Var) PopBinding() {
	if len(v.bindings) > 0 {
		v.bindings = v.bindings[:len(v.bindings)-1]
	}
}

func (v *Var) Type() ValueType {
	return v.Deref().Type()
}

func (v *Var) Unbox() interface{} {
	return v.Deref().Unbox()
}

func (v *Var) String() string {
	return fmt.Sprintf("#'%s/%s", v.ns, v.name)
}

func (v *Var) IsMacro() bool {
	return v.isMacro
}

func (v *Var) IsDynamic() bool {
	return v.isDynamic
}

func (v *Var) IsPrivate() bool {
	return v.isPrivate
}

// NS returns the namespace name.
func (v *Var) NS() string { return v.ns }

// VarName returns the var name.
func (v *Var) VarName() string { return v.name }

func (v *Var) SetMacro() {
	v.isMacro = true
}

func (v *Var) SetDynamic() {
	v.isDynamic = true
}

func (v *Var) SetPrivate() {
	v.isPrivate = true
}

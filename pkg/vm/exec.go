/*
 * Copyright (c) 2026 Marcin Gasperowicz <xnooga@gmail.com>
 * SPDX-License-Identifier: MIT
 *
 * SPIKE: Option A-i (Execution-scoped binding model). See
 * backlog/decisions/decision-1 for the full design rationale.
 *
 * Status: prototype only. Coexists with the legacy Var.bindings stack —
 * the legacy `binding` macro and `push-binding!`/`pop-binding!` natives
 * still mutate Var.bindings. The bytecode integration that would route
 * deref / push / pop through Execution is the load-bearing follow-up
 * step that is intentionally NOT in this spike. Goal here is to give
 * @nooga the type-and-contract sketch to react to, with the four panel
 * acceptance tests proving the contract holds at this layer.
 */

package vm

import (
	"fmt"
	"sync"
)

// Execution carries dynamic state that flows with one logical execution
// path through the VM (a sequence of bytecode frames driven by one
// goroutine until it completes or spawns a child execution). The binding
// stack lives here, not on *Var.
//
// Single-driver invariant: an *Execution is intended to be driven by
// exactly one goroutine at a time. PushBinding / PopBinding / SetBound /
// Lookup are not safe to call concurrently from multiple goroutines on
// the same Execution. Conveyance creates a new Execution (see Fork) for
// the spawned goroutine to drive — that's the only correct way to share
// binding state across goroutine boundaries.
//
// The bindings inside a frame are TBoxes — mutable cells with an owner
// Execution. A child Execution forked via Fork() shares the parent's
// TBoxes (matches JVM Clojure binding-conveyor-fn semantics: conveyance
// shares the boxes; the receiving execution can READ them but cannot
// SET! them because of the owner check). The Execution.mu only guards
// the bindings linked-list head; per-TBox mutation is guarded by the
// TBox's own mu, which makes cross-execution reads safe.
type Execution struct {
	mu       sync.RWMutex
	bindings *BindingFrame // immutable linked list head; nil = no bindings
}

// BindingFrame is one node of the binding stack. The bindings map is
// immutable once published; pushes allocate a new BindingFrame whose
// `prev` points to the previous head.
type BindingFrame struct {
	bindings map[*Var]*TBox
	prev     *BindingFrame
}

// TBox is the per-binding mutable cell. Mirrors clojure.lang.Var.TBox:
// `set!` mutates the box in place rather than pushing a new frame, so
// nested callees can communicate back to their dynamic-extent caller
// (Hickey's stated intended use of set! on dynamic vars).
//
// The owner field is captured at push time and enforces JVM Clojure's
// "Can't set! from non-binding thread" check — for let-go, "non-binding
// execution".
type TBox struct {
	mu    sync.Mutex
	val   Value
	owner *Execution
}

// NewExecution returns a fresh execution with no bindings.
func NewExecution() *Execution {
	return &Execution{}
}

// Snapshot returns the current binding-frame head. The linked list of
// frames is immutable, but the TBoxes inside the frames are NOT — the
// receiving execution will see any subsequent mutations the owner
// Execution makes via SetIn (matches JVM Clojure binding-conveyor-fn
// semantics — conveyance shares the boxes, only the owner can set!).
//
// To create a child execution preinstalled with this snapshot, prefer
// Fork() over manual WithSnapshot — see Fork's docstring.
func (e *Execution) Snapshot() *BindingFrame {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	f := e.bindings
	e.mu.RUnlock()
	return f
}

// Fork is the canonical way to create a child Execution for spawning
// onto a new goroutine. The child shares the parent's TBoxes (so the
// child reads see the parent's current values, including any later
// SetIn from the parent), but cannot mutate them — the owner check on
// SetIn fails for the child Execution.
//
// Spawn sites (go / future / pmap / agent dispatch) should always use
// Fork rather than WithSnapshot(parent.Snapshot()) — Fork is the
// documented contract surface, the manual form is a low-level escape
// hatch.
func (e *Execution) Fork() *Execution {
	return WithSnapshot(e.Snapshot())
}

// WithSnapshot returns a fresh Execution preinstalled with the given
// frame. The TBoxes inside the frame retain their original owner — `set!`
// from the new execution will fail the owner check, just as on the JVM
// where bindings conveyed via binding-conveyor-fn are read-only on the
// receiving thread.
func WithSnapshot(snapshot *BindingFrame) *Execution {
	return &Execution{bindings: snapshot}
}

// PushBinding installs a new frame extending the current head with the
// given (var → value) mappings. Every TBox in the new frame is owned by
// this Execution. Vars are flagged as ever-thread-bound so future derefs
// know to consult the binding stack.
//
// Returns an error (matching Clojure's behaviour) if any var is not
// dynamic — non-dynamic vars must not be dynamically bindable, otherwise
// `set!` semantics break.
func (e *Execution) PushBinding(bindings map[*Var]Value) error {
	for v := range bindings {
		if !v.IsDynamic() {
			return fmt.Errorf("can't dynamically bind non-dynamic var: #'%s/%s", v.ns, v.name)
		}
	}
	if len(bindings) == 0 {
		e.mu.Lock()
		e.bindings = &BindingFrame{bindings: nil, prev: e.bindings}
		e.mu.Unlock()
		return nil
	}
	m := make(map[*Var]*TBox, len(bindings))
	for v, val := range bindings {
		m[v] = &TBox{val: val, owner: e}
		v.markThreadBound()
	}
	e.mu.Lock()
	e.bindings = &BindingFrame{bindings: m, prev: e.bindings}
	e.mu.Unlock()
	return nil
}

// PopBinding removes the topmost frame. Calling PopBinding without a
// matching push is a no-op — the contract is that the binding macro
// guarantees push/pop balance via try/finally, and an unbalanced pop is
// a compiler bug, not a runtime concern.
func (e *Execution) PopBinding() {
	e.mu.Lock()
	if e.bindings != nil {
		e.bindings = e.bindings.prev
	}
	e.mu.Unlock()
}

// Lookup returns the TBox for v in the nearest frame, or nil if v is
// not bound in this Execution. Walks the linked list one step at a time
// — O(stack-depth) for depth-d misses, O(1) for hits at the top frame.
//
// JVM Clojure folds the new bindings into the parent's bmap on push so
// lookup is one hash lookup regardless of depth. We keep it simple here;
// optimizing is a follow-up if profiling justifies it.
func (e *Execution) Lookup(v *Var) *TBox {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for f := e.bindings; f != nil; f = f.prev {
		if f.bindings == nil {
			continue
		}
		if t, ok := f.bindings[v]; ok {
			return t
		}
	}
	return nil
}

// SetBound mutates the value of v in the nearest frame. Returns an
// error if v has no binding in this Execution (matching Clojure's
// "Can't change/establish a binding without binding"), or if the
// binding's owner is a different Execution (matching JVM's "Can't set!
// from non-binding thread").
func (e *Execution) SetBound(v *Var, val Value) error {
	t := e.Lookup(v)
	if t == nil {
		return fmt.Errorf("can't change/establish a binding without binding: #'%s/%s", v.ns, v.name)
	}
	if t.owner != e {
		return fmt.Errorf("can't set!: #'%s/%s from non-binding execution", v.ns, v.name)
	}
	t.mu.Lock()
	t.val = val
	t.mu.Unlock()
	return nil
}

// Value returns the value held in this TBox.
func (t *TBox) Value() Value {
	t.mu.Lock()
	v := t.val
	t.mu.Unlock()
	return v
}

// ConveyedFn pairs a captured binding snapshot with an Fn, ready for a
// spawn site to install on the child goroutine before invoking the body.
// Once the bytecode VM is wired with Frame.exec and Fn.InvokeWithExec,
// spawn sites (go / future / pmap / agent dispatch in pkg/rt/lang.go
// and pkg/rt/async.go) will:
//
//	conveyed := BindingConveyorFn(currentExec, fn)
//	go conveyed.Inner.InvokeWithExec(conveyed.ChildExec(), args)
//
// Until InvokeWithExec exists, ConveyedFn deliberately does NOT
// implement the Fn interface — there is no useful Invoke we can offer
// without losing the child Execution. The struct exists to fix the
// shape so the wiring follow-up is mechanical.
type ConveyedFn struct {
	snapshot *BindingFrame
	Inner    Fn
}

// BindingConveyorFn captures parent's binding snapshot and pairs it
// with fn. Mirrors clojure.core/binding-conveyor-fn — captured TBoxes
// retain their original owner; the receiving Execution can read but
// not set!.
func BindingConveyorFn(parent *Execution, fn Fn) *ConveyedFn {
	return &ConveyedFn{snapshot: parent.Snapshot(), Inner: fn}
}

// ChildExec returns a fresh Execution pre-loaded with the captured
// snapshot. Spawn sites call this to build the child goroutine's
// Execution before invoking the inner fn.
func (c *ConveyedFn) ChildExec() *Execution {
	return WithSnapshot(c.snapshot)
}

/*
 * SPIKE A-i acceptance tests — see backlog/decisions/decision-1.
 *
 * Four panel-mandated tests proving the binding contract holds at the
 * vm.Execution layer. These run at the Go level, against the new
 * vm.Execution / vm.Var.DerefIn / vm.Var.SetIn primitives — they do NOT
 * yet exercise the bytecode VM or the binding macro. That integration
 * is the post-spike work pending @nooga's design sign-off.
 */

package vm

import (
	"strings"
	"sync"
	"testing"
)

// AC #1 — sibling invisibility.
//
// Two goroutines each push a different binding for the same dynamic var.
// Each sees its own value; neither sees the other's. Under the legacy
// Var.bindings model this fails (the global slice races and the loser's
// value either crashes or is overwritten).
func TestExecution_SiblingInvisibility(t *testing.T) {
	v := NewVar(nil, "test", "x").SetRoot(MakeInt(0))
	v.SetDynamic()

	const goroutines = 32
	const iters = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	leaks := make(chan int, goroutines*iters)

	for g := 0; g < goroutines; g++ {
		want := g + 1
		go func(want int) {
			defer wg.Done()
			exec := NewExecution()
			_ = exec.PushBinding(map[*Var]Value{v: MakeInt(want)})
			for i := 0; i < iters; i++ {
				got := int(v.DerefIn(exec).(Int))
				if got != want {
					leaks <- got
				}
			}
			exec.PopBinding()
		}(want)
	}

	wg.Wait()
	close(leaks)

	if leak, ok := <-leaks; ok {
		t.Errorf("sibling-invisibility violated: a goroutine observed %d (expected its own per-goroutine value)", leak)
	}
}

// AC #2 — throw-safe pop.
//
// In the spike's Go-level surface this is "the caller wraps in
// defer/recover; PopBinding still runs." Demonstrates the contract that
// the eventual binding-macro rewrite must enforce via try/finally.
func TestExecution_ThrowSafePop(t *testing.T) {
	v := NewVar(nil, "test", "y").SetRoot(MakeInt(10))
	v.SetDynamic()
	exec := NewExecution()

	func() {
		defer func() { _ = recover() }()
		_ = exec.PushBinding(map[*Var]Value{v: MakeInt(99)})
		defer exec.PopBinding() // this is what the new binding macro must emit

		if got := int(v.DerefIn(exec).(Int)); got != 99 {
			t.Fatalf("inside binding: want 99, got %d", got)
		}
		panic("simulated user-code throw")
	}()

	if got := int(v.DerefIn(exec).(Int)); got != 10 {
		t.Errorf("after throw: want root (10), got %d — pop did not run", got)
	}
}

// AC #3 — set! owner check.
//
// Goroutine A pushes a binding; goroutine B (different Execution)
// captures a snapshot of A's frame. B's set! must fail because B is not
// the owner. Mirrors JVM Clojure's "Can't set!: %s from non-binding
// thread".
func TestExecution_SetOwnerCheck(t *testing.T) {
	v := NewVar(nil, "test", "z").SetRoot(MakeInt(0))
	v.SetDynamic()

	execA := NewExecution()
	_ = execA.PushBinding(map[*Var]Value{v: MakeInt(1)})

	// A can set!
	if err := v.SetIn(execA, MakeInt(7)); err != nil {
		t.Fatalf("A's set! must succeed: %v", err)
	}
	if got := int(v.DerefIn(execA).(Int)); got != 7 {
		t.Errorf("after A's set!: want 7, got %d", got)
	}

	// B captures A's snapshot, then tries to set!.
	snapshot := execA.Snapshot()
	execB := WithSnapshot(snapshot)

	// B can read the conveyed value.
	if got := int(v.DerefIn(execB).(Int)); got != 7 {
		t.Errorf("B reading conveyed snapshot: want 7, got %d", got)
	}

	// B's set! must fail — owner is execA.
	err := v.SetIn(execB, MakeInt(99))
	if err == nil {
		t.Fatal("B's set! against conveyed binding must fail (owner is A); got nil err")
	}
	if !strings.Contains(err.Error(), "non-binding execution") {
		t.Errorf("expected 'non-binding execution' in error, got: %v", err)
	}

	// A's binding is still 7 — B's set! attempt did not mutate the TBox.
	if got := int(v.DerefIn(execA).(Int)); got != 7 {
		t.Errorf("after B's failed set!: A still expects 7, got %d", got)
	}
}

// AC #4 — conveyance.
//
// Parent execution pushes a binding; child is forked via Fork() (the
// canonical spawn-site API). The child Execution sees the captured
// value, and the captured snapshot survives parent's pop.
func TestExecution_Conveyance(t *testing.T) {
	v := NewVar(nil, "test", "w").SetRoot(MakeInt(0))
	v.SetDynamic()

	parent := NewExecution()
	_ = parent.PushBinding(map[*Var]Value{v: MakeInt(42)})
	defer parent.PopBinding()

	child := parent.Fork() // what spawn sites will call

	if got := int(v.DerefIn(child).(Int)); got != 42 {
		t.Errorf("child execution should see conveyed binding: want 42, got %d", got)
	}

	// Even after parent pops, the conveyed snapshot survives — the
	// captured TBox is still alive in the child's binding frame.
	parent.PopBinding()
	_ = parent.PushBinding(map[*Var]Value{v: MakeInt(0)}) // restore for the deferred Pop

	if got := int(v.DerefIn(child).(Int)); got != 42 {
		t.Errorf("conveyed snapshot must survive parent pop: want 42, got %d", got)
	}
}

// JVM-faithfulness lock-in: conveyance shares TBoxes, not values.
// After Fork, parent's later SetIn IS visible to the child (matches
// JVM Clojure binding-conveyor-fn — the cloned frame contains the same
// TBoxes; the receiving thread reads the parent's current values). The
// child cannot SetIn (owner check); covered separately by AC#3.
//
// This is locked in as a contract test rather than a behavioural option
// because it changes nothing for the common case (parent rarely SetIns
// after spawning) but makes the spec unambiguous. If a future maintainer
// wants value-snapshot semantics, this test will fail and force the
// conversation.
func TestExecution_ConveyanceSharesBoxes(t *testing.T) {
	v := NewVar(nil, "test", "share").SetRoot(MakeInt(0))
	v.SetDynamic()

	parent := NewExecution()
	_ = parent.PushBinding(map[*Var]Value{v: MakeInt(1)})
	defer parent.PopBinding()

	child := parent.Fork()
	if got := int(v.DerefIn(child).(Int)); got != 1 {
		t.Fatalf("child initial: want 1, got %d", got)
	}

	// Parent SetIn — child reads should reflect it (shared TBox).
	if err := v.SetIn(parent, MakeInt(99)); err != nil {
		t.Fatalf("parent SetIn: %v", err)
	}
	if got := int(v.DerefIn(child).(Int)); got != 99 {
		t.Errorf("after parent SetIn: child should see 99 (shared TBox), got %d", got)
	}

	// Child SetIn — must fail (owner check).
	if err := v.SetIn(child, MakeInt(7)); err == nil {
		t.Error("child SetIn must fail (owner is parent), got nil")
	}
}

// PushBinding rejects non-dynamic vars (matches Clojure's "Can't
// dynamically bind non-dynamic var" error from Var.pushThreadBindings).
func TestExecution_RejectsNonDynamicVar(t *testing.T) {
	v := NewVar(nil, "test", "static").SetRoot(MakeInt(0))
	// deliberately do NOT call SetDynamic

	exec := NewExecution()
	err := exec.PushBinding(map[*Var]Value{v: MakeInt(1)})
	if err == nil {
		t.Fatal("expected error binding non-dynamic var, got nil")
	}
	if !strings.Contains(err.Error(), "non-dynamic var") {
		t.Errorf("expected 'non-dynamic var' in error, got: %v", err)
	}
}

// Bonus — sibling executions cannot see each other's bindings even
// when running concurrently with set! on each. Proves the in-place
// set! mutation only touches the owner's TBox.
func TestExecution_ConcurrentSetIsolation(t *testing.T) {
	v := NewVar(nil, "test", "iso").SetRoot(MakeInt(0))
	v.SetDynamic()

	const goroutines = 16
	const iters = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		base := g * 1000
		go func(base int) {
			defer wg.Done()
			exec := NewExecution()
			_ = exec.PushBinding(map[*Var]Value{v: MakeInt(base)})
			for i := 0; i < iters; i++ {
				_ = v.SetIn(exec, MakeInt(base+i))
				if got := int(v.DerefIn(exec).(Int)); got != base+i {
					t.Errorf("set!/deref isolation broken: want %d got %d", base+i, got)
					return
				}
			}
			exec.PopBinding()
		}(base)
	}

	wg.Wait()
}

// Sanity — the threadBound fast-path: if no Execution has ever bound
// this var, DerefIn skips the binding-frame lookup entirely.
func TestExecution_ThreadBoundFastPath(t *testing.T) {
	v := NewVar(nil, "test", "fast").SetRoot(MakeInt(123))
	v.SetDynamic()

	if v.threadBound.Load() {
		t.Fatal("fresh var should not be threadBound")
	}

	exec := NewExecution()
	if got := int(v.DerefIn(exec).(Int)); got != 123 {
		t.Errorf("never-bound deref: want root (123), got %d", got)
	}

	// After any Execution binds it once, the flag stays set.
	other := NewExecution()
	_ = other.PushBinding(map[*Var]Value{v: MakeInt(999)})
	if !v.threadBound.Load() {
		t.Error("after PushBinding, threadBound must be true")
	}
	other.PopBinding()
	if !v.threadBound.Load() {
		t.Error("threadBound is sticky — must remain true after pop")
	}

	// Fresh execution still sees root (no bindings on this exec).
	exec2 := NewExecution()
	if got := int(v.DerefIn(exec2).(Int)); got != 123 {
		t.Errorf("unbound exec after fast-path flipped: want root (123), got %d", got)
	}
}

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

// testExecAwareFn is a minimal Fn-implementer that also satisfies
// ExecAwareFn — for testing InvokeFnWithExec dispatch.
type testExecAwareFn struct {
	gotExec *Execution
	gotArgs []Value
	via     string // "Invoke" or "InvokeWithExec"
}

func (f *testExecAwareFn) Type() ValueType    { return FuncType }
func (f *testExecAwareFn) Unbox() interface{} { return f }
func (f *testExecAwareFn) String() string     { return "<test-exec-aware-fn>" }
func (f *testExecAwareFn) Arity() int         { return -1 }
func (f *testExecAwareFn) Invoke(args []Value) (Value, error) {
	f.via = "Invoke"
	f.gotArgs = args
	return NIL, nil
}
func (f *testExecAwareFn) InvokeWithExec(e *Execution, args []Value) (Value, error) {
	f.via = "InvokeWithExec"
	f.gotExec = e
	f.gotArgs = args
	return NIL, nil
}

// InvokeFnWithExec routes ExecAwareFn implementers through
// InvokeWithExec (so the callee receives the caller's Execution).
func TestInvokeFnWithExec_DispatchesToExecAwareFn(t *testing.T) {
	fn := &testExecAwareFn{}
	exec := NewExecution()
	args := []Value{MakeInt(1), MakeInt(2)}

	if _, err := InvokeFnWithExec(exec, fn, args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fn.via != "InvokeWithExec" {
		t.Errorf("dispatch routed via %q, want InvokeWithExec", fn.via)
	}
	if fn.gotExec != exec {
		t.Errorf("ExecAwareFn did not receive caller's exec: got %p, want %p", fn.gotExec, exec)
	}
	if len(fn.gotArgs) != 2 {
		t.Errorf("args not threaded: got %v", fn.gotArgs)
	}
}

// InvokeFnWithExec falls back to plain Invoke for any Fn that does not
// implement ExecAwareFn — this is the backward-compat shim. Most Fn
// implementers (keywords, vectors, maps, sets, records, transients,
// vars-as-fn, typed arrays) will stay as plain Fn because their bodies
// never re-enter the VM and never need an Execution.
func TestInvokeFnWithExec_FallsBackToInvoke(t *testing.T) {
	// Keyword is a plain Fn — Invoke is "look me up in the map arg".
	kw := Keyword("x")
	m, _ := MapType.Box(map[Value]Value{Keyword("x"): MakeInt(42)})

	got, err := InvokeFnWithExec(NewExecution(), kw, []Value{m})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := got.(Int); !ok || int(v) != 42 {
		t.Errorf("fallback dispatch lost result: got %v (%T), want 42", got, got)
	}
}

// nil Execution must round-trip cleanly — early call sites in the
// bytecode VM may not yet have an exec to pass, and the helper must
// not panic. The aware implementer is permitted to see nil.
func TestInvokeFnWithExec_NilExec(t *testing.T) {
	fn := &testExecAwareFn{}
	if _, err := InvokeFnWithExec(nil, fn, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fn.via != "InvokeWithExec" {
		t.Errorf("nil-exec dispatch routed via %q, want InvokeWithExec", fn.via)
	}
	if fn.gotExec != nil {
		t.Errorf("nil exec should propagate as nil, got %p", fn.gotExec)
	}
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

// Regression lock-in (step-2 review, Hickey/Cox concern): DerefIn must
// not panic when called with a nil Execution after some OTHER execution
// has flipped the threadBound flag. Plain Invoke callers (REPL, Unbox
// proxies, every unmigrated native) pass nil exec; if a parallel
// goroutine binds the var, the legacy path must still read root safely.
func TestVar_DerefInNilExecWithThreadBound(t *testing.T) {
	v := NewVar(nil, "test", "step2-nilexec").SetRoot(MakeInt(7))
	v.SetDynamic()

	other := NewExecution()
	_ = other.PushBinding(map[*Var]Value{v: MakeInt(99)})
	defer other.PopBinding()

	if !v.threadBound.Load() {
		t.Fatal("setup: expected threadBound flipped")
	}

	// Must not panic; must return root because nil exec has no bindings.
	got := v.DerefIn(nil)
	if i, ok := got.(Int); !ok || int(i) != 7 {
		t.Errorf("DerefIn(nil) with threadBound true: want root (7), got %v", got)
	}
}

// --- Step-2 wiring: Frame.exec propagation through the bytecode VM ---
//
// Up to this point the spike's contract tests were Go-level only — they
// exercised the *Execution / DerefIn / SetIn primitives in isolation. The
// step-2 tests below prove the same contract holds end-to-end through a
// real bytecode call: when InvokeWithExec is the entry point, the
// caller's *Execution flows onto the callee Frame and OP_LOAD_VAR resolves
// against it.

// makeVarDerefFunc builds a 0-arity *Func whose body is `OP_LOAD_VAR v;
// OP_RETURN`. Returning the value of v at call time is the simplest body
// that touches the binding stack through bytecode.
func makeVarDerefFunc(v *Var) *Func {
	consts := NewConsts()
	idx := consts.Intern(v)
	c := NewCodeChunk(consts)
	c.SetMaxStack(2)
	c.Append(OP_LOAD_VAR)
	c.Append32(idx)
	c.Append(OP_RETURN)
	return MakeFunc(0, false, c)
}

// Step 2 / AC #1 — caller's binding visible inside a Func body via
// OP_LOAD_VAR. With Frame.exec wired, the callee's frame inherits exec
// and OP_LOAD_VAR routes through DerefIn(exec). Without the wiring the
// callee would read the root.
func TestFrameExec_FuncSeesCallerBinding(t *testing.T) {
	v := NewVar(nil, "test", "step2-func").SetRoot(MakeInt(0))
	v.SetDynamic()
	fn := makeVarDerefFunc(v)

	exec := NewExecution()
	_ = exec.PushBinding(map[*Var]Value{v: MakeInt(42)})
	defer exec.PopBinding()

	out, err := fn.InvokeWithExec(exec, nil)
	if err != nil {
		t.Fatalf("InvokeWithExec: %v", err)
	}
	if got := int(out.(Int)); got != 42 {
		t.Errorf("callee read root through bytecode — want 42, got %d", got)
	}
}

// Same for Closure — same body, wrapped via MakeClosure.
func TestFrameExec_ClosureSeesCallerBinding(t *testing.T) {
	v := NewVar(nil, "test", "step2-closure").SetRoot(MakeInt(0))
	v.SetDynamic()
	cls := makeVarDerefFunc(v).MakeClosure().(*Closure)

	exec := NewExecution()
	_ = exec.PushBinding(map[*Var]Value{v: MakeInt(7)})
	defer exec.PopBinding()

	out, err := cls.InvokeWithExec(exec, nil)
	if err != nil {
		t.Fatalf("InvokeWithExec: %v", err)
	}
	if got := int(out.(Int)); got != 7 {
		t.Errorf("closure read root through bytecode — want 7, got %d", got)
	}
}

// Step 2 / AC #2 — MultiArityFn dispatches to the body-bearing inner Fn
// via InvokeFnWithExec, so the inner Func's frame still inherits exec.
func TestFrameExec_MultiArityFnPropagatesExec(t *testing.T) {
	v := NewVar(nil, "test", "step2-multi").SetRoot(MakeInt(0))
	v.SetDynamic()

	zeroArity := makeVarDerefFunc(v)
	multi, err := makeMultiArity([]Value{zeroArity})
	if err != nil {
		t.Fatalf("makeMultiArity: %v", err)
	}

	exec := NewExecution()
	_ = exec.PushBinding(map[*Var]Value{v: MakeInt(99)})
	defer exec.PopBinding()

	out, err := multi.InvokeWithExec(exec, nil)
	if err != nil {
		t.Fatalf("multi InvokeWithExec: %v", err)
	}
	if got := int(out.(Int)); got != 99 {
		t.Errorf("multiarity didn't propagate exec — want 99, got %d", got)
	}
}

// Step 2 / AC #3 — nested call. Outer Func calls inner Func via
// OP_INVOKE; inner reads the var. The outer call site is reached via
// InvokeWithExec; the OP_INVOKE inside it must route through
// InvokeFnWithExec so the inner inherits exec too.
func TestFrameExec_NestedCallInheritsExec(t *testing.T) {
	v := NewVar(nil, "test", "step2-nested").SetRoot(MakeInt(0))
	v.SetDynamic()

	inner := makeVarDerefFunc(v)

	// outer: (inner) — push inner as const, OP_INVOKE 0, OP_RETURN.
	outerConsts := NewConsts()
	innerIdx := outerConsts.Intern(inner)
	c := NewCodeChunk(outerConsts)
	c.SetMaxStack(4)
	c.Append(OP_LOAD_CONST)
	c.Append32(innerIdx)
	c.Append(OP_INVOKE)
	c.Append32(0)
	c.Append(OP_RETURN)
	outer := MakeFunc(0, false, c)

	exec := NewExecution()
	_ = exec.PushBinding(map[*Var]Value{v: MakeInt(1234)})
	defer exec.PopBinding()

	out, err := outer.InvokeWithExec(exec, nil)
	if err != nil {
		t.Fatalf("outer InvokeWithExec: %v", err)
	}
	if got := int(out.(Int)); got != 1234 {
		t.Errorf("nested call lost exec — want 1234, got %d", got)
	}
}

// Step 2 / AC #4 — conveyance through Func invocation. Parent forks a
// child Execution; the child invokes a Func that reads v; the value the
// parent bound is visible. This is the building block that go/future
// spawn sites will use.
func TestFrameExec_ConveyanceThroughFunc(t *testing.T) {
	v := NewVar(nil, "test", "step2-conveyance").SetRoot(MakeInt(0))
	v.SetDynamic()
	fn := makeVarDerefFunc(v)

	parent := NewExecution()
	_ = parent.PushBinding(map[*Var]Value{v: MakeInt(555)})
	defer parent.PopBinding()

	child := parent.Fork()
	out, err := fn.InvokeWithExec(child, nil)
	if err != nil {
		t.Fatalf("InvokeWithExec on child: %v", err)
	}
	if got := int(out.(Int)); got != 555 {
		t.Errorf("conveyance through Func — want 555, got %d", got)
	}
}

// Step 2 / regression — invoking via plain Invoke (no exec) still works
// and reads the root. This is the backward-compat invariant: every
// call site that hasn't been migrated yet (rt natives, REPL entry,
// Unbox proxies) continues to function.
func TestFrameExec_PlainInvokeReadsRoot(t *testing.T) {
	v := NewVar(nil, "test", "step2-plain").SetRoot(MakeInt(11))
	v.SetDynamic()
	fn := makeVarDerefFunc(v)

	// Even if some other Execution has bound v, plain Invoke sees the
	// root — because no exec is threaded through.
	other := NewExecution()
	_ = other.PushBinding(map[*Var]Value{v: MakeInt(22)})
	defer other.PopBinding()

	out, err := fn.Invoke(nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got := int(out.(Int)); got != 11 {
		t.Errorf("plain Invoke should read root (sibling-invisible) — want 11, got %d", got)
	}
}

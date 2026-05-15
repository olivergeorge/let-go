package vm

import (
	"sync"
	"testing"
)

// TestAlterRootConcurrent is a coarse stress test: 100 goroutines × 1000
// increments = 100_000 expected. Detects gross AlterRoot regressions but
// is scheduler-dependent — see TestAlterRootRetryUnderConflict for a
// deterministic test of the generation/CAS retry, and
// TestVarSetRootRaceWithDeref for explicit -race coverage of the
// SetRoot/Deref pairing. Keep this test runnable under `-race`.
func TestAlterRootConcurrent(t *testing.T) {
	const goroutines = 100
	const iters = 1000

	v := NewVar(nil, "test", "counter").SetRoot(MakeInt(0))

	incFn, _ := NativeFnType.Wrap(func(vs []Value) (Value, error) {
		n := int(vs[0].(Int))
		return MakeInt(n + 1), nil
	})
	inc := incFn.(Fn)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if _, err := v.AlterRoot(inc, nil); err != nil {
					t.Errorf("AlterRoot: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got := int(v.Deref().(Int))
	if want := goroutines * iters; got != want {
		t.Errorf("AlterRoot: lost updates — got %d, want %d", got, want)
	}
}

// TestAlterRootBypassesBindings is the Go-level companion to the Lisp
// dynamic-binding-bypass tests in test/alter_var_root_test.lg. AlterRoot
// must read and write the root, not the top of the binding stack.
func TestAlterRootBypassesBindings(t *testing.T) {
	v := NewVar(nil, "test", "x").SetRoot(MakeInt(10))
	v.PushBinding(MakeInt(99))

	if got := int(v.Deref().(Int)); got != 99 {
		t.Fatalf("Deref under binding: want 99, got %d", got)
	}

	incFn, _ := NativeFnType.Wrap(func(vs []Value) (Value, error) {
		return MakeInt(int(vs[0].(Int)) + 1), nil
	})

	result, err := v.AlterRoot(incFn.(Fn), nil)
	if err != nil {
		t.Fatalf("AlterRoot: %v", err)
	}
	if got := int(result.(Int)); got != 11 {
		t.Errorf("AlterRoot result: want 11 (root+1), got %d", got)
	}
	if got := int(v.Deref().(Int)); got != 99 {
		t.Errorf("Deref after AlterRoot (binding still pushed): want 99, got %d", got)
	}

	v.PopBinding()
	if got := int(v.Deref().(Int)); got != 11 {
		t.Errorf("Deref after PopBinding: want 11 (root), got %d", got)
	}
}

// TestAlterRootErrorLeavesRootUnchanged — when fn returns an error,
// AlterRoot must not store anything and must surface the error.
func TestAlterRootErrorLeavesRootUnchanged(t *testing.T) {
	v := NewVar(nil, "test", "x").SetRoot(MakeInt(7))

	boomFn, _ := NativeFnType.Wrap(func(vs []Value) (Value, error) {
		return NIL, errBoom
	})

	_, err := v.AlterRoot(boomFn.(Fn), nil)
	if err == nil {
		t.Fatal("AlterRoot: expected error, got nil")
	}
	if got := int(v.Deref().(Int)); got != 7 {
		t.Errorf("AlterRoot error path mutated root: want 7, got %d", got)
	}
}

// errBoom is a sentinel error used by TestAlterRootErrorLeavesRootUnchanged.
var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

// TestAlterRootRetryUnderConflict deterministically exercises the
// generation-counter retry path. Both goroutines snapshot root=0 before
// either writes; both fns block until both have entered. Whichever lands
// the first CAS bumps gen 0→1 and writes 1; the other's CAS fails (its
// snapshot was gen 0), it retries, sees root=1 on second snapshot,
// computes 2, and writes 2.
//
// Without the retry loop in AlterRoot, both writes would be 1 and the
// final root would be 1 — this test fails deterministically in that case.
func TestAlterRootRetryUnderConflict(t *testing.T) {
	v := NewVar(nil, "test", "x").SetRoot(MakeInt(0))

	var entered sync.WaitGroup
	entered.Add(2)
	proceed := make(chan struct{})

	makeFn := func(once *sync.Once) Fn {
		f, _ := NativeFnType.Wrap(func(vs []Value) (Value, error) {
			once.Do(func() { entered.Done() })
			<-proceed
			return MakeInt(int(vs[0].(Int)) + 1), nil
		})
		return f.(Fn)
	}

	var oncesA, oncesB sync.Once
	go func() {
		entered.Wait()
		close(proceed)
	}()

	var done sync.WaitGroup
	done.Add(2)
	go func() {
		defer done.Done()
		if _, err := v.AlterRoot(makeFn(&oncesA), nil); err != nil {
			t.Errorf("AlterRoot A: %v", err)
		}
	}()
	go func() {
		defer done.Done()
		if _, err := v.AlterRoot(makeFn(&oncesB), nil); err != nil {
			t.Errorf("AlterRoot B: %v", err)
		}
	}()
	done.Wait()

	if got := int(v.Deref().(Int)); got != 2 {
		t.Errorf("AlterRoot retry under conflict: want 2, got %d (no-retry → lost update)", got)
	}
}

// TestVarSetRootRaceWithDeref exercises SetRoot vs Deref(root path)
// concurrently. Under `go test -race`, an unlocked SetRoot or unlocked
// rootSnapshot path would trip the race detector here. The functional
// assertion is weak (just "we got *some* value"); the load-bearing check
// is `-race` cleanliness.
func TestVarSetRootRaceWithDeref(t *testing.T) {
	v := NewVar(nil, "test", "x").SetRoot(MakeInt(0))

	const goroutines = 16
	const iters = 5000
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				v.SetRoot(MakeInt(i))
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = v.Deref()
			}
		}()
	}
	wg.Wait()
}

// TestVarInvokeRaceWithSetRoot exercises Var.Invoke / Var.Arity vs
// concurrent SetRoot replacing the underlying Fn. Under `-race`, an
// unlocked Invoke/Arity reading v.root directly would trip here.
func TestVarInvokeRaceWithSetRoot(t *testing.T) {
	v := NewVar(nil, "test", "f").SetRoot(NIL)

	identityFn, _ := NativeFnType.Wrap(func(vs []Value) (Value, error) {
		if len(vs) == 0 {
			return NIL, nil
		}
		return vs[0], nil
	})
	v.SetRoot(identityFn)

	const goroutines = 8
	const iters = 5000
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				v.SetRoot(identityFn)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_, _ = v.Invoke([]Value{MakeInt(i)})
				_ = v.Arity()
			}
		}()
	}
	wg.Wait()
}

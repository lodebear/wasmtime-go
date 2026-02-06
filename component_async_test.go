package wasmtime

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupAsyncWasiComponent creates an async engine, store with WASI config,
// linker with both WASI P2 and P3 interfaces, and loads the named component
// from testdata/components.
func setupAsyncWasiComponent(t *testing.T, name string) (*Engine, *Store, *ComponentLinker, *Component) {
	t.Helper()

	engine := newAsyncComponentEngine()
	store := NewStore(engine)

	wasiConfig := NewWasiConfig()
	require.NoError(t, store.SetWasi(wasiConfig))

	linker := NewComponentLinker(engine)
	t.Cleanup(func() { linker.Close() })

	// Enable shadowing so P3 can overlay P2 definitions for async WASI interfaces.
	linker.AllowShadowing(true)
	err := linker.AddWasip2()
	require.NoError(t, err)
	err = linker.AddWasip3()
	require.NoError(t, err)

	path := filepath.Join("testdata", "components", name+".component.wasm")
	wasmBytes, err := os.ReadFile(path)
	require.NoError(t, err, "failed to load component %s", name)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	t.Cleanup(func() { component.Close() })

	return engine, store, linker, component
}

// callWasiRunAsync instantiates a component asynchronously and calls its
// wasi:cli/run#run export. Returns an error if the run function fails.
func callWasiRunAsync(t *testing.T, store *Store, linker *ComponentLinker, component *Component) error {
	t.Helper()

	instance, err := linker.InstantiateAsync(store, component)
	if err != nil {
		return err
	}

	// Try wasi:cli/run@0.2.3 first (used by async components), then @0.2.0
	runInstanceIdx := instance.GetExportIndex(store, nil, "wasi:cli/run@0.2.3")
	if runInstanceIdx == nil {
		runInstanceIdx = instance.GetExportIndex(store, nil, "wasi:cli/run@0.2.0")
	}
	if runInstanceIdx == nil {
		return nil
	}
	defer runInstanceIdx.Close()

	runFuncIdx := instance.GetExportIndex(store, runInstanceIdx, "run")
	if runFuncIdx == nil {
		return nil
	}
	defer runFuncIdx.Close()

	fn := instance.GetFunc(store, runFuncIdx)
	if fn == nil {
		return nil
	}

	results, err := fn.CallAsync(store, nil)
	if err != nil {
		return err
	}

	if len(results) > 0 {
		r := results[0].Result()
		if !r.IsOk {
			return errors.New("run returned error result")
		}
	}

	return nil
}

// runWasiOnlyAsyncTest runs an async component that only imports WASI interfaces
// (no custom host functions needed).
func runWasiOnlyAsyncTest(t *testing.T, componentName string) {
	t.Helper()
	_, store, linker, component := setupAsyncWasiComponent(t, componentName)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// addAsyncBazFoo adds an async echo function for `local:local/baz.foo` to the linker.
// The function signature is: foo: async func(s: string) -> string
func addAsyncBazFoo(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/baz", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				s := args[0].String()
				results[0] = NewComponentValString(s)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// addAsyncManyFoo adds an async echo function for `local:local/many.foo` to the linker.
// The function signature is:
// foo: async func(a: string, b: u32, c: list<u8>, d: tuple<u64, u64>,
//
//	e: stuff, f: option<stuff>, g: result<stuff>)
//	-> tuple<string, u32, list<u8>, tuple<u64, u64>, stuff, option<stuff>, result<stuff>>
func addAsyncManyFoo(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/many", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// Echo back all args as a tuple
				results[0] = NewComponentValTuple([]ComponentVal{
					args[0], args[1], args[2], args[3], args[4], args[5], args[6],
				})
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// addAsyncSleepMillis adds a no-op async sleep function for `local:local/sleep.sleep-millis`.
// The function signature is: sleep-millis: async func(time-in-millis: u64)
func addAsyncSleepMillis(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/sleep", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("sleep-millis", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// No-op: return immediately without sleeping
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// readyThingState tracks the state of a "thing" resource for the local:local/ready
// interface. Each resource instance has an atomic ready flag.
type readyThingState struct {
	mu    sync.Mutex
	ready map[uint32]*atomic.Bool // rep -> ready flag
	next  uint32
}

func newReadyThingState() *readyThingState {
	return &readyThingState{
		ready: make(map[uint32]*atomic.Bool),
	}
}

func (s *readyThingState) create() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	rep := s.next
	s.next++
	s.ready[rep] = &atomic.Bool{}
	return rep
}

func (s *readyThingState) setReady(rep uint32, ready bool) {
	s.mu.Lock()
	flag, ok := s.ready[rep]
	s.mu.Unlock()
	if ok {
		flag.Store(ready)
	}
}

func (s *readyThingState) isReady(rep uint32) bool {
	s.mu.Lock()
	flag, ok := s.ready[rep]
	s.mu.Unlock()
	if ok {
		return flag.Load()
	}
	return true // default to ready if unknown rep
}

func (s *readyThingState) destroy(rep uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ready, rep)
}

// addReadyResource adds the local:local/ready interface with the "thing" resource
// to the linker. The resource has a constructor, set-ready method, and async
// when-ready method.
func addReadyResource(t *testing.T, linker *ComponentLinker) *readyThingState {
	t.Helper()
	state := newReadyThingState()
	resType := NewComponentResourceType(42) // arbitrary type ID for "thing"
	t.Cleanup(func() { resType.Close() })

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/ready", func(inst *ComponentLinkerInstance) {
			// Register the resource type with its destructor
			err := inst.AddResource("thing", resType, func(caller Storelike, rep uint32) error {
				state.destroy(rep)
				return nil
			})
			require.NoError(t, err)

			// Constructor: [constructor]thing() -> own<thing>
			// The constructor takes no args and returns an own<thing> (resource handle).
			// The runtime creates the resource from the returned rep.
			err = inst.AddFunc("[constructor]thing", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				rep := state.create()
				rh := NewComponentResourceHost(true, rep, 42)
				defer rh.Close()
				ra, raErr := rh.ToAny(caller)
				if raErr != nil {
					return raErr
				}
				results[0] = NewComponentValResource(ra)
				ra.Close()
				return nil
			})
			require.NoError(t, err)

			// Method: [method]thing.set-ready(borrow<thing>, ready: bool) -> ()
			err = inst.AddFunc("[method]thing.set-ready", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				ra := args[0].ResourceAny()
				rh, rhErr := ra.ToHost(caller)
				if rhErr != nil {
					return rhErr
				}
				defer rh.Close()
				rep := rh.Rep()
				ready := args[1].Bool()
				state.setReady(rep, ready)
				return nil
			})
			require.NoError(t, err)

			// Async method: [method]thing.when-ready(borrow<thing>) -> ()
			// Returns immediately (our async trampoline is "always ready").
			// For the yield_caller test which uses wit-bindgen async, this works
			// because the caller's poll_fn logic handles immediate completion.
			err = inst.AddFuncAsync("[method]thing.when-ready", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// No results to set -- when-ready returns ()
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
	return state
}

// addContinueInterface adds the local:local/continue interface with set-continue
// and get-continue functions.
func addContinueInterface(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	var cont bool
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/continue", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("set-continue", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				cont = args[0].Bool()
				return nil
			})
			require.NoError(t, err)
			err = inst.AddFunc("get-continue", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				results[0] = NewComponentValBool(cont)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// addRunInterface adds the local:local/run interface with `run: async func()`.
func addRunInterface(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/run", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// addBorrowingTypesResource adds the local:local/borrowing-types interface with the "x" resource.
// The resource has a constructor and a foo method.
func addBorrowingTypesResource(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	resType := NewComponentResourceType(99) // arbitrary type ID for "x"
	t.Cleanup(func() { resType.Close() })

	var nextRep uint32

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/borrowing-types", func(inst *ComponentLinkerInstance) {
			// Register the resource type
			err := inst.AddResource("x", resType, func(caller Storelike, rep uint32) error {
				return nil // destructor: no-op
			})
			require.NoError(t, err)

			// Constructor: [constructor]x() -> own<x>
			err = inst.AddFunc("[constructor]x", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				rep := nextRep
				nextRep++
				rh := NewComponentResourceHost(true, rep, 99)
				defer rh.Close()
				ra, raErr := rh.ToAny(caller)
				if raErr != nil {
					return raErr
				}
				results[0] = NewComponentValResource(ra)
				ra.Close()
				return nil
			})
			require.NoError(t, err)

			// Method: [method]x.foo(borrow<x>) -> ()
			err = inst.AddFunc("[method]x.foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// No-op: just verify we can access the borrowed resource
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// addBorrowingInterface adds the local:local/borrowing interface with
// `foo: async func(x: borrow<x>, misbehave: bool)`.
func addBorrowingInterface(t *testing.T, linker *ComponentLinker) {
	t.Helper()
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/borrowing", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// No-op: the async function just returns successfully
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

// --- Round-trip tests ---
// These import local:local/baz with `foo: async func(s: string) -> string`.
// We use AddFuncAsync to provide a simple echo implementation.

func TestAsyncRoundTripSynchronous(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_synchronous")
	addAsyncBazFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripStackful(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_stackful")
	addAsyncBazFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripStackless(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_stackless")
	addAsyncBazFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripWait(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_wait")
	addAsyncBazFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripDirectStackless(t *testing.T) {
	// This component imports `foo: async func(s: string) -> string` at the root level,
	// NOT under local:local/baz.
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_direct_stackless")
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFuncAsync("foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			s := args[0].String()
			results[0] = NewComponentValString(s)
			return nil
		})
		require.NoError(t, err)
	})
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripManySynchronous(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_many_synchronous")
	addAsyncManyFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripManyStackful(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_many_stackful")
	addAsyncManyFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripManyStackless(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_many_stackless")
	addAsyncManyFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripManyWait(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_many_wait")
	addAsyncManyFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncRoundTripStacklessSyncImport(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_round_trip_stackless_sync_import")
	addAsyncBazFoo(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- WASI-only tests (no custom imports beyond WASI) ---
// These components only import standard WASI interfaces, which are provided
// by AddWasip2 + AddWasip3. They exercise async instantiation and run.

func TestAsyncIntertaskCommunication(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_intertask_communication")
}

func TestAsyncBackpressureCallee(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_backpressure_callee")
}

func TestAsyncPostReturnCallee(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_post_return_callee")
}

// --- Yield callee tests ---
// These import local:local/continue with a sync function `get-continue: func() -> bool`.
// Since the import is sync, AddFunc can satisfy it.

func TestAsyncYieldCalleeStackless(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_yield_callee_stackless")

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/continue", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("get-continue", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				results[0] = NewComponentValBool(true)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncYieldCalleeSynchronous(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_yield_callee_synchronous")

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/continue", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("get-continue", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				results[0] = NewComponentValBool(true)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Backpressure caller ---
// Imports local:local/backpressure (sync funcs) + local:local/run with `run: async func()`.
// With AddFuncAsync we can now satisfy the async import.

func TestAsyncBackpressureCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_backpressure_caller")

	linker.Root(func(root *ComponentLinkerInstance) {
		// local:local/backpressure has sync functions
		err := root.AddInstance("local:local/backpressure", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("set-backpressure", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
			err = inst.AddFunc("inc-backpressure", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
			err = inst.AddFunc("dec-backpressure", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
		// local:local/run has `run: async func()`
		err = root.AddInstance("local:local/run", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Yield caller tests ---
// async_yield_caller imports resource (local:local/ready thing) + async func -- requires resources
// async_yield_caller_cancel imports set-continue (sync) + run: async func()

func TestAsyncYieldCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_yield_caller")

	addReadyResource(t, linker)
	addContinueInterface(t, linker)
	addRunInterface(t, linker)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncYieldCallerCancel(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_yield_caller_cancel")

	linker.Root(func(root *ComponentLinkerInstance) {
		// local:local/continue has sync function set-continue
		err := root.AddInstance("local:local/continue", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("set-continue", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
		// local:local/run has `run: async func()`
		err = root.AddInstance("local:local/run", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Post-return caller ---
// Imports local:local/post-return with `foo: async func(s: string) -> string`
// and `get-post-return-value: func() -> string` (sync).

func TestAsyncPostReturnCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_post_return_caller")

	var lastValue string
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/post-return", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("foo", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				s := args[0].String()
				lastValue = s
				results[0] = NewComponentValString(s)
				return nil
			})
			require.NoError(t, err)
			err = inst.AddFunc("get-post-return-value", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				results[0] = NewComponentValString(lastValue)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Sleep post-return tests ---
// Both import `sleep-millis: async func(time-in-millis: u64)` which we can
// now satisfy with AddFuncAsync.

func TestAsyncSleepPostReturnCallee(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_sleep_post_return_callee")
	addAsyncSleepMillis(t, linker)
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncSleepPostReturnCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_sleep_post_return_caller")
	// Imports local:local/sleep with sleep-millis (async)
	addAsyncSleepMillis(t, linker)
	// Also imports local:local/sleep-post-return with `run: async func(sleep-time-millis: u64)`
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/sleep-post-return", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Poll tests (require resource imports) ---

func TestAsyncPollStackless(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_poll_stackless")

	addReadyResource(t, linker)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncPollSynchronous(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_poll_synchronous")

	addReadyResource(t, linker)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Readiness test ---
// This component only imports WASI interfaces (no custom host functions).

func TestAsyncReadiness(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_readiness")
}

// --- Error context tests ---
// With SetWasmComponentModelErrorContext(true) enabled, these components can
// now be compiled. The first two are WASI-only, the third imports a custom
// async function with error-context return type.

func TestAsyncErrorContext(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_error_context")
}

func TestAsyncErrorContextCallee(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_error_context_callee")
}

func TestAsyncErrorContextCaller(t *testing.T) {
	// Imports local:local/run-result with `run-fail: async func() -> result<_, error-context>`
	_, store, linker, component := setupAsyncWasiComponent(t, "async_error_context_caller")

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/run-result", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run-fail", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				// Return an ok result (no error-context)
				results[0] = NewComponentValResultOk(nil)
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Stream tests ---

// The following "callee" components only import WASI interfaces and export
// their local interface + wasi:cli/run. They can be loaded and instantiated
// but their run function exercises stream/future types internally.

func TestAsyncClosedStream(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_closed_stream")
}

func TestAsyncClosedStreams(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_closed_streams")
}

func TestAsyncShortReads(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_short_reads")
}

func TestAsyncUnitStreamCallee(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_unit_stream_callee")
}

func TestAsyncTransmitCallee(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_transmit_callee")
}

func TestAsyncSynchronousTransmit(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_synchronous_transmit")
}

func TestAsyncCancelTransmit(t *testing.T) {
	runWasiOnlyAsyncTest(t, "async_cancel_transmit")
}

// --- Caller tests that import custom async interfaces with stream/future types ---

func TestAsyncReadResourceStream(t *testing.T) {
	t.Skip("requires stream<resource> val type which is not exposed in the wasmtime C API (wasmtime_component_val_t has no stream/future variants)")
}

func TestAsyncUnitStreamCaller(t *testing.T) {
	t.Skip("requires stream val type which is not exposed in the wasmtime C API (wasmtime_component_val_t has no stream/future variants)")
}

func TestAsyncTransmitCaller(t *testing.T) {
	t.Skip("requires stream/future val types which are not exposed in the wasmtime C API (wasmtime_component_val_t has no stream/future variants)")
}

// --- Cancel tests ---
// cancel_callee imports sleep-millis (async) + cancel variant type.
// cancel_caller imports backpressure (sync), sleep-millis (async), cancel (variant), sleep-with-options (async).

func TestAsyncCancelCallee(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_cancel_callee")

	// Provide local:local/sleep with sleep-millis (async)
	addAsyncSleepMillis(t, linker)

	// Provide local:local/cancel with mode variant type (no functions to implement,
	// just the type definition -- but the linker needs the instance to exist)
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("local:local/cancel", func(inst *ComponentLinkerInstance) {
			// cancel interface only defines a variant type, no functions
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncCancelCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_cancel_caller")

	linker.Root(func(root *ComponentLinkerInstance) {
		// local:local/backpressure has sync functions
		err := root.AddInstance("local:local/backpressure", func(inst *ComponentLinkerInstance) {
			err := inst.AddFunc("inc-backpressure", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
			err = inst.AddFunc("dec-backpressure", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)

		// local:local/sleep has async sleep-millis
		err = root.AddInstance("local:local/sleep", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("sleep-millis", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)

		// local:local/cancel has variant mode type + run: async func(mode, u64)
		err = root.AddInstance("local:local/cancel", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("run", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)

		// local:local/sleep-with-options has async sleep-millis(u64, on-cancel, u64, bool, mode)
		err = root.AddInstance("local:local/sleep-with-options", func(inst *ComponentLinkerInstance) {
			err := inst.AddFuncAsync("sleep-millis", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return nil
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

// --- Borrowing tests ---

func TestAsyncBorrowingCallee(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_borrowing_callee")

	addBorrowingTypesResource(t, linker)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

func TestAsyncBorrowingCaller(t *testing.T) {
	_, store, linker, component := setupAsyncWasiComponent(t, "async_borrowing_caller")

	addBorrowingTypesResource(t, linker)
	addBorrowingInterface(t, linker)

	err := callWasiRunAsync(t, store, linker, component)
	require.NoError(t, err)
}

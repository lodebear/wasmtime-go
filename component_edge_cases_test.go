package wasmtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// Edge case: Wasm function that traps via `unreachable`
// Inspired by: Rust func.rs `thunks` test
//
// Existing Go tests never call a function that traps. This verifies
// that traps from guest code propagate as Go errors.
// =============================================================================

func TestEdgeCaseTrap(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "thunk"))
        (func (export "thunk-trap") unreachable)
    )
    (core instance $i (instantiate $m))
    (func (export "thunk")
        (canon lift (core func $i "thunk")))
    (func (export "thunk-trap")
        (canon lift (core func $i "thunk-trap")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// "thunk" should succeed with no error
	thunkIdx := instance.GetExportIndex(store, nil, "thunk")
	require.NotNil(t, thunkIdx)
	defer thunkIdx.Close()
	thunk := instance.GetFunc(store, thunkIdx)
	require.NotNil(t, thunk)
	_, err = thunk.Call(store, []ComponentVal{})
	require.NoError(t, err)

	// "thunk-trap" should fail with an error containing "unreachable"
	// Need a fresh store because the instance is poisoned after a trap
	store2 := NewStore(engine)
	instance2, err := linker.Instantiate(store2, component)
	require.NoError(t, err)
	trapIdx := instance2.GetExportIndex(store2, nil, "thunk-trap")
	require.NotNil(t, trapIdx)
	defer trapIdx.Close()
	trapFn := instance2.GetFunc(store2, trapIdx)
	require.NotNil(t, trapFn)
	_, err = trapFn.Call(store2, []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unreachable")
}

// =============================================================================
// Edge case: Integer truncation/wrapping on return
// Inspired by: Rust func.rs `integers` test
//
// Existing Go tests only pass integers and verify exact round-trips.
// This tests that returning i32(-1) as u8 produces 255 (0xff), i.e. proper
// truncation happens. Also verifies returning 100000 truncates for u8/s8/u16/s16.
// =============================================================================

func TestEdgeCaseIntegerTruncation(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "ret-minus-1") (result i32) i32.const -1)
        (func (export "ret-100000") (result i32) i32.const 100000)
    )
    (core instance $i (instantiate $m))

    (func (export "retm1-u8") (result u8) (canon lift (core func $i "ret-minus-1")))
    (func (export "retm1-s8") (result s8) (canon lift (core func $i "ret-minus-1")))
    (func (export "retm1-u16") (result u16) (canon lift (core func $i "ret-minus-1")))
    (func (export "retm1-s16") (result s16) (canon lift (core func $i "ret-minus-1")))
    (func (export "retm1-u32") (result u32) (canon lift (core func $i "ret-minus-1")))
    (func (export "retm1-s32") (result s32) (canon lift (core func $i "ret-minus-1")))

    (func (export "retbig-u8") (result u8) (canon lift (core func $i "ret-100000")))
    (func (export "retbig-u16") (result u16) (canon lift (core func $i "ret-100000")))
    (func (export "retbig-u32") (result u32) (canon lift (core func $i "ret-100000")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	callExport := func(name string) []ComponentVal {
		idx := instance.GetExportIndex(store, nil, name)
		require.NotNil(t, idx, "missing export %s", name)
		defer idx.Close()
		fn := instance.GetFunc(store, idx)
		require.NotNil(t, fn)
		results, err := fn.Call(store, []ComponentVal{})
		require.NoError(t, err)
		require.Len(t, results, 1)
		return results
	}

	// Returning -1 (all bits set) should produce max unsigned / -1 signed values
	// u8: 0xFF = 255
	r := callExport("retm1-u8")
	require.Equal(t, ComponentValKindU8, r[0].Kind())
	require.Equal(t, uint8(0xFF), r[0].U8())

	// s8: -1
	r = callExport("retm1-s8")
	require.Equal(t, ComponentValKindS8, r[0].Kind())
	require.Equal(t, int8(-1), r[0].S8())

	// u16: 0xFFFF = 65535
	r = callExport("retm1-u16")
	require.Equal(t, ComponentValKindU16, r[0].Kind())
	require.Equal(t, uint16(0xFFFF), r[0].U16())

	// s16: -1
	r = callExport("retm1-s16")
	require.Equal(t, ComponentValKindS16, r[0].Kind())
	require.Equal(t, int16(-1), r[0].S16())

	// u32: 0xFFFFFFFF
	r = callExport("retm1-u32")
	require.Equal(t, ComponentValKindU32, r[0].Kind())
	require.Equal(t, uint32(0xFFFFFFFF), r[0].U32())

	// s32: -1
	r = callExport("retm1-s32")
	require.Equal(t, ComponentValKindS32, r[0].Kind())
	require.Equal(t, int32(-1), r[0].S32())

	// Returning 100000 (0x186A0) should truncate for narrow types
	// u8: 100000 & 0xFF = 0xA0 = 160
	r = callExport("retbig-u8")
	require.Equal(t, uint8(100000&0xFF), r[0].U8())

	// u16: 100000 & 0xFFFF = 0x86A0 = 34464
	r = callExport("retbig-u16")
	require.Equal(t, uint16(100000&0xFFFF), r[0].U16())

	// u32: 100000 (fits)
	r = callExport("retbig-u32")
	require.Equal(t, uint32(100000), r[0].U32())
}

// =============================================================================
// Edge case: Bool round-trip through a component
// Inspired by: Rust func.rs `bools` test
//
// Existing Go tests only test Go-side bool construction. This tests that
// bools pass through component function calls correctly and that non-zero
// i32 values map to true.
// =============================================================================

func TestEdgeCaseBoolRoundTrip(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "pass") (param i32) (result i32) local.get 0)
    )
    (core instance $i (instantiate $m))
    (func (export "bool-to-u32") (param "a" bool) (result u32)
        (canon lift (core func $i "pass")))
    (func (export "u32-to-bool") (param "a" u32) (result bool)
        (canon lift (core func $i "pass")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	callFunc := func(name string, args []ComponentVal) []ComponentVal {
		idx := instance.GetExportIndex(store, nil, name)
		require.NotNil(t, idx)
		defer idx.Close()
		fn := instance.GetFunc(store, idx)
		require.NotNil(t, fn)
		results, err := fn.Call(store, args)
		require.NoError(t, err)
		require.Len(t, results, 1)
		return results
	}

	// bool -> u32: false=0, true=1
	r := callFunc("bool-to-u32", []ComponentVal{NewComponentValBool(false)})
	require.Equal(t, uint32(0), r[0].U32())
	r = callFunc("bool-to-u32", []ComponentVal{NewComponentValBool(true)})
	require.Equal(t, uint32(1), r[0].U32())

	// u32 -> bool: 0=false, 1=true, 2=true (non-zero is true)
	r = callFunc("u32-to-bool", []ComponentVal{NewComponentValU32(0)})
	require.False(t, r[0].Bool())
	r = callFunc("u32-to-bool", []ComponentVal{NewComponentValU32(1)})
	require.True(t, r[0].Bool())
	r = callFunc("u32-to-bool", []ComponentVal{NewComponentValU32(2)})
	require.True(t, r[0].Bool())
}

// =============================================================================
// Edge case: Char round-trip through a component
// Inspired by: Rust func.rs `chars` test
//
// Existing Go tests only create char values on the Go side. This tests
// that chars pass through component calls correctly, including multi-byte
// Unicode characters.
// =============================================================================

func TestEdgeCaseCharRoundTrip(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "pass") (param i32) (result i32) local.get 0)
    )
    (core instance $i (instantiate $m))
    (func (export "char-to-u32") (param "a" char) (result u32)
        (canon lift (core func $i "pass")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "char-to-u32")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	testCases := []rune{'x', 'a', '\n', 0, 0x1F600} // includes emoji (💀) and NUL
	for _, ch := range testCases {
		results, err := fn.Call(store, []ComponentVal{NewComponentValChar(ch)})
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, uint32(ch), results[0].U32(), "char round-trip failed for U+%04X", ch)
	}
}

// =============================================================================
// Edge case: Float pass-through via reinterpret
// Inspired by: Rust func.rs `floats` test
//
// Existing Go tests only create float values on the Go side. This tests
// float32 and float64 round-trip through component functions via
// reinterpret operations.
// =============================================================================

func TestEdgeCaseFloatRoundTrip(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "f32-pass") (param f32) (result f32) local.get 0)
        (func (export "f64-pass") (param f64) (result f64) local.get 0)
    )
    (core instance $i (instantiate $m))
    (func (export "f32-pass") (param "a" float32) (result float32)
        (canon lift (core func $i "f32-pass")))
    (func (export "f64-pass") (param "a" float64) (result float64)
        (canon lift (core func $i "f64-pass")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// float32 round-trip
	f32Idx := instance.GetExportIndex(store, nil, "f32-pass")
	require.NotNil(t, f32Idx)
	defer f32Idx.Close()
	f32Fn := instance.GetFunc(store, f32Idx)
	require.NotNil(t, f32Fn)

	f32Results, err := f32Fn.Call(store, []ComponentVal{NewComponentValF32(3.14)})
	require.NoError(t, err)
	require.Len(t, f32Results, 1)
	require.Equal(t, ComponentValKindF32, f32Results[0].Kind())
	require.Equal(t, float32(3.14), f32Results[0].F32())

	// float64 round-trip
	f64Idx := instance.GetExportIndex(store, nil, "f64-pass")
	require.NotNil(t, f64Idx)
	defer f64Idx.Close()
	f64Fn := instance.GetFunc(store, f64Idx)
	require.NotNil(t, f64Fn)

	f64Results, err := f64Fn.Call(store, []ComponentVal{NewComponentValF64(2.718281828)})
	require.NoError(t, err)
	require.Len(t, f64Results, 1)
	require.Equal(t, ComponentValKindF64, f64Results[0].Kind())
	require.Equal(t, float64(2.718281828), f64Results[0].F64())

	// Negative zero
	negZeroResults, err := f32Fn.Call(store, []ComponentVal{NewComponentValF32(-0.0)})
	require.NoError(t, err)
	require.Equal(t, float32(-0.0), negZeroResults[0].F32())
}

// =============================================================================
// Edge case: Post-return handler validates return values
// Inspired by: Rust post_return.rs `post_return_all_types` test
//
// Existing Go tests never exercise post-return. This creates a component
// whose `canon lift` specifies a `(post-return ...)` function that validates
// the returned value. If the post-return assertion fails, the call itself
// should still succeed (post-return runs after results are read).
// =============================================================================

func TestEdgeCasePostReturn(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "get-i32") (result i32) i32.const 42)
        (func (export "post-i32") (param i32)
            local.get 0
            i32.const 42
            i32.ne
            if unreachable end)
    )
    (core instance $i (instantiate $m))
    (func (export "get-i32") (result u32)
        (canon lift (core func $i "get-i32") (post-return (func $i "post-i32"))))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "get-i32")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	results, err := fn.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, uint32(42), results[0].U32())
}

// =============================================================================
// Edge case: Trap in post-return poisons the instance
// Inspired by: Rust post_return.rs `trap_in_post_return_poisons_instance`
//
// If a post-return handler traps, the component instance should be
// unusable for subsequent calls (it enters a "poisoned" state).
// =============================================================================

func TestEdgeCasePostReturnTrapPoisons(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "f"))
        (func (export "post") unreachable)
    )
    (core instance $i (instantiate $m))
    (func (export "f")
        (canon lift (core func $i "f") (post-return (func $i "post"))))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "f")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	// First call should fail with unreachable trap (from post-return)
	_, err = fn.Call(store, []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unreachable")

	// Second call should fail because the instance is poisoned
	_, err = fn.Call(store, []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot enter component")
}

// =============================================================================
// Edge case: Host function returning an error
// Inspired by: General Rust host function error handling patterns
//
// Existing Go tests always have host callbacks succeed. This tests that
// returning an error from a host callback propagates as an error to the caller.
// =============================================================================

func TestEdgeCaseHostFuncError(t *testing.T) {
	wat := `
(component
    (import "fail" (func $fail))
    (core func $fail_lower (canon lower (func $fail)))
    (core module $m
        (import "" "fail" (func $fail))
        (func (export "call") call $fail)
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "fail" (func $fail_lower))))))
    (func (export "call") (canon lift (core func $i "call")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFunc("fail", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			return &customError{msg: "intentional host error"}
		})
		require.NoError(t, err)
	})

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "call")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	_, err = fn.Call(store, []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "intentional host error")
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

// =============================================================================
// Edge case: Empty list round-trip through a component
// Inspired by: Rust func.rs strings/list tests (empty string, empty list)
//
// Existing Go tests only pass non-empty lists. This verifies that an empty
// list passes through correctly.
// =============================================================================

func TestEdgeCaseEmptyListRoundTrip(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(list u32)`,
			`(param $x i32)
(param $y i32)
(result i32)
(local $res i32)
local.get $x
local.get $y
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 8))
local.tee $res
call $do
local.get $res`,
			"(param i32 i32 i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			// Return an empty list
			results[0] = NewComponentValList([]ComponentVal{})
			return nil
		},
	)
	defer ctx.Close()

	// Send empty list
	arg := NewComponentValList([]ComponentVal{})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received empty list
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindList, hostArgs[0].Kind())
	require.Empty(t, hostArgs[0].List())

	// Verify result is also empty
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindList, results[0].Kind())
	require.Empty(t, results[0].List())
}

// =============================================================================
// Edge case: Multiple integer types as parameters
// Inspired by: Rust func.rs `integers` test (take-* family)
//
// Existing Go tests only pass u32 params. This tests passing different
// integer widths (s8, u16, s32, u64) to component functions.
// =============================================================================

func TestEdgeCaseMultipleIntegerParams(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "add-mixed") (param i32 i32 i64) (result i64)
            (i64.add
                (i64.extend_i32_u (i32.add (local.get 0) (local.get 1)))
                (local.get 2))
        )
    )
    (core instance $i (instantiate $m))
    (func (export "add-mixed") (param "a" u8) (param "b" s16) (param "c" u64) (result u64)
        (canon lift (core func $i "add-mixed")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "add-mixed")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	// 10 + 20 + 1000 = 1030
	results, err := fn.Call(store, []ComponentVal{
		NewComponentValU8(10),
		NewComponentValS16(20),
		NewComponentValU64(1000),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, uint64(1030), results[0].U64())
}

// =============================================================================
// Edge case: Nested option types (option<option<u32>>)
// Inspired by: Rust func.rs tests with nested compound types
//
// Existing Go tests only test option<u32>. This tests nested options.
// =============================================================================

func TestEdgeCaseNestedOption(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(option (option u32))`,
			`(param $disc1 i32)
(param $disc2 i32)
(param $val i32)
(result i32)
(local $res i32)
local.get $disc1
local.get $disc2
local.get $val
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 12))
local.tee $res
call $do
local.get $res`,
			"(param i32 i32 i32 i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args

			// Return Some(None)
			results[0] = NewComponentValOptionSome(NewComponentValOptionNone())
			return nil
		},
	)
	defer ctx.Close()

	// Send Some(Some(42))
	inner := NewComponentValOptionSome(NewComponentValU32(42))
	arg := NewComponentValOptionSome(inner)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received Some(Some(42))
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindOption, hostArgs[0].Kind())
	outerOpt := hostArgs[0].Option()
	require.NotNil(t, outerOpt)
	require.Equal(t, ComponentValKindOption, outerOpt.Kind())
	innerOpt := outerOpt.Option()
	require.NotNil(t, innerOpt)
	require.Equal(t, uint32(42), innerOpt.U32())

	// Verify result is Some(None)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindOption, results[0].Kind())
	resOuter := results[0].Option()
	require.NotNil(t, resOuter)
	require.Equal(t, ComponentValKindOption, resOuter.Kind())
	require.Nil(t, resOuter.Option())
}

// =============================================================================
// Edge case: Host-defined resource with import/export and destructor callback
// Inspired by: Rust resources.rs host_resource_types / resource_any tests
//
// Existing Go tests only exercise guest-defined resources. This tests
// host-defined resources: the host creates a resource type, defines it
// in the linker, and the guest creates/drops instances while the host
// tracks destructor calls.
// =============================================================================

func TestEdgeCaseHostResource(t *testing.T) {
	wat := `
(component
    (import "t" (type $t (sub resource)))
    (import "ctor" (func $ctor (param "rep" u32) (result (own $t))))
    (import "dtor" (func $dtor (param "r" (own $t))))

    (core module $libc
        (memory (export "memory") 1)
    )
    (core instance $libc (instantiate $libc))

    (core func $ctor_lower (canon lower (func $ctor)
        (memory $libc "memory")))
    (core func $dtor_lower (canon lower (func $dtor)
        (memory $libc "memory")))

    (core module $m
        (import "" "ctor" (func $ctor (param i32) (result i32)))
        (import "" "dtor" (func $dtor (param i32)))

        (func (export "roundtrip") (param i32) (result i32)
            local.get 0
            call $ctor
        )
        (func (export "drop") (param i32)
            local.get 0
            call $dtor
        )
    )
    (core instance $i (instantiate $m
        (with "" (instance
            (export "ctor" (func $ctor_lower))
            (export "dtor" (func $dtor_lower))
        ))))

    (func (export "roundtrip") (param "rep" u32) (result (own $t))
        (canon lift (core func $i "roundtrip")))
    (func (export "drop") (param "r" (own $t))
        (canon lift (core func $i "drop")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	// Track destructor calls
	var dtorCalls []uint32

	resourceType := NewComponentResourceType(1)
	defer resourceType.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddResource("t", resourceType, func(caller Storelike, rep uint32) error {
			dtorCalls = append(dtorCalls, rep)
			return nil
		})
		require.NoError(t, err)

		err = root.AddFunc("ctor", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			rep := args[0].U32()
			// Create a host resource with the given rep
			rh := NewComponentResourceHost(true, rep, 1)
			defer rh.Close()
			ra, err := rh.ToAny(caller)
			if err != nil {
				return err
			}
			results[0] = NewComponentValResource(ra)
			return nil
		})
		require.NoError(t, err)

		err = root.AddFunc("dtor", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			require.Equal(t, ComponentValKindResource, args[0].Kind())
			ra := args[0].ResourceAny()
			return ra.DropAndClose(caller)
		})
		require.NoError(t, err)
	})

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// Get exported functions
	getFunc := func(name string) *ComponentFunc {
		idx := instance.GetExportIndex(store, nil, name)
		require.NotNil(t, idx, "missing export %s", name)
		defer idx.Close()
		fn := instance.GetFunc(store, idx)
		require.NotNil(t, fn)
		return fn
	}

	roundtripFn := getFunc("roundtrip")
	dropFn := getFunc("drop")

	// Create a resource with rep=99
	res, err := roundtripFn.Call(store, []ComponentVal{NewComponentValU32(99)})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, ComponentValKindResource, res[0].Kind())

	// No destructors yet
	require.Empty(t, dtorCalls)

	// Drop the resource
	_, err = dropFn.Call(store, res)
	require.NoError(t, err)

	// Destructor should have been called with rep=99
	require.Equal(t, []uint32{99}, dtorCalls)
}

// =============================================================================
// Edge case: Multiple exports with distinct types
// Inspired by: Rust func.rs + general component model multi-export patterns
//
// Existing Go tests only call a single export per component. This tests
// calling multiple exports from the same instance with different signatures.
// =============================================================================

func TestEdgeCaseMultipleExports(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "add") (param i32 i32) (result i32)
            (i32.add (local.get 0) (local.get 1)))
        (func (export "negate") (param i32) (result i32)
            (i32.sub (i32.const 0) (local.get 0)))
        (func (export "is-zero") (param i32) (result i32)
            (i32.eqz (local.get 0)))
    )
    (core instance $i (instantiate $m))
    (func (export "add") (param "a" u32) (param "b" u32) (result u32)
        (canon lift (core func $i "add")))
    (func (export "negate") (param "a" s32) (result s32)
        (canon lift (core func $i "negate")))
    (func (export "is-zero") (param "a" u32) (result bool)
        (canon lift (core func $i "is-zero")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	callFunc := func(name string, args []ComponentVal) []ComponentVal {
		idx := instance.GetExportIndex(store, nil, name)
		require.NotNil(t, idx)
		defer idx.Close()
		fn := instance.GetFunc(store, idx)
		require.NotNil(t, fn)
		results, err := fn.Call(store, args)
		require.NoError(t, err)
		require.Len(t, results, 1)
		return results
	}

	// add(10, 32) = 42
	r := callFunc("add", []ComponentVal{NewComponentValU32(10), NewComponentValU32(32)})
	require.Equal(t, uint32(42), r[0].U32())

	// negate(7) = -7
	r = callFunc("negate", []ComponentVal{NewComponentValS32(7)})
	require.Equal(t, int32(-7), r[0].S32())

	// is-zero(0) = true
	r = callFunc("is-zero", []ComponentVal{NewComponentValU32(0)})
	require.True(t, r[0].Bool())

	// is-zero(1) = false
	r = callFunc("is-zero", []ComponentVal{NewComponentValU32(1)})
	require.False(t, r[0].Bool())
}

// =============================================================================
// Edge case: Unicode string with multi-byte characters round-trip
// Inspired by: Rust strings.rs STRINGS constant and roundtrip tests
//
// Existing Go tests only pass ASCII strings. This tests various Unicode
// strings including Latin extended, Greek, CJK, and emoji.
// =============================================================================

func TestEdgeCaseUnicodeStrings(t *testing.T) {
	testStrings := []string{
		"",                         // empty
		"hello",                    // ASCII
		"à á â ã ä å æ ç è é ê ë", // Latin extended (2-byte UTF-8)
		"Ξ Ο Π Ρ Σ Τ Υ Φ Χ Ψ Ω",  // Greek (2-byte UTF-8)
		"ＳＴＵＶＷＸＹＺ",                   // Fullwidth (3-byte UTF-8)
		"\U00010000",               // Supplementary plane (4-byte UTF-8)
		"Löwe 老虎 Léopard",         // Mixed scripts
	}

	for _, s := range testStrings {
		t.Run(truncateForName(s), func(t *testing.T) {
			var hostReceived string

			ctx := newEchoTestContext(t,
				echoComponent(
					`string`,
					`(param $x i32)
(param $y i32)
(result i32)
(local $res i32)
local.get $x
local.get $y
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 8))
local.tee $res
call $do
local.get $res`,
					"(param i32 i32 i32)",
				),
				func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
					require.Len(t, args, 1)
					hostReceived = args[0].String()
					results[0] = NewComponentValString(args[0].String())
					return nil
				},
			)
			defer ctx.Close()

			arg := NewComponentValString(s)
			results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
			require.NoError(t, err)

			require.Equal(t, s, hostReceived)
			require.Len(t, results, 1)
			require.Equal(t, s, results[0].String())
		})
	}
}

// truncateForName creates a safe test name from a string.
func truncateForName(s string) string {
	if s == "" {
		return "empty"
	}
	if len(s) > 20 {
		s = s[:20]
	}
	return strings.ReplaceAll(s, " ", "_")
}

// =============================================================================
// Edge case: Variant with no payload (unit case)
// Inspired by: Rust func.rs variant tests
//
// Existing Go tests always pass variants with payloads. This tests a
// variant with a case that has no payload (unit case).
// =============================================================================

func TestEdgeCaseVariantUnitCase(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(variant (case "ok") (case "err" string))`,
			`(param $disc i32)
(param $p1 i32)
(param $p2 i32)
(result i32)
(local $res i32)
local.get $disc
local.get $p1
local.get $p2
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 12))
local.tee $res
call $do
local.get $res`,
			"(param i32 i32 i32 i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args

			// Return variant ok (no payload)
			results[0] = NewComponentValVariant("ok", nil)
			return nil
		},
	)
	defer ctx.Close()

	// Send variant err("something broke")
	errPayload := NewComponentValString("something broke")
	arg := NewComponentValVariant("err", &errPayload)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received variant err("something broke")
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindVariant, hostArgs[0].Kind())
	v := hostArgs[0].Variant()
	require.Equal(t, "err", v.Discriminant)
	require.NotNil(t, v.Val)
	require.Equal(t, "something broke", v.Val.String())

	// Verify result is variant ok (no payload)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindVariant, results[0].Kind())
	resV := results[0].Variant()
	require.Equal(t, "ok", resV.Discriminant)
	require.Nil(t, resV.Val)
}

// =============================================================================
// Edge case: Flags with many flags (near the 32-flag limit)
// Inspired by: Rust component model flags spec
//
// Existing Go tests only use 2 flags. This tests having 8 flags, setting
// non-adjacent ones, to verify sparse flag selection works.
// =============================================================================

func TestEdgeCaseManyFlags(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(flags "f0" "f1" "f2" "f3" "f4" "f5" "f6" "f7")`,
			`(param $x i32)
(result i32)
local.get $x
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			// Return flags f0, f2, f4, f6 (even-numbered)
			results[0] = NewComponentValFlags([]string{"f0", "f2", "f4", "f6"})
			return nil
		},
	)
	defer ctx.Close()

	// Set flags f1, f3, f5, f7 (odd-numbered)
	arg := NewComponentValFlags([]string{"f1", "f3", "f5", "f7"})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received odd-numbered flags
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindFlags, hostArgs[0].Kind())
	require.Equal(t, []string{"f1", "f3", "f5", "f7"}, hostArgs[0].Flags())

	// Verify result has even-numbered flags
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindFlags, results[0].Kind())
	require.Equal(t, []string{"f0", "f2", "f4", "f6"}, results[0].Flags())
}

// =============================================================================
// Edge case: Result<_, _> with both ok and err having no payload
// Inspired by: Rust component model result type edge cases
//
// Existing Go tests only exercise result<u32, u32>. This tests
// result<_, _> (no payloads on either side).
// =============================================================================

func TestEdgeCaseResultNoPayload(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(result)`,
			`(param $disc i32)
(result i32)
local.get $disc
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			// Return Err with no payload
			results[0] = NewComponentValResultErr(nil)
			return nil
		},
	)
	defer ctx.Close()

	// Send Ok with no payload
	arg := NewComponentValResultOk(nil)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received Ok
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindResult, hostArgs[0].Kind())
	r := hostArgs[0].Result()
	require.True(t, r.IsOk)
	require.Nil(t, r.Val)

	// Verify result is Err
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindResult, results[0].Kind())
	resR := results[0].Result()
	require.False(t, resR.IsOk)
	require.Nil(t, resR.Val)
}

// =============================================================================
// Edge case: Post-return with string result
// Inspired by: Rust post_return.rs `post_return_string`
//
// Tests that post-return works correctly with a string result (which
// involves memory and pointer management).
// =============================================================================

func TestEdgeCasePostReturnString(t *testing.T) {
	wat := `
(component
    (core module $m
        (memory (export "memory") 1)
        (func (export "get") (result i32)
            (i32.store offset=0 (i32.const 8) (i32.const 100))
            (i32.store offset=4 (i32.const 8) (i32.const 11))
            i32.const 8
        )
        (func (export "post") (param i32)
            local.get 0
            i32.const 8
            i32.ne
            if unreachable end)
        (data (i32.const 100) "hello world")
    )
    (core instance $i (instantiate $m))
    (func (export "get") (result string)
        (canon lift
            (core func $i "get")
            (post-return (func $i "post"))
            (memory $i "memory")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "get")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	results, err := fn.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindString, results[0].Kind())
	require.Equal(t, "hello world", results[0].String())
}

// =============================================================================
// Edge case: DefineUnknownImportsAsTraps with instance imports
// Inspired by: Rust linker.rs `linker_defines_unknown_imports_as_traps`
//
// Existing Go tests only use DefineUnknownImportsAsTraps with a single func
// import. This tests it with both a func and an instance import.
// =============================================================================

func TestEdgeCaseUnknownImportsWithInstance(t *testing.T) {
	wat := `
(component
    (import "foo" (func))
    (import "bar" (instance (export "baz" (func))))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	err = linker.DefineUnknownImportsAsTraps(component)
	require.NoError(t, err)

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

// =============================================================================
// Edge case: Record with mixed field types
// Inspired by: Rust func.rs `tuple_result` test
//
// Existing Go tests only test records with uniform field types (all u64).
// This tests a record with mixed types: s8, u16, float32, float64.
// =============================================================================

func TestEdgeCaseRecordMixedTypes(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(record (field "a" s8) (field "b" u16) (field "c" float32))`,
			`(param $a i32)
(param $b i32)
(param $c f32)
(result i32)
(local $res i32)
local.get $a
local.get $b
local.get $c
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 8))
local.tee $res
call $do
local.get $res`,
			"(param i32 i32 f32 i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			results[0] = NewComponentValRecord([]ComponentRecordField{
				{Name: "a", Val: NewComponentValS8(-1)},
				{Name: "b", Val: NewComponentValU16(1000)},
				{Name: "c", Val: NewComponentValF32(2.5)},
			})
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValRecord([]ComponentRecordField{
		{Name: "a", Val: NewComponentValS8(42)},
		{Name: "b", Val: NewComponentValU16(65535)},
		{Name: "c", Val: NewComponentValF32(1.5)},
	})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received the mixed-type record
	require.Len(t, hostArgs, 1)
	fields := hostArgs[0].Record()
	require.Len(t, fields, 3)
	require.Equal(t, "a", fields[0].Name)
	require.Equal(t, int8(42), fields[0].Val.S8())
	require.Equal(t, "b", fields[1].Name)
	require.Equal(t, uint16(65535), fields[1].Val.U16())
	require.Equal(t, "c", fields[2].Name)
	require.Equal(t, float32(1.5), fields[2].Val.F32())

	// Verify result
	require.Len(t, results, 1)
	resFields := results[0].Record()
	require.Len(t, resFields, 3)
	require.Equal(t, int8(-1), resFields[0].Val.S8())
	require.Equal(t, uint16(1000), resFields[1].Val.U16())
	require.Equal(t, float32(2.5), resFields[2].Val.F32())
}

// =============================================================================
// Edge case: Enum with more than 2 cases
// Inspired by: Rust component model enum tests
//
// Existing Go tests only use 2-case enums ("aa", "bb"). This tests a
// 4-case enum and verifies all cases round-trip.
// =============================================================================

func TestEdgeCaseEnumMultipleCases(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(enum "red" "green" "blue" "alpha")`,
			`(param $x i32)
(result i32)
local.get $x
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			results[0] = NewComponentValEnum("blue")
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValEnum("alpha")
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received "alpha" (case index 3)
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindEnum, hostArgs[0].Kind())
	require.Equal(t, "alpha", hostArgs[0].Enum())

	// Verify result is "blue" (case index 2)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindEnum, results[0].Kind())
	require.Equal(t, "blue", results[0].Enum())
}

// =============================================================================
// Edge case: Multiple calls to the same function reuse the same instance
// Inspired by: Rust func.rs general test patterns
//
// Existing Go tests typically call each function once. This verifies that
// repeated calls to the same component function work correctly and don't
// corrupt state.
// =============================================================================

func TestEdgeCaseRepeatedCalls(t *testing.T) {
	wat := `
(component
    (core module $m
        (global $counter (mut i32) (i32.const 0))
        (func (export "increment") (result i32)
            (global.set $counter (i32.add (global.get $counter) (i32.const 1)))
            global.get $counter)
    )
    (core instance $i (instantiate $m))
    (func (export "increment") (result u32)
        (canon lift (core func $i "increment")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "increment")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	// Call increment 5 times and verify counter
	for i := uint32(1); i <= 5; i++ {
		results, err := fn.Call(store, []ComponentVal{})
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, i, results[0].U32(), "call #%d", i)
	}
}

// =============================================================================
// Edge case: Host function modifying multiple results
// Inspired by: Rust func.rs multi-return tests
//
// Existing Go tests only have host functions produce a single result. This
// tests a host function that produces no results (void) and is still callable.
// =============================================================================

func TestEdgeCaseHostFuncNoResult(t *testing.T) {
	var called bool

	wat := `
(component
    (import "log" (func $log (param "msg" string)))
    (core module $libc
        (memory (export "memory") 1)
` + reallocAndFree + `
    )
    (core instance $libc (instantiate $libc))
    (core func $log_lower (canon lower (func $log) (memory $libc "memory") (realloc (func $libc "realloc"))))
    (core module $m
        (import "" "log" (func $log (param i32 i32)))
        (import "libc" "memory" (memory 1))
        (func (export "run")
            i32.const 0
            i32.const 5
            call $log)
        (data (i32.const 0) "hello")
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "log" (func $log_lower))))
        (with "libc" (instance $libc))))
    (func (export "run") (canon lift (core func $i "run")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFunc("log", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			require.Equal(t, "hello", args[0].String())
			called = true
			return nil
		})
		require.NoError(t, err)
	})

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "run")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	results, err := fn.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Nil(t, results)
	require.True(t, called, "host log function was not called")
}

// =============================================================================
// Edge case: Host callback panics
// Reviewer GAP: goComponentFuncCallback has defer/recover() to catch panics
// crossing the CGo boundary, but no test exercises this path.
//
// This test verifies that a panic in a host callback is caught and converted
// to a wasmtime error rather than crashing the process.
// =============================================================================

func TestEdgeCaseHostCallbackPanic(t *testing.T) {
	wat := `
(component
    (import "boom" (func $boom))
    (core func $boom_lower (canon lower (func $boom)))
    (core module $m
        (import "" "boom" (func $boom))
        (func (export "call") call $boom)
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "boom" (func $boom_lower))))))
    (func (export "call") (canon lift (core func $i "call")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFunc("boom", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			panic("intentional panic from host callback")
		})
		require.NoError(t, err)
	})

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "call")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	// The panic should be caught by defer/recover in goComponentFuncCallback
	// and converted to an error, NOT crash the process
	_, err = fn.Call(store, []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "intentional panic from host callback")
}

// =============================================================================
// Edge case: Wrong number of arguments passed to component function
// Reviewer GAP: No test passes wrong-type or wrong-count args to verify
// the bindings don't crash on type mismatches.
//
// This verifies that calling a function with the wrong number of args
// produces an error rather than a segfault or undefined behavior.
// =============================================================================

func TestEdgeCaseWrongArgCount(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "add") (param i32 i32) (result i32)
            local.get 0
            local.get 1
            i32.add)
    )
    (core instance $i (instantiate $m))
    (func (export "add") (param "a" u32) (param "b" u32) (result u32)
        (canon lift (core func $i "add")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	linker := NewComponentLinker(engine)
	defer linker.Close()

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	idx := instance.GetExportIndex(store, nil, "add")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	// Too few args — expects 2, pass 1
	_, err = fn.Call(store, []ComponentVal{NewComponentValU32(1)})
	require.Error(t, err, "should error on too few args")

	// Too many args — expects 2, pass 3
	_, err = fn.Call(store, []ComponentVal{
		NewComponentValU32(1),
		NewComponentValU32(2),
		NewComponentValU32(3),
	})
	require.Error(t, err, "should error on too many args")

	// Correct args still work
	results, err := fn.Call(store, []ComponentVal{
		NewComponentValU32(10),
		NewComponentValU32(20),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, uint32(30), results[0].U32())
}

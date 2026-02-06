package wasmtime

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test fixtures extracted from the Rust wasmtime test harness:
//   - tests/all/component_model/func.rs   (thunks, integers, floats, bools, chars)
//   - tests/all/component_model/post_return.rs  (post-return semantics)
//   - tests/all/component_model/nested.rs  (nested instantiations with host callbacks)
//   - tests/all/component_model/resources.rs (resource lifecycle)
//   - tests/all/cli_tests/ (standalone WAT files)
// =============================================================================

// --- helpers -----------------------------------------------------------------

// fixtureCallFunc compiles a self-contained component WAT, instantiates it, and
// returns a helper to call exported functions by name.
type fixtureInstance struct {
	t        *testing.T
	engine   *Engine
	store    *Store
	instance *ComponentInstance
	linker   *ComponentLinker
}

func newFixtureInstance(t *testing.T, wat string) *fixtureInstance {
	t.Helper()
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	return &fixtureInstance{t: t, engine: engine, store: store, instance: instance, linker: linker}
}

func newFixtureInstanceWithLinker(t *testing.T, wat string, setup func(*ComponentLinker)) *fixtureInstance {
	t.Helper()
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	setup(linker)

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	return &fixtureInstance{t: t, engine: engine, store: store, instance: instance, linker: linker}
}

func (fi *fixtureInstance) Close() {
	fi.linker.Close()
}

func (fi *fixtureInstance) getFunc(name string) *ComponentFunc {
	fi.t.Helper()
	idx := fi.instance.GetExportIndex(fi.store, nil, name)
	require.NotNil(fi.t, idx, "expected export %q", name)
	defer idx.Close()
	fn := fi.instance.GetFunc(fi.store, idx)
	require.NotNil(fi.t, fn, "expected func for %q", name)
	return fn
}

func (fi *fixtureInstance) call(name string, args []ComponentVal) ([]ComponentVal, error) {
	fi.t.Helper()
	fn := fi.getFunc(name)
	return fn.Call(fi.store, args)
}

func (fi *fixtureInstance) mustCall(name string, args []ComponentVal) []ComponentVal {
	fi.t.Helper()
	results, err := fi.call(name, args)
	require.NoError(fi.t, err)
	return results
}

// =============================================================================
// From func.rs: thunks()
// Tests basic no-arg/no-result functions and trap handling.
// =============================================================================

func TestFixtureThunks(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "thunk"))
        (func (export "thunk-trap") unreachable)
    )
    (core instance $i (instantiate $m))
    (func (export "thunk")
        (canon lift (core func $i "thunk"))
    )
    (func (export "thunk-trap")
        (canon lift (core func $i "thunk-trap"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// Successful thunk: no args, no results
	results := fi.mustCall("thunk", []ComponentVal{})
	require.Nil(t, results)

	// Trapping thunk: should return an error
	_, err := fi.call("thunk-trap", []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unreachable")
}

// =============================================================================
// From func.rs: integers()
// Tests all integer types: take-* validates input, ret-* returns constants,
// retm1-* tests -1 truncation, retbig-* tests 100000 truncation.
// =============================================================================

func TestFixtureIntegers(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "take-i32-100") (param i32)
            local.get 0
            i32.const 100
            i32.eq
            br_if 0
            unreachable
        )
        (func (export "take-i64-100") (param i64)
            local.get 0
            i64.const 100
            i64.eq
            br_if 0
            unreachable
        )
        (func (export "ret-i32-0") (result i32) i32.const 0)
        (func (export "ret-i64-0") (result i64) i64.const 0)
        (func (export "ret-i32-minus-1") (result i32) i32.const -1)
        (func (export "ret-i64-minus-1") (result i64) i64.const -1)
        (func (export "ret-i32-100000") (result i32) i32.const 100000)
    )
    (core instance $i (instantiate $m))
    (func (export "take-u8") (param "a" u8) (canon lift (core func $i "take-i32-100")))
    (func (export "take-s8") (param "a" s8) (canon lift (core func $i "take-i32-100")))
    (func (export "take-u16") (param "a" u16) (canon lift (core func $i "take-i32-100")))
    (func (export "take-s16") (param "a" s16) (canon lift (core func $i "take-i32-100")))
    (func (export "take-u32") (param "a" u32) (canon lift (core func $i "take-i32-100")))
    (func (export "take-s32") (param "a" s32) (canon lift (core func $i "take-i32-100")))
    (func (export "take-u64") (param "a" u64) (canon lift (core func $i "take-i64-100")))
    (func (export "take-s64") (param "a" s64) (canon lift (core func $i "take-i64-100")))

    (func (export "ret-u8") (result u8) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-s8") (result s8) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-u16") (result u16) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-s16") (result s16) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-u32") (result u32) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-s32") (result s32) (canon lift (core func $i "ret-i32-0")))
    (func (export "ret-u64") (result u64) (canon lift (core func $i "ret-i64-0")))
    (func (export "ret-s64") (result s64) (canon lift (core func $i "ret-i64-0")))

    (func (export "retm1-u8") (result u8) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-s8") (result s8) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-u16") (result u16) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-s16") (result s16) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-u32") (result u32) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-s32") (result s32) (canon lift (core func $i "ret-i32-minus-1")))
    (func (export "retm1-u64") (result u64) (canon lift (core func $i "ret-i64-minus-1")))
    (func (export "retm1-s64") (result s64) (canon lift (core func $i "ret-i64-minus-1")))

    (func (export "retbig-u8") (result u8) (canon lift (core func $i "ret-i32-100000")))
    (func (export "retbig-s8") (result s8) (canon lift (core func $i "ret-i32-100000")))
    (func (export "retbig-u16") (result u16) (canon lift (core func $i "ret-i32-100000")))
    (func (export "retbig-s16") (result s16) (canon lift (core func $i "ret-i32-100000")))
    (func (export "retbig-u32") (result u32) (canon lift (core func $i "ret-i32-100000")))
    (func (export "retbig-s32") (result s32) (canon lift (core func $i "ret-i32-100000")))
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// Passing 100 should succeed for all integer types
	t.Run("take-100", func(t *testing.T) {
		fi.mustCall("take-u8", []ComponentVal{NewComponentValU8(100)})
		fi.mustCall("take-s8", []ComponentVal{NewComponentValS8(100)})
		fi.mustCall("take-u16", []ComponentVal{NewComponentValU16(100)})
		fi.mustCall("take-s16", []ComponentVal{NewComponentValS16(100)})
		fi.mustCall("take-u32", []ComponentVal{NewComponentValU32(100)})
		fi.mustCall("take-s32", []ComponentVal{NewComponentValS32(100)})
		fi.mustCall("take-u64", []ComponentVal{NewComponentValU64(100)})
		fi.mustCall("take-s64", []ComponentVal{NewComponentValS64(100)})
	})

	// Returning 0 should produce zero for all types
	t.Run("ret-zero", func(t *testing.T) {
		require.Equal(t, uint8(0), fi.mustCall("ret-u8", []ComponentVal{})[0].U8())
		require.Equal(t, int8(0), fi.mustCall("ret-s8", []ComponentVal{})[0].S8())
		require.Equal(t, uint16(0), fi.mustCall("ret-u16", []ComponentVal{})[0].U16())
		require.Equal(t, int16(0), fi.mustCall("ret-s16", []ComponentVal{})[0].S16())
		require.Equal(t, uint32(0), fi.mustCall("ret-u32", []ComponentVal{})[0].U32())
		require.Equal(t, int32(0), fi.mustCall("ret-s32", []ComponentVal{})[0].S32())
		require.Equal(t, uint64(0), fi.mustCall("ret-u64", []ComponentVal{})[0].U64())
		require.Equal(t, int64(0), fi.mustCall("ret-s64", []ComponentVal{})[0].S64())
	})

	// Returning -1 (all bits set) should reinterpret bytes per type.
	// The component model truncates the value to the type's width.
	t.Run("ret-minus-1", func(t *testing.T) {
		require.Equal(t, uint8(0xff), fi.mustCall("retm1-u8", []ComponentVal{})[0].U8())
		require.Equal(t, int8(-1), fi.mustCall("retm1-s8", []ComponentVal{})[0].S8())
		require.Equal(t, uint16(0xffff), fi.mustCall("retm1-u16", []ComponentVal{})[0].U16())
		require.Equal(t, int16(-1), fi.mustCall("retm1-s16", []ComponentVal{})[0].S16())
		require.Equal(t, uint32(0xffffffff), fi.mustCall("retm1-u32", []ComponentVal{})[0].U32())
		require.Equal(t, int32(-1), fi.mustCall("retm1-s32", []ComponentVal{})[0].S32())
		require.Equal(t, uint64(0xffffffff_ffffffff), fi.mustCall("retm1-u64", []ComponentVal{})[0].U64())
		require.Equal(t, int64(-1), fi.mustCall("retm1-s64", []ComponentVal{})[0].S64())
	})

	// Returning 100000 should truncate to the type's width.
	// 100000 = 0x186A0:
	//   u8: 0xA0 = 160, s8: -96
	//   u16: 0x86A0 = 34464, s16: -31072
	//   u32/s32: 100000
	t.Run("ret-big", func(t *testing.T) {
		require.Equal(t, uint8(0xA0), fi.mustCall("retbig-u8", []ComponentVal{})[0].U8())
		require.Equal(t, int8(-96), fi.mustCall("retbig-s8", []ComponentVal{})[0].S8())
		require.Equal(t, uint16(0x86A0), fi.mustCall("retbig-u16", []ComponentVal{})[0].U16())
		require.Equal(t, int16(-31072), fi.mustCall("retbig-s16", []ComponentVal{})[0].S16())
		require.Equal(t, uint32(100000), fi.mustCall("retbig-u32", []ComponentVal{})[0].U32())
		require.Equal(t, int32(100000), fi.mustCall("retbig-s32", []ComponentVal{})[0].S32())
	})
}

// =============================================================================
// From func.rs: floats()
// Tests float32/float64 parameter passing and bit-level reinterpretation.
// =============================================================================

func TestFixtureFloats(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "i32.reinterpret_f32") (param f32) (result i32)
            local.get 0
            i32.reinterpret_f32
        )
        (func (export "i64.reinterpret_f64") (param f64) (result i64)
            local.get 0
            i64.reinterpret_f64
        )
        (func (export "f32.reinterpret_i32") (param i32) (result f32)
            local.get 0
            f32.reinterpret_i32
        )
        (func (export "f64.reinterpret_i64") (param i64) (result f64)
            local.get 0
            f64.reinterpret_i64
        )
    )
    (core instance $i (instantiate $m))

    (func (export "f32-to-u32") (param "a" float32) (result u32)
        (canon lift (core func $i "i32.reinterpret_f32"))
    )
    (func (export "f64-to-u64") (param "a" float64) (result u64)
        (canon lift (core func $i "i64.reinterpret_f64"))
    )
    (func (export "u32-to-f32") (param "a" u32) (result float32)
        (canon lift (core func $i "f32.reinterpret_i32"))
    )
    (func (export "u64-to-f64") (param "a" u64) (result float64)
        (canon lift (core func $i "f64.reinterpret_i64"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// Float-to-int reinterpretation
	t.Run("f32-to-u32", func(t *testing.T) {
		res := fi.mustCall("f32-to-u32", []ComponentVal{NewComponentValF32(1.0)})
		require.Equal(t, math.Float32bits(1.0), res[0].U32())
	})
	t.Run("f64-to-u64", func(t *testing.T) {
		res := fi.mustCall("f64-to-u64", []ComponentVal{NewComponentValF64(2.0)})
		require.Equal(t, math.Float64bits(2.0), res[0].U64())
	})

	// Int-to-float reinterpretation
	t.Run("u32-to-f32", func(t *testing.T) {
		res := fi.mustCall("u32-to-f32", []ComponentVal{NewComponentValU32(math.Float32bits(3.0))})
		require.Equal(t, float32(3.0), res[0].F32())
	})
	t.Run("u64-to-f64", func(t *testing.T) {
		res := fi.mustCall("u64-to-f64", []ComponentVal{NewComponentValU64(math.Float64bits(4.0))})
		require.Equal(t, float64(4.0), res[0].F64())
	})

	// NaN passthrough: component model preserves NaN payloads for float32/float64
	t.Run("nan-f32", func(t *testing.T) {
		canon32NaN := uint32(0b01111111110000000000000000000000)
		nanBits := canon32NaN | 1
		res := fi.mustCall("u32-to-f32", []ComponentVal{NewComponentValU32(nanBits)})
		require.Equal(t, nanBits, math.Float32bits(res[0].F32()))
	})
	t.Run("nan-f64", func(t *testing.T) {
		canon64NaN := uint64(0b0111111111111000000000000000000000000000000000000000000000000000)
		nanBits := canon64NaN | 1
		res := fi.mustCall("u64-to-f64", []ComponentVal{NewComponentValU64(nanBits)})
		require.Equal(t, nanBits, math.Float64bits(res[0].F64()))
	})
}

// =============================================================================
// From func.rs: bools()
// Tests bool parameter/result handling, including non-0/1 u32 -> bool coercion.
// =============================================================================

func TestFixtureBools(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "pass") (param i32) (result i32) local.get 0)
    )
    (core instance $i (instantiate $m))

    (func (export "u32-to-bool") (param "a" u32) (result bool)
        (canon lift (core func $i "pass"))
    )
    (func (export "bool-to-u32") (param "a" bool) (result u32)
        (canon lift (core func $i "pass"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// bool -> u32: false=0, true=1
	res := fi.mustCall("bool-to-u32", []ComponentVal{NewComponentValBool(false)})
	require.Equal(t, uint32(0), res[0].U32())

	res = fi.mustCall("bool-to-u32", []ComponentVal{NewComponentValBool(true)})
	require.Equal(t, uint32(1), res[0].U32())

	// u32 -> bool: 0=false, 1=true, 2=true (any non-zero is true)
	res = fi.mustCall("u32-to-bool", []ComponentVal{NewComponentValU32(0)})
	require.Equal(t, false, res[0].Bool())

	res = fi.mustCall("u32-to-bool", []ComponentVal{NewComponentValU32(1)})
	require.Equal(t, true, res[0].Bool())

	res = fi.mustCall("u32-to-bool", []ComponentVal{NewComponentValU32(2)})
	require.Equal(t, true, res[0].Bool())
}

// =============================================================================
// From func.rs: chars()
// Tests char (Unicode scalar value) roundtripping.
// =============================================================================

func TestFixtureChars(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "pass") (param i32) (result i32) local.get 0)
    )
    (core instance $i (instantiate $m))

    (func (export "u32-to-char") (param "a" u32) (result char)
        (canon lift (core func $i "pass"))
    )
    (func (export "char-to-u32") (param "a" char) (result u32)
        (canon lift (core func $i "pass"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	roundtrip := func(c rune) {
		t.Helper()
		// char -> u32
		res := fi.mustCall("char-to-u32", []ComponentVal{NewComponentValChar(c)})
		require.Equal(t, uint32(c), res[0].U32())
		// u32 -> char
		res = fi.mustCall("u32-to-char", []ComponentVal{NewComponentValU32(uint32(c))})
		require.Equal(t, c, res[0].Char())
	}

	roundtrip('x')
	roundtrip('a')
	roundtrip('\x00')
	roundtrip('\n')
	roundtrip('\U0001F49D') // 💝

	// Invalid surrogate half should fail
	_, err := fi.call("u32-to-char", []ComponentVal{NewComponentValU32(0xd800)})
	require.Error(t, err)

	// Code point beyond Unicode max should fail
	_, err = fi.call("u32-to-char", []ComponentVal{NewComponentValU32(0x110000)})
	require.Error(t, err)
}

// =============================================================================
// From post_return.rs: post_return_all_types()
// Tests that post-return hooks are invoked after reading results.
// Our Go Call() automatically invokes post_return, so if the post-return
// function traps (verifies wrong value), the whole call would fail.
// A successful call here proves the post-return hook ran and validated OK.
// =============================================================================

func TestFixturePostReturnAllTypes(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "i32") (result i32)
            i32.const 1)
        (func (export "i64") (result i64)
            i64.const 2)
        (func (export "f32") (result f32)
            f32.const 3)
        (func (export "f64") (result f64)
            f64.const 4)

        (func (export "post-i32") (param i32)
            local.get 0
            i32.const 1
            i32.ne
            if unreachable end)
        (func (export "post-i64") (param i64)
            local.get 0
            i64.const 2
            i64.ne
            if unreachable end)
        (func (export "post-f32") (param f32)
            local.get 0
            f32.const 3
            f32.ne
            if unreachable end)
        (func (export "post-f64") (param f64)
            local.get 0
            f64.const 4
            f64.ne
            if unreachable end)
    )
    (core instance $i (instantiate $m))
    (func (export "i32") (result u32)
        (canon lift (core func $i "i32") (post-return (func $i "post-i32")))
    )
    (func (export "i64") (result u64)
        (canon lift (core func $i "i64") (post-return (func $i "post-i64")))
    )
    (func (export "f32") (result float32)
        (canon lift (core func $i "f32") (post-return (func $i "post-f32")))
    )
    (func (export "f64") (result float64)
        (canon lift (core func $i "f64") (post-return (func $i "post-f64")))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// Each call succeeds only if the post-return hook validates the return value.
	res := fi.mustCall("i32", []ComponentVal{})
	require.Equal(t, uint32(1), res[0].U32())

	res = fi.mustCall("i64", []ComponentVal{})
	require.Equal(t, uint64(2), res[0].U64())

	res = fi.mustCall("f32", []ComponentVal{})
	require.Equal(t, float32(3.0), res[0].F32())

	res = fi.mustCall("f64", []ComponentVal{})
	require.Equal(t, float64(4.0), res[0].F64())
}

// =============================================================================
// From post_return.rs: post_return_string()
// Tests post-return with a string return value that reads from linear memory.
// =============================================================================

func TestFixturePostReturnString(t *testing.T) {
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
            (memory $i "memory")
        )
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	res := fi.mustCall("get", []ComponentVal{})
	require.Len(t, res, 1)
	require.Equal(t, ComponentValKindString, res[0].Kind())
	require.Equal(t, "hello world", res[0].String())
}

// =============================================================================
// From nested.rs: nested_many_instantiations()
// Tests recursive nested component instantiation with a host callback that
// counts invocations. The component tree calls the host 16 times total
// (binary tree of depth 4, each leaf calls twice).
// =============================================================================

func TestFixtureNestedManyInstantiations(t *testing.T) {
	wat := `
(component
  (import "count" (func $count))
  (component $c1
    (import "count" (func $count))
    (core func $count_lower (canon lower (func $count)))
    (core module $m
        (import "" "" (func $count))
        (start $count)
    )
    (core instance (instantiate $m (with "" (instance (export "" (func $count_lower))))))
    (core instance (instantiate $m (with "" (instance (export "" (func $count_lower))))))
  )
  (component $c2
    (import "count" (func $count))
    (instance (instantiate $c1 (with "count" (func $count))))
    (instance (instantiate $c1 (with "count" (func $count))))
  )
  (component $c3
    (import "count" (func $count))
    (instance (instantiate $c2 (with "count" (func $count))))
    (instance (instantiate $c2 (with "count" (func $count))))
  )
  (component $c4
    (import "count" (func $count))
    (instance (instantiate $c3 (with "count" (func $count))))
    (instance (instantiate $c3 (with "count" (func $count))))
  )

  (instance (instantiate $c4 (with "count" (func $count))))
)
`
	callCount := 0

	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFunc("count", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			callCount++
			return nil
		})
		require.NoError(t, err)
	})

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)

	// The component tree calls "count" exactly 16 times during instantiation:
	// c1 calls twice, c2 instantiates c1 twice (4), c3 instantiates c2 twice (8),
	// c4 instantiates c3 twice (16).
	require.Equal(t, 16, callCount)
}

// =============================================================================
// From func.rs: multi-value returns
// Component with a function that returns multiple values via a record.
// =============================================================================

func TestFixtureMultipleReturnValues(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "swap") (param i32 i32) (result i32)
            (local $ret i32)
            (local.set $ret
                (call $realloc
                    (i32.const 0) (i32.const 0) (i32.const 4) (i32.const 8)))
            (i32.store offset=0 (local.get $ret) (local.get 1))
            (i32.store offset=4 (local.get $ret) (local.get 0))
            local.get $ret
        )
        (memory (export "memory") 1)
` + reallocAndFree + `
    )
    (core instance $i (instantiate $m))
    (type $pair (record (field "a" u32) (field "b" u32)))
    (export $Pair "pair" (type $pair))
    (func (export "swap") (param "x" u32) (param "y" u32) (result $Pair)
        (canon lift (core func $i "swap") (memory $i "memory") (realloc (func $i "realloc")))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	res := fi.mustCall("swap", []ComponentVal{
		NewComponentValU32(10),
		NewComponentValU32(20),
	})
	require.Len(t, res, 1)
	require.Equal(t, ComponentValKindRecord, res[0].Kind())
	fields := res[0].Record()
	require.Len(t, fields, 2)
	require.Equal(t, "a", fields[0].Name)
	require.Equal(t, uint32(20), fields[0].Val.U32())
	require.Equal(t, "b", fields[1].Name)
	require.Equal(t, uint32(10), fields[1].Val.U32())
}

// =============================================================================
// Self-contained echo component for complex types.
// This component takes a value, passes it to the host "do" function,
// and returns the host's result. Tests the full round-trip of complex types.
// =============================================================================

// TestFixtureEchoString tests string round-trip through host callback with
// a non-ASCII string to exercise UTF-8 handling.
func TestFixtureEchoStringUTF8(t *testing.T) {
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
			hostReceived = args[0].String()
			results[0] = NewComponentValString("こんにちは世界")
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValString("Héllo wörld 🌍")
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	require.Equal(t, "Héllo wörld 🌍", hostReceived)
	require.Equal(t, "こんにちは世界", results[0].String())
}

// TestFixtureEchoEmptyList tests that an empty list round-trips correctly.
func TestFixtureEchoEmptyList(t *testing.T) {
	var hostReceived []ComponentVal

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
			hostReceived = args[0].List()
			results[0] = NewComponentValList([]ComponentVal{})
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValList([]ComponentVal{})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	require.Len(t, hostReceived, 0)
	require.Equal(t, ComponentValKindList, results[0].Kind())
	require.Len(t, results[0].List(), 0)
}

// TestFixtureRecordTransform tests a 3-field record round-trip through the component
// model. The component swaps fields a/b and multiplies c by 10.
func TestFixtureRecordTransform(t *testing.T) {
	wat := `
(component
	(core module $m
		(memory (export "memory") 1)
` + reallocAndFree + `
		(func (export "transform") (param $a i32) (param $b i32) (param $c i32) (result i32)
			(local $ret i32)
			(local.set $ret
				(call $realloc (i32.const 0) (i32.const 0) (i32.const 4) (i32.const 12)))
			(i32.store offset=0 (local.get $ret) (local.get $b))
			(i32.store offset=4 (local.get $ret) (local.get $a))
			(i32.store offset=8 (local.get $ret) (i32.mul (local.get $c) (i32.const 10)))
			local.get $ret
		)
	)
	(core instance $i (instantiate $m))

	(type $Rec' (record (field "a" u32) (field "b" u32) (field "c" u32)))
	(export $Rec "rec" (type $Rec'))

	(func (export "transform") (param "x" $Rec) (result $Rec)
		(canon lift (core func $i "transform") (memory $i "memory") (realloc (func $i "realloc")))
	)
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	arg := NewComponentValRecord([]ComponentRecordField{
		{Name: "a", Val: NewComponentValU32(1)},
		{Name: "b", Val: NewComponentValU32(2)},
		{Name: "c", Val: NewComponentValU32(3)},
	})

	results := fi.mustCall("transform", []ComponentVal{arg})
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindRecord, results[0].Kind())

	fields := results[0].Record()
	require.Len(t, fields, 3)
	require.Equal(t, "a", fields[0].Name)
	require.Equal(t, uint32(2), fields[0].Val.U32()) // was b=2
	require.Equal(t, "b", fields[1].Name)
	require.Equal(t, uint32(1), fields[1].Val.U32()) // was a=1
	require.Equal(t, "c", fields[2].Name)
	require.Equal(t, uint32(30), fields[2].Val.U32()) // 3 * 10
}

// TestFixtureEchoOptionNone tests option with None in both directions.
func TestFixtureEchoOptionNone(t *testing.T) {
	ctx := newEchoTestContext(t,
		echoComponent(
			`(option u32)`,
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
			// Host receives None, returns None
			require.Nil(t, args[0].Option())
			results[0] = NewComponentValOptionNone()
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValOptionNone()
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)
	require.Nil(t, results[0].Option())
}

// TestFixtureEchoResultErrToOk tests result type converting from Err to Ok.
func TestFixtureEchoResultErrToOk(t *testing.T) {
	ctx := newEchoTestContext(t,
		echoComponent(
			`(result u32 (error u32))`,
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
			// Host receives Err(999), converts to Ok(42)
			r := args[0].Result()
			require.False(t, r.IsOk)
			require.Equal(t, uint32(999), r.Val.U32())
			okVal := NewComponentValU32(42)
			results[0] = NewComponentValResultOk(&okVal)
			return nil
		},
	)
	defer ctx.Close()

	errVal := NewComponentValU32(999)
	arg := NewComponentValResultErr(&errVal)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	r := results[0].Result()
	require.True(t, r.IsOk)
	require.Equal(t, uint32(42), r.Val.U32())
}

// =============================================================================
// From resources.rs: guest resource with multiple simultaneous handles.
// Tests creating, borrowing, and dropping multiple resource handles.
// =============================================================================

func TestFixtureGuestResourceMultipleHandles(t *testing.T) {
	wat := `
(component
  (core module $a
    (global $g (mut i32) (i32.const 0))
    (global $count (mut i32) (i32.const 0))
    (func (export "dtor") (param i32)
        (global.set $g (local.get 0))
        (global.set $count (i32.add (global.get $count) (i32.const 1)))
    )
    (func (export "last-dtor-rep") (result i32) global.get $g)
    (func (export "dtor-count") (result i32) global.get $count)
  )
  (core instance $a (instantiate $a))
  (type $r' (resource (rep i32) (dtor (func $a "dtor"))))
  (export $r "r" (type $r'))

  (core func $new (canon resource.new $r))
  (core func $drop (canon resource.drop $r))

  (core module $b
    (import "" "new" (func $new (param i32) (result i32)))
    (import "" "drop" (func $drop (param i32)))

    (func (export "new") (param i32) (result i32) local.get 0 call $new)
    (func (export "rep") (param i32) (result i32) local.get 0)
    (func (export "drop") (param i32) local.get 0 call $drop)
  )
  (core instance $b (instantiate $b
    (with "" (instance
      (export "new" (func $new))
      (export "drop" (func $drop))
    ))
  ))

  (func (export "new") (param "x" u32) (result (own $r))
    (canon lift (core func $b "new")))
  (func (export "rep") (param "x" (borrow $r)) (result u32)
    (canon lift (core func $b "rep")))
  (func (export "drop") (param "x" (own $r))
    (canon lift (core func $b "drop")))
  (func (export "last-dtor-rep") (result u32)
    (canon lift (core func $a "last-dtor-rep")))
  (func (export "dtor-count") (result u32)
    (canon lift (core func $a "dtor-count")))
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	// Create three resources
	res1 := fi.mustCall("new", []ComponentVal{NewComponentValU32(10)})
	res2 := fi.mustCall("new", []ComponentVal{NewComponentValU32(20)})
	res3 := fi.mustCall("new", []ComponentVal{NewComponentValU32(30)})

	// Verify reps via borrow
	rep1 := fi.mustCall("rep", []ComponentVal{res1[0]})
	require.Equal(t, uint32(10), rep1[0].U32())
	rep2 := fi.mustCall("rep", []ComponentVal{res2[0]})
	require.Equal(t, uint32(20), rep2[0].U32())
	rep3 := fi.mustCall("rep", []ComponentVal{res3[0]})
	require.Equal(t, uint32(30), rep3[0].U32())

	// No destructors called yet
	dtorCount := fi.mustCall("dtor-count", []ComponentVal{})
	require.Equal(t, uint32(0), dtorCount[0].U32())

	// Drop resource 2
	fi.mustCall("drop", []ComponentVal{res2[0]})
	lastDtor := fi.mustCall("last-dtor-rep", []ComponentVal{})
	require.Equal(t, uint32(20), lastDtor[0].U32())
	dtorCount = fi.mustCall("dtor-count", []ComponentVal{})
	require.Equal(t, uint32(1), dtorCount[0].U32())

	// Drop resource 1
	fi.mustCall("drop", []ComponentVal{res1[0]})
	lastDtor = fi.mustCall("last-dtor-rep", []ComponentVal{})
	require.Equal(t, uint32(10), lastDtor[0].U32())
	dtorCount = fi.mustCall("dtor-count", []ComponentVal{})
	require.Equal(t, uint32(2), dtorCount[0].U32())

	// Drop resource 3
	fi.mustCall("drop", []ComponentVal{res3[0]})
	lastDtor = fi.mustCall("last-dtor-rep", []ComponentVal{})
	require.Equal(t, uint32(30), lastDtor[0].U32())
	dtorCount = fi.mustCall("dtor-count", []ComponentVal{})
	require.Equal(t, uint32(3), dtorCount[0].U32())
}

// =============================================================================
// Component with host-provided function that returns an error.
// Tests that host callback errors propagate correctly.
// =============================================================================

func TestFixtureHostCallbackError(t *testing.T) {
	wat := `
(component
    (import "fail" (func $fail))
    (core func $fail_lower (canon lower (func $fail)))
    (core module $m
        (import "" "" (func $fail))
        (func (export "call") (call $fail))
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "" (func $fail_lower))))
    ))
    (func (export "call")
        (canon lift (core func $i "call"))
    )
)
`
	fi := newFixtureInstanceWithLinker(t, wat, func(linker *ComponentLinker) {
		linker.Root(func(root *ComponentLinkerInstance) {
			err := root.AddFunc("fail", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
				return fmt.Errorf("intentional host error")
			})
			require.NoError(t, err)
		})
	})
	defer fi.Close()

	_, err := fi.call("call", []ComponentVal{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "intentional host error")
}

// =============================================================================
// From cli_tests: component-basic.wat
// A basic component with a WASI-style result return value.
// =============================================================================

func TestFixtureComponentBasicResultType(t *testing.T) {
	wat := `
(component
  (core module $m
    (func (export "run") (result i32)
      i32.const 0)
  )
  (core instance $i (instantiate $m))
  (func $run (result (result))
    (canon lift (core func $i "run")))
  (export "run" (func $run))
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	res := fi.mustCall("run", []ComponentVal{})
	require.Len(t, res, 1)
	require.Equal(t, ComponentValKindResult, res[0].Kind())
	r := res[0].Result()
	require.True(t, r.IsOk)
	// "result" with no ok/err payloads has nil Val
	require.Nil(t, r.Val)
}

// =============================================================================
// Tests multi-call: calling the same function multiple times on one instance.
// Ensures the instance doesn't get stuck in a "called" state.
// =============================================================================

func TestFixtureMultipleCallsSameFunc(t *testing.T) {
	wat := `
(component
    (core module $m
        (global $counter (mut i32) (i32.const 0))
        (func (export "inc") (result i32)
            (global.set $counter (i32.add (global.get $counter) (i32.const 1)))
            global.get $counter
        )
    )
    (core instance $i (instantiate $m))
    (func (export "inc") (result u32)
        (canon lift (core func $i "inc"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	for i := uint32(1); i <= 5; i++ {
		res := fi.mustCall("inc", []ComponentVal{})
		require.Equal(t, i, res[0].U32())
	}
}

// =============================================================================
// Tests unknown imports as traps: defining missing imports as trap stubs,
// then verifying the stub actually traps when called.
// =============================================================================

func TestFixtureUnknownImportsAsTrapsCalled(t *testing.T) {
	wat := `
(component
    (import "unknown-func" (func $uf))
    (core func $uf_lower (canon lower (func $uf)))
    (core module $m
        (import "" "" (func $uf))
        (func (export "call") (call $uf))
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "" (func $uf_lower))))
    ))
    (func (export "call")
        (canon lift (core func $i "call"))
    )
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

	// Without defining the import, instantiation fails
	_, err = linker.Instantiate(store, component)
	require.Error(t, err)

	// Define unknown imports as traps
	err = linker.DefineUnknownImportsAsTraps(component)
	require.NoError(t, err)

	// Now instantiation succeeds
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// But calling the trapped function should fail with a trap
	idx := instance.GetExportIndex(store, nil, "call")
	require.NotNil(t, idx)
	defer idx.Close()
	fn := instance.GetFunc(store, idx)
	require.NotNil(t, fn)

	_, err = fn.Call(store, []ComponentVal{})
	require.Error(t, err)
}

// =============================================================================
// Tests the add-two-u32 function (already in component_val_test.go as
// TestComponentCallFunc), but here we add subtraction, multiply, and
// boundary values to validate arithmetic.
// =============================================================================

func TestFixtureArithmetic(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "add") (param i32 i32) (result i32)
            local.get 0 local.get 1 i32.add)
        (func (export "sub") (param i32 i32) (result i32)
            local.get 0 local.get 1 i32.sub)
        (func (export "mul") (param i32 i32) (result i32)
            local.get 0 local.get 1 i32.mul)
    )
    (core instance $i (instantiate $m))
    (func (export "add") (param "x" u32) (param "y" u32) (result u32) (canon lift (core func $i "add")))
    (func (export "sub") (param "x" u32) (param "y" u32) (result u32) (canon lift (core func $i "sub")))
    (func (export "mul") (param "x" u32) (param "y" u32) (result u32) (canon lift (core func $i "mul")))
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	t.Run("add", func(t *testing.T) {
		res := fi.mustCall("add", []ComponentVal{NewComponentValU32(10), NewComponentValU32(20)})
		require.Equal(t, uint32(30), res[0].U32())
	})
	t.Run("sub", func(t *testing.T) {
		res := fi.mustCall("sub", []ComponentVal{NewComponentValU32(100), NewComponentValU32(42)})
		require.Equal(t, uint32(58), res[0].U32())
	})
	t.Run("mul", func(t *testing.T) {
		res := fi.mustCall("mul", []ComponentVal{NewComponentValU32(7), NewComponentValU32(6)})
		require.Equal(t, uint32(42), res[0].U32())
	})
	t.Run("overflow", func(t *testing.T) {
		// u32 overflow wraps: 0xFFFFFFFF + 1 = 0
		res := fi.mustCall("add", []ComponentVal{NewComponentValU32(0xFFFFFFFF), NewComponentValU32(1)})
		require.Equal(t, uint32(0), res[0].U32())
	})
}

// =============================================================================
// Tests host function with namespaced import (nested instance).
// =============================================================================

func TestFixtureNamespacedImport(t *testing.T) {
	wat := `
(component
    (import "math" (instance $math
        (export "double" (func (param "x" u32) (result u32)))
    ))
    (alias export $math "double" (func $double))
    (core func $double_lower (canon lower (func $double)))
    (core module $m
        (import "" "double" (func $double (param i32) (result i32)))
        (func (export "quadruple") (param i32) (result i32)
            local.get 0
            call $double
            call $double
        )
    )
    (core instance $i (instantiate $m
        (with "" (instance (export "double" (func $double_lower))))
    ))
    (func (export "quadruple") (param "x" u32) (result u32)
        (canon lift (core func $i "quadruple"))
    )
)
`
	fi := newFixtureInstanceWithLinker(t, wat, func(linker *ComponentLinker) {
		linker.Root(func(root *ComponentLinkerInstance) {
			err := root.AddInstance("math", func(inst *ComponentLinkerInstance) {
				err := inst.AddFunc("double", func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
					results[0] = NewComponentValU32(args[0].U32() * 2)
					return nil
				})
				require.NoError(t, err)
			})
			require.NoError(t, err)
		})
	})
	defer fi.Close()

	res := fi.mustCall("quadruple", []ComponentVal{NewComponentValU32(5)})
	require.Equal(t, uint32(20), res[0].U32())
}

// =============================================================================
// From component_model.rs: components_importing_modules
// Tests that a component importing a core module compiles correctly.
// =============================================================================

func TestFixtureComponentImportingModule(t *testing.T) {
	engine := newComponentEngine()

	// A component that imports a core module and uses it
	wat := `
(component
    (import "a" (core module $m1
        (import "" "" (func))
        (import "" "x" (global i32))

        (export "a" (table 1 funcref))
        (export "b" (memory 1))
        (export "c" (func (result f32)))
        (export "d" (global i64))
    ))

    (core module $m2
        (func (export ""))
        (global (export "x") i32 i32.const 0)
    )
    (core instance $i2 (instantiate $m2))
    (core instance $i1 (instantiate $m1 (with "" (instance $i2))))

    (core module $m3
        (import "mod" "1" (memory 1))
        (import "mod" "2" (table 1 funcref))
        (import "mod" "3" (global i64))
        (import "mod" "4" (func (result f32)))
    )

    (core instance $i3 (instantiate $m3
        (with "mod" (instance
            (export "1" (memory $i1 "b"))
            (export "2" (table $i1 "a"))
            (export "3" (global $i1 "d"))
            (export "4" (func $i1 "c"))
        ))
    ))
)
`
	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	// Compile a core module to satisfy the import
	coreWat := `
(module
    (import "" "" (func))
    (import "" "x" (global i32))
    (table (export "a") 1 funcref)
    (memory (export "b") 1)
    (func (export "c") (result f32) f32.const 0)
    (global (export "d") i64 i64.const 0)
)
`
	coreWasm, err := Wat2Wasm(coreWat)
	require.NoError(t, err)
	module, err := NewModule(engine, coreWasm)
	require.NoError(t, err)
	defer module.Close()

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddModule("a", module)
		require.NoError(t, err)
	})

	store := NewStore(engine)
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

// =============================================================================
// Tests variant with no payload case (unit variant).
// =============================================================================

func TestFixtureEchoVariantNoPayload(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(variant (case "empty") (case "has-val" u32))`,
			`(param $disc i32)
(param $payload i32)
(result i32)
(local $res i32)
local.get $disc
local.get $payload
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
			hostArgs = args
			// Return unit variant "empty"
			results[0] = NewComponentValVariant("empty", nil)
			return nil
		},
	)
	defer ctx.Close()

	// Send variant with value
	val := NewComponentValU32(42)
	arg := NewComponentValVariant("has-val", &val)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Host received "has-val" with payload
	require.Equal(t, "has-val", hostArgs[0].Variant().Discriminant)
	require.Equal(t, uint32(42), hostArgs[0].Variant().Val.U32())

	// Result is "empty" with nil payload
	require.Equal(t, "empty", results[0].Variant().Discriminant)
	require.Nil(t, results[0].Variant().Val)
}

// =============================================================================
// Tests multiple flags (more than 2).
// =============================================================================

func TestFixtureEchoManyFlags(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(flags "read" "write" "execute" "admin")`,
			`(param $x i32)
(result i32)
local.get $x
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			hostArgs = args
			results[0] = NewComponentValFlags([]string{"read", "execute"})
			return nil
		},
	)
	defer ctx.Close()

	// Send all flags set
	arg := NewComponentValFlags([]string{"read", "write", "execute", "admin"})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Host received all four flags
	require.Equal(t, []string{"read", "write", "execute", "admin"}, hostArgs[0].Flags())

	// Result has only read and execute
	require.Equal(t, []string{"read", "execute"}, results[0].Flags())
}

// =============================================================================
// Tests component with multiple distinct exports.
// =============================================================================

func TestFixtureMultipleExports(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "get-name") (result i32)
            (i32.store offset=0 (i32.const 0) (i32.const 100))
            (i32.store offset=4 (i32.const 0) (i32.const 5))
            i32.const 0
        )
        (func (export "get-version") (result i32) i32.const 42)
        (func (export "is-ready") (result i32) i32.const 1)
        (memory (export "memory") 1)
        (data (i32.const 100) "hello")
` + reallocAndFree + `
    )
    (core instance $i (instantiate $m))
    (func (export "get-name") (result string)
        (canon lift (core func $i "get-name") (memory $i "memory") (realloc (func $i "realloc")))
    )
    (func (export "get-version") (result u32)
        (canon lift (core func $i "get-version"))
    )
    (func (export "is-ready") (result bool)
        (canon lift (core func $i "is-ready"))
    )
)
`
	fi := newFixtureInstance(t, wat)
	defer fi.Close()

	name := fi.mustCall("get-name", []ComponentVal{})
	require.Equal(t, "hello", name[0].String())

	version := fi.mustCall("get-version", []ComponentVal{})
	require.Equal(t, uint32(42), version[0].U32())

	ready := fi.mustCall("is-ready", []ComponentVal{})
	require.Equal(t, true, ready[0].Bool())
}

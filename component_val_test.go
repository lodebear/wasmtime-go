package wasmtime

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// REALLOC_AND_FREE is the standard bump-allocator used in component model tests.
// Translated from crates/component-util/src/lib.rs.
const reallocAndFree = `
(global $last (mut i32) (i32.const 8))
(func $realloc (export "realloc")
	(param $old_ptr i32)
	(param $old_size i32)
	(param $align i32)
	(param $new_size i32)
	(result i32)

	(local $ret i32)

	;; Test if the old pointer is non-null
	local.get $old_ptr
	if
		;; If the old size is bigger than the new size then
		;; this is a shrink and transparently allow it
		local.get $old_size
		local.get $new_size
		i32.gt_u
		if
			local.get $old_ptr
			return
		end

		;; otherwise fall through to allocate a new chunk which will later
		;; copy data over
	end

	;; align up $last
	(global.set $last
		(i32.and
			(i32.add
				(global.get $last)
				(i32.add
					(local.get $align)
					(i32.const -1)))
			(i32.xor
				(i32.add
					(local.get $align)
					(i32.const -1))
				(i32.const -1))))

	;; save the current value of $last as the return value
	global.get $last
	local.set $ret

	;; bump our pointer
	(global.set $last
		(i32.add
			(global.get $last)
			(local.get $new_size)))

	;; while memory.size is less than $last, grow memory by one page
	(loop $loop
		(if
			(i32.lt_u
				(i32.mul (memory.size) (i32.const 65536))
				(global.get $last))
			(then
				i32.const 1
				memory.grow
				;; test to make sure growth succeeded
				i32.const -1
				i32.eq
				if unreachable end

				br $loop)))

	;; ensure anything necessary is set to valid data by spraying a bit
	;; pattern that is invalid
	local.get $ret
	i32.const 0xde
	local.get $new_size
	memory.fill

	;; If the old pointer is present then that means this was a reallocation
	;; of an existing chunk which means the existing data must be copied.
	local.get $old_ptr
	if
		local.get $ret          ;; destination
		local.get $old_ptr      ;; source
		local.get $old_size     ;; size
		memory.copy
	end

	local.get $ret
)
`

// echoComponent generates a component WAT that:
// - Imports a type $Foo (parameterized by `typeDef`)
// - Imports a host function "do" (param "a" $Foo) (result $Foo)
// - Has a core module with realloc/free + a "call" function with the given `funcBody`
// - Exports "call" lifted through the component model
//
// This mirrors the echo_component helper in the C++ values.cc test.
func echoComponent(typeDef, funcBody, hostParams string) string {
	return fmt.Sprintf(`
(component
	(type $Foo' %s)
	(import "foo" (type $Foo (eq $Foo')))
	(import "do" (func $do (param "a" $Foo) (result $Foo)))
	(core module $libc
		(memory (export "memory") 1)
		%s
	)
	(core instance $libc (instantiate $libc))
	(core func $do_lower (canon lower (func $do) (memory $libc "memory") (realloc (func $libc "realloc"))))

	(core module $doer
		(import "host" "do" (func $do %s))
		(import "libc" "memory" (memory 1))
		(import "libc" "realloc" (func $realloc (param i32 i32 i32 i32) (result i32)))

		(func (export "call")
			%s)
	)
	(core instance $doer (instantiate $doer
		(with "host" (instance (export "do" (func $do_lower))))
		(with "libc" (instance $libc))
	))

	(func $call
		(param "a" $Foo)
		(result $Foo)
		(canon lift
			(core func $doer "call")
			(memory $libc "memory")
			(realloc (func $libc "realloc")))
	)

	(export "call" (func $call))
)
`, typeDef, reallocAndFree, hostParams, funcBody)
}

// echoTestContext sets up the engine, linker with a host "do" callback, compiles
// the component, instantiates it, and returns the "call" function ready to invoke.
type echoTestContext struct {
	engine   *Engine
	store    *Store
	linker   *ComponentLinker
	instance *ComponentInstance
	callFunc *ComponentFunc
}

func newEchoTestContext(t *testing.T, componentWAT string, callback ComponentFuncCallback) *echoTestContext {
	t.Helper()
	engine := newComponentEngine()
	store := NewStore(engine)

	linker := NewComponentLinker(engine)

	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddFunc("do", callback)
		require.NoError(t, err)
	})

	wasm, err := Wat2Wasm(componentWAT)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	exportIdx := component.GetExportIndex(nil, "call")
	require.NotNil(t, exportIdx, "expected to find export 'call'")
	defer exportIdx.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn, "expected to get function 'call'")

	return &echoTestContext{
		engine:   engine,
		store:    store,
		linker:   linker,
		instance: instance,
		callFunc: fn,
	}
}

func (ctx *echoTestContext) Close() {
	ctx.linker.Close()
}

// --- Test: call_func (from call_func.cc) ---

func TestComponentCallFunc(t *testing.T) {
	// A component with a function "f" that adds two u32 params
	wat := `
(component
    (core module $m
        (func (export "f") (param $x i32) (param $y i32) (result i32)
            (local.get $x)
            (local.get $y)
            (i32.add)
        )
    )
    (core instance $i (instantiate $m))
    (func $f (param "x" u32) (param "y" u32) (result u32) (canon lift (core func $i "f")))
    (export "f" (func $f))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	exportIdx := component.GetExportIndex(nil, "f")
	require.NotNil(t, exportIdx)
	defer exportIdx.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn)

	results, err := fn.Call(store, []ComponentVal{
		NewComponentValU32(34),
		NewComponentValU32(35),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindU32, results[0].Kind())
	require.Equal(t, uint32(69), results[0].U32())
}

// --- Test: lookup_func (from lookup_func.cc) ---
// Already covered by TestComponentExportLookup and TestComponentInstanceGetFunc
// in component_test.go, but we add a direct translation for completeness.

func TestComponentLookupFunc(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "f"))
    )
    (core instance $i (instantiate $m))
    (func (export "f")
        (canon lift (core func $i "f")))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	// Non-existent export returns nil
	missing := component.GetExportIndex(nil, "ff")
	require.Nil(t, missing)

	// Existing export returns non-nil
	exportIdx := component.GetExportIndex(nil, "f")
	require.NotNil(t, exportIdx)
	defer exportIdx.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn)

	// Also test instance-level export index lookup
	instanceExportIdx := instance.GetExportIndex(store, nil, "f")
	require.NotNil(t, instanceExportIdx)
	defer instanceExportIdx.Close()
}

// --- Tests: value types (from values.cc) ---

func TestComponentValueRecord(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(record (field "x" u64) (field "y" u64))`,
			`(param $x i64)
(param $y i64)
(result i32)
(local $res i32)
local.get $x
local.get $y
(call $realloc
	(i32.const 0)
	(i32.const 0)
	(i32.const 4)
	(i32.const 16))
local.tee $res
call $do
local.get $res`,
			"(param i64 i64 i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args

			// Return a record {x: 3, y: 4}
			results[0] = NewComponentValRecord([]ComponentRecordField{
				{Name: "x", Val: NewComponentValU64(3)},
				{Name: "y", Val: NewComponentValU64(4)},
			})
			return nil
		},
	)
	defer ctx.Close()

	// Call with record {x: 1, y: 2}
	arg := NewComponentValRecord([]ComponentRecordField{
		{Name: "x", Val: NewComponentValU64(1)},
		{Name: "y", Val: NewComponentValU64(2)},
	})

	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify the host callback received {x: 1, y: 2}
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindRecord, hostArgs[0].Kind())
	fields := hostArgs[0].Record()
	require.Len(t, fields, 2)
	require.Equal(t, "x", fields[0].Name)
	require.Equal(t, uint64(1), fields[0].Val.U64())
	require.Equal(t, "y", fields[1].Name)
	require.Equal(t, uint64(2), fields[1].Val.U64())

	// Verify the result is {x: 3, y: 4}
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindRecord, results[0].Kind())
	resFields := results[0].Record()
	require.Len(t, resFields, 2)
	require.Equal(t, "x", resFields[0].Name)
	require.Equal(t, uint64(3), resFields[0].Val.U64())
	require.Equal(t, "y", resFields[1].Name)
	require.Equal(t, uint64(4), resFields[1].Val.U64())
}

func TestComponentValueString(t *testing.T) {
	var hostArgs []ComponentVal

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
			hostArgs = args
			results[0] = NewComponentValString("hello from B!")
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValString("hello from A!")
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received the input string
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindString, hostArgs[0].Kind())
	require.Equal(t, "hello from A!", hostArgs[0].String())

	// Verify result string
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindString, results[0].Kind())
	require.Equal(t, "hello from B!", results[0].String())
}

func TestComponentValueList(t *testing.T) {
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
			results[0] = NewComponentValList([]ComponentVal{
				NewComponentValU32(4),
				NewComponentValU32(5),
				NewComponentValU32(6),
				NewComponentValU32(7),
			})
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValList([]ComponentVal{
		NewComponentValU32(1),
		NewComponentValU32(2),
		NewComponentValU32(3),
	})

	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received [1, 2, 3]
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindList, hostArgs[0].Kind())
	list := hostArgs[0].List()
	require.Len(t, list, 3)
	require.Equal(t, uint32(1), list[0].U32())
	require.Equal(t, uint32(2), list[1].U32())
	require.Equal(t, uint32(3), list[2].U32())

	// Verify result [4, 5, 6, 7]
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindList, results[0].Kind())
	resList := results[0].List()
	require.Len(t, resList, 4)
	require.Equal(t, uint32(4), resList[0].U32())
	require.Equal(t, uint32(5), resList[1].U32())
	require.Equal(t, uint32(6), resList[2].U32())
	require.Equal(t, uint32(7), resList[3].U32())
}

func TestComponentValueTuple(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(tuple u32 u32 u32)`,
			`(param $x i32)
(param $y i32)
(param $z i32)
(result i32)
(local $res i32)
local.get $x
local.get $y
local.get $z
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
			results[0] = NewComponentValTuple([]ComponentVal{
				NewComponentValU32(4),
				NewComponentValU32(5),
				NewComponentValU32(6),
			})
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValTuple([]ComponentVal{
		NewComponentValU32(1),
		NewComponentValU32(2),
		NewComponentValU32(3),
	})

	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received (1, 2, 3)
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindTuple, hostArgs[0].Kind())
	tuple := hostArgs[0].Tuple()
	require.Len(t, tuple, 3)
	require.Equal(t, uint32(1), tuple[0].U32())
	require.Equal(t, uint32(2), tuple[1].U32())
	require.Equal(t, uint32(3), tuple[2].U32())

	// Verify result (4, 5, 6)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindTuple, results[0].Kind())
	resTuple := results[0].Tuple()
	require.Len(t, resTuple, 3)
	require.Equal(t, uint32(4), resTuple[0].U32())
	require.Equal(t, uint32(5), resTuple[1].U32())
	require.Equal(t, uint32(6), resTuple[2].U32())
}

func TestComponentValueVariant(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(variant
	(case "aa" u32)
	(case "bb" string)
)`,
			`(param $x i32)
(param $y i32)
(param $z i32)
(result i32)
(local $res i32)
local.get $x
local.get $y
local.get $z
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

			// Return variant bb("textt")
			strVal := NewComponentValString("textt")
			results[0] = NewComponentValVariant("bb", &strVal)
			return nil
		},
	)
	defer ctx.Close()

	// Send variant aa(123)
	u32Val := NewComponentValU32(123)
	arg := NewComponentValVariant("aa", &u32Val)

	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received variant aa(123)
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindVariant, hostArgs[0].Kind())
	v := hostArgs[0].Variant()
	require.Equal(t, "aa", v.Discriminant)
	require.NotNil(t, v.Val)
	require.Equal(t, uint32(123), v.Val.U32())

	// Verify result is variant bb("textt")
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindVariant, results[0].Kind())
	resVariant := results[0].Variant()
	require.Equal(t, "bb", resVariant.Discriminant)
	require.NotNil(t, resVariant.Val)
	require.Equal(t, "textt", resVariant.Val.String())
}

func TestComponentValueEnum(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(enum "aa" "bb")`,
			`(param $x i32)
(result i32)
local.get $x
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			results[0] = NewComponentValEnum("bb")
			return nil
		},
	)
	defer ctx.Close()

	arg := NewComponentValEnum("aa")
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received "aa"
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindEnum, hostArgs[0].Kind())
	require.Equal(t, "aa", hostArgs[0].Enum())

	// Verify result is "bb"
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindEnum, results[0].Kind())
	require.Equal(t, "bb", results[0].Enum())
}

func TestComponentValueOption(t *testing.T) {
	var hostArgs []ComponentVal

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
			require.Len(t, args, 1)
			hostArgs = args
			// Return None
			results[0] = NewComponentValOptionNone()
			return nil
		},
	)
	defer ctx.Close()

	// Send Some(123)
	arg := NewComponentValOptionSome(NewComponentValU32(123))
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received Some(123)
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindOption, hostArgs[0].Kind())
	optVal := hostArgs[0].Option()
	require.NotNil(t, optVal)
	require.Equal(t, uint32(123), optVal.U32())

	// Verify result is None
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindOption, results[0].Kind())
	require.Nil(t, results[0].Option())
}

func TestComponentValueResult(t *testing.T) {
	var hostArgs []ComponentVal

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
			require.Len(t, args, 1)
			hostArgs = args
			// Return Err(456)
			errVal := NewComponentValU32(456)
			results[0] = NewComponentValResultErr(&errVal)
			return nil
		},
	)
	defer ctx.Close()

	// Send Ok(123)
	okVal := NewComponentValU32(123)
	arg := NewComponentValResultOk(&okVal)
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received Ok(123)
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindResult, hostArgs[0].Kind())
	r := hostArgs[0].Result()
	require.True(t, r.IsOk)
	require.NotNil(t, r.Val)
	require.Equal(t, uint32(123), r.Val.U32())

	// Verify result is Err(456)
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindResult, results[0].Kind())
	resResult := results[0].Result()
	require.False(t, resResult.IsOk)
	require.NotNil(t, resResult.Val)
	require.Equal(t, uint32(456), resResult.Val.U32())
}

func TestComponentValueFlags(t *testing.T) {
	var hostArgs []ComponentVal

	ctx := newEchoTestContext(t,
		echoComponent(
			`(flags "aa" "bb")`,
			`(param $x i32)
(result i32)
local.get $x
call $do`,
			"(param i32) (result i32)",
		),
		func(caller Storelike, args []ComponentVal, results []ComponentVal) error {
			require.Len(t, args, 1)
			hostArgs = args
			results[0] = NewComponentValFlags([]string{"aa", "bb"})
			return nil
		},
	)
	defer ctx.Close()

	// Send flags with just "aa" set
	arg := NewComponentValFlags([]string{"aa"})
	results, err := ctx.callFunc.Call(ctx.store, []ComponentVal{arg})
	require.NoError(t, err)

	// Verify host received {"aa"}
	require.Len(t, hostArgs, 1)
	require.Equal(t, ComponentValKindFlags, hostArgs[0].Kind())
	require.Equal(t, []string{"aa"}, hostArgs[0].Flags())

	// Verify result is {"aa", "bb"}
	require.Len(t, results, 1)
	require.Equal(t, ComponentValKindFlags, results[0].Kind())
	require.Equal(t, []string{"aa", "bb"}, results[0].Flags())
}

// --- Tests: pure Go-side value construction and accessors ---

func TestComponentValScalars(t *testing.T) {
	// bool
	bv := NewComponentValBool(true)
	require.Equal(t, ComponentValKindBool, bv.Kind())
	require.True(t, bv.Bool())

	bv2 := NewComponentValBool(false)
	require.False(t, bv2.Bool())

	// integers
	s8v := NewComponentValS8(-1)
	require.Equal(t, int8(-1), s8v.S8())
	u8v := NewComponentValU8(255)
	require.Equal(t, uint8(255), u8v.U8())
	s16v := NewComponentValS16(-1000)
	require.Equal(t, int16(-1000), s16v.S16())
	u16v := NewComponentValU16(65535)
	require.Equal(t, uint16(65535), u16v.U16())
	s32v := NewComponentValS32(-100000)
	require.Equal(t, int32(-100000), s32v.S32())
	u32v := NewComponentValU32(4000000000)
	require.Equal(t, uint32(4000000000), u32v.U32())
	s64v := NewComponentValS64(-1)
	require.Equal(t, int64(-1), s64v.S64())
	u64v := NewComponentValU64(18446744073709551615)
	require.Equal(t, uint64(18446744073709551615), u64v.U64())

	// floats
	f32v := NewComponentValF32(3.14)
	require.Equal(t, float32(3.14), f32v.F32())
	f64v := NewComponentValF64(2.718281828)
	require.Equal(t, float64(2.718281828), f64v.F64())

	// char
	cv := NewComponentValChar('A')
	require.Equal(t, 'A', cv.Char())
	cv2 := NewComponentValChar(0x1F600)
	require.Equal(t, rune(0x1F600), cv2.Char())
}

func TestComponentValString(t *testing.T) {
	sv := NewComponentValString("hello")
	require.Equal(t, ComponentValKindString, sv.Kind())
	require.Equal(t, "hello", sv.String())

	// Empty string
	sv2 := NewComponentValString("")
	require.Equal(t, "", sv2.String())
}

func TestComponentValList(t *testing.T) {
	lv := NewComponentValList([]ComponentVal{
		NewComponentValU32(1),
		NewComponentValU32(2),
		NewComponentValU32(3),
	})
	require.Equal(t, ComponentValKindList, lv.Kind())
	items := lv.List()
	require.Len(t, items, 3)
	require.Equal(t, uint32(1), items[0].U32())
	require.Equal(t, uint32(2), items[1].U32())
	require.Equal(t, uint32(3), items[2].U32())
}

func TestComponentValRecord(t *testing.T) {
	rv := NewComponentValRecord([]ComponentRecordField{
		{Name: "x", Val: NewComponentValU64(1)},
		{Name: "y", Val: NewComponentValU64(2)},
	})
	require.Equal(t, ComponentValKindRecord, rv.Kind())
	fields := rv.Record()
	require.Len(t, fields, 2)
	require.Equal(t, "x", fields[0].Name)
	require.Equal(t, uint64(1), fields[0].Val.U64())
	require.Equal(t, "y", fields[1].Name)
	require.Equal(t, uint64(2), fields[1].Val.U64())
}

func TestComponentValTuple(t *testing.T) {
	tv := NewComponentValTuple([]ComponentVal{
		NewComponentValU32(10),
		NewComponentValU64(20),
	})
	require.Equal(t, ComponentValKindTuple, tv.Kind())
	items := tv.Tuple()
	require.Len(t, items, 2)
	require.Equal(t, uint32(10), items[0].U32())
	require.Equal(t, uint64(20), items[1].U64())
}

func TestComponentValVariant(t *testing.T) {
	payload := NewComponentValU32(42)
	vv := NewComponentValVariant("hello", &payload)
	require.Equal(t, ComponentValKindVariant, vv.Kind())
	variant := vv.Variant()
	require.Equal(t, "hello", variant.Discriminant)
	require.NotNil(t, variant.Val)
	require.Equal(t, uint32(42), variant.Val.U32())

	// Variant with nil payload
	vv2 := NewComponentValVariant("unit-case", nil)
	variant2 := vv2.Variant()
	require.Equal(t, "unit-case", variant2.Discriminant)
	require.Nil(t, variant2.Val)
}

func TestComponentValEnum(t *testing.T) {
	ev := NewComponentValEnum("hello")
	require.Equal(t, ComponentValKindEnum, ev.Kind())
	require.Equal(t, "hello", ev.Enum())
}

func TestComponentValOption(t *testing.T) {
	// Some
	ov := NewComponentValOptionSome(NewComponentValU32(42))
	require.Equal(t, ComponentValKindOption, ov.Kind())
	opt := ov.Option()
	require.NotNil(t, opt)
	require.Equal(t, uint32(42), opt.U32())

	// None
	ov2 := NewComponentValOptionNone()
	require.Equal(t, ComponentValKindOption, ov2.Kind())
	require.Nil(t, ov2.Option())
}

func TestComponentValResult(t *testing.T) {
	// Ok with payload
	okVal := NewComponentValU32(42)
	rv := NewComponentValResultOk(&okVal)
	require.Equal(t, ComponentValKindResult, rv.Kind())
	r := rv.Result()
	require.True(t, r.IsOk)
	require.NotNil(t, r.Val)
	require.Equal(t, uint32(42), r.Val.U32())

	// Ok without payload
	rv2 := NewComponentValResultOk(nil)
	r2 := rv2.Result()
	require.True(t, r2.IsOk)
	require.Nil(t, r2.Val)

	// Err without payload
	rv3 := NewComponentValResultErr(nil)
	r3 := rv3.Result()
	require.False(t, r3.IsOk)
	require.Nil(t, r3.Val)
}

func TestComponentValFlags(t *testing.T) {
	fv := NewComponentValFlags([]string{"a", "b", "c"})
	require.Equal(t, ComponentValKindFlags, fv.Kind())
	flags := fv.Flags()
	require.Equal(t, []string{"a", "b", "c"}, flags)

	// Empty flags
	fv2 := NewComponentValFlags([]string{})
	require.Empty(t, fv2.Flags())
}

// --- Test: ComponentFuncType ---

func TestComponentFuncType(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	// Component with a function that takes 2 params and returns 1 result
	wat := `
(component
    (core module $m
        (func (export "f") (param i32 i32) (result i32)
            (local.get 0)
            (local.get 1)
            (i32.add)
        )
    )
    (core instance $i (instantiate $m))
    (func $f (param "x" u32) (param "y" u32) (result u32) (canon lift (core func $i "f")))
    (export "f" (func $f))
)
`
	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	exportIdx := component.GetExportIndex(nil, "f")
	require.NotNil(t, exportIdx)
	defer exportIdx.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn)

	ft := fn.Type(store)
	defer ft.Close()

	require.Equal(t, 2, ft.ParamCount())
	require.Equal(t, 1, ft.ResultArity())
	require.False(t, ft.IsAsync())
}

// --- Test: ComponentFunc with no results ---

func TestComponentCallFuncNoResult(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "f"))
    )
    (core instance $i (instantiate $m))
    (func $f (canon lift (core func $i "f")))
    (export "f" (func $f))
)
`
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	exportIdx := component.GetExportIndex(nil, "f")
	require.NotNil(t, exportIdx)
	defer exportIdx.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn)

	results, err := fn.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Nil(t, results)
}

// --- Test: host callback for guest resource ---

func TestComponentValueGuestResource(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	wat := `
(component
  (core module $a
    (global $g (mut i32) (i32.const 0))

    (func (export "dtor") (param i32) (global.set $g (local.get 0)))
    (func (export "last-dtor-rep") (result i32) global.get $g)
  )
  (core instance $a (instantiate $a))
  (type $r' (resource (rep i32) (dtor (func $a "dtor"))))
  (export $r "r" (type $r'))

  (core func $new (canon resource.new $r))
  (core func $drop (canon resource.drop $r))

  (core module $b
    (import "" "new" (func $new (param i32) (result i32)))
    (import "" "drop" (func $drop (param i32)))

    (func (export "new") (param i32) (result i32)
      local.get 0
      call $new)

    (func (export "rep") (param i32) (result i32)
      local.get 0)

    (func (export "drop") (param i32)
      local.get 0
      call $drop)
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
)
`
	wasm, err := Wat2Wasm(wat)
	require.NoError(t, err)

	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// Look up all the exported functions
	getFunc := func(name string) *ComponentFunc {
		idx := instance.GetExportIndex(store, nil, name)
		require.NotNil(t, idx, "expected export %q", name)
		defer idx.Close()
		fn := instance.GetFunc(store, idx)
		require.NotNil(t, fn, "expected func for %q", name)
		return fn
	}

	newFunc := getFunc("new")
	repFunc := getFunc("rep")
	dropFunc := getFunc("drop")
	lastDtorRepFunc := getFunc("last-dtor-rep")

	// Create two resources with reps 100 and 101
	res1, err := newFunc.Call(store, []ComponentVal{NewComponentValU32(100)})
	require.NoError(t, err)
	require.Len(t, res1, 1)
	require.Equal(t, ComponentValKindResource, res1[0].Kind())

	res2, err := newFunc.Call(store, []ComponentVal{NewComponentValU32(101)})
	require.NoError(t, err)
	require.Len(t, res2, 1)
	require.Equal(t, ComponentValKindResource, res2[0].Kind())

	// Use borrow to get rep values back
	repRes1, err := repFunc.Call(store, []ComponentVal{res1[0]})
	require.NoError(t, err)
	require.Len(t, repRes1, 1)
	require.Equal(t, uint32(100), repRes1[0].U32())

	repRes2, err := repFunc.Call(store, []ComponentVal{res2[0]})
	require.NoError(t, err)
	require.Len(t, repRes2, 1)
	require.Equal(t, uint32(101), repRes2[0].U32())

	// Check last destructor rep (should be 0, no drops yet)
	lastRep, err := lastDtorRepFunc.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Len(t, lastRep, 1)
	require.Equal(t, uint32(0), lastRep[0].U32())

	// Drop the first resource (own)
	_, err = dropFunc.Call(store, []ComponentVal{res1[0]})
	require.NoError(t, err)

	// Last dtor rep should now be 100
	lastRep, err = lastDtorRepFunc.Call(store, []ComponentVal{})
	require.NoError(t, err)
	require.Len(t, lastRep, 1)
	require.Equal(t, uint32(100), lastRep[0].U32())
}

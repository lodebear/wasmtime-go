package wasmtime

// #include <wasmtime.h>
// #include <stdlib.h>
// #include "shims.h"
import "C"
import (
	"runtime"
	"unsafe"
)

// ComponentValKind represents the kind discriminant for component model values.
// This corresponds to wasmtime_component_valkind_t in the C API.
type ComponentValKind uint8

const (
	ComponentValKindBool     ComponentValKind = C.WASMTIME_COMPONENT_BOOL
	ComponentValKindS8       ComponentValKind = C.WASMTIME_COMPONENT_S8
	ComponentValKindU8       ComponentValKind = C.WASMTIME_COMPONENT_U8
	ComponentValKindS16      ComponentValKind = C.WASMTIME_COMPONENT_S16
	ComponentValKindU16      ComponentValKind = C.WASMTIME_COMPONENT_U16
	ComponentValKindS32      ComponentValKind = C.WASMTIME_COMPONENT_S32
	ComponentValKindU32      ComponentValKind = C.WASMTIME_COMPONENT_U32
	ComponentValKindS64      ComponentValKind = C.WASMTIME_COMPONENT_S64
	ComponentValKindU64      ComponentValKind = C.WASMTIME_COMPONENT_U64
	ComponentValKindF32      ComponentValKind = C.WASMTIME_COMPONENT_F32
	ComponentValKindF64      ComponentValKind = C.WASMTIME_COMPONENT_F64
	ComponentValKindChar     ComponentValKind = C.WASMTIME_COMPONENT_CHAR
	ComponentValKindString   ComponentValKind = C.WASMTIME_COMPONENT_STRING
	ComponentValKindList     ComponentValKind = C.WASMTIME_COMPONENT_LIST
	ComponentValKindRecord   ComponentValKind = C.WASMTIME_COMPONENT_RECORD
	ComponentValKindTuple    ComponentValKind = C.WASMTIME_COMPONENT_TUPLE
	ComponentValKindVariant  ComponentValKind = C.WASMTIME_COMPONENT_VARIANT
	ComponentValKindEnum     ComponentValKind = C.WASMTIME_COMPONENT_ENUM
	ComponentValKindOption   ComponentValKind = C.WASMTIME_COMPONENT_OPTION
	ComponentValKindResult   ComponentValKind = C.WASMTIME_COMPONENT_RESULT
	ComponentValKindFlags    ComponentValKind = C.WASMTIME_COMPONENT_FLAGS
	ComponentValKindResource ComponentValKind = C.WASMTIME_COMPONENT_RESOURCE
)

// String returns a human-readable name for the value kind.
func (k ComponentValKind) String() string {
	switch k {
	case ComponentValKindBool:
		return "bool"
	case ComponentValKindS8:
		return "s8"
	case ComponentValKindU8:
		return "u8"
	case ComponentValKindS16:
		return "s16"
	case ComponentValKindU16:
		return "u16"
	case ComponentValKindS32:
		return "s32"
	case ComponentValKindU32:
		return "u32"
	case ComponentValKindS64:
		return "s64"
	case ComponentValKindU64:
		return "u64"
	case ComponentValKindF32:
		return "f32"
	case ComponentValKindF64:
		return "f64"
	case ComponentValKindChar:
		return "char"
	case ComponentValKindString:
		return "string"
	case ComponentValKindList:
		return "list"
	case ComponentValKindRecord:
		return "record"
	case ComponentValKindTuple:
		return "tuple"
	case ComponentValKindVariant:
		return "variant"
	case ComponentValKindEnum:
		return "enum"
	case ComponentValKindOption:
		return "option"
	case ComponentValKindResult:
		return "result"
	case ComponentValKindFlags:
		return "flags"
	case ComponentValKindResource:
		return "resource"
	}
	panic("unknown component val kind")
}

// ComponentVal represents a runtime value in the component model.
// It stores Go-native types internally and only converts to/from C at call boundaries.
type ComponentVal struct {
	kind ComponentValKind
	val  interface{}
}

// ComponentRecordField represents a single named field in a record value.
type ComponentRecordField struct {
	Name string
	Val  ComponentVal
}

// ComponentVariant represents a variant value with a named discriminant and optional payload.
type ComponentVariant struct {
	Discriminant string
	Val          *ComponentVal
}

// ComponentResult represents a result value (ok or err) with optional payload.
type ComponentResult struct {
	IsOk bool
	Val  *ComponentVal
}

// Kind returns the kind of this component value.
func (v *ComponentVal) Kind() ComponentValKind {
	return v.kind
}

// --- Constructors ---

// NewComponentValBool creates a bool component value.
func NewComponentValBool(v bool) ComponentVal {
	return ComponentVal{kind: ComponentValKindBool, val: v}
}

// NewComponentValS8 creates an s8 component value.
func NewComponentValS8(v int8) ComponentVal {
	return ComponentVal{kind: ComponentValKindS8, val: v}
}

// NewComponentValU8 creates a u8 component value.
func NewComponentValU8(v uint8) ComponentVal {
	return ComponentVal{kind: ComponentValKindU8, val: v}
}

// NewComponentValS16 creates an s16 component value.
func NewComponentValS16(v int16) ComponentVal {
	return ComponentVal{kind: ComponentValKindS16, val: v}
}

// NewComponentValU16 creates a u16 component value.
func NewComponentValU16(v uint16) ComponentVal {
	return ComponentVal{kind: ComponentValKindU16, val: v}
}

// NewComponentValS32 creates an s32 component value.
func NewComponentValS32(v int32) ComponentVal {
	return ComponentVal{kind: ComponentValKindS32, val: v}
}

// NewComponentValU32 creates a u32 component value.
func NewComponentValU32(v uint32) ComponentVal {
	return ComponentVal{kind: ComponentValKindU32, val: v}
}

// NewComponentValS64 creates an s64 component value.
func NewComponentValS64(v int64) ComponentVal {
	return ComponentVal{kind: ComponentValKindS64, val: v}
}

// NewComponentValU64 creates a u64 component value.
func NewComponentValU64(v uint64) ComponentVal {
	return ComponentVal{kind: ComponentValKindU64, val: v}
}

// NewComponentValF32 creates an f32 component value.
func NewComponentValF32(v float32) ComponentVal {
	return ComponentVal{kind: ComponentValKindF32, val: v}
}

// NewComponentValF64 creates an f64 component value.
func NewComponentValF64(v float64) ComponentVal {
	return ComponentVal{kind: ComponentValKindF64, val: v}
}

// NewComponentValChar creates a char component value.
func NewComponentValChar(v rune) ComponentVal {
	return ComponentVal{kind: ComponentValKindChar, val: v}
}

// NewComponentValString creates a string component value.
func NewComponentValString(v string) ComponentVal {
	return ComponentVal{kind: ComponentValKindString, val: v}
}

// NewComponentValList creates a list component value.
func NewComponentValList(items []ComponentVal) ComponentVal {
	cp := make([]ComponentVal, len(items))
	copy(cp, items)
	return ComponentVal{kind: ComponentValKindList, val: cp}
}

// NewComponentValRecord creates a record component value.
func NewComponentValRecord(fields []ComponentRecordField) ComponentVal {
	cp := make([]ComponentRecordField, len(fields))
	copy(cp, fields)
	return ComponentVal{kind: ComponentValKindRecord, val: cp}
}

// NewComponentValTuple creates a tuple component value.
func NewComponentValTuple(items []ComponentVal) ComponentVal {
	cp := make([]ComponentVal, len(items))
	copy(cp, items)
	return ComponentVal{kind: ComponentValKindTuple, val: cp}
}

// NewComponentValVariant creates a variant component value.
// payload may be nil if the case carries no data.
func NewComponentValVariant(discriminant string, payload *ComponentVal) ComponentVal {
	v := ComponentVariant{Discriminant: discriminant, Val: payload}
	return ComponentVal{kind: ComponentValKindVariant, val: v}
}

// NewComponentValEnum creates an enum component value.
func NewComponentValEnum(v string) ComponentVal {
	return ComponentVal{kind: ComponentValKindEnum, val: v}
}

// NewComponentValOptionSome creates an option component value with a value (Some).
func NewComponentValOptionSome(inner ComponentVal) ComponentVal {
	return ComponentVal{kind: ComponentValKindOption, val: &inner}
}

// NewComponentValOptionNone creates an option component value with no value (None).
func NewComponentValOptionNone() ComponentVal {
	return ComponentVal{kind: ComponentValKindOption, val: (*ComponentVal)(nil)}
}

// NewComponentValResultOk creates a result component value with an ok payload.
// payload may be nil if ok carries no data.
func NewComponentValResultOk(payload *ComponentVal) ComponentVal {
	r := ComponentResult{IsOk: true, Val: payload}
	return ComponentVal{kind: ComponentValKindResult, val: r}
}

// NewComponentValResultErr creates a result component value with an err payload.
// payload may be nil if err carries no data.
func NewComponentValResultErr(payload *ComponentVal) ComponentVal {
	r := ComponentResult{IsOk: false, Val: payload}
	return ComponentVal{kind: ComponentValKindResult, val: r}
}

// NewComponentValFlags creates a flags component value.
func NewComponentValFlags(flags []string) ComponentVal {
	cp := make([]string, len(flags))
	copy(cp, flags)
	return ComponentVal{kind: ComponentValKindFlags, val: cp}
}

// NewComponentValResource creates a resource component value from a
// ComponentResourceAny. The resource handle is cloned so the ComponentVal
// owns its own copy.
func NewComponentValResource(ra *ComponentResourceAny) ComponentVal {
	cloned := C.wasmtime_component_resource_any_clone(ra.ptr())
	runtime.KeepAlive(ra)
	return ComponentVal{kind: ComponentValKindResource, val: mkComponentResourceAny(cloned)}
}

// --- Accessors (panic if wrong kind) ---

// Bool returns the bool value, or panics.
func (v *ComponentVal) Bool() bool {
	if v.kind != ComponentValKindBool {
		panic("not a bool")
	}
	return v.val.(bool)
}

// S8 returns the s8 value, or panics.
func (v *ComponentVal) S8() int8 {
	if v.kind != ComponentValKindS8 {
		panic("not an s8")
	}
	return v.val.(int8)
}

// U8 returns the u8 value, or panics.
func (v *ComponentVal) U8() uint8 {
	if v.kind != ComponentValKindU8 {
		panic("not a u8")
	}
	return v.val.(uint8)
}

// S16 returns the s16 value, or panics.
func (v *ComponentVal) S16() int16 {
	if v.kind != ComponentValKindS16 {
		panic("not an s16")
	}
	return v.val.(int16)
}

// U16 returns the u16 value, or panics.
func (v *ComponentVal) U16() uint16 {
	if v.kind != ComponentValKindU16 {
		panic("not a u16")
	}
	return v.val.(uint16)
}

// S32 returns the s32 value, or panics.
func (v *ComponentVal) S32() int32 {
	if v.kind != ComponentValKindS32 {
		panic("not an s32")
	}
	return v.val.(int32)
}

// U32 returns the u32 value, or panics.
func (v *ComponentVal) U32() uint32 {
	if v.kind != ComponentValKindU32 {
		panic("not a u32")
	}
	return v.val.(uint32)
}

// S64 returns the s64 value, or panics.
func (v *ComponentVal) S64() int64 {
	if v.kind != ComponentValKindS64 {
		panic("not an s64")
	}
	return v.val.(int64)
}

// U64 returns the u64 value, or panics.
func (v *ComponentVal) U64() uint64 {
	if v.kind != ComponentValKindU64 {
		panic("not a u64")
	}
	return v.val.(uint64)
}

// F32 returns the f32 value, or panics.
func (v *ComponentVal) F32() float32 {
	if v.kind != ComponentValKindF32 {
		panic("not an f32")
	}
	return v.val.(float32)
}

// F64 returns the f64 value, or panics.
func (v *ComponentVal) F64() float64 {
	if v.kind != ComponentValKindF64 {
		panic("not an f64")
	}
	return v.val.(float64)
}

// Char returns the char value, or panics.
func (v *ComponentVal) Char() rune {
	if v.kind != ComponentValKindChar {
		panic("not a char")
	}
	return v.val.(rune)
}

// String returns the string value, or panics.
// Note: if you want a debug representation, use Kind().String().
func (v *ComponentVal) String() string {
	if v.kind != ComponentValKindString {
		panic("not a string")
	}
	return v.val.(string)
}

// List returns the list elements, or panics.
func (v *ComponentVal) List() []ComponentVal {
	if v.kind != ComponentValKindList {
		panic("not a list")
	}
	return v.val.([]ComponentVal)
}

// Record returns the record fields, or panics.
func (v *ComponentVal) Record() []ComponentRecordField {
	if v.kind != ComponentValKindRecord {
		panic("not a record")
	}
	return v.val.([]ComponentRecordField)
}

// Tuple returns the tuple elements, or panics.
func (v *ComponentVal) Tuple() []ComponentVal {
	if v.kind != ComponentValKindTuple {
		panic("not a tuple")
	}
	return v.val.([]ComponentVal)
}

// Variant returns the variant value, or panics.
func (v *ComponentVal) Variant() ComponentVariant {
	if v.kind != ComponentValKindVariant {
		panic("not a variant")
	}
	return v.val.(ComponentVariant)
}

// Enum returns the enum discriminant string, or panics.
func (v *ComponentVal) Enum() string {
	if v.kind != ComponentValKindEnum {
		panic("not an enum")
	}
	return v.val.(string)
}

// Option returns the option payload (nil for None), or panics.
func (v *ComponentVal) Option() *ComponentVal {
	if v.kind != ComponentValKindOption {
		panic("not an option")
	}
	return v.val.(*ComponentVal)
}

// Result returns the result value, or panics.
func (v *ComponentVal) Result() ComponentResult {
	if v.kind != ComponentValKindResult {
		panic("not a result")
	}
	return v.val.(ComponentResult)
}

// Flags returns the list of active flag names, or panics.
func (v *ComponentVal) Flags() []string {
	if v.kind != ComponentValKindFlags {
		panic("not flags")
	}
	return v.val.([]string)
}

// ResourceAny returns the resource handle, or panics.
// The returned handle is still owned by this ComponentVal; clone it if you
// need an independent copy.
func (v *ComponentVal) ResourceAny() *ComponentResourceAny {
	if v.kind != ComponentValKindResource {
		panic("not a resource")
	}
	return v.val.(*ComponentResourceAny)
}

// --- Marshal Go -> C ---

// componentValToC writes the Go ComponentVal into a C wasmtime_component_val_t.
// The caller is responsible for calling wasmtime_component_val_delete on dst
// when done (or letting the vec _delete handle it for vec elements).
func componentValToC(src *ComponentVal, dst *C.wasmtime_component_val_t) {
	dst.kind = C.wasmtime_component_valkind_t(src.kind)

	switch src.kind {
	case ComponentValKindBool:
		C.go_wasmtime_component_val_boolean_set(dst, C.bool(src.val.(bool)))

	case ComponentValKindS8:
		C.go_wasmtime_component_val_s8_set(dst, C.int8_t(src.val.(int8)))

	case ComponentValKindU8:
		C.go_wasmtime_component_val_u8_set(dst, C.uint8_t(src.val.(uint8)))

	case ComponentValKindS16:
		C.go_wasmtime_component_val_s16_set(dst, C.int16_t(src.val.(int16)))

	case ComponentValKindU16:
		C.go_wasmtime_component_val_u16_set(dst, C.uint16_t(src.val.(uint16)))

	case ComponentValKindS32:
		C.go_wasmtime_component_val_s32_set(dst, C.int32_t(src.val.(int32)))

	case ComponentValKindU32:
		C.go_wasmtime_component_val_u32_set(dst, C.uint32_t(src.val.(uint32)))

	case ComponentValKindS64:
		C.go_wasmtime_component_val_s64_set(dst, C.int64_t(src.val.(int64)))

	case ComponentValKindU64:
		C.go_wasmtime_component_val_u64_set(dst, C.uint64_t(src.val.(uint64)))

	case ComponentValKindF32:
		C.go_wasmtime_component_val_f32_set(dst, C.float(src.val.(float32)))

	case ComponentValKindF64:
		C.go_wasmtime_component_val_f64_set(dst, C.double(src.val.(float64)))

	case ComponentValKindChar:
		C.go_wasmtime_component_val_character_set(dst, C.uint32_t(src.val.(rune)))

	case ComponentValKindString:
		s := src.val.(string)
		strPtr := C.go_component_val_string_ptr(dst)
		C.wasm_byte_vec_new_uninitialized(strPtr, C.size_t(len(s)))
		if len(s) > 0 {
			C.memcpy(unsafe.Pointer(strPtr.data), unsafe.Pointer(C._GoStringPtr(s)), C.size_t(len(s)))
			runtime.KeepAlive(s)
		}

	case ComponentValKindList:
		items := src.val.([]ComponentVal)
		listPtr := C.go_component_val_list_ptr(dst)
		if len(items) == 0 {
			C.wasmtime_component_vallist_new_empty(listPtr)
		} else {
			C.wasmtime_component_vallist_new_uninit(listPtr, C.size_t(len(items)))
			for i, item := range items {
				elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
					uintptr(unsafe.Pointer(listPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
				))
				itemCopy := item
				componentValToC(&itemCopy, elemPtr)
			}
		}

	case ComponentValKindRecord:
		fields := src.val.([]ComponentRecordField)
		recPtr := C.go_component_val_record_ptr(dst)
		if len(fields) == 0 {
			C.wasmtime_component_valrecord_new_empty(recPtr)
		} else {
			C.wasmtime_component_valrecord_new_uninit(recPtr, C.size_t(len(fields)))
			for i, field := range fields {
				entryPtr := (*C.wasmtime_component_valrecord_entry_t)(unsafe.Pointer(
					uintptr(unsafe.Pointer(recPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_valrecord_entry_t{}),
				))
				// Set the name
				namePtr := C.go_component_valrecord_entry_name_ptr(entryPtr)
				C.wasm_byte_vec_new_uninitialized(namePtr, C.size_t(len(field.Name)))
				if len(field.Name) > 0 {
					C.memcpy(unsafe.Pointer(namePtr.data), unsafe.Pointer(C._GoStringPtr(field.Name)), C.size_t(len(field.Name)))
					runtime.KeepAlive(field.Name)
				}
				// Set the val
				valPtr := C.go_component_valrecord_entry_val_ptr(entryPtr)
				fieldVal := field.Val
				componentValToC(&fieldVal, valPtr)
			}
		}

	case ComponentValKindTuple:
		items := src.val.([]ComponentVal)
		tuplePtr := C.go_component_val_tuple_ptr(dst)
		if len(items) == 0 {
			C.wasmtime_component_valtuple_new_empty(tuplePtr)
		} else {
			C.wasmtime_component_valtuple_new_uninit(tuplePtr, C.size_t(len(items)))
			for i, item := range items {
				elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
					uintptr(unsafe.Pointer(tuplePtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
				))
				itemCopy := item
				componentValToC(&itemCopy, elemPtr)
			}
		}

	case ComponentValKindVariant:
		variant := src.val.(ComponentVariant)
		varPtr := C.go_component_val_variant_ptr(dst)

		// Set discriminant
		discPtr := C.go_component_valvariant_discriminant_ptr(varPtr)
		C.wasm_byte_vec_new_uninitialized(discPtr, C.size_t(len(variant.Discriminant)))
		if len(variant.Discriminant) > 0 {
			C.memcpy(unsafe.Pointer(discPtr.data), unsafe.Pointer(C._GoStringPtr(variant.Discriminant)), C.size_t(len(variant.Discriminant)))
			runtime.KeepAlive(variant.Discriminant)
		}

		// Set payload: heap-allocate via wasmtime_component_val_new if non-nil
		if variant.Val != nil {
			var tmp C.wasmtime_component_val_t
			componentValToC(variant.Val, &tmp)
			heapVal := C.wasmtime_component_val_new(&tmp)
			C.go_component_valvariant_val_set(varPtr, heapVal)
		} else {
			C.go_component_valvariant_val_set(varPtr, nil)
		}

	case ComponentValKindEnum:
		s := src.val.(string)
		enumPtr := C.go_component_val_enum_ptr(dst)
		C.wasm_byte_vec_new_uninitialized(enumPtr, C.size_t(len(s)))
		if len(s) > 0 {
			C.memcpy(unsafe.Pointer(enumPtr.data), unsafe.Pointer(C._GoStringPtr(s)), C.size_t(len(s)))
			runtime.KeepAlive(s)
		}

	case ComponentValKindOption:
		optVal := src.val.(*ComponentVal)
		if optVal != nil {
			var tmp C.wasmtime_component_val_t
			componentValToC(optVal, &tmp)
			heapVal := C.wasmtime_component_val_new(&tmp)
			C.go_component_val_option_set(dst, heapVal)
		} else {
			C.go_component_val_option_set(dst, nil)
		}

	case ComponentValKindResult:
		result := src.val.(ComponentResult)
		resPtr := C.go_component_val_result_ptr(dst)
		C.go_component_valresult_set_is_ok(resPtr, C.bool(result.IsOk))
		if result.Val != nil {
			var tmp C.wasmtime_component_val_t
			componentValToC(result.Val, &tmp)
			heapVal := C.wasmtime_component_val_new(&tmp)
			C.go_component_valresult_val_set(resPtr, heapVal)
		} else {
			C.go_component_valresult_val_set(resPtr, nil)
		}

	case ComponentValKindFlags:
		flags := src.val.([]string)
		flagsPtr := C.go_component_val_flags_ptr(dst)
		if len(flags) == 0 {
			C.wasmtime_component_valflags_new_empty(flagsPtr)
		} else {
			C.wasmtime_component_valflags_new_uninit(flagsPtr, C.size_t(len(flags)))
			for i, flag := range flags {
				namePtr := (*C.wasm_name_t)(unsafe.Pointer(
					uintptr(unsafe.Pointer(flagsPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasm_name_t{}),
				))
				C.wasm_byte_vec_new_uninitialized(namePtr, C.size_t(len(flag)))
				if len(flag) > 0 {
					C.memcpy(unsafe.Pointer(namePtr.data), unsafe.Pointer(C._GoStringPtr(flag)), C.size_t(len(flag)))
					runtime.KeepAlive(flag)
				}
			}
		}

	case ComponentValKindResource:
		ra := src.val.(*ComponentResourceAny)
		cloned := C.wasmtime_component_resource_any_clone(ra.ptr())
		runtime.KeepAlive(ra)
		C.go_component_val_resource_set(dst, cloned)

	default:
		panic("unknown component val kind in componentValToC")
	}
}

// --- Unmarshal C -> Go ---

// componentValFromC reads a C wasmtime_component_val_t and produces a Go ComponentVal.
// After this returns, the caller should call wasmtime_component_val_delete on src
// to free any C-owned memory (the Go side has copied everything it needs).
func componentValFromC(src *C.wasmtime_component_val_t) ComponentVal {
	kind := ComponentValKind(src.kind)

	switch kind {
	case ComponentValKindBool:
		return NewComponentValBool(bool(C.go_wasmtime_component_val_boolean_get(src)))

	case ComponentValKindS8:
		return NewComponentValS8(int8(C.go_wasmtime_component_val_s8_get(src)))

	case ComponentValKindU8:
		return NewComponentValU8(uint8(C.go_wasmtime_component_val_u8_get(src)))

	case ComponentValKindS16:
		return NewComponentValS16(int16(C.go_wasmtime_component_val_s16_get(src)))

	case ComponentValKindU16:
		return NewComponentValU16(uint16(C.go_wasmtime_component_val_u16_get(src)))

	case ComponentValKindS32:
		return NewComponentValS32(int32(C.go_wasmtime_component_val_s32_get(src)))

	case ComponentValKindU32:
		return NewComponentValU32(uint32(C.go_wasmtime_component_val_u32_get(src)))

	case ComponentValKindS64:
		return NewComponentValS64(int64(C.go_wasmtime_component_val_s64_get(src)))

	case ComponentValKindU64:
		return NewComponentValU64(uint64(C.go_wasmtime_component_val_u64_get(src)))

	case ComponentValKindF32:
		return NewComponentValF32(float32(C.go_wasmtime_component_val_f32_get(src)))

	case ComponentValKindF64:
		return NewComponentValF64(float64(C.go_wasmtime_component_val_f64_get(src)))

	case ComponentValKindChar:
		return NewComponentValChar(rune(C.go_wasmtime_component_val_character_get(src)))

	case ComponentValKindString:
		strPtr := C.go_component_val_string_ptr(src)
		goStr := C.GoStringN(strPtr.data, C.int(strPtr.size))
		return NewComponentValString(goStr)

	case ComponentValKindList:
		listPtr := C.go_component_val_list_ptr(src)
		count := int(listPtr.size)
		items := make([]ComponentVal, count)
		for i := 0; i < count; i++ {
			elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(listPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
			))
			items[i] = componentValFromC(elemPtr)
		}
		return ComponentVal{kind: ComponentValKindList, val: items}

	case ComponentValKindRecord:
		recPtr := C.go_component_val_record_ptr(src)
		count := int(recPtr.size)
		fields := make([]ComponentRecordField, count)
		for i := 0; i < count; i++ {
			entryPtr := (*C.wasmtime_component_valrecord_entry_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(recPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_valrecord_entry_t{}),
			))
			namePtr := C.go_component_valrecord_entry_name_ptr(entryPtr)
			name := C.GoStringN(namePtr.data, C.int(namePtr.size))
			valPtr := C.go_component_valrecord_entry_val_ptr(entryPtr)
			fields[i] = ComponentRecordField{
				Name: name,
				Val:  componentValFromC(valPtr),
			}
		}
		return ComponentVal{kind: ComponentValKindRecord, val: fields}

	case ComponentValKindTuple:
		tuplePtr := C.go_component_val_tuple_ptr(src)
		count := int(tuplePtr.size)
		items := make([]ComponentVal, count)
		for i := 0; i < count; i++ {
			elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(tuplePtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
			))
			items[i] = componentValFromC(elemPtr)
		}
		return ComponentVal{kind: ComponentValKindTuple, val: items}

	case ComponentValKindVariant:
		varPtr := C.go_component_val_variant_ptr(src)
		discPtr := C.go_component_valvariant_discriminant_ptr(varPtr)
		disc := C.GoStringN(discPtr.data, C.int(discPtr.size))

		payloadCPtr := C.go_component_valvariant_val_get(varPtr)
		var payload *ComponentVal
		if payloadCPtr != nil {
			v := componentValFromC(payloadCPtr)
			payload = &v
		}
		return ComponentVal{kind: ComponentValKindVariant, val: ComponentVariant{
			Discriminant: disc,
			Val:          payload,
		}}

	case ComponentValKindEnum:
		enumPtr := C.go_component_val_enum_ptr(src)
		goStr := C.GoStringN(enumPtr.data, C.int(enumPtr.size))
		return NewComponentValEnum(goStr)

	case ComponentValKindOption:
		optCPtr := C.go_component_val_option_get(src)
		if optCPtr != nil {
			v := componentValFromC(optCPtr)
			return ComponentVal{kind: ComponentValKindOption, val: &v}
		}
		return ComponentVal{kind: ComponentValKindOption, val: (*ComponentVal)(nil)}

	case ComponentValKindResult:
		resPtr := C.go_component_val_result_ptr(src)
		isOk := bool(C.go_component_valresult_is_ok(resPtr))
		valCPtr := C.go_component_valresult_val_get(resPtr)
		var payload *ComponentVal
		if valCPtr != nil {
			v := componentValFromC(valCPtr)
			payload = &v
		}
		return ComponentVal{kind: ComponentValKindResult, val: ComponentResult{
			IsOk: isOk,
			Val:  payload,
		}}

	case ComponentValKindFlags:
		flagsPtr := C.go_component_val_flags_ptr(src)
		count := int(flagsPtr.size)
		flags := make([]string, count)
		for i := 0; i < count; i++ {
			namePtr := (*C.wasm_name_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(flagsPtr.data)) + uintptr(i)*unsafe.Sizeof(C.wasm_name_t{}),
			))
			flags[i] = C.GoStringN(namePtr.data, C.int(namePtr.size))
		}
		return ComponentVal{kind: ComponentValKindFlags, val: flags}

	case ComponentValKindResource:
		resAny := C.go_component_val_resource_get(src)
		cloned := C.wasmtime_component_resource_any_clone(resAny)
		return ComponentVal{kind: ComponentValKindResource, val: mkComponentResourceAny(cloned)}

	default:
		panic("unknown component val kind in componentValFromC")
	}
}

// componentValsToC marshals a slice of Go ComponentVals into a C array.
// Returns the C pointer and count. The caller must free the C array by calling
// wasmtime_component_val_delete on each element, then C.free on the array pointer.
func componentValsToC(vals []ComponentVal) (*C.wasmtime_component_val_t, C.size_t) {
	if len(vals) == 0 {
		return nil, 0
	}
	size := C.size_t(len(vals))
	arrPtr := (*C.wasmtime_component_val_t)(C.malloc(size * C.size_t(unsafe.Sizeof(C.wasmtime_component_val_t{}))))

	for i := range vals {
		elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(arrPtr)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
		))
		componentValToC(&vals[i], elemPtr)
	}
	return arrPtr, size
}

// componentValsFromC unmarshals a C array of wasmtime_component_val_t into Go.
// After calling this, the caller should clean up each C val with
// wasmtime_component_val_delete, then C.free the array.
func componentValsFromC(ptr *C.wasmtime_component_val_t, count C.size_t) []ComponentVal {
	n := int(count)
	if n == 0 {
		return nil
	}
	result := make([]ComponentVal, n)
	for i := 0; i < n; i++ {
		elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(ptr)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
		))
		result[i] = componentValFromC(elemPtr)
	}
	return result
}

// freeComponentValsCArray cleans up a C array allocated by componentValsToC.
func freeComponentValsCArray(ptr *C.wasmtime_component_val_t, count C.size_t) {
	n := int(count)
	for i := 0; i < n; i++ {
		elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(ptr)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
		))
		C.wasmtime_component_val_delete(elemPtr)
	}
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

package wasmtime

// #include <wasmtime/component.h>
// #include "shims.h"
import "C"

import "runtime"

// ComponentItemKind represents the kind of an import/export item in a component.
type ComponentItemKind C.wasmtime_component_item_kind_t

const (
	// ComponentItemKindComponent indicates the item is a component.
	ComponentItemKindComponent ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_COMPONENT
	// ComponentItemKindComponentInstance indicates the item is a component instance.
	ComponentItemKindComponentInstance ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_COMPONENT_INSTANCE
	// ComponentItemKindModule indicates the item is a core wasm module.
	ComponentItemKindModule ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_MODULE
	// ComponentItemKindComponentFunc indicates the item is a component function.
	ComponentItemKindComponentFunc ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_COMPONENT_FUNC
	// ComponentItemKindResource indicates the item is a resource.
	ComponentItemKindResource ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_RESOURCE
	// ComponentItemKindCoreFunc indicates the item is a core function.
	ComponentItemKindCoreFunc ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_CORE_FUNC
	// ComponentItemKindType indicates the item is a type.
	ComponentItemKindType ComponentItemKind = C.WASMTIME_COMPONENT_ITEM_TYPE
)

// ComponentType represents the type of a WebAssembly component.
type ComponentType struct {
	_ptr *C.wasmtime_component_type_t
}

func mkComponentType(ptr *C.wasmtime_component_type_t) *ComponentType {
	ty := &ComponentType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentType) ptr() *C.wasmtime_component_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_type_delete(ty._ptr)
	ty._ptr = nil
}

// Type returns the type of this component.
func (c *Component) Type() *ComponentType {
	ptr := C.wasmtime_component_type(c.ptr())
	runtime.KeepAlive(c)
	return mkComponentType(ptr)
}

// ImportCount returns the number of imports of this component type.
func (ty *ComponentType) ImportCount(engine *Engine) int {
	ret := C.wasmtime_component_type_import_count(ty.ptr(), engine.ptr())
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	return int(ret)
}

// ImportGet retrieves the import with the specified name.
//
// Returns nil if no import with that name exists.
func (ty *ComponentType) ImportGet(engine *Engine, name string) *ComponentItem {
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_type_import_get(
		ty.ptr(),
		engine.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(name)
	if !ok {
		return nil
	}
	return mkComponentItem(&ret)
}

// ImportNth retrieves the nth import, returning its name and item.
//
// Returns "", nil if the index is out of bounds.
func (ty *ComponentType) ImportNth(engine *Engine, nth int) (string, *ComponentItem) {
	var namePtr *C.char
	var nameLen C.size_t
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_type_import_nth(
		ty.ptr(),
		engine.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	if !ok {
		return "", nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	return name, mkComponentItem(&ret)
}

// ExportCount returns the number of exports of this component type.
func (ty *ComponentType) ExportCount(engine *Engine) int {
	ret := C.wasmtime_component_type_export_count(ty.ptr(), engine.ptr())
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	return int(ret)
}

// ExportGet retrieves the export with the specified name.
//
// Returns nil if no export with that name exists.
func (ty *ComponentType) ExportGet(engine *Engine, name string) *ComponentItem {
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_type_export_get(
		ty.ptr(),
		engine.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(name)
	if !ok {
		return nil
	}
	return mkComponentItem(&ret)
}

// ExportNth retrieves the nth export, returning its name and item.
//
// Returns "", nil if the index is out of bounds.
func (ty *ComponentType) ExportNth(engine *Engine, nth int) (string, *ComponentItem) {
	var namePtr *C.char
	var nameLen C.size_t
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_type_export_nth(
		ty.ptr(),
		engine.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	if !ok {
		return "", nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	return name, mkComponentItem(&ret)
}

// ComponentItem represents a single item in a component's import or export list.
//
// This is a discriminated union; use Kind() to determine the variant, then
// call the corresponding accessor method.
type ComponentItem struct {
	val    C.wasmtime_component_item_t
	closed bool
}

func mkComponentItem(ptr *C.wasmtime_component_item_t) *ComponentItem {
	// Copy the value so the caller's stack allocation can be freed
	item := &ComponentItem{val: *ptr}
	runtime.SetFinalizer(item, func(item *ComponentItem) {
		item.Close()
	})
	return item
}

// Close will deallocate this item's owned resources.
func (item *ComponentItem) Close() {
	if item.closed {
		return
	}
	item.closed = true
	runtime.SetFinalizer(item, nil)
	C.wasmtime_component_item_delete(&item.val)
}

// Kind returns the discriminant of this item.
func (item *ComponentItem) Kind() ComponentItemKind {
	return ComponentItemKind(item.val.kind)
}

// Component returns the component type if Kind() is ComponentItemKindComponent.
// The returned type is owned by the caller.
func (item *ComponentItem) Component() *ComponentType {
	ptr := C.go_component_item_component_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkComponentType(C.wasmtime_component_type_clone(ptr))
	runtime.KeepAlive(item)
	return ret
}

// ComponentInstance returns the instance type if Kind() is ComponentItemKindComponentInstance.
// The returned type is owned by the caller.
func (item *ComponentItem) ComponentInstance() *ComponentInstanceType {
	ptr := C.go_component_item_component_instance_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkComponentInstanceType(C.wasmtime_component_instance_type_clone(ptr))
	runtime.KeepAlive(item)
	return ret
}

// Module returns the module type if Kind() is ComponentItemKindModule.
// The returned type is owned by the caller.
func (item *ComponentItem) Module() *ComponentModuleType {
	ptr := C.go_component_item_module_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkComponentModuleType(C.wasmtime_module_type_clone(ptr))
	runtime.KeepAlive(item)
	return ret
}

// ComponentFunc returns the function type if Kind() is ComponentItemKindComponentFunc.
// The returned type is owned by the caller.
func (item *ComponentItem) ComponentFunc() *ComponentFuncType {
	ptr := C.go_component_item_component_func_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkComponentFuncType(C.wasmtime_component_func_type_clone(ptr))
	runtime.KeepAlive(item)
	return ret
}

// Resource returns the resource type if Kind() is ComponentItemKindResource.
// The returned type is owned by the caller.
func (item *ComponentItem) Resource() *ComponentResourceType {
	ptr := C.go_component_item_resource_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkComponentResourceType(C.wasmtime_component_resource_type_clone(ptr))
	runtime.KeepAlive(item)
	return ret
}

// CoreFunc returns the core function type if Kind() is ComponentItemKindCoreFunc.
// The returned type is owned by the caller.
func (item *ComponentItem) CoreFunc() *FuncType {
	ptr := C.go_component_item_core_func_get(&item.val)
	if ptr == nil {
		runtime.KeepAlive(item)
		return nil
	}
	ret := mkFuncType(C.wasm_functype_copy(ptr), nil)
	runtime.KeepAlive(item)
	return ret
}

// ValType returns the value type if Kind() is ComponentItemKindType.
func (item *ComponentItem) ValType() *ComponentValType {
	valtypePtr := C.go_component_item_type_ptr(&item.val)
	var ret C.wasmtime_component_valtype_t
	C.wasmtime_component_valtype_clone(valtypePtr, &ret)
	runtime.KeepAlive(item)
	return mkComponentValType(&ret)
}

// ComponentFuncType represents the type of a component function.
type ComponentFuncType struct {
	_ptr *C.wasmtime_component_func_type_t
}

func mkComponentFuncType(ptr *C.wasmtime_component_func_type_t) *ComponentFuncType {
	ty := &ComponentFuncType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentFuncType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentFuncType) ptr() *C.wasmtime_component_func_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentFuncType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_func_type_delete(ty._ptr)
	ty._ptr = nil
}

// Type returns the type of this component function.
func (f *ComponentFunc) Type(store Storelike) *ComponentFuncType {
	ptr := C.wasmtime_component_func_type(&f.val, store.Context())
	runtime.KeepAlive(f)
	runtime.KeepAlive(store)
	return mkComponentFuncType(ptr)
}

// IsAsync returns whether this is an async function.
func (ty *ComponentFuncType) IsAsync() bool {
	ret := C.wasmtime_component_func_type_async(ty.ptr())
	runtime.KeepAlive(ty)
	return bool(ret)
}

// ParamCount returns the number of parameters of this function type.
func (ty *ComponentFuncType) ParamCount() int {
	ret := C.wasmtime_component_func_type_param_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// ParamNth retrieves the nth parameter, returning its name and type.
//
// Returns "", nil if the index is out of bounds.
func (ty *ComponentFuncType) ParamNth(nth int) (string, *ComponentValType) {
	var namePtr *C.char
	var nameLen C.size_t
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_func_type_param_nth(
		ty.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&ret,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return "", nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	return name, mkComponentValType(&ret)
}

// Result retrieves the result type of this function, if any.
//
// Returns nil if this function has no result type.
func (ty *ComponentFuncType) Result() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_func_type_result(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}

// ResultArity returns the number of results (0 or 1) from this function type.
// Component model functions return either zero or one result (the result itself
// may be a composite type like a tuple or record).
func (ty *ComponentFuncType) ResultArity() int {
	var dummy C.wasmtime_component_valtype_t
	hasResult := bool(C.wasmtime_component_func_type_result(ty.ptr(), &dummy))
	runtime.KeepAlive(ty)
	if hasResult {
		C.wasmtime_component_valtype_delete(&dummy)
		return 1
	}
	return 0
}

// ComponentInstanceType represents the type of a component instance.
type ComponentInstanceType struct {
	_ptr *C.wasmtime_component_instance_type_t
}

func mkComponentInstanceType(ptr *C.wasmtime_component_instance_type_t) *ComponentInstanceType {
	ty := &ComponentInstanceType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentInstanceType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentInstanceType) ptr() *C.wasmtime_component_instance_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentInstanceType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_instance_type_delete(ty._ptr)
	ty._ptr = nil
}

// ExportCount returns the number of exports of this instance type.
func (ty *ComponentInstanceType) ExportCount(engine *Engine) int {
	ret := C.wasmtime_component_instance_type_export_count(ty.ptr(), engine.ptr())
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	return int(ret)
}

// ExportGet retrieves the export with the specified name.
//
// Returns nil if no export with that name exists.
func (ty *ComponentInstanceType) ExportGet(engine *Engine, name string) *ComponentItem {
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_instance_type_export_get(
		ty.ptr(),
		engine.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(name)
	if !ok {
		return nil
	}
	return mkComponentItem(&ret)
}

// ExportNth retrieves the nth export, returning its name and item.
//
// Returns "", nil if the index is out of bounds.
func (ty *ComponentInstanceType) ExportNth(engine *Engine, nth int) (string, *ComponentItem) {
	var namePtr *C.char
	var nameLen C.size_t
	var ret C.wasmtime_component_item_t
	ok := C.wasmtime_component_instance_type_export_nth(
		ty.ptr(),
		engine.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&ret,
	)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	if !ok {
		return "", nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	return name, mkComponentItem(&ret)
}

// ComponentModuleType represents the type of a core wasm module within a component.
type ComponentModuleType struct {
	_ptr *C.wasmtime_module_type_t
}

func mkComponentModuleType(ptr *C.wasmtime_module_type_t) *ComponentModuleType {
	ty := &ComponentModuleType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentModuleType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentModuleType) ptr() *C.wasmtime_module_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentModuleType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_module_type_delete(ty._ptr)
	ty._ptr = nil
}

// ImportCount returns the number of imports of this module type.
func (ty *ComponentModuleType) ImportCount(engine *Engine) int {
	ret := C.wasmtime_module_type_import_count(ty.ptr(), engine.ptr())
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	return int(ret)
}

// ImportNth retrieves the nth import type.
//
// Returns nil if the index is out of bounds.
func (ty *ComponentModuleType) ImportNth(engine *Engine, nth int) *ImportType {
	ptr := C.wasmtime_module_type_import_nth(ty.ptr(), engine.ptr(), C.size_t(nth))
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	if ptr == nil {
		return nil
	}
	return mkImportType(ptr, nil)
}

// ExportCount returns the number of exports of this module type.
func (ty *ComponentModuleType) ExportCount(engine *Engine) int {
	ret := C.wasmtime_module_type_export_count(ty.ptr(), engine.ptr())
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	return int(ret)
}

// ExportNth retrieves the nth export type.
//
// Returns nil if the index is out of bounds.
func (ty *ComponentModuleType) ExportNth(engine *Engine, nth int) *ExportType {
	ptr := C.wasmtime_module_type_export_nth(ty.ptr(), engine.ptr(), C.size_t(nth))
	runtime.KeepAlive(ty)
	runtime.KeepAlive(engine)
	if ptr == nil {
		return nil
	}
	return mkExportType(ptr, nil)
}

// ComponentValTypeKind represents the kind of a component value type.
type ComponentValTypeKind C.wasmtime_component_valtype_kind_t

const (
	ComponentValTypeKindBool         ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_BOOL
	ComponentValTypeKindS8           ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_S8
	ComponentValTypeKindS16          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_S16
	ComponentValTypeKindS32          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_S32
	ComponentValTypeKindS64          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_S64
	ComponentValTypeKindU8           ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_U8
	ComponentValTypeKindU16          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_U16
	ComponentValTypeKindU32          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_U32
	ComponentValTypeKindU64          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_U64
	ComponentValTypeKindF32          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_F32
	ComponentValTypeKindF64          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_F64
	ComponentValTypeKindChar         ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_CHAR
	ComponentValTypeKindString       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_STRING
	ComponentValTypeKindList         ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_LIST
	ComponentValTypeKindRecord       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_RECORD
	ComponentValTypeKindTuple        ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_TUPLE
	ComponentValTypeKindVariant      ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_VARIANT
	ComponentValTypeKindEnum         ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_ENUM
	ComponentValTypeKindOption       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_OPTION
	ComponentValTypeKindResult       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_RESULT
	ComponentValTypeKindFlags        ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_FLAGS
	ComponentValTypeKindOwn          ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_OWN
	ComponentValTypeKindBorrow       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_BORROW
	ComponentValTypeKindFuture       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_FUTURE
	ComponentValTypeKindStream       ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_STREAM
	ComponentValTypeKindErrorContext ComponentValTypeKind = C.WASMTIME_COMPONENT_VALTYPE_ERROR_CONTEXT
)

// ComponentValType represents a single value type in the component model.
//
// This is a discriminated union; use Kind() to determine the variant.
// For primitive types (bool, integers, floats, char, string, error_context),
// Kind() is sufficient. For composite types, use the corresponding accessor.
type ComponentValType struct {
	val    C.wasmtime_component_valtype_t
	closed bool
}

func mkComponentValType(ptr *C.wasmtime_component_valtype_t) *ComponentValType {
	vt := &ComponentValType{val: *ptr}
	runtime.SetFinalizer(vt, func(vt *ComponentValType) {
		vt.Close()
	})
	return vt
}

// Close will deallocate this value type's owned resources.
func (vt *ComponentValType) Close() {
	if vt.closed {
		return
	}
	vt.closed = true
	runtime.SetFinalizer(vt, nil)
	C.wasmtime_component_valtype_delete(&vt.val)
}

// Kind returns the discriminant of this value type.
func (vt *ComponentValType) Kind() ComponentValTypeKind {
	return ComponentValTypeKind(vt.val.kind)
}

// Equal returns whether two value types are the same.
func (vt *ComponentValType) Equal(other *ComponentValType) bool {
	ret := C.wasmtime_component_valtype_equal(&vt.val, &other.val)
	runtime.KeepAlive(vt)
	runtime.KeepAlive(other)
	return bool(ret)
}

// List returns the list type if Kind() is ComponentValTypeKindList.
// The returned type is owned by the caller.
func (vt *ComponentValType) List() *ComponentListType {
	ptr := C.go_component_valtype_list_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentListType(C.wasmtime_component_list_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Record returns the record type if Kind() is ComponentValTypeKindRecord.
// The returned type is owned by the caller.
func (vt *ComponentValType) Record() *ComponentRecordType {
	ptr := C.go_component_valtype_record_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentRecordType(C.wasmtime_component_record_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Tuple returns the tuple type if Kind() is ComponentValTypeKindTuple.
// The returned type is owned by the caller.
func (vt *ComponentValType) Tuple() *ComponentTupleType {
	ptr := C.go_component_valtype_tuple_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentTupleType(C.wasmtime_component_tuple_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Variant returns the variant type if Kind() is ComponentValTypeKindVariant.
// The returned type is owned by the caller.
func (vt *ComponentValType) Variant() *ComponentVariantType {
	ptr := C.go_component_valtype_variant_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentVariantType(C.wasmtime_component_variant_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Enum returns the enum type if Kind() is ComponentValTypeKindEnum.
// The returned type is owned by the caller.
func (vt *ComponentValType) Enum() *ComponentEnumType {
	ptr := C.go_component_valtype_enum_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentEnumType(C.wasmtime_component_enum_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Option returns the option type if Kind() is ComponentValTypeKindOption.
// The returned type is owned by the caller.
func (vt *ComponentValType) Option() *ComponentOptionType {
	ptr := C.go_component_valtype_option_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentOptionType(C.wasmtime_component_option_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Result returns the result type if Kind() is ComponentValTypeKindResult.
// The returned type is owned by the caller.
func (vt *ComponentValType) Result() *ComponentResultType {
	ptr := C.go_component_valtype_result_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentResultType(C.wasmtime_component_result_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Flags returns the flags type if Kind() is ComponentValTypeKindFlags.
// The returned type is owned by the caller.
func (vt *ComponentValType) Flags() *ComponentFlagsType {
	ptr := C.go_component_valtype_flags_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentFlagsType(C.wasmtime_component_flags_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Own returns the resource type if Kind() is ComponentValTypeKindOwn.
// The returned type is owned by the caller.
func (vt *ComponentValType) Own() *ComponentResourceType {
	ptr := C.go_component_valtype_own_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentResourceType(C.wasmtime_component_resource_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Borrow returns the resource type if Kind() is ComponentValTypeKindBorrow.
// The returned type is owned by the caller.
func (vt *ComponentValType) Borrow() *ComponentResourceType {
	ptr := C.go_component_valtype_borrow_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentResourceType(C.wasmtime_component_resource_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Future returns the future type if Kind() is ComponentValTypeKindFuture.
// The returned type is owned by the caller.
func (vt *ComponentValType) Future() *ComponentFutureType {
	ptr := C.go_component_valtype_future_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentFutureType(C.wasmtime_component_future_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// Stream returns the stream type if Kind() is ComponentValTypeKindStream.
// The returned type is owned by the caller.
func (vt *ComponentValType) Stream() *ComponentStreamType {
	ptr := C.go_component_valtype_stream_get(&vt.val)
	if ptr == nil {
		runtime.KeepAlive(vt)
		return nil
	}
	ret := mkComponentStreamType(C.wasmtime_component_stream_type_clone(ptr))
	runtime.KeepAlive(vt)
	return ret
}

// ComponentListType represents a component list type.
type ComponentListType struct {
	_ptr *C.wasmtime_component_list_type_t
}

func mkComponentListType(ptr *C.wasmtime_component_list_type_t) *ComponentListType {
	ty := &ComponentListType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentListType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentListType) ptr() *C.wasmtime_component_list_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentListType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_list_type_delete(ty._ptr)
	ty._ptr = nil
}

// Element returns the element type of this list type.
func (ty *ComponentListType) Element() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	C.wasmtime_component_list_type_element(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	return mkComponentValType(&ret)
}

// ComponentRecordType represents a component record type.
type ComponentRecordType struct {
	_ptr *C.wasmtime_component_record_type_t
}

func mkComponentRecordType(ptr *C.wasmtime_component_record_type_t) *ComponentRecordType {
	ty := &ComponentRecordType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentRecordType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentRecordType) ptr() *C.wasmtime_component_record_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentRecordType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_record_type_delete(ty._ptr)
	ty._ptr = nil
}

// FieldCount returns the number of fields in this record type.
func (ty *ComponentRecordType) FieldCount() int {
	ret := C.wasmtime_component_record_type_field_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// FieldNth retrieves the nth field, returning its name and type.
//
// Returns "", nil if the index is out of bounds.
func (ty *ComponentRecordType) FieldNth(nth int) (string, *ComponentValType) {
	var namePtr *C.char
	var nameLen C.size_t
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_record_type_field_nth(
		ty.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&ret,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return "", nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	return name, mkComponentValType(&ret)
}

// ComponentTupleType represents a component tuple type.
type ComponentTupleType struct {
	_ptr *C.wasmtime_component_tuple_type_t
}

func mkComponentTupleType(ptr *C.wasmtime_component_tuple_type_t) *ComponentTupleType {
	ty := &ComponentTupleType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentTupleType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentTupleType) ptr() *C.wasmtime_component_tuple_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentTupleType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_tuple_type_delete(ty._ptr)
	ty._ptr = nil
}

// TypesCount returns the number of types in this tuple type.
func (ty *ComponentTupleType) TypesCount() int {
	ret := C.wasmtime_component_tuple_type_types_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// TypesNth retrieves the nth type in this tuple type.
//
// Returns nil if the index is out of bounds.
func (ty *ComponentTupleType) TypesNth(nth int) *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_tuple_type_types_nth(
		ty.ptr(),
		C.size_t(nth),
		&ret,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}

// ComponentVariantType represents a component variant type.
type ComponentVariantType struct {
	_ptr *C.wasmtime_component_variant_type_t
}

func mkComponentVariantType(ptr *C.wasmtime_component_variant_type_t) *ComponentVariantType {
	ty := &ComponentVariantType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentVariantType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentVariantType) ptr() *C.wasmtime_component_variant_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentVariantType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_variant_type_delete(ty._ptr)
	ty._ptr = nil
}

// CaseCount returns the number of cases in this variant type.
func (ty *ComponentVariantType) CaseCount() int {
	ret := C.wasmtime_component_variant_type_case_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// CaseNth retrieves the nth case, returning its name, whether it has a
// payload, and the payload type (if any).
//
// Returns "", false, nil if the index is out of bounds.
func (ty *ComponentVariantType) CaseNth(nth int) (string, bool, *ComponentValType) {
	var namePtr *C.char
	var nameLen C.size_t
	var hasPayload C.bool
	var payloadRet C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_variant_type_case_nth(
		ty.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
		&hasPayload,
		&payloadRet,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return "", false, nil
	}
	name := C.GoStringN(namePtr, C.int(nameLen))
	if bool(hasPayload) {
		return name, true, mkComponentValType(&payloadRet)
	}
	return name, false, nil
}

// ComponentEnumType represents a component enum type.
type ComponentEnumType struct {
	_ptr *C.wasmtime_component_enum_type_t
}

func mkComponentEnumType(ptr *C.wasmtime_component_enum_type_t) *ComponentEnumType {
	ty := &ComponentEnumType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentEnumType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentEnumType) ptr() *C.wasmtime_component_enum_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentEnumType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_enum_type_delete(ty._ptr)
	ty._ptr = nil
}

// NamesCount returns the number of names in this enum type.
func (ty *ComponentEnumType) NamesCount() int {
	ret := C.wasmtime_component_enum_type_names_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// NamesNth retrieves the nth name in this enum type.
//
// Returns "" if the index is out of bounds.
func (ty *ComponentEnumType) NamesNth(nth int) string {
	var namePtr *C.char
	var nameLen C.size_t
	ok := C.wasmtime_component_enum_type_names_nth(
		ty.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return ""
	}
	return C.GoStringN(namePtr, C.int(nameLen))
}

// ComponentOptionType represents a component option type.
type ComponentOptionType struct {
	_ptr *C.wasmtime_component_option_type_t
}

func mkComponentOptionType(ptr *C.wasmtime_component_option_type_t) *ComponentOptionType {
	ty := &ComponentOptionType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentOptionType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentOptionType) ptr() *C.wasmtime_component_option_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentOptionType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_option_type_delete(ty._ptr)
	ty._ptr = nil
}

// Inner returns the inner type of this option type.
func (ty *ComponentOptionType) Inner() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	C.wasmtime_component_option_type_ty(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	return mkComponentValType(&ret)
}

// ComponentResultType represents a component result type.
type ComponentResultType struct {
	_ptr *C.wasmtime_component_result_type_t
}

func mkComponentResultType(ptr *C.wasmtime_component_result_type_t) *ComponentResultType {
	ty := &ComponentResultType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentResultType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentResultType) ptr() *C.wasmtime_component_result_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentResultType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_result_type_delete(ty._ptr)
	ty._ptr = nil
}

// Ok returns the ok type of this result, if it has one.
//
// Returns nil if there is no ok type.
func (ty *ComponentResultType) Ok() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_result_type_ok(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}

// Err returns the error type of this result, if it has one.
//
// Returns nil if there is no error type.
func (ty *ComponentResultType) Err() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_result_type_err(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}

// ComponentFlagsType represents a component flags type.
type ComponentFlagsType struct {
	_ptr *C.wasmtime_component_flags_type_t
}

func mkComponentFlagsType(ptr *C.wasmtime_component_flags_type_t) *ComponentFlagsType {
	ty := &ComponentFlagsType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentFlagsType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentFlagsType) ptr() *C.wasmtime_component_flags_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentFlagsType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_flags_type_delete(ty._ptr)
	ty._ptr = nil
}

// NamesCount returns the number of flag names.
func (ty *ComponentFlagsType) NamesCount() int {
	ret := C.wasmtime_component_flags_type_names_count(ty.ptr())
	runtime.KeepAlive(ty)
	return int(ret)
}

// NamesNth retrieves the nth flag name.
//
// Returns "" if the index is out of bounds.
func (ty *ComponentFlagsType) NamesNth(nth int) string {
	var namePtr *C.char
	var nameLen C.size_t
	ok := C.wasmtime_component_flags_type_names_nth(
		ty.ptr(),
		C.size_t(nth),
		&namePtr,
		&nameLen,
	)
	runtime.KeepAlive(ty)
	if !ok {
		return ""
	}
	return C.GoStringN(namePtr, C.int(nameLen))
}

// ComponentFutureType represents a component future type.
type ComponentFutureType struct {
	_ptr *C.wasmtime_component_future_type_t
}

func mkComponentFutureType(ptr *C.wasmtime_component_future_type_t) *ComponentFutureType {
	ty := &ComponentFutureType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentFutureType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentFutureType) ptr() *C.wasmtime_component_future_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentFutureType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_future_type_delete(ty._ptr)
	ty._ptr = nil
}

// Inner returns the inner type of this future type, if it has one.
//
// Returns nil if there is no inner type.
func (ty *ComponentFutureType) Inner() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_future_type_ty(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}

// ComponentStreamType represents a component stream type.
type ComponentStreamType struct {
	_ptr *C.wasmtime_component_stream_type_t
}

func mkComponentStreamType(ptr *C.wasmtime_component_stream_type_t) *ComponentStreamType {
	ty := &ComponentStreamType{_ptr: ptr}
	runtime.SetFinalizer(ty, func(ty *ComponentStreamType) {
		ty.Close()
	})
	return ty
}

func (ty *ComponentStreamType) ptr() *C.wasmtime_component_stream_type_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this type's state explicitly.
func (ty *ComponentStreamType) Close() {
	if ty._ptr == nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasmtime_component_stream_type_delete(ty._ptr)
	ty._ptr = nil
}

// Inner returns the inner type of this stream type, if it has one.
//
// Returns nil if there is no inner type.
func (ty *ComponentStreamType) Inner() *ComponentValType {
	var ret C.wasmtime_component_valtype_t
	ok := C.wasmtime_component_stream_type_ty(ty.ptr(), &ret)
	runtime.KeepAlive(ty)
	if !ok {
		return nil
	}
	return mkComponentValType(&ret)
}


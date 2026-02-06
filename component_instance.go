package wasmtime

// #include <wasmtime/component.h>
import "C"

import "runtime"

// ComponentInstance represents an instantiated component in the Component Model.
//
// Instances are represented with a small value type internally and do not have
// any destructor associated with them. Instances cannot interoperate between
// Store instances.
type ComponentInstance struct {
	val C.wasmtime_component_instance_t
}

func mkComponentInstance(val C.wasmtime_component_instance_t) *ComponentInstance {
	return &ComponentInstance{val}
}

// GetExportIndex looks up a specific export of this instance by `name`,
// optionally nested within the `instanceIdx` provided.
//
// The `instanceIdx` parameter may be nil to look up a top-level export.
// Returns nil if the export is not found.
func (i *ComponentInstance) GetExportIndex(store Storelike, instanceIdx *ComponentExportIndex, name string) *ComponentExportIndex {
	var instancePtr *C.wasmtime_component_export_index_t
	if instanceIdx != nil {
		instancePtr = instanceIdx.ptr()
	}
	ptr := C.wasmtime_component_instance_get_export_index(
		&i.val,
		store.Context(),
		instancePtr,
		C._GoStringPtr(name),
		C._GoStringLen(name),
	)
	runtime.KeepAlive(i)
	runtime.KeepAlive(store)
	runtime.KeepAlive(instanceIdx)
	runtime.KeepAlive(name)
	if ptr == nil {
		return nil
	}
	return mkComponentExportIndex(ptr)
}

// GetFunc looks up an exported function by its export index within this instance.
//
// Returns nil if no function is found for the given export index.
func (i *ComponentInstance) GetFunc(store Storelike, exportIdx *ComponentExportIndex) *ComponentFunc {
	var ret C.wasmtime_component_func_t
	ok := C.wasmtime_component_instance_get_func(
		&i.val,
		store.Context(),
		exportIdx.ptr(),
		&ret,
	)
	runtime.KeepAlive(i)
	runtime.KeepAlive(store)
	runtime.KeepAlive(exportIdx)
	if ok {
		return mkComponentFunc(ret)
	}
	return nil
}

// ComponentFunc represents a function within a component instance.
//
// Functions in the Component Model are represented as a small value type
// (an index into a store) and don't have any destructor associated with them.
type ComponentFunc struct {
	val C.wasmtime_component_func_t
}

func mkComponentFunc(val C.wasmtime_component_func_t) *ComponentFunc {
	return &ComponentFunc{val}
}

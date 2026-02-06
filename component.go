package wasmtime

// #include <wasmtime/component.h>
// #include <stdlib.h>
import "C"

import (
	"os"
	"runtime"
	"unsafe"
)

// Component represents a compiled component in the WebAssembly Component Model.
//
// A component is analogous to a Module but for the Component Model. It can
// declare imports and exports and is the unit of deployment and compilation
// for the Component Model.
type Component struct {
	_ptr *C.wasmtime_component_t
}

// NewComponent compiles a new `Component` from the `wasm` provided with the
// given configuration in `engine`.
//
// The `wasm` parameter must be a valid component binary, such as what is
// produced by the Rust `cargo component` tooling.
func NewComponent(engine *Engine, wasm []byte) (*Component, error) {
	var wasmPtr *C.uint8_t
	if len(wasm) > 0 {
		wasmPtr = (*C.uint8_t)(unsafe.Pointer(&wasm[0]))
	}
	var ptr *C.wasmtime_component_t
	err := C.wasmtime_component_new(engine.ptr(), wasmPtr, C.size_t(len(wasm)), &ptr)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(wasm)

	if err != nil {
		return nil, mkError(err)
	}

	return mkComponent(ptr), nil
}

// NewComponentFromFile reads the contents of the `file` provided and interprets
// them as a WebAssembly component binary.
//
// Afterwards delegates to the `NewComponent` constructor with the contents read.
func NewComponentFromFile(engine *Engine, file string) (*Component, error) {
	wasm, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return NewComponent(engine, wasm)
}

func mkComponent(ptr *C.wasmtime_component_t) *Component {
	component := &Component{_ptr: ptr}
	runtime.SetFinalizer(component, func(component *Component) {
		component.Close()
	})
	return component
}

func (c *Component) ptr() *C.wasmtime_component_t {
	ret := c._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this component's state explicitly.
//
// For more information see the documentation for engine.Close()
func (c *Component) Close() {
	if c._ptr == nil {
		return
	}
	runtime.SetFinalizer(c, nil)
	C.wasmtime_component_delete(c._ptr)
	c._ptr = nil
}

// Serialize will convert this in-memory compiled component into a list of bytes.
//
// The purpose of this method is to extract an artifact which can be stored
// elsewhere from this `Component`. The returned bytes can, for example, be
// stored on disk or in an object store. The `NewComponentDeserialize` function
// can be used to deserialize the returned bytes at a later date to get the
// component back.
func (c *Component) Serialize() ([]byte, error) {
	retVec := C.wasm_byte_vec_t{}
	err := C.wasmtime_component_serialize(c.ptr(), &retVec)
	runtime.KeepAlive(c)

	if err != nil {
		return nil, mkError(err)
	}
	ret := C.GoBytes(unsafe.Pointer(retVec.data), C.int(retVec.size))
	C.wasm_byte_vec_delete(&retVec)
	return ret, nil
}

// NewComponentDeserialize decodes and deserializes in-memory bytes previously
// produced by `component.Serialize()`.
//
// This function does not take a WebAssembly binary as input. It takes
// as input the results of a previous call to `Serialize()`, and only takes
// that as input.
//
// If deserialization is successful then a compiled component is returned,
// otherwise nil and an error are returned.
//
// Note that to deserialize successfully the bytes provided must have been
// produced with an `Engine` that has the same compilation options as the
// provided engine, and from the same version of this library.
func NewComponentDeserialize(engine *Engine, encoded []byte) (*Component, error) {
	var encodedPtr *C.uint8_t
	var ptr *C.wasmtime_component_t
	if len(encoded) > 0 {
		encodedPtr = (*C.uint8_t)(unsafe.Pointer(&encoded[0]))
	}
	err := C.wasmtime_component_deserialize(
		engine.ptr(),
		encodedPtr,
		C.size_t(len(encoded)),
		&ptr,
	)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(encoded)

	if err != nil {
		return nil, mkError(err)
	}

	return mkComponent(ptr), nil
}

// NewComponentDeserializeFile is the same as `NewComponentDeserialize` except
// that the bytes are read from a file instead of provided as an argument.
func NewComponentDeserializeFile(engine *Engine, path string) (*Component, error) {
	cs := C.CString(path)
	var ptr *C.wasmtime_component_t
	err := C.wasmtime_component_deserialize_file(engine.ptr(), cs, &ptr)
	runtime.KeepAlive(engine)
	C.free(unsafe.Pointer(cs))

	if err != nil {
		return nil, mkError(err)
	}

	return mkComponent(ptr), nil
}

// ComponentExportIndex represents a known export of a component, used to
// efficiently look up exported functions on instances.
type ComponentExportIndex struct {
	_ptr *C.wasmtime_component_export_index_t
}

func mkComponentExportIndex(ptr *C.wasmtime_component_export_index_t) *ComponentExportIndex {
	idx := &ComponentExportIndex{_ptr: ptr}
	runtime.SetFinalizer(idx, func(idx *ComponentExportIndex) {
		idx.Close()
	})
	return idx
}

func (idx *ComponentExportIndex) ptr() *C.wasmtime_component_export_index_t {
	ret := idx._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this export index's state explicitly.
//
// For more information see the documentation for engine.Close()
func (idx *ComponentExportIndex) Close() {
	if idx._ptr == nil {
		return
	}
	runtime.SetFinalizer(idx, nil)
	C.wasmtime_component_export_index_delete(idx._ptr)
	idx._ptr = nil
}

// GetExportIndex looks up a specific export of this component by `name`,
// optionally nested within the `instanceIdx` provided.
//
// The `instanceIdx` parameter may be nil to look up a top-level export.
// Returns nil if the export is not found.
func (c *Component) GetExportIndex(instanceIdx *ComponentExportIndex, name string) *ComponentExportIndex {
	var instancePtr *C.wasmtime_component_export_index_t
	if instanceIdx != nil {
		instancePtr = instanceIdx.ptr()
	}
	ptr := C.wasmtime_component_get_export_index(
		c.ptr(),
		instancePtr,
		C._GoStringPtr(name),
		C._GoStringLen(name),
	)
	runtime.KeepAlive(c)
	runtime.KeepAlive(instanceIdx)
	runtime.KeepAlive(name)
	if ptr == nil {
		return nil
	}
	return mkComponentExportIndex(ptr)
}

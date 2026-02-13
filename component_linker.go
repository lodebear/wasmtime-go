package wasmtime

// #include <wasmtime/component.h>
// #include <wasmtime/async.h>
// #include "shims.h"
import "C"

import (
	"errors"
	"runtime"
)

// ComponentLinker implements linking for the WebAssembly Component Model,
// which can link component instances together and provide host function
// implementations.
type ComponentLinker struct {
	_ptr   *C.wasmtime_component_linker_t
	Engine *Engine
}

// NewComponentLinker creates a new `ComponentLinker` for the specified engine.
func NewComponentLinker(engine *Engine) *ComponentLinker {
	ptr := C.wasmtime_component_linker_new(engine.ptr())
	linker := &ComponentLinker{_ptr: ptr, Engine: engine}
	runtime.SetFinalizer(linker, func(linker *ComponentLinker) {
		linker.Close()
	})
	return linker
}

func (l *ComponentLinker) ptr() *C.wasmtime_component_linker_t {
	ret := l._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this linker's state explicitly.
//
// For more information see the documentation for engine.Close()
func (l *ComponentLinker) Close() {
	if l._ptr == nil {
		return
	}
	runtime.SetFinalizer(l, nil)
	C.wasmtime_component_linker_delete(l._ptr)
	l._ptr = nil
}

// AllowShadowing configures whether names can be redefined after they've
// already been defined in this linker.
func (l *ComponentLinker) AllowShadowing(allow bool) {
	C.wasmtime_component_linker_allow_shadowing(l.ptr(), C.bool(allow))
	runtime.KeepAlive(l)
}

// Root provides access to the "root instance" of this linker, used to define
// names into the root namespace.
//
// The callback `f` is invoked with the root `ComponentLinkerInstance`.
// The linker must not be used concurrently while the callback is executing,
// as the C API acquires exclusive access to the linker for the lifetime of the
// returned instance.
func (l *ComponentLinker) Root(f func(*ComponentLinkerInstance)) {
	ptr := C.wasmtime_component_linker_root(l.ptr())
	runtime.KeepAlive(l)
	instance := &ComponentLinkerInstance{_ptr: ptr}
	defer func() {
		C.wasmtime_component_linker_instance_delete(instance._ptr)
		instance._ptr = nil
	}()
	f(instance)
}

// Instantiate instantiates a component with all imports defined in this linker.
//
// Returns an error if the component's imports couldn't be satisfied or had the
// wrong types.
func (l *ComponentLinker) Instantiate(store Storelike, component *Component) (*ComponentInstance, error) {
	var ret C.wasmtime_component_instance_t
	err := C.wasmtime_component_linker_instantiate(
		l.ptr(),
		store.Context(),
		component.ptr(),
		&ret,
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(store)
	runtime.KeepAlive(component)
	if err != nil {
		return nil, mkError(err)
	}
	return mkComponentInstance(ret), nil
}

// InstantiateAsync instantiates a component asynchronously with all imports
// defined in this linker.
//
// This is the async equivalent of Instantiate. It creates a future, polls it
// to completion, and returns the instance. This is needed for components that
// use the async ABI.
func (l *ComponentLinker) InstantiateAsync(store Storelike, component *Component) (*ComponentInstance, error) {
	var ret C.wasmtime_component_instance_t
	var errRet *C.wasmtime_error_t

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	future := C.go_component_linker_instantiate_async(
		l.ptr(),
		store.Context(),
		component.ptr(),
		&ret,
		&errRet,
	)
	if future == nil {
		runtime.KeepAlive(l)
		runtime.KeepAlive(store)
		runtime.KeepAlive(component)
		if errRet != nil {
			return nil, mkError(errRet)
		}
		return nil, errors.New("component async instantiation is not supported by linked Wasmtime")
	}
	defer C.wasmtime_call_future_delete(future)

	for !C.wasmtime_call_future_poll(future) {
		runtime.Gosched()
	}
	runtime.KeepAlive(l)
	runtime.KeepAlive(store)
	runtime.KeepAlive(component)

	if errRet != nil {
		return nil, mkError(errRet)
	}
	return mkComponentInstance(ret), nil
}

// AddWasip2 adds all WASI preview 2 interfaces into this linker.
func (l *ComponentLinker) AddWasip2() error {
	err := C.wasmtime_component_linker_add_wasip2(l.ptr())
	runtime.KeepAlive(l)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// AddWasip3 adds all WASI P3 interfaces into this linker.
//
// This requires the component-model-async feature to be enabled in
// the engine configuration.
func (l *ComponentLinker) AddWasip3() error {
	err := C.go_component_linker_add_wasip3(l.ptr())
	runtime.KeepAlive(l)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// AddWasiHttp adds WASI HTTP interfaces to the linker.
// This enables components to make outbound HTTP requests using wasi:http.
func (l *ComponentLinker) AddWasiHttp() error {
	err := C.go_component_linker_add_wasi_http(l.ptr())
	runtime.KeepAlive(l)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// DefineUnknownImportsAsTraps defines all unknown imports of `component` as
// functions that immediately trap when called.
func (l *ComponentLinker) DefineUnknownImportsAsTraps(component *Component) error {
	err := C.wasmtime_component_linker_define_unknown_imports_as_traps(
		l.ptr(),
		component.ptr(),
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(component)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// ComponentLinkerInstance represents an "instance" being defined within a
// component linker. It is used to define functions, modules, and nested
// instances.
//
// This type should only be used within a callback passed to
// ComponentLinker.Root or ComponentLinkerInstance.AddInstance.
type ComponentLinkerInstance struct {
	_ptr *C.wasmtime_component_linker_instance_t
}

func (li *ComponentLinkerInstance) ptr() *C.wasmtime_component_linker_instance_t {
	ret := li._ptr
	if ret == nil {
		panic("linker instance has been closed or is being used outside its callback")
	}
	maybeGC()
	return ret
}

// AddInstance defines a nested instance within this instance.
//
// This can be used to describe arbitrarily nested levels of instances within a
// linker to satisfy nested instance exports of components.
//
// The callback `f` is invoked with the new nested `ComponentLinkerInstance`.
// The parent linker instance must not be used concurrently while the callback
// is executing.
func (li *ComponentLinkerInstance) AddInstance(name string, f func(*ComponentLinkerInstance)) error {
	var childPtr *C.wasmtime_component_linker_instance_t
	err := C.wasmtime_component_linker_instance_add_instance(
		li.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&childPtr,
	)
	runtime.KeepAlive(li)
	runtime.KeepAlive(name)
	if err != nil {
		return mkError(err)
	}
	child := &ComponentLinkerInstance{_ptr: childPtr}
	defer func() {
		C.wasmtime_component_linker_instance_delete(child._ptr)
		child._ptr = nil
	}()
	f(child)
	return nil
}

// AddModule defines a core wasm Module within this instance.
//
// This can be used to provide a core wasm Module as an import to a component.
func (li *ComponentLinkerInstance) AddModule(name string, module *Module) error {
	err := C.wasmtime_component_linker_instance_add_module(
		li.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		module.ptr(),
	)
	runtime.KeepAlive(li)
	runtime.KeepAlive(name)
	runtime.KeepAlive(module)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// AddFunc defines a component function within this linker instance.
//
// The callback receives the args as Go ComponentVal slices and must populate
// the results slice in-place.
func (li *ComponentLinkerInstance) AddFunc(name string, callback ComponentFuncCallback) error {
	idx := insertComponentCallback(callback)
	err := C.go_component_linker_instance_add_func(
		li.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		C.size_t(idx),
	)
	runtime.KeepAlive(li)
	runtime.KeepAlive(name)
	if err != nil {
		deleteComponentCallback(idx)
		return mkError(err)
	}
	return nil
}

// AddFuncAsync defines an async component function within this linker instance.
//
// This is the async equivalent of AddFunc. It uses the same Go callback type
// (ComponentFuncCallback) but registers the function with the async ABI so it
// can be called from components that use async instantiation.
//
// The callback executes synchronously within the async trampoline, and the
// continuation is set to "immediately ready" so that the async runtime
// proceeds without further polling.
func (li *ComponentLinkerInstance) AddFuncAsync(name string, callback ComponentFuncCallback) error {
	idx := insertComponentCallback(callback)
	err := C.go_component_linker_instance_add_func_async(
		li.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		C.size_t(idx),
	)
	runtime.KeepAlive(li)
	runtime.KeepAlive(name)
	if err != nil {
		deleteComponentCallback(idx)
		return mkError(err)
	}
	return nil
}

// AddResource defines a resource type within this linker instance.
//
// The `resourceType` identifies the resource, and `destructor` is called when
// an owned resource is dropped. The `rep` parameter in the destructor is the
// host-defined 32-bit representation.
func (li *ComponentLinkerInstance) AddResource(
	name string,
	resourceType *ComponentResourceType,
	destructor ComponentResourceDestructor,
) error {
	idx := insertComponentCallback(destructor)
	err := C.go_component_linker_instance_add_resource(
		li.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		resourceType.ptr(),
		C.size_t(idx),
	)
	runtime.KeepAlive(li)
	runtime.KeepAlive(name)
	runtime.KeepAlive(resourceType)
	if err != nil {
		deleteComponentCallback(idx)
		return mkError(err)
	}
	return nil
}

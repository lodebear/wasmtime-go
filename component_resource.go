package wasmtime

// #include <wasmtime/component.h>
// #include <stdlib.h>
// #include "shims.h"
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// ComponentResourceType represents the type of a component resource.
// This is an opaque type used to equate resource types and define new resources
// in the linker.
type ComponentResourceType struct {
	_ptr *C.wasmtime_component_resource_type_t
}

// NewComponentResourceType creates a new resource type representing a
// host-defined resource identified by the given type ID.
//
// Two host resources with different type IDs are considered not-equal.
func NewComponentResourceType(ty uint32) *ComponentResourceType {
	ptr := C.wasmtime_component_resource_type_new_host(C.uint32_t(ty))
	return mkComponentResourceType(ptr)
}

func mkComponentResourceType(ptr *C.wasmtime_component_resource_type_t) *ComponentResourceType {
	rt := &ComponentResourceType{_ptr: ptr}
	runtime.SetFinalizer(rt, func(rt *ComponentResourceType) {
		rt.Close()
	})
	return rt
}

func (rt *ComponentResourceType) ptr() *C.wasmtime_component_resource_type_t {
	ret := rt._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Equal returns whether two resource types are the same.
func (rt *ComponentResourceType) Equal(other *ComponentResourceType) bool {
	ret := bool(C.wasmtime_component_resource_type_equal(rt.ptr(), other.ptr()))
	runtime.KeepAlive(rt)
	runtime.KeepAlive(other)
	return ret
}

// Clone creates a new owned copy of this resource type.
func (rt *ComponentResourceType) Clone() *ComponentResourceType {
	ptr := C.wasmtime_component_resource_type_clone(rt.ptr())
	runtime.KeepAlive(rt)
	return mkComponentResourceType(ptr)
}

// Close deallocates this resource type.
func (rt *ComponentResourceType) Close() {
	if rt._ptr == nil {
		return
	}
	runtime.SetFinalizer(rt, nil)
	C.wasmtime_component_resource_type_delete(rt._ptr)
	rt._ptr = nil
}

// ComponentResourceAny represents an opaque component resource handle that may
// be either guest-owned or host-owned.
type ComponentResourceAny struct {
	_ptr *C.wasmtime_component_resource_any_t
}

func mkComponentResourceAny(ptr *C.wasmtime_component_resource_any_t) *ComponentResourceAny {
	ra := &ComponentResourceAny{_ptr: ptr}
	runtime.SetFinalizer(ra, func(ra *ComponentResourceAny) {
		ra.Close()
	})
	return ra
}

func (ra *ComponentResourceAny) ptr() *C.wasmtime_component_resource_any_t {
	ret := ra._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Type returns the resource type of this resource.
func (ra *ComponentResourceAny) Type() *ComponentResourceType {
	ptr := C.wasmtime_component_resource_any_type(ra.ptr())
	runtime.KeepAlive(ra)
	return mkComponentResourceType(ptr)
}

// Owned returns whether this resource is an `own` (true) or `borrow` (false).
func (ra *ComponentResourceAny) Owned() bool {
	ret := bool(C.wasmtime_component_resource_any_owned(ra.ptr()))
	runtime.KeepAlive(ra)
	return ret
}

// Drop performs the component-model "drop" operation on this resource.
// This cleans up borrow-tracking metadata and may invoke a guest destructor.
// After calling Drop, you must still call Close to free host-side memory.
func (ra *ComponentResourceAny) Drop(store Storelike) error {
	err := C.wasmtime_component_resource_any_drop(store.Context(), ra.ptr())
	runtime.KeepAlive(ra)
	runtime.KeepAlive(store)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// ToHost attempts to convert this resource to a host-defined resource.
// Returns an error if the resource is guest-defined.
func (ra *ComponentResourceAny) ToHost(store Storelike) (*ComponentResourceHost, error) {
	var ret *C.wasmtime_component_resource_host_t
	err := C.wasmtime_component_resource_any_to_host(store.Context(), ra.ptr(), &ret)
	runtime.KeepAlive(ra)
	runtime.KeepAlive(store)
	if err != nil {
		return nil, mkError(err)
	}
	return mkComponentResourceHost(ret), nil
}

// Clone creates a new owned copy of this resource handle.
func (ra *ComponentResourceAny) Clone() *ComponentResourceAny {
	ptr := C.wasmtime_component_resource_any_clone(ra.ptr())
	runtime.KeepAlive(ra)
	return mkComponentResourceAny(ptr)
}

// DropAndClose performs the component-model "drop" and then frees host-side
// memory in a single call. This is a convenience for the common pattern of
// calling Drop followed by Close.
func (ra *ComponentResourceAny) DropAndClose(store Storelike) error {
	err := ra.Drop(store)
	ra.Close()
	return err
}

// Close deallocates host-side memory for this resource handle.
// This does not perform the component-model "drop"; use Drop for that.
func (ra *ComponentResourceAny) Close() {
	if ra._ptr == nil {
		return
	}
	runtime.SetFinalizer(ra, nil)
	C.wasmtime_component_resource_any_delete(ra._ptr)
	ra._ptr = nil
}

// ComponentResourceHost represents a host-defined component resource with
// a "rep" (representation) that is a 32-bit integer defined by the host.
type ComponentResourceHost struct {
	_ptr *C.wasmtime_component_resource_host_t
}

// NewComponentResourceHost creates a new host-defined resource.
// `owned` indicates whether this is an `own` or `borrow`.
// `rep` is the host-defined representation (a trusted u32).
// `ty` is the host-defined type discriminator (a trusted u32).
func NewComponentResourceHost(owned bool, rep uint32, ty uint32) *ComponentResourceHost {
	ptr := C.wasmtime_component_resource_host_new(C.bool(owned), C.uint32_t(rep), C.uint32_t(ty))
	return mkComponentResourceHost(ptr)
}

func mkComponentResourceHost(ptr *C.wasmtime_component_resource_host_t) *ComponentResourceHost {
	rh := &ComponentResourceHost{_ptr: ptr}
	runtime.SetFinalizer(rh, func(rh *ComponentResourceHost) {
		rh.Close()
	})
	return rh
}

func (rh *ComponentResourceHost) ptr() *C.wasmtime_component_resource_host_t {
	ret := rh._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Rep returns the 32-bit host representation of this resource.
func (rh *ComponentResourceHost) Rep() uint32 {
	ret := uint32(C.wasmtime_component_resource_host_rep(rh.ptr()))
	runtime.KeepAlive(rh)
	return ret
}

// HostType returns the 32-bit host type identifier for this resource.
func (rh *ComponentResourceHost) HostType() uint32 {
	ret := uint32(C.wasmtime_component_resource_host_type(rh.ptr()))
	runtime.KeepAlive(rh)
	return ret
}

// Owned returns whether this resource is an `own` (true) or `borrow` (false).
func (rh *ComponentResourceHost) Owned() bool {
	ret := bool(C.wasmtime_component_resource_host_owned(rh.ptr()))
	runtime.KeepAlive(rh)
	return ret
}

// ToAny converts this host resource to a ComponentResourceAny handle.
func (rh *ComponentResourceHost) ToAny(store Storelike) (*ComponentResourceAny, error) {
	var ret *C.wasmtime_component_resource_any_t
	err := C.wasmtime_component_resource_host_to_any(store.Context(), rh.ptr(), &ret)
	runtime.KeepAlive(rh)
	runtime.KeepAlive(store)
	if err != nil {
		return nil, mkError(err)
	}
	return mkComponentResourceAny(ret), nil
}

// Clone creates a new owned copy of this host resource.
func (rh *ComponentResourceHost) Clone() *ComponentResourceHost {
	ptr := C.wasmtime_component_resource_host_clone(rh.ptr())
	runtime.KeepAlive(rh)
	return mkComponentResourceHost(ptr)
}

// Close deallocates this host resource.
func (rh *ComponentResourceHost) Close() {
	if rh._ptr == nil {
		return
	}
	runtime.SetFinalizer(rh, nil)
	C.wasmtime_component_resource_host_delete(rh._ptr)
	rh._ptr = nil
}

// --- Component linker callback infrastructure ---

// ComponentFuncCallback is the Go type for a component function callback.
// It receives the store context, the function type, and the argument vals.
// It should populate the result vals and return nil on success or an error.
type ComponentFuncCallback func(
	caller Storelike,
	args []ComponentVal,
	results []ComponentVal,
) error

// ComponentResourceDestructor is called when a resource is dropped.
// `rep` is the 32-bit host representation.
type ComponentResourceDestructor func(caller Storelike, rep uint32) error

var (
	gComponentCallbackLock sync.Mutex
	gComponentCallbackMap  = make(map[int]interface{})
	gComponentCallbackSlab slab
)

func insertComponentCallback(cb interface{}) int {
	gComponentCallbackLock.Lock()
	defer gComponentCallbackLock.Unlock()
	idx := gComponentCallbackSlab.allocate()
	gComponentCallbackMap[idx] = cb
	return idx
}

func getComponentCallback(idx int) interface{} {
	gComponentCallbackLock.Lock()
	defer gComponentCallbackLock.Unlock()
	return gComponentCallbackMap[idx]
}

func deleteComponentCallback(idx int) {
	gComponentCallbackLock.Lock()
	defer gComponentCallbackLock.Unlock()
	delete(gComponentCallbackMap, idx)
	gComponentCallbackSlab.deallocate(idx)
}

//export goComponentFuncCallback
func goComponentFuncCallback(
	contextPtr *C.wasmtime_context_t,
	env C.size_t,
	ty *C.wasmtime_component_func_type_t,
	argsPtr *C.wasmtime_component_val_t,
	argsNum C.size_t,
	resultsPtr *C.wasmtime_component_val_t,
	resultsNum C.size_t,
) *C.wasmtime_error_t {
	cb := getComponentCallback(int(env)).(ComponentFuncCallback)

	// Create a minimal Storelike wrapper from the raw context pointer
	caller := &componentCaller{ctx: contextPtr}

	// Unmarshal args from C to Go
	goArgs := componentValsFromC(argsPtr, argsNum)

	// Create Go result slice
	goResults := make([]ComponentVal, int(resultsNum))

	// Call the Go callback, catching panics to avoid crashing across CGo boundary
	var cbErr error
	var lastPanic interface{}
	func() {
		defer func() { lastPanic = recover() }()
		cbErr = cb(caller, goArgs, goResults)
	}()

	// Invalidate the caller so it can't be used after the callback returns
	caller.ctx = nil

	if lastPanic != nil {
		msg := fmt.Sprintf("Go callback panicked: %v", lastPanic)
		cMsg := C.CString(msg)
		defer C.free(unsafe.Pointer(cMsg))
		return C.wasmtime_error_new(cMsg)
	}

	if cbErr != nil {
		msg := cbErr.Error()
		cMsg := C.CString(msg)
		defer C.free(unsafe.Pointer(cMsg))
		return C.wasmtime_error_new(cMsg)
	}

	// Marshal Go results back to C
	for i := 0; i < int(resultsNum); i++ {
		elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(resultsPtr)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
		))
		componentValToC(&goResults[i], elemPtr)
	}

	return nil
}

//export goFinalizeComponentFunc
func goFinalizeComponentFunc(env unsafe.Pointer) {
	deleteComponentCallback(int(uintptr(env)))
}

//export goComponentResourceDtor
func goComponentResourceDtor(
	contextPtr *C.wasmtime_context_t,
	env C.size_t,
	rep C.uint32_t,
) *C.wasmtime_error_t {
	cb := getComponentCallback(int(env)).(ComponentResourceDestructor)

	caller := &componentCaller{ctx: contextPtr}

	var cbErr error
	var lastPanic interface{}
	func() {
		defer func() { lastPanic = recover() }()
		cbErr = cb(caller, uint32(rep))
	}()

	// Invalidate the caller so it can't be used after the callback returns
	caller.ctx = nil

	if lastPanic != nil {
		msg := fmt.Sprintf("Go resource destructor panicked: %v", lastPanic)
		cMsg := C.CString(msg)
		defer C.free(unsafe.Pointer(cMsg))
		return C.wasmtime_error_new(cMsg)
	}

	if cbErr != nil {
		msg := cbErr.Error()
		cMsg := C.CString(msg)
		defer C.free(unsafe.Pointer(cMsg))
		return C.wasmtime_error_new(cMsg)
	}
	return nil
}

//export goFinalizeComponentResourceDtor
func goFinalizeComponentResourceDtor(env unsafe.Pointer) {
	deleteComponentCallback(int(uintptr(env)))
}

// componentCaller wraps a raw wasmtime_context_t pointer to implement Storelike.
// The ctx pointer is ONLY valid for the duration of the callback invocation.
// After the callback returns, ctx is set to nil and any further use will panic.
// Do not store references to this value beyond the callback scope.
type componentCaller struct {
	ctx *C.wasmtime_context_t
}

func (c *componentCaller) Context() *C.wasmtime_context_t {
	if c.ctx == nil {
		panic("component caller used outside of callback scope")
	}
	return c.ctx
}

func (c *componentCaller) Data() interface{} {
	data := uintptr(C.wasmtime_context_get_data(c.ctx))
	gStoreLock.Lock()
	defer gStoreLock.Unlock()
	entry := gStoreMap[int(data)]
	if entry == nil {
		return nil
	}
	return entry.data
}

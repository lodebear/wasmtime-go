package wasmtime

// #include <wasmtime/component.h>
// #include <wasmtime/async.h>
// #include "shims.h"
// #include <stdlib.h>
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

// Call invokes this component function with the provided `args`.
//
// The args must match the function's parameter types. Results are returned as
// a slice of ComponentVal. Returns an error if the call traps or otherwise fails.
func (f *ComponentFunc) Call(store Storelike, args []ComponentVal) ([]ComponentVal, error) {
	// Marshal Go args to C
	var cArgs *C.wasmtime_component_val_t
	var cArgsSize C.size_t
	if len(args) > 0 {
		cArgs, cArgsSize = componentValsToC(args)
		defer freeComponentValsCArray(cArgs, cArgsSize)
	}

	// Determine the number of results from the function type
	funcType := f.Type(store)
	defer funcType.Close()
	nResults := funcType.ResultArity()

	// The C API assigns Rust enum values into these slots, which drops the
	// previous value. Zero-initialize each slot as the valid Bool(false) value.
	var cResults *C.wasmtime_component_val_t
	resultsInitialized := false
	if nResults > 0 {
		cResults = (*C.wasmtime_component_val_t)(C.calloc(
			C.size_t(nResults), C.size_t(unsafe.Sizeof(C.wasmtime_component_val_t{})),
		))
		defer func() {
			// Only a successful call replaces the placeholders with results.
			if resultsInitialized {
				for i := 0; i < nResults; i++ {
					elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
						uintptr(unsafe.Pointer(cResults)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
					))
					C.wasmtime_component_val_delete(elemPtr)
				}
			}
			C.free(unsafe.Pointer(cResults))
		}()
	}

	// Call the function
	err := C.wasmtime_component_func_call(
		&f.val,
		store.Context(),
		cArgs,
		cArgsSize,
		cResults,
		C.size_t(nResults),
	)
	runtime.KeepAlive(store)
	runtime.KeepAlive(args)
	runtime.KeepAlive(f)

	if err != nil {
		return nil, mkError(err)
	}
	resultsInitialized = true

	// Unmarshal C results to Go (copies all data to Go-owned memory)
	results := componentValsFromC(cResults, C.size_t(nResults))

	// Signal to the component that we're done processing the return values.
	// This must happen after reading results but before the next call into
	// this component instance; otherwise the instance remains in a "called"
	// state and subsequent calls trap with "cannot enter component instance".
	err = C.wasmtime_component_func_post_return(&f.val, store.Context())
	runtime.KeepAlive(store)
	runtime.KeepAlive(f)
	if err != nil {
		return nil, mkError(err)
	}

	return results, nil
}

// CallAsync invokes this component function asynchronously with the provided `args`.
//
// This is the async equivalent of Call. It creates a future, polls it to
// completion, and returns the results. This is needed for components that
// use the async ABI (e.g., those with stream or future types).
//
// No separate post_return call is needed — call_async handles it automatically.
func (f *ComponentFunc) CallAsync(store Storelike, args []ComponentVal) ([]ComponentVal, error) {
	// Marshal Go args to C
	var cArgs *C.wasmtime_component_val_t
	var cArgsSize C.size_t
	if len(args) > 0 {
		cArgs, cArgsSize = componentValsToC(args)
		defer freeComponentValsCArray(cArgs, cArgsSize)
	}

	// Determine the number of results from the function type
	funcType := f.Type(store)
	defer funcType.Close()
	nResults := funcType.ResultArity()

	// Initialize result slots as Bool(false), as in Call.
	var cResults *C.wasmtime_component_val_t
	resultsInitialized := false
	if nResults > 0 {
		cResults = (*C.wasmtime_component_val_t)(C.calloc(
			C.size_t(nResults), C.size_t(unsafe.Sizeof(C.wasmtime_component_val_t{})),
		))
		defer func() {
			// Failed and unsupported calls leave the placeholders unchanged.
			if resultsInitialized {
				for i := 0; i < nResults; i++ {
					elemPtr := (*C.wasmtime_component_val_t)(unsafe.Pointer(
						uintptr(unsafe.Pointer(cResults)) + uintptr(i)*unsafe.Sizeof(C.wasmtime_component_val_t{}),
					))
					C.wasmtime_component_val_delete(elemPtr)
				}
			}
			C.free(unsafe.Pointer(cResults))
		}()
	}

	// Error return pointer
	var errRet *C.wasmtime_error_t

	// Create the async future
	// Lock the OS thread since wasmtime futures may not be thread-safe
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	future := C.go_component_func_call_async(
		&f.val,
		store.Context(),
		cArgs,
		cArgsSize,
		cResults,
		C.size_t(nResults),
		&errRet,
	)
	if future == nil {
		runtime.KeepAlive(store)
		runtime.KeepAlive(args)
		runtime.KeepAlive(f)
		if errRet != nil {
			return nil, mkError(errRet)
		}
		return nil, errors.New("component async call is not supported by linked Wasmtime")
	}
	defer C.wasmtime_call_future_delete(future)

	// Poll the future to completion
	for !C.wasmtime_call_future_poll(future) {
		runtime.Gosched()
	}
	runtime.KeepAlive(store)
	runtime.KeepAlive(args)
	runtime.KeepAlive(f)

	if errRet != nil {
		return nil, mkError(errRet)
	}
	resultsInitialized = true

	// Unmarshal C results to Go
	results := componentValsFromC(cResults, C.size_t(nResults))
	return results, nil
}

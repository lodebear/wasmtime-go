package wasmtime

// #include "shims.h"
import "C"
import (
	"runtime"
	"unsafe"
)

// SharedMemory represents a shared linear memory.
type SharedMemory struct {
	_ptr  *C.wasmtime_sharedmemory_t
	owned bool
}

// NewSharedMemory creates a new shared memory in the given engine.
func NewSharedMemory(engine *Engine, ty *MemoryType) (*SharedMemory, error) {
	var ret *C.wasmtime_sharedmemory_t
	err := C.wasmtime_sharedmemory_new(engine.ptr(), ty.ptr(), &ret)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(ty)
	if err != nil {
		return nil, mkError(err)
	}
	return mkSharedMemory(ret, true), nil
}

func mkSharedMemory(ptr *C.wasmtime_sharedmemory_t, owned bool) *SharedMemory {
	if ptr == nil {
		return nil
	}
	mem := &SharedMemory{_ptr: ptr, owned: owned}
	if owned {
		runtime.SetFinalizer(mem, func(mem *SharedMemory) {
			mem.Close()
		})
	}
	return mem
}

func (m *SharedMemory) ptr() *C.wasmtime_sharedmemory_t {
	ret := m._ptr
	if ret == nil {
		panic("object already closed")
	}
	maybeGC()
	return ret
}

// Close releases the shared memory if this wrapper owns it.
func (m *SharedMemory) Close() {
	if m._ptr == nil || !m.owned {
		return
	}
	runtime.SetFinalizer(m, nil)
	C.wasmtime_sharedmemory_delete(m._ptr)
	m._ptr = nil
}

// Clone creates another reference-counted handle to this shared memory.
func (m *SharedMemory) Clone() *SharedMemory {
	ptr := C.wasmtime_sharedmemory_clone(m.ptr())
	runtime.KeepAlive(m)
	return mkSharedMemory(ptr, true)
}

// Type returns the memory type for this shared memory.
func (m *SharedMemory) Type() *MemoryType {
	ptr := C.wasmtime_sharedmemory_type(m.ptr())
	runtime.KeepAlive(m)
	return mkMemoryType(ptr, nil)
}

// Data returns the raw pointer for this shared memory.
func (m *SharedMemory) Data() unsafe.Pointer {
	ret := unsafe.Pointer(C.wasmtime_sharedmemory_data(m.ptr()))
	runtime.KeepAlive(m)
	return ret
}

// DataSize returns the size, in bytes, of this shared memory.
func (m *SharedMemory) DataSize() uintptr {
	ret := uintptr(C.wasmtime_sharedmemory_data_size(m.ptr()))
	runtime.KeepAlive(m)
	return ret
}

// UnsafeData returns the shared memory as a byte slice.
func (m *SharedMemory) UnsafeData() []byte {
	length := m.DataSize()
	return unsafe.Slice((*byte)(m.Data()), length)
}

// Size returns the size, in WebAssembly pages, of this shared memory.
func (m *SharedMemory) Size() uint64 {
	ret := uint64(C.wasmtime_sharedmemory_size(m.ptr()))
	runtime.KeepAlive(m)
	return ret
}

// Grow grows this shared memory by delta pages.
func (m *SharedMemory) Grow(delta uint64) (uint64, error) {
	prev := C.uint64_t(0)
	err := C.wasmtime_sharedmemory_grow(m.ptr(), C.uint64_t(delta), &prev)
	runtime.KeepAlive(m)
	if err != nil {
		return 0, mkError(err)
	}
	return uint64(prev), nil
}

// AsExtern converts this shared memory to an extern for import/export.
func (m *SharedMemory) AsExtern() C.wasmtime_extern_t {
	var ret C.wasmtime_extern_t
	ret.kind = C.WASMTIME_EXTERN_SHAREDMEMORY
	C.go_wasmtime_extern_sharedmemory_set(&ret, m.ptr())
	runtime.KeepAlive(m)
	return ret
}

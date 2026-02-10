package wasmtime

// #include "shims.h"
import "C"

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// wasiOutputEntry is a Go-side object referenced from Rust via a small integer
// ID stored in the WASI CustomOutputStream's user data.
//
// Writes are serialized with a mutex because the WASI runtime may write from
// different tasks/threads.
type wasiOutputEntry struct {
	mu sync.Mutex
	w  io.Writer
}

var (
	gWasiOutputLock sync.Mutex
	gWasiOutput     = make(map[int]*wasiOutputEntry)
	gWasiOutputSlab slab
)

func insertWasiOutputWriter(w io.Writer) int {
	if w == nil {
		panic("nil io.Writer")
	}
	gWasiOutputLock.Lock()
	defer gWasiOutputLock.Unlock()
	idx := gWasiOutputSlab.allocate()
	gWasiOutput[idx] = &wasiOutputEntry{w: w}
	return idx
}

func removeWasiOutputWriter(idx int) {
	gWasiOutputLock.Lock()
	defer gWasiOutputLock.Unlock()
	delete(gWasiOutput, idx)
	gWasiOutputSlab.deallocate(idx)
}

func getWasiOutputWriter(idx int) *wasiOutputEntry {
	gWasiOutputLock.Lock()
	defer gWasiOutputLock.Unlock()
	return gWasiOutput[idx]
}

// SetStdoutWriter configures WASI stdout to be written to the provided writer.
//
// This uses Wasmtime's C API `wasi_config_set_stdout_custom` internally.
func (c *WasiConfig) SetStdoutWriter(w io.Writer) {
	idx := insertWasiOutputWriter(w)
	C.go_wasi_config_set_stdout_custom(c.ptr(), C.size_t(idx))
	runtime.KeepAlive(c)
	runtime.KeepAlive(w)
}

// SetStderrWriter configures WASI stderr to be written to the provided writer.
//
// This uses Wasmtime's C API `wasi_config_set_stderr_custom` internally.
func (c *WasiConfig) SetStderrWriter(w io.Writer) {
	idx := insertWasiOutputWriter(w)
	C.go_wasi_config_set_stderr_custom(c.ptr(), C.size_t(idx))
	runtime.KeepAlive(c)
	runtime.KeepAlive(w)
}

// MemoryOutputPipe is a concurrency-safe in-memory output sink useful for
// capturing stdout/stderr without writing to temporary files.
//
// This is modeled after Rust's `wasmtime_wasi::p2::pipe::MemoryOutputPipe`.
//
// The capacity is enforced to avoid unbounded memory growth.
type MemoryOutputPipe struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	capacity int
}

// NewMemoryOutputPipe creates a new in-memory stdout/stderr sink with the
// specified maximum capacity in bytes.
func NewMemoryOutputPipe(capacity int) *MemoryOutputPipe {
	if capacity <= 0 {
		panic("capacity must be > 0")
	}
	return &MemoryOutputPipe{capacity: capacity}
}

func (p *MemoryOutputPipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.buf.Len()+len(b) > p.capacity {
		return 0, syscall.ENOSPC
	}
	return p.buf.Write(b)
}

// Contents returns a copy of all bytes written so far.
func (p *MemoryOutputPipe) Contents() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	cpy := make([]byte, p.buf.Len())
	copy(cpy, p.buf.Bytes())
	return cpy
}

// Bytes is an alias for Contents.
func (p *MemoryOutputPipe) Bytes() []byte {
	return p.Contents()
}

func (p *MemoryOutputPipe) String() string {
	return string(p.Contents())
}

func (p *MemoryOutputPipe) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.Len()
}

//export goWasiOutputWrite
func goWasiOutputWrite(env unsafe.Pointer, data *C.uchar, nbytes C.size_t) C.ptrdiff_t {
	idx := int(uintptr(env))
	entry := getWasiOutputWriter(idx)
	if entry == nil {
		return C.ptrdiff_t(-int(syscall.EBADF))
	}
	if nbytes == 0 {
		return 0
	}
	// Copy the bytes to avoid surprising retention of foreign memory by a custom writer.
	if nbytes > C.size_t(int(^uint(0)>>1)) {
		return C.ptrdiff_t(-int(syscall.EOVERFLOW))
	}
	b := C.GoBytes(unsafe.Pointer(data), C.int(nbytes))

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Rust requires full writes (no partial writes). Loop until the full buffer is written.
	written := 0
	for written < len(b) {
		n, err := entry.w.Write(b[written:])
		written += n
		if err != nil {
			// Return a negative OS error code when possible.
			var errno syscall.Errno
			if errors.As(err, &errno) {
				return C.ptrdiff_t(-int(errno))
			}
			return C.ptrdiff_t(-int(syscall.EIO))
		}
		if n == 0 {
			return C.ptrdiff_t(-int(syscall.EIO))
		}
	}
	return C.ptrdiff_t(nbytes)
}

//export goFinalizeWasiOutput
func goFinalizeWasiOutput(env unsafe.Pointer) {
	idx := int(uintptr(env))
	removeWasiOutputWriter(idx)
}

package wasmtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadP3Component loads a P3 component wasm from testdata/components.
func loadP3Component(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "components", name+".component.wasm")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to load component %s", name)
	return data
}

// setupP3Test creates an async engine, store with WASI config, linker with
// P3+P2 interfaces, and loads the named component. Returns all four objects.
// The linker and component are registered for cleanup via t.Cleanup.
func setupP3Test(t *testing.T, name string, wasiConfig *WasiConfig) (*Engine, *Store, *ComponentLinker, *Component) {
	t.Helper()

	engine := newAsyncComponentEngine()
	store := NewStore(engine)

	err := store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	t.Cleanup(func() { linker.Close() })

	// P3 components import both P3 and P2 WASI interfaces
	err = linker.AddWasip3()
	require.NoError(t, err)
	err = linker.AddWasip2()
	require.NoError(t, err)

	wasmBytes := loadP3Component(t, name)
	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	t.Cleanup(func() { component.Close() })

	return engine, store, linker, component
}

// callWasiRunP3 instantiates a P3 WASI component asynchronously and calls its
// wasi:cli/run@0.3.0-rc-2026-01-06#run export. Returns an error if the run
// function returns an error (e.g. non-zero exit code).
func callWasiRunP3(t *testing.T, store *Store, linker *ComponentLinker, component *Component) error {
	t.Helper()

	instance, err := linker.InstantiateAsync(store, component)
	require.NoError(t, err)

	// Look up the "wasi:cli/run@0.3.0-rc-2026-01-06" instance export first
	runInstanceIdx := instance.GetExportIndex(store, nil, "wasi:cli/run@0.3.0-rc-2026-01-06")
	if runInstanceIdx == nil {
		// Some components may not export the run interface — just instantiation runs them
		return nil
	}
	defer runInstanceIdx.Close()

	// Then look up "run" within that instance
	runFuncIdx := instance.GetExportIndex(store, runInstanceIdx, "run")
	if runFuncIdx == nil {
		return nil
	}
	defer runFuncIdx.Close()

	fn := instance.GetFunc(store, runFuncIdx)
	if fn == nil {
		return nil
	}

	results, err := fn.CallAsync(store, nil)
	if err != nil {
		return err
	}

	// wasi:cli/run#run returns result<_, _> — check if it's an error
	if len(results) > 0 {
		r := results[0].Result()
		if !r.IsOk {
			return errors.New("run returned error result")
		}
	}

	return nil
}

// --- CLI tests ---

func TestP3CliHelloStdout(t *testing.T) {
	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	stderrFile := filepath.Join(tmpDir, "stderr.txt")

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))
	require.NoError(t, wasiConfig.SetStderrFile(stderrFile))

	_, store, linker, component := setupP3Test(t, "p3_cli_hello_stdout", wasiConfig)

	err := callWasiRunP3(t, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello, world")
}

func TestP3CliHelloStdoutPostReturn(t *testing.T) {
	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	stderrFile := filepath.Join(tmpDir, "stderr.txt")

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))
	require.NoError(t, wasiConfig.SetStderrFile(stderrFile))

	_, store, linker, component := setupP3Test(t, "p3_cli_hello_stdout_post_return", wasiConfig)

	err := callWasiRunP3(t, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.NotEmpty(t, string(stdout), "expected non-empty stdout output")
}

func TestP3CliMuchStdout(t *testing.T) {
	// The component reads args: args[1] = string to write, args[2] = number of times
	stringToWrite := strings.Repeat("a", 1024)
	timesToWrite := "256"
	expectedLen := 1024 * 256

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", stringToWrite, timesToWrite})
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))

	_, store, linker, component := setupP3Test(t, "p3_cli_much_stdout", wasiConfig)

	err := callWasiRunP3(t, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Equal(t, expectedLen, len(stdout))
}

func TestP3Cli(t *testing.T) {
	// The p3_cli component asserts:
	//   args == ["p3_cli.component", "."]
	//   env is non-empty
	//   initial_cwd is None
	//   terminal_stdin/stdout/stderr are None (satisfied by file redirection)
	//   stdin returns EOF immediately
	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	stderrFile := filepath.Join(tmpDir, "stderr.txt")
	stdinFile := filepath.Join(tmpDir, "stdin.txt")

	// Provide empty stdin so read_via_stream returns EOF
	require.NoError(t, os.WriteFile(stdinFile, []byte{}, 0644))

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"p3_cli.component", "."})
	wasiConfig.SetEnv([]string{"FOO"}, []string{"bar"})
	require.NoError(t, wasiConfig.SetStdinFile(stdinFile))
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))
	require.NoError(t, wasiConfig.SetStderrFile(stderrFile))

	_, store, linker, component := setupP3Test(t, "p3_cli", wasiConfig)

	err := callWasiRunP3(t, store, linker, component)
	require.NoError(t, err)

	// Verify the component wrote to stdout and stderr
	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello stdout")

	stderr, err := os.ReadFile(stderrFile)
	require.NoError(t, err)
	require.Contains(t, string(stderr), "hello stderr")
}

// --- Filesystem tests ---
// P3 filesystem operations require a Tokio runtime in the host WASI
// implementation, which is not available when running through the C API
// without an async executor. These tests must be skipped.

func TestP3FileWrite(t *testing.T) {
	t.Skip("P3 filesystem tests require Tokio runtime not available through the C API's WASI host implementation")
}

func TestP3FilesystemFileReadWrite(t *testing.T) {
	t.Skip("P3 filesystem tests require Tokio runtime not available through the C API's WASI host implementation")
}

func TestP3Readdir(t *testing.T) {
	t.Skip("P3 filesystem tests require Tokio runtime not available through the C API's WASI host implementation")
}

// --- Clock tests ---
// P3 clock operations (monotonic_clock::wait_until/wait_for) require a Tokio
// runtime in the host WASI implementation.

func TestP3ClocksSleep(t *testing.T) {
	t.Skip("P3 clock tests require Tokio runtime not available through the C API's WASI host implementation")
}

// --- Random tests ---

func TestP3RandomImports(t *testing.T) {
	wasiConfig := NewWasiConfig()

	_, store, linker, component := setupP3Test(t, "p3_random_imports", wasiConfig)

	err := callWasiRunP3(t, store, linker, component)
	require.NoError(t, err)
}

// --- HTTP tests (skip — require wasi-http trigger infrastructure) ---

func TestP3HttpEcho(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpMiddlewareWithChain(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpMiddleware(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestContentLength(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestGet(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestInvalidDnsname(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestInvalidHeader(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestInvalidPort(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestInvalidVersion(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestLargePost(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestMissingPathAndQuery(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestPost(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestPut(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestResponseBuild(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestTimeout(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestUnknownMethod(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpOutboundRequestUnsupportedScheme(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

func TestP3HttpProxy(t *testing.T) {
	t.Skip("P3 HTTP tests require wasi-http trigger infrastructure not available in Go bindings")
}

// --- Socket tests (skip — require network infrastructure) ---

func TestP3SocketsIpNameLookup(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpBind(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpConnect(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpListen(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpSampleApplication(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpSockopts(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpStates(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsTcpStreams(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpBind(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpConnect(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpReceive(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpSampleApplication(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpSend(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpSockopts(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

func TestP3SocketsUdpStates(t *testing.T) {
	t.Skip("P3 socket tests require network infrastructure not available in Go bindings")
}

// --- Serve tests (skip — require HTTP trigger infrastructure) ---

func TestP3CliServeHelloWorld(t *testing.T) {
	t.Skip("P3 serve tests require HTTP trigger infrastructure not available in Go bindings")
}

func TestP3CliServeSleep(t *testing.T) {
	t.Skip("P3 serve tests require HTTP trigger infrastructure not available in Go bindings")
}

// --- API proxy test (skip — requires HTTP proxy infrastructure) ---

func TestP3ApiProxy(t *testing.T) {
	t.Skip("P3 API proxy test requires HTTP proxy infrastructure not available in Go bindings")
}

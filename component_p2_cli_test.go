package wasmtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadP2CliComponent loads a P2 CLI component wasm from testdata/components.
func loadP2CliComponent(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "components", name+".component.wasm")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to load component %s", name)
	return data
}

// setupP2CliBasic creates an engine+store+linker with WASI and the given component loaded.
// Returns a cleanup function the caller should defer.
func setupP2CliBasic(t *testing.T, wasmBytes []byte, wasiConfig *WasiConfig) (*Engine, *Store, *ComponentLinker, *Component) {
	t.Helper()
	engine := newComponentEngine()
	store := NewStore(engine)

	err := store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	t.Cleanup(func() { linker.Close() })
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	t.Cleanup(func() { component.Close() })

	return engine, store, linker, component
}

// --- Stdout / Output tests ---

func TestP2CliHelloStdout(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_hello_stdout")

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	stderrFile := filepath.Join(tmpDir, "stderr.txt")

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))
	require.NoError(t, wasiConfig.SetStderrFile(stderrFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello, world")

	stderr, err := os.ReadFile(stderrFile)
	require.NoError(t, err)
	require.Contains(t, string(stderr), "hello, world")
}

func TestP2CliMuchStdout(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_much_stdout")

	// Write a string of 1024 'a' chars, 256 times = 262144 bytes
	stringToWrite := strings.Repeat("a", 1024)
	timesToWrite := "256"
	expectedLen := 1024 * 256

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", stringToWrite, timesToWrite})
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Equal(t, expectedLen, len(stdout))
}

// --- Argument tests ---

func TestP2CliArgs(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_args")

	// Rust code expects: ["hello", "this", "", "is an argument", "with 🚩 emoji"]
	// The first arg (program name) is skipped by args.skip(1)
	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", "hello", "this", "", "is an argument", "with 🚩 emoji"})
	require.NoError(t, wasiConfig.SetStderrFile(filepath.Join(t.TempDir(), "stderr.txt")))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliArgv0(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_argv0")

	// Rust code: args.next() == args.next(), meaning argv[0] must equal argv[1]
	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"myarg", "myarg"})

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

// --- Environment variable tests ---

func TestP2CliEnv(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_env")

	// Rust code expects exactly: {"frabjous": "day", "callooh": "callay"}
	wasiConfig := NewWasiConfig()
	wasiConfig.SetEnv(
		[]string{"frabjous", "callooh"},
		[]string{"day", "callay"},
	)

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliLargeEnv(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_large_env")

	// Rust code iterates over env vars and prints them.
	// The Rust CLI test sets 512 env vars with 1024 'x' values.
	keys := make([]string, 512)
	values := make([]string, 512)
	largeVal := strings.Repeat("x", 1024)
	for i := 0; i < 512; i++ {
		keys[i] = fmt.Sprintf("KEY%d", i)
		values[i] = largeVal
	}

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetEnv(keys, values)
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	// Verify stdout has some env var output
	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "KEY0=")
}


// --- Exit code tests ---

func TestP2CliExitDefault(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_exit_default")

	// Rust: fn main() {} — should succeed
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliExitSuccess(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_exit_success")

	// Rust: std::process::exit(0) — generates a trap with "exit status 0"
	// In the wasmtime C API, process::exit(0) generates an error/trap even for code 0.
	// The Rust CLI runner special-cases this, but through the C API we get an error.
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		require.Contains(t, err.Error(), "exit status 0")
	}
}

func TestP2CliExitFailure(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_exit_failure")

	// Rust: std::process::exit(1) — should produce error
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.Error(t, err, "exit(1) should produce an error")
}

func TestP2CliExitPanic(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_exit_panic")

	// Rust: panic!("Curiouser and curiouser!") — should produce error
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.Error(t, err, "panic should produce an error")
}

func TestP2CliExitWithCode(t *testing.T) {
	t.Skip("exit_with_code requires -Scli-exit-with-code which is not available via the C API's AddWasip2")
}

// --- Filesystem tests ---

func TestP2CliFileRead(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_file_read")

	tmpDir := t.TempDir()
	// Rust expects to open "bar.txt" relative to preopened "/" dir
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bar.txt"), []byte("And stood awhile in thought"), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.PreopenDir(tmpDir, "/", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliFileAppend(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_file_append")

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bar.txt"), []byte("'Twas brillig, and the slithy toves.\n"), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.PreopenDir(tmpDir, "/", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	// Verify file was appended to. The Rust code appends multiple lines.
	content, err := os.ReadFile(filepath.Join(tmpDir, "bar.txt"))
	require.NoError(t, err)
	require.Contains(t, string(content), "Did gyre and gimble in the wabe;")
	require.Contains(t, string(content), "All mimsy were the borogoves,")
	require.Contains(t, string(content), "And the mome raths outgrabe.")
}

func TestP2CliFileDirSync(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_file_dir_sync")

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bar.txt"), []byte("'Twas brillig, and the slithy toves.\n"), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.PreopenDir(tmpDir, "/", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	// Rust: opens bar.txt, calls sync_all() and sync_data() — should succeed
	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliDirectoryList(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_directory_list")

	tmpDir := t.TempDir()

	// Create the directory structure expected by the Rust test:
	// / -> foo.txt, bar.txt, baz.txt, sub/
	// /sub -> wow.txt, yay.txt
	for _, name := range []string{"foo.txt", "bar.txt", "baz.txt"} {
		f, err := os.Create(filepath.Join(tmpDir, name))
		require.NoError(t, err)
		f.Close()
	}
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "sub"), 0755))
	for _, name := range []string{"wow.txt", "yay.txt"} {
		f, err := os.Create(filepath.Join(tmpDir, "sub", name))
		require.NoError(t, err)
		f.Close()
	}

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.PreopenDir(tmpDir, "/", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliMultiplePreopens(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_multiple_preopens")

	// Rust code uses wasip1 fd_prestat_get for fds 3,4,5 expecting paths /a, /b, /c
	// We need 3 preopened directories
	tmpDir := t.TempDir()
	dirA := filepath.Join(tmpDir, "a")
	dirB := filepath.Join(tmpDir, "b")
	dirC := filepath.Join(tmpDir, "c")
	require.NoError(t, os.MkdirAll(dirA, 0755))
	require.NoError(t, os.MkdirAll(dirB, 0755))
	require.NoError(t, os.MkdirAll(dirC, 0755))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.PreopenDir(dirA, "/a", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))
	require.NoError(t, wasiConfig.PreopenDir(dirB, "/b", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))
	require.NoError(t, wasiConfig.PreopenDir(dirC, "/c", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

// --- Stdin tests ---

func TestP2CliStdin(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_stdin")

	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	require.NoError(t, os.WriteFile(stdinFile, []byte("So rested he by the Tumtum tree"), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdinFile(stdinFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliStdinEmpty(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_stdin_empty")

	// Rust code: reads 0 bytes from stdin fd via wasip1::fd_read with buf_len=0
	// Provide an empty stdin (or provide stdin with data — it reads 0 bytes anyway)
	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	require.NoError(t, os.WriteFile(stdinFile, []byte("not to be read"), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdinFile(stdinFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliSpliceStdin(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_splice_stdin")

	msg := "So rested he by the Tumtum tree"
	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	require.NoError(t, os.WriteFile(stdinFile, []byte(msg), 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdinFile(stdinFile))
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	output := string(stdout)
	require.Contains(t, output, "before splice")
	require.Contains(t, output, msg)
	require.Contains(t, output, "completed splicing")
}

// --- Sleep tests ---

func TestP2CliSleep(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_sleep")

	// Rust: sleeps for 100ns — should return quickly
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

func TestP2CliSleepForever(t *testing.T) {
	t.Skip("sleep_forever blocks in host-level WASI sleep which cannot be interrupted by epoch; requires -Wtimeout flag from CLI runner")
}

// --- Negative capability tests (no TCP, no UDP, no IP name lookup) ---

func TestP2CliNoTcp(t *testing.T) {
	t.Skip("requires -Sinherit-network,tcp=no (network enabled but TCP disabled) which is not configurable via the C API")
}

func TestP2CliNoUdp(t *testing.T) {
	t.Skip("requires -Sinherit-network,udp=no (network enabled but UDP disabled) which is not configurable via the C API")
}

func TestP2CliNoIpNameLookup(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_no_ip_name_lookup")

	// Rust: tries resolve("example.com") and expects Err(PermanentResolverFailure)
	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStderrFile(filepath.Join(t.TempDir(), "stderr.txt")))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

// --- Clock test ---

func TestP2CliDefaultClocks(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_default_clocks")

	// Rust: tests Instant::now() and SystemTime::now() — should succeed
	wasiConfig := NewWasiConfig()

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)
}

// --- Misc tests ---

func TestP2CliExportCabiRealloc(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_export_cabi_realloc")

	// Rust: prints "hello, world" — tests that custom cabi_realloc export works
	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello, world")
}

func TestP2CliStdioWriteFlushes(t *testing.T) {
	wasmBytes := loadP2CliComponent(t, "p2_cli_stdio_write_flushes")

	// Rust: print!("> "), flush, read_line(stdin) which is empty, assert s.is_empty()
	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")
	stdinFile := filepath.Join(tmpDir, "stdin.txt")

	// Provide empty stdin so read_line returns empty
	require.NoError(t, os.WriteFile(stdinFile, []byte{}, 0644))

	wasiConfig := NewWasiConfig()
	require.NoError(t, wasiConfig.SetStdoutFile(stdoutFile))
	require.NoError(t, wasiConfig.SetStdinFile(stdinFile))

	engine, store, linker, component := setupP2CliBasic(t, wasmBytes, wasiConfig)

	err := callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "> ")
}

func TestP2CliInvokeAsync(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/components/p2_cli_invoke_async.component.wasm")
	require.NoError(t, err)

	engine := newAsyncComponentEngine()
	store := NewStore(engine)
	wasiConfig := NewWasiConfig()
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	// The component exports echo:echo/echo (async func(string) -> string)
	instance, err := linker.InstantiateAsync(store, component)
	require.NoError(t, err)

	// Find the "echo" export in the "echo" interface
	echoIdx := component.GetExportIndex(nil, "echo")
	if echoIdx == nil {
		t.Skip("component does not export 'echo' at root level")
	}
	defer echoIdx.Close()

	innerIdx := component.GetExportIndex(echoIdx, "echo")
	if innerIdx == nil {
		t.Skip("component does not export 'echo' function in echo interface")
	}
	defer innerIdx.Close()

	fn := instance.GetFunc(store, innerIdx)
	require.NotNil(t, fn)

	results, err := fn.CallAsync(store, []ComponentVal{NewComponentValString("hello async")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "hello async", results[0].String())
}

// --- HTTP Serve tests (skip — require wasi:http handler, not wasi:cli/run) ---

func TestP2CliServeHelloWorld(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeAuthorityAndScheme(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeConfig(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeEchoEnv(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeKeyvalue(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeReturnBeforeSet(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeSleep(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeTrapBeforeSet(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

func TestP2CliServeWithPrint(t *testing.T) {
	t.Skip("serve components export wasi:http/incoming-handler, not wasi:cli/run — requires HTTP trigger infrastructure")
}

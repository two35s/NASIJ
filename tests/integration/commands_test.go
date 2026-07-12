// Package integration_test contains end-to-end tests for the nasij CLI binary.
// The binary is compiled once in TestMain and reused across all tests.
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

// TestMain builds the nasij binary once before all integration tests run.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "nasij-integration-*")
	if err != nil {
		panic("failed to create temp dir for binary: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "nasij")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/nasij/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build nasij binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// run executes the nasij binary with the given args and returns stdout, stderr, and exit code.
func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)

	// Use isolated home directory so tests don't touch the real ~/.nasij
	home := t.TempDir()
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"NASIJ_LOG_FORMAT=json", // json for easier parsing
		"NASIJ_LOG_LEVEL=error", // suppress noise in test output
	)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestVersion(t *testing.T) {
	stdout, _, code := run(t, "version")
	assert.Equal(t, 0, code, "nasij version should exit 0")
	assert.Contains(t, stdout, "0.1.0", "should include version string")
}

func TestVersion_ContainsGoRuntime(t *testing.T) {
	stdout, _, code := run(t, "version")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "go1.", "should include Go runtime version")
}

func TestDoctor_ExitsZero(t *testing.T) {
	_, _, code := run(t, "doctor")
	assert.Equal(t, 0, code, "nasij doctor should exit 0 in a healthy environment")
}

func TestDoctor_ContainsSQLite(t *testing.T) {
	stdout, _, _ := run(t, "doctor")
	assert.Contains(t, strings.ToLower(stdout), "sqlite", "doctor should report SQLite status")
}

func TestInit_CreatesConfigFile(t *testing.T) {
	home := t.TempDir()
	cmd := exec.Command(binaryPath, "init")
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"NASIJ_LOG_FORMAT=json",
		"NASIJ_LOG_LEVEL=error",
	)
	require.NoError(t, cmd.Run())

	cfgPath := filepath.Join(home, ".nasij", "config.yaml")
	require.FileExists(t, cfgPath, "nasij init should create ~/.nasij/config.yaml")
}

func TestInit_Idempotent(t *testing.T) {
	home := t.TempDir()
	env := append(os.Environ(),
		"HOME="+home,
		"NASIJ_LOG_FORMAT=json",
		"NASIJ_LOG_LEVEL=error",
	)

	cmd1 := exec.Command(binaryPath, "init")
	cmd1.Env = env
	require.NoError(t, cmd1.Run())

	cmd2 := exec.Command(binaryPath, "init")
	cmd2.Env = env
	require.NoError(t, cmd2.Run(), "second nasij init should also succeed")
}

func TestWorkspaceCreate(t *testing.T) {
	stdout, _, code := run(t, "workspace", "create",
		"--name", "integration-test",
		"--target", "https://example.com",
	)
	require.Equal(t, 0, code, "workspace create should exit 0")
	assert.Contains(t, stdout, "integration-test")
	assert.Contains(t, stdout, "https://example.com")
}

func TestWorkspaceCreate_RequiresName(t *testing.T) {
	_, _, code := run(t, "workspace", "create", "--target", "https://example.com")
	assert.NotEqual(t, 0, code, "missing --name should fail")
}

func TestWorkspaceCreate_RequiresTarget(t *testing.T) {
	_, _, code := run(t, "workspace", "create", "--name", "test")
	assert.NotEqual(t, 0, code, "missing --target should fail")
}

func TestWorkspaceList_Empty(t *testing.T) {
	stdout, _, code := run(t, "workspace", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, strings.ToLower(stdout), "no workspaces")
}

func TestWorkspaceList_AfterCreate(t *testing.T) {
	home := t.TempDir()
	env := append(os.Environ(),
		"HOME="+home,
		"NASIJ_LOG_FORMAT=json",
		"NASIJ_LOG_LEVEL=error",
	)

	// Create a workspace
	cmd := exec.Command(binaryPath, "workspace", "create",
		"--name", "listed-workspace",
		"--target", "https://example.com",
	)
	cmd.Env = env
	require.NoError(t, cmd.Run())

	// List should show it
	listCmd := exec.Command(binaryPath, "workspace", "list")
	listCmd.Env = env
	var out strings.Builder
	listCmd.Stdout = &out
	require.NoError(t, listCmd.Run())

	assert.Contains(t, out.String(), "listed-workspace")
}

func TestNoArgs_PrintsHelp(t *testing.T) {
	stdout, _, code := run(t)
	assert.Equal(t, 0, code, "nasij with no args should exit 0 and print help")
	assert.Contains(t, strings.ToLower(stdout), "usage")
}

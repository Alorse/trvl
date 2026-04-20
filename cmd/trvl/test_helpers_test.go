package main

import (
	"runtime"
	"testing"
)

// setTestHome sets both HOME (Unix) and USERPROFILE (Windows) to the given
// directory so that tests using os.UserHomeDir() or preferences paths work
// correctly on all platforms.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

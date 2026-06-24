package main

import "testing"

func TestRunVersionWritesStandardOutput(t *testing.T) {
	previous := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previous
	})

	code, stdout := captureStdout(t, func() int {
		return run([]string{"--version"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "ichsm 1.2.3\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "ichsm 1.2.3\n")
	}
}

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "v1.2.3", want: "1.2.3"},
		{raw: "V1.2.3", want: "1.2.3"},
		{raw: "1.2.3", want: "1.2.3"},
		{raw: "local", want: "local"},
		{raw: "version", want: "version"},
	}

	for _, tt := range tests {
		if got := displayVersion(tt.raw); got != tt.want {
			t.Fatalf("displayVersion(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

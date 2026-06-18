package main

import (
	"bytes"
	"testing"
)

func TestWriteAlignedRows(t *testing.T) {
	var out bytes.Buffer
	err := writeAlignedRows(&out, [][]string{
		{"a", "long"},
		{"aa", "x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	const want = "a   long\n" +
		"aa  x\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

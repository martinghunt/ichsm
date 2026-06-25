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

func TestWriteDelimitedRowsNormalizesControlWhitespace(t *testing.T) {
	var out bytes.Buffer
	err := writeDelimitedRows(&out, [][]string{
		{"sample_accession", "description"},
		{"SAMEA1", "line one\nline two\tline three\r\nline four"},
	}, "\t")
	if err != nil {
		t.Fatal(err)
	}

	const want = "sample_accession\tdescription\n" +
		"SAMEA1\tline one line two line three line four\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

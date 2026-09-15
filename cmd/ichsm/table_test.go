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

func TestWriteDelimitedRowsNeutralizesSpreadsheetFormulas(t *testing.T) {
	var out bytes.Buffer
	err := writeDelimitedRows(&out, [][]string{
		{"sample_accession", "description"},
		{"SAMEA1", "=cmd|' /C calc.exe'!A0"},
		{"SAMEA2", "+1+1"},
		{"SAMEA3", "-1+1"},
		{"SAMEA4", "@SUM(1+1)"},
		{"SAMEA5", "normal description"},
	}, "\t")
	if err != nil {
		t.Fatal(err)
	}

	const want = "sample_accession\tdescription\n" +
		"SAMEA1\t'=cmd|' /C calc.exe'!A0\n" +
		"SAMEA2\t'+1+1\n" +
		"SAMEA3\t'-1+1\n" +
		"SAMEA4\t'@SUM(1+1)\n" +
		"SAMEA5\tnormal description\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

package main

import (
	"strings"
	"testing"
)

func TestRunIdentifyWritesTSV(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "SAMN05276490", "GCF_000001405.40", "ERZ26912061", "--outfmt", "tsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tnormalized_accession\ttype\tdescription\tena_search\tncbi_search\n" +
		"SAMN05276490\tSAMN05276490\tsample\tSample accession\tyes\tno\n" +
		"GCF_000001405.40\tGCF_000001405\tassembly\tGenome assembly accession\tyes\tyes\n" +
		"ERZ26912061\tERZ26912061\tanalysis\tAnalysis accession\tyes\tno\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunIdentifyDefaultsToHumanReadableTable(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "WP_002248791.1"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Protein or coding sequence accession") {
		t.Fatalf("stdout = %q, want human-readable description", stdout)
	}
	if strings.Contains(stdout, "\t") {
		t.Fatalf("stdout = %q, did not expect tabs in default table output", stdout)
	}
}

func TestRunIdentifyReportsUnknownAccessions(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "not-an-accession", "--outfmt", "tsv"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}

	const want = "input_accession\tnormalized_accession\ttype\tdescription\tena_search\tncbi_search\n" +
		"not-an-accession\t.\tunknown\tUnrecognized accession format\tno\tno\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

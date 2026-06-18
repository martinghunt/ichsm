package main

import (
	"testing"
)

func TestRunOpenPrintsURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "SRR3675520", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ebi.ac.uk/ena/browser/view/SRR3675520\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenPrintsWGSSetURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "AGQU00000000.1", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ebi.ac.uk/ena/browser/view/AGQU00000000.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenUsesBrowserOpener(t *testing.T) {
	var openedURL string
	withTestBrowserOpener(t, func(browserURL string) error {
		openedURL = browserURL
		return nil
	})

	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "-a", "GCA_000195955.2"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}

	const wantURL = "https://www.ebi.ac.uk/ena/browser/view/GCA_000195955.2"
	if openedURL != wantURL {
		t.Fatalf("openedURL = %q, want %q", openedURL, wantURL)
	}
}

func TestRunOpenRejectsInvalidAccession(t *testing.T) {
	code, _ := captureStdout(t, func() int {
		return run([]string{"open", "not-an-accession", "--print-url"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestRunOpenPrintsNCBIProteinURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "WP_002248791.1", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/protein/WP_002248791.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenPrintsNCBIAssemblyURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "GCF_000001405.40", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/datasets/genome/GCF_000001405.40/\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForSharedProteinAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "AAA98665.1", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/protein/AAA98665.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForSharedNucleotideAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "U49845.1", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/nuccore/U49845.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForRunAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "DRR013337", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/sra/?term=DRR013337\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenRejectsNCBIOnlyAccessionWithENASource(t *testing.T) {
	code, _ := captureStdout(t, func() int {
		return run([]string{"open", "WP_002248791.1", "--source", "ena", "--print-url"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

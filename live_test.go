package ichsm

import (
	"context"
	"os"
	"testing"
	"time"
)

const liveTestsEnv = "ICHSM_LIVE_TESTS"

func TestLiveENASearchSmoke(t *testing.T) {
	ctx := liveTestContext(t)

	client := liveTestClient()
	results := liveENASearchWithRetry(t, ctx, client, SearchOptions{
		Accessions: []string{"SRR3675520"},
		Fields:     []string{"run_accession"},
		Source:     SearchSourceENA,
	})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.Source != SearchSourceENA {
		t.Fatalf("source = %q, want %q", result.Source, SearchSourceENA)
	}
	if result.ResultType != AccessionTypeRun {
		t.Fatalf("result type = %q, want %q", result.ResultType, AccessionTypeRun)
	}
	if len(result.Records) == 0 {
		t.Fatal("no ENA records returned")
	}
	if got := result.Records[0]["run_accession"]; got != "SRR3675520" {
		t.Fatalf("run_accession = %q, want SRR3675520", got)
	}
}

func TestLiveNCBISearchSmoke(t *testing.T) {
	ctx := liveTestContext(t)

	client := liveTestClient()
	results, err := client.Search(ctx, SearchOptions{
		Accessions: []string{"GCF_000001405.40"},
		Fields:     []string{"accession", "version", "scientific_name"},
		Source:     SearchSourceNCBI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.Source != SearchSourceNCBI {
		t.Fatalf("source = %q, want %q", result.Source, SearchSourceNCBI)
	}
	if result.ResultType != AccessionTypeAssembly {
		t.Fatalf("result type = %q, want %q", result.ResultType, AccessionTypeAssembly)
	}
	if len(result.Records) == 0 {
		t.Fatal("no NCBI records returned")
	}

	record := result.Records[0]
	if got := record["accession"]; got != "GCF_000001405" {
		t.Fatalf("accession = %q, want GCF_000001405", got)
	}
	if got := record["version"]; got != "40" {
		t.Fatalf("version = %q, want 40", got)
	}
}

func liveTestContext(t *testing.T) context.Context {
	t.Helper()

	if os.Getenv(liveTestsEnv) != "1" {
		t.Skipf("set %s=1 to run live ENA/NCBI smoke tests", liveTestsEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func liveENASearchWithRetry(t *testing.T, ctx context.Context, client *Client, options SearchOptions) []SearchResult {
	t.Helper()

	const maxAttempts = 2
	var lastErr error
	attempts := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attempts = attempt
		results, err := client.Search(ctx, options)
		if err == nil {
			return results
		}

		lastErr = err
		if ctx.Err() != nil {
			break
		}
		if attempt < maxAttempts {
			t.Logf("ENA live smoke attempt %d failed: %v; retrying once", attempt, err)
		}
	}

	t.Fatalf("ENA live smoke failed after %d attempt(s): %v", attempts, lastErr)
	return nil
}

func liveTestClient() *Client {
	client := NewClient()
	client.NCBITool = "ichsm-live-test"
	client.NCBIAPIKey = os.Getenv("NCBI_API_KEY")
	client.NCBIEmail = os.Getenv("NCBI_EMAIL")
	return client
}

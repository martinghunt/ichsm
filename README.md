# ichsm

Finding sequence metadata from ENA and NCBI.

Currently supported: run, experiment, sample, study/project, assembly, INSDC sequence/coding,
WGS/TSA/TLS contig set, and selected NCBI/RefSeq accessions. `ichsm search`
uses `--source auto` by default: it queries ENA first where applicable, then
falls back to NCBI for accessions such as `GCF_`, `NC_`, and `WP_`.

This repository was developed with substantial coding assistance from
[OpenAI Codex](https://openai.com/codex), which helped with implementation,
refactoring, tests, documentation, and benchmarking under human direction and review.


## Install

The simplest way to install `ichsm` is to download the latest prebuilt binary from the GitHub releases page:

- https://github.com/martinghunt/ichsm/releases/latest

Choose the archive or binary matching your OS and CPU architecture.

After installing, check the version with:

```
ichsm --version
```

If you want to build locally instead:

```
./build.sh
```

That builds `ichsm` for the current OS and architecture into `./build/ichsm` or `./build/ichsm.exe`.
Local builds report version `dev` unless you pass an explicit release version.

For a cross-platform release build:

```
./build.sh --release --version v1.2.3
```


## Synopsis

Identify an accession type, its normalized form, and supported metadata sources:
```
ichsm identify SAMN05276490
```

Get metadata for sample `SAMN05276490` in (default) TSV format:
```
ichsm search -a SAMN05276490
```

Get metadata for accessions (one per line, must all be same type eg runs, samples etc)
in the file `acc.txt`:
```
ichsm search -f acc.txt
```

Get metadata for sample `SAMN05276490` in JSON format:
```
ichsm search -a SAMN05276490 --outfmt json
```

Get metadata for sample `SAMN05276490` as an aligned table:
```
ichsm search -a SAMN05276490 --outfmt table
```

Get metadata for sample `SAMN05276490` as a transposed aligned table:
```
ichsm search -a SAMN05276490 --outfmt ttable
```

Get metadata for sample `SAMN05276490` as transposed TSV:
```
ichsm search -a SAMN05276490 --outfmt ttsv
```

Get all available metadata for sample `SAMN05276490`:
```
ichsm search -a SAMN05276490 -c ALL
```

Get runs for sample `SAMN05276490`:
```
ichsm search -a SAMN05276490 --level run
```

Get metadata for study/project `PRJEB1787`:
```
ichsm search -a PRJEB1787
```

Get samples for study/project `PRJEB1787`:
```
ichsm search -a PRJEB1787 --level sample
```

Get runs for study/project `PRJEB1787`:
```
ichsm search -a PRJEB1787 --level run
```

Count runs for study/project `PRJEB1787` without fetching the run metadata:
```
ichsm search -a PRJEB1787 --level run --count
```

Show project, sample, assembly, experiment, run, analysis, and contig set links as a tree:
```
ichsm links SRR3675520
```

Show PubMed publications linked to study/project `PRJEB1787`, including
publications attached to immediate parent projects:
```
ichsm pubs PRJEB1787
```

Get a FASTQ download manifest for sample `SAMN05276490`:
```
ichsm reads -a SAMN05276490
```

Get the FASTQ download manifest as an aligned table:
```
ichsm reads -a SAMN05276490 --outfmt table
```

Print `wget` commands to download FASTQs for sample `SAMN05276490`:
```
ichsm reads -a SAMN05276490 --outfmt wget
```

Print MD5 checksum lines for those FASTQs:
```
ichsm reads -a SAMN05276490 --outfmt md5
```

Open sample `SAMN05276490` in the ENA browser:
```
ichsm open SAMN05276490
```

Print the ENA browser URL for run `SRR3675520`:
```
ichsm open SRR3675520 --print-url
```

Print the NCBI browser URL for a RefSeq assembly:
```
ichsm open GCF_000001405.40 --print-url
```

Print the NCBI protein URL for a RefSeq protein:
```
ichsm open WP_002248791.1 --print-url
```

Force NCBI for an accession that is also available from ENA:
```
ichsm open U49845.1 --source ncbi --print-url
```

Print the NCBI SRA URL for a run accession:
```
ichsm open DRR013337 --source ncbi --print-url
```

List available ENA data types and whether `ichsm search` supports them, with
supported types first:
```
ichsm get_fields --outfmt table
```

List available fields for ENA data type `read_run`:
```
ichsm get_fields read_run
```

Get metadata for study accession `ERP001736`:
```
ichsm search -a ERP001736
```

Get metadata for run `SRR3675520`:
```
ichsm search -a SRR3675520
```

Get metadata for assembly `GCA_000195955.2`:
```
ichsm search -a GCA_000195955.2
```

Get metadata for WGS master accession `AGQU00000000.1`:
```
ichsm search -a AGQU00000000.1
```

Get metadata for TSA master accession `GHIQ00000000.1`:
```
ichsm search -a GHIQ00000000.1
```

Get metadata for INSDC nucleotide sequence `U49845.1`:
```
ichsm search -a U49845.1
```

Get metadata for INSDC coding/protein accession `AAA98665.1`:
```
ichsm search -a AAA98665.1
```

Get metadata for an NCBI RefSeq assembly, falling back to NCBI automatically:
```
ichsm search -a GCF_000001405.40
```

Get metadata for an NCBI protein accession:
```
ichsm search -a WP_002248791.1
```

Force a metadata source when needed:
```
ichsm search -a U49845.1 --source ena
ichsm search -a WP_002248791.1 --source ncbi
```

When NCBI is queried, set `NCBI_API_KEY` and `NCBI_EMAIL`, or pass
`--api-key` and `--email`, to send those values with NCBI E-utilities requests.


## Go library

Import the module and use the client directly:

```go
package main

import (
	"context"
	"fmt"

	"github.com/martinghunt/ichsm"
)

func main() {
	client := ichsm.NewClient()
	results, err := client.Search(context.Background(), ichsm.SearchOptions{
		Accessions: []string{"SAMN05276490"},
		Fields:     []string{"DEFAULT"},
		Level:      ichsm.AccessionTypeRun,
		Source:     ichsm.SearchSourceAuto,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(results[0].Records)
}
```

For ENA FASTQ download manifests, use `ReadFiles`:

```go
files, err := client.ReadFiles(context.Background(), ichsm.ReadFileOptions{
	Accessions: []string{"ERR123456"},
})
if err != nil {
	panic(err)
}

fmt.Println(files[0].URL, files[0].MD5)
```


## For developers

Releases are made from Git tags. The GitHub Actions release workflow runs when a tag matching `v*.*.*` is pushed. It runs the tests, builds binaries for Darwin, Linux, and Windows on amd64 and arm64, then uploads the archives to the GitHub release.

Before tagging, run:

```
go test ./...
./build.sh
```

Then create and push the release tag:

```
git tag -a v1.2.3 -m "ichsm v1.2.3"
git push origin main
git push origin v1.2.3
```

For a local check of the full release matrix:

```
./build.sh --release --version v1.2.3
```

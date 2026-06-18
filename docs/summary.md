# ichsm summary

`ichsm summary` summarizes one accession with linked IDs, counts, and selected
metadata.

For ENA-backed results, it can include linked sample, run, assembly, analysis,
and contig set counts. For study/project accessions, it also includes a
publication count.

## Usage

```
ichsm summary [accession] [flags]
```

## Flags

- `--outfmt`: output format. Default is `ttable`. See
  [Output formats](output-formats.md).
- `--source auto|ena|ncbi`: choose the metadata source. Default is `auto`.
- `--api-key`, `--email`: NCBI settings. These default to `NCBI_API_KEY` and
  `NCBI_EMAIL`.

## Examples

Summarize a study/project:

```
ichsm summary PRJEB1787
```

Write TSV:

```
ichsm summary PRJEB1787 --outfmt tsv
```

Write JSON:

```
ichsm summary PRJEB1787 --outfmt json
```

Force NCBI where supported:

```
ichsm summary GCF_000001405.40 --source ncbi
```

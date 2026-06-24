# Output formats

Many `ichsm` commands use `--outfmt` to choose the output format.

Use row-oriented formats when you want to pipe results into another command or
load them into a spreadsheet. Use transposed formats when you are looking at one
record and want fields down the left-hand side.

## Common formats

| Format | Description | Best for |
| --- | --- | --- |
| `tsv` | Tab-separated rows with a header line. Missing values are written as `.`. | Scripts, spreadsheets, and command-line tools. |
| `table` | Space-aligned table with a header line. | Reading in a terminal. |
| `ttsv` | Transposed tab-separated output. Field names are in the first column. | One record in scripts or notes. |
| `ttable` | Transposed space-aligned output. Field names are in the first column. | Reading one record in a terminal. |
| `json` | Structured JSON. | Programs that need nested or typed data. |

## Command support

| Command | Default | Supported formats |
| --- | --- | --- |
| `identify` | `table` | `json`, `table`, `tsv`, `ttable`, `ttsv` |
| `search` | `tsv` | `json`, `table`, `tsv`, `ttable`, `ttsv` |
| `query` | `tsv` | `json`, `table`, `tsv`, `ttable`, `ttsv` |
| `summary` | `ttable` | `json`, `table`, `tsv`, `ttable`, `ttsv` |
| `reads` | `manifest` | `manifest`, `table`, `ttable`, `ttsv`, `urls`, `wget`, `curl`, `md5` |
| `links` | `tree` | `tree`, `json`, `table`, `tsv` |
| `pubs` | `table` | `json`, `table`, `tsv`, `ttable`, `ttsv` |
| `get_fields` | `tsv` | `table`, `tsv`, `ttable`, `ttsv` |
| `get_values` | `tsv` | `json`, `table`, `tsv`, `ttable`, `ttsv` |

`open` and `completion` do not use `--outfmt`.

## Reads formats

`ichsm reads` has formats that are specific to FASTQ downloads:

| Format | Description |
| --- | --- |
| `manifest` | Tab-separated FASTQ manifest with input accession, run accession, filename, URL, MD5, and byte count. |
| `urls` | One FASTQ URL per line. |
| `wget` | One resumable `wget` command per FASTQ file. |
| `curl` | One resumable `curl` command per FASTQ file. |
| `md5` | MD5 checksum lines suitable for `md5sum -c` after download. |

Use `--protocol https` or `--protocol ftp` to choose the download URL scheme.
Use `--output-dir` to change the printed filenames for `wget`, `curl`, and
`md5`.

## Links formats

`ichsm links` defaults to `tree`, which is meant for quick inspection in a
terminal.

Use `--outfmt tsv` or `--outfmt table` for tabular output with parent and child
accessions in separate columns. Use `--outfmt json` to keep the hierarchy.

## Examples

Write TSV:

```
ichsm search -a SAMN05276490 --outfmt tsv
```

Write JSON:

```
ichsm summary PRJEB1787 --outfmt json
```

Transpose one record:

```
ichsm search -a SAMN05276490 --outfmt ttable
```

Print download commands:

```
ichsm reads -a SAMN05276490 --outfmt wget --output-dir reads
```

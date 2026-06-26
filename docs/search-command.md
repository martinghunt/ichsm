# ichsm search

`ichsm search` searches ENA and NCBI metadata for accessions.

By default it searches metadata at the same level as the input accession. Use
`--level` to fan out from a study or sample to another level, such as runs.

## Usage

```
ichsm search [flags]
```

Provide exactly one input source:

- `-a, --accession`
- `-f, --acc-file`

The accession file must contain one accession per line. Accessions in the same
file must all have the same inferred accession type.

## Common flags

- `--source auto|ena|ncbi`: choose the metadata source. Default is `auto`.
- `--level`: output level. Supported values are `study`, `sample`, `run`,
  `assembly`, `sequence`, `coding`, `analysis`, `contig_set`, `wgs_set`,
  `tsa_set`, and `tls_set`.
- `-c, --columns`: comma-separated fields, or `SMALL`, `DEFAULT`, `BIG`, `ALL`.
  See [Fields and columns](fields-and-columns.md).
- `--fields`: alias for `--columns`.
- `--outfmt`: output format. See [Output formats](output-formats.md).
- `--on-no-results`: how to handle an accession that returns no records. Values
  are `skip`, `empty`, `report`, and `fail`. Default is `skip`.
- `--count`: only count matching ENA records.
- `--api-key`, `--email`: NCBI settings. These default to `NCBI_API_KEY` and
  `NCBI_EMAIL`.

## No-result accessions

Some accessions can appear in linked ENA result types but return no records at
their own search level. For example, a BioSample accession may be present on run
or assembly records while the ENA `sample` result returns no row.

Use `--on-no-results` to choose batch behavior:

- `skip`: write a warning to stderr, omit that accession from output, continue
  with the remaining accessions, and exit non-zero.
- `empty`: include one output record for that accession with requested fields
  empty, continue, and exit non-zero.
- `report`: include one output record for that accession with requested fields
  empty plus `ichsm_status` and `ichsm_error` diagnostic fields, continue, and
  exit non-zero.
- `fail`: stop immediately without writing partial output.

## Examples

Get sample metadata:

```
ichsm search -a SAMN05276490
```

Get metadata for accessions in a file:

```
ichsm search -f acc.txt
```

Include no-result accessions as diagnostic rows:

```
ichsm search -f acc.txt --on-no-results report
```

Get JSON output:

```
ichsm search -a SAMN05276490 --outfmt json
```

Get an aligned table:

```
ichsm search -a SAMN05276490 --outfmt table
```

Get all available metadata fields:

```
ichsm search -a SAMN05276490 -c ALL
```

Get runs for a sample:

```
ichsm search -a SAMN05276490 --level run
```

Get runs for a project:

```
ichsm search -a PRJEB1787 --level run
```

Count runs for a project without fetching run metadata:

```
ichsm search -a PRJEB1787 --level run --count
```

Search NCBI explicitly:

```
ichsm search -a GCF_000001405.40 --source ncbi
```

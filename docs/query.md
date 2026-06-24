# ichsm query

`ichsm query` runs an arbitrary ENA Portal API query for one ENA result type.
Use this when you want metadata selected by ENA fields rather than by a known
accession.

For example, bacterial samples are a taxonomic subtree query:

```
ichsm query --result sample --query 'tax_tree(2)' \
  --columns sample_accession,scientific_name,tax_id
```

`tax_tree(2)` means taxon 2 and descendants. `tax_id=2` matches only records
whose taxon is exactly 2.

## Usage

```
ichsm query [flags]
```

Required flags:

- `--result`: result type to query. Use ENA result names such as `sample` and
  `read_run`, or the `ichsm` alias `run` for `read_run`.
- `--query`: ENA Portal API query string.

## Common flags

- `-c, --columns`: comma-separated fields, or `SMALL`, `DEFAULT`, `BIG`, `ALL`.
  See [Fields and columns](fields-and-columns.md).
- `--fields`: alias for `--columns`.
- `--outfmt`: output format. See [Output formats](output-formats.md).
- `--count`: only count matching ENA records.
- `--limit`: maximum number of records to fetch.
- `--offset`: offset for paging through ENA records.

## Examples

Find bacterial samples:

```
ichsm query --result sample --query 'tax_tree(2)' \
  --columns sample_accession,scientific_name,tax_id
```

Find bacterial Illumina runs:

```
ichsm query --result run \
  --query 'tax_tree(2) AND instrument_platform=ILLUMINA' \
  --columns sample_accession,run_accession,instrument_platform
```

Count bacterial Oxford Nanopore runs without fetching metadata:

```
ichsm query --result read_run \
  --query 'tax_tree(2) AND instrument_platform=OXFORD_NANOPORE' \
  --count
```

Fetch a page of run metadata:

```
ichsm query --result run --query 'tax_tree(2)' \
  --columns sample_accession,run_accession,instrument_platform \
  --limit 100 --offset 200
```

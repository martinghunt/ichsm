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

Use [`ichsm get_fields`](get-fields.md) to list queryable fields. For fields
whose type is `controlled value`, use [`ichsm get_values`](get-values.md) to
list allowed values.

Use [`ichsm match`](match.md) when you need to group query results and require
that each group contains rows matching multiple conditions.

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
- `--verbose`: print download progress to stderr.

## Large queries

The default `--outfmt tsv` path streams ENA TSV rows as they are received, so it
is the safest choice for broad queries such as `tax_tree(2)`. JSON, aligned
table, and transposed output are buffered locally because those formats need the
complete result set.

Progress from `--verbose` is written to stderr. Redirect stdout when you want a
clean result file:

```
ichsm query --verbose --result run --query 'tax_tree(2)' \
  --columns sample_accession,run_accession,instrument_platform \
  > runs.tsv
```

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

List supported platform values:

```
ichsm get_values instrument_platform
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

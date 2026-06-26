# ichsm match

`ichsm match` runs an ENA Portal API query, groups the returned rows by a field,
and keeps groups that satisfy row-level requirements.

This is useful for questions such as "samples that have at least one Illumina
run and at least one Oxford Nanopore or PacBio run".

## Usage

```
ichsm match [flags]
```

Required flags:

- `--result`: result type to query. Use ENA result names such as `read_run`, or
  the `ichsm` alias `run` for `read_run`.
- `--query`: ENA Portal API base query string.
- `--group-by`: field used to group records, such as `sample_accession`.
- `--has`: group requirement. Repeat this flag to require multiple conditions.

## Matching Rules

Each `--has` flag means the group must contain at least one row matching that
requirement. Multiple `--has` flags are combined with group-level AND.

Inside one `--has` requirement:

- `field=value` matches one value.
- `field=value1,value2` matches either value.
- `field=value;other=value` requires both terms on the same row.

For example:

```
--has 'instrument_platform=ILLUMINA'
--has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE'
```

means:

```
group has at least one Illumina row
AND group has at least one PacBio or Oxford Nanopore row
```

## Common flags

- `-c, --columns`: record columns for `--output records`, comma-separated, or
  `SMALL`, `DEFAULT`, `BIG`, `ALL`.
- `--fields`: alias for `--columns`.
- `--output groups|records`: output one row per matching group, or record rows
  from matching groups. Default is `groups`.
- `--record-scope matching|all`: for `--output records`, write only records
  matching at least one `--has` requirement, or all records from matching
  groups. Default is `matching`.
- `--strategy auto|local`: choose the matching strategy. Default is `auto`.
- `--outfmt`: output format. See [Output formats](output-formats.md).
- `--on-no-results`: how to handle a query that returns no matching groups.
  Values are `skip`, `empty`, `error`, and `fail`. Default is `skip`.
- `--limit`: maximum number of ENA records to fetch before grouping with
  `--strategy local`.
- `--offset`: offset for paging through ENA records with `--strategy local`.
- `--verbose`: print match progress to stderr.

## No Matching Groups

Use `--on-no-results` to choose behavior when no groups satisfy all `--has`
requirements:

- `skip`: write a warning to stderr, write the normal empty output shape,
  continue command cleanup, and exit non-zero.
- `empty`: include one placeholder output row or record with empty fields and
  exit non-zero.
- `error`: include one placeholder output row or record with empty fields plus
  `ichsm_status` and `ichsm_error` diagnostic fields and exit non-zero.
- `fail`: stop without writing output and exit non-zero.

## Examples

### Group Output

Find bacterial samples with Illumina and Oxford Nanopore runs. This is the
default `--output groups` mode, so the output has one row per matching sample:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=OXFORD_NANOPORE'
```

The output columns are the group key, the number of records in that group, and
the distinct values for the fields used in `--has`:

```
sample_accession  record_count  instrument_platform
SAMD00000344      5             ILLUMINA;PACBIO_SMRT
```

Find bacterial samples with Illumina and either PacBio or Oxford Nanopore runs:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE'
```

### Matching Records

Use `--output records` to write run rows instead of one row per sample. By
default, this writes only records that match at least one `--has` requirement.
For example, this writes Illumina, PacBio, and Oxford Nanopore runs from samples
that have both Illumina and long-read data:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE' \
  --output records \
  --columns sample_accession,run_accession,instrument_platform,library_layout
```

This is equivalent to `--record-scope matching`, which is the default for
`--output records`.

### All Records From Matched Groups

Use `--record-scope all` when you want every record from each matching group,
including records that did not satisfy any `--has` requirement. For example,
this finds samples with paired Illumina data and writes all run records from
those samples:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA;library_layout=PAIRED' \
  --output records \
  --record-scope all \
  --columns sample_accession,run_accession,instrument_platform,library_layout
```

### Same-Row Terms

Use semicolons inside one `--has` requirement when terms must be true on the
same record. This finds samples that have at least one paired Illumina run and
at least one Oxford Nanopore run:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA;library_layout=PAIRED' \
  --has 'instrument_platform=OXFORD_NANOPORE'
```

### Verbose Progress

Use `--verbose` for broad queries. Progress is written to stderr, so redirect
stdout to keep the result table clean:

```
ichsm match --verbose --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE' \
  --output records \
  --columns sample_accession,run_accession,instrument_platform \
  > matched-runs.tsv
```

## Notes

The default `auto` strategy counts one ENA seed query per `--has` requirement,
fetches the smallest seed first, intersects group IDs across requirements, then
fetches records only for matching groups.

The `auto` strategy streams TSV rows from ENA and keeps only candidate group IDs
and final matching records locally.

Use `--strategy local` to fetch rows matching the base `--query` and apply group
matching locally in memory. This is simpler but can use a lot of RAM for broad
queries.

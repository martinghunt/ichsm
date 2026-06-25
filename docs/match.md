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
- `--output groups|records`: output one row per matching group, or all records
  belonging to matching groups. Default is `groups`.
- `--strategy auto|local`: choose the matching strategy. Default is `auto`.
- `--outfmt`: output format. See [Output formats](output-formats.md).
- `--limit`: maximum number of ENA records to fetch before grouping with
  `--strategy local`.
- `--offset`: offset for paging through ENA records with `--strategy local`.

## Examples

Find bacterial samples with Illumina and Oxford Nanopore runs:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=OXFORD_NANOPORE'
```

Find bacterial samples with Illumina and either PacBio or Oxford Nanopore runs:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA' \
  --has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE'
```

Find samples with paired Illumina data and output all records from matching
groups:

```
ichsm match --result run \
  --query 'tax_tree(2)' \
  --group-by sample_accession \
  --has 'instrument_platform=ILLUMINA;library_layout=PAIRED' \
  --output records \
  --columns sample_accession,run_accession,instrument_platform,library_layout
```

## Notes

The default `auto` strategy counts one ENA seed query per `--has` requirement,
fetches the smallest seed first, intersects group IDs across requirements, then
fetches records only for matching groups.

Use `--strategy local` to fetch rows matching the base `--query` and apply group
matching locally in memory. This is simpler but can use a lot of RAM for broad
queries.

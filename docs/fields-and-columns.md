# Fields and columns

`ichsm search` writes metadata fields as output columns. The available fields
depend on the result level and metadata source.

For ENA-backed searches, field names come from ENA result types such as
`sample`, `read_run`, `study`, `assembly`, `analysis`, `wgs_set`, `tsa_set`,
and `tls_set`. Use [`ichsm get_fields`](get-fields.md) to inspect those result
types and fields.

## Presets

Use `--columns` or `--fields` with one of these presets:

| Preset | Meaning |
| --- | --- |
| `SMALL` | A compact set of identifying fields. |
| `DEFAULT` | The normal field set for the accession type or output level. |
| `BIG` | A broader field set with commonly useful extra metadata. |
| `ALL` | All fields returned by the metadata source. |

`DEFAULT` is used when you do not pass `--columns` or `--fields`.

## Custom field lists

You can pass a comma-separated field list instead of a preset:

```
ichsm search -a SAMN05276490 --columns sample_accession,scientific_name,country
```

The field names must be valid for the result level being queried. For example,
run-level output uses ENA `read_run` fields:

```
ichsm search -a PRJEB1787 --level run --columns run_accession,instrument_platform,fastq_ftp
```

## Finding fields

The [`ichsm get_fields`](get-fields.md) command is the field discovery tool.

List ENA result types and whether `ichsm search` supports them:

```
ichsm get_fields
```

List fields for a result type:

```
ichsm get_fields read_run
```

Sort fields by the `ichsm` preset they belong to:

```
ichsm get_fields read_run --sort ichsm_columns
```

The `ichsm_columns` column reports `SMALL`, `DEFAULT`, `BIG`, `ALL`, or `.`
for fields that are not part of an `ichsm` preset.

See the [`ichsm get_fields` command reference](get-fields.md) for flags and
output options.

## Choosing the result type

The `--level` option controls the result level for `ichsm search`.

For example, this searches at sample level:

```
ichsm search -a SAMN05276490
```

This searches at run level from the same sample accession:

```
ichsm search -a SAMN05276490 --level run
```

When looking up fields for a run-level search, use:

```
ichsm get_fields read_run
```

For a sample-level search, use:

```
ichsm get_fields sample
```

Use [`ichsm get_fields`](get-fields.md) with the matching ENA result type before
building a custom `--columns` list.

## Missing values

In row-oriented text output, missing values are written as `.`. This keeps TSV
and table outputs rectangular even when ENA or NCBI omits a field.

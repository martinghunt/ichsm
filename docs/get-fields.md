# ichsm get_fields

`ichsm get_fields` lists ENA data types or the available fields for one ENA data
type.

It is the companion command for choosing `ichsm search --columns` values. Start
with [Fields and columns](fields-and-columns.md) for the broader workflow, then
use this page as the command reference.

Without an argument, it lists ENA result types and adds an `ichsm_search`
column showing whether `ichsm search` supports that type. Supported types are
listed first.

With a data type argument, it lists available fields and adds an `ichsm_columns`
column showing whether a field appears in the `SMALL`, `DEFAULT`, `BIG`, or
`ALL` presets used by `ichsm search`. For how those presets affect
`ichsm search`, see [Fields and columns](fields-and-columns.md).

## Usage

```
ichsm get_fields [data_type] [flags]
```

## Flags

- `--outfmt`: output format. Default is `tsv`. See
  [Output formats](output-formats.md).
- `--sort ichsm_columns`: sort field rows by `ichsm` column preset.
- `--debug`: more verbose logging.

## Examples

These examples support the workflow described in
[Fields and columns](fields-and-columns.md).

List ENA data types:

```
ichsm get_fields
```

List data types as an aligned table:

```
ichsm get_fields --outfmt table
```

List fields for `read_run`:

```
ichsm get_fields read_run
```

Sort fields by the `ichsm` preset column:

```
ichsm get_fields read_run --sort ichsm_columns
```

Then use the chosen field names with `ichsm search --columns`, as shown in
[Fields and columns](fields-and-columns.md).

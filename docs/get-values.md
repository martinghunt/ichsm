# ichsm get_values

`ichsm get_values` lists ENA controlled vocabulary values for one field.

Use [`ichsm get_fields`](get-fields.md) to find fields whose type is
`controlled value`, then pass one of those field names to `ichsm get_values`.

## Usage

```
ichsm get_values [field] [flags]
```

## Flags

- `--outfmt`: output format. Default is `tsv`. See
  [Output formats](output-formats.md).
- `--debug`: more verbose logging.

## Examples

List sequencing platforms:

```
ichsm get_values instrument_platform
```

List library layouts:

```
ichsm get_values library_layout
```

Use a value in an ENA query:

```
ichsm query --result run \
  --query 'tax_tree(2) AND instrument_platform=OXFORD_NANOPORE' \
  --columns sample_accession,run_accession,instrument_platform
```

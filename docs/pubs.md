# ichsm pubs

`ichsm pubs` shows PubMed publications linked to a study/project accession.

It checks publications linked directly to the project and publications attached
to immediate parent projects. Results include the project accession, relation,
source, PubMed ID, year, journal, DOI, and title.

## Usage

```
ichsm pubs [project_accession] [flags]
```

## Flags

- `--outfmt`: output format. Default is `table`. See
  [Output formats](output-formats.md).
- `--api-key`, `--email`: NCBI settings. These default to `NCBI_API_KEY` and
  `NCBI_EMAIL`.

## Examples

Show linked publications:

```
ichsm pubs PRJEB1787
```

Write TSV:

```
ichsm pubs PRJEB1787 --outfmt tsv
```

Write JSON:

```
ichsm pubs PRJEB1787 --outfmt json
```

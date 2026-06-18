# ichsm identify

`ichsm identify` identifies accession types without querying ENA or NCBI.

Use it when you want to check how `ichsm` will normalize an accession and which
metadata sources can be searched for that accession type.

## Usage

```
ichsm identify [accession ...] [flags]
```

Provide exactly one input source:

- positional accessions
- `-a, --accession`
- `-f, --acc-file`

The accession file must contain one accession per line.

## Output

Default output is an aligned table. See [Output formats](output-formats.md) for
all supported formats.

The output reports:

- input accession
- normalized accession
- inferred type
- short type description
- whether ENA search is supported
- whether NCBI search is supported

## Examples

Identify one accession:

```
ichsm identify SAMN05276490
```

Identify several accessions:

```
ichsm identify SAMN05276490 GCF_000001405.40 ERZ26912061
```

Write TSV:

```
ichsm identify SAMN05276490 --outfmt tsv
```

Read accessions from a file:

```
ichsm identify -f accessions.txt
```

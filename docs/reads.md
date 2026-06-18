# ichsm reads

`ichsm reads` prints FASTQ download manifests or shell commands for an
accession.

It searches ENA run metadata, extracts FASTQ URLs, and can print manifests,
plain URLs, `wget` commands, `curl` commands, or MD5 checksum lines.

## Usage

```
ichsm reads [flags]
```

Provide exactly one input source:

- `-a, --accession`
- `-f, --acc-file`

The accession file must contain one accession per line. Accessions in the same
file must all have the same inferred accession type.

## Flags

- `--outfmt`: output format. Default is `manifest`. See
  [Output formats](output-formats.md).
- `--protocol`: `https` or `ftp`. Default is `https`.
- `-o, --output-dir`: directory to use in printed output filenames.
- `--debug`: more verbose logging.

## Examples

Get a FASTQ download manifest:

```
ichsm reads -a SAMN05276490
```

Get the manifest as an aligned table:

```
ichsm reads -a SAMN05276490 --outfmt table
```

Print plain FASTQ URLs:

```
ichsm reads -a SAMN05276490 --outfmt urls
```

Print `wget` commands:

```
ichsm reads -a SAMN05276490 --outfmt wget --output-dir reads
```

Print `curl` commands:

```
ichsm reads -a SAMN05276490 --outfmt curl --output-dir reads
```

Print MD5 checksum lines:

```
ichsm reads -a SAMN05276490 --outfmt md5 --output-dir reads
```

Use FTP URLs:

```
ichsm reads -a SAMN05276490 --protocol ftp
```

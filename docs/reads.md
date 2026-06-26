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
- `--on-no-results`: how to handle an accession that returns no read records.
  Values are `skip`, `empty`, `error`, and `fail`. Default is `skip`.
- `--debug`: more verbose logging.

## No-result accessions

Use `--on-no-results` to choose behavior when one accession in a batch has no
read records:

- `skip`: write a warning to stderr, omit that accession from output, continue
  with the remaining accessions, and exit non-zero.
- `empty`: for tabular outputs (`manifest`, `table`, `ttable`, `ttsv`), include
  one placeholder row with empty fields, continue, and exit non-zero. For
  `urls`, `wget`, `curl`, and `md5`, skip the accession because placeholder
  commands or checksum lines would be invalid.
- `error`: like `empty`, but tabular placeholder rows include `ichsm_status` and
  `ichsm_error` diagnostic fields.
- `fail`: stop immediately without writing partial output.

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

Include no-result accessions as diagnostic manifest rows:

```
ichsm reads -f acc.txt --on-no-results error
```

Use FTP URLs:

```
ichsm reads -a SAMN05276490 --protocol ftp
```

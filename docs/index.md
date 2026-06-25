# ichsm

`ichsm` (ICanHazSequenceMetadata) finds sequence metadata from ENA and NCBI.

It recognises run, experiment, sample, study/project, assembly, nucleotide
sequence, coding/protein, analysis, and WGS/TSA/TLS contig set accessions.
`ichsm search` uses `--source auto` by default: it queries ENA first where
applicable, then falls back to NCBI for accessions such as `GCF_`, `NC_`, and
`WP_`.

Source code: [github.com/martinghunt/ichsm](https://github.com/martinghunt/ichsm)

## What it looks like

These shortened examples show the shape of the output.

Check what an accession is:

```text
$ ichsm identify SAMN05276490
input_accession  normalized_accession  type    description       ena_search  ncbi_search
SAMN05276490     SAMN05276490           sample  Sample accession  yes         no
```

Choose a few metadata columns:

```text
$ ichsm search -a SAMN05276490 --columns sample_accession,scientific_name,country --outfmt table
input_accession  sample_accession  scientific_name             country
SAMN05276490     SAMN05276490      Mycobacterium tuberculosis  United Kingdom: Oxford
```

Follow sample, assembly, and contig set links:

```text
$ ichsm links SAMN02471593
Project: PRJNA73255
└── Sample: SAMN02471593
    └── Assembly: GCA_000231155
        └── ContigSet: AGQU01000000
```

Turn read metadata into download commands:

```text
$ ichsm reads -a SAMN05276490 --outfmt wget --output-dir reads
wget -c -O 'reads/SRR3675520_1.fastq.gz' 'https://.../SRR3675520_1.fastq.gz'
wget -c -O 'reads/SRR3675520_2.fastq.gz' 'https://.../SRR3675520_2.fastq.gz'
```

## Quick start

1. Install `ichsm`.
2. Check an accession type:

   ```
   ichsm identify SAMN05276490
   ```

3. Get sample metadata:

   ```
   ichsm search -a SAMN05276490
   ```

4. Run an ENA field query, for example bacterial samples:

   ```
   ichsm query --result sample --query 'tax_tree(2)' --columns sample_accession,scientific_name,tax_id
   ```

5. Find grouped ENA records that satisfy row-level requirements:

   ```
   ichsm match --result run --query 'tax_tree(2)' --group-by sample_accession --has 'instrument_platform=ILLUMINA' --has 'instrument_platform=OXFORD_NANOPORE'
   ```

6. Summarize a project:

   ```
   ichsm summary PRJEB1787
   ```

7. Print FASTQ download commands:

   ```
   ichsm reads -a SAMN05276490 --outfmt wget --output-dir reads
   ```

8. Show linked project, sample, assembly, experiment, run, analysis, and
   contig set accessions:

   ```
   ichsm links SRR3675520
   ```

9. Show PubMed publications linked to a project:

   ```
   ichsm pubs PRJEB1787
   ```

10. Open an accession in your browser:

   ```
   ichsm open GCF_000001405.40
   ```

11. List ENA fields for run metadata:

   ```
   ichsm get_fields read_run
   ```

12. List values for an ENA controlled vocabulary field:

   ```
   ichsm get_values instrument_platform
   ```

13. Generate shell completion:

   ```
   ichsm completion zsh
   ```

Most commands default to TSV or an aligned table. See
[Output formats](output-formats.md) for the formats supported by each command.
See [Fields and columns](fields-and-columns.md) for choosing metadata columns.

For more detail, see:

```{toctree}
:maxdepth: 2
:caption: Contents

install
output-formats
fields-and-columns
identify
search-command
query
match
get-fields
get-values
summary
reads
links
pubs
open
completion
```

# ichsm links

`ichsm links` shows linked project, sample, assembly, experiment, run, analysis,
and contig set accessions.

The default output is a tree. Use TSV, table, or JSON when you want structured
output for another tool.

## Usage

```
ichsm links [accession] [flags]
```

## Flags

- `--outfmt`: output format. Default is `tree`. See
  [Output formats](output-formats.md).

## Examples

Show links for a run as a tree:

```
ichsm links SRR3675520
```

Show links for a sample:

```
ichsm links SAMN02471593
```

Example tree output:

```text
Project: PRJNA73255
└── Sample: SAMN02471593
    └── Assembly: GCA_000231155
        └── ContigSet: AGQU01000000
```

Show links for an assembly:

```
ichsm links GCA_000231155
```

Write tabular TSV:

```
ichsm links SRR3675520 --outfmt tsv
```

Write hierarchical JSON:

```
ichsm links SRR3675520 --outfmt json
```

## Supported inputs

`links` is intended for accessions that can be connected through ENA metadata,
including studies/projects, samples, runs, experiments, assemblies, analyses,
and WGS/TSA/TLS contig sets.

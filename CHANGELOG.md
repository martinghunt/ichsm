# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Add `ichsm links <accession>` to print ENA project, sample, experiment, run, analysis, and WGS/TSA/TLS contig set relationships as a project-first tree.
- Add ENA analysis accession support, including `ERZ`, `DRZ`, and `SRZ` accessions.
- Add `--outfmt ttable` and `--outfmt ttsv` for row-oriented output, equivalent to transposing the normal tabular rows before table or TSV formatting.

### Changed
- Avoid redundant ENA study-resolution requests when `links` or count/search queries already have a primary `PRJ...` project accession.
- Limit ENA Portal API requests to 25 per second per process, limit NCBI E-utilities requests to 3 per second or 10 with an API key, and retry HTTP 429/5xx responses with backoff.

## [0.4.0] - 2026-06-17

### Added
- Add `ichsm search --count` to report how many ENA records a search would return without fetching the metadata records.
- Warn before large JSON searches that fan out from a study/project or contig set, so users can choose TSV output for very large result sets.
- Add an `ichsm_columns` column to `ichsm get_fields <data_type>` and `--sort ichsm_columns` to group fields by their lowest matching `-c` preset.

### Changed
- Expand sample search default columns to include ENA's default sample fields plus study, taxonomy, collection date, and country metadata.
- Include ENA/NCBI default descriptions and study accessions in default assembly, WGS, study, and run search columns where available.
- Expand assembly `BIG` search columns with assembly status, naming, WGS, submitter, provenance, and update metadata.
- Order annotated `ichsm get_fields <data_type>` output as `columnId`, `type`, `ichsm_columns`, then `description`.

### Fixed
- Make `ichsm search` fail clearly when any requested accession cannot be searched or returns no results, instead of silently leaving that accession out of the output.
- Include columns found in later records when `ichsm search --columns ALL` writes TSV or table output; records without a unioned column now show `null` for that cell.
- Preserve empty description cells in annotated `ichsm get_fields <data_type>` TSV output so every row has the same field count.
- Retry flaky ENA live smoke tests to reduce release-check noise from transient upstream failures.

## [0.3.0] - 2026-06-10

### Added
- Add public read FASTQ manifest helpers for ENA run metadata, URLs, MD5 sums, and byte counts.
- Add `ichsm open --source ncbi` support for run accessions via the NCBI SRA browser.

### Changed
- Make `ichsm reads` use the public read manifest API instead of private command helpers.

## [0.2.0] - 2026-06-09

### Added
- Add `ichsm identify` to classify accessions, show normalized forms, and report ENA/NCBI search support.
- Add a weekly GitHub Actions live smoke test for the public ENA and NCBI endpoints.
- Add ENA `sequence`, `coding`, `tsa_set`, and `tls_set` search support.
- Add support for WGS/TSA/TLS short set IDs and component-shaped sequence accessions.
- Add `ichsm search --source auto|ena|ncbi`, defaulting to ENA first with NCBI fallback.
- Add `ichsm open --source auto|ena|ncbi` with NCBI browser URLs for NCBI-only or forced NCBI accessions.
- Add NCBI E-utilities metadata fallback for `GCF_`, RefSeq nucleotide, and RefSeq protein accessions.
- Add `ichsm search --api-key` and `--email`, defaulting to `NCBI_API_KEY` and `NCBI_EMAIL`, for NCBI requests.
- Add support for WGS master accessions such as `AGQU00000000.1`.
- Add support for ENA study/project accessions, including `PRJEB`, `PRJDB`, `PRJNA`, `ERP`, `DRP`, and `SRP` accessions.
- Add `ichsm search --level` to choose study, sample, run, or assembly output level where supported by the input accession type.
- Add `ichsm reads` to print FASTQ download manifests, URLs, `wget` commands, `curl` commands, or MD5 checksum lines.
- Add `ichsm open` to open an accession in the ENA browser or print its browser URL.
- Let `ichsm get_fields` list available ENA data types and whether `ichsm search` supports them when no data type is supplied.
- Add aligned table output for `ichsm search`, `ichsm reads`, and `ichsm get_fields`.

### Changed
- Refresh CLI help and Go documentation to describe ENA and NCBI support.
- Rename the project, CLI, Go module path, and release artifacts from `ftep` to `ichsm`.
- Use `ichsm reads --outfmt` for output selection, matching `ichsm search` and `ichsm get_fields`.

### Removed
- Remove `ichsm search --s2r`; use `ichsm search --level run` instead.

## [0.1.0] - 2026-05-29

Release `v0.1.0`, before changelog tracking started in this file.

[Unreleased]: https://github.com/martinghunt/ichsm/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/martinghunt/ichsm/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/martinghunt/ichsm/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/martinghunt/ichsm/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/martinghunt/ichsm/releases/tag/v0.1.0

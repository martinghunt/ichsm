#!/usr/bin/env python3
"""Reproduce the bacterial mixed-platform `ichsm match` output from a TSV dump.

The dump is assumed to already contain ENA read_run rows under taxon 2, so this
script mirrors:

    ichsm match --result run \
      --query 'tax_tree(2)' \
      --group-by sample_accession \
      --has 'instrument_platform=ILLUMINA' \
      --has 'instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE'

It streams the compressed TSV and keeps only per-sample counters and platform
sets in memory.
"""

from __future__ import annotations

import argparse
import csv
import gzip
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import TextIO


DEFAULT_INPUT = Path("/Users/mh46/tmp/metadata.20260317.tsv.gz")
DEFAULT_ILLUMINA_PLATFORM = "ILLUMINA"
DEFAULT_LONG_READ_PLATFORMS = ("PACBIO_SMRT", "OXFORD_NANOPORE")


@dataclass
class SampleSummary:
    record_count: int = 0
    platforms: set[str] = field(default_factory=set)
    has_illumina: bool = False
    has_long_read: bool = False


def raise_csv_field_limit() -> None:
    limit = sys.maxsize
    while True:
        try:
            csv.field_size_limit(limit)
            return
        except OverflowError:
            limit //= 10


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Reproduce mixed-platform sample groups from an ENA run metadata TSV dump.",
    )
    parser.add_argument(
        "input",
        nargs="?",
        type=Path,
        default=DEFAULT_INPUT,
        help=f"gzipped TSV metadata dump (default: {DEFAULT_INPUT})",
    )
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="write matching group TSV here instead of stdout",
    )
    parser.add_argument(
        "--group-by",
        default="sample_accession",
        help="grouping column to mirror ichsm --group-by (default: sample_accession)",
    )
    parser.add_argument(
        "--platform-field",
        default="instrument_platform",
        help="platform column to test and summarize (default: instrument_platform)",
    )
    parser.add_argument(
        "--illumina-platform",
        default=DEFAULT_ILLUMINA_PLATFORM,
        help="platform value that satisfies the Illumina requirement",
    )
    parser.add_argument(
        "--long-read-platforms",
        default=",".join(DEFAULT_LONG_READ_PLATFORMS),
        help="comma-separated platform values satisfying the long-read requirement",
    )
    parser.add_argument(
        "--progress-every",
        type=int,
        default=0,
        help="print progress to stderr every N input rows; 0 disables progress",
    )
    return parser.parse_args()


def split_ena_values(value: str | None) -> list[str]:
    if value is None:
        return []
    value = value.strip()
    if value == "" or value == "." or value == "null":
        return []

    values: list[str] = []
    for part in value.split(";"):
        part = part.strip()
        if part and part != "." and part != "null":
            values.append(part)
    return values


def require_columns(fieldnames: list[str] | None, columns: list[str]) -> None:
    if fieldnames is None:
        raise SystemExit("input TSV has no header row")
    missing = [column for column in columns if column not in fieldnames]
    if missing:
        raise SystemExit(f"missing required column(s): {', '.join(missing)}")


def parse_dump(
    path: Path,
    group_by: str,
    platform_field: str,
    illumina_platform: str,
    long_read_platforms: set[str],
    progress_every: int,
) -> tuple[dict[str, SampleSummary], int]:
    samples: dict[str, SampleSummary] = defaultdict(SampleSummary)
    rows_read = 0

    with gzip.open(path, "rt", newline="") as handle:
        reader = csv.DictReader(handle, delimiter="\t")
        require_columns(reader.fieldnames, [group_by, platform_field])

        for row in reader:
            rows_read += 1
            group_values = split_ena_values(row.get(group_by))
            platform_values = split_ena_values(row.get(platform_field))
            if not group_values:
                continue

            platform_set = set(platform_values)
            row_has_illumina = illumina_platform in platform_set
            row_has_long_read = bool(platform_set & long_read_platforms)

            for group_value in group_values:
                summary = samples[group_value]
                summary.record_count += 1
                summary.platforms.update(platform_values)
                summary.has_illumina = summary.has_illumina or row_has_illumina
                summary.has_long_read = summary.has_long_read or row_has_long_read

            if progress_every > 0 and rows_read % progress_every == 0:
                print(f"read {rows_read} rows; saw {len(samples)} groups", file=sys.stderr)

    return samples, rows_read


def write_matching_groups(
    out: TextIO,
    samples: dict[str, SampleSummary],
    group_by: str,
    platform_field: str,
) -> int:
    writer = csv.writer(out, delimiter="\t", lineterminator="\n")
    writer.writerow([group_by, "record_count", platform_field])

    matching_groups = 0
    for group_value in sorted(samples):
        summary = samples[group_value]
        if not (summary.has_illumina and summary.has_long_read):
            continue
        matching_groups += 1
        writer.writerow(
            [
                group_value,
                summary.record_count,
                ";".join(sorted(summary.platforms)),
            ],
        )

    return matching_groups


def main() -> int:
    raise_csv_field_limit()

    args = parse_args()
    long_read_platforms = {
        value.strip()
        for value in args.long_read_platforms.split(",")
        if value.strip()
    }
    if not long_read_platforms:
        raise SystemExit("--long-read-platforms must contain at least one value")

    samples, rows_read = parse_dump(
        args.input,
        args.group_by,
        args.platform_field,
        args.illumina_platform,
        long_read_platforms,
        args.progress_every,
    )

    if args.output:
        with args.output.open("w", newline="") as out:
            matching_groups = write_matching_groups(out, samples, args.group_by, args.platform_field)
    else:
        matching_groups = write_matching_groups(sys.stdout, samples, args.group_by, args.platform_field)

    print(
        f"read {rows_read} rows; saw {len(samples)} groups; wrote {matching_groups} matching groups",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

# ichsm open

`ichsm open` opens an accession URL in your web browser.

Use `--print-url` when you want to see the selected ENA or NCBI URL without
launching a browser.

## Usage

```
ichsm open [accession] [flags]
```

You can provide the accession as a positional argument or with `-a,
--accession`, but not both.

## Flags

- `--source auto|ena|ncbi`: choose the browser source. Default is `auto`.
- `--print-url`: print the URL instead of opening a browser.

## Examples

Open a sample in the ENA browser:

```
ichsm open SAMN05276490
```

Print the ENA browser URL for a run:

```
ichsm open SRR3675520 --print-url
```

Print the NCBI browser URL for a RefSeq assembly:

```
ichsm open GCF_000001405.40 --print-url
```

Print the NCBI protein URL for a RefSeq protein:

```
ichsm open WP_002248791.1 --print-url
```

Force NCBI for an accession also available from ENA:

```
ichsm open U49845.1 --source ncbi --print-url
```

Print the NCBI SRA URL for a run accession:

```
ichsm open DRR013337 --source ncbi --print-url
```

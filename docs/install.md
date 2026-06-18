# Install

## Download

Download the latest build from the [latest release](https://github.com/martinghunt/ichsm/releases/latest).

Choose the archive or binary that matches your operating system and CPU
architecture. Put the `ichsm` executable somewhere on your `PATH`, then check:

```
ichsm --version
```

## macOS

If macOS Gatekeeper blocks the downloaded binary, allow it in "Privacy &
Security" in the Settings app, or remove the quarantine attribute in a terminal:

```
xattr -d com.apple.quarantine /path/to/ichsm
```

## Windows

Download the Windows build, extract it if needed, and run `ichsm.exe` from a
terminal. If Windows Defender warns about the binary, allow it if you trust the
downloaded release.

## Linux

You may need to make the downloaded file executable:

```
chmod +x ichsm
```

Then move it to a directory on your `PATH`.

## Build locally

Local builds require Go 1.21 or later.

```
./build.sh
```

That builds `ichsm` for the current OS and architecture into `./build/ichsm` or
`./build/ichsm.exe`. Local builds report version `dev` unless you pass an
explicit release version.

For a cross-platform release build:

```
./build.sh --release --version v1.2.3
```

## NCBI settings

Some commands can query NCBI. You can pass credentials per command:

```
ichsm search -a GCF_000001405.40 --api-key "$NCBI_API_KEY" --email you@example.org
```

Or set environment variables:

```
export NCBI_API_KEY=...
export NCBI_EMAIL=you@example.org
```

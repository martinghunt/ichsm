package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

const (
	readsFormatManifest = "manifest"
	readsFormatTable    = "table"
	readsFormatURLs     = "urls"
	readsFormatWget     = "wget"
	readsFormatCurl     = "curl"
	readsFormatMD5      = "md5"
)

type readsOptions struct {
	accession string
	accFile   string
	outfmt    string
	protocol  string
	outputDir string
	debug     bool
}

func newReadsCommand() *cobra.Command {
	opts := readsOptions{
		outfmt:   readsFormatManifest,
		protocol: ichsm.ReadProtocolHTTPS,
	}

	cmd := &cobra.Command{
		Use:   "reads",
		Short: "Print FASTQ download manifests or commands for an accession",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeReads(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.debug, "debug", false, "More verbose logging")
	flags.StringVarP(&opts.accession, "accession", "a", "", "Accession to find reads for")
	flags.StringVarP(&opts.accFile, "acc-file", "f", "", "File of accessions to find reads for, one per line")
	flags.StringVar(&opts.accFile, "acc_file", "", "File of accessions to find reads for, one per line")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: manifest, table, ttable, ttsv, urls, wget, curl, or md5")
	flags.StringVar(&opts.protocol, "protocol", opts.protocol, "Download URL protocol: https or ftp")
	flags.StringVarP(&opts.outputDir, "output-dir", "o", "", "Directory to use in printed output filenames")
	_ = flags.MarkHidden("acc_file")

	return cmd
}

func executeReads(cmd *cobra.Command, opts readsOptions) error {
	if (opts.accession == "") == (opts.accFile == "") {
		return fmt.Errorf("exactly one of -a/--accession or -f/--acc-file is required")
	}

	outfmt, err := parseReadsOutfmt(opts.outfmt)
	if err != nil {
		return err
	}
	protocol, err := parseReadsProtocol(opts.protocol)
	if err != nil {
		return err
	}

	accessions, err := accessionsFromInputs(opts.accession, opts.accFile)
	if err != nil {
		return err
	}

	client := newClient()
	results, err := searchAccessions(cmd.Context(), client, accessions, ichsm.ReadFileFields, ichsm.AccessionTypeRun, ichsm.SearchSourceENA, opts.debug, cmd.ErrOrStderr(), false)
	if err != nil {
		return err
	}

	files, err := ichsm.ReadFilesFromSearchResults(results, ichsm.ReadFileOptions{
		Protocol:  protocol,
		OutputDir: opts.outputDir,
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no FASTQ files found")
	}

	return writeReads(cmd.OutOrStdout(), files, outfmt)
}

func parseReadsOutfmt(outfmt string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(outfmt)) {
	case readsFormatManifest:
		return readsFormatManifest, nil
	case readsFormatTable, "human":
		return readsFormatTable, nil
	case outputFormatTTable:
		return outputFormatTTable, nil
	case outputFormatTTSV:
		return outputFormatTTSV, nil
	case readsFormatURLs:
		return readsFormatURLs, nil
	case readsFormatWget:
		return readsFormatWget, nil
	case readsFormatCurl:
		return readsFormatCurl, nil
	case readsFormatMD5:
		return readsFormatMD5, nil
	default:
		return "", fmt.Errorf("unsupported --outfmt %q; expected manifest, table, ttable, ttsv, urls, wget, curl, or md5", outfmt)
	}
}

func parseReadsProtocol(protocol string) (string, error) {
	parsed, err := ichsm.NormalizeReadFileProtocol(protocol)
	if err != nil {
		return "", fmt.Errorf("unsupported --protocol %q; expected https or ftp", protocol)
	}
	return parsed, nil
}

func writeReads(out io.Writer, files []ichsm.ReadFile, format string) error {
	switch format {
	case readsFormatManifest:
		return writeReadsManifest(out, files)
	case readsFormatTable:
		return writeRowsForOutputFormat(out, readFilesRows(files), outputFormatTable)
	case outputFormatTTable:
		return writeRowsForOutputFormat(out, readFilesRows(files), outputFormatTTable)
	case outputFormatTTSV:
		return writeRowsForOutputFormat(out, readFilesRows(files), outputFormatTTSV)
	case readsFormatURLs:
		for _, file := range files {
			fmt.Fprintln(out, file.URL)
		}
	case readsFormatWget:
		for _, file := range files {
			fmt.Fprintf(out, "wget -c -O %s %s\n", shellQuote(file.OutputPath), shellQuote(file.URL))
		}
	case readsFormatCurl:
		for _, file := range files {
			fmt.Fprintf(out, "curl -L --fail --continue-at - --output %s %s\n", shellQuote(file.OutputPath), shellQuote(file.URL))
		}
	case readsFormatMD5:
		for _, file := range files {
			if file.MD5 == "" {
				return fmt.Errorf("missing MD5 checksum for %s", file.URL)
			}
			fmt.Fprintf(out, "%s  %s\n", file.MD5, file.OutputPath)
		}
	default:
		return fmt.Errorf("unsupported reads format %q", format)
	}

	return nil
}

func writeReadsManifest(out io.Writer, files []ichsm.ReadFile) error {
	return writeDelimitedRows(out, readFilesRows(files), "\t")
}

func readFilesRows(files []ichsm.ReadFile) [][]string {
	rows := [][]string{{"input_accession", "run_accession", "filename", "url", "md5", "bytes"}}
	for _, file := range files {
		rows = append(rows, []string{
			file.InputAccession,
			file.RunAccession,
			file.Filename,
			file.URL,
			emptyAsDot(file.MD5),
			emptyAsDot(file.Bytes),
		})
	}
	return rows
}

func emptyAsDot(value string) string {
	if value == "" {
		return "."
	}
	return value
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

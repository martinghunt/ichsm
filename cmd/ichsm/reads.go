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
	noResults string
	debug     bool
}

func newReadsCommand() *cobra.Command {
	opts := readsOptions{
		outfmt:    readsFormatManifest,
		protocol:  ichsm.ReadProtocolHTTPS,
		noResults: noResultsModeSkip,
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
	flags.StringVar(&opts.noResults, "on-no-results", opts.noResults, "How to handle accessions with no read records: skip, empty, error, or fail")
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
	noResultsMode, err := parseNoResultsMode(opts.noResults)
	if err != nil {
		return err
	}

	accessions, err := accessionsFromInputs(opts.accession, opts.accFile)
	if err != nil {
		return err
	}

	client := newClient()
	searchNoResultsMode := readsSearchNoResultsMode(noResultsMode, outfmt)
	results, searchErr := searchAccessions(cmd.Context(), client, accessions, ichsm.ReadFileFields, ichsm.AccessionTypeRun, ichsm.SearchSourceENA, searchNoResultsMode, opts.debug, cmd.ErrOrStderr(), false)
	if searchErr != nil && !isNoResultsSearchError(searchErr) {
		return searchErr
	}

	files, err := ichsm.ReadFilesFromSearchResults(results, ichsm.ReadFileOptions{
		Protocol:  protocol,
		OutputDir: opts.outputDir,
	})
	if err != nil {
		return err
	}
	noResultAccessions := noResultsSearchAccessions(searchErr)
	if len(files) == 0 && len(noResultAccessions) == 0 {
		return errors.New("no FASTQ files found")
	}

	if len(files) > 0 || readsWritesNoResultsRows(noResultsMode, outfmt, len(noResultAccessions) > 0) {
		if writeErr := writeReadsWithNoResults(cmd.OutOrStdout(), files, noResultAccessions, noResultsMode, outfmt); writeErr != nil {
			return writeErr
		}
	}

	return searchErr
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

func writeReadsWithNoResults(out io.Writer, files []ichsm.ReadFile, noResultAccessions []string, noResultsMode string, format string) error {
	if !readsWritesNoResultsRows(noResultsMode, format, len(noResultAccessions) > 0) {
		return writeReads(out, files, format)
	}

	rows := readFilesRowsWithNoResults(files, noResultAccessions, noResultsMode == noResultsModeError)
	switch format {
	case readsFormatManifest:
		return writeDelimitedRows(out, rows, "\t")
	case readsFormatTable:
		return writeRowsForOutputFormat(out, rows, outputFormatTable)
	case outputFormatTTable:
		return writeRowsForOutputFormat(out, rows, outputFormatTTable)
	case outputFormatTTSV:
		return writeRowsForOutputFormat(out, rows, outputFormatTTSV)
	default:
		return writeReads(out, files, format)
	}
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
			dotIfEmpty(file.MD5),
			dotIfEmpty(file.Bytes),
		})
	}
	return rows
}

func readFilesRowsWithNoResults(files []ichsm.ReadFile, noResultAccessions []string, includeDiagnostics bool) [][]string {
	rows := readFilesRows(files)
	if includeDiagnostics {
		rows[0] = append(rows[0], noResultsStatusField, noResultsErrorField)
		for i := 1; i < len(rows); i++ {
			rows[i] = append(rows[i], ".", ".")
		}
	}

	for _, accession := range noResultAccessions {
		row := []string{accession, ".", ".", ".", ".", "."}
		if includeDiagnostics {
			row = append(row, "no_results", "no results returned")
		}
		rows = append(rows, row)
	}
	return rows
}

func readsSearchNoResultsMode(noResultsMode string, outfmt string) string {
	if readsSupportsNoResultsRows(outfmt) {
		return noResultsMode
	}
	switch noResultsMode {
	case noResultsModeEmpty, noResultsModeError:
		return noResultsModeSkip
	default:
		return noResultsMode
	}
}

func readsWritesNoResultsRows(noResultsMode string, outfmt string, hasNoResults bool) bool {
	return hasNoResults && readsSupportsNoResultsRows(outfmt) && (noResultsMode == noResultsModeEmpty || noResultsMode == noResultsModeError)
}

func readsSupportsNoResultsRows(outfmt string) bool {
	switch outfmt {
	case readsFormatManifest, readsFormatTable, outputFormatTTable, outputFormatTTSV:
		return true
	default:
		return false
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

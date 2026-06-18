package main

import (
	"fmt"
	"io"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

type identifyOptions struct {
	accession string
	accFile   string
	outfmt    string
}

type identifyResult struct {
	InputAccession      string `json:"input_accession"`
	NormalizedAccession string `json:"normalized_accession"`
	Type                string `json:"type"`
	Description         string `json:"description"`
	ENASearch           bool   `json:"ena_search"`
	NCBISearch          bool   `json:"ncbi_search"`
}

func newIdentifyCommand() *cobra.Command {
	opts := identifyOptions{
		outfmt: outputFormatTable,
	}

	cmd := &cobra.Command{
		Use:   "identify [accession ...]",
		Short: "Identify accession types without querying metadata services",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeIdentify(cmd, args, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.accession, "accession", "a", "", "Accession to identify")
	flags.StringVarP(&opts.accFile, "acc-file", "f", "", "File of accessions to identify, one per line")
	flags.StringVar(&opts.accFile, "acc_file", "", "File of accessions to identify, one per line")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	_ = flags.MarkHidden("acc_file")

	return cmd
}

func executeIdentify(cmd *cobra.Command, args []string, opts identifyOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}

	accessions, err := identifyAccessionsFromInputs(args, opts.accession, opts.accFile)
	if err != nil {
		return err
	}

	results, unknownCount := identifyAccessions(accessions)
	if err := writeIdentifyResults(cmd.OutOrStdout(), results, outfmt); err != nil {
		return err
	}
	if unknownCount > 0 {
		return fmt.Errorf("could not identify %d accession(s)", unknownCount)
	}
	return nil
}

func identifyAccessionsFromInputs(args []string, accession string, accFile string) ([]string, error) {
	sources := 0
	if len(args) > 0 {
		sources++
	}
	if accession != "" {
		sources++
	}
	if accFile != "" {
		sources++
	}
	if sources != 1 {
		return nil, fmt.Errorf("exactly one of positional accessions, -a/--accession, or -f/--acc-file is required")
	}
	if len(args) > 0 {
		return args, nil
	}
	return accessionsFromInputs(accession, accFile)
}

func identifyAccessions(accessions []string) ([]identifyResult, int) {
	results := make([]identifyResult, 0, len(accessions))
	var unknownCount int
	for _, accession := range accessions {
		normalized, accessionType, ok := ichsm.IdentifyAccession(accession)
		if !ok {
			unknownCount++
			results = append(results, identifyResult{
				InputAccession:      accession,
				NormalizedAccession: ".",
				Type:                "unknown",
				Description:         "Unrecognized accession format",
			})
			continue
		}

		results = append(results, identifyResult{
			InputAccession:      accession,
			NormalizedAccession: normalized,
			Type:                string(accessionType),
			Description:         accessionTypeDescription(accessionType),
			ENASearch:           ichsm.SupportsENA(accessionType),
			NCBISearch:          ichsm.SupportsNCBI(accessionType),
		})
	}
	return results, unknownCount
}

func writeIdentifyResults(out io.Writer, results []identifyResult, outfmt string) error {
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, results)
	}

	return writeRowsForOutputFormat(out, identifyRows(results), outfmt)
}

func identifyRows(results []identifyResult) [][]string {
	rows := [][]string{{
		"input_accession",
		"normalized_accession",
		"type",
		"description",
		"ena_search",
		"ncbi_search",
	}}
	for _, result := range results {
		rows = append(rows, []string{
			result.InputAccession,
			result.NormalizedAccession,
			result.Type,
			result.Description,
			yesNo(result.ENASearch),
			yesNo(result.NCBISearch),
		})
	}
	return rows
}

func accessionTypeDescription(accessionType ichsm.AccessionType) string {
	switch accessionType {
	case ichsm.AccessionTypeAssembly:
		return "Genome assembly accession"
	case ichsm.AccessionTypeContigSet:
		return "WGS/TSA/TLS contig set accession"
	case ichsm.AccessionTypeWGSSet:
		return "WGS contig set accession"
	case ichsm.AccessionTypeTSASet:
		return "TSA contig set accession"
	case ichsm.AccessionTypeTLSSet:
		return "TLS contig set accession"
	case ichsm.AccessionTypeSequence:
		return "Nucleotide sequence accession"
	case ichsm.AccessionTypeCoding:
		return "Protein or coding sequence accession"
	case ichsm.AccessionTypeStudy:
		return "Study or project accession"
	case ichsm.AccessionTypeSample:
		return "Sample accession"
	case ichsm.AccessionTypeRun:
		return "Read run accession"
	case ichsm.AccessionTypeExperiment:
		return "Read experiment accession"
	case ichsm.AccessionTypeAnalysis:
		return "Analysis accession"
	default:
		return "Unrecognized accession format"
	}
}

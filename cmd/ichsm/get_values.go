package main

import (
	"fmt"
	"io"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

type getValuesOptions struct {
	outfmt string
	debug  bool
}

func newGetValuesCommand() *cobra.Command {
	opts := getValuesOptions{
		outfmt: outputFormatTSV,
	}

	cmd := &cobra.Command{
		Use:   "get_values [field]",
		Short: "List values for an ENA controlled vocabulary field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeGetValues(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.debug, "debug", false, "More verbose logging")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	return cmd
}

func executeGetValues(cmd *cobra.Command, field string, opts getValuesOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}

	if opts.debug {
		fmt.Fprintf(cmd.ErrOrStderr(), "getting controlled vocabulary values for %s\n", field)
	}

	values, err := newClient().GetControlledVocabulary(cmd.Context(), field)
	if err != nil {
		return err
	}
	return writeGetValues(cmd.OutOrStdout(), values, outfmt)
}

func writeGetValues(out io.Writer, values []ichsm.ENAControlledVocabValue, outfmt string) error {
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, values)
	}
	return writeRowsForOutputFormat(out, getValuesRows(values), outfmt)
}

func getValuesRows(values []ichsm.ENAControlledVocabValue) [][]string {
	rows := [][]string{{"value", "description"}}
	for _, value := range values {
		rows = append(rows, []string{
			value.Value,
			dotIfEmpty(value.Description),
		})
	}
	return rows
}

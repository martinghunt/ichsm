package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

type queryOptions struct {
	result  string
	query   string
	columns string
	outfmt  string
	limit   int
	offset  int
	count   bool
	debug   bool
	verbose bool
}

type queryCountResult struct {
	ResultType ichsm.AccessionType `json:"result_type"`
	ENAResult  string              `json:"ena_result"`
	Query      string              `json:"query"`
	Count      int                 `json:"count"`
}

func newQueryCommand() *cobra.Command {
	opts := queryOptions{
		columns: "DEFAULT",
		outfmt:  outputFormatTSV,
	}
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Run an arbitrary ENA Portal API query",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeQuery(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.debug, "debug", false, "More verbose logging")
	flags.BoolVar(&opts.verbose, "verbose", false, "Print download progress to stderr")
	flags.StringVarP(&opts.result, "result", "r", "", "ENA result type to query, such as sample or read_run")
	flags.StringVarP(&opts.query, "query", "q", "", "ENA Portal API query string, such as tax_tree(2)")
	flags.StringVarP(&opts.columns, "columns", "c", opts.columns, "Columns/fields to output, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.columns, "fields", opts.columns, "Columns/fields to output, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	flags.IntVar(&opts.limit, "limit", 0, "Maximum number of ENA records to fetch; 0 means no explicit limit")
	flags.IntVar(&opts.offset, "offset", 0, "ENA result offset for paging")
	flags.BoolVar(&opts.count, "count", false, "Only count matching ENA records; do not fetch metadata")
	return cmd
}

func executeQuery(cmd *cobra.Command, opts queryOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}

	client := newClient()
	fields := parseColumnList(opts.columns)
	queryOpts := ichsm.ENAQueryOptions{
		Result: opts.result,
		Query:  opts.query,
		Fields: fields,
		Limit:  opts.limit,
		Offset: opts.offset,
	}

	if opts.count {
		resultType, enaResult, count, err := client.CountENAQuery(cmd.Context(), queryOpts)
		if err != nil {
			return err
		}
		result := queryCountResult{
			ResultType: resultType,
			ENAResult:  enaResult,
			Query:      strings.TrimSpace(opts.query),
			Count:      count,
		}
		return writeQueryCountResult(cmd.OutOrStdout(), result, outfmt)
	}

	if opts.debug {
		logQuery(opts)
	}

	if outfmt == outputFormatTSV && !requestedAllFields(fields) {
		return streamQueryTSV(cmd.Context(), client, queryOpts, cmd.OutOrStdout(), opts.verbose, cmd.ErrOrStderr())
	}

	result, err := client.QueryENA(cmd.Context(), queryOpts)
	if err != nil {
		return err
	}

	if outfmt == outputFormatJSON {
		return writeJSONValue(cmd.OutOrStdout(), result.Records)
	}
	return writeRowsForOutputFormat(cmd.OutOrStdout(), queryRows(result), outfmt)
}

func streamQueryTSV(ctx context.Context, client *ichsm.Client, opts ichsm.ENAQueryOptions, out io.Writer, verbose bool, errOut io.Writer) error {
	var columns []string
	var records int
	_, err := client.StreamENATSV(ctx, opts, func(result ichsm.ENAQueryResult) error {
		columns = append([]string(nil), result.Fields...)
		_, err := fmt.Fprintln(out, strings.Join(columns, "\t"))
		return err
	}, func(record ichsm.Record) error {
		records++
		row := make([]string, 0, len(columns))
		for _, column := range columns {
			row = append(row, formatRecordColumn(record, column, false))
		}
		row = sanitizeTabularRow(row)
		if _, err := fmt.Fprintln(out, strings.Join(row, "\t")); err != nil {
			return err
		}
		if verbose && records%100000 == 0 {
			fmt.Fprintf(errOut, "downloaded %d records\n", records)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if verbose {
		fmt.Fprintf(errOut, "downloaded %d records\n", records)
	}
	return nil
}

func parseColumnList(columns string) []string {
	parts := strings.Split(columns, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func logQuery(opts queryOptions) {
	message := fmt.Sprintf("query ENA result %s with %q", strings.TrimSpace(opts.result), strings.TrimSpace(opts.query))
	if opts.limit > 0 {
		message += fmt.Sprintf(" limit %d", opts.limit)
	}
	if opts.offset > 0 {
		message += fmt.Sprintf(" offset %d", opts.offset)
	}
	log.Print(message)
}

func writeQueryCountResult(out io.Writer, result queryCountResult, outfmt string) error {
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, result)
	}
	return writeRowsForOutputFormat(out, queryCountRows(result), outfmt)
}

func queryCountRows(result queryCountResult) [][]string {
	return [][]string{
		{"result_type", "ena_result", "query", "count"},
		{string(result.ResultType), result.ENAResult, result.Query, fmt.Sprint(result.Count)},
	}
}

func queryRows(result ichsm.ENAQueryResult) [][]string {
	columns := result.Fields
	allFields := requestedAllFields(columns)
	if allFields {
		columns = queryRecordKeys(result.Records)
	}
	if len(columns) == 0 {
		return nil
	}

	rows := [][]string{append([]string(nil), columns...)}
	for _, record := range result.Records {
		row := make([]string, 0, len(columns))
		for _, column := range columns {
			row = append(row, formatRecordColumn(record, column, allFields))
		}
		rows = append(rows, row)
	}
	return rows
}

func queryRecordKeys(records []ichsm.Record) []string {
	keySet := map[string]bool{}
	for _, record := range records {
		for key := range record {
			keySet[key] = true
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

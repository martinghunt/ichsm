package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

type pubsOptions struct {
	outfmt string
	apiKey string
	email  string
}

func newPubsCommand() *cobra.Command {
	opts := pubsOptions{
		outfmt: outputFormatTable,
	}
	cmd := &cobra.Command{
		Use:   "pubs [project_accession]",
		Short: "Show PubMed publications linked to a project/study accession",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executePubs(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	flags.StringVar(&opts.apiKey, "api-key", "", "NCBI API key; defaults to NCBI_API_KEY")
	flags.StringVar(&opts.email, "email", "", "Email address sent to NCBI; defaults to NCBI_EMAIL")
	return cmd
}

func executePubs(cmd *cobra.Command, accession string, opts pubsOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}

	client := newNCBIConfiguredClient(opts.apiKey, opts.email)

	publications, err := client.Publications(cmd.Context(), accession)
	if err != nil {
		return err
	}
	if len(publications) == 0 {
		return fmt.Errorf("no publications found for accession %s", accession)
	}

	return writePublications(cmd.OutOrStdout(), publications, outfmt)
}

func writePublications(out io.Writer, publications []ichsm.Publication, outfmt string) error {
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, publications)
	}

	return writeRowsForOutputFormat(out, publicationRows(publications), outfmt)
}

func publicationRows(publications []ichsm.Publication) [][]string {
	rows := [][]string{{"input_accession", "project_accession", "relation", "sources", "pubmed_id", "year", "journal", "doi", "title"}}
	for _, publication := range publications {
		rows = append(rows, []string{
			publication.InputAccession,
			publication.ProjectAccession,
			publication.Relation,
			strings.Join(publication.Sources, ","),
			publication.PubMedID,
			formatPublicationCell(publication.Year),
			formatPublicationCell(publication.Journal),
			formatPublicationCell(publication.DOI),
			formatPublicationCell(publication.Title),
		})
	}
	return rows
}

func formatPublicationCell(value string) string {
	if value == "" {
		return "."
	}
	return value
}

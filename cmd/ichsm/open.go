package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

const (
	enaBrowserViewBaseURL      = "https://www.ebi.ac.uk/ena/browser/view/"
	ncbiAssemblyBrowserBaseURL = "https://www.ncbi.nlm.nih.gov/datasets/genome/"
	ncbiNuccoreBrowserBaseURL  = "https://www.ncbi.nlm.nih.gov/nuccore/"
	ncbiProteinBrowserBaseURL  = "https://www.ncbi.nlm.nih.gov/protein/"
	ncbiSRABrowserBaseURL      = "https://www.ncbi.nlm.nih.gov/sra/?term="
)

var openBrowser = openURLInBrowser

type openOptions struct {
	accession string
	source    string
	printURL  bool
}

func newOpenCommand() *cobra.Command {
	var opts openOptions

	cmd := &cobra.Command{
		Use:   "open [accession]",
		Short: "Open an accession URL in your web browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeOpen(cmd, args, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.accession, "accession", "a", "", "Accession to open")
	flags.StringVar(&opts.source, "source", string(ichsm.SearchSourceAuto), "Browser source: auto, ena, or ncbi")
	flags.BoolVar(&opts.printURL, "print-url", false, "Print the ENA or NCBI URL instead of opening your web browser")

	return cmd
}

func executeOpen(cmd *cobra.Command, args []string, opts openOptions) error {
	accession, err := openAccessionFromInputs(opts.accession, args)
	if err != nil {
		return err
	}

	source, err := parseSearchSource(opts.source)
	if err != nil {
		return err
	}

	browserURL, err := accessionBrowserURL(accession, source)
	if err != nil {
		return err
	}

	if opts.printURL {
		fmt.Fprintln(cmd.OutOrStdout(), browserURL)
		return nil
	}

	if err := openBrowser(browserURL); err != nil {
		return fmt.Errorf("error opening browser: %w; try --print-url", err)
	}
	return nil
}

func openAccessionFromInputs(flagAccession string, args []string) (string, error) {
	if flagAccession != "" && len(args) > 0 {
		return "", fmt.Errorf("provide accession either as an argument or with -a/--accession, not both")
	}
	if flagAccession != "" {
		return flagAccession, nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return "", fmt.Errorf("accession is required")
}

func accessionBrowserURL(accession string, source ichsm.SearchSource) (string, error) {
	accession = strings.TrimSpace(accession)
	if accession == "" {
		return "", fmt.Errorf("accession is required")
	}

	_, accessionType, ok := ichsm.IdentifyAccession(accession)
	if !ok {
		return "", fmt.Errorf("accession format not recognised: %s", accession)
	}

	source, err := openBrowserSource(accession, accessionType, source)
	if err != nil {
		return "", err
	}

	switch source {
	case ichsm.SearchSourceENA:
		return enaBrowserViewBaseURL + url.PathEscape(accession), nil
	case ichsm.SearchSourceNCBI:
		return ncbiBrowserURL(accession, accessionType), nil
	default:
		return "", fmt.Errorf("unsupported source %q; expected auto, ena, or ncbi", source)
	}
}

func openBrowserSource(accession string, accessionType ichsm.AccessionType, source ichsm.SearchSource) (ichsm.SearchSource, error) {
	switch source {
	case ichsm.SearchSourceAuto:
		if enaBrowserSupports(accession, accessionType) {
			return ichsm.SearchSourceENA, nil
		}
		if ichsm.SupportsNCBIBrowser(accessionType) {
			return ichsm.SearchSourceNCBI, nil
		}
	case ichsm.SearchSourceENA:
		if enaBrowserSupports(accession, accessionType) {
			return ichsm.SearchSourceENA, nil
		}
		return "", fmt.Errorf("accession is not supported by the ENA browser: %s", accession)
	case ichsm.SearchSourceNCBI:
		if ichsm.SupportsNCBIBrowser(accessionType) {
			return ichsm.SearchSourceNCBI, nil
		}
		return "", fmt.Errorf("accession is not supported by the NCBI browser: %s", accession)
	}

	return "", fmt.Errorf("accession is not supported by an available browser: %s", accession)
}

func enaBrowserSupports(accession string, accessionType ichsm.AccessionType) bool {
	upper := strings.ToUpper(strings.TrimSpace(accession))
	switch accessionType {
	case ichsm.AccessionTypeAssembly:
		return strings.HasPrefix(upper, "GCA_")
	case ichsm.AccessionTypeSequence, ichsm.AccessionTypeCoding:
		return !strings.Contains(upper, "_")
	default:
		return true
	}
}

func ncbiBrowserURL(accession string, accessionType ichsm.AccessionType) string {
	accession = strings.ToUpper(strings.TrimSpace(accession))
	switch accessionType {
	case ichsm.AccessionTypeAssembly:
		return ncbiAssemblyBrowserBaseURL + url.PathEscape(accession) + "/"
	case ichsm.AccessionTypeCoding:
		return ncbiProteinBrowserBaseURL + url.PathEscape(accession)
	case ichsm.AccessionTypeRun:
		return ncbiSRABrowserBaseURL + url.QueryEscape(accession)
	default:
		return ncbiNuccoreBrowserBaseURL + url.PathEscape(accession)
	}
}

func openURLInBrowser(browserURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", browserURL).Run()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", browserURL).Run()
	default:
		return exec.Command("xdg-open", browserURL).Run()
	}
}

package main

import (
	"fmt"
	"os"

	"github.com/martinghunt/ichsm/internal/selfupdate"
	"github.com/spf13/cobra"
)

var runSelfUpdate = selfupdate.Update

func newUpdateCommand() *cobra.Command {
	var (
		checkOnly bool
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update ichsm to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runSelfUpdate(cmd.Context(), selfupdate.Options{
				CurrentVersion: version,
				CheckOnly:      checkOnly,
				Force:          force,
				Token:          githubToken(),
			})
			if err != nil {
				return err
			}
			return printUpdateResult(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for an update without installing")
	cmd.Flags().BoolVar(&force, "force", false, "Install the latest release even when the current version is not older")
	return cmd
}

func githubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

func printUpdateResult(cmd *cobra.Command, result selfupdate.Result) error {
	out := cmd.OutOrStdout()
	switch {
	case result.Updated && displayVersion(result.CurrentVersion) == displayVersion(result.LatestVersion):
		_, err := fmt.Fprintf(out, "installed ichsm %s\n", displayVersion(result.LatestVersion))
		return err
	case result.Updated:
		_, err := fmt.Fprintf(out, "updated ichsm from %s to %s\n", displayVersion(result.CurrentVersion), displayVersion(result.LatestVersion))
		return err
	case result.UpToDate:
		_, err := fmt.Fprintf(out, "ichsm is up to date (%s)\n", displayVersion(result.CurrentVersion))
		return err
	case result.CheckOnly:
		_, err := fmt.Fprintf(out, "ichsm %s is available (current %s)\n", displayVersion(result.LatestVersion), displayVersion(result.CurrentVersion))
		return err
	default:
		return nil
	}
}

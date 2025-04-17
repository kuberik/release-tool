package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "release-tool",
		Short: "A tool for managing releases",
		Long:  `A command line tool for managing releases with semantic versioning.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Hello, World!")
			return nil
		},
	}

	rootCmd.AddCommand(NewPublishCmd())
	return rootCmd
}

func Execute() error {
	return NewRootCmd().Execute()
}

package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "release-tool",
		Short: "A tool for managing releases",
		Long:  `A command line tool for managing releases with semantic versioning.`,
	}

	rootCmd.AddCommand(NewPublishCmd())
	rootCmd.AddCommand(NewOciCmd())
	rootCmd.AddCommand(NewVersionCmd())
	return rootCmd
}

func Execute() error {
	return NewRootCmd().Execute()
}

package cli

import (
	"github.com/spf13/cobra"

	workspacecmd "github.com/nasij/nasij/internal/cli/workspace"
	"github.com/nasij/nasij/internal/container"
)

func NewRoot(c *container.Container) *cobra.Command {
	root := &cobra.Command{
		Use:           "nasij",
		Short:         "NASIJ — Intelligent JavaScript Reconnaissance Framework",
		Long:          `A scope-gated, passive-first JavaScript reconnaissance framework for authorized security assessments.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.SetHelpFunc(helpFunc(c))

	root.AddCommand(
		newVersionCmd(c),
		newDoctorCmd(c),
		newInitCmd(c),
		newScanCmd(c),
		newCrawlCmd(c),
		workspacecmd.NewWorkspaceCmd(c),
		newConfigCmd(c),
		newPluginsCmd(c),
		newReportCmd(c),
	)

	root.PersistentFlags().String("config", "", "path to config file")
	root.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")
	root.PersistentFlags().BoolP("quiet", "q", false, "suppress non-essential output")

	return root
}

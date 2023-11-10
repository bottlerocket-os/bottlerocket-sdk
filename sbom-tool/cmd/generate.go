package cmd

import (
	"github.com/anchore/clio"
	"github.com/anchore/syft/cmd/syft/cli"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an SBOM file from a source directory or image",
	RunE:  generate,
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

func generate(cmd *cobra.Command, args []string) error {
	id := clio.Identification{
		Name:    "bottlerocket-sbom-tool",
		Version: "v1.0.0",
	}

	syftArgs := []string{
		"--source-name",
		"host-ctr",
		"-o",
		"spdx-json",
		args[0],
	}

	// syftArgs = append(syftArgs, args...)
	// syftArgs = append(syftArgs, "> /tmp/bom/host-ctr.spdx.json")

	genCmd := cli.Command(id)
	genCmd.SetArgs(syftArgs)
	return genCmd.Execute()
}

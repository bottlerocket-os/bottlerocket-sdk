package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/interlynk-io/sbomasm/pkg/assemble"
	"github.com/interlynk-io/sbomasm/pkg/logger"
	"github.com/spf13/cobra"
)

// combineCmd represents the combine command
var combineCmd = &cobra.Command{
	Use:   "combine",
	Short: "Combines multiple SBOM files into one",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := assemble.NewParams()
		params.Output = opts.OutputPath
		params.Name = opts.Name
		params.Version = opts.Version
		params.Type = opts.Purpose
		params.HierMerge = true
		params.Json = true

		for _, inputFile := range args {
			if !isValidFile(inputFile) {
				return fmt.Errorf("%q is not a valid file", inputFile)
			}

			params.Input = append(params.Input, inputFile)
		}

		if opts.Debug {
			logger.InitDebugLogger()
		} else {
			logger.InitProdLogger()
		}

		ctx := logger.WithLogger(context.Background())
		params.Ctx = &ctx

		return assemble.Assemble(params)
	},
}

// isValidFile verifies the input points to an actual file
func isValidFile(input string) bool {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) == 0 {
		return false
	}

	if _, err := os.Stat(trimmed); errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

// assembleOpts are the possible input options to the combine command
type assembleOpts struct {
	Name       string
	OutputPath string
	Purpose    string
	Version    string
	Debug      bool
}

var opts = assembleOpts{}

// init registers this subcommand and sets up the command line options
func init() {
	rootCmd.AddCommand(combineCmd)

	// Flags and settings for the combine command
	combineCmd.Flags().StringVarP(&opts.OutputPath, "output", "o", "", "File to write output to, default is STDOUT")
	combineCmd.Flags().StringVarP(&opts.Name, "name", "n", "bottlerocket", "The name for the overall combined package")
	combineCmd.Flags().StringVarP(&opts.Purpose, "purpose", "p", "OPERATING-SYSTEM", "The primary purpose of the SBOM")
	combineCmd.Flags().StringVarP(&opts.Version, "version", "", "", "The version of the combined package")
	combineCmd.MarkFlagRequired("version")

	combineCmd.Flags().BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug logging")
}

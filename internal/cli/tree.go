package cli

import (
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/region"
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree [dir]",
	Short: "Generate tree view from region markers in source files",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := projectRoot()
		if len(args) > 0 {
			dir = args[0]
		}

		gamignore := region.ParseGamignore(projectRoot())

		markers, warnings, err := region.ScanDirectory(dir, gamignore)
		if err != nil {
			return fmt.Errorf("scan directory: %w", err)
		}

		tree := region.BuildTree(markers)
		fmt.Print(region.FormatTree(tree, "", true))

		if len(warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, w := range warnings {
				fmt.Printf("  %s\n", w)
			}
		}

		// Check for unregioned code
		unregioned, _ := region.FindUnregionedCode(dir, gamignore)
		if len(unregioned) > 0 {
			fmt.Println("\n⚠ UNREGIONED CODE:")
			for _, f := range unregioned {
				fmt.Printf("  %s (no region markers — add to .gamignore or wrap in region)\n", f)
			}
		}

		// Check for arch.md mismatches
		archPaths, _ := region.ParseArchMd(projectRoot())
		if len(archPaths) > 0 {
			markerPaths := make(map[string]bool)
			for _, m := range markers {
				markerPaths[m.Path] = true
			}

			var mismatches []string
			for _, ap := range archPaths {
				if !markerPaths[ap] {
					mismatches = append(mismatches, ap)
				}
			}
			if len(mismatches) > 0 {
				fmt.Println("\n⚠ ARCH.MD MISMATCH:")
				for _, m := range mismatches {
					fmt.Printf("  %s exists in arch.md but has no code regions\n", m)
				}
			}
		}

		return nil
	},
}

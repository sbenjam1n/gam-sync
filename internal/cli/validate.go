package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/gam"
	"github.com/sbenjam1n/gamsync/internal/region"
	"github.com/sbenjam1n/gamsync/internal/validator"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Run Tier 0 + Tier 1 validation against a region or file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		ctx := context.Background()

		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		v := validator.New(pool, projectRoot())

		if all {
			// Validate all regions
			rows, err := pool.Query(ctx, `SELECT path FROM regions ORDER BY path`)
			if err != nil {
				return err
			}
			defer rows.Close()

			passed := 0
			failed := 0
			for rows.Next() {
				var path string
				rows.Scan(&path)

				// Create a minimal proposal for structural validation
				proposal := &gam.Proposal{
					RegionPath: path,
				}
				result := v.Tier0Structural(ctx, proposal)
				if result.Passed {
					passed++
				} else {
					failed++
					fmt.Printf("FAIL %s: %s\n", path, result.Message)
					for _, d := range result.Details {
						if !d.Passed && d.Fix != "" {
							fmt.Printf("  Fix: %s\n", d.Fix)
						}
					}
				}
			}

			fmt.Printf("\n%d passed, %d failed\n", passed, failed)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("specify a region path or use --all")
		}

		regionPath := args[0]

		// Structural validation
		fmt.Printf("Validating %s...\n", regionPath)

		// Check region markers in source files
		gamignore := region.ParseGamignore(projectRoot())
		markers, warnings, _ := region.ScanDirectory(projectRoot(), gamignore)

		found := false
		for _, m := range markers {
			if m.Path == regionPath {
				found = true
				fmt.Printf("  Region markers: found in %s:%d-%d\n", m.File, m.StartLine, m.EndLine)
			}
		}
		if !found {
			fmt.Printf("  Region markers: NOT FOUND in source files\n")
		}

		for _, w := range warnings {
			fmt.Printf("  Warning: %s\n", w)
		}

		// Database validation
		proposal := &gam.Proposal{
			RegionPath: regionPath,
		}

		result := v.Tier0Structural(ctx, proposal)
		fmt.Printf("  Tier 0 (Structural): %s\n", formatValidationResult(result))

		if result.Passed {
			result1, err := v.Tier1StateMachine(ctx, proposal)
			if err != nil {
				fmt.Printf("  Tier 1: ERROR: %v\n", err)
			} else {
				fmt.Printf("  Tier 1 (State Machine): %s\n", formatValidationResult(result1))
			}
		}

		return nil
	},
}

func formatValidationResult(r *gam.ValidationResult) string {
	if r.Passed {
		return "PASSED"
	}
	result := fmt.Sprintf("FAILED (code %d): %s", r.Code, r.Message)
	for _, d := range r.Details {
		if !d.Passed && d.Fix != "" {
			result += fmt.Sprintf("\n    Fix: %s", d.Fix)
		}
	}
	return result
}

func init() {
	validateCmd.Flags().Bool("all", false, "Validate entire project")
}

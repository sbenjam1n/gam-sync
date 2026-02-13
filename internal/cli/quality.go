package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/memorizer"
	"github.com/spf13/cobra"
)

var qualityCmd = &cobra.Command{
	Use:   "quality",
	Short: "Quality and gardening",
}

var qualityGradesCmd = &cobra.Command{
	Use:   "grades",
	Short: "Show quality grades for all regions",
	RunE: func(cmd *cobra.Command, args []string) error {
		regionFilter, _ := cmd.Flags().GetString("region")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		query := `
			SELECT r.path, qg.category, qg.grade
			FROM quality_grades qg
			JOIN regions r ON r.id = qg.region_id
			ORDER BY r.path, qg.category
		`
		queryArgs := []any{}
		if regionFilter != "" {
			query = `
				SELECT r.path, qg.category, qg.grade
				FROM quality_grades qg
				JOIN regions r ON r.id = qg.region_id
				WHERE r.path = $1
				ORDER BY qg.category
			`
			queryArgs = []any{regionFilter}
		}

		rows, err := pool.Query(ctx, query, queryArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Quality Grades:")
		currentRegion := ""
		for rows.Next() {
			var path, category, grade string
			rows.Scan(&path, &category, &grade)
			if path != currentRegion {
				fmt.Printf("\n  %s:\n", path)
				currentRegion = path
			}
			fmt.Printf("    %s: %s\n", category, grade)
		}
		return nil
	},
}

var qualityPrinciplesCmd = &cobra.Command{
	Use:   "principles",
	Short: "List golden principles",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT name, rule, remediation, enabled FROM golden_principles ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Golden Principles:")
		for rows.Next() {
			var name, rule, remediation string
			var enabled bool
			rows.Scan(&name, &rule, &remediation, &enabled)
			status := "enabled"
			if !enabled {
				status = "disabled"
			}
			fmt.Printf("  [%s] %s\n", status, name)
			fmt.Printf("    Rule: %s\n", rule)
			fmt.Printf("    Remediation: %s\n\n", remediation)
		}
		return nil
	},
}

var qualityPrinciplesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a golden principle",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		rule, _ := cmd.Flags().GetString("rule")
		remediation, _ := cmd.Flags().GetString("remediation")

		if name == "" || rule == "" || remediation == "" {
			return fmt.Errorf("--name, --rule, and --remediation are required")
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		_, err = pool.Exec(ctx, `
			INSERT INTO golden_principles (name, rule, remediation, enabled)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (name) DO UPDATE SET rule = $2, remediation = $3
		`, name, rule, remediation)
		if err != nil {
			return fmt.Errorf("add principle: %w", err)
		}

		fmt.Printf("Golden principle '%s' added.\n", name)
		return nil
	},
}

var gardenerCmd = &cobra.Command{
	Use:   "gardener",
	Short: "Entropy management",
}

var gardenerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run entropy sweep and queue fix-up turns",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rdb, err := connectRedis()
		if err != nil {
			return err
		}
		defer rdb.Close()

		m := memorizer.New(pool, rdb, projectRoot())

		findings, err := m.RunGardener(ctx, dryRun)
		if err != nil {
			return fmt.Errorf("gardener: %w", err)
		}

		if len(findings) == 0 {
			fmt.Println("No entropy issues found.")
			return nil
		}

		for _, f := range findings {
			mechStr := ""
			if f.Mechanical {
				mechStr = " [auto-fixable]"
			}
			fmt.Printf("  [%s] %s%s\n    %s\n\n", f.Category, f.RegionPath, mechStr, f.Description)
		}

		fmt.Printf("%d finding(s)", len(findings))
		if dryRun {
			fmt.Print(" (dry run — no turns queued)")
		}
		fmt.Println()
		return nil
	},
}

func init() {
	qualityGradesCmd.Flags().String("region", "", "Filter by region path")

	qualityPrinciplesAddCmd.Flags().String("name", "", "Principle name")
	qualityPrinciplesAddCmd.Flags().String("rule", "", "Principle rule")
	qualityPrinciplesAddCmd.Flags().String("remediation", "", "Agent-actionable remediation")

	gardenerRunCmd.Flags().Bool("dry", false, "Preview findings without creating turns")

	qualityCmd.AddCommand(qualityGradesCmd)
	qualityCmd.AddCommand(qualityPrinciplesCmd)
	qualityPrinciplesCmd.AddCommand(qualityPrinciplesAddCmd)

	gardenerCmd.AddCommand(gardenerRunCmd)
}

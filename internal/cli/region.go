package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/region"
	"github.com/spf13/cobra"
)

var regionCmd = &cobra.Command{
	Use:   "region",
	Short: "Region management",
}

var regionTouchCmd = &cobra.Command{
	Use:   "touch [path]",
	Short: "Create/scaffold region markers in a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		regionPath := args[0]
		file, _ := cmd.Flags().GetString("file")

		if file == "" {
			return fmt.Errorf("--file is required")
		}

		// Scaffold region markers in the file
		if err := region.ScaffoldRegion(file, regionPath); err != nil {
			return fmt.Errorf("scaffold region: %w", err)
		}
		fmt.Printf("Region %s scaffolded in %s\n", regionPath, file)

		// Also register in DB if connected
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			fmt.Println("(database not available — region created in file only)")
			return nil
		}
		defer pool.Close()

		_, err = pool.Exec(ctx, `
			INSERT INTO regions (path, lifecycle_state)
			VALUES ($1, 'draft')
			ON CONFLICT (path) DO NOTHING
		`, regionPath)
		if err != nil {
			fmt.Printf("Warning: could not register region in DB: %v\n", err)
		} else {
			fmt.Printf("Region %s registered in database\n", regionPath)
		}

		return nil
	},
}

var regionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all regions",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT r.path, r.lifecycle_state, r.description,
			       COALESCE(string_agg(c.name, ', '), '') as concepts
			FROM regions r
			LEFT JOIN concept_region_assignments cra ON cra.region_id = r.id
			LEFT JOIN concepts c ON c.id = cra.concept_id
			GROUP BY r.id, r.path, r.lifecycle_state, r.description
			ORDER BY r.path
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Regions:")
		for rows.Next() {
			var path, state string
			var desc, concepts *string
			rows.Scan(&path, &state, &desc, &concepts)
			conceptStr := ""
			if concepts != nil && *concepts != "" {
				conceptStr = fmt.Sprintf("  concepts=[%s]", *concepts)
			}
			descStr := ""
			if desc != nil && *desc != "" {
				descStr = fmt.Sprintf("  %s", *desc)
			}
			fmt.Printf("  %-40s [%s]%s%s\n", path, state, conceptStr, descStr)
		}
		return nil
	},
}

var regionShowCmd = &cobra.Command{
	Use:   "show [path]",
	Short: "Show region details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		regionPath := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Region info
		var state string
		var desc *string
		err = pool.QueryRow(ctx, `
			SELECT lifecycle_state, description FROM regions WHERE path = $1
		`, regionPath).Scan(&state, &desc)
		if err != nil {
			return fmt.Errorf("region %s not found", regionPath)
		}

		fmt.Printf("Region: %s\n", regionPath)
		fmt.Printf("State: %s\n", state)
		if desc != nil {
			fmt.Printf("Description: %s\n", *desc)
		}

		// Concept assignments
		rows, _ := pool.Query(ctx, `
			SELECT c.name, c.purpose, cra.role
			FROM concept_region_assignments cra
			JOIN concepts c ON c.id = cra.concept_id
			JOIN regions r ON r.id = cra.region_id
			WHERE r.path = $1
		`, regionPath)
		if rows != nil {
			fmt.Println("\nConcept Assignments:")
			for rows.Next() {
				var name, purpose, role string
				rows.Scan(&name, &purpose, &role)
				fmt.Printf("  [%s] %s: %s\n", role, name, purpose)
			}
			rows.Close()
		}

		// Recent turns
		turnRows, _ := pool.Query(ctx, `
			SELECT t.id, t.task_type, t.status, t.created_at
			FROM turns t
			JOIN turn_regions tr ON tr.turn_id = t.id
			JOIN regions r ON r.id = tr.region_id
			WHERE r.path = $1
			ORDER BY t.created_at DESC
			LIMIT 5
		`, regionPath)
		if turnRows != nil {
			fmt.Println("\nRecent Turns:")
			for turnRows.Next() {
				var id, taskType, status string
				var createdAt interface{}
				turnRows.Scan(&id, &taskType, &status, &createdAt)
				fmt.Printf("  %s  type=%s  status=%s\n", id, taskType, status)
			}
			turnRows.Close()
		}

		// Quality grades
		gradeRows, _ := pool.Query(ctx, `
			SELECT qg.category, qg.grade
			FROM quality_grades qg
			JOIN regions r ON r.id = qg.region_id
			WHERE r.path = $1
		`, regionPath)
		if gradeRows != nil {
			fmt.Println("\nQuality Grades:")
			for gradeRows.Next() {
				var cat, grade string
				gradeRows.Scan(&cat, &grade)
				fmt.Printf("  %s: %s\n", cat, grade)
			}
			gradeRows.Close()
		}

		return nil
	},
}

func init() {
	regionTouchCmd.Flags().String("file", "", "Target file for region markers")

	regionCmd.AddCommand(regionTouchCmd)
	regionCmd.AddCommand(regionListCmd)
	regionCmd.AddCommand(regionShowCmd)
}

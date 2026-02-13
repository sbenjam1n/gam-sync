package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sbenjam1n/gamsync/internal/gam"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Execution plan management",
}

var planCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new execution plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		goal, _ := cmd.Flags().GetString("goal")
		if goal == "" {
			return fmt.Errorf("--goal is required")
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var planID string
		err = pool.QueryRow(ctx, `
			INSERT INTO execution_plans (name, goal, status)
			VALUES ($1, $2, 'ACTIVE')
			RETURNING id
		`, name, goal).Scan(&planID)
		if err != nil {
			return fmt.Errorf("create plan: %w", err)
		}

		fmt.Printf("Plan '%s' created (id: %s)\n", name, planID)
		fmt.Printf("Goal: %s\n", goal)
		return nil
	},
}

var planShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show plan with progress, decisions, quality grade",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var planID, goal, status string
		var decisionsJSON []byte
		var qualityGrade *string
		var createdAt time.Time
		var completedAt *time.Time
		err = pool.QueryRow(ctx, `
			SELECT id, goal, status, decisions, quality_grade, created_at, completed_at
			FROM execution_plans WHERE name = $1
		`, name).Scan(&planID, &goal, &status, &decisionsJSON, &qualityGrade, &createdAt, &completedAt)
		if err != nil {
			return fmt.Errorf("plan '%s' not found", name)
		}

		fmt.Printf("Plan: %s\n", name)
		fmt.Printf("Goal: %s\n", goal)
		fmt.Printf("Status: %s\n", status)
		if qualityGrade != nil {
			fmt.Printf("Quality Grade: %s\n", *qualityGrade)
		}
		fmt.Printf("Created: %s\n", createdAt.Format(time.RFC3339))
		if completedAt != nil {
			fmt.Printf("Completed: %s\n", completedAt.Format(time.RFC3339))
		}

		// Show turns
		rows, _ := pool.Query(ctx, `
			SELECT turn_id, region_path, ordering, status
			FROM plan_turns WHERE plan_id = $1 ORDER BY ordering
		`, planID)
		if rows != nil {
			fmt.Println("\nProgress:")
			for rows.Next() {
				var turnID, regionPath, turnStatus string
				var ordering int
				rows.Scan(&turnID, &regionPath, &ordering, &turnStatus)
				marker := "[ ]"
				switch turnStatus {
				case "completed":
					marker = "[x]"
				case "active":
					marker = "[>]"
				case "blocked":
					marker = "[!]"
				}
				fmt.Printf("  %s %s — %s (%s)\n", marker, turnID, regionPath, turnStatus)
			}
			rows.Close()
		}

		// Show decisions
		var decisions []gam.Decision
		json.Unmarshal(decisionsJSON, &decisions)
		if len(decisions) > 0 {
			fmt.Println("\nDecisions:")
			for _, d := range decisions {
				fmt.Printf("  - %s\n    Rationale: %s\n", d.Description, d.Rationale)
			}
		}

		return nil
	},
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List execution plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		activeOnly, _ := cmd.Flags().GetBool("active")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		query := `SELECT name, goal, status, quality_grade FROM execution_plans ORDER BY created_at DESC`
		if activeOnly {
			query = `SELECT name, goal, status, quality_grade FROM execution_plans WHERE status = 'ACTIVE' ORDER BY created_at DESC`
		}

		rows, err := pool.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Execution Plans:")
		for rows.Next() {
			var name, goal, status string
			var grade *string
			rows.Scan(&name, &goal, &status, &grade)
			gradeStr := ""
			if grade != nil {
				gradeStr = fmt.Sprintf(" [%s]", *grade)
			}
			fmt.Printf("  %-25s [%s]%s %s\n", name, status, gradeStr, goal)
		}
		return nil
	},
}

var planDecideCmd = &cobra.Command{
	Use:   "decide [name]",
	Short: "Record a design decision in a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		decision, _ := cmd.Flags().GetString("decision")
		rationale, _ := cmd.Flags().GetString("rationale")

		if decision == "" || rationale == "" {
			return fmt.Errorf("--decision and --rationale are required")
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		dec := gam.Decision{
			Description: decision,
			Rationale:   rationale,
			DecidedAt:   time.Now().Format(time.RFC3339),
		}
		decJSON, _ := json.Marshal([]gam.Decision{dec})

		_, err = pool.Exec(ctx, `
			UPDATE execution_plans
			SET decisions = COALESCE(decisions, '[]'::jsonb) || $1::jsonb
			WHERE name = $2 AND status = 'ACTIVE'
		`, decJSON, name)
		if err != nil {
			return fmt.Errorf("record decision: %w", err)
		}

		fmt.Printf("Decision recorded in plan '%s'\n", name)
		return nil
	},
}

var planCloseCmd = &cobra.Command{
	Use:   "close [name]",
	Short: "Mark a plan as completed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		_, err = pool.Exec(ctx, `
			UPDATE execution_plans
			SET status = 'COMPLETED', completed_at = NOW()
			WHERE name = $1 AND status = 'ACTIVE'
		`, name)
		if err != nil {
			return fmt.Errorf("close plan: %w", err)
		}

		fmt.Printf("Plan '%s' marked as completed.\n", name)
		return nil
	},
}

func init() {
	planCreateCmd.Flags().String("goal", "", "Plan goal description")
	planCreateCmd.MarkFlagRequired("goal")

	planListCmd.Flags().Bool("active", false, "Show only active plans")

	planDecideCmd.Flags().String("decision", "", "Decision description")
	planDecideCmd.Flags().String("rationale", "", "Decision rationale")

	planCmd.AddCommand(planCreateCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planDecideCmd)
	planCmd.AddCommand(planCloseCmd)
}

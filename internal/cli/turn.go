package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sbenjam1n/gamsync/internal/memorizer"
	"github.com/spf13/cobra"
)

var turnCmd = &cobra.Command{
	Use:   "turn",
	Short: "Turn lifecycle management",
}

var turnStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new turn: generate ID, create branch, load scratchpad, compile context",
	RunE: func(cmd *cobra.Command, args []string) error {
		regionPath, _ := cmd.Flags().GetString("region")
		if regionPath == "" {
			return fmt.Errorf("--region is required")
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		turnID := memorizer.GenerateTurnID()

		// Insert the turn
		_, err = pool.Exec(ctx, `
			INSERT INTO turns (id, agent_role, scope_path, status, task_type)
			VALUES ($1, 'researcher', $2, 'ACTIVE', 'implement')
		`, turnID, regionPath)
		if err != nil {
			return fmt.Errorf("create turn: %w", err)
		}

		// Load previous scratchpad for this region
		var prevScratchpad *string
		var prevTurnID *string
		pool.QueryRow(ctx, `
			SELECT t.scratchpad, t.id
			FROM turns t
			JOIN turn_regions tr ON tr.turn_id = t.id
			JOIN regions r ON r.id = tr.region_id
			WHERE r.path <@ $1::ltree AND t.scratchpad IS NOT NULL AND t.status = 'COMPLETED'
			ORDER BY t.completed_at DESC NULLS LAST
			LIMIT 1
		`, regionPath).Scan(&prevScratchpad, &prevTurnID)

		fmt.Printf("Turn started: %s\n", turnID)
		fmt.Printf("Region: %s\n", regionPath)

		if prevScratchpad != nil && *prevScratchpad != "" {
			fmt.Printf("\n--- Previous Scratchpad [%s] ---\n%s\n---\n", *prevTurnID, *prevScratchpad)
		}

		// Show concept assignments
		rows, _ := pool.Query(ctx, `
			SELECT c.name, c.purpose, cra.role
			FROM regions r
			JOIN concept_region_assignments cra ON cra.region_id = r.id
			JOIN concepts c ON c.id = cra.concept_id
			WHERE r.path @> $1::ltree OR r.path = $1::ltree
		`, regionPath)
		if rows != nil {
			fmt.Println("\nConcepts in scope:")
			for rows.Next() {
				var name, purpose, role string
				rows.Scan(&name, &purpose, &role)
				fmt.Printf("  [%s] %s: %s\n", role, name, purpose)
			}
			rows.Close()
		}

		return nil
	},
}

var turnEndCmd = &cobra.Command{
	Use:   "end",
	Short: "End a turn: validate, save memory, generate tree, queue proposals",
	RunE: func(cmd *cobra.Command, args []string) error {
		scratchpad, _ := cmd.Flags().GetString("scratchpad")
		if scratchpad == "" {
			return fmt.Errorf("--scratchpad is required")
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Find the most recent active turn
		var turnID string
		err = pool.QueryRow(ctx, `
			SELECT id FROM turns WHERE status = 'ACTIVE' ORDER BY created_at DESC LIMIT 1
		`).Scan(&turnID)
		if err != nil {
			return fmt.Errorf("no active turn found: %w", err)
		}

		now := time.Now()
		_, err = pool.Exec(ctx, `
			UPDATE turns
			SET scratchpad = $1, status = 'COMPLETED', completed_at = $2
			WHERE id = $3
		`, scratchpad, now, turnID)
		if err != nil {
			return fmt.Errorf("end turn: %w", err)
		}

		fmt.Printf("Turn ended: %s\n", turnID)
		fmt.Printf("Scratchpad saved.\n")
		return nil
	},
}

var turnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active turns",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT t.id, t.scope_path, t.task_type, t.agent_role, t.created_at,
			       ep.name as plan_name
			FROM turns t
			LEFT JOIN plan_turns pt ON pt.turn_id = t.id
			LEFT JOIN execution_plans ep ON ep.id = pt.plan_id
			WHERE t.status = 'ACTIVE'
			ORDER BY t.created_at DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Active Turns:")
		found := false
		for rows.Next() {
			found = true
			var id, scope, taskType string
			var agentRole *string
			var createdAt time.Time
			var planName *string
			rows.Scan(&id, &scope, &taskType, &agentRole, &createdAt, &planName)
			role := "unknown"
			if agentRole != nil {
				role = *agentRole
			}
			fmt.Printf("  %s  scope=%s  type=%s  role=%s  started=%s",
				id, scope, taskType, role, createdAt.Format(time.RFC3339))
			if planName != nil {
				fmt.Printf("  plan=%s", *planName)
			}
			fmt.Println()
		}
		if !found {
			fmt.Println("  (none)")
		}
		return nil
	},
}

var turnMemoryCmd = &cobra.Command{
	Use:   "memory [region]",
	Short: "Query scratchpads from turns that touched a region",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		regionPath := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT t.id, t.scratchpad, t.completed_at
			FROM turns t
			JOIN turn_regions tr ON tr.turn_id = t.id
			JOIN regions r ON r.id = tr.region_id
			WHERE r.path <@ $1::ltree AND t.scratchpad IS NOT NULL
			ORDER BY t.completed_at DESC NULLS LAST
			LIMIT 10
		`, regionPath)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Printf("Turn memory for %s:\n\n", regionPath)
		for rows.Next() {
			var id, scratchpad string
			var completedAt *time.Time
			rows.Scan(&id, &scratchpad, &completedAt)
			ts := "(active)"
			if completedAt != nil {
				ts = completedAt.Format(time.RFC3339)
			}
			fmt.Printf("[%s] (%s)\n%s\n\n", id, ts, scratchpad)
		}
		return nil
	},
}

var turnSearchCmd = &cobra.Command{
	Use:   "search [text]",
	Short: "Full-text search across all scratchpads",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		searchText := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT t.id, t.scope_path, t.scratchpad, t.completed_at,
			       similarity(t.scratchpad, $1) AS sim
			FROM turns t
			WHERE t.scratchpad % $1
			ORDER BY sim DESC
			LIMIT 10
		`, searchText)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Printf("Search results for \"%s\":\n\n", searchText)
		for rows.Next() {
			var id, scope, scratchpad string
			var completedAt *time.Time
			var sim float64
			rows.Scan(&id, &scope, &scratchpad, &completedAt, &sim)
			fmt.Printf("[%s] scope=%s (similarity=%.2f)\n%s\n\n", id, scope, sim, scratchpad)
		}
		return nil
	},
}

var turnDiffCmd = &cobra.Command{
	Use:   "diff [turn_id]",
	Short: "Show structural diff for a turn",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		turnID := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT r.path, tr.action
			FROM turn_regions tr
			JOIN regions r ON r.id = tr.region_id
			WHERE tr.turn_id = $1
			ORDER BY tr.action, r.path
		`, turnID)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Printf("Structural diff for %s:\n\n", turnID)
		for rows.Next() {
			var path, action string
			rows.Scan(&path, &action)
			prefix := "~"
			switch action {
			case "created":
				prefix = "+"
			case "deleted":
				prefix = "-"
			}
			fmt.Printf("  %s %s\n", prefix, path)
		}
		return nil
	},
}

func init() {
	turnStartCmd.Flags().String("region", "", "Target region path")
	turnStartCmd.MarkFlagRequired("region")

	turnEndCmd.Flags().String("scratchpad", "", "What you did and what's next")
	turnEndCmd.MarkFlagRequired("scratchpad")

	turnCmd.AddCommand(turnStartCmd)
	turnCmd.AddCommand(turnEndCmd)
	turnCmd.AddCommand(turnStatusCmd)
	turnCmd.AddCommand(turnMemoryCmd)
	turnCmd.AddCommand(turnSearchCmd)
	turnCmd.AddCommand(turnDiffCmd)
}

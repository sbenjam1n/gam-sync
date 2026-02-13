package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sbenjam1n/gamsync/internal/gam"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronization management",
}

var syncAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Register a synchronization from a spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		specFile, _ := cmd.Flags().GetString("spec")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var sync gam.Synchronization
		sync.Name = name

		if specFile != "" {
			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("read spec file: %w", err)
			}
			if err := json.Unmarshal(data, &sync); err != nil {
				return fmt.Errorf("parse spec file: %w", err)
			}
			if sync.Name == "" {
				sync.Name = name
			}
		}

		whenJSON, _ := json.Marshal(sync.WhenClause)
		whereJSON, _ := json.Marshal(sync.WhereClause)
		thenJSON, _ := json.Marshal(sync.ThenClause)

		_, err = pool.Exec(ctx, `
			INSERT INTO synchronizations (name, when_clause, where_clause, then_clause, description, enabled)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (name) DO UPDATE
			SET when_clause = $2, where_clause = $3, then_clause = $4,
			    description = $5, updated_at = NOW()
		`, sync.Name, whenJSON, whereJSON, thenJSON, sync.Description)
		if err != nil {
			return fmt.Errorf("insert sync: %w", err)
		}

		// Build sync_refs index
		var syncID string
		pool.QueryRow(ctx, "SELECT id FROM synchronizations WHERE name = $1", sync.Name).Scan(&syncID)

		if syncID != "" {
			// Clear old refs
			pool.Exec(ctx, "DELETE FROM sync_refs WHERE sync_id = $1", syncID)

			// Index when clause
			for _, w := range sync.WhenClause {
				pool.Exec(ctx, `
					INSERT INTO sync_refs (sync_id, concept_name, action_name, clause_type)
					VALUES ($1, $2, $3, 'when')
					ON CONFLICT DO NOTHING
				`, syncID, w.Concept, w.Action)
			}

			// Index then clause
			for _, t := range sync.ThenClause {
				pool.Exec(ctx, `
					INSERT INTO sync_refs (sync_id, concept_name, action_name, clause_type)
					VALUES ($1, $2, $3, 'then')
					ON CONFLICT DO NOTHING
				`, syncID, t.Concept, t.Action)
			}

			// Index where clause
			for _, w := range sync.WhereClause {
				for _, patternVal := range w.Pattern {
					if fields, ok := patternVal.(map[string]any); ok {
						for fieldName := range fields {
							pool.Exec(ctx, `
								INSERT INTO sync_refs (sync_id, concept_name, state_field, clause_type)
								VALUES ($1, $2, $3, 'where')
								ON CONFLICT DO NOTHING
							`, syncID, w.Concept, fieldName)
						}
					}
				}
			}
		}

		fmt.Printf("Sync '%s' registered.\n", name)
		return nil
	},
}

var syncListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all synchronizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		conceptFilter, _ := cmd.Flags().GetString("concept")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var query string
		var queryArgs []any
		if conceptFilter != "" {
			query = `
				SELECT DISTINCT s.name, s.description, s.enabled
				FROM synchronizations s
				JOIN sync_refs sr ON sr.sync_id = s.id
				WHERE sr.concept_name = $1
				ORDER BY s.name
			`
			queryArgs = []any{conceptFilter}
		} else {
			query = `SELECT name, description, enabled FROM synchronizations ORDER BY name`
		}

		rows, err := pool.Query(ctx, query, queryArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		if conceptFilter != "" {
			fmt.Printf("Syncs referencing concept '%s':\n", conceptFilter)
		} else {
			fmt.Println("Synchronizations:")
		}

		for rows.Next() {
			var name string
			var desc *string
			var enabled bool
			rows.Scan(&name, &desc, &enabled)
			status := "enabled"
			if !enabled {
				status = "disabled"
			}
			descStr := ""
			if desc != nil {
				descStr = *desc
			}
			fmt.Printf("  %-30s [%s] %s\n", name, status, descStr)
		}
		return nil
	},
}

var syncShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Display sync details with referenced concepts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var whenJSON, whereJSON, thenJSON []byte
		var desc *string
		var enabled bool
		err = pool.QueryRow(ctx, `
			SELECT when_clause, where_clause, then_clause, description, enabled
			FROM synchronizations WHERE name = $1
		`, name).Scan(&whenJSON, &whereJSON, &thenJSON, &desc, &enabled)
		if err != nil {
			return fmt.Errorf("sync '%s' not found", name)
		}

		fmt.Printf("sync %s\n", name)
		if desc != nil {
			fmt.Printf("  %s\n", *desc)
		}
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		fmt.Printf("  Status: %s\n", status)

		fmt.Println("\nwhen:")
		prettyWhen, _ := json.MarshalIndent(json.RawMessage(whenJSON), "  ", "  ")
		fmt.Printf("  %s\n", string(prettyWhen))

		if whereJSON != nil && string(whereJSON) != "null" {
			fmt.Println("\nwhere:")
			prettyWhere, _ := json.MarshalIndent(json.RawMessage(whereJSON), "  ", "  ")
			fmt.Printf("  %s\n", string(prettyWhere))
		}

		fmt.Println("\nthen:")
		prettyThen, _ := json.MarshalIndent(json.RawMessage(thenJSON), "  ", "  ")
		fmt.Printf("  %s\n", string(prettyThen))

		// Show referenced concepts
		rows, _ := pool.Query(ctx, `
			SELECT concept_name, action_name, state_field, clause_type
			FROM sync_refs
			WHERE sync_id = (SELECT id FROM synchronizations WHERE name = $1)
			ORDER BY clause_type, concept_name
		`, name)
		if rows != nil {
			fmt.Println("\nReferences:")
			for rows.Next() {
				var concept, clause string
				var action, field *string
				rows.Scan(&concept, &action, &field, &clause)
				ref := concept
				if action != nil && *action != "" {
					ref += "/" + *action
				}
				if field != nil && *field != "" {
					ref += "." + *field
				}
				fmt.Printf("  [%s] %s\n", clause, ref)
			}
			rows.Close()
		}

		return nil
	},
}

var syncCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify all sync references are valid",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Find sync refs that point to nonexistent concepts or actions
		rows, err := pool.Query(ctx, `
			SELECT sr.concept_name, sr.action_name, sr.clause_type, s.name as sync_name
			FROM sync_refs sr
			JOIN synchronizations s ON s.id = sr.sync_id
			WHERE sr.action_name IS NOT NULL
			  AND NOT EXISTS (
				  SELECT 1 FROM concepts c
				  WHERE c.name = sr.concept_name
				  AND c.spec->'actions' ? sr.action_name
			  )
			ORDER BY s.name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		issues := 0
		for rows.Next() {
			var concept, clause, syncName string
			var action *string
			rows.Scan(&concept, &action, &clause, &syncName)
			actionStr := ""
			if action != nil {
				actionStr = *action
			}
			fmt.Printf("BROKEN: sync %s [%s] references %s/%s — action not found\n",
				syncName, clause, concept, actionStr)
			fmt.Printf("  Fix: Define action '%s' in concept '%s' or update sync reference\n", actionStr, concept)
			issues++
		}

		if issues == 0 {
			fmt.Println("All sync references valid.")
		} else {
			fmt.Printf("\n%d broken reference(s) found.\n", issues)
		}
		return nil
	},
}

func init() {
	syncAddCmd.Flags().String("spec", "", "Path to sync spec JSON file")
	syncListCmd.Flags().String("concept", "", "Filter syncs by concept name")

	syncCmd.AddCommand(syncAddCmd)
	syncCmd.AddCommand(syncListCmd)
	syncCmd.AddCommand(syncShowCmd)
	syncCmd.AddCommand(syncCheckCmd)
}

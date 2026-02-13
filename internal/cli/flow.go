package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Flow provenance tracking",
}

var flowTraceCmd = &cobra.Command{
	Use:   "trace [token]",
	Short: "Show causal graph for a flow token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			WITH RECURSIVE trace AS (
				SELECT id, flow_token, concept_name, action_name, input_args, output_args,
				       sync_name, parent_id, created_at, 0 as depth
				FROM flow_log
				WHERE flow_token = $1 AND parent_id IS NULL
				UNION ALL
				SELECT fl.id, fl.flow_token, fl.concept_name, fl.action_name, fl.input_args, fl.output_args,
				       fl.sync_name, fl.parent_id, fl.created_at, t.depth + 1
				FROM flow_log fl
				JOIN trace t ON fl.parent_id = t.id
			)
			SELECT depth, concept_name, action_name, sync_name, created_at
			FROM trace
			ORDER BY depth, created_at
		`, token)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Printf("Flow trace for %s:\n\n", token)
		for rows.Next() {
			var depth int
			var concept, action string
			var syncName *string
			var createdAt time.Time
			rows.Scan(&depth, &concept, &action, &syncName, &createdAt)

			indent := ""
			for i := 0; i < depth; i++ {
				indent += "  "
			}

			syncStr := ""
			if syncName != nil && *syncName != "" {
				syncStr = fmt.Sprintf(" (via sync: %s)", *syncName)
			}

			fmt.Printf("%s%s/%s%s  [%s]\n", indent, concept, action, syncStr, createdAt.Format(time.RFC3339))
		}
		return nil
	},
}

var flowListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show recent flow tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		recent, _ := cmd.Flags().GetInt("recent")
		if recent <= 0 {
			recent = 10
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT DISTINCT ON (flow_token) flow_token, concept_name, action_name, created_at
			FROM flow_log
			WHERE parent_id IS NULL
			ORDER BY flow_token, created_at DESC
			LIMIT $1
		`, recent)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Recent flow tokens:")
		for rows.Next() {
			var token, concept, action string
			var createdAt time.Time
			rows.Scan(&token, &concept, &action, &createdAt)
			fmt.Printf("  %s  %s/%s  [%s]\n", token, concept, action, createdAt.Format(time.RFC3339))
		}
		return nil
	},
}

func init() {
	flowListCmd.Flags().Int("recent", 10, "Number of recent flow tokens to show")

	flowCmd.AddCommand(flowTraceCmd)
	flowCmd.AddCommand(flowListCmd)
}

package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/queue"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Queue management",
}

var queueStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show pending tasks and proposals in Redis",
	RunE: func(cmd *cobra.Command, args []string) error {
		rdb, err := connectRedis()
		if err != nil {
			return err
		}
		defer rdb.Close()

		ctx := context.Background()
		q := queue.New(rdb)

		tasks, proposals, err := q.Status(ctx)
		if err != nil {
			return fmt.Errorf("queue status: %w", err)
		}

		fmt.Printf("Queue Status:\n")
		fmt.Printf("  agent_tasks:     %d pending\n", tasks)
		fmt.Printf("  agent_proposals: %d pending\n", proposals)
		return nil
	},
}

var queueEscalatedCmd = &cobra.Command{
	Use:   "escalated",
	Short: "Show proposals awaiting human review",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT p.id, r.path, p.rejection_reason, p.created_at
			FROM proposals p
			JOIN regions r ON r.id = p.region_id
			WHERE p.status = 'PENDING' AND p.rejection_reason LIKE 'ESCALATED%'
			ORDER BY p.created_at DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Escalated Proposals (awaiting human review):")
		found := false
		for rows.Next() {
			found = true
			var id, path, reason string
			var createdAt interface{}
			rows.Scan(&id, &path, &reason, &createdAt)
			fmt.Printf("  %s  region=%s\n    %s\n\n", id, path, reason)
		}
		if !found {
			fmt.Println("  (none)")
		}
		return nil
	},
}

func init() {
	queueCmd.AddCommand(queueStatusCmd)
	queueCmd.AddCommand(queueEscalatedCmd)
}

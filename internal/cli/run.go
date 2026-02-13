package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/memorizer"
	"github.com/spf13/cobra"
)

var memorizerCmd = &cobra.Command{
	Use:   "memorizer",
	Short: "Memorizer agent operations",
}

var memorizerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Memorizer: process proposals, create turns, manage plans",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("Memorizer running. Consuming proposals from Redis...")
		return m.ConsumeProposals(ctx)
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Memorizer-Researcher loop",
	RunE: func(cmd *cobra.Command, args []string) error {
		auto, _ := cmd.Flags().GetBool("auto")
		withGardener, _ := cmd.Flags().GetBool("gardener")

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

		if withGardener {
			fmt.Println("Running gardener sweep...")
			findings, err := m.RunGardener(ctx, false)
			if err != nil {
				fmt.Printf("Gardener error: %v\n", err)
			} else {
				fmt.Printf("Gardener found %d issue(s)\n", len(findings))
			}
		}

		if auto {
			fmt.Println("Running automated Memorizer loop...")
			fmt.Println("(Press Ctrl+C to stop)")
			return m.ConsumeProposals(ctx)
		}

		fmt.Println("Sequential mode: run 'gam memorizer run' and 'gam researcher run' separately.")
		fmt.Println("Or use 'gam run --auto' for automated loop.")
		return nil
	},
}

func init() {
	runCmd.Flags().Bool("auto", false, "Automated loop until queues empty")
	runCmd.Flags().Bool("gardener", false, "Include gardener sweeps")

	memorizerCmd.AddCommand(memorizerRunCmd)
}

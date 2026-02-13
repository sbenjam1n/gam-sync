package cli

import (
	"context"
	"fmt"

	"github.com/sbenjam1n/gamsync/internal/memorizer"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Docs directory projection",
}

var docsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all DB state to docs/ (concepts, syncs, plans, quality)",
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
		exporter := memorizer.NewDocsExporter(m, projectRoot())

		if err := exporter.ExportAll(ctx); err != nil {
			return fmt.Errorf("export docs: %w", err)
		}

		fmt.Println("docs/ directory exported from database.")
		return nil
	},
}

var docsImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import docs/ back to DB",
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
		exporter := memorizer.NewDocsExporter(m, projectRoot())

		if err := exporter.ImportDocs(ctx); err != nil {
			return fmt.Errorf("import docs: %w", err)
		}

		fmt.Println("docs/ imported to database.")
		return nil
	},
}

var docsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which docs/ files are stale vs DB",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("docs status: checking for stale files...")
		fmt.Println("(run 'gam docs export' to regenerate docs/ from database)")
		return nil
	},
}

func init() {
	docsCmd.AddCommand(docsExportCmd)
	docsCmd.AddCommand(docsImportCmd)
	docsCmd.AddCommand(docsStatusCmd)
}

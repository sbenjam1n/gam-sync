package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sbenjam1n/gamsync/internal/region"
	"github.com/spf13/cobra"
)

var archCmd = &cobra.Command{
	Use:   "arch",
	Short: "Architecture file management",
}

var archSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Bidirectional sync between arch.md and PostgreSQL",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Read arch.md regions
		archPaths, err := region.ParseArchMd(projectRoot())
		if err != nil {
			return fmt.Errorf("parse arch.md: %w", err)
		}

		// Read DB regions
		rows, err := pool.Query(ctx, `SELECT path FROM regions ORDER BY path`)
		if err != nil {
			return err
		}
		defer rows.Close()

		dbPaths := make(map[string]bool)
		for rows.Next() {
			var path string
			rows.Scan(&path)
			dbPaths[path] = true
		}

		archPathMap := make(map[string]bool)
		for _, p := range archPaths {
			archPathMap[p] = true
		}

		// Add arch.md paths to DB
		added := 0
		for _, p := range archPaths {
			if !dbPaths[p] {
				_, err := pool.Exec(ctx, `
					INSERT INTO regions (path, lifecycle_state) VALUES ($1, 'draft')
					ON CONFLICT (path) DO NOTHING
				`, p)
				if err == nil {
					added++
					fmt.Printf("  DB <- arch.md: added %s\n", p)
				}
			}
		}

		// Add DB paths to arch.md
		var newArchPaths []string
		for path := range dbPaths {
			if !archPathMap[path] {
				newArchPaths = append(newArchPaths, path)
				fmt.Printf("  arch.md <- DB: adding %s\n", path)
			}
		}

		if len(newArchPaths) > 0 {
			archFile := filepath.Join(projectRoot(), "arch.md")
			data, err := os.ReadFile(archFile)
			if err != nil {
				return err
			}
			content := string(data)
			for _, p := range newArchPaths {
				marker := fmt.Sprintf("# @region:%s\n# @endregion:%s\n", p, p)
				content += marker
			}
			os.WriteFile(archFile, []byte(content), 0644)
		}

		fmt.Printf("\nSync complete: %d added to DB, %d added to arch.md\n", added, len(newArchPaths))
		return nil
	},
}

var archExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export DB namespace tree to arch.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT r.path, r.description FROM regions r ORDER BY r.path
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var content strings.Builder
		content.WriteString("# Architecture\n")
		content.WriteString("# Region markers define the namespace tree for this project.\n\n")

		for rows.Next() {
			var path string
			var desc *string
			rows.Scan(&path, &desc)
			descStr := ""
			if desc != nil && *desc != "" {
				descStr = " " + *desc
			}
			content.WriteString(fmt.Sprintf("# @region:%s%s\n# @endregion:%s\n", path, descStr, path))
		}

		archFile := filepath.Join(projectRoot(), "arch.md")
		if err := os.WriteFile(archFile, []byte(content.String()), 0644); err != nil {
			return err
		}

		fmt.Println("arch.md exported from database.")
		return nil
	},
}

var archImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import arch.md namespace tree to DB",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		archPaths, err := region.ParseArchMd(projectRoot())
		if err != nil {
			return fmt.Errorf("parse arch.md: %w", err)
		}

		imported := 0
		for _, p := range archPaths {
			_, err := pool.Exec(ctx, `
				INSERT INTO regions (path, lifecycle_state) VALUES ($1, 'draft')
				ON CONFLICT (path) DO NOTHING
			`, p)
			if err == nil {
				imported++
			}
		}

		fmt.Printf("Imported %d regions from arch.md.\n", imported)
		return nil
	},
}

func init() {
	archCmd.AddCommand(archSyncCmd)
	archCmd.AddCommand(archExportCmd)
	archCmd.AddCommand(archImportCmd)
}

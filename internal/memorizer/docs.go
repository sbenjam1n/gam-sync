package memorizer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sbenjam1n/gamsync/internal/gam"
)

// DocsExporter generates the docs/ directory from PostgreSQL state.
type DocsExporter struct {
	m           *Memorizer
	projectRoot string
}

// NewDocsExporter creates a new docs exporter.
func NewDocsExporter(m *Memorizer, projectRoot string) *DocsExporter {
	return &DocsExporter{m: m, projectRoot: projectRoot}
}

// ExportAll regenerates the entire docs/ directory.
func (d *DocsExporter) ExportAll(ctx context.Context) error {
	docsDir := filepath.Join(d.projectRoot, "docs")
	for _, sub := range []string{
		"concepts",
		"syncs",
		"exec-plans/active",
		"exec-plans/completed",
		"quality",
		"design",
	} {
		os.MkdirAll(filepath.Join(docsDir, sub), 0755)
	}

	if err := d.ExportConcepts(ctx); err != nil {
		return err
	}
	if err := d.ExportSyncs(ctx); err != nil {
		return err
	}
	if err := d.ExportPlans(ctx); err != nil {
		return err
	}
	if err := d.ExportQuality(ctx); err != nil {
		return err
	}
	return nil
}

// ExportConcepts writes concept specs to docs/concepts/.
func (d *DocsExporter) ExportConcepts(ctx context.Context) error {
	rows, err := d.m.db.Query(ctx, `
		SELECT name, purpose, spec, invariants FROM concepts ORDER BY name
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var index strings.Builder
	index.WriteString("# Concept Catalog\n\n")

	for rows.Next() {
		var name, purpose string
		var specJSON, invJSON []byte
		rows.Scan(&name, &purpose, &specJSON, &invJSON)

		var spec gam.ConceptSpec
		json.Unmarshal(specJSON, &spec)

		var invariants []gam.Invariant
		json.Unmarshal(invJSON, &invariants)

		index.WriteString(fmt.Sprintf("- **%s**: %s\n", name, purpose))

		// Write individual concept file
		var content strings.Builder
		content.WriteString(fmt.Sprintf("# %s\n\n", name))
		content.WriteString(fmt.Sprintf("**Purpose**: %s\n\n", purpose))

		if len(spec.TypeParams) > 0 {
			content.WriteString(fmt.Sprintf("**Type Parameters**: %s\n\n", strings.Join(spec.TypeParams, ", ")))
		}

		if len(spec.State) > 0 {
			content.WriteString("## State\n\n")
			for field, sc := range spec.State {
				if sc.Type == "set" {
					content.WriteString(fmt.Sprintf("- `%s`: set %s\n", field, sc.Of))
				} else if sc.Type == "map" {
					content.WriteString(fmt.Sprintf("- `%s`: %s -> %s\n", field, sc.From, sc.To))
				}
			}
			content.WriteString("\n")
		}

		if len(spec.Actions) > 0 {
			content.WriteString("## Actions\n\n")
			for actionName, action := range spec.Actions {
				for _, c := range action.Cases {
					inputParts := make([]string, 0)
					for k, v := range c.Input {
						inputParts = append(inputParts, fmt.Sprintf("%s: %s", k, v))
					}
					outputParts := make([]string, 0)
					for k, v := range c.Output {
						outputParts = append(outputParts, fmt.Sprintf("%s: %s", k, v))
					}
					content.WriteString(fmt.Sprintf("- `%s [%s] => [%s]`\n",
						actionName,
						strings.Join(inputParts, "; "),
						strings.Join(outputParts, "; "),
					))
					if c.Description != "" {
						content.WriteString(fmt.Sprintf("  %s\n", c.Description))
					}
				}
			}
			content.WriteString("\n")
		}

		if len(invariants) > 0 {
			content.WriteString("## Invariants\n\n")
			for _, inv := range invariants {
				content.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", inv.Name, inv.Type, inv.Rule))
			}
			content.WriteString("\n")
		}

		if spec.OperationalPrinciple != "" {
			content.WriteString("## Operational Principle\n\n")
			content.WriteString(fmt.Sprintf("```\n%s\n```\n", spec.OperationalPrinciple))
		}

		slug := strings.ReplaceAll(strings.ToLower(name), " ", "-")
		filename := filepath.Join(d.projectRoot, "docs", "concepts", slug+".md")
		os.WriteFile(filename, []byte(content.String()), 0644)
	}

	indexFile := filepath.Join(d.projectRoot, "docs", "concepts", "index.md")
	return os.WriteFile(indexFile, []byte(index.String()), 0644)
}

// ExportSyncs writes synchronization definitions to docs/syncs/.
func (d *DocsExporter) ExportSyncs(ctx context.Context) error {
	rows, err := d.m.db.Query(ctx, `
		SELECT name, description, when_clause, where_clause, then_clause, enabled
		FROM synchronizations ORDER BY name
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var index strings.Builder
	index.WriteString("# Synchronization Catalog\n\n")

	for rows.Next() {
		var name string
		var description *string
		var whenJSON, whereJSON, thenJSON []byte
		var enabled bool
		rows.Scan(&name, &description, &whenJSON, &whereJSON, &thenJSON, &enabled)

		desc := ""
		if description != nil {
			desc = *description
		}

		status := "enabled"
		if !enabled {
			status = "disabled"
		}

		index.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", name, status, desc))

		var content strings.Builder
		content.WriteString(fmt.Sprintf("# sync %s\n\n", name))
		if desc != "" {
			content.WriteString(fmt.Sprintf("%s\n\n", desc))
		}
		content.WriteString(fmt.Sprintf("Status: %s\n\n", status))

		content.WriteString("## When\n```json\n")
		prettyWhen, _ := json.MarshalIndent(json.RawMessage(whenJSON), "", "  ")
		content.WriteString(string(prettyWhen))
		content.WriteString("\n```\n\n")

		if whereJSON != nil {
			content.WriteString("## Where\n```json\n")
			prettyWhere, _ := json.MarshalIndent(json.RawMessage(whereJSON), "", "  ")
			content.WriteString(string(prettyWhere))
			content.WriteString("\n```\n\n")
		}

		content.WriteString("## Then\n```json\n")
		prettyThen, _ := json.MarshalIndent(json.RawMessage(thenJSON), "", "  ")
		content.WriteString(string(prettyThen))
		content.WriteString("\n```\n")

		slug := strings.ReplaceAll(strings.ToLower(name), " ", "-")
		filename := filepath.Join(d.projectRoot, "docs", "syncs", slug+".md")
		os.WriteFile(filename, []byte(content.String()), 0644)
	}

	indexFile := filepath.Join(d.projectRoot, "docs", "syncs", "index.md")
	return os.WriteFile(indexFile, []byte(index.String()), 0644)
}

// ExportPlans writes execution plans to docs/exec-plans/.
func (d *DocsExporter) ExportPlans(ctx context.Context) error {
	rows, err := d.m.db.Query(ctx, `
		SELECT id, name, goal, status, decisions, quality_grade FROM execution_plans ORDER BY created_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var planID, name, goal, status string
		var decisionsJSON []byte
		var qualityGrade *string
		rows.Scan(&planID, &name, &goal, &status, &decisionsJSON, &qualityGrade)

		var content strings.Builder
		content.WriteString(fmt.Sprintf("# %s\n\n", name))
		content.WriteString(fmt.Sprintf("**Goal**: %s\n\n", goal))
		content.WriteString(fmt.Sprintf("**Status**: %s\n\n", status))
		if qualityGrade != nil {
			content.WriteString(fmt.Sprintf("**Quality Grade**: %s\n\n", *qualityGrade))
		}

		// Get plan turns
		turnRows, _ := d.m.db.Query(ctx, `
			SELECT turn_id, region_path, ordering, status
			FROM plan_turns WHERE plan_id = $1 ORDER BY ordering
		`, planID)
		if turnRows != nil {
			content.WriteString("## Progress\n\n")
			for turnRows.Next() {
				var turnID, regionPath, turnStatus string
				var ordering int
				turnRows.Scan(&turnID, &regionPath, &ordering, &turnStatus)
				marker := "[ ]"
				if turnStatus == "completed" {
					marker = "[x]"
				} else if turnStatus == "active" {
					marker = "[>]"
				}
				content.WriteString(fmt.Sprintf("%s %s — %s (%s)\n", marker, turnID, regionPath, turnStatus))
			}
			turnRows.Close()
			content.WriteString("\n")
		}

		// Decisions
		var decisions []gam.Decision
		json.Unmarshal(decisionsJSON, &decisions)
		if len(decisions) > 0 {
			content.WriteString("## Decisions\n\n")
			for _, dec := range decisions {
				content.WriteString(fmt.Sprintf("- **%s**: %s\n", dec.Description, dec.Rationale))
			}
			content.WriteString("\n")
		}

		subdir := "active"
		if status == "COMPLETED" {
			subdir = "completed"
		}
		slug := strings.ReplaceAll(strings.ToLower(name), " ", "-")
		filename := filepath.Join(d.projectRoot, "docs", "exec-plans", subdir, slug+".md")
		os.WriteFile(filename, []byte(content.String()), 0644)
	}
	return nil
}

// ExportQuality writes quality grades and golden principles to docs/quality/.
func (d *DocsExporter) ExportQuality(ctx context.Context) error {
	// Export quality grades
	var grades strings.Builder
	grades.WriteString("# Quality Grades\n\n")

	rows, _ := d.m.db.Query(ctx, `
		SELECT r.path, qg.category, qg.grade
		FROM quality_grades qg
		JOIN regions r ON r.id = qg.region_id
		ORDER BY r.path, qg.category
	`)
	if rows != nil {
		currentRegion := ""
		for rows.Next() {
			var path, category, grade string
			rows.Scan(&path, &category, &grade)
			if path != currentRegion {
				grades.WriteString(fmt.Sprintf("\n## %s\n\n", path))
				currentRegion = path
			}
			grades.WriteString(fmt.Sprintf("- %s: **%s**\n", category, grade))
		}
		rows.Close()
	}

	gradesFile := filepath.Join(d.projectRoot, "docs", "quality", "grades.md")
	os.WriteFile(gradesFile, []byte(grades.String()), 0644)

	// Export golden principles
	var principles strings.Builder
	principles.WriteString("# Golden Principles\n\n")

	pRows, _ := d.m.db.Query(ctx, `
		SELECT name, rule, remediation, enabled FROM golden_principles ORDER BY name
	`)
	if pRows != nil {
		for pRows.Next() {
			var name, rule, remediation string
			var enabled bool
			pRows.Scan(&name, &rule, &remediation, &enabled)
			status := "enabled"
			if !enabled {
				status = "disabled"
			}
			principles.WriteString(fmt.Sprintf("## %s (%s)\n\n", name, status))
			principles.WriteString(fmt.Sprintf("**Rule**: %s\n\n", rule))
			principles.WriteString(fmt.Sprintf("**Remediation**: %s\n\n", remediation))
		}
		pRows.Close()
	}

	principlesFile := filepath.Join(d.projectRoot, "docs", "quality", "golden-principles.md")
	return os.WriteFile(principlesFile, []byte(principles.String()), 0644)
}

// ImportDocs reads docs/ directory and imports content back to the database.
func (d *DocsExporter) ImportDocs(ctx context.Context) error {
	// This is a bootstrap/reconciliation feature.
	// For MVP, import reads concept and sync markdown files and parses them back.
	// Full implementation would parse the markdown structure.
	return fmt.Errorf("docs import not yet implemented — use gam concept add and gam sync add for individual imports")
}

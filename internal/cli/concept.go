package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sbenjam1n/gamsync/internal/gam"
	"github.com/spf13/cobra"
)

var conceptCmd = &cobra.Command{
	Use:   "concept",
	Short: "Concept management",
}

var conceptAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Register a concept from a spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		specFile, _ := cmd.Flags().GetString("spec")
		purpose, _ := cmd.Flags().GetString("purpose")

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var concept gam.Concept
		concept.Name = name

		if specFile != "" {
			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("read spec file: %w", err)
			}

			// Try parsing as full concept JSON
			if err := json.Unmarshal(data, &concept); err != nil {
				// Try parsing as just ConceptSpec
				if err := json.Unmarshal(data, &concept.Spec); err != nil {
					return fmt.Errorf("parse spec file: %w (expected JSON with concept spec fields)", err)
				}
			}
		}

		if purpose != "" {
			concept.Purpose = purpose
		}

		if concept.Purpose == "" {
			return fmt.Errorf("--purpose is required when not provided in spec file")
		}

		specJSON, _ := json.Marshal(concept.Spec)
		smJSON, _ := json.Marshal(concept.StateMachine)
		invJSON, _ := json.Marshal(concept.Invariants)

		_, err = pool.Exec(ctx, `
			INSERT INTO concepts (name, purpose, spec, state_machine, invariants)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (name) DO UPDATE
			SET purpose = $2, spec = $3, state_machine = $4, invariants = $5, updated_at = NOW()
		`, concept.Name, concept.Purpose, specJSON, smJSON, invJSON)
		if err != nil {
			return fmt.Errorf("insert concept: %w", err)
		}

		fmt.Printf("Concept '%s' registered.\n", name)
		return nil
	},
}

var conceptShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Display concept spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		var purpose string
		var specJSON, smJSON, invJSON []byte
		err = pool.QueryRow(ctx, `
			SELECT purpose, spec, state_machine, invariants FROM concepts WHERE name = $1
		`, name).Scan(&purpose, &specJSON, &smJSON, &invJSON)
		if err != nil {
			return fmt.Errorf("concept '%s' not found", name)
		}

		var spec gam.ConceptSpec
		json.Unmarshal(specJSON, &spec)

		var sm gam.StateMachine
		json.Unmarshal(smJSON, &sm)

		var invariants []gam.Invariant
		json.Unmarshal(invJSON, &invariants)

		fmt.Printf("concept %s", name)
		if len(spec.TypeParams) > 0 {
			fmt.Printf(" [%s]", strings.Join(spec.TypeParams, ", "))
		}
		fmt.Println()
		fmt.Printf("purpose\n  %s\n", purpose)

		if len(spec.State) > 0 {
			fmt.Println("state")
			for field, sc := range spec.State {
				if sc.Type == "set" {
					fmt.Printf("  %s: set %s\n", field, sc.Of)
				} else if sc.Type == "map" {
					fmt.Printf("  %s: %s -> %s\n", field, sc.From, sc.To)
				}
			}
		}

		if len(spec.Actions) > 0 {
			fmt.Println("actions")
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
					fmt.Printf("  %s [%s]\n", actionName, strings.Join(inputParts, "; "))
					fmt.Printf("    => [%s]\n", strings.Join(outputParts, "; "))
					if c.Description != "" {
						fmt.Printf("    %s\n", c.Description)
					}
				}
			}
		}

		if len(invariants) > 0 {
			fmt.Println("invariants")
			for _, inv := range invariants {
				fmt.Printf("  - %s (%s): %s\n", inv.Name, inv.Type, inv.Rule)
			}
		}

		if len(sm.States) > 0 {
			fmt.Println("state_machine")
			fmt.Printf("  states: %s\n", strings.Join(sm.States, ", "))
			for _, t := range sm.Transitions {
				fmt.Printf("  %s -> %s via %s\n", t.From, t.To, t.Action)
			}
		}

		if spec.OperationalPrinciple != "" {
			fmt.Println("operational principle")
			fmt.Printf("  %s\n", spec.OperationalPrinciple)
		}

		// Show region assignments
		rows, _ := pool.Query(ctx, `
			SELECT r.path, cra.role
			FROM concept_region_assignments cra
			JOIN regions r ON r.id = cra.region_id
			JOIN concepts c ON c.id = cra.concept_id
			WHERE c.name = $1
			ORDER BY r.path
		`, name)
		if rows != nil {
			fmt.Println("regions")
			for rows.Next() {
				var path, role string
				rows.Scan(&path, &role)
				fmt.Printf("  %s [%s]\n", path, role)
			}
			rows.Close()
		}

		return nil
	},
}

var conceptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all concepts with purposes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		rows, err := pool.Query(ctx, `
			SELECT name, purpose FROM concepts ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Println("Concepts:")
		for rows.Next() {
			var name, purpose string
			rows.Scan(&name, &purpose)
			fmt.Printf("  %-30s %s\n", name, purpose)
		}
		return nil
	},
}

var conceptAssignCmd = &cobra.Command{
	Use:   "assign [concept] [region]",
	Short: "Create concept-region assignment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conceptName := args[0]
		regionPath := args[1]
		role, _ := cmd.Flags().GetString("role")
		if role == "" {
			role = "implementation"
		}

		ctx := context.Background()
		pool, err := connectDB(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		_, err = pool.Exec(ctx, `
			INSERT INTO concept_region_assignments (concept_id, region_id, role)
			SELECT c.id, r.id, $3
			FROM concepts c, regions r
			WHERE c.name = $1 AND r.path = $2
			ON CONFLICT (concept_id, region_id) DO UPDATE SET role = $3
		`, conceptName, regionPath, role)
		if err != nil {
			return fmt.Errorf("assign concept: %w", err)
		}

		fmt.Printf("Concept '%s' assigned to region '%s' with role '%s'\n", conceptName, regionPath, role)
		return nil
	},
}

func init() {
	conceptAddCmd.Flags().String("spec", "", "Path to concept spec JSON file")
	conceptAddCmd.Flags().String("purpose", "", "Concept purpose (overrides spec file)")

	conceptAssignCmd.Flags().String("role", "implementation", "Assignment role: implementation|integration|test|consumer")

	conceptCmd.AddCommand(conceptAddCmd)
	conceptCmd.AddCommand(conceptShowCmd)
	conceptCmd.AddCommand(conceptListCmd)
	conceptCmd.AddCommand(conceptAssignCmd)
}

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getzep/zep-go/v3"
	"github.com/getzep/zepctl/internal/client"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var ontologyCmd = &cobra.Command{
	Use:   "ontology",
	Short: "Manage graph ontology",
	Long:  `Get and set custom entity and edge type definitions for graphs.`,
}

var ontologyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get ontology definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}

		result, err := c.Graph.ListEntityTypes(context.Background(), &zep.GraphListEntityTypesRequest{})
		if err != nil {
			return fmt.Errorf("getting ontology: %w", err)
		}

		return output.Print(result)
	},
}

var ontologySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set ontology definitions",
	Long:  `Set custom entity and edge types for your graph schema.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")

		if file == "" {
			return fmt.Errorf("--file is required")
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		// Parse ontology file (supports both YAML and JSON)
		var ontologyDef OntologyDefinition
		if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
			if err := yaml.Unmarshal(data, &ontologyDef); err != nil {
				return fmt.Errorf("parsing YAML: %w", err)
			}
		} else {
			if err := json.Unmarshal(data, &ontologyDef); err != nil {
				return fmt.Errorf("parsing JSON: %w", err)
			}
		}

		c, err := client.New()
		if err != nil {
			return err
		}

		// Build entity types
		var entityTypes []*zep.EntityType
		for name, entity := range ontologyDef.Entities {
			entityDef := &zep.EntityType{
				Name:        name,
				Description: entity.Description,
			}
			if len(entity.Fields) > 0 {
				var properties []*zep.EntityProperty
				for fieldName, fieldDef := range entity.Fields {
					properties = append(properties, &zep.EntityProperty{
						Name:        fieldName,
						Description: fieldDef.Description,
						Type:        zep.EntityPropertyTypeText, // Default to text type
					})
				}
				entityDef.Properties = properties
			}
			entityTypes = append(entityTypes, entityDef)
		}

		// Build edge types
		var edgeTypes []*zep.EdgeType
		for name, edge := range ontologyDef.Edges {
			edgeDef := &zep.EdgeType{
				Name:        name,
				Description: edge.Description,
			}
			// Build source/target constraints
			if len(edge.SourceTypes) > 0 && len(edge.TargetTypes) > 0 {
				var sourceTargets []*zep.EntityEdgeSourceTarget
				for _, source := range edge.SourceTypes {
					for _, target := range edge.TargetTypes {
						sourceTargets = append(sourceTargets, &zep.EntityEdgeSourceTarget{
							Source: zep.String(source),
							Target: zep.String(target),
						})
					}
				}
				edgeDef.SourceTargets = sourceTargets
			}
			edgeTypes = append(edgeTypes, edgeDef)
		}

		req := &zep.EntityTypeRequest{
			EntityTypes: entityTypes,
			EdgeTypes:   edgeTypes,
		}

		result, err := c.Graph.SetEntityTypesInternal(context.Background(), req)
		if err != nil {
			return fmt.Errorf("setting ontology: %w", err)
		}

		if output.GetFormat() == output.FormatTable {
			output.Info("Ontology set successfully")
			return nil
		}

		return output.Print(result)
	},
}

// OntologyDefinition represents the YAML/JSON file format for ontology.
type OntologyDefinition struct {
	Entities map[string]EntityDefinition `json:"entities" yaml:"entities"`
	Edges    map[string]EdgeDefinition   `json:"edges" yaml:"edges"`
}

// EntityDefinition represents an entity type in the ontology file.
type EntityDefinition struct {
	Description string                     `json:"description" yaml:"description"`
	Fields      map[string]FieldDefinition `json:"fields" yaml:"fields"`
}

// FieldDefinition represents a field on an entity type.
type FieldDefinition struct {
	Description string `json:"description" yaml:"description"`
}

// EdgeDefinition represents an edge type in the ontology file.
type EdgeDefinition struct {
	Description string   `json:"description" yaml:"description"`
	SourceTypes []string `json:"source_types" yaml:"source_types"`
	TargetTypes []string `json:"target_types" yaml:"target_types"`
}

func init() {
	rootCmd.AddCommand(ontologyCmd)
	ontologyCmd.AddCommand(ontologyGetCmd)
	ontologyCmd.AddCommand(ontologySetCmd)

	// Set flags
	ontologySetCmd.Flags().String("file", "", "Path to ontology definition file (YAML/JSON)")
}

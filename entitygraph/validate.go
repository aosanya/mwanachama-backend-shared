package entitygraph

import (
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// FindTypeDef returns the [schema.TypeDefinition] for the given typeName
// within s, or an error if no matching type exists.
//
// Intended for use by DataManager implementations that need to look up the
// definition of an entity's type before executing a write operation (e.g.
// to retrieve its declared Relationships for validation).
func FindTypeDef(s schema.Schema, typeName string) (schema.TypeDefinition, error) {
	for _, td := range s.Types {
		if td.Name == typeName {
			return td, nil
		}
	}
	return schema.TypeDefinition{}, fmt.Errorf("type %q not found in schema %s", typeName, s.ID)
}

// FindRelationshipDef returns the [schema.RelationshipDefinition] with the
// given label declared on the source TypeDefinition, or an error if no
// matching definition exists.
//
// Implementations should call this inside CreateRelationship to validate
// the edge label and target type before writing to the relationships
// table:
//
//	td, err := entitygraph.FindTypeDef(s, fromEntity.TypeID)
//	if err != nil { return entitygraph.ErrInvalidRelationship }
//	rd, err := entitygraph.FindRelationshipDef(td, label)
//	if err != nil { return entitygraph.ErrInvalidRelationship }
//	if rd.ToType != toEntity.TypeID { return entitygraph.ErrInvalidRelationship }
func FindRelationshipDef(td schema.TypeDefinition, label string) (schema.RelationshipDefinition, error) {
	for _, rd := range td.Relationships {
		if rd.Name == label {
			return rd, nil
		}
	}
	return schema.RelationshipDefinition{}, fmt.Errorf("relationship %q not declared on type %q", label, td.Name)
}

// ValidateCreateRelationship checks that the proposed edge is permitted by
// the schema. Must be called by every DataManager backend before writing an
// edge.
//
// Rules enforced:
//  1. label must match a RelationshipDefinition.Name on fromTypeDef.
//  2. toTypeID must equal RelationshipDefinition.ToType.
//
// Cardinality (ToMany=false upsert vs. ToMany=true insert) is handled by
// the backend write strategy — not by this function.
//
// Returns [ErrInvalidRelationship] if either rule is violated.
func ValidateCreateRelationship(fromTypeDef schema.TypeDefinition, label, toTypeID string) error {
	rd, err := FindRelationshipDef(fromTypeDef, label)
	if err != nil {
		return ErrInvalidRelationship
	}
	if rd.ToType != toTypeID {
		return ErrInvalidRelationship
	}
	return nil
}

// ValidateSchema checks the internal consistency of a [schema.Schema]
// before it is persisted by [SchemaManager.Publish]. Called inside Publish
// — invalid schemas are rejected and no snapshot is created.
//
// Rules enforced:
//  1. All TypeDefinition.Name values are unique within the schema.
//  2. For every RelationshipDefinition where Inverse != "":
//     a. ToType must reference a TypeDefinition.Name in the same schema.
//     b. The ToType's TypeDefinition must declare a RelationshipDefinition
//     with Name == rd.Inverse.
//  3. Every UniqueKey field references a declared property.
//
// Returns a descriptive error on the first violation found.
func ValidateSchema(s schema.Schema) error {
	typeNames := make(map[string]struct{}, len(s.Types))

	for _, td := range s.Types {
		if _, dup := typeNames[td.Name]; dup {
			return fmt.Errorf("ValidateSchema: duplicate type name %q", td.Name)
		}
		typeNames[td.Name] = struct{}{}
	}

	for _, td := range s.Types {
		for _, rd := range td.Relationships {
			if rd.Inverse != "" {
				toTypeDef, err := FindTypeDef(s, rd.ToType)
				if err != nil {
					return fmt.Errorf("ValidateSchema: type %q: relationship %q: ToType %q not found in schema",
						td.Name, rd.Name, rd.ToType)
				}
				if _, err := FindRelationshipDef(toTypeDef, rd.Inverse); err != nil {
					return fmt.Errorf("ValidateSchema: type %q: relationship %q: inverse %q not declared on %q",
						td.Name, rd.Name, rd.Inverse, rd.ToType)
				}
			}
		}

		// Validate that every UniqueKey field references a declared property.
		if len(td.UniqueKey) > 0 {
			propNames := make(map[string]struct{}, len(td.Properties))
			for _, pd := range td.Properties {
				propNames[pd.Name] = struct{}{}
			}
			for _, keyField := range td.UniqueKey {
				if _, ok := propNames[keyField]; !ok {
					return fmt.Errorf("ValidateSchema: type %q: UniqueKey field %q not found in Properties",
						td.Name, keyField)
				}
			}
		}
	}

	return nil
}

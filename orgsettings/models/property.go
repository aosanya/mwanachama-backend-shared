package models

import "fmt"

// PropertyRange is the datatype an Attributes entry may hold. Mirrors
// mwanachama-backend-actor/models.PropertyRange exactly.
type PropertyRange string

const (
	RangeText        PropertyRange = "text"
	RangeNumber      PropertyRange = "number"
	RangeBoolean     PropertyRange = "boolean"
	RangeEnumeration PropertyRange = "enumeration"
)

// Property declares one entry Settings.Attributes may carry — mirrors
// mwanachama-backend-actor/models.Property exactly, including the fields
// this package's own DefaultOrgSettingsProperties doesn't use yet (Unique,
// Options): the shape is copied whole so a future declared property here
// can use the full vocabulary without a second copy-and-adapt pass.
type Property struct {
	Name  string        `json:"name"`
	Label string        `json:"label"`
	Range PropertyRange `json:"range"`
	// Options are the permitted values, for RangeEnumeration only.
	Options []string `json:"options,omitempty"`
	// Required rejects a write whose Attributes omits this property or sets
	// it to the empty string / nil.
	Required bool `json:"required"`
	// Unique rejects a write whose Attributes value for this property
	// already appears on another row. Unused today — org_settings is a
	// single-row-per-deployment table, so nothing is unique against.
	Unique bool `json:"unique"`
}

// DefaultOrgSettingsProperties is the built-in property catalog validated
// against Settings.Attributes on every Put — mirrors
// mwanachama-backend-actor/models.DefaultActorProperties's shape exactly.
//
// Every value moved here from a named Settings field (DEV-1683 follow-up),
// matching how mwanachama-backend-actor's phone/email are validated
// Attributes entries rather than dedicated columns. None are Required —
// the original database columns were all `NOT NULL DEFAULT ''`, never
// actually enforced as mandatory at write time, so an org_settings row with
// every value blank was always a valid state (an org that has not filled
// its profile in yet). None are Unique — this table has one row per
// deployment, so nothing is ever unique against.
func DefaultOrgSettingsProperties() []Property {
	return []Property{
		{Name: "display_name", Label: "Display name", Range: RangeText},
		{Name: "primary_color", Label: "Primary color", Range: RangeText},
		{Name: "accent_color", Label: "Accent color", Range: RangeText},
		{Name: "logo_url", Label: "Logo URL", Range: RangeText},
		{Name: "support_email", Label: "Support email", Range: RangeText},
		{Name: "default_dialling_region", Label: "Default dialling region", Range: RangeText},
	}
}

// ValidateAttributes checks attrs against every Required and Range/Options
// constraint in properties. Mirrors
// mwanachama-backend-actor/models.ValidateAttributes exactly — a property
// absent from properties entirely is left alone, this is not a
// closed-schema check.
func ValidateAttributes(properties []Property, attrs map[string]any) error {
	for _, p := range properties {
		v, present := attrs[p.Name]
		if !present || isBlank(v) {
			if p.Required {
				return fmt.Errorf("%q is required", p.Name)
			}
			continue
		}
		if err := checkRange(p, v); err != nil {
			return fmt.Errorf("%q: %w", p.Name, err)
		}
	}
	return nil
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

func checkRange(p Property, v any) error {
	switch p.Range {
	case RangeText:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("must be text, got %T", v)
		}
	case RangeNumber:
		switch v.(type) {
		case float64, float32, int, int32, int64:
		default:
			return fmt.Errorf("must be a number, got %T", v)
		}
	case RangeBoolean:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("must be a boolean, got %T", v)
		}
	case RangeEnumeration:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("must be text (an enumeration option), got %T", v)
		}
		for _, opt := range p.Options {
			if opt == s {
				return nil
			}
		}
		return fmt.Errorf("%q is not one of %v", s, p.Options)
	}
	return nil
}

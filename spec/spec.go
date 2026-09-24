// Package spec is the declared shape of a domain: the objects it stores,
// their fields, and the indexes worth keeping. A module supplies the acts; a
// spec supplies the nouns.
//
// Nothing here knows what a catalog is. The same loader serves an agency
// storefront declaring roles and objectives and a library declaring copies
// and loans, which is the whole reason the objects are data rather than Go
// types.
//
// Identifiers in a spec reach the database as SQL text — a table name cannot
// be a bound parameter — so every name is validated against a strict
// alphabet when the spec loads, and a spec that fails validation is refused
// at startup rather than at the first request.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// NamePattern is the only shape an object, field or index name may take:
// lowercase, starting with a letter, words joined by single underscores.
//
// It is strict because these names are interpolated into DDL. Anything a
// caller could use to close an identifier and continue the statement is
// excluded from the alphabet outright rather than escaped.
var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// SegmentPattern is the shape of the instance and module segments, which is
// [NamePattern] minus the underscore. A physical name is
// <instance>_<module>_<table>, and with the separator legal inside the first
// two segments that name could not be read back apart — a_b_c_goals does not
// say which part is the instance. The object's own table name is last, so it
// keeps its underscores.
var SegmentPattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// MaxIdentifier is Postgres's identifier limit. It matters because the
// database does not complain: NAMEDATALEN-1 is 63 bytes and anything longer
// is silently truncated, so two names that agree for 63 bytes are one name.
// Every identifier the spec emits is measured against this when it loads.
const MaxIdentifier = 63

// FieldType is what a column holds. The set is closed: a type without an arm
// in [Field.columnType] should fail when the spec loads, not produce a column
// nobody meant.
type FieldType string

const (
	// TypeString is a short identifier or label.
	TypeString FieldType = "string"

	// TypeText is free prose of any length.
	TypeText FieldType = "text"

	// TypeInt is a whole number.
	TypeInt FieldType = "int"

	// TypeBool is true or false.
	TypeBool FieldType = "bool"

	// TypeJSON is a document — jsonb where the database has it.
	TypeJSON FieldType = "json"

	// TypeTimestamp is an RFC 3339 instant, stored as text so every dialect
	// compares it the same way.
	TypeTimestamp FieldType = "timestamp"

	// TypeEnum is a string restricted to Values.
	TypeEnum FieldType = "enum"
)

var fieldTypes = map[FieldType]bool{
	TypeString: true, TypeText: true, TypeInt: true, TypeBool: true,
	TypeJSON: true, TypeTimestamp: true, TypeEnum: true,
}

// Field is one column on one object.
type Field struct {
	Name string    `json:"name"`
	Type FieldType `json:"type"`

	// Description says what this field is for, in the domain's own words.
	// Required: a column nobody can explain is the one that rots, and the
	// spec is the only place the explanation would live.
	Description string `json:"description"`

	// Primary marks the primary key. An object needs at least one; several
	// make a composite key in declared order.
	Primary bool `json:"primary,omitempty"`

	// Required rejects an empty value at write time.
	Required bool `json:"required,omitempty"`

	// Unique constrains the column across the table.
	Unique bool `json:"unique,omitempty"`

	// Immutable rejects a change after creation.
	Immutable bool `json:"immutable,omitempty"`

	// Default is the value a row takes when the field is absent.
	Default string `json:"default,omitempty"`

	// Values is the permitted set, for an enum.
	Values []string `json:"values,omitempty"`

	// Matches names a validation pattern the module supplies, such as
	// "slug". The spec names it; the module owns what it means.
	Matches string `json:"matches,omitempty"`
}

// Index is one index on one object. Either Fields or Path is set, never
// both: Path indexes a document path, which is how a domain makes its own
// vocabulary fast without the module learning it.
type Index struct {
	Name string `json:"name"`

	// Description says what query this index is for. Optional, but an index
	// without a reason is the one nobody dares drop.
	Description string `json:"description,omitempty"`

	Fields []string `json:"fields,omitempty"`

	// Path is a dotted path inside a json field, e.g.
	// {"field": "doc", "path": "_template.sector"}.
	Path *PathRef `json:"path,omitempty"`

	Unique bool `json:"unique,omitempty"`

	// NotDeleted restricts the index to rows whose "deleted" column is
	// false. It is a named condition rather than free SQL, because free SQL
	// here would be an injection surface with no upside.
	NotDeleted bool `json:"not_deleted,omitempty"`
}

// PathRef names a path inside a json field.
type PathRef struct {
	Field string `json:"field"`
	Path  string `json:"path"`
}

// Object is one declared thing, and one table.
//
// Name and Table are the domain's own words — an agency, a title. Role is
// the module's, and is how a rule finds the object it operates on: the
// visibility check needs to know which object is the "entry" whatever the
// domain calls it. An object with no Role is the domain's own, and no rule
// in the module touches it.
type Object struct {
	Name string `json:"name"`

	// Description says what this object is, in the domain's own words.
	// Required, for the same reason a field's is.
	Description string `json:"description"`

	// Role names this object in the module's vocabulary — "entry",
	// "share_link", "suggestion". Optional: an object without one is
	// storage the domain asked for and nothing else.
	Role string `json:"role,omitempty"`

	// Table is the table's name within the instance. The instance prefix is
	// added by [Spec.TableFor].
	Table string `json:"table"`

	Fields  []Field `json:"fields"`
	Indexes []Index `json:"indexes,omitempty"`
}

// Spec is one domain's declared shape.
type Spec struct {
	// Module names the Go module whose acts this spec is declared against.
	Module string `json:"module"`

	// Domain names the domain — "agency", "library". It is a label, never
	// read as behaviour.
	Domain string `json:"domain"`

	// Instance prefixes every table, so several domains share a database.
	Instance string `json:"instance"`

	Objects []Object `json:"objects"`
}

// Load reads and validates a spec. A spec that does not validate is refused
// here, which is the only moment at which refusing it is cheap.
func Load(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec: read %s: %w", path, err)
	}
	return Parse(raw)
}

// Parse validates a spec already in memory.
func Parse(raw []byte) (*Spec, error) {
	var s Spec
	if err := decode(raw, &s); err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func decode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

// DocPathPattern is the shape a path inside a document may take. Same
// alphabet and same reason as a name: it reaches SQL as text.
var DocPathPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// TableFor returns the physical table name for one object:
// <instance>_<module>_<table>, e.g. agency_catalog_agencies.
//
// The module segment is what lets two modules share one database without
// arranging not to collide. Without it, a catalog instance named "agency"
// and an agency instance of the same name both want agency_agencies, and
// nothing in either module would notice: GORM's AutoMigrate adopts a table
// that already exists and adds the missing columns, and this package's own
// "create table if not exists" is a no-op against one.
func (s *Spec) TableFor(o Object) string {
	return s.Instance + "_" + s.Module + "_" + o.table()
}

// IndexFor returns the physical index name for one index on one object.
// Postgres keeps indexes in the same per-schema namespace as tables, so this
// has to be unique across the whole database, not just its table — which is
// why the table name is part of it.
func (s *Spec) IndexFor(o Object, idx Index) string {
	return s.TableFor(o) + "_" + idx.Name + "_idx"
}

func (o Object) table() string {
	if o.Table != "" {
		return o.Table
	}
	return o.Name
}

// Object returns the declared object of that name.
func (s *Spec) Object(name string) (Object, bool) {
	for _, o := range s.Objects {
		if o.Name == name {
			return o, true
		}
	}
	return Object{}, false
}

// ByRole returns the object playing one of the module's roles — how a rule
// reaches its table without knowing what the domain calls it.
func (s *Spec) ByRole(role string) (Object, bool) {
	for _, o := range s.Objects {
		if o.Role == role {
			return o, true
		}
	}
	return Object{}, false
}

// RequireRoles reports every role the module needs that this spec does not
// fill. A module calls it once, when the spec loads: a rule that cannot find
// its object should stop the process starting, not fail a request later.
func (s *Spec) RequireRoles(roles ...string) error {
	var missing []string
	for _, r := range roles {
		if _, ok := s.ByRole(r); !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("spec: domain %q fills no object for the role(s) %s",
			s.Domain, strings.Join(missing, ", "))
	}
	return nil
}

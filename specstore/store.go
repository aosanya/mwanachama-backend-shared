package specstore

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

type Store struct {
	db      *gorm.DB
	spec    *spec.Spec
	objects map[string]spec.Object
}

func New(db *gorm.DB, s *spec.Spec, carriers map[string]any) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("specstore: db must not be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("specstore: spec must not be nil")
	}

	roles := make([]string, 0, len(carriers))
	for role := range carriers {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	if err := s.RequireRoles(roles...); err != nil {
		return nil, err
	}

	objects := make(map[string]spec.Object, len(carriers))
	var problems []string
	for _, role := range roles {
		o, _ := s.ByRole(role)
		objects[role] = o
		problems = append(problems, disagreements(o, carriers[role])...)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("specstore: spec and types disagree: %s", strings.Join(problems, "; "))
	}
	return &Store{db: db, spec: s, objects: objects}, nil
}

func disagreements(o spec.Object, carrier any) []string {
	t := reflect.TypeOf(carrier)

	declared := make(map[string]spec.Field, len(o.Fields))
	for _, f := range o.Fields {
		declared[f.Name] = f
	}
	carried := make(map[string]reflect.StructField, t.NumField())
	for _, f := range carriedFields(t) {
		carried[ColumnName(f.Name)] = f
	}

	var out []string
	for name := range declared {
		if _, ok := carried[name]; !ok {
			out = append(out, fmt.Sprintf("%s declares %q, which %s does not carry", o.Name, name, t))
		}
	}
	for name, held := range carried {
		f, ok := declared[name]
		if !ok {
			out = append(out, fmt.Sprintf("%s carries %q, which the object %q does not declare", t, name, o.Name))
			continue
		}
		switch pointer := held.Type.Kind() == reflect.Pointer; {
		case f.Nullable && !pointer:
			out = append(out, fmt.Sprintf("%s declares %q nullable and %s carries it as %s, so a stored NULL would read back as the zero value",
				o.Name, name, t, held.Type))
		case pointer && !f.Nullable:
			out = append(out, fmt.Sprintf("%s carries %q as %s and the object %q does not declare it nullable, so an absent value would be written as the zero one",
				t, name, held.Type, o.Name))
		}
	}
	return out
}

func (st *Store) Spec() *spec.Spec { return st.spec }

func (st *Store) Table(role string) string { return st.spec.TableFor(st.Object(role)) }

func (st *Store) Object(role string) spec.Object {
	o, ok := st.objects[role]
	if !ok {
		panic("specstore: no object for role " + role)
	}
	return o
}

func (st *Store) Query(ctx context.Context, role string) *gorm.DB {
	return st.db.WithContext(ctx).Table(st.Table(role))
}

func (st *Store) Take(q *gorm.DB, role string, out any, notFound error) error {
	var rows []map[string]any
	if err := q.Limit(1).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return notFound
	}
	return Decode(st.Object(role), rows[0], out)
}

func (st *Store) Insert(ctx context.Context, role string, v any) error {
	row, err := Encode(st.Object(role), v)
	if err != nil {
		return err
	}
	return st.Query(ctx, role).Create(row).Error
}

func List[T any](st *Store, q *gorm.DB, role string) ([]T, error) {
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	o := st.Object(role)
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		var v T
		if err := Decode(o, r, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func NewID() string { return uuid.NewString() }

func ColumnsOf(t reflect.Type) map[string]bool {
	out := map[string]bool{}
	for _, f := range carriedFields(t) {
		out[ColumnName(f.Name)] = true
	}
	return out
}

const skipTag = "-"

func carriedFields(t reflect.Type) []reflect.StructField {
	out := make([]reflect.StructField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" || f.Tag.Get("spec") == skipTag {
			continue
		}
		out = append(out, f)
	}
	return out
}

func ColumnName(field string) string {
	var b strings.Builder
	runes := []rune(field)
	for i, r := range runes {
		upper := r >= 'A' && r <= 'Z'
		if upper && i > 0 {
			prevLower := runes[i-1] >= 'a' && runes[i-1] <= 'z' || runes[i-1] >= '0' && runes[i-1] <= '9'
			// A lone trailing "s" pluralises the run it follows rather than
			// starting a word of its own, so OptionIDs is option_ids and not
			// option_i_ds. Every other lowercase after a run does start one.
			plural := i+2 == len(runes) && runes[i+1] == 's'
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' && !plural
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		if upper {
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func Encode(o spec.Object, v any) (map[string]any, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("encode %s: want a struct, got %T", o.Name, v)
	}

	byColumn := fieldsByColumn(rv)

	row := make(map[string]any, len(o.Fields))
	for _, f := range o.Fields {
		fv, ok := byColumn[f.Name]
		if !ok {
			return nil, fmt.Errorf("encode %s: nothing carries %q", o.Name, f.Name)
		}
		cell, err := cell(f, fv)
		if err != nil {
			return nil, fmt.Errorf("encode %s.%s: %w", o.Name, f.Name, err)
		}
		row[f.Name] = cell
	}
	return row, nil
}

func cell(f spec.Field, fv reflect.Value) (any, error) {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil, nil
		}
		fv = fv.Elem()
	}
	switch f.Type {
	case spec.TypeInt:
		return fv.Int(), nil
	case spec.TypeFloat:
		return fv.Float(), nil
	case spec.TypeBool:
		return fv.Bool(), nil
	case spec.TypeJSON:
		return document(fv)
	default:
		return fv.String(), nil
	}
}

// document renders a json column's value. A string is already the document
// and is stored as it stands; anything else is marshalled. Empty is SQL
// NULL, which is what keeps a column never written apart from one holding
// an empty document.
func document(fv reflect.Value) (any, error) {
	switch fv.Kind() {
	case reflect.String:
		if s := fv.String(); s != "" {
			return s, nil
		}
		return nil, nil
	case reflect.Slice, reflect.Array, reflect.Map:
		if fv.Len() == 0 {
			return nil, nil
		}
	}
	raw, err := json.Marshal(fv.Interface())
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func Decode(o spec.Object, row map[string]any, out any) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("decode %s: want a pointer to a struct, got %T", o.Name, out)
	}
	rv = rv.Elem()

	for _, f := range carriedFields(rv.Type()) {
		raw, ok := row[ColumnName(f.Name)]
		if !ok || raw == nil {
			continue
		}
		if err := assign(rv.FieldByIndex(f.Index), raw); err != nil {
			return fmt.Errorf("decode %s.%s: %w", o.Name, f.Name, err)
		}
	}
	return nil
}

func fieldsByColumn(rv reflect.Value) map[string]reflect.Value {
	out := map[string]reflect.Value{}
	for _, f := range carriedFields(rv.Type()) {
		out[ColumnName(f.Name)] = rv.FieldByIndex(f.Index)
	}
	return out
}

func assign(field reflect.Value, raw any) error {
	if field.Kind() == reflect.Pointer {
		held := reflect.New(field.Type().Elem())
		if err := assign(held.Elem(), raw); err != nil {
			return err
		}
		field.Set(held)
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		switch v := raw.(type) {
		case string:
			field.SetString(v)
		case []byte:
			field.SetString(string(v))
		default:
			return fmt.Errorf("cannot read %T as text", raw)
		}
	case reflect.Bool:
		switch v := raw.(type) {
		case bool:
			field.SetBool(v)
		case int64:
			field.SetBool(v != 0)
		default:
			return fmt.Errorf("cannot read %T as a boolean", raw)
		}
	case reflect.Int, reflect.Int64:
		switch v := raw.(type) {
		case int64:
			field.SetInt(v)
		case int:
			field.SetInt(int64(v))
		case float64:
			field.SetInt(int64(v))
		default:
			return fmt.Errorf("cannot read %T as a number", raw)
		}
	case reflect.Float32, reflect.Float64:
		switch v := raw.(type) {
		case float64:
			field.SetFloat(v)
		case float32:
			field.SetFloat(float64(v))
		case int64:
			field.SetFloat(float64(v))
		default:
			return fmt.Errorf("cannot read %T as a number", raw)
		}
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return unmarshal(field, raw)
	default:
		return fmt.Errorf("no rule for a %s field", field.Kind())
	}
	return nil
}

func unmarshal(field reflect.Value, raw any) error {
	var data []byte
	switch v := raw.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("cannot read %T as a document", raw)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, field.Addr().Interface())
}

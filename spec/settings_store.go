package spec

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const settingsTablePrefix = "spec_settings_"

const (
	SettingsDefinitionTable = "spec_settings_definition"
	SettingsOverrideTable   = "spec_settings_override"
	SettingsChangeTable     = "spec_settings_change"
)

func textField(name, description string, primary bool) Field {
	return Field{Name: name, Type: TypeText, Description: description, Primary: primary}
}

func (s *Spec) SettingsObjects() []Object {
	if len(s.Settings) == 0 {
		return nil
	}
	return []Object{
		{
			Name: SettingsDefinitionTable, Table: SettingsDefinitionTable,
			Description: "The values this agency's definition gives the module's settings.",
			Fields: []Field{
				textField("name", "The declared setting.", true),
				textField("value", "The value as a JSON document.", false),
			},
		},
		{
			Name: SettingsOverrideTable, Table: SettingsOverrideTable,
			Description: "The owner's overrides of the module's settings.",
			Fields: []Field{
				textField("name", "The declared setting.", true),
				textField("value", "The value as a JSON document.", false),
				textField("set_by", "Who set the override.", false),
				textField("set_at", "When the override was set.", false),
			},
		},
		{
			Name: SettingsChangeTable, Table: SettingsChangeTable,
			Description: "Every override set or cleared, in order.",
			Fields: []Field{
				textField("id", "Storage key.", true),
				textField("name", "The declared setting.", false),
				textField("from_value", "The override replaced, as a JSON document, or empty.", false),
				textField("to_value", "The override set, as a JSON document, or empty when cleared.", false),
				{Name: "cleared", Type: TypeBool, Description: "Whether the change cleared the override."},
				textField("changed_by", "Who made the change.", false),
				textField("changed_at", "When the change was made.", false),
			},
			Indexes: []Index{{Name: "order", Fields: []string{"changed_at", "id"}}},
		},
	}
}

func (s *Spec) storedObjects() []Object {
	return append(append([]Object(nil), s.Objects...), s.SettingsObjects()...)
}

func (s *Spec) settingsTable(table string) string {
	for _, o := range s.SettingsObjects() {
		if o.Table == table {
			return s.TableFor(o)
		}
	}
	return ""
}

func (s *Spec) SettingsTables() []string {
	var out []string
	for _, o := range s.SettingsObjects() {
		out = append(out, s.TableFor(o))
	}
	return out
}

func encodeSetting(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeSetting(raw string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	var v any
	err := json.Unmarshal([]byte(raw), &v)
	return v, err
}

func (s *Spec) requireSettings() error {
	if len(s.Settings) == 0 {
		return fmt.Errorf("spec: module %q declares no settings", s.Module)
	}
	return nil
}

func PutDefinitionSettings(db *gorm.DB, s *Spec, values map[string]any) error {
	if err := s.requireSettings(); err != nil {
		return err
	}
	if problems := s.CheckSettings(values); len(problems) > 0 {
		return fmt.Errorf("spec: settings: %s", strings.Join(problems, "; "))
	}
	table := s.settingsTable(SettingsDefinitionTable)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("delete from " + table).Error; err != nil {
			return err
		}
		for name, v := range values {
			raw, err := encodeSetting(v)
			if err != nil {
				return err
			}
			if err := tx.Table(table).Create(map[string]any{"name": name, "value": raw}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type settingRow struct {
	Name  string `gorm:"column:name"`
	Value string `gorm:"column:value"`
	SetBy string `gorm:"column:set_by"`
	SetAt string `gorm:"column:set_at"`
}

func LoadSettingLayers(db *gorm.DB, s *Spec) (map[string]any, map[string]Override, error) {
	if err := s.requireSettings(); err != nil {
		return nil, nil, err
	}
	var defs []settingRow
	if err := db.Table(s.settingsTable(SettingsDefinitionTable)).Select("name", "value").Find(&defs).Error; err != nil {
		return nil, nil, fmt.Errorf("spec: settings: %w", err)
	}
	definition := map[string]any{}
	for _, r := range defs {
		v, err := decodeSetting(r.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("spec: settings: %s: %w", r.Name, err)
		}
		definition[r.Name] = v
	}
	var ovs []settingRow
	if err := db.Table(s.settingsTable(SettingsOverrideTable)).Find(&ovs).Error; err != nil {
		return nil, nil, fmt.Errorf("spec: settings: %w", err)
	}
	overrides := map[string]Override{}
	for _, r := range ovs {
		v, err := decodeSetting(r.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("spec: settings: %s: %w", r.Name, err)
		}
		overrides[r.Name] = Override{Value: v, By: r.SetBy, At: r.SetAt}
	}
	return definition, overrides, nil
}

func ResolveStored(db *gorm.DB, s *Spec, name string) (Effective, error) {
	definition, overrides, err := LoadSettingLayers(db, s)
	if err != nil {
		return Effective{}, err
	}
	return Resolve(s.Settings, definition, overrides, name)
}

func ResolveAllStored(db *gorm.DB, s *Spec) ([]Effective, error) {
	definition, overrides, err := LoadSettingLayers(db, s)
	if err != nil {
		return nil, err
	}
	return ResolveAll(s.Settings, definition, overrides), nil
}

func StoreOverride(db *gorm.DB, s *Spec, name string, value any, by, at string) (SettingChange, error) {
	var change SettingChange
	err := db.Transaction(func(tx *gorm.DB) error {
		_, overrides, err := LoadSettingLayers(tx, s)
		if err != nil {
			return err
		}
		next, c, err := SetOverride(s.Settings, overrides, name, value, by, at)
		if err != nil {
			return err
		}
		raw, err := encodeSetting(next[name].Value)
		if err != nil {
			return err
		}
		err = tx.Table(s.settingsTable(SettingsOverrideTable)).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "set_by", "set_at"}),
		}).Create(map[string]any{"name": name, "value": raw, "set_by": by, "set_at": at}).Error
		if err != nil {
			return err
		}
		change = c
		return recordChange(tx, s, c)
	})
	return change, err
}

func StoreClearOverride(db *gorm.DB, s *Spec, name, by, at string) (SettingChange, bool, error) {
	var change SettingChange
	var cleared bool
	err := db.Transaction(func(tx *gorm.DB) error {
		_, overrides, err := LoadSettingLayers(tx, s)
		if err != nil {
			return err
		}
		_, c, ok, err := ClearOverride(s.Settings, overrides, name, by, at)
		if err != nil || !ok {
			return err
		}
		if err := tx.Exec("delete from "+s.settingsTable(SettingsOverrideTable)+" where name = ?", name).Error; err != nil {
			return err
		}
		change, cleared = c, true
		return recordChange(tx, s, c)
	})
	return change, cleared, err
}

func recordChange(tx *gorm.DB, s *Spec, c SettingChange) error {
	from, err := encodeSetting(c.From)
	if err != nil {
		return err
	}
	to, err := encodeSetting(c.To)
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	return tx.Table(s.settingsTable(SettingsChangeTable)).Create(map[string]any{
		"id": hex.EncodeToString(id), "name": c.Name, "from_value": from, "to_value": to,
		"cleared": c.Cleared, "changed_by": c.By, "changed_at": c.At,
	}).Error
}

type changeRow struct {
	Name      string `gorm:"column:name"`
	FromValue string `gorm:"column:from_value"`
	ToValue   string `gorm:"column:to_value"`
	Cleared   bool   `gorm:"column:cleared"`
	ChangedBy string `gorm:"column:changed_by"`
	ChangedAt string `gorm:"column:changed_at"`
}

func SettingChanges(db *gorm.DB, s *Spec) ([]SettingChange, error) {
	if err := s.requireSettings(); err != nil {
		return nil, err
	}
	var rows []changeRow
	if err := db.Table(s.settingsTable(SettingsChangeTable)).Order("changed_at, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("spec: settings: %w", err)
	}
	out := make([]SettingChange, 0, len(rows))
	for _, r := range rows {
		from, err := decodeSetting(r.FromValue)
		if err != nil {
			return nil, err
		}
		to, err := decodeSetting(r.ToValue)
		if err != nil {
			return nil, err
		}
		out = append(out, SettingChange{Name: r.Name, From: from, To: to, Cleared: r.Cleared, By: r.ChangedBy, At: r.ChangedAt})
	}
	return out, nil
}

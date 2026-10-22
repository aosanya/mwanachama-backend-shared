package spec

import (
	"errors"
	"fmt"
	"maps"
)

var ErrSettingChangeUnattributed = errors.New("a settings change needs who made it and when")

type SettingChange struct {
	Name    string `json:"name"`
	From    any    `json:"from,omitempty"`
	To      any    `json:"to,omitempty"`
	Cleared bool   `json:"cleared,omitempty"`
	By      string `json:"by"`
	At      string `json:"at"`
}

func SetOverride(settings []Setting, overrides map[string]Override, name string, value any, by, at string) (map[string]Override, SettingChange, error) {
	st, ok := findSetting(settings, name)
	if !ok {
		return overrides, SettingChange{}, fmt.Errorf("setting %q is not declared by this module", name)
	}
	if by == "" || at == "" {
		return overrides, SettingChange{}, ErrSettingChangeUnattributed
	}
	if problem := st.check(value); problem != "" {
		return overrides, SettingChange{}, fmt.Errorf("setting %q: %s", name, problem)
	}
	next := maps.Clone(overrides)
	if next == nil {
		next = map[string]Override{}
	}
	change := SettingChange{Name: name, To: value, By: by, At: at}
	if prior, ok := overrides[name]; ok {
		change.From = prior.Value
	}
	next[name] = Override{Value: value, By: by, At: at}
	return next, change, nil
}

func ClearOverride(settings []Setting, overrides map[string]Override, name, by, at string) (map[string]Override, SettingChange, bool, error) {
	if _, ok := findSetting(settings, name); !ok {
		return overrides, SettingChange{}, false, fmt.Errorf("setting %q is not declared by this module", name)
	}
	if by == "" || at == "" {
		return overrides, SettingChange{}, false, ErrSettingChangeUnattributed
	}
	prior, ok := overrides[name]
	if !ok {
		return overrides, SettingChange{}, false, nil
	}
	next := maps.Clone(overrides)
	delete(next, name)
	return next, SettingChange{Name: name, From: prior.Value, Cleared: true, By: by, At: at}, true, nil
}

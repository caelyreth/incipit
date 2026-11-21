package settings

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/policy"
)

// persistedVersion is the settings record's format. A record from another
// version is not read; the app starts from its defaults instead.
const persistedVersion = 1

// persisted is the settings record as it lives beside the notebook.
// Appearance and writing modes are stored by name, not by enum number, so
// inserting a new mode cannot silently change an old preference.
type persisted struct {
	Version      int                        `json:"version"`
	Motion       bool                       `json:"motion"`
	Typewriter   bool                       `json:"typewriter"`
	Texture      *bool                      `json:"texture,omitempty"`
	Theme        string                     `json:"theme"`
	UIFontSize   float32                    `json:"uiFontSize"`
	ProseSize    float32                    `json:"proseFontSize"`
	LineSpacing  float32                    `json:"lineSpacing"`
	HanFace      string                     `json:"hanFace,omitempty"`
	Keymap       map[string]*persistedChord `json:"keymap,omitempty"`
	PolicyValues map[string]json.RawMessage `json:"policy,omitempty"`
}

type persistedChord struct {
	Key       string `json:"key"`
	Modifiers uint8  `json:"modifiers"`
}

// Encode writes every setting the app keeps between launches.
func (s State) Encode() ([]byte, error) {
	values, err := json.Marshal(s.Policy)
	if err != nil {
		return nil, fmt.Errorf("settings: encode policy values: %w", err)
	}
	var policyValues map[string]json.RawMessage
	if err := json.Unmarshal(values, &policyValues); err != nil {
		return nil, fmt.Errorf("settings: encode policy values: %w", err)
	}
	texture := s.Presentation.Texture
	record := persisted{
		Version:      persistedVersion,
		Motion:       s.Presentation.Motion,
		Typewriter:   s.Presentation.Typewriter,
		Texture:      &texture,
		Theme:        themeName(s.Appearance.Appearance),
		UIFontSize:   s.Appearance.UIFontSize,
		ProseSize:    s.Appearance.ProseFontSize,
		LineSpacing:  s.Appearance.LineSpacing,
		HanFace:      s.Appearance.HanFace,
		Keymap:       persistedOverrides(s.Keymap),
		PolicyValues: policyValues,
	}
	return json.Marshal(record)
}

// Decode replaces s with the recorded settings. A record that cannot be
// read or compiled leaves s unchanged; the caller may then keep defaults
// and tell the person that the preferences were reset. Pages are never
// touched by this reset.
func (s *State) Decode(data []byte) error {
	if s == nil {
		return ErrInvalidChange
	}
	var record persisted
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("settings: decode: %w", err)
	}
	if record.Version != persistedVersion {
		return fmt.Errorf("settings: unknown settings version %d", record.Version)
	}
	theme, ok := parseTheme(record.Theme)
	if !ok {
		return ErrInvalidAppearance
	}
	values, err := parsePolicyValues(record.PolicyValues)
	if err != nil {
		return err
	}
	prog, err := manifest.Compile(values, false)
	if err != nil {
		return fmt.Errorf("settings: compile saved policy values: %w", err)
	}
	keys, err := parseOverrides(keymap.Declared(prog.Catalog()), record.Keymap)
	if err != nil {
		return err
	}
	texture := true
	if record.Texture != nil {
		texture = *record.Texture
	}
	candidate := State{
		Presentation: PresentationState{
			Motion:     record.Motion,
			Typewriter: record.Typewriter,
			Texture:    texture,
		},
		Appearance: AppearanceSettings{
			Appearance:    theme,
			UIFontSize:    record.UIFontSize,
			ProseFontSize: record.ProseSize,
			LineSpacing:   record.LineSpacing,
			HanFace:       record.HanFace,
		}.Normalized(),
		Keymap:  keys,
		Policy:  values,
		Program: prog,
	}
	*s = candidate
	return nil
}

func themeName(theme ThemeChoice) string {
	switch theme {
	case Daybreak:
		return "daybreak"
	default:
		return "starfield"
	}
}

func parseTheme(name string) (ThemeChoice, bool) {
	switch name {
	case "starfield":
		return Starfield, true
	case "daybreak":
		return Daybreak, true
	default:
		return 0, false
	}
}

func persistedOverrides(keys *keymap.Keymap) map[string]*persistedChord {
	if keys == nil {
		return nil
	}
	overrides := keys.Overrides()
	if len(overrides) == 0 {
		return nil
	}
	out := make(map[string]*persistedChord, len(overrides))
	for command, chord := range overrides {
		var saved *persistedChord
		if chord != nil {
			saved = &persistedChord{Key: chord.Key, Modifiers: uint8(chord.Modifiers)}
		}
		out[string(command)] = saved
	}
	return out
}

func parseOverrides(commands []keymap.Command, saved map[string]*persistedChord) (*keymap.Keymap, error) {
	declared := keymap.Default(commands)
	overrides := keymap.Overrides{}
	for name, saved := range saved {
		command := keymap.CommandID(name)
		if _, ok := declared.Command(command); !ok {
			continue
		}
		if saved == nil {
			overrides[command] = nil
			continue
		}
		chord := keymap.Chord{Key: saved.Key, Modifiers: keymap.Modifiers(saved.Modifiers)}
		overrides[command] = &chord
	}
	keys, err := keymap.New(commands, overrides)
	if err != nil {
		return nil, fmt.Errorf("settings: saved keymap: %w", err)
	}
	return keys, nil
}

// parsePolicyValues reads values against this build's catalog. Options the
// build no longer knows, or values outside their domain, are dropped, so a
// settings record survives a policy being added or removed.
func parsePolicyValues(saved map[string]json.RawMessage) (policy.Values, error) {
	if len(saved) == 0 {
		return nil, nil
	}
	catalog := DefaultState().Program.Catalog()
	known := make(map[string]policy.OptionInfo)
	for _, p := range catalog.Policies {
		for _, o := range p.Options {
			known[o.Key] = o
		}
	}
	values := policy.Values{}
	for key, raw := range saved {
		info, ok := known[key]
		if !ok {
			continue
		}
		value, ok, err := parseOption(info, raw)
		if err != nil {
			return nil, fmt.Errorf("settings: option %s: %w", key, err)
		}
		if ok {
			values[key] = value
		}
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}

func parseOption(info policy.OptionInfo, raw json.RawMessage) (any, bool, error) {
	switch info.Kind {
	case policy.OptionBool:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false, nil
		}
		return value, true, nil
	case policy.OptionInt:
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false, nil
		}
		if value < info.Min || value > info.Max {
			return nil, false, nil
		}
		return value, true, nil
	case policy.OptionEnum:
		return parseChoice(info.Choices, raw)
	case policy.OptionList:
		var raws []json.RawMessage
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, false, nil
		}
		if len(raws) == 0 || len(info.Choices) == 0 {
			return nil, false, nil
		}
		list := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(info.Choices[0])), 0, len(raws))
		for _, item := range raws {
			value, ok, err := parseChoice(info.Choices, item)
			if err != nil || !ok {
				return nil, false, nil
			}
			list = reflect.Append(list, reflect.ValueOf(value))
		}
		return list.Interface(), true, nil
	}
	return nil, false, nil
}

func parseChoice(choices []any, raw json.RawMessage) (any, bool, error) {
	for _, choice := range choices {
		target := reflect.New(reflect.TypeOf(choice))
		if err := json.Unmarshal(raw, target.Interface()); err != nil {
			continue
		}
		if reflect.DeepEqual(choice, target.Elem().Interface()) {
			return choice, true, nil
		}
	}
	return nil, false, nil
}

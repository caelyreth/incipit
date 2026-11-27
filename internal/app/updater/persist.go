package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const stateFileName = "updater.json"

// Settings persists user update preferences and background-check state under
// the application's user data folder.
type Settings struct {
	AutomaticChecks    bool      `json:"automaticChecks"`
	AutomaticDownloads bool      `json:"automaticDownloads"`
	Channel            string    `json:"channel,omitempty"`
	SkippedVersion     string    `json:"skippedVersion,omitempty"`
	LastCheck          time.Time `json:"lastCheck,omitzero"`
	HighestVersion     string    `json:"highestVersion,omitempty"`
}

func defaultSettings() Settings {
	return Settings{
		AutomaticChecks: true,
	}
}

func loadSettings(dir string) Settings {
	def := defaultSettings()
	if dir == "" {
		return def
	}
	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return def
	}
	// Preserve missing booleans (e.g. legacy/partial files) with defaults.
	s := def
	if err := json.Unmarshal(data, &s); err != nil {
		return def
	}
	if s.Channel != "" && s.Channel != ChannelRelease && s.Channel != ChannelPrerelease {
		s.Channel = ""
	}
	return s
}

func saveSettings(dir string, s Settings) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, stateFileName+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, stateFileName)); err != nil {
		return err
	}
	cleanup = false
	return nil
}

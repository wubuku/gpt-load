package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

var keyModelOverrideMap map[string]string

func init() {
	loadKeyModelOverride()
}

func loadKeyModelOverride() {
	execPath, err := os.Executable()
	if err != nil {
		logrus.Debug("Could not determine executable path, skipping key model override")
		return
	}
	execDir := filepath.Dir(execPath)
	configPath := filepath.Join(execDir, "key-model-override.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.WithError(err).Warn("Failed to read key model override config")
		}
		return
	}

	if err := json.Unmarshal(data, &keyModelOverrideMap); err != nil {
		logrus.WithError(err).Warn("Failed to parse key model override config")
		return
	}

	logrus.Infof("Loaded %d key model override entries", len(keyModelOverrideMap))
}

// GetOverrideModel returns the override model for a given key prefix, or empty string if not found
func GetOverrideModel(keyValue string) string {
	for prefix, model := range keyModelOverrideMap {
		if strings.HasPrefix(keyValue, prefix) {
			return model
		}
	}
	return ""
}

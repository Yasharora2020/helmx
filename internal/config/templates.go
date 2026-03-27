package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// ErrTemplateExists is returned when adding a template with a name that already exists
	ErrTemplateExists = errors.New("template with this name already exists")

	// ErrTemplateNotFound is returned when a template is not found
	ErrTemplateNotFound = errors.New("template not found")
)

// SaveTemplate extracts overrides from userValues vs chartDefaults and writes to a file.
// Returns ErrTemplateExists if a template with the same name already exists for this chart.
func SaveTemplate(name, chartName, chartVersion, description, userValues, chartDefaults string) error {
	if _, err := LoadTemplate(chartName, name); err == nil {
		return ErrTemplateExists
	}

	overrides, err := ExtractOverrides(userValues, chartDefaults)
	if err != nil {
		return err
	}

	tf := TemplateFile{
		Name:         name,
		Chart:        chartName,
		ChartVersion: chartVersion,
		Description:  description,
		CreatedAt:    time.Now(),
		Values:       overrides,
	}

	dir, err := TemplatesDir()
	if err != nil {
		return err
	}
	chartDir := filepath.Join(dir, chartDirName(chartName))
	if err := os.MkdirAll(chartDir, ConfigDirPerm); err != nil {
		return err
	}

	data, err := yaml.Marshal(tf)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(chartDir, name+".yaml"), data, ConfigFilePerm)
}

// LoadTemplate reads a template file by chart name and template name.
// Returns ErrTemplateNotFound if the template does not exist.
func LoadTemplate(chartName, name string) (TemplateFile, error) {
	dir, err := TemplatesDir()
	if err != nil {
		return TemplateFile{}, err
	}
	path := filepath.Join(dir, chartDirName(chartName), name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateFile{}, ErrTemplateNotFound
		}
		return TemplateFile{}, err
	}
	var tf TemplateFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return TemplateFile{}, err
	}
	return tf, nil
}

// ListTemplates returns all templates saved for a specific chart.
func ListTemplates(chartName string) ([]TemplateFile, error) {
	dir, err := TemplatesDir()
	if err != nil {
		return nil, err
	}
	chartDir := filepath.Join(dir, chartDirName(chartName))
	if _, err := os.Stat(chartDir); os.IsNotExist(err) {
		return nil, nil
	}
	return listTemplatesFromDir(chartDir)
}

// ListAllTemplates returns all templates across all charts.
func ListAllTemplates() ([]TemplateFile, error) {
	dir, err := TemplatesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []TemplateFile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		templates, err := listTemplatesFromDir(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		all = append(all, templates...)
	}
	return all, nil
}

func listTemplatesFromDir(chartDir string) ([]TemplateFile, error) {
	entries, err := os.ReadDir(chartDir)
	if err != nil {
		return nil, err
	}
	var templates []TemplateFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(chartDir, e.Name()))
		if err != nil {
			continue
		}
		var tf TemplateFile
		if err := yaml.Unmarshal(data, &tf); err != nil {
			continue
		}
		templates = append(templates, tf)
	}
	return templates, nil
}

// DeleteTemplate removes a template file.
// Returns ErrTemplateNotFound if the template does not exist.
func DeleteTemplate(chartName, name string) error {
	dir, err := TemplatesDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, chartDirName(chartName), name+".yaml")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrTemplateNotFound
		}
		return err
	}
	return nil
}

// TemplateFile is the on-disk format for the new file-based template storage.
// Each template is stored as an individual YAML file under ~/.config/helmx/templates/<chart>/<name>.yaml.
// This replaces the InstallTemplate type that was previously embedded in config.yaml.
// Values stores only the user's overrides (not the full chart defaults).
type TemplateFile struct {
	Name         string                 `yaml:"name"`
	Chart        string                 `yaml:"chart"`
	ChartVersion string                 `yaml:"chartVersion"`
	Description  string                 `yaml:"description"`
	CreatedAt    time.Time              `yaml:"createdAt"`
	Values       map[string]interface{} `yaml:"values,omitempty"`
}

// TemplatesDir returns the path to the templates directory.
func TemplatesDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, ConfigDir, "templates"), nil
}

// chartDirName sanitizes a chart name for use as a directory name.
func chartDirName(chartName string) string {
	r := strings.NewReplacer("/", "-", ":", "-")
	return r.Replace(chartName)
}

// ExtractOverrides returns only the keys in userYAML that differ from defaultYAML.
// The result contains only the user's customizations, not the full chart values.
func ExtractOverrides(userYAML, defaultYAML string) (map[string]interface{}, error) {
	var userMap, defaultMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(userYAML), &userMap); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal([]byte(defaultYAML), &defaultMap); err != nil {
		return nil, err
	}
	return diffMaps(userMap, defaultMap), nil
}

// ApplyTemplate merges overrides onto chartDefaults and returns the full merged YAML.
// This produces the complete values that will be shown to the user in the install dialog.
func ApplyTemplate(overrides map[string]interface{}, chartDefaults string) (string, error) {
	if len(overrides) == 0 {
		return chartDefaults, nil
	}
	var defaultMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(chartDefaults), &defaultMap); err != nil {
		return "", err
	}
	merged := deepMerge(defaultMap, overrides)
	data, err := yaml.Marshal(merged)
	return string(data), err
}

// deepMerge recursively merges override onto base, returning a new map.
// Override values take precedence; base values are preserved when not overridden.
func deepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, overrideVal := range override {
		baseVal, exists := result[k]
		if !exists {
			result[k] = overrideVal
			continue
		}
		baseMap, baseIsMap := baseVal.(map[string]interface{})
		overrideMap, overrideIsMap := overrideVal.(map[string]interface{})
		if baseIsMap && overrideIsMap {
			result[k] = deepMerge(baseMap, overrideMap)
		} else {
			result[k] = overrideVal
		}
	}
	return result
}

// diffMaps returns only the key/value pairs in user that differ from defaults.
func diffMaps(user, defaults map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, userVal := range user {
		defaultVal, exists := defaults[k]
		if !exists {
			result[k] = userVal
			continue
		}
		userMap, userIsMap := userVal.(map[string]interface{})
		defaultMap, defaultIsMap := defaultVal.(map[string]interface{})
		if userIsMap && defaultIsMap {
			nested := diffMaps(userMap, defaultMap)
			if len(nested) > 0 {
				result[k] = nested
			}
		} else if !reflect.DeepEqual(userVal, defaultVal) {
			result[k] = userVal
		}
	}
	return result
}

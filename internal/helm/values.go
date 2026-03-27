package helm

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/strvals"
)

// ValuesComposer merges multiple value sources for Helm operations.
// Sources are merged in Helm's standard order: value files (left to right)
// → inline YAML → --set overrides.
type ValuesComposer struct {
	ValueFiles []string // Ordered file paths, merged left to right
	InlineYAML string   // YAML from the editor
	SetValues  []string // key=value pairs (e.g., "image.tag=v2")
}

// Compose merges all value sources in Helm's order and returns the final
// merged values map. Value files are merged left to right, then inline YAML
// is applied, then --set overrides take highest precedence.
func (vc *ValuesComposer) Compose() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 1. Merge value files in order (left to right)
	for _, path := range vc.ValueFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading values file %s: %w", path, err)
		}

		var fileVals map[string]interface{}
		if err := yaml.Unmarshal(data, &fileVals); err != nil {
			return nil, fmt.Errorf("parsing values file %s: %w", path, err)
		}

		result = mergeMaps(result, fileVals)
	}

	// 2. Merge inline YAML
	if strings.TrimSpace(vc.InlineYAML) != "" {
		var inlineVals map[string]interface{}
		if err := yaml.Unmarshal([]byte(vc.InlineYAML), &inlineVals); err != nil {
			return nil, fmt.Errorf("parsing inline YAML: %w", err)
		}

		result = mergeMaps(result, inlineVals)
	}

	// 3. Merge --set overrides (highest precedence)
	for _, sv := range vc.SetValues {
		setVals, err := strvals.ParseString(sv)
		if err != nil {
			return nil, fmt.Errorf("parsing --set value %q: %w", sv, err)
		}

		result = mergeMaps(result, setVals)
	}

	return result, nil
}

// ComposeToYAML calls Compose() then marshals the result to a YAML string.
// Useful for diff display.
func (vc *ValuesComposer) ComposeToYAML() (string, error) {
	vals, err := vc.Compose()
	if err != nil {
		return "", err
	}

	if len(vals) == 0 {
		return "", nil
	}

	out, err := yaml.Marshal(vals)
	if err != nil {
		return "", fmt.Errorf("marshaling composed values to YAML: %w", err)
	}

	return string(out), nil
}

// Summary returns a human-readable summary of the value sources,
// e.g. "2 files, 3 overrides" or "" if empty.
func (vc *ValuesComposer) Summary() string {
	var parts []string

	if len(vc.ValueFiles) > 0 {
		n := len(vc.ValueFiles)
		if n == 1 {
			parts = append(parts, "1 file")
		} else {
			parts = append(parts, fmt.Sprintf("%d files", n))
		}
	}

	if strings.TrimSpace(vc.InlineYAML) != "" {
		parts = append(parts, "inline values")
	}

	if len(vc.SetValues) > 0 {
		n := len(vc.SetValues)
		if n == 1 {
			parts = append(parts, "1 override")
		} else {
			parts = append(parts, fmt.Sprintf("%d overrides", n))
		}
	}

	return strings.Join(parts, ", ")
}

// IsEmpty returns true if no value sources are configured.
func (vc *ValuesComposer) IsEmpty() bool {
	return len(vc.ValueFiles) == 0 &&
		strings.TrimSpace(vc.InlineYAML) == "" &&
		len(vc.SetValues) == 0
}

// mergeMaps recursively merges override into base, returning a new map.
// When both values for a key are maps, they are merged recursively.
// Otherwise the override value wins. Neither input map is mutated.
func mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base))

	// Copy base into result
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides
	for k, overrideVal := range override {
		baseVal, exists := result[k]
		if !exists {
			result[k] = overrideVal
			continue
		}

		// If both are maps, merge recursively
		baseMap, baseIsMap := baseVal.(map[string]interface{})
		overrideMap, overrideIsMap := overrideVal.(map[string]interface{})
		if baseIsMap && overrideIsMap {
			result[k] = mergeMaps(baseMap, overrideMap)
			continue
		}

		// Otherwise override wins
		result[k] = overrideVal
	}

	return result
}

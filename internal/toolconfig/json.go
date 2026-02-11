package toolconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// jsonRead reads and parses a JSON file into a map. Returns empty map if file doesn't exist.
func jsonRead(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	// Handle empty files
	if len(strings.TrimSpace(string(data))) == 0 {
		return make(map[string]any), nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// jsonWrite writes a map to a JSON file with indentation.
func jsonWrite(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// jsonHasKey checks if a dotted key path exists in a JSON file.
func jsonHasKey(path string, dottedKey string) bool {
	m, err := jsonRead(path)
	if err != nil {
		return false
	}
	return navigateMap(m, strings.Split(dottedKey, ".")) != nil
}

// jsonSetNested sets a value at a dotted key path, creating intermediate objects as needed.
func jsonSetNested(path string, dottedKey string, value any) error {
	m, err := jsonRead(path)
	if err != nil {
		return err
	}

	keys := strings.Split(dottedKey, ".")
	setNestedValue(m, keys, value)

	return jsonWrite(path, m)
}

// jsonRemoveKey removes a key at a dotted path.
func jsonRemoveKey(path string, dottedKey string) error {
	m, err := jsonRead(path)
	if err != nil {
		return err
	}

	keys := strings.Split(dottedKey, ".")
	removeNestedKey(m, keys)

	return jsonWrite(path, m)
}

// jsonAddHook adds a Claude Code hook entry if not already present.
func jsonAddHook(path string, event string, command string) error {
	m, err := jsonRead(path)
	if err != nil {
		return err
	}

	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
		m["hooks"] = hooks
	}

	eventHooks, _ := hooks[event].([]any)

	// Check if hook already exists
	for _, h := range eventHooks {
		hookMap, ok := h.(map[string]any)
		if !ok {
			continue
		}
		hookList, _ := hookMap["hooks"].([]any)
		for _, hh := range hookList {
			hhMap, ok := hh.(map[string]any)
			if !ok {
				continue
			}
			if hhMap["command"] == command {
				return jsonWrite(path, m) // already exists
			}
		}
	}

	// Add new hook entry
	newEntry := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}
	eventHooks = append(eventHooks, newEntry)
	hooks[event] = eventHooks

	return jsonWrite(path, m)
}

// jsonRemoveHook removes a Claude Code hook entry by command.
func jsonRemoveHook(path string, event string, command string) error {
	m, err := jsonRead(path)
	if err != nil {
		return err
	}

	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	eventHooks, _ := hooks[event].([]any)
	var filtered []any
	for _, h := range eventHooks {
		hookMap, ok := h.(map[string]any)
		if !ok {
			filtered = append(filtered, h)
			continue
		}
		hookList, _ := hookMap["hooks"].([]any)
		var filteredHooks []any
		for _, hh := range hookList {
			hhMap, ok := hh.(map[string]any)
			if !ok {
				filteredHooks = append(filteredHooks, hh)
				continue
			}
			if hhMap["command"] != command {
				filteredHooks = append(filteredHooks, hh)
			}
		}
		if len(filteredHooks) > 0 {
			hookMap["hooks"] = filteredHooks
			filtered = append(filtered, hookMap)
		}
	}

	if len(filtered) > 0 {
		hooks[event] = filtered
	} else {
		delete(hooks, event)
	}

	if len(hooks) == 0 {
		delete(m, "hooks")
	}

	return jsonWrite(path, m)
}

func setNestedValue(m map[string]any, keys []string, value any) {
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}
	next, ok := m[keys[0]].(map[string]any)
	if !ok {
		next = make(map[string]any)
		m[keys[0]] = next
	}
	setNestedValue(next, keys[1:], value)
}

func removeNestedKey(m map[string]any, keys []string) {
	if len(keys) == 1 {
		delete(m, keys[0])
		return
	}
	next, ok := m[keys[0]].(map[string]any)
	if !ok {
		return
	}
	removeNestedKey(next, keys[1:])
}

func navigateMap(m map[string]any, keys []string) any {
	if len(keys) == 0 {
		return m
	}
	val, ok := m[keys[0]]
	if !ok {
		return nil
	}
	if len(keys) == 1 {
		return val
	}
	next, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return navigateMap(next, keys[1:])
}

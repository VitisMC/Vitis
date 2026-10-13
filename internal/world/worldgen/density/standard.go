package density

import "encoding/json"

// EnableStandardOverworld binds generated evaluators after the caller validates all definitions.
func (c *Compiler) EnableStandardOverworld() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	standard, err := newStandardDensity(c)
	if err != nil {
		return err
	}
	c.standard = standard
	return nil
}

// standardJSONKey normalizes compile-time expressions for generated root lookup.
func standardJSONKey(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

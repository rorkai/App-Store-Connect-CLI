package asc

import (
	"bytes"
	"encoding/json"
)

// UnmarshalJSON keeps the original attribute shape for sparse app responses.
func (a *AppAttributes) UnmarshalJSON(data []byte) error {
	type alias AppAttributes
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var original map[string]json.RawMessage
	if err := json.Unmarshal(data, &original); err != nil {
		return err
	}
	baseline, err := json.Marshal(decoded)
	if err != nil {
		return err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(baseline, &values); err != nil {
		return err
	}
	*a = AppAttributes(decoded)
	a.originalAttributes = original
	a.decodedAttributes = values
	return nil
}

// MarshalJSON preserves absent, null, and unknown API attributes while retaining
// caller edits and the existing shape of directly constructed AppAttributes.
func (a AppAttributes) MarshalJSON() ([]byte, error) {
	type alias AppAttributes
	encoded, err := json.Marshal(alias(a))
	if err != nil || a.decodedAttributes == nil {
		return encoded, err
	}
	var current map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &current); err != nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(a.originalAttributes))
	for name, value := range a.originalAttributes {
		result[name] = value
	}
	for name, value := range current {
		if !bytes.Equal(value, a.decodedAttributes[name]) {
			result[name] = value
		}
	}
	for name := range a.decodedAttributes {
		if _, present := current[name]; !present {
			delete(result, name)
		}
	}
	return json.Marshal(result)
}

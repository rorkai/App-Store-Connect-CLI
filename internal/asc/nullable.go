package asc

import "encoding/json"

// NullableString represents a string that may be explicitly null in JSON.
// Use a nil pointer to omit the field entirely.
type NullableString struct {
	Value *string
}

func (n NullableString) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}

func (n *NullableString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	return nil
}

// NullableBool represents a boolean that may be explicitly null in JSON.
// Use a nil pointer to omit the field entirely.
type NullableBool struct {
	Value *bool
}

func (n NullableBool) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}

func (n *NullableBool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var value bool
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	return nil
}

// NullablePlatform represents a platform that may be explicitly null in JSON.
// Use a nil pointer to omit the field entirely.
type NullablePlatform struct {
	Value *Platform
}

func (n NullablePlatform) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}

func (n *NullablePlatform) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var value Platform
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	return nil
}

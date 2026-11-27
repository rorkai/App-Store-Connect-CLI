package shared

import (
	"flag"
	"time"
)

// FlagMetadata describes parsing without exposing a flag's current value.
// Default is textual and appears only for explicitly declared static defaults.
type FlagMetadata struct {
	Type        string  `json:"type"`
	DefaultKind string  `json:"defaultKind"`
	Default     *string `json:"default,omitempty"`
}

// DescribeFlagValue reports known parsing metadata. A Getter is used only for
// its Go type; neither its value nor Flag.DefValue is serialized.
func DescribeFlagValue(value flag.Value) FlagMetadata {
	metadata := FlagMetadata{}
	if declared, ok := value.(interface{ commandFlagMetadata() FlagMetadata }); ok {
		metadata = declared.commandFlagMetadata()
	}
	if metadata.Type == "" {
		metadata.Type = "unknown"
		if getter, ok := value.(flag.Getter); ok {
			switch getter.Get().(type) {
			case string:
				metadata.Type = "string"
			case bool:
				metadata.Type = "boolean"
			case time.Duration:
				metadata.Type = "duration"
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				metadata.Type = "integer"
			case float32, float64:
				metadata.Type = "number"
			}
		} else if boolean, ok := value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			metadata.Type = "boolean"
		}
	}
	if metadata.DefaultKind == "" {
		metadata.DefaultKind = "unknown"
	}
	if metadata.DefaultKind != "static" {
		metadata.Default = nil
	}
	return metadata
}

func (*resourceIDValue) commandFlagMetadata() FlagMetadata {
	return FlagMetadata{Type: "string", DefaultKind: "static", Default: new(string)}
}

func (*OnceCSVValue) commandFlagMetadata() FlagMetadata {
	return FlagMetadata{Type: "string", DefaultKind: "static", Default: new(string)}
}

func (v *validatedOutputValue) commandFlagMetadata() FlagMetadata {
	return FlagMetadata{Type: "string", DefaultKind: v.defaultKind, Default: v.staticDefault}
}

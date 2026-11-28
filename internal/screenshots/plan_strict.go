package screenshots

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/tidwall/jsonc"
)

// LoadPlanStrict validates plan fields and action requirements without execution.
func LoadPlanStrict(path string) (*Plan, error) {
	root, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanRead, err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanRead, err)
	}
	data = jsonc.ToJSON(data)
	plan, err := decodePlan(data)
	if err != nil {
		return nil, err
	}
	var problems []error
	if err := validatePlanJSONFields(json.NewDecoder(bytes.NewReader(data)), reflect.TypeFor[Plan](), "", 0, &problems); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanParseJSON, err)
	}
	if err := validatePlan(plan); err != nil {
		problems = append(problems, err)
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	return plan, nil
}

func validatePlanJSONFields(decoder *json.Decoder, typ reflect.Type, path string, step int, problems *[]error) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch delimiter {
	case '{':
		fields := make(map[string]reflect.Type)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			fields[name] = field.Type
		}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name := key.(string)
			fieldPath := name
			if path != "" {
				fieldPath = path + "." + name
			}
			fieldType, known := fields[name]
			if !known {
				message := fmt.Sprintf("unknown field %q", fieldPath)
				if step > 0 {
					message += fmt.Sprintf(" (step %d)", step)
				}
				*problems = append(*problems, newPlanValidationError(PlanErrUnexpectedField, step, message))
				var value json.RawMessage
				if err := decoder.Decode(&value); err != nil {
					return err
				}
			} else if err := validatePlanJSONFields(decoder, fieldType, fieldPath, step, problems); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; decoder.More(); i++ {
			itemStep := step
			if typ.Elem() == reflect.TypeFor[PlanStep]() {
				itemStep = i + 1
			}
			if err := validatePlanJSONFields(decoder, typ.Elem(), fmt.Sprintf("%s[%d]", path, i), itemStep, problems); err != nil {
				return err
			}
		}
	}
	_, err = decoder.Token()
	return err
}

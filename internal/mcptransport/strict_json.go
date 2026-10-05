// pattern: Functional Core
package mcptransport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

var jsonRawMessageType = reflect.TypeOf(json.RawMessage{})

// rejectDuplicateJSONKeys rejects ambiguous JSON objects before decoding them
// into Go maps or structs, where duplicate keys otherwise silently overwrite
// earlier values.
func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var readValue func() error
	readValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			return nil
		}
		switch delimiter {
		case '{':
			keys := make(map[string]struct{})
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				key, ok := keyToken.(string)
				if keyErr != nil || !ok {
					return errors.New("JSON object key is invalid")
				}
				if _, exists := keys[key]; exists {
					return errors.New("JSON object contains a duplicate key")
				}
				keys[key] = struct{}{}
				if err = readValue(); err != nil {
					return err
				}
			}
			closing, closeErr := decoder.Token()
			if closeErr != nil || closing != json.Delim('}') {
				return errors.New("JSON object is incomplete")
			}
		case '[':
			for decoder.More() {
				if err = readValue(); err != nil {
					return err
				}
			}
			closing, closeErr := decoder.Token()
			if closeErr != nil || closing != json.Delim(']') {
				return errors.New("JSON array is incomplete")
			}
		default:
			return errors.New("JSON has an unexpected closing delimiter")
		}
		return nil
	}
	if err := readValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing data")
	}
	return nil
}

// ValidateStrictJSONStructKeys rejects duplicate JSON keys and keys that would
// case-fold onto a tagged field in target. It only follows statically typed
// struct fields; arbitrary maps and json.RawMessage values retain JSON's
// case-sensitive key semantics.
func ValidateStrictJSONStructKeys(raw []byte, target any) error {
	if target == nil {
		return errors.New("JSON target type is missing")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	return rejectCaseVariantStructKeys(raw, reflect.TypeOf(target))
}

// validateJSONRPCResponseOutcome requires exactly one of result or error to
// appear in a JSON-RPC response. A present error must also decode to a non-null
// typed error value; result:null remains a valid present result.
func validateJSONRPCResponseOutcome(raw []byte, decodedErrorPresent bool) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errors.New("JSON-RPC response envelope is invalid")
	}
	_, hasResult := fields["result"]
	_, hasError := fields["error"]
	if hasResult == hasError || hasError && !decodedErrorPresent {
		return errors.New("JSON-RPC response must contain exactly one non-null result or error")
	}
	return nil
}

func rejectCaseVariantStructKeys(raw []byte, targetType reflect.Type) error {
	for targetType != nil && targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == nil || targetType == jsonRawMessageType {
		return nil
	}
	switch targetType.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return nil // The normal typed decoder reports shape and syntax errors.
		}
		fieldTypes := make(map[string]reflect.Type, targetType.NumField())
		for index := 0; index < targetType.NumField(); index++ {
			field := targetType.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			fieldTypes[name] = field.Type
		}
		for name, value := range fields {
			fieldType, exact := fieldTypes[name]
			if !exact {
				for canonical := range fieldTypes {
					if strings.EqualFold(name, canonical) {
						return errors.New("JSON struct field name is not canonical")
					}
				}
				continue
			}
			if err := rejectCaseVariantStructKeys(value, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if targetType.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var elements []json.RawMessage
		if err := json.Unmarshal(raw, &elements); err != nil {
			return nil // The normal typed decoder reports shape and syntax errors.
		}
		for _, element := range elements {
			if err := rejectCaseVariantStructKeys(element, targetType.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

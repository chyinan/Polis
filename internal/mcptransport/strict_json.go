// pattern: Functional Core
package mcptransport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

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

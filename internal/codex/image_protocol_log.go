// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func redactImageInputMessage(message Message) Message {
	if message.Method != "turn/start" || len(message.Params) == 0 {
		return message
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(message.Params, &params); err != nil {
		message.Params = json.RawMessage(`"[redacted malformed turn input]"`)
		return message
	}
	var inputs []json.RawMessage
	if err := json.Unmarshal(params["input"], &inputs); err != nil {
		message.Params = json.RawMessage(`"[redacted malformed turn input]"`)
		return message
	}
	changed := false
	for index, raw := range inputs {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			inputs[index] = json.RawMessage(`"[redacted malformed turn input item]"`)
			changed = true
			continue
		}
		var kind string
		_ = json.Unmarshal(item["type"], &kind)
		field := ""
		switch kind {
		case "image":
			field = "url"
		case "localImage":
			field = "path"
		default:
			for _, candidate := range []string{"url", "image_url"} {
				var possibleURL string
				if json.Unmarshal(item[candidate], &possibleURL) == nil && strings.HasPrefix(possibleURL, "data:image/") {
					item[candidate], _ = json.Marshal("[redacted image attachment]")
					inputs[index], _ = json.Marshal(item)
					changed = true
					break
				}
			}
			continue
		}
		var source string
		if json.Unmarshal(item[field], &source) != nil {
			continue
		}
		payload := []byte(source)
		if kind == "image" && strings.HasPrefix(source, "data:image/") {
			if header, encoded, found := strings.Cut(source, ","); found && strings.HasSuffix(header, ";base64") {
				if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
					payload = decoded
				}
			}
		}
		digest := sha256.Sum256(payload)
		item[field], _ = json.Marshal("[redacted image attachment]")
		item["imagePayloadRedacted"], _ = json.Marshal(true)
		item["imagePayloadBytes"], _ = json.Marshal(len(payload))
		item["imagePayloadSha256"], _ = json.Marshal(hex.EncodeToString(digest[:]))
		inputs[index], _ = json.Marshal(item)
		changed = true
	}
	if changed {
		params["input"], _ = json.Marshal(inputs)
		message.Params, _ = json.Marshal(params)
	}
	return message
}

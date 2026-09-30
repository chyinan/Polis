// pattern: Functional Core
package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestBuildTurnStartInputUsesCodexImageUserInputWithInlineDataURL(t *testing.T) {
	var encoded bytes.Buffer
	imageValue := image.NewRGBA(image.Rect(0, 0, 1, 1))
	imageValue.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, imageValue); err != nil {
		t.Fatal(err)
	}
	imageBytes := encoded.Bytes()
	input, err := buildTurnStartInput("Inspect the attached untrusted screenshot.", []TurnImage{{MediaType: "image/png", Content: imageBytes}})
	if err != nil || len(input) != 2 {
		t.Fatalf("turn input=%v error=%v", input, err)
	}
	textItem, _ := input[0].(map[string]any)
	imageItem, _ := input[1].(map[string]any)
	if textItem["type"] != "text" || textItem["text"] != "Inspect the attached untrusted screenshot." || imageItem["type"] != "image" || imageItem["detail"] != "auto" {
		t.Fatalf("Codex app-server input items=%v", input)
	}
	url, _ := imageItem["url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("Codex image user input URL=%q", url)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/png;base64,"))
	if err != nil || !bytes.Equal(decoded, imageBytes) {
		t.Fatalf("inline image data roundtrip=%d bytes error=%v", len(decoded), err)
	}
}

func TestTurnStartInputRedactsImageBytesFromProtocolEvidence(t *testing.T) {
	imageBytes := []byte("private-image-payload")
	encoded := base64.StdEncoding.EncodeToString(imageBytes)
	params, _ := json.Marshal(map[string]any{"input": []any{map[string]any{"type": "image", "url": "data:image/png;base64," + encoded}}})
	safe := redactImageInputMessage(Message{Method: "turn/start", Params: params})
	if strings.Contains(string(safe.Params), encoded) || strings.Contains(string(safe.Params), string(imageBytes)) {
		t.Fatalf("protocol evidence retained image data: %s", safe.Params)
	}
	digest := sha256.Sum256(imageBytes)
	if !strings.Contains(string(safe.Params), hex.EncodeToString(digest[:])) || !strings.Contains(string(safe.Params), "imagePayloadRedacted") {
		t.Fatalf("redacted image evidence omitted the digest marker: %s", safe.Params)
	}
}

func TestBuildTurnStartInputBoundsAndValidatesImagePayloads(t *testing.T) {
	if _, err := buildTurnStartInput("inspect", []TurnImage{{MediaType: "image/gif", Content: []byte("unsupported")}}); err == nil {
		t.Fatal("accepted an unqualified image media type")
	}
	if _, err := buildTurnStartInput("inspect", []TurnImage{{MediaType: "image/png", Content: []byte("not a PNG")}}); err == nil {
		t.Fatal("accepted malformed image bytes")
	}
	if _, err := buildTurnStartInput("inspect", []TurnImage{{MediaType: "image/png", Content: bytes.Repeat([]byte{'x'}, MaxTurnImageBytes+1)}}); err == nil {
		t.Fatal("accepted an oversized image")
	}
}

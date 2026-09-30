// pattern: Functional Core
package kernel

import (
	"testing"

	"polis/internal/intake"
)

func TestVerifyProductTaskInputContextRejectsChangedPayloadFields(t *testing.T) {
	canonical := ProductTaskInputContext{
		ManifestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Payload: intake.ModelInputContext{
			ManifestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			PayloadDigest:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Inputs: []intake.ModelInputPayload{{
				Reference: intake.ModelInputManifestEntry{InputID: "input-1", Revision: 1, SourceKind: "upload", ContentDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
				MediaType: "text/plain", ByteSize: 4, ContentDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Text: "fact",
			}},
			PromptSection: "canonical prompt",
		},
	}

	if err := verifyProductTaskInputContext(canonical, canonical); err != nil {
		t.Fatalf("canonical Worker input context was rejected: %v", err)
	}

	for name, mutate := range map[string]func(*ProductTaskInputContext){
		"prompt section":  func(value *ProductTaskInputContext) { value.Payload.PromptSection = "forged prompt" },
		"input reference": func(value *ProductTaskInputContext) { value.Payload.Inputs[0].RelativePath = "other/path.txt" },
		"input text":      func(value *ProductTaskInputContext) { value.Payload.Inputs[0].Text = "forged" },
		"payload digest": func(value *ProductTaskInputContext) {
			value.Payload.PayloadDigest = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		},
		"manifest digest": func(value *ProductTaskInputContext) {
			value.ManifestDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		},
	} {
		t.Run(name, func(t *testing.T) {
			supplied := cloneProductTaskInputContext(canonical)
			mutate(&supplied)
			if err := verifyProductTaskInputContext(canonical, supplied); err == nil {
				t.Fatal("changed Worker input context was accepted")
			}
		})
	}
}

func cloneProductTaskInputContext(value ProductTaskInputContext) ProductTaskInputContext {
	value.Payload.Inputs = append([]intake.ModelInputPayload(nil), value.Payload.Inputs...)
	value.Payload.Excluded = append([]intake.ModelInputExclusion(nil), value.Payload.Excluded...)
	return value
}

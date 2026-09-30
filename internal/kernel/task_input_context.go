// pattern: Imperative Shell
package kernel

import (
	"context"

	"polis/internal/core"
	"polis/internal/intake"
)

type ProductTaskInputContext struct {
	ManifestDigest string
	Payload        intake.ModelInputContext
}

func (k *Kernel) ProductTaskInputContext(ctx context.Context, binding Binding) (ProductTaskInputContext, error) {
	if binding.task == "" || !core.ValidID(binding.task) {
		return ProductTaskInputContext{}, core.Malformed
	}
	handover, err := k.Handover(ctx, binding)
	if err != nil {
		return ProductTaskInputContext{}, err
	}
	if !handover.Task.IsProductProviderExecutable() || !handover.Task.HasValidProductValidationBinding() {
		return ProductTaskInputContext{}, core.Denied
	}
	manifest, err := k.TaskInputManifest(ctx, binding.scope, binding.task)
	if err != nil {
		return ProductTaskInputContext{}, err
	}
	if manifest.MissionID != handover.Task.Mission || manifest.TaskID != handover.Task.ID {
		return ProductTaskInputContext{}, core.Integrity
	}
	contents := make(map[string][]byte)
	loadedBytes := int64(0)
	directoryArchives := 0
	for _, reference := range manifest.Manifest.CandidateInputs {
		if !intake.IsInputArchiveSource(reference.SourceKind) && !intake.ProviderTextInputEligible(reference) && !intake.ProviderImageInputEligible(reference) {
			continue
		}
		if intake.IsInputArchiveSource(reference.SourceKind) {
			directoryArchives++
			if directoryArchives > intake.MaxModelInputDirectoryArchives {
				return ProductTaskInputContext{}, core.TooLarge
			}
		}
		if loadedBytes+reference.ByteSize > intake.MaxModelInputSourceBytes {
			return ProductTaskInputContext{}, core.TooLarge
		}
		content, readErr := readBlob(k.root, binding.scope.company, reference.ContentDigest)
		if readErr != nil {
			return ProductTaskInputContext{}, readErr
		}
		contents[reference.InputID] = content
		loadedBytes += int64(len(content))
	}
	payload, err := intake.PrepareModelInputContext(manifest.Manifest, manifest.Digest, contents)
	if err != nil {
		return ProductTaskInputContext{}, core.Integrity
	}
	return ProductTaskInputContext{ManifestDigest: manifest.Digest, Payload: payload}, nil
}

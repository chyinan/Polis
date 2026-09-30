// pattern: Functional Core

export type MissionInputState = 'uploading' | 'stored' | 'usable' | 'partial' | 'unsupported' | 'rejected';

export type MissionInputSourceKind = 'upload' | 'directory_snapshot' | 'zip_snapshot' | 'pdf_snapshot' | 'git_snapshot';

export function isInputArchiveSource(sourceKind: MissionInputSourceKind): boolean {
	return sourceKind === 'directory_snapshot' || sourceKind === 'zip_snapshot' || sourceKind === 'pdf_snapshot' || sourceKind === 'git_snapshot';
}

export type MissionInputView = Readonly<{
  companyId: string;
  inputId: string;
  missionId: string;
  requestId: string;
  revision: string;
  sourceKind: MissionInputSourceKind;
  displayName: string;
  mediaType: string;
  byteSize: string;
  contentDigest: string;
  state: MissionInputState;
  imageWidth: string;
  imageHeight: string;
  createdAt: string;
}>;

export type MissionInputCommandReceipt = Readonly<{
  companyId: string;
  inputId: string;
  missionId: string;
  revision: number;
  requestId: string;
  sourceKind: MissionInputSourceKind;
  displayName: string;
  mediaType: string;
  byteSize: number;
  contentDigest: string;
  state: MissionInputState;
}>;

export type ModelInputManifestEntry = Readonly<{
  inputId: string;
  revision: number;
  requestId: string;
  sourceKind: MissionInputSourceKind;
  displayName: string;
  mediaType: string;
  byteSize: number;
  contentDigest: string;
  state: MissionInputState;
}>;

export type ModelInputManifest = Readonly<{
  schemaVersion: 'polis-model-input-manifest@1';
  companyId: string;
  missionId: string;
  taskId: string;
  candidateInputs: ReadonlyArray<ModelInputManifestEntry>;
  excludedInputs: ReadonlyArray<ModelInputManifestEntry>;
  deliveryStatus: 'not_delivered';
}>;

export type ModelInputExclusion = Readonly<{
  inputId: string;
  relativePath: string;
  mediaType: string;
  byteSize: number;
  contentDigest: string;
  reason: 'representation_not_supported' | 'context_limit';
}>;

export type ModelInputDeliveryRef = Readonly<{
  inputId: string;
  relativePath: string;
  mediaType: string;
  byteSize: number;
  contentDigest: string;
}>;

export type TaskInputManifestView = Readonly<{
  companyId: string;
  missionId: string;
  taskId: string;
  manifestDigest: string;
  deliveryStatus: 'not_delivered' | 'sending' | 'provider_delivered' | 'local_context_loaded' | 'outcome_unknown' | 'not_sent' | 'not_required';
  payloadDigest: string | null;
  includedInputIds: ReadonlyArray<string>;
  includedInputPaths: ReadonlyArray<ModelInputDeliveryRef>;
  inputExclusions: ReadonlyArray<ModelInputExclusion>;
  manifest: ModelInputManifest;
  createdAt: string;
}>;

// pattern: Functional Core

import type {ModelInputDeliveryRef} from './mission-input';

export function countIncludedTaskInputSources(includedFiles: ReadonlyArray<ModelInputDeliveryRef>): number {
  return new Set(includedFiles.map(file => file.inputId)).size;
}

export function countIncludedTaskInputTextFiles(includedFiles: ReadonlyArray<ModelInputDeliveryRef>): number {
  return includedFiles.filter(file => file.mediaType.startsWith('text/') || file.mediaType === 'application/json').length;
}

export function countIncludedTaskInputImages(includedFiles: ReadonlyArray<ModelInputDeliveryRef>): number {
  return includedFiles.filter(file => file.mediaType === 'image/png' || file.mediaType === 'image/jpeg').length;
}

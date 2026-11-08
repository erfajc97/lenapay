import type { LocalUpload } from '@/shared/api/types';

const BY_EXTENSION: Record<string, string> = { pdf: 'application/pdf', jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png' };

/**
 * The content type of a picked file. Pickers sometimes report none (Android content URIs) or a
 * generic one; the API decides by magic bytes anyway, but the multipart part needs a type.
 */
export function mimeTypeFor(fileName: string, reported: string | null | undefined): string {
  if (reported && reported !== 'application/octet-stream') return reported;
  const extension = fileName.split('.').pop()?.toLowerCase() ?? '';
  return BY_EXTENSION[extension] ?? 'application/octet-stream';
}

/** A document-picker asset as an upload (name, local URI, content type). */
export function toUpload(asset: { name: string; uri: string; mimeType?: string | null }): LocalUpload {
  return { fileName: asset.name, fileUri: asset.uri, mimeType: mimeTypeFor(asset.name, asset.mimeType) };
}

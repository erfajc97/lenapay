import { describe, expect, it } from 'vitest';

import { mimeTypeFor, toUpload } from '../files';

// Why: a multipart part without a content type is refused before the API even reads the
// magic bytes, so an invoice picked on Android (no MIME from content URIs) would never upload.
describe('mimeTypeFor', () => {
  it('keeps a specific type the picker reported', () => {
    expect(mimeTypeFor('factura.pdf', 'application/pdf')).toBe('application/pdf');
    expect(mimeTypeFor('foto', 'image/heic')).toBe('image/heic');
  });

  it('derives the type from the extension when the picker gave none or a generic one', () => {
    expect(mimeTypeFor('factura.PDF', null)).toBe('application/pdf');
    expect(mimeTypeFor('dni.jpeg', undefined)).toBe('image/jpeg');
    expect(mimeTypeFor('dni.jpg', 'application/octet-stream')).toBe('image/jpeg');
    expect(mimeTypeFor('selfie.png', '')).toBe('image/png');
  });

  it('falls back to a generic type for an unknown extension', () => {
    expect(mimeTypeFor('notas.txt', null)).toBe('application/octet-stream');
    expect(mimeTypeFor('sin-extension', null)).toBe('application/octet-stream');
  });
});

describe('toUpload', () => {
  it('maps a picker asset to an upload', () => {
    expect(toUpload({ name: 'f.pdf', uri: 'file:///cache/f.pdf', mimeType: null })).toEqual({ fileName: 'f.pdf', fileUri: 'file:///cache/f.pdf', mimeType: 'application/pdf' });
  });
});

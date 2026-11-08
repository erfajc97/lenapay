import * as DocumentPicker from 'expo-document-picker';

import { isMockApi } from '@/shared/api';
import type { KycDocumentKind, LocalUpload } from '@/shared/api/types';
import { mimeTypeFor, toUpload } from '@/shared/helpers/files';

const MOCK_FILE_NAMES: Record<KycDocumentKind, string> = {
  bylaws: 'estatuto-social.pdf',
  afip_constancy: 'constancia-afip.pdf',
  dni_front: 'dni-frente.jpg',
  dni_back: 'dni-dorso.jpg',
  selfie: 'selfie.jpg',
  other: 'documento.pdf',
};

/**
 * Where a KYC file comes from. Mock mode fakes the file (the fake KYC provider never reads
 * it); live mode opens the system document picker (no permission needed). A selfie must be a
 * photo (JPEG or PNG): the API decides by magic bytes.
 */
export async function pickKycFile(kind: KycDocumentKind): Promise<LocalUpload | null> {
  if (isMockApi()) {
    const fileName = MOCK_FILE_NAMES[kind];
    return { fileName, fileUri: `mock://${kind}`, mimeType: mimeTypeFor(fileName, null) };
  }
  const types = kind === 'selfie' ? ['image/jpeg', 'image/png'] : ['application/pdf', 'image/jpeg', 'image/png'];
  const picked = await DocumentPicker.getDocumentAsync({ type: types, copyToCacheDirectory: true });
  const asset = picked.canceled ? undefined : picked.assets[0];
  return asset ? toUpload(asset) : null;
}

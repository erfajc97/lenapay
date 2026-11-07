import * as DocumentPicker from 'expo-document-picker';

import { isMockApi } from '@/shared/api';
import type { KycDocumentType } from '@/shared/api/types';

interface PickedFile {
  fileName: string;
  fileUri: string;
}

const MOCK_FILE_NAMES: Record<KycDocumentType, string> = {
  bylaws: 'estatuto-social.pdf',
  afip_constancy: 'constancia-afip.pdf',
  dni_front: 'dni-frente.jpg',
  dni_back: 'dni-dorso.jpg',
  selfie: 'selfie.jpg',
};

/**
 * Where a KYC file comes from. Mock mode fakes the file (the fake KYC provider never reads
 * it); live mode opens the system document picker (no permission needed).
 */
export async function pickKycFile(type: KycDocumentType): Promise<PickedFile | null> {
  if (isMockApi || type === 'selfie') return { fileName: MOCK_FILE_NAMES[type], fileUri: `mock://${type}` };
  const picked = await DocumentPicker.getDocumentAsync({ type: ['application/pdf', 'image/*'], copyToCacheDirectory: true });
  const asset = picked.canceled ? undefined : picked.assets[0];
  return asset ? { fileName: asset.name, fileUri: asset.uri } : null;
}

/** Decides which step-up path the sheet offers and what it shows (R-ID-3). */
export function stepUpMode(params: { biometrics: boolean; choosePassword: boolean; lastErrorCode: string | null }): 'biometric' | 'password' {
  if (!params.biometrics || params.choosePassword || params.lastErrorCode === 'stepup.device_key_unavailable') return 'password';
  return 'biometric';
}

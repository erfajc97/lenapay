/**
 * CBU/CVU helpers (BCRA 22-digit key). A CVU is a CBU whose entity code is `000`.
 * Block 1 (8 digits): entity(3) + branch(4) + check; block 2 (14 digits): account(13) + check.
 */
const BLOCK1_WEIGHTS = [7, 1, 3, 9, 7, 1, 3];
const BLOCK2_WEIGHTS = [3, 9, 7, 1, 3, 9, 7, 1, 3, 9, 7, 1, 3];

function checkDigit(digits: string, weights: number[]): number {
  const sum = weights.reduce((total, weight, index) => total + weight * Number(digits[index]), 0);
  return (10 - (sum % 10)) % 10;
}

export function isValidCbu(value: string): boolean {
  if (!/^\d{22}$/.test(value)) return false;
  const block1 = value.slice(0, 8);
  const block2 = value.slice(8);
  return (
    checkDigit(block1, BLOCK1_WEIGHTS) === Number(block1[7]) &&
    checkDigit(block2, BLOCK2_WEIGHTS) === Number(block2[13])
  );
}

/** Builds a key with valid check digits from entity+branch (7 digits) and an account number. */
export function buildCbu(entityBranch: string, accountNumber: string): string {
  const block1 = entityBranch.padStart(7, '0').slice(0, 7);
  const account = accountNumber.padStart(13, '0').slice(-13);
  return `${block1}${checkDigit(block1, BLOCK1_WEIGHTS)}${account}${checkDigit(account, BLOCK2_WEIGHTS)}`;
}

/** A CVU is a CBU with entity `000`: the PSP prefix fills the branch digits. */
export function buildCvu(pspPrefix: string, accountNumber: string): string {
  return buildCbu(`000${pspPrefix}`, accountNumber);
}

type DestinationKind = 'cbu' | 'cvu' | 'alias' | 'invalid' | 'incomplete';

/**
 * Classifies what the person typed in "Destino" before asking the API to resolve it: only
 * a valid key or a well-formed alias is ever sent.
 */
export function classifyDestination(raw: string): DestinationKind {
  const value = raw.trim();
  if (value === '') return 'incomplete';
  if (/^\d+$/.test(value)) {
    if (value.length < 22) return 'incomplete';
    if (!isValidCbu(value)) return 'invalid';
    return value.startsWith('000') ? 'cvu' : 'cbu';
  }
  // BCRA alias: 6–20 characters, letters, digits, dots and hyphens.
  return /^[a-zA-Z0-9.-]{6,20}$/.test(value) ? 'alias' : 'invalid';
}

/** `0000003 1 0000000000123 4` grouped for reading; never used as the value sent. */
export function groupCbu(value: string): string {
  return value.length === 22 ? `${value.slice(0, 8)} ${value.slice(8)}` : value;
}

/** Alias rule of the BCRA: 6–20 characters, letters, digits, dots and hyphens. */
export function aliasError(alias: string): 'validation.alias' | null {
  return classifyDestination(alias.trim()) === 'alias' ? null : 'validation.alias';
}

/** What a CVU QR encodes: the CVU itself, readable by any banking app's scanner as text. */
export function cvuQrPayload(cvu: { cvu: string }): string {
  return cvu.cvu;
}

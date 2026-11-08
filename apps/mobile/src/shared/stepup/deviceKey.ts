import * as ed from '@noble/ed25519';
import { sha512 } from '@noble/hashes/sha2.js';
import * as Crypto from 'expo-crypto';

/**
 * Ed25519 device key for step-up (R-ID-3, architecture §6). Pure JS (`@noble/ed25519`), so it
 * needs no native module: the secret comes from `expo-crypto` (ME4), the sync API gets its
 * SHA-512 from `@noble/hashes` (Hermes has no WebCrypto). Storage lives in the session module.
 */
ed.hashes.sha512 = sha512;

const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';

/** RFC 4648 §5 without padding: the contract's encoding for keys, nonces and signatures. */
export function toBase64Url(bytes: Uint8Array): string {
  let out = '';
  for (let index = 0; index < bytes.length; index += 3) {
    const chunk = ((bytes[index] ?? 0) << 16) | ((bytes[index + 1] ?? 0) << 8) | (bytes[index + 2] ?? 0);
    const chars = Math.min(4, Math.ceil(((bytes.length - index) * 8) / 6));
    for (let shift = 0; shift < chars; shift += 1) out += ALPHABET[(chunk >> (18 - shift * 6)) & 63];
  }
  return out;
}

export function fromBase64Url(text: string): Uint8Array {
  const clean = text.replace(/=+$/, '');
  if (!/^[A-Za-z0-9_-]*$/.test(clean) || clean.length % 4 === 1) throw new Error('invalid base64url');
  const bytes = new Uint8Array(Math.floor((clean.length * 6) / 8));
  let buffer = 0;
  let bits = 0;
  let position = 0;
  for (const char of clean) {
    buffer = (buffer << 6) | ALPHABET.indexOf(char);
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      bytes[position] = (buffer >> bits) & 255;
      position += 1;
    }
  }
  return bytes;
}

export interface DeviceKeyPair {
  /** 32-byte Ed25519 secret, base64url (kept only in the secure store). */
  secretKey: string;
  /** 32-byte public key, base64url (sent to POST /devices). */
  publicKey: string;
}

export function generateDeviceKey(): DeviceKeyPair {
  const secret = Crypto.getRandomValues(new Uint8Array(32));
  return { secretKey: toBase64Url(secret), publicKey: toBase64Url(ed.getPublicKey(secret)) };
}

/** Signs the challenge nonce exactly as the API verifies it: Ed25519 over the raw nonce bytes. */
export function signNonce(params: { secretKey: string; nonce: string }): string {
  return toBase64Url(ed.sign(fromBase64Url(params.nonce), fromBase64Url(params.secretKey)));
}

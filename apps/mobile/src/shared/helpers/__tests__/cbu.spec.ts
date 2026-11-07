import { describe, expect, it } from 'vitest';

import { buildCvu, classifyDestination, groupCbu, isValidCbu } from '../cbu';

// Why: a typo in a CBU must be caught on the phone; sending a payout to a key with a wrong
// check digit wastes a resolve call at best and pays a stranger at worst.
describe('CBU/CVU check digits', () => {
  it('accepts a real-format CBU and the CVUs it builds', () => {
    expect(isValidCbu('2850590940090418135201')).toBe(true);
    const cvu = buildCvu('0031', '1234567');
    expect(cvu).toHaveLength(22);
    expect(cvu.startsWith('000')).toBe(true);
    expect(isValidCbu(cvu)).toBe(true);
  });

  it('rejects a key with one digit changed in either block', () => {
    expect(isValidCbu('2850590940090418135202')).toBe(false);
    expect(isValidCbu('2850591940090418135201')).toBe(false);
    expect(isValidCbu('285059094009041813520')).toBe(false);
  });
});

describe('classifyDestination', () => {
  it('tells CVU, CBU, alias, incomplete and invalid apart', () => {
    expect(classifyDestination(buildCvu('0031', '42'))).toBe('cvu');
    expect(classifyDestination('2850590940090418135201')).toBe('cbu');
    expect(classifyDestination('torre.palermo.ars')).toBe('alias');
    expect(classifyDestination('')).toBe('incomplete');
    expect(classifyDestination('28505909')).toBe('incomplete');
    expect(classifyDestination('2850590940090418135202')).toBe('invalid');
    expect(classifyDestination('con espacios')).toBe('invalid');
  });

  it('bounds the alias length at 6 and 20 characters', () => {
    expect(classifyDestination('abcde')).toBe('invalid');
    expect(classifyDestination('abcdef')).toBe('alias');
    expect(classifyDestination('a'.repeat(20))).toBe('alias');
    expect(classifyDestination('a'.repeat(21))).toBe('invalid');
  });

  it('groups a key for reading only', () => {
    expect(groupCbu('2850590940090418135201')).toBe('28505909 40090418135201');
  });
});

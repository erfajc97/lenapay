import { describe, expect, it } from 'vitest';

import { erpStage, importPercent } from '../erpRules';

// Why: linking before KYC approval, or by a non-admin, imports a company's buyers into an
// org nobody verified (R-ERP-2).
describe('erpStage', () => {
  it('shows the candidate only to admins of approved orgs', () => {
    expect(erpStage({ kycApproved: true, isAdmin: true, linkStatus: 'not_linked' })).toBe('candidate');
    expect(erpStage({ kycApproved: false, isAdmin: true, linkStatus: 'not_linked' })).toBe('blocked_kyc');
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: undefined })).toBe('blocked_role');
  });

  it('shows progress and result whatever the role', () => {
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: 'importing' })).toBe('importing');
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: 'linked' })).toBe('linked');
  });
});

describe('importPercent', () => {
  it('rounds down and treats an empty import as done', () => {
    expect(importPercent({ projects: { done: 1, total: 3 }, payers: { done: 0, total: 0 } })).toBe(33);
    expect(importPercent({ projects: { done: 3, total: 3 }, payers: { done: 4, total: 4 } })).toBe(100);
    expect(importPercent({ projects: { done: 0, total: 0 }, payers: { done: 0, total: 0 } })).toBe(100);
  });
});

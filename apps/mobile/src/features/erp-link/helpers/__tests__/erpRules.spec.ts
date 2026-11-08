import { describe, expect, it } from 'vitest';

import { erpStage } from '../erpRules';

// Why: linking before KYC approval, or by a non-admin, imports a company's buyers into an
// org nobody verified (R-ERP-2).
describe('erpStage', () => {
  it('shows the candidate only to admins of approved orgs', () => {
    expect(erpStage({ kycApproved: true, isAdmin: true, linkStatus: 'not_linked' })).toBe('candidate');
    expect(erpStage({ kycApproved: false, isAdmin: true, linkStatus: 'not_linked' })).toBe('blocked_kyc');
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: undefined })).toBe('blocked_role');
  });

  it('shows the import state whatever the role', () => {
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: 'importing' })).toBe('importing');
    expect(erpStage({ kycApproved: true, isAdmin: false, linkStatus: 'linked' })).toBe('linked');
    expect(erpStage({ kycApproved: false, isAdmin: false, linkStatus: 'failed' })).toBe('failed');
  });
});

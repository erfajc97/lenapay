/**
 * Endpoint catalog (architecture §5). Services take URLs only from here (M1), so a contract
 * rename is a one-line change.
 */
const org = (orgId: string) => `/orgs/${encodeURIComponent(orgId)}`;
const id = (value: string) => encodeURIComponent(value);

export const endpoints = {
  auth: {
    signup: '/auth/signup',
    verifyEmail: '/auth/verify-email',
    login: '/auth/login',
    refresh: '/auth/refresh',
    logout: '/auth/logout',
    stepUpChallenge: '/auth/step-up/challenge',
    stepUpVerify: '/auth/step-up/verify',
  },
  me: '/me',
  pushTokens: '/push-tokens',
  orgs: {
    list: '/orgs',
    members: (orgId: string) => `${org(orgId)}/members`,
    invites: (orgId: string) => `${org(orgId)}/invites`,
  },
  kyc: {
    case: (orgId: string) => `${org(orgId)}/kyc`,
    documents: (orgId: string) => `${org(orgId)}/kyc/documents`,
    submit: (orgId: string) => `${org(orgId)}/kyc/submit`,
  },
  accounts: {
    list: (orgId: string) => `${org(orgId)}/accounts`,
    one: (orgId: string, accountId: string) => `${org(orgId)}/accounts/${id(accountId)}`,
    balances: (orgId: string) => `${org(orgId)}/balances`,
    internalTransfers: (orgId: string) => `${org(orgId)}/internal-transfers`,
  },
  payers: {
    list: (orgId: string) => `${org(orgId)}/payers`,
    one: (orgId: string, payerId: string) => `${org(orgId)}/payers/${id(payerId)}`,
    cvus: (orgId: string, payerId: string) => `${org(orgId)}/payers/${id(payerId)}/cvus`,
    cvu: (orgId: string, cvuId: string) => `${org(orgId)}/cvus/${id(cvuId)}`,
  },
  payins: {
    one: (orgId: string, payinId: string) => `${org(orgId)}/payins/${id(payinId)}`,
  },
  payouts: {
    quote: (orgId: string) => `${org(orgId)}/payouts/quote`,
    create: (orgId: string) => `${org(orgId)}/payouts`,
    one: (orgId: string, payoutId: string) => `${org(orgId)}/payouts/${id(payoutId)}`,
    approve: (orgId: string, payoutId: string) => `${org(orgId)}/payouts/${id(payoutId)}/approve`,
    reject: (orgId: string, payoutId: string) => `${org(orgId)}/payouts/${id(payoutId)}/reject`,
    cancel: (orgId: string, payoutId: string) => `${org(orgId)}/payouts/${id(payoutId)}/cancel`,
    proof: (orgId: string, payoutId: string) => `${org(orgId)}/payouts/${id(payoutId)}/proof.pdf`,
    approvals: (orgId: string) => `${org(orgId)}/approvals`,
    resolveDestination: (orgId: string) => `${org(orgId)}/destinations/resolve`,
  },
  movements: {
    list: (orgId: string) => `${org(orgId)}/movements`,
  },
  reconciliation: {
    runs: (orgId: string) => `${org(orgId)}/reconciliation/runs`,
  },
  erp: {
    candidate: (orgId: string) => `${org(orgId)}/erp-link/candidate`,
    link: (orgId: string) => `${org(orgId)}/erp-link`,
  },
  events: (orgId: string) => `${org(orgId)}/events`,
} as const;

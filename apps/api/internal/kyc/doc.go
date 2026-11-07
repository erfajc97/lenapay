// Package kyc runs onboarding (lane A, R-KYC-1…3): one case per org (company data and
// representatives, or a person's DNI), private document uploads, submission, the vendor check
// through the Provider port and the ops decision. On approval it records the org's status and
// calls the ApprovalHooks (accounts opens the main/personal account, payers queues personal
// CVUs) in the SAME transaction. It owns kyc_cases and kyc_documents.
package kyc

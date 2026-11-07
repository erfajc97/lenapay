// Package payers owns payers and their CVUs (lane C, R-CVU-1…3): a payer under a project
// account gets one CVU (and alias) per currency issued through the psp.Provider port; aliases
// can change, CVUs can be disabled, and the share payload (link + QR) is derived here. A person
// in personal mode gets one CVU per currency on KYC approval (queued, never called inside the
// approval transaction). payins resolves the receiving CVU through CVUByNumber.
package payers

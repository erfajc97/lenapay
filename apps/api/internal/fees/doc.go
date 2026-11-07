// Package fees computes Lenapay's fees from the org's fee plan (or the default plan) per
// currency (lane B, R-FEE-1): payin = gross × payin_bps, payout = fixed + amount × payout_bps.
// ONE function with ONE rounding (money.ApplyBps) serves the quote, the hold and the capture
// (MO1); the quote is stored with the payout so later plan changes never alter it (MO5).
// It owns fee_plans and fee_plan_rates.
package fees

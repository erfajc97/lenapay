import { printToFileAsync } from 'expo-print';

import { formatCuit } from '@/shared/helpers/cuit';
import { i18n } from '@/shared/i18n';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';

import type { LocalFile } from '../client';
import { ApiError } from '../errors';
import type { Payout } from '../types';

/** Mock stand-in for `GET /payouts/{id}/proof.pdf` (R-OUT-6): a PDF rendered on the device. */
function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => `&#${char.charCodeAt(0)};`);
}

export async function renderProofPdf(payout: Payout): Promise<LocalFile> {
  if (payout.status !== 'settled') throw new ApiError('payout.invalid_state', 409);
  const t = i18n.t.bind(i18n);
  const rows: [string, string][] = [
    [t('proof.amount'), formatMoney(payout.amount, payout.currency)],
    [t('proof.fee'), formatMoney(payout.fee, payout.currency)],
    [t('proof.to'), payout.destination.holderName],
    [t('proof.cuit'), formatCuit(payout.destination.holderCuit)],
    [t('proof.cbu'), payout.destination.cbu],
    [t('proof.bank'), payout.destination.bankName],
    [t('proof.from'), payout.accountName],
    [t('proof.concept'), payout.concept],
    [t('proof.date'), formatDateTime(payout.events.at(-1)?.at ?? payout.createdAt)],
    [t('proof.reference'), payout.id],
  ];
  const html = `<html><body style="font-family: Helvetica, Arial, sans-serif; padding: 32px;">
    <h1 style="font-size: 22px;">${escapeHtml(t('proof.title'))}</h1>
    <table style="width: 100%; border-collapse: collapse; font-size: 14px;">
      ${rows.map(([label, value]) => `<tr><td style="padding: 8px 0; opacity: 0.6;">${escapeHtml(label)}</td><td style="padding: 8px 0; text-align: right;">${escapeHtml(value)}</td></tr>`).join('')}
    </table>
    <p style="font-size: 11px; opacity: 0.6; margin-top: 32px;">${escapeHtml(t('proof.footer'))}</p>
  </body></html>`;
  const file = await printToFileAsync({ html });
  return { uri: file.uri, mimeType: 'application/pdf' };
}

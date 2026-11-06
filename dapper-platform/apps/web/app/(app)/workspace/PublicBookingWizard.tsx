'use client';

import { useEffect, useRef, useState } from 'react';
import BookingWizard, { type WizardAvailability, type WizardBooking, type WizardCustomer, type WizardRule, type WizardService, type WizardTaxes } from './BookingWizard';
import SquareCardField from './SquareCardField';

/* The customer's booking on the website. Talks only to the public booking API: no sign-in, prices and the deposit are worked out on the server. */

const SITE = process.env.NEXT_PUBLIC_DAPPER_SITE_SLUG || 'dapper-detailing-pros-peterborough';
const api = `/api/v1/public/booking/${SITE}`;

type Options = { timezone: string; depositPercent: number; phoneVerification: boolean; rules: WizardRule[]; services: (WizardService & { priceCents: number })[]; taxes: NonNullable<WizardTaxes> };

async function json<T>(r: Response): Promise<T> {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'Something went wrong. Please try again.');
  return body as T;
}
const post = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export default function PublicBookingWizard() {
  const [options, setOptions] = useState<Options | null>(null);
  const [error, setError] = useState('');
  // One key per booking attempt; a failed attempt gets a new one so a released time can be tried again.
  const key = useRef(crypto.randomUUID());
  const verified = useRef('');

  useEffect(() => {
    let live = true;
    fetch(`${api}/options`, { cache: 'no-store' }).then((r) => json<Options>(r)).then((o) => live && setOptions(o)).catch((e) => live && setError(e.message));
    return () => { live = false; };
  }, []);

  if (error) return <p role="alert" style={{ color: 'var(--error)' }}>{error}</p>;
  if (!options) return <p role="status" style={{ color: 'var(--text-muted)' }}>Loading…</p>;

  return (
    <BookingWizard
      timezone={options.timezone}
      depositPercent={options.depositPercent}
      loadOptions={async () => ({ rules: options.rules })}
      loadServices={async () => options.services}
      loadTaxes={async () => options.taxes}
      loadAvailability={async (rule, from, days) => json<WizardAvailability>(await fetch(`${api}/availability?resourceId=${rule.resourceId}&ruleId=${rule.id}&version=${rule.version}&from=${from}&days=${days}`, { cache: 'no-store' }))}
      phoneCheck={options.phoneVerification ? {
        start: async (phone) => { await json(await fetch(`${api}/verify/start`, post({ phone }))); },
        check: async (phone, code): Promise<WizardCustomer[]> => { const r = await json<{ token: string; customers: WizardCustomer[] }>(await fetch(`${api}/verify/check`, post({ phone, code }))); verified.current = r.token; return r.customers; },
      } : undefined}
      deposit={{ percent: options.depositPercent, renderCard: (register) => <SquareCardField infoUrl={`${api}/square`} register={register} /> }}
      doneNote={({ name, pack, when, deposit }) => ({
        title: 'You are booked',
        lines: [
          `${pack} for ${name} on ${when}.`,
          deposit === 'card' ? 'Your deposit was paid by card and we are holding your time.' : 'We are holding your time. Please pay the full amount on the day.',
          'A team member will contact you if anything about the booking needs to change.',
        ],
      })}
      submit={async (b: WizardBooking) => {
        const body = { requestKey: key.current, resourceId: b.rule.resourceId, ruleId: b.rule.id, ruleVersion: b.rule.version, start: b.startIso, addonIds: b.addonIds, name: b.customer.name, email: b.customer.email, phone: b.customer.phone, notes: b.notes, serviceAddress: b.serviceAddress, couponCode: b.couponCode, depositMethod: b.deposit?.method ?? 'cash', sourceId: b.deposit?.sourceId };
        try {
          await json(await fetch(`${api}/book`, post(body)));
          key.current = crypto.randomUUID();
        } catch (e) {
          key.current = crypto.randomUUID();
          throw e;
        }
      }}
    />
  );
}

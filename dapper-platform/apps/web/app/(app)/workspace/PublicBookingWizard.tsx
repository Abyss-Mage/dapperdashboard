'use client';

import { useEffect, useRef, useState } from 'react';
import BookingWizard, { type WizardAvailability, type WizardBooking, type WizardCustomer, type WizardRule, type WizardService, type WizardTaxes } from './BookingWizard';
import SquareCardField from './SquareCardField';
import { categories as siteCategories } from '../../(site)/content';

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
  // The website categories set in the dashboard (service id -> category); without any, the wizard falls back to its built-in groups.
  const [cats, setCats] = useState<Map<string, string> | null>(null);
  // A signed-in customer's saved details (silently skipped when nobody is signed in).
  // Where a Book now button on the website pointed: ?package=<service id> or ?category=<kind of vehicle>.
  const [preselect] = useState<{ serviceId?: string; category?: string } | undefined>(() => {
    if (typeof window === 'undefined') return undefined;
    const q = new URLSearchParams(window.location.search);
    const serviceId = q.get('package') || undefined, category = q.get('category') || undefined;
    return serviceId || category ? { serviceId, category } : undefined;
  });
  const [prefill, setPrefill] = useState<{ name: string; email: string; phone: string; addresses: { id: string; label: string; address: string; isDefault: boolean }[] } | undefined>();
  // One key per booking attempt; a failed attempt gets a new one so a released time can be tried again.
  const key = useRef(crypto.randomUUID());
  const verified = useRef('');

  useEffect(() => {
    let live = true;
    fetch(`${api}/options`, { cache: 'no-store' }).then((r) => json<Options>(r)).then((o) => live && setOptions(o)).catch((e) => live && setError(e.message));
    fetch(`/api/v1/public/sites/${SITE}`, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : Promise.reject())).then((site: { services: { serviceId: string; category: string }[] }) => { if (live) { const m = new Map(site.services.filter((s) => s.category).map((s) => [s.serviceId, s.category])); setCats(m.size ? m : null); } }).catch(() => undefined);
    (async () => {
      try {
        const list = await fetch('/api/v1/portal/accounts', { cache: 'no-store', credentials: 'same-origin' });
        if (!list.ok) return;
        const first = ((await list.json()).items ?? [])[0] as { customerId: string } | undefined;
        if (!first) return;
        const rec = await fetch(`/api/v1/portal/accounts/${encodeURIComponent(first.customerId)}/records`, { cache: 'no-store', credentials: 'same-origin' });
        if (!rec.ok) return;
        const r = await rec.json();
        if (live) setPrefill({ name: r.account.name ?? '', email: r.account.email ?? '', phone: r.account.phone ?? '', addresses: r.addresses ?? [] });
      } catch { /* not signed in: the form simply starts empty */ }
    })();
    return () => { live = false; };
  }, []);

  if (error) return <p role="alert" style={{ color: 'var(--error)' }}>{error}</p>;
  if (!options) return <p role="status" style={{ color: 'var(--text-muted)' }}>Loading…</p>;

  return (
    <BookingWizard
      audience="customer"
      prefill={prefill}
      preselect={preselect}
      categories={cats ? { list: siteCategories.map((c) => ({ key: c.key, title: c.title })), of: (id) => cats.get(id) ?? '' } : undefined}
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
          const res = await json<{ bookingId: string; invoiceNumber: string; totalCents: number; paidCents: number; serviceName: string }>(await fetch(`${api}/book`, post(body)));
          key.current = crypto.randomUUID();
          // The booking is made: hand its details to the thank-you page, which is where the ad tags report the purchase.
          try {
            sessionStorage.setItem('dapper:booked', JSON.stringify({ ...res, pack: b.rule.serviceName, start: b.localStart, name: b.customer.name, email: b.customer.email, phone: b.customer.phone, address: b.serviceAddress, notes: b.notes }));
          } catch { /* the thank-you page still opens, with fewer details */ }
          window.location.assign('/booking/thank-you?b=' + encodeURIComponent(res.bookingId));
        } catch (e) {
          key.current = crypto.randomUUID();
          throw e;
        }
      }}
    />
  );
}

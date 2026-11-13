'use client';

import { useEffect, useRef, useState } from 'react';
import BookingWizard, { type WizardAvailability, type WizardBooking, type WizardCustomer, type WizardRule, type WizardService, type WizardTaxes, type WizardUpsell } from './BookingWizard';

/* Connects the guided booking flow to the staff APIs. "employee" = the Workspace (the signed-in employee's own franchise);
   "master" = Master (the franchise picked at the top, reached through its organization and location). */

async function json<T>(r: Response): Promise<T> {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'Something went wrong. Please try again.'));
  return body as T;
}
const post = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
const digits = (s: string) => s.replace(/\D/g, '');

export type StaffBookingWizardProps =
  | { mode: 'employee'; timezone: string; onBooked: () => Promise<void> | void }
  | { mode: 'master'; base: string; timezone: string; onBooked: () => Promise<void> | void };

export default function StaffBookingWizard(props: StaffBookingWizardProps) {
  // One key per attempt so pressing Save twice, or a slow network retry, never books twice.
  const keys = useRef(new Map<string, string>());
  const keyFor = (payload: unknown) => { const k = JSON.stringify(payload); if (!keys.current.has(k)) keys.current.set(k, crypto.randomUUID()); return keys.current.get(k) as string; };
  const query = (rule: WizardRule, from: string, days: number) => `resourceId=${rule.resourceId}&ruleId=${rule.id}&version=${rule.version}&from=${from}&days=${days}`;

  const employee = props.mode === 'employee';
  const base = props.mode === 'master' ? props.base : '';
  // Master: the organization's switched-on upgrade offers (the Workspace books without them).
  const [upsells, setUpsells] = useState<WizardUpsell[]>([]);
  const orgBase = /^\/api\/v1\/organizations\/[^/]+/.exec(base)?.[0];
  useEffect(() => {
    if (!orgBase) return;
    let live = true;
    fetch(`${orgBase}/package-upsells`, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).then((b: { items?: (WizardUpsell & { active: boolean })[] } | null) => { if (live) setUpsells((b?.items ?? []).filter((u) => u.active).map((u) => ({ ...u, discountPercent: 0, discountCents: 0 }))) /* the upgrade discount is for online bookings (error404 #34) */ }).catch(() => undefined);
    return () => { live = false; };
  }, [orgBase]);

  return (
    <BookingWizard
      timezone={props.timezone}
      upsells={upsells}
      allowCoupon={!employee}
      checkCoupon={employee ? undefined : async (code, serviceId, subtotalCents) => json(await fetch(`${base}/bookings/coupon-check`, post({ code, serviceId, subtotalCents })))}
      loadOptions={async () => (employee ? json<{ rules: WizardRule[] }>(await fetch('/api/v1/employee/walk-in/options', { cache: 'no-store' })) : json<{ rules: WizardRule[] }>(await fetch(`${base}/bookings/reservation-options`, { cache: 'no-store' })))}
      loadServices={async () => (await json<{ items: WizardService[] }>(await fetch(employee ? '/api/v1/employee/services' : `${base}/services`, { cache: 'no-store' }))).items}
      loadTaxes={async (): Promise<WizardTaxes> => {
        const r = await fetch(employee ? '/api/v1/employee/walk-in/taxes' : `${base}/taxes`, { cache: 'no-store' });
        return r.ok ? ((await r.json()) as WizardTaxes) : null;
      }}
      allowPast={!employee}
      loadAvailability={async (rule, from, days) => json<WizardAvailability>(await fetch(employee ? `/api/v1/employee/walk-in/availability?${query(rule, from, days)}` : `${base}/bookings/availability?${query(rule, from, days)}&past=1`, { cache: 'no-store' }))}
      // error404 #100: find a returning customer by email (by phone only when no email is typed), in the dashboard and the workspace.
      findCustomers={async (query): Promise<WizardCustomer[]> => {
        if (employee) return (await json<{ items: WizardCustomer[] }>(await fetch(`/api/v1/employee/customers?q=${encodeURIComponent(query)}`, { cache: 'no-store' }))).items;
        const list = (await json<{ items: WizardCustomer[] }>(await fetch(`${base}/customers`, { cache: 'no-store' }))).items;
        const q = query.trim().toLowerCase(), d = digits(query);
        if (/[a-z@]/.test(q)) return list.filter((c) => (c.email ?? '').toLowerCase().includes(q)).sort((a, b) => Number((b.email ?? '').toLowerCase() === q) - Number((a.email ?? '').toLowerCase() === q)).slice(0, 5);
        return list.filter((c) => d.length >= 4 && digits(c.phone ?? '').includes(d)).slice(0, 5);
      }}
      submit={async (b: WizardBooking) => {
        if (employee) {
          const body = { name: b.customer.name, email: b.customer.email, phone: b.customer.phone, ruleId: b.rule.id, ruleVersion: b.rule.version, resourceId: b.rule.resourceId, localStart: b.localStart, ...(b.addonIds.length ? { addonIds: b.addonIds } : {}), notes: b.notes, serviceAddress: b.serviceAddress };
          await json(await fetch('/api/v1/employee/walk-in', post({ ...body, requestKey: keyFor(body) })));
        } else {
          let customerId = b.customer.id;
          if (!customerId) {
            // The email decides who the customer is (error404 #98); the phone is used only when no email is given.
            const list = (await json<{ items: WizardCustomer[] }>(await fetch(`${base}/customers`, { cache: 'no-store' }))).items;
            const email = b.customer.email.trim().toLowerCase(), phone = digits(b.customer.phone);
            customerId = list.find((c) => (email ? (c.email ?? '').toLowerCase() === email : phone.length >= 7 && digits(c.phone ?? '') === phone))?.id;
          }
          if (!customerId) {
            const made = await json<{ id: string }>(await fetch(`${base}/customers`, post({ name: b.customer.name, email: b.customer.email, phone: b.customer.phone })));
            customerId = made.id;
            // A new customer with an email gets a link to set up their account password (error404 #85); the booking does not wait for it.
            if (b.customer.email.trim()) void fetch(`${base}/customers/${made.id}/account-setup`, post({ onlyIfNew: true })).catch(() => undefined);
          }
          const body = { customerId, vehicleId: null, resourceId: b.rule.resourceId, ruleId: b.rule.id, ruleVersion: b.rule.version, start: b.startIso, channel: 'walk_in', notes: b.notes, serviceAddress: b.serviceAddress, ...(b.addonIds.length ? { addonIds: b.addonIds } : {}), ...(b.couponCode ? { couponCode: b.couponCode } : {}) };
          await json(await fetch(`${base}/bookings/reserve`, post({ ...body, requestKey: keyFor(body) })));
        }
        await props.onBooked();
      }}
    />
  );
}

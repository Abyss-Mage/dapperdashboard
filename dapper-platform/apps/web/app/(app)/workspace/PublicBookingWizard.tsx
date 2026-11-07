'use client';

import { useEffect, useRef, useState } from 'react';
import BookingWizard, { type WizardAvailability, type WizardBooking, type WizardCustomer, type WizardRule, type WizardService, type WizardTaxes } from './BookingWizard';
import SquareCardField from './SquareCardField';
import styles from './bookingWizard.module.css';
import { categories as siteCategories } from '../../(site)/content';

/* The customer's booking on the website. Talks only to the public booking API: no sign-in, prices and the deposit are worked out on the server. */

const MAIN_SITE = process.env.NEXT_PUBLIC_DAPPER_SITE_SLUG || 'dapper-detailing-pros-peterborough';
type Place = { slug: string; name: string; address: string };
// The main shop's address (the invoice details hold test text, so they are not used for it).
const SHOP_FALLBACK = '660 The Kingsway, Peterborough, ON K9J 7C8';

type Options = { timezone: string; phoneVerification: boolean; rules: WizardRule[]; services: (WizardService & { priceCents: number })[]; taxes: NonNullable<WizardTaxes> };

async function json<T>(r: Response): Promise<T> {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'Something went wrong. Please try again.');
  return body as T;
}
const post = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

function BookingFlow({ site, place, onBack }: { site: string; place: Place; onBack: () => void }) {
  const api = `/api/v1/public/booking/${site}`;
  const shopAddress = (place.name ? place.name + ', ' : '') + (site === MAIN_SITE ? SHOP_FALLBACK : place.address);
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
    fetch(`/api/v1/public/sites/${site}`, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : Promise.reject())).then((site: { services: { serviceId: string; category: string }[] }) => { if (live) { const m = new Map(site.services.filter((s) => s.category).map((s) => [s.serviceId, s.category])); setCats(m.size ? m : null); } }).catch(() => undefined);
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
  }, [api, site]);

  if (error) return <p role="alert" style={{ color: 'var(--error)' }}>{error}</p>;
  if (!options) return <p role="status" style={{ color: 'var(--text-muted)' }}>Loading…</p>;

  return (
    <BookingWizard
      audience="customer"
      prefill={prefill}
      extraSteps={1}
      onBack={onBack}
      shopAddress={shopAddress.replace(/,\s*$/, '')}
      checkCoupon={async (code, serviceId, subtotalCents) => json(await fetch(`${api}/coupon`, post({ code, serviceId, subtotalCents })))}
      preselect={preselect}
      categories={cats ? { list: siteCategories.map((c) => ({ key: c.key, title: c.title })), of: (id) => cats.get(id) ?? '' } : undefined}
      timezone={options.timezone}
      loadOptions={async () => ({ rules: options.rules })}
      loadServices={async () => options.services}
      loadTaxes={async () => options.taxes}
      loadAvailability={async (rule, from, days) => json<WizardAvailability>(await fetch(`${api}/availability?resourceId=${rule.resourceId}&ruleId=${rule.id}&version=${rule.version}&from=${from}&days=${days}`, { cache: 'no-store' }))}
      phoneCheck={options.phoneVerification ? {
        start: async (phone) => { await json(await fetch(`${api}/verify/start`, post({ phone }))); },
        check: async (phone, code): Promise<WizardCustomer[]> => { const r = await json<{ token: string; customers: WizardCustomer[] }>(await fetch(`${api}/verify/check`, post({ phone, code }))); verified.current = r.token; return r.customers; },
      } : undefined}
      deposit={{ renderCard: (register) => <SquareCardField infoUrl={`${api}/square`} register={register} /> }}
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
          const res = await json<{ bookingId: string; invoiceNumber: string; totalCents: number; paidCents: number; paymentPending: boolean; serviceName: string }>(await fetch(`${api}/book`, post(body)));
          key.current = crypto.randomUUID();
          // The booking is made: hand its details to the thank-you page, which is where the ad tags report the purchase.
          try {
            const saved = JSON.stringify({ ...res, pack: b.rule.serviceName, start: b.localStart, name: b.customer.name, email: b.customer.email, phone: b.customer.phone, address: b.serviceAddress, shop: shopAddress, notes: b.notes });
            sessionStorage.setItem('dapper:booked', saved);
            localStorage.setItem('dapper:booked', saved); // a second copy, in case the browser clears the first on the way to the next page
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

/* The first step: where to book. Each location has its own services, opening times and bookings, so everything after this belongs to the one chosen here. */
export default function PublicBookingWizard() {
  const [places, setPlaces] = useState<Place[] | null>(null);
  const [error, setError] = useState('');
  const [pick, setPick] = useState('');
  const [site, setSite] = useState('');
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/booking-locations', { cache: 'no-store' }).then((r) => json<{ items: Place[] }>(r))
      .then((b) => { if (!live) return; setPlaces(b.items); setPick(b.items[0]?.slug ?? ''); const q = new URLSearchParams(window.location.search), asked = q.get('location'); if (asked && b.items.some((x) => x.slug === asked)) { setPick(asked); setSite(asked); } else if ((q.get('package') || q.get('category')) && b.items[0]) setSite(b.items[0].slug); })
      .catch((e) => live && setError(e.message));
    return () => { live = false; };
  }, []);
  if (error) return <p role="alert" style={{ color: 'var(--error)' }}>{error}</p>;
  if (!places) return <p role="status" style={{ color: 'var(--text-muted)' }}>Loading…</p>;
  const chosen = places.find((x) => x.slug === site);
  if (chosen) return <BookingFlow key={site} site={site} place={chosen} onBack={() => setSite('')} />;
  return (
    <div className={styles.wiz}>
      <div className={styles.top}>
        <p className={styles.kicker}>Step 1 of 8 · Location</p>
        <div className={styles.bar} aria-hidden="true"><i className={styles.on} />{Array.from({ length: 7 }, (_, i) => <i key={i} />)}</div>
      </div>
      <div className={styles.layout}>
        <section className={styles.panel}>
          <h2 className={styles.title}>Where would you like to book?</h2>
          <div className={`${styles.choices} ${styles.two}`}>
            {places.map((x) => (
              <button key={x.slug} type="button" className={styles.choice} aria-pressed={pick === x.slug} onClick={() => setPick(x.slug)}>
                <span><strong>{x.name}</strong><small>{x.slug === MAIN_SITE ? SHOP_FALLBACK : x.address}</small></span>
              </button>
            ))}
          </div>
          <div className={styles.nav}>
            <span />
            <button type="button" className={styles.next} disabled={!pick} onClick={() => setSite(pick)}>Continue →</button>
          </div>
        </section>
      </div>
    </div>
  );
}

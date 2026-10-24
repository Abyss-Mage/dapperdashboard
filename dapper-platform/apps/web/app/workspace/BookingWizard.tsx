'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import styles from './bookingWizard.module.css';

/* One guided booking flow, one question at a time. Used by the Workspace (walk-ins) and Master (new booking).
   It only shows what exists in the catalogue: real prices, real durations and real free times. */

export type WizardRule = { id: string; resourceId: string; serviceId: string; serviceName: string; addonIds: string[]; addons: { serviceId: string; name: string }[]; availableAddons?: { serviceId: string; name: string }[]; version: number; durationMinutes: number };
export type WizardService = { id: string; name: string; kind: string; priceCents?: number | null; archived?: boolean };
export type WizardTaxes = { rates: { rateBasisPoints: number; active: boolean }[]; settings: { rounding: string | null } } | null;
export type WizardAvailability = { timezone: string; durationMinutes: number; days: { date: string; starts: string[] }[] };
export type WizardCustomer = { id?: string; name: string; email: string; phone: string };
export type WizardBooking = {
  customer: WizardCustomer; rule: WizardRule; startIso: string; localStart: string; addonIds: string[]; notes: string;
  deposit?: { method: 'card' | 'cash'; sourceId?: string };
};
/* Customer-facing extras: a phone check for returning customers, and a deposit step with a card form. */
export type WizardDeposit = {
  percent: number;
  // Draws the card form; call register(fn) with a function that turns the typed card into a single-use token (or null when not ready).
  renderCard: (register: (tokenize: (() => Promise<string>) | null) => void, amountCents: number) => React.ReactNode;
};
export type WizardPhoneCheck = { start: (phone: string) => Promise<void>; check: (phone: string, code: string) => Promise<WizardCustomer[]> };
export type WizardDone = { title: string; lines: string[] };
export type WizardProps = {
  loadOptions: () => Promise<{ rules: WizardRule[] }>;
  loadServices: () => Promise<WizardService[]>;
  loadTaxes: () => Promise<WizardTaxes>;
  loadAvailability: (rule: WizardRule, fromDay: string, days: number) => Promise<WizardAvailability>;
  submit: (booking: WizardBooking) => Promise<void>;
  findCustomers?: (phone: string) => Promise<WizardCustomer[]>;
  timezone: string;
  depositPercent?: number;
  phoneCheck?: WizardPhoneCheck;
  deposit?: WizardDeposit;
  /** Wording for the final screen; staff see a default that mentions taking payment later. */
  doneNote?: (b: { name: string; pack: string; when: string; deposit?: 'card' | 'cash' }) => WizardDone;
};

const VEHICLE = /^(hatchback|sedan|suv|pickup|van|2 wheeler|aircraft (?:small|medium|large)|boat (?:small|medium|large))\s*-\s*(.+)$/i;
function parseName(name: string) {
  const m = VEHICLE.exec(name.trim());
  if (!m) return { vehicle: '', pack: name.trim() };
  const v = m[1].toLowerCase();
  return { vehicle: v === 'suv' ? 'SUV' : v.replace(/\b\w/g, (c) => c.toUpperCase()), pack: m[2].trim() };
}
const GROUPS = ['Detailing', 'Ceramic coating', 'Paint protection film', 'Paint correction'] as const;
const GROUP_HELP: Record<string, string> = {
  Detailing: 'Washes, interior and full-body detailing', 'Ceramic coating': 'Long-lasting paint protection', 'Paint protection film': 'Clear film that guards the paint', 'Paint correction': 'Remove swirls and scratches',
};
function groupOf(pack: string) {
  if (/ppf/i.test(pack)) return 'Paint protection film';
  if (/ceramic/i.test(pack)) return 'Ceramic coating';
  if (/paint correction/i.test(pack)) return 'Paint correction';
  return 'Detailing';
}
function span(total: number) {
  if (total < 60) return `${total} min`;
  if (total < 1440) { const h = Math.floor(total / 60), r = total % 60; return `${h} hr${h === 1 ? '' : 's'}${r ? ` ${r} min` : ''}`; }
  const d = Math.floor(total / 1440), h = Math.floor((total % 1440) / 60);
  return `${d} day${d === 1 ? '' : 's'}${h ? ` ${h} hr${h === 1 ? '' : 's'}` : ''}`;
}
const money = (cents: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(cents / 100);
function taxOn(subtotal: number, taxes: WizardTaxes) {
  if (!taxes) return 0;
  return taxes.rates.filter((r) => r.active).reduce((sum, r) => {
    const num = subtotal * r.rateBasisPoints; let q = Math.floor(num / 10000); const rem = num % 10000;
    if (taxes.settings.rounding === 'down') q += 0; else if (taxes.settings.rounding === 'half_even') q += rem > 5000 || (rem === 5000 && q % 2 === 1) ? 1 : 0; else q += rem >= 5000 ? 1 : 0;
    return sum + q;
  }, 0);
}
// Clock time in the shop's own time zone for an exact moment.
function zoned(iso: string, tz: string) {
  const p = Object.fromEntries(new Intl.DateTimeFormat('en-CA', { timeZone: tz, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).formatToParts(new Date(iso)).map((x) => [x.type, x.value]));
  return { date: `${p.year}-${p.month}-${p.day}`, time: `${p.hour}:${p.minute}` };
}
const slotLabel = (iso: string, tz: string) => new Date(iso).toLocaleTimeString('en-CA', { timeZone: tz, hour: 'numeric', minute: '2-digit' });
const ymd = (y: number, m: number, d: number) => `${y}-${String(m + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;

type Where = 'shop' | 'mobile' | 'valet';

export default function BookingWizard(p: WizardProps) {
  const [rules, setRules] = useState<WizardRule[] | null>(null);
  const [services, setServices] = useState<Map<string, WizardService>>(new Map());
  const [taxes, setTaxes] = useState<WizardTaxes>(null);
  const [loadError, setLoadError] = useState('');
  const [step, setStep] = useState(0);
  const [payMethod, setPayMethod] = useState<'card' | 'cash'>('card');
  const tokenizer = useRef<(() => Promise<string>) | null>(null);
  const [cardReady, setCardReady] = useState(false);
  const [phoneIn, setPhoneIn] = useState('');
  const [codeSent, setCodeSent] = useState(false);
  const [code, setCode] = useState('');
  const [welcomeBusy, setWelcomeBusy] = useState(false);
  const [welcomeNote, setWelcomeNote] = useState('');
  const [returning, setReturning] = useState<WizardCustomer[] | null>(null);
  const [doneInfo, setDoneInfo] = useState<WizardDone | null>(null);
  const [group, setGroup] = useState('');
  const [where, setWhere] = useState<Where>('shop');
  const [address, setAddress] = useState('');
  const [vehicle, setVehicle] = useState('');
  const [car, setCar] = useState({ year: '', make: '', model: '' });
  const [ruleId, setRuleId] = useState('');
  const [picked, setPicked] = useState<string[]>([]);
  const today = new Date();
  const [month, setMonth] = useState({ y: today.getFullYear(), m: today.getMonth() });
  const [avail, setAvail] = useState<WizardAvailability | null>(null);
  const [availBusy, setAvailBusy] = useState(false);
  const [availError, setAvailError] = useState('');
  const [day, setDay] = useState('');
  const [start, setStart] = useState('');
  const [me, setMe] = useState<WizardCustomer>({ name: '', email: '', phone: '' });
  const [notes, setNotes] = useState('');
  const [referred, setReferred] = useState(false);
  const [ref, setRef] = useState({ name: '', phone: '' });
  const [found, setFound] = useState<WizardCustomer[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const topRef = useRef<HTMLHeadingElement>(null);
  const steps = [...(p.phoneCheck ? ['Welcome'] : []), 'Service', 'Where', 'Vehicle', 'Package', 'Extras', 'Day & time', 'Details', ...(p.deposit ? ['Deposit'] : [])];
  const name = steps[step];

  useEffect(() => {
    let live = true;
    Promise.all([p.loadOptions(), p.loadServices(), p.loadTaxes().catch(() => null)])
      .then(([o, s, t]) => { if (!live) return; setRules(o.rules); setServices(new Map(s.map((x) => [x.id, x]))); setTaxes(t); })
      .catch((e) => live && setLoadError(e instanceof Error ? e.message : 'Booking choices could not be loaded.'));
    return () => { live = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => { topRef.current?.focus(); }, [step, done]);

  const baseRules = useMemo(() => (rules ?? []).filter((r) => r.addonIds.length === 0), [rules]);
  const parsed = useMemo(() => new Map(baseRules.map((r) => [r.id, parseName(r.serviceName)])), [baseRules]);
  const groups = useMemo(() => GROUPS.filter((g) => baseRules.some((r) => groupOf(parsed.get(r.id)!.pack) === g)), [baseRules, parsed]);
  const inGroup = useMemo(() => baseRules.filter((r) => groupOf(parsed.get(r.id)!.pack) === group), [baseRules, parsed, group]);
  const vehicles = useMemo(() => [...new Set(inGroup.map((r) => parsed.get(r.id)!.vehicle).filter(Boolean))], [inGroup, parsed]);
  const packages = useMemo(() => inGroup.filter((r) => { const v = parsed.get(r.id)!.vehicle; return !v || !vehicle || v === vehicle; }), [inGroup, parsed, vehicle]);
  const rule = baseRules.find((r) => r.id === ruleId);
  const extras = rule?.availableAddons ?? [];
  const price = (id: string) => services.get(id)?.priceCents ?? null;
  const subtotal = (rule ? price(rule.serviceId) ?? 0 : 0) + picked.reduce((s, id) => s + (price(id) ?? 0), 0);
  const unpriced = !!rule && (price(rule.serviceId) === null || picked.some((id) => price(id) === null));
  const tax = taxOn(subtotal, taxes);
  const total = subtotal + tax;
  const pct = p.depositPercent ?? 30;
  const deposit = Math.round((total * pct) / 100);
  const tz = avail?.timezone || p.timezone;

  const monthDays = new Date(month.y, month.m + 1, 0).getDate();
  const loadAvail = useCallback(async () => {
    if (!rule) return;
    setAvailBusy(true); setAvailError('');
    try { setAvail(await p.loadAvailability(rule, ymd(month.y, month.m, 1), monthDays)); }
    catch (e) { setAvail(null); setAvailError(e instanceof Error ? e.message : 'Times could not be loaded.'); }
    finally { setAvailBusy(false); }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rule, month.y, month.m, monthDays]);
  useEffect(() => { if (name === 'Day & time') void loadAvail(); }, [name, loadAvail]);

  const starts = useMemo(() => new Map((avail?.days ?? []).map((d) => [d.date, d.starts])), [avail]);
  const daySlots = starts.get(day) ?? [];

  const addressNeeded = where !== 'shop';
  const detailsOk = me.name.trim().length >= 2 && me.phone.replace(/\D/g, '').length >= 7 && /.+@.+\..+/.test(me.email.trim()) && (!referred || (ref.name.trim().length >= 2 && ref.phone.replace(/\D/g, '').length >= 7));
  const stepOk = ({
    Welcome: true,
    Service: !!group,
    Where: !addressNeeded || address.trim().length >= 5,
    Vehicle: !vehicles.length || !!vehicle,
    Package: !!ruleId,
    Extras: true,
    'Day & time': !!start,
    Details: detailsOk,
    Deposit: payMethod === 'cash' || cardReady,
  } as Record<string, boolean>)[name];

  function go(n: number) { setError(''); setStep(n); }
  async function lookup() {
    if (!p.findCustomers) return;
    setError('');
    try { setFound(await p.findCustomers(me.phone)); } catch (e) { setError(e instanceof Error ? e.message : 'Search failed.'); }
  }
  async function save() {
    if (!rule) return;
    setBusy(true); setError('');
    const lines = [
      `Vehicle: ${[vehicle, [car.year, car.make, car.model].filter(Boolean).join(' ')].filter(Boolean).join(' · ') || 'not given'}`,
      `Service place: ${where === 'shop' ? 'In the shop' : where === 'mobile' ? `We go to the customer: ${address.trim()}` : `Valet pick-up and drop-off: ${address.trim()}`}`,
      ...(referred ? [`Referred by: ${ref.name.trim()} (${ref.phone.trim()})`] : []),
      ...(notes.trim() ? [`Notes: ${notes.trim()}`] : []),
    ];
    const z = zoned(start, tz);
    try {
      let deposit: WizardBooking['deposit'];
      if (p.deposit) {
        if (payMethod === 'card') {
          if (!tokenizer.current) throw new Error('The card form is not ready yet.');
          deposit = { method: 'card', sourceId: await tokenizer.current() };
        } else deposit = { method: 'cash' };
      }
      await p.submit({ customer: { ...me, name: me.name.trim(), email: me.email.trim(), phone: me.phone.trim() }, rule, startIso: start, localStart: `${z.date}T${z.time}`, addonIds: picked, notes: lines.join('\n'), deposit });
      const when = `${new Date(`${z.date}T12:00:00`).toLocaleDateString('en-CA', { weekday: 'long', month: 'long', day: 'numeric' })} at ${slotLabel(start, tz)}`;
      setDoneInfo(p.doneNote ? p.doneNote({ name: me.name.trim(), pack: parsed.get(rule.id)!.pack, when, deposit: deposit?.method }) : null);
      setDone(true);
    } catch (e) { setError(e instanceof Error ? e.message : 'The booking could not be saved.'); }
    finally { setBusy(false); }
  }
  function again() {
    setDone(false); setStep(0); setGroup(''); setWhere('shop'); setAddress(''); setVehicle(''); setCar({ year: '', make: '', model: '' }); setRuleId(''); setPicked([]); setDay(''); setStart(''); setMe({ name: '', email: '', phone: '' }); setNotes(''); setReferred(false); setRef({ name: '', phone: '' }); setFound(null); setDoneInfo(null); setPayMethod('card'); setCardReady(false); setCode(''); setCodeSent(false); setReturning(null);
  }

  async function sendCode() {
    if (!p.phoneCheck) return;
    setWelcomeBusy(true); setWelcomeNote('');
    try { await p.phoneCheck.start(phoneIn); setCodeSent(true); setWelcomeNote('We sent a code by text message. It is good for 10 minutes.'); }
    catch (e) { setWelcomeNote(e instanceof Error ? e.message : 'The code could not be sent.'); }
    finally { setWelcomeBusy(false); }
  }
  async function verifyCode() {
    if (!p.phoneCheck) return;
    setWelcomeBusy(true); setWelcomeNote('');
    try { const list = await p.phoneCheck.check(phoneIn, code); setReturning(list); if (!list.length) setWelcomeNote('We could not find saved details for that number. You can continue as a new customer.'); }
    catch (e) { setWelcomeNote(e instanceof Error ? e.message : 'That code did not work.'); }
    finally { setWelcomeBusy(false); }
  }

  if (loadError) return <p role="alert" className={styles.err}>{loadError}</p>;
  if (!rules) return <p role="status" className={styles.help}>Loading…</p>;
  if (!baseRules.length) return <section className={styles.panel}><h2 className={styles.title}>No services are ready to book</h2><p className={styles.help}>Set up a service, its time and an opening time first (Setup → Services, then Bookings → Opening times).</p></section>;

  if (done) {
    const z = zoned(start, tz);
    return (
      <section className={`${styles.panel} ${styles.done}`} aria-live="polite">
        <h2 ref={topRef} tabIndex={-1} className={styles.title}>{doneInfo?.title ?? 'Booking saved'}</h2>
        {doneInfo ? doneInfo.lines.map((l, i) => <p key={i} className={styles.help}>{l}</p>) : <>
          <p className={styles.help}>{parsed.get(rule!.id)!.pack} for {me.name} on {new Date(`${z.date}T12:00:00`).toLocaleDateString('en-CA', { weekday: 'long', month: 'long', day: 'numeric' })} at {slotLabel(start, tz)}.</p>
          <p className={styles.help}>The time is held. Nothing has been paid yet; take the deposit or payment from the booking&rsquo;s payment page.</p></>}
        <button type="button" className={styles.next} onClick={again}>Book another</button>
      </section>
    );
  }

  const firstDow = new Date(month.y, month.m, 1).getDay();
  const todayStr = ymd(today.getFullYear(), today.getMonth(), today.getDate());
  const isCurrentMonth = month.y === today.getFullYear() && month.m === today.getMonth();

  return (
    <div className={styles.wiz}>
      <div className={styles.top}>
        <p className={styles.kicker}>Step {step + 1} of {steps.length} · {name}</p>
        <div className={styles.bar} aria-hidden="true">{steps.map((_, i) => <i key={i} className={i <= step ? styles.on : ''} />)}</div>
      </div>
      <section className={styles.panel}>
        {name === 'Welcome' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Have you booked with us before?</h2>
          <p className={styles.help}>Enter your mobile number and we will text you a code. If we know you, your details fill in for you. New here? Skip this.</p>
          <div className={styles.fields}>
            <label className={styles.field}>Mobile number<input type="tel" value={phoneIn} onChange={(e) => setPhoneIn(e.target.value)} autoComplete="tel" placeholder="+1 705 555 0123" /></label>
            {codeSent && !returning && <label className={styles.field}>The code we texted you<input inputMode="numeric" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} /></label>}
          </div>
          {welcomeNote && <p className={styles.help} role="status">{welcomeNote}</p>}
          <div className={styles.nav}>
            {!codeSent ? <button type="button" className={styles.back} disabled={welcomeBusy || phoneIn.replace(/\D/g, '').length < 7} onClick={() => void sendCode()}>{welcomeBusy ? 'Sending…' : 'Text me a code'}</button>
              : !returning ? <button type="button" className={styles.back} disabled={welcomeBusy || code.length !== 6} onClick={() => void verifyCode()}>{welcomeBusy ? 'Checking…' : 'Check the code'}</button> : <span />}
          </div>
          {returning && returning.length > 0 && <div className={styles.choices}>{returning.map((c) => (
            <button key={c.id} type="button" className={styles.choice} onClick={() => { setMe({ id: c.id, name: c.name, email: c.email, phone: c.phone }); go(step + 1); }}><span><strong>Yes, I&rsquo;m {c.name}</strong><small>{c.email || 'No email saved'}</small></span></button>
          ))}</div>}
        </>}
        {name === 'Deposit' && p.deposit && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Hold your time with a deposit</h2>
          <div className={styles.sum} aria-label="Deposit summary">
            <div><span>Total, tax included</span><span>{money(total)}</span></div>
            <div className={styles.grand}><span>Deposit due now ({p.deposit.percent}%)</span><span>{money(deposit)}</span></div>
            <div><span>Balance on the day</span><span>{money(total - deposit)}</span></div>
          </div>
          <div className={`${styles.choices} ${styles.two}`}>
            <button type="button" className={styles.choice} aria-pressed={payMethod === 'card'} onClick={() => setPayMethod('card')}><span><strong>Pay the deposit by card</strong><small>Secure card form by Square</small></span></button>
            <button type="button" className={styles.choice} aria-pressed={payMethod === 'cash'} onClick={() => setPayMethod('cash')}><span><strong>Pay everything on the day</strong><small>Nothing is charged now</small></span></button>
          </div>
          {payMethod === 'card' && p.deposit.renderCard((fn) => { tokenizer.current = fn; setCardReady(!!fn); }, deposit)}
        </>}
        {name === 'Service' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>What can we do for them?</h2>
          <div className={styles.choices}>{groups.map((g) => (
            <button key={g} type="button" className={styles.choice} aria-pressed={group === g} onClick={() => { setGroup(g); setVehicle(''); setRuleId(''); setPicked([]); }}><span><strong>{g}</strong><small>{GROUP_HELP[g]}</small></span></button>
          ))}</div>
        </>}
        {name === 'Where' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Where will the work happen?</h2>
          <div className={`${styles.choices} ${styles.two}`}>
            {([['shop', 'In the shop', 'The customer brings the vehicle in'], ['mobile', 'We come to them', 'Our team travels to the customer'], ['valet', 'Valet', 'We pick up and drop off the vehicle']] as [Where, string, string][]).map(([id, t, h]) => (
              <button key={id} type="button" className={styles.choice} aria-pressed={where === id} onClick={() => setWhere(id)}><span><strong>{t}</strong><small>{h}</small></span></button>
            ))}
          </div>
          {addressNeeded && <label className={styles.field}>Address<input value={address} onChange={(e) => setAddress(e.target.value)} autoComplete="street-address" placeholder="Street, city" /></label>}
        </>}
        {name === 'Vehicle' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>What are we working on?</h2>
          {vehicles.length > 0 && <div className={`${styles.choices} ${styles.tiles}`}>{vehicles.map((v) => (
            <button key={v} type="button" className={`${styles.choice} ${styles.tile}`} aria-pressed={vehicle === v} onClick={() => { setVehicle(v); setRuleId(''); setPicked([]); }}><strong>{v}</strong></button>
          ))}</div>}
          <div className={styles.fields}>
            <label className={styles.field}>Year (optional)<input inputMode="numeric" maxLength={4} value={car.year} onChange={(e) => setCar({ ...car, year: e.target.value.replace(/\D/g, '') })} /></label>
            <label className={styles.field}>Make (optional)<input value={car.make} onChange={(e) => setCar({ ...car, make: e.target.value })} placeholder="e.g. BMW" /></label>
            <label className={styles.field}>Model (optional)<input value={car.model} onChange={(e) => setCar({ ...car, model: e.target.value })} placeholder="e.g. 330i" /></label>
          </div>
        </>}
        {name === 'Package' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Choose the package</h2>
          <div className={styles.choices}>{packages.map((r) => (
            <button key={r.id} type="button" className={styles.choice} aria-pressed={ruleId === r.id} onClick={() => { setRuleId(r.id); setPicked([]); setStart(''); setDay(''); }}>
              <span><strong>{parsed.get(r.id)!.pack}</strong><small>About {span(r.durationMinutes)}</small></span>
              <span className={styles.price}>{price(r.serviceId) === null ? 'Price not set' : money(price(r.serviceId) as number)}</span>
            </button>
          ))}</div>
          {!packages.length && <p className={styles.help}>No packages for this choice yet.</p>}
        </>}
        {name === 'Extras' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Anything extra?</h2>
          <p className={styles.help}>Optional. You can skip this.</p>
          <div className={styles.choices}>{extras.map((a) => (
            <label key={a.serviceId} className={styles.check}>
              <input type="checkbox" checked={picked.includes(a.serviceId)} onChange={(e) => setPicked((cur) => (e.target.checked ? [...cur, a.serviceId] : cur.filter((x) => x !== a.serviceId)))} />
              <span>{a.name}</span>
              <span className={styles.price}>{price(a.serviceId) === null ? '' : `+${money(price(a.serviceId) as number)}`}</span>
            </label>
          ))}</div>
          {!extras.length && <p className={styles.help}>No extras for this package.</p>}
        </>}
        {name === 'Day & time' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Pick a day and time</h2>
          <div className={styles.calHead}>
            <button type="button" disabled={isCurrentMonth} onClick={() => { setMonth((c) => (c.m === 0 ? { y: c.y - 1, m: 11 } : { y: c.y, m: c.m - 1 })); setDay(''); setStart(''); }} aria-label="Earlier month">‹</button>
            <strong>{new Date(month.y, month.m, 1).toLocaleDateString('en-CA', { month: 'long', year: 'numeric' })}</strong>
            <button type="button" onClick={() => { setMonth((c) => (c.m === 11 ? { y: c.y + 1, m: 0 } : { y: c.y, m: c.m + 1 })); setDay(''); setStart(''); }} aria-label="Later month">›</button>
          </div>
          {availError && <p role="alert" className={styles.err}>{availError}</p>}
          <div className={styles.cal} aria-busy={availBusy}>
            {['S', 'M', 'T', 'W', 'T', 'F', 'S'].map((d, i) => <span key={i} className={styles.dow}>{d}</span>)}
            {Array.from({ length: firstDow }, (_, i) => <span key={'b' + i} />)}
            {Array.from({ length: monthDays }, (_, i) => {
              const d = ymd(month.y, month.m, i + 1); const n = starts.get(d)?.length ?? 0;
              return <button key={d} type="button" disabled={n === 0} aria-pressed={day === d} className={`${styles.day} ${d === todayStr ? styles.today : ''} ${n > 0 && n <= 3 ? styles.few : ''}`} onClick={() => { setDay(d); setStart(''); }} aria-label={`${d}, ${n} times free`}>{i + 1}</button>;
            })}
          </div>
          <p className={styles.help}>Greyed days have no free time. A yellow dot means only a few times are left.</p>
          {day && <><strong>Times on {new Date(`${day}T12:00:00`).toLocaleDateString('en-CA', { weekday: 'long', month: 'long', day: 'numeric' })}</strong>
            <div className={styles.slots}>{daySlots.map((s) => <button key={s} type="button" className={styles.slot} aria-pressed={start === s} onClick={() => setStart(s)}>{slotLabel(s, tz)}</button>)}</div></>}
        </>}
        {name === 'Details' && <>
          <h2 ref={topRef} tabIndex={-1} className={styles.title}>Almost done: the customer&rsquo;s details</h2>
          <div className={styles.fields}>
            <label className={`${styles.field} ${styles.wide}`}>Full name<input value={me.name} onChange={(e) => setMe({ ...me, name: e.target.value })} autoComplete="name" /></label>
            <label className={styles.field}>Mobile number<input type="tel" value={me.phone} onChange={(e) => setMe({ ...me, phone: e.target.value })} autoComplete="tel" placeholder="+1 416 555 0123" /></label>
            <label className={styles.field}>Email<input type="email" value={me.email} onChange={(e) => setMe({ ...me, email: e.target.value })} autoComplete="email" /></label>
            {p.findCustomers && <div className={styles.wide}><button type="button" className={styles.back} disabled={me.phone.replace(/\D/g, '').length < 4} onClick={() => void lookup()}>Find an existing customer by phone</button>
              {found && (found.length ? <div className={styles.choices} style={{ marginTop: 8 }}>{found.map((c) => <button key={c.id} type="button" className={styles.choice} onClick={() => { setMe({ id: c.id, name: c.name, email: c.email, phone: c.phone }); setFound(null); }}><span><strong>{c.name}</strong><small>{c.email || 'No email'} · {c.phone}</small></span></button>)}</div> : <p className={styles.help}>No customer with that number.</p>)}</div>}
            <label className={`${styles.field} ${styles.wide}`}>Notes (optional)<textarea rows={3} maxLength={1000} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Anything we should know?" /></label>
          </div>
          <div><strong>Were they referred?</strong>
            <div className={styles.seg} style={{ marginTop: 8 }}><button type="button" aria-pressed={referred} onClick={() => setReferred(true)}>Yes</button><button type="button" aria-pressed={!referred} onClick={() => setReferred(false)}>No</button></div>
            {referred && <div className={styles.fields} style={{ marginTop: 10 }}>
              <label className={styles.field}>Who referred them?<input value={ref.name} onChange={(e) => setRef({ ...ref, name: e.target.value })} /></label>
              <label className={styles.field}>Their phone number<input type="tel" value={ref.phone} onChange={(e) => setRef({ ...ref, phone: e.target.value })} /></label>
            </div>}
          </div>
          {rule && <div className={styles.sum} aria-label="Price summary">
            <div><span>{parsed.get(rule.id)!.pack}</span><span>{price(rule.serviceId) === null ? 'Price not set' : money(price(rule.serviceId) as number)}</span></div>
            {picked.map((id) => <div key={id}><span>{extras.find((a) => a.serviceId === id)?.name}</span><span>{price(id) === null ? '' : money(price(id) as number)}</span></div>)}
            <div><span>Tax</span><span>{taxes ? money(tax) : 'Added on the invoice'}</span></div>
            <div className={styles.grand}><span>Total</span><span>{money(total)}</span></div>
            {total > 0 && <><div><span>Deposit due now ({pct}%)</span><span>{money(deposit)}</span></div><div><span>Balance on the day</span><span>{money(total - deposit)}</span></div></>}
            {unpriced && <p className={styles.help}>Some items have no price yet, so this total is not final.</p>}
          </div>}
        </>}
        {error && <p role="alert" className={styles.err}>{error}</p>}
        <div className={styles.nav}>
          {step > 0 ? <button type="button" className={styles.back} onClick={() => go(step - 1)} disabled={busy}>← Back</button> : <span />}
          {subtotalNote(name, rule, subtotal, total)}
          {step < steps.length - 1
            ? <button type="button" className={styles.next} disabled={!stepOk} onClick={() => go(step + 1)}>{name === 'Welcome' ? 'I’m new, let’s start' : name === 'Extras' && !picked.length ? 'Skip' : 'Continue'} →</button>
            : <button type="button" className={styles.next} disabled={!stepOk || busy} onClick={() => void save()}>{busy ? (p.deposit && payMethod === 'card' ? 'Taking deposit…' : 'Saving…') : p.deposit ? (payMethod === 'card' ? `Pay ${money(deposit)} and book` : 'Book it') : 'Save booking'}</button>}
        </div>
      </section>
    </div>
  );
}

function subtotalNote(name: string, rule: WizardRule | undefined, subtotal: number, total: number) {
  if (!rule || ['Welcome', 'Service', 'Where', 'Vehicle'].includes(name)) return <span />;
  return <span className={styles.total}>{name === 'Details' || name === 'Deposit' ? 'Total with tax' : 'Total so far'}<strong>{money(name === 'Details' || name === 'Deposit' ? total : subtotal)}</strong></span>;
}

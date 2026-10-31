'use client';
import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { ArrowRight, CalendarCheck, Car, Clock, Home, Mail, MapPin, Phone, Play, Send, Truck } from 'lucide-react';
import { contact } from '../dapper';
import { Chip, SectionHead } from './blocks';

/* The remaining home-page sections, in the same family as Delta's (booking band, online booking, live availability, mobile
   detailing, a film showcase, a contact form), written in Dapper's own words and backed by Dapper's real systems:
   the availability calendars read the public booking API; the contact form saves the enquiry to the dashboard inbox (an email draft is the fallback). Nothing here invents a claim, price or result. */

// "Ready for precise detailing?" — the short booking band (the current site's own closing line).
export function ReadyBand() {
  return (
    <section className="d-band" data-d-in>
      <div>
        <h2>Ready for precise detailing?</h2>
        <p>Choose mobile service or at-shop detailing and pick your date and time in seconds.</p>
      </div>
      <div className="d-hero__actions">
        <a className="d-btn d-btn--red" href="/booking">Book now<ArrowRight size={18} /></a>
        <a className="d-btn d-btn--dark" href={'tel:' + contact.tel}><Phone size={16} />{contact.phone}</a>
      </div>
    </section>
  );
}

type Options = { timezone: string; resources: { id: string; name: string }[]; rules: { id: string; resourceId: string; version: number; durationMinutes: number; serviceName: string; addonIds: string[] | null }[] };
type Avail = { timezone: string; durationMinutes: number; days: { date: string; starts: string[] }[] };
type Calendar = { name: string; service: string; days: Avail['days']; timezone: string };

const iso = (d: Date) => d.toISOString().slice(0, 10);

// Live availability for each booking resource (one calendar each), from the public booking API.
function useAvailability() {
  const [state, setState] = useState<{ status: 'loading' | 'ready' | 'off'; calendars: Calendar[] }>({ status: 'loading', calendars: [] });
  useEffect(() => {
    let live = true;
    (async () => {
      try {
        const r = await fetch('/api/v1/public/booking/dapper/options', { cache: 'no-store' });
        if (!r.ok) throw Error();
        const o: Options = await r.json();
        const from = iso(new Date());
        const calendars: Calendar[] = [];
        for (const res of o.resources.slice(0, 2)) {
          const rule = o.rules.find((x) => x.resourceId === res.id && !(x.addonIds && x.addonIds.length));
          if (!rule) continue;
          const q = new URLSearchParams({ resourceId: res.id, ruleId: rule.id, version: String(rule.version), from, days: '7' });
          const a = await fetch('/api/v1/public/booking/dapper/availability?' + q, { cache: 'no-store' });
          if (!a.ok) continue;
          const av: Avail = await a.json();
          calendars.push({ name: res.name, service: rule.serviceName, days: av.days, timezone: av.timezone || o.timezone });
        }
        if (live) setState({ status: calendars.length ? 'ready' : 'off', calendars });
      } catch { if (live) setState({ status: 'off', calendars: [] }); }
    })();
    return () => { live = false; };
  }, []);
  return state;
}

function CalendarCard({ c }: { c: Calendar }) {
  const day = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('en-CA', { weekday: 'short', day: 'numeric', timeZone: c.timezone });
  const time = (t: string) => new Date(t).toLocaleTimeString('en-CA', { hour: 'numeric', minute: '2-digit', timeZone: c.timezone });
  return (
    <article className="d-cal" data-d-in>
      <header><span className="d-icon"><CalendarCheck size={20} /></span><div><h3>{c.name}</h3><p>{c.service}</p></div><span className="d-live"><i />Live</span></header>
      <ol>
        {c.days.map((d) => (
          <li key={d.date} className={d.starts.length ? undefined : 'is-full'}>
            <b>{day(d.date)}</b>
            {d.starts.length ? <span>{d.starts.slice(0, 2).map(time).join(' · ')}{d.starts.length > 2 ? ` +${d.starts.length - 2}` : ''}</span> : <span>Fully booked</span>}
          </li>
        ))}
      </ol>
      <a className="d-btn d-btn--red d-btn--sm" href="/booking">Pick a time<ArrowRight size={16} /></a>
    </article>
  );
}

// Online booking: how it works, beside the live calendars.
export function BookOnline() {
  const av = useAvailability();
  return (
    <section className="d-section d-book" id="book">
      <div className="d-book__text">
        <SectionHead chip="Book online" title="Your detail, booked in minutes" text="An easy online booking system: choose mobile service or at-shop detailing, pick your date and time in seconds, and get instant confirmation." />
        <ol className="d-book__steps">
          <li data-d-in><b>1</b><span><strong>Choose your service</strong>Pick your vehicle and the care it needs.</span></li>
          <li data-d-in><b>2</b><span><strong>Pick a time</strong>At the shop, or at your place with mobile service.</span></li>
          <li data-d-in><b>3</b><span><strong>You’re confirmed</strong>Our pros arrive on time, fully equipped.</span></li>
        </ol>
        <a className="d-btn d-btn--red" href="/booking" data-d-in>Book now<ArrowRight size={18} /></a>
      </div>
      <div className="d-book__cals" aria-live="polite">
        {av.status === 'loading' && <div className="d-cal d-cal--empty"><Clock size={22} /><p>Checking live availability…</p></div>}
        {av.status === 'ready' && av.calendars.map((c) => <CalendarCard key={c.name} c={c} />)}
        {av.status === 'off' && (
          <div className="d-cal d-cal--empty">
            <CalendarCheck size={26} />
            <h3>See open times when you book</h3>
            <p>Live availability appears here once online booking is switched on. Until then, book online or call and we’ll find you a time.</p>
            <div className="d-hero__actions"><a className="d-btn d-btn--red d-btn--sm" href="/booking">Book now</a><a className="d-btn d-btn--dark d-btn--sm" href={'tel:' + contact.tel}>{contact.phone}</a></div>
          </div>
        )}
      </div>
    </section>
  );
}

// Mobile detailing at the customer's door.
export function MobileDetailing() {
  return (
    <section className="d-panel d-mobile">
      <div className="d-mobile__media" data-d-in><img src="/site/assets/portfolio/portfolio-08.jpg" alt="A Dapper detailer washing a GMC Hummer EV in a customer's driveway" loading="lazy" /><span className="d-mobile__tag"><Truck size={16} />Mobile service</span></div>
      <div className="d-mobile__text">
        <Chip>Mobile detailing</Chip>
        <h2 data-d-head>We’ll come to you — no waiting rooms</h2>
        <p data-d-in>Book mobile service and our pros show up on time at your location, fully equipped and ready to go. Prefer the studio? We’ll welcome you at 660 The Kingsway.</p>
        <ul className="d-mobile__list">
          <li data-d-in><Home size={18} />At your home or workplace</li>
          <li data-d-in><Car size={18} />Pick-up, drop-off and shuttle options</li>
          <li data-d-in><MapPin size={18} />Based in Peterborough — ask us about your area</li>
        </ul>
        <div className="d-hero__actions" data-d-in><a className="d-btn d-btn--red" href="/booking">Book mobile detailing<ArrowRight size={18} /></a></div>
      </div>
    </section>
  );
}

// A large film of the finish, with the studio's channels.
export function GlossFilm() {
  const [playing, setPlaying] = useState(true);
  return (
    <section className="d-section d-gloss">
      <SectionHead chip="Watch the finish" title="The finish, in motion" text="Real work from the Dapper studio. Follow along for more transformations." center />
      <div className="d-gloss__stage" data-d-in>
        <video src="/site/assets/transformations/transformation-890.webm" muted loop playsInline autoPlay preload="metadata"
          ref={(v) => { if (v) { if (playing) v.play().catch(() => {}); else v.pause(); } }} />
        <button type="button" className="d-gloss__toggle" onClick={() => setPlaying((p) => !p)} aria-pressed={!playing}>{playing ? 'Pause' : <><Play size={16} fill="currentColor" />Play</>}</button>
        <div className="d-gloss__side">
          <video src="/site/assets/transformations/transformation-500.webm" muted loop playsInline autoPlay preload="metadata" />
          <video src="/site/assets/transformations/transformation-676.webm" muted loop playsInline autoPlay preload="metadata" />
        </div>
      </div>
      <div className="d-center d-hero__actions">
        <a className="d-btn d-btn--dark" href={contact.instagram} target="_blank" rel="noopener noreferrer">Follow on Instagram</a>
        <a className="d-btn d-btn--dark" href={contact.youtube} target="_blank" rel="noopener noreferrer">Watch on YouTube</a>
      </div>
    </section>
  );
}

// Contact form: saves the enquiry to the dashboard inbox; if that cannot be reached it falls back to an email in the visitor's own mail app.
export function ContactForm() {
  const [state, setState] = useState<'idle' | 'sending' | 'saved' | 'mail'>('idle');
  const [error, setError] = useState('');
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const f = new FormData(form);
    const field = (k: string) => String(f.get(k) ?? '').trim();
    setState('sending'); setError('');
    try {
      const r = await fetch('/api/v1/public/enquiries', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: field('name'), email: field('email'), phone: field('phone'), vehicle: field('vehicle'), service: field('service'), message: field('message'), website: field('website') }) });
      if (r.status === 422 || r.status === 429) { setError((await r.json().catch(() => ({}))).error ?? 'Your message could not be sent. Please call us.'); setState('idle'); return; }
      if (!r.ok) throw new Error('unavailable');
      form.reset();
      setState('saved');
    } catch {
      const body = `Name: ${field('name')}\nPhone: ${field('phone')}\nEmail: ${field('email')}\nVehicle: ${field('vehicle')}\nService: ${field('service')}\n\n${field('message')}`;
      location.href = 'mailto:' + contact.email + '?subject=' + encodeURIComponent('Website enquiry: ' + field('service')) + '&body=' + encodeURIComponent(body);
      setState('mail');
    }
  };
  return (
    <section className="d-section d-contact-form" id="contact">
      <div className="d-contact-form__info">
        <SectionHead chip="Contact us" title="Get in touch with the team" text="Tell us about your vehicle and the care you have in mind. We’ll help you choose the next step." />
        <ul>
          <li data-d-in><Phone size={18} /><a href={'tel:' + contact.tel}>{contact.phone}</a></li>
          <li data-d-in><Mail size={18} /><a href={'mailto:' + contact.email}>{contact.email}</a></li>
          <li data-d-in><MapPin size={18} /><a href={contact.directions} target="_blank" rel="noopener noreferrer">{contact.street}, {contact.city}</a></li>
        </ul>
      </div>
      <form className="d-form" onSubmit={(e) => void submit(e)} data-d-in style={{ position: 'relative' }}>
        <label>Name<input name="name" required autoComplete="name" /></label>
        <label>Phone<input name="phone" type="tel" autoComplete="tel" inputMode="tel" /></label>
        <label className="d-form__wide">Email<input name="email" type="email" required autoComplete="email" /></label>
        <label>Vehicle<input name="vehicle" placeholder="e.g. 2021 Honda CR-V" /></label>
        <label>Service
          <select name="service" defaultValue="Detailing">
            <option>Detailing</option><option>Ceramic coating</option><option>Paint protection film</option><option>Window tint</option><option>Mobile detailing</option><option>Something else</option>
          </select>
        </label>
        <label className="d-form__wide">Message<textarea name="message" rows={4} maxLength={3000} placeholder="Condition, timing, anything we should know" /></label>
        <div aria-hidden="true" style={{ position: 'absolute', left: '-9999px', width: 1, height: 1, overflow: 'hidden' }}><label>Leave this empty<input name="website" tabIndex={-1} autoComplete="off" /></label></div>
        <button type="submit" className="d-btn d-btn--red d-form__wide"><Send size={16} />{state === 'sending' ? 'Sending…' : 'Send message'}</button>
        <p className="d-form__note d-form__wide" role="status">{state === 'saved' ? 'Thank you. Your message reached the team and we will get back to you soon.' : state === 'mail' ? 'We could not reach our inbox, so your mail app should open with the message ready. Press send there, or email ' + contact.email + '.' : error || 'We reply by phone or email.'}</p>
      </form>
    </section>
  );
}

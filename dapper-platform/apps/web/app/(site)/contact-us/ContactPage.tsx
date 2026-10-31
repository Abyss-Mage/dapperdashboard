'use client';
import Link from 'next/link';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { FormEvent, KeyboardEvent } from 'react';
import { ArrowRight, Car, Ellipsis, Tag } from 'lucide-react';
import { weekHours } from '../content';
import { contact } from '../dapper';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Contact: the owner-approved comps, built in Dapper's own words. The enquiry form saves to the dashboard inbox (an email
   draft is the fallback). Motion is the GSAP demo hub's: split-text headlines, an image mask that slides against its photo,
   a pinned hero the next section overlaps, scrambled city tag, button text that diffs between states, and a small physics
   burst once an enquiry is sent. Everything animated is gated behind prefers-reduced-motion. */

type Topic = 'service' | 'quote' | 'other';
type Send = 'idle' | 'sending' | 'saved' | 'mail';

const topics = [
  { key: 'service', label: 'Choose a service', Icon: Car },
  { key: 'quote', label: 'Request a quote', Icon: Tag },
  { key: 'other', label: 'Something else', Icon: Ellipsis },
] as const;
const serviceOptions = ['Detailing', 'Ceramic coating', 'Paint protection film', 'Window tint', 'Mobile detailing'];
const placeholders: Record<Topic, string> = {
  service: 'Tell us what you have in mind.',
  quote: 'Tell us about your vehicle and what you would like quoted.',
  other: 'Tell us how we can help.',
};
const faq = [
  { q: 'How do I book an appointment?', a: 'Use Book an appointment to start the booking process. For help choosing a service, call our team or send an enquiry.', link: { href: '/booking', label: 'Book an appointment', plain: true } },
  { q: 'Not sure which service to choose?', a: 'Tell us about your vehicle and what you would like to change. The team will point you to the right care and, if you ask, a quote.', link: { href: '/services', label: 'See our services' } },
  { q: 'Can you come to my location?', a: 'Dapper offers mobile service from its Peterborough base. Tell us your location, vehicle and the service you have in mind, and the team will confirm whether it can be arranged.' },
  { q: 'Does sending an enquiry confirm a booking?', a: 'No. An enquiry starts a conversation. An appointment is only booked once you complete online booking or the team confirms a time with you.' },
];

const reduced = () => matchMedia('(prefers-reduced-motion: reduce)').matches;
const Roll = ({ children }: { children: string }) => <span className="c-roll"><span>{children}</span><span aria-hidden="true">{children}</span></span>;

export default function ContactPage() {
  const root = useRef<HTMLDivElement>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const tabsRef = useRef<HTMLDivElement>(null);
  const inkRef = useRef<HTMLSpanElement>(null);
  const labelRef = useRef<HTMLSpanElement>(null);
  const sendRef = useRef<HTMLButtonElement>(null);
  const placed = useRef(false);
  const [topic, setTopic] = useState<Topic>('service');
  const [service, setService] = useState('');
  const [state, setState] = useState<Send>('idle');
  const [error, setError] = useState('');
  const [open, setOpen] = useState<number | null>(0);
  const [today, setToday] = useState<string | null>(null);

  useEffect(() => { setToday(new Intl.DateTimeFormat('en-CA', { weekday: 'long', timeZone: 'America/Toronto' }).format(new Date())); }, []);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      // Arrival: the headline rises out of its mask, the photo opens from the top, the tag slides in and scrambles to its word.
      SplitText.create('.c-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.1, stagger: .07, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .fromTo('.c-hero__photo', { clipPath: 'inset(0 0 100% 0)' }, { clipPath: 'inset(0 0 0% 0)', duration: 1.2 }, .15)
        .from('.c-hero__photo img', { scale: 1.12, duration: 1.8, ease: 'power3.out' }, .15)
        .from('.c-tag', { xPercent: 100, duration: .9 }, .45)
        .from('.c-hero__info > *', { y: 20, opacity: 0, duration: .9, stagger: .08 }, .6)
        .to('.c-tag span', { duration: 1.1, ease: 'none', scrambleText: { text: 'PETERBOROUGH', chars: 'upperCase', speed: .6 } }, .6);

      // Section titles rise out of masks as they arrive; everything else fades up in one batch.
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.lines, { yPercent: 110, duration: 1.05, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      ScrollTrigger.batch('[data-in]', { start: 'top 92%', once: true, onEnter: (els) => gsap.from(els, { y: 24, opacity: 0, duration: .9, stagger: .07, ease: 'expo.out', overwrite: true }) });

      // Image mask on scroll: the frame slides in one way while the photo slides the other, so the picture holds still.
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.c-mobile__media', start: 'top 85%', end: 'top 25%', scrub: true } })
        .fromTo('.c-mobile__mask', { xPercent: -100 }, { xPercent: 0 })
        .fromTo('.c-mobile__mask img', { xPercent: 100 }, { xPercent: 0 }, 0);
    });

    // Pinned panel with overscroll: on a desktop screen the hero stays put and settles back as the enquiry section slides over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const hero = root.current!.querySelector<HTMLElement>('.c-hero')!;
      gsap.timeline({ scrollTrigger: { trigger: hero, start: () => (hero.offsetHeight > innerHeight ? 'bottom bottom' : 'top top'), end: () => '+=' + innerHeight, pin: true, pinSpacing: false, scrub: true, invalidateOnRefresh: true } })
        .fromTo('.c-hero__stage', { scale: 1, opacity: 1, transformOrigin: '50% 30%' }, { scale: .88, opacity: .35, duration: .9, ease: 'none' })
        .to('.c-hero__stage', { opacity: 0, duration: .1, ease: 'none' });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  // The active topic's ink block glides between tabs (it just snaps on first paint and on resize).
  useLayoutEffect(() => {
    const list = tabsRef.current, ink = inkRef.current;
    if (!list || !ink) return;
    const place = (animate: boolean) => {
      const on = list.querySelector<HTMLElement>('[aria-checked="true"]');
      if (!on) return;
      const to = { x: on.offsetLeft, width: on.offsetWidth };
      if (animate && !reduced()) gsap.to(ink, { ...to, duration: .38, ease: 'power3.out', overwrite: true });
      else gsap.set(ink, to);
    };
    place(placed.current);
    placed.current = true;
    const ro = new ResizeObserver(() => place(false));
    ro.observe(list);
    return () => ro.disconnect();
  }, [topic]);

  // The send button's label changes by diffing the old text into the new, in step with the real request.
  useEffect(() => {
    const el = labelRef.current;
    if (!el) return;
    const next = { idle: 'Send enquiry', sending: 'Sending…', saved: 'Sent', mail: 'Opening mail…' }[state];
    if (el.textContent === next) return;
    if (reduced()) { el.textContent = next; return; }
    const t = gsap.to(el, { duration: .45, ease: 'sine.inOut', text: { value: next, type: 'diff' }, overwrite: true });
    return () => { t.kill(); el.textContent = next; };
  }, [state]);

  const burst = (from: HTMLElement) => {
    if (reduced()) return;
    const b = from.getBoundingClientRect();
    const layer = document.createElement('div');
    layer.className = 'c-burst';
    layer.setAttribute('aria-hidden', 'true');
    document.body.appendChild(layer);
    const tl = gsap.timeline({ onComplete: () => layer.remove() });
    for (let i = 0; i < 18; i++) {
      const dot = document.createElement('i');
      if (i % 3 === 0) dot.className = 'is-ink';
      layer.appendChild(dot);
      gsap.set(dot, { x: b.left + b.width / 2 + gsap.utils.random(-b.width / 3, b.width / 3), y: b.top + b.height / 2, scale: gsap.utils.random(.6, 1.2), rotation: gsap.utils.random(0, 90) });
      tl.to(dot, { duration: gsap.utils.random(.8, 1.2), physics2D: { angle: gsap.utils.random(235, 305), velocity: gsap.utils.random(260, 560), gravity: 1100 } }, 0)
        .to(dot, { opacity: 0, duration: .3, ease: 'power2.inOut' }, .65);
    }
  };

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const f = new FormData(form);
    const field = (k: string) => String(f.get(k) ?? '').trim();
    const what = topic === 'other' ? 'Something else' : topic === 'quote' ? 'Quote: ' + field('service') : field('service');
    setState('sending'); setError('');
    try {
      const r = await fetch('/api/v1/public/enquiries', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: field('name'), email: field('email'), phone: field('phone'), vehicle: field('vehicle'), service: what, message: field('message'), website: field('website') }) });
      if (r.status === 422 || r.status === 429) { setError((await r.json().catch(() => ({}))).error ?? 'Your message could not be sent. Please call us.'); setState('idle'); return; }
      if (!r.ok) throw new Error('unavailable');
      form.reset();
      setService('');
      setState('saved');
      if (sendRef.current) burst(sendRef.current);
    } catch {
      const body = `Name: ${field('name')}\nPhone: ${field('phone')}\nEmail: ${field('email')}\nVehicle: ${field('vehicle')}\nService: ${what}\n\n${field('message')}`;
      location.href = 'mailto:' + contact.email + '?subject=' + encodeURIComponent('Website enquiry: ' + what) + '&body=' + encodeURIComponent(body);
      setState('mail');
    }
  };

  const tabKeys = (e: KeyboardEvent) => {
    const i = topics.findIndex((t) => t.key === topic);
    const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = topics[(i + step + topics.length) % topics.length].key;
    setTopic(next);
    requestAnimationFrame(() => tabsRef.current?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus());
  };

  const askMobile = () => {
    setTopic('service');
    setService('Mobile detailing');
    document.getElementById('enquiry')?.scrollIntoView({ behavior: reduced() ? 'auto' : 'smooth', block: 'start' });
    setTimeout(() => formRef.current?.querySelector<HTMLInputElement>('[name=name]')?.focus({ preventScroll: true }), 650);
  };

  const phone = '+1 ' + contact.phone;
  const note = state === 'saved' ? 'Thank you. Your enquiry reached the team and we will get back to you soon.'
    : state === 'mail' ? 'We could not reach our inbox, so your mail app should open with the message ready. Press send there, or email ' + contact.email + '.'
    : error;

  return (
    <div ref={root} className="c-page">
      {/* Hero */}
      <section className="c-hero" aria-labelledby="c-title">
        <div className="c-hero__stage">
          <h1 id="c-title" className="c-h1">Let’s talk <em>cars.</em></h1>
          <div className="c-hero__band">
            <div className="c-hero__photo"><img src="/site/assets/portfolio/portfolio-06.jpg" alt="A white Porsche 911 cabriolet outside the Dapper studio" fetchPriority="high" /></div>
            <div className="c-tag" aria-hidden="true"><span>PETERBOROUGH</span></div>
          </div>
          <div className="c-hero__info">
            <div className="c-hero__lead">
              <h2>A question. A quote. Your next detail.</h2>
              <p>Tell us about your vehicle. We’ll help you choose the right care.</p>
            </div>
            <dl className="c-hero__reach">
              <div><dt>Call</dt><dd><a href={'tel:' + contact.tel}>{phone}</a></dd></div>
              <div><dt>Email</dt><dd><a href={'mailto:' + contact.email}>{contact.email}</a></dd></div>
            </dl>
          </div>
          <p className="c-hero__where">
            <svg viewBox="0 0 24 28" width="18" height="21" aria-hidden="true"><path d="M12 27S3 18.5 3 11a9 9 0 1 1 18 0c0 7.5-9 16-9 16Z" fill="currentColor" /><circle cx="12" cy="11" r="3.4" fill="#faf9f6" /></svg>
            <span>{contact.street}, Peterborough</span><i aria-hidden="true" />
            <a className="c-link" href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions<ArrowRight size={16} aria-hidden="true" /></a>
          </p>
        </div>
      </section>

      <div className="c-after">
        {/* Enquiry */}
        <section className="c-sec c-form" id="enquiry" aria-labelledby="c-form-title">
          <div className="c-sec__head">
            <h2 id="c-form-title" data-head>What can we <em>help</em> with?</h2>
            <p data-in>Choose a topic, then tell us a little about your vehicle.</p>
          </div>
          <div className="c-tabs" role="radiogroup" aria-label="Topic" ref={tabsRef} onKeyDown={tabKeys} data-in>
            <span className="c-tabs__ink" ref={inkRef} aria-hidden="true" />
            {topics.map(({ key, label, Icon }) => (
              <button key={key} type="button" role="radio" aria-checked={topic === key} tabIndex={topic === key ? 0 : -1} className="c-tab" onClick={() => setTopic(key)}>
                <Icon size={26} strokeWidth={1.6} aria-hidden="true" />{label}
              </button>
            ))}
          </div>
          <form ref={formRef} className="c-fields" onSubmit={(e) => void submit(e)} data-in>
            <div className="c-col">
              <label className="c-field"><span>Name <i aria-hidden="true">*</i></span><input name="name" required autoComplete="name" /></label>
              <label className="c-field"><span>Email <i aria-hidden="true">*</i></span><input name="email" type="email" required autoComplete="email" /></label>
              <label className="c-field"><span>Phone <small>(optional)</small></span><input name="phone" type="tel" autoComplete="tel" inputMode="tel" /></label>
              <label className="c-field"><span>Vehicle <small>(optional)</small></span><input name="vehicle" placeholder="Year, make and model" /></label>
            </div>
            <div className="c-col c-col--grow">
              {topic !== 'other' && (
                <label className="c-field"><span>{topic === 'quote' ? 'Service to quote' : 'Service'} <i aria-hidden="true">*</i></span>
                  <select name="service" required value={service} onChange={(e) => setService(e.target.value)}>
                    <option value="" disabled>Choose a service</option>
                    {serviceOptions.map((o) => <option key={o}>{o}</option>)}
                  </select>
                </label>
              )}
              <label className="c-field c-field--grow"><span>Message <i aria-hidden="true">*</i></span><textarea name="message" required maxLength={3000} placeholder={placeholders[topic]} /></label>
            </div>
            <div aria-hidden="true" className="c-trap"><label>Leave this empty<input name="website" tabIndex={-1} autoComplete="off" /></label></div>
            <div className="c-foot">
              <p>Required fields are marked <b aria-hidden="true">*</b></p><i aria-hidden="true" />
              <p>Submitting an enquiry does not book an appointment.</p>
              <button type="submit" ref={sendRef} className="c-btn c-btn--red" disabled={state === 'sending'}>
                <span ref={labelRef}>Send enquiry</span><ArrowRight size={20} aria-hidden="true" />
              </button>
            </div>
            <p className="c-note" role="status">{note}</p>
          </form>
        </section>

        {/* Visit */}
        <section className="c-sec c-visit" aria-labelledby="c-visit-title">
          <div className="c-visit__text">
            <h2 id="c-visit-title" data-head>Visit the studio<span className="c-stop">.</span></h2>
            <p className="c-sub" data-in>Find your way to Dapper.</p>
            <div className="c-datum" data-in><span>Address</span><address>{contact.street}<br />{contact.city}, {contact.country}</address>
              <a className="c-btn c-btn--red" href={contact.directions} target="_blank" rel="noopener noreferrer"><Roll>Get directions</Roll><ArrowRight size={20} aria-hidden="true" /></a>
            </div>
            <div className="c-datum" data-in><span>Call the studio</span><a className="c-phone" href={'tel:' + contact.tel}>{phone}</a></div>
          </div>
          <a className="c-map" href={contact.directions} target="_blank" rel="noopener noreferrer" aria-label="Open the studio location in maps" data-in>
            <svg viewBox="0 0 800 860" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
              <rect width="800" height="860" fill="#e9e8e4" />
              <g fill="#dcdbd6"><path d="M60 70l95-20 38 125-95 28z" /><path d="M215 100l150-30 40 180-150 36z" /><path d="M410 40h120v90H410z" /><path d="M690 70l110-40v140l-110 40z" /><path d="M590 250l110-20 30 110-120 20z" /><path d="M95 330l100-18 36 150-110 30z" /><path d="M300 330l150-10 20 170-160 20z" /><path d="M520 450l140-30 40 160-150 30z" /><path d="M60 620l140-20 30 150-150 40z" /><path d="M330 620l120-16 20 170-130 20z" /><path d="M560 660l120-20 50 140-150 30z" /></g>
              <g fill="none" stroke="#fff" strokeWidth="26" strokeLinecap="round" strokeLinejoin="round"><path d="M0 240L200 330 330 300 560 130 800 70" /><path d="M0 560L260 520 470 590 800 420" /><path d="M180 0L250 330 270 560 220 860" /><path d="M540 0L470 590 560 860" /><path d="M0 760L300 800 800 760" /><path d="M640 120L690 330 560 520" /></g>
            </svg>
            <span className="c-map__pin"><svg viewBox="0 0 24 28" width="46" height="54" aria-hidden="true"><path d="M12 27S3 18.5 3 11a9 9 0 1 1 18 0c0 7.5-9 16-9 16Z" fill="#e3172b" /><circle cx="12" cy="11" r="3.6" fill="#fff" /></svg><b>Dapper Detailing Pros</b></span>
            <span className="c-map__note">Map preview · illustrative</span>
          </a>
        </section>

        {/* Hours */}
        <section className="c-sec c-hours" aria-labelledby="c-hours-title">
          <div className="c-hours__text">
            <h2 id="c-hours-title" data-head>Opening hours<span className="c-stop">.</span></h2>
            <p className="c-hours__lede" data-in>Open 7 days a week</p>
            <p className="c-muted" data-in>Hours shown in Peterborough local time. Contact us to confirm holiday hours and appointment availability.</p>
            <div className="c-datum" data-in><span>Call the studio</span><a className="c-phone" href={'tel:' + contact.tel}>{phone}</a></div>
          </div>
          <div className="c-week" data-in>
            <h3>Weekly schedule</h3>
            <table>
              <caption className="sr">Opening hours by day</caption>
              <tbody>
                {weekHours.days.map((d) => (
                  <tr key={d} data-today={today === d || undefined}><th scope="row">{d}</th><td>{weekHours.open} – {weekHours.close}</td></tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Mobile service */}
        <section className="c-mobile" aria-labelledby="c-mobile-title">
          <div className="c-mobile__media">
            <div className="c-mobile__mask"><img src="/site/assets/portfolio/portfolio-08.jpg" alt="A Dapper detailer washing a GMC Hummer EV in a customer’s driveway" loading="lazy" /></div>
          </div>
          <div className="c-mobile__text">
            <h2 id="c-mobile-title" data-head>Can we come <em>to you?</em></h2>
            <p className="c-muted c-mobile__lede" data-in>Based in Peterborough. Tell us your location, vehicle and the service you have in mind. Our team will confirm whether mobile service can be arranged.</p>
            <button type="button" className="c-btn c-btn--red" onClick={askMobile} data-in><Roll>Ask about mobile service</Roll><ArrowRight size={20} aria-hidden="true" /></button>
            <p className="c-mobile__call" data-in>Call <a href={'tel:' + contact.tel}>{phone}</a></p>
            <p className="c-fine" data-in>Service area and appointment availability are confirmed by the team.</p>
          </div>
        </section>

        {/* FAQ */}
        <section className="c-sec c-faq" aria-labelledby="c-faq-title">
          <div className="c-faq__side">
            <h2 id="c-faq-title" data-head>A few things to <em>know.</em></h2>
            <p className="c-sub" data-in>Before you get in touch.</p>
            <p className="c-faq__ask" data-in>Still have a question?<br /><a className="c-link" href="#enquiry">Ask the team<ArrowRight size={16} aria-hidden="true" /></a></p>
          </div>
          <div className="c-faq__list" data-in>
            {faq.map((item, i) => (
              <div key={item.q} className="c-faq__item" data-open={open === i || undefined}>
                <h3><button type="button" id={'c-q' + i} aria-expanded={open === i} aria-controls={'c-a' + i} onClick={() => setOpen(open === i ? null : i)}>{item.q}<span className="c-faq__icon" aria-hidden="true" /></button></h3>
                <div className="c-faq__panel" id={'c-a' + i} role="region" aria-labelledby={'c-q' + i} inert={open !== i}>
                  <div>
                    <p>{item.a}</p>
                    {item.link && (item.link.plain
                      ? <a className="c-link" href={item.link.href}>{item.link.label}<ArrowRight size={16} aria-hidden="true" /></a>
                      : <Link className="c-link" href={item.link.href}>{item.link.label}<ArrowRight size={16} aria-hidden="true" /></Link>)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

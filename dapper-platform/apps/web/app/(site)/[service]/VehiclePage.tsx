'use client';
import { ScrollRow } from '../_components/ScrollRow';
import { CtaBand } from '../_components/CtaBand';
import Link from 'next/link';
import { useRef, useState } from 'react';
import { ArrowRight, Check, Plus, Star } from 'lucide-react';
import { categories } from '../content';
import { faqs } from '../faq';
import { contact, cta, processSteps, proof, testimonials, why } from '../dapper';
import { usePublished } from '../_components/usePublished';
import { money, useLiveCatalog, useLiveFrom } from '../_components/useLiveFrom';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* One template for every service page, in the same darker "hard" direction as Services.
   V1 hero with the vehicle, price and a switcher for the other nine. V2 packages published by the team. V3 what to expect. V4 reviews. V5 closing band. */

const kind = (key: string) => (key === 'ceramic' ? 'Coating' : key === 'ppf' ? 'Paint protection film' : key === 'window-tint' ? 'Window tint' : 'Detailing');

/* What each package includes, in the studio's own words from dapperdetailing.ca. Used when the package's published summary is still the
   generic booking line; once the team writes a real summary in the dashboard (one item per line) that is shown instead. */
const GENERIC = /book online, or call the team/i;
const INCLUDES: [RegExp, string[]][] = [
  [/ultimate/i, ['Includes the Luxury Full Body Detail', '12–18 month ceramic coating', '12-month fabric protection', 'Windshield ceramic coating', 'Glass cleaned inside and out']],
  [/luxury/i, ['Full interior detail', 'Exterior detail and wax', 'Rims and wheels detailed', 'Exterior surfaces enhanced', 'Tire dressing finish']],
  [/vip/i, ['Interior revived and shined', 'Professional steam cleaning', 'Deep seat cleaning', 'Glass and windows cleaned', 'Air freshener treatment']],
  [/exterior/i, ['Thorough hand wash', 'Wheel and tire cleaning', 'Bug and tar removal', 'Door jamb cleaning', '30-day wax protection']],
];
const includes = (name: string, summary: string): { list: string[]; text: string } => {
  if (summary && !GENERIC.test(summary)) {
    const lines = summary.split(/\r?\n|•|;/).map((x) => x.trim()).filter(Boolean);
    return lines.length > 1 ? { list: lines, text: '' } : { list: [], text: summary };
  }
  const hit = INCLUDES.find(([re]) => re.test(name));
  return { list: hit ? hit[1] : [], text: hit || GENERIC.test(summary) ? '' : summary };
};

function Packages({ category, name }: { category: string; name: string }) {
  const { status, services, retry } = usePublished();
  const { price } = useLiveCatalog();
  const [tab, setTab] = useState('');
  // Cheapest first (unpriced last).
  const items = services.filter((s) => s.category === category).sort((a, b) => (price[a.serviceId] ?? Infinity) - (price[b.serviceId] ?? Infinity));
  // Film is priced per vehicle ("Sedan - Front PPF"): choose the vehicle first, then its packages open.
  const typeOf = (n: string) => (n.includes(' - ') ? n.split(' - ')[0].trim() : '');
  const types = [...new Set(category === 'ppf' ? items.map((s) => typeOf(s.name)).filter(Boolean) : [])];
  const active = types.includes(tab) ? tab : types[0] ?? '';
  const shown = types.length ? items.filter((s) => typeOf(s.name) === active) : items;
  if (status === 'loading') return <p className="v-note" role="status">Loading packages…</p>;
  if (status === 'error') return <div className="v-note" role="status"><span>Current packages could not be loaded. Call the team or try again.</span><button type="button" className="v-ghost" onClick={retry}>Try again</button></div>;
  if (!items.length) return (
    <div className="v-note v-note--big" role="status">
      <span><b>No packages listed for {name.toLowerCase()} yet.</b> Tell us what you drive and what shape it’s in. We’ll tell you what it needs.</span>
      <a className="v-ghost" href={'tel:' + contact.tel}>Call {contact.phone}</a>
    </div>
  );
  return (
    <>
      {types.length > 1 && (
        <div className="v-tabs" role="tablist" aria-label="Choose your vehicle">
          {types.map((t) => <button key={t} type="button" role="tab" aria-selected={t === active} className={t === active ? 'is-on' : undefined} onClick={() => setTab(t)}>{t}</button>)}
        </div>
      )}
      <ul className="v-pkgs" aria-live="polite">
        {shown.map((s) => {
          const inc = includes(s.name, s.summary);
          return (
            <li key={s.serviceId} className="v-row">
              <div className="v-row__main">
                <h3>{types.length || category === 'window-tint' ? s.name.replace(/^.*? - /, '') : s.name}</h3>
                {price[s.serviceId] !== undefined && <p className="v-row__price"><b>${money(price[s.serviceId])}</b></p>}
              </div>
              <div className="v-row__inc">
                {inc.list.length ? <><small>What’s included</small><ul>{inc.list.map((x) => <li key={x}><Check size={16} strokeWidth={3} aria-hidden="true" />{x}</li>)}</ul></> : inc.text ? <p>{inc.text}</p> : null}
              </div>
              <a className="v-row__book" href={'/booking?package=' + encodeURIComponent(s.serviceId)}>Book now<ArrowRight size={18} aria-hidden="true" /></a>
            </li>
          );
        })}
      </ul>
    </>
  );
}

export default function VehiclePage({ slug }: { slug: string }) {
  const root = useRef<HTMLDivElement>(null);
  const c = categories.find((x) => x.slug === slug)!;
  const live = useLiveFrom();
  const fromPrice = live[c.key] ?? c.from;
  const words = c.title.split(' ');
  const last = words.pop();

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.v-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .05, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.v-hero__img', { scale: 1.14, duration: 1.4, ease: 'power3.out' }, 0)
        .from('.v-crumb', { y: 14, opacity: 0, duration: .8 }, .15)
        .from('.v-lede', { y: 22, opacity: 0, duration: .9 }, .45)
        .from('.v-buy > *', { y: 24, opacity: 0, duration: .8, stagger: .08 }, .6);
      // The standard: the line fills and each point is ticked as it reaches the middle of the screen.
      gsap.fromTo('.v-steps__line i', { scaleY: 0 }, { scaleY: 1, ease: 'none', scrollTrigger: { trigger: '.v-steps', start: 'top 62%', end: 'bottom 62%', scrub: true } });
      gsap.utils.toArray<HTMLElement>('.v-step').forEach((el) => {
        ScrollTrigger.create({ trigger: el, start: 'top 62%', onEnter: () => el.classList.add('is-done'), onLeaveBack: () => el.classList.remove('is-done') });
        gsap.from(el.querySelectorAll('h3, p'), { y: 32, opacity: 0, duration: .9, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } });
      });
      // The closing call: the picture settles from a slight zoom as it arrives.
      gsap.fromTo('.v-end__img', { scale: 1.14 }, { scale: 1, ease: 'none', scrollTrigger: { trigger: '.v-end', start: 'top bottom', end: 'bottom bottom', scrub: true } });
      // The price counts up to itself.
      const price = document.querySelector<HTMLElement>('.v-price b');
      if (price && fromPrice && Number.isInteger(fromPrice)) {
        const o = { n: 0 };
        gsap.to(o, { n: fromPrice, duration: 1.4, ease: 'power3.out', delay: .6, onUpdate: () => { price.textContent = '$' + Math.round(o.n); } });
      }
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .04, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      ScrollTrigger.batch('.v-row, .v-inc li', { start: 'top 92%', once: true,
        onEnter: (els) => gsap.from(els, { y: 36, opacity: 0, duration: .8, stagger: .07, ease: 'expo.out', overwrite: true }) });
      gsap.fromTo('.v-hero__img', { yPercent: 0 }, { yPercent: 12, ease: 'none', scrollTrigger: { trigger: '.v-hero', start: 'top top', end: 'bottom top', scrub: true } });
    });
    // On a phone the steps simply rise in one after another.
    mm.add('(max-width: 899px) and (prefers-reduced-motion: no-preference)', () => {
      ScrollTrigger.batch('.v-exp__item', { start: 'top 92%', once: true,
        onEnter: (els) => gsap.from(els, { y: 36, opacity: 0, duration: .8, stagger: .08, ease: 'expo.out', overwrite: true }) });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  const other = categories.filter((x) => x.slug !== c.slug);
  const noun = c.noun ?? c.short.toLowerCase().replace(/s$/, '');
  return (
    <div ref={root} className="s-page v-page">
      {/* V1 Hero: the vehicle edge to edge, the title over it, the price and the booking call on a bar along the bottom */}
      <section className="v-hero" aria-labelledby="v-title">
        <img className="v-hero__img" src={c.image} alt={c.photo ? c.title + ' at Dapper' : 'Vehicle detailed by Dapper'} fetchPriority="high" />
        <div className="v-hero__scrim" aria-hidden="true" />
        <div className="v-hero__copy">
          <p className="v-crumb"><Link href="/services">Services</Link><span aria-hidden="true">/</span>{c.short}</p>
          <h1 id="v-title" className="v-h1">{words.join(' ')} <em>{last}.</em></h1>
          <p className="v-lede">{c.summary}</p>
        </div>
        {c.photo ? null : <span className="s-tile__tag">Studio photo</span>}
        <div className="v-buy">
          {fromPrice
            ? <p className="v-price"><small>Starting from</small><b aria-label={'$' + money(fromPrice)}>${money(fromPrice)}</b></p>
            : <p className="v-price v-price--ask"><small>Pricing</small><b>Ask the team</b></p>}
          <a className="s-btn" href={'/booking?category=' + c.key}><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
          <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
        </div>
      </section>

      {/* V2 Packages: exactly what the team has published for this category */}
      <section className="v-sec v-light" aria-labelledby="v-pk-title">
        <div className="v-sec__head">
          <h2 id="v-pk-title" className="s-h2" data-head>Find your <em>care.</em></h2>
          <p>Every package, what is in it and what it costs. Pick the one that fits and book it online in a few minutes.</p>
        </div>
        <Packages category={c.key} name={c.short} />
      </section>

      {/* V3 What is in every job, in the studio's own words */}
      <section className="v-sec v-inc" aria-labelledby="v-in-title">
        <div className="v-sec__head">
          <h2 id="v-in-title" className="s-h2" data-head>Done the <em>Dapper way.</em></h2>
          <p>{kind(c.key)} for your {noun} always comes with the same standard.</p>
        </div>
        <ol className="v-steps">
          <span className="v-steps__line" aria-hidden="true"><i /></span>
          {why.map((w) => (
            <li key={w.title} className="v-step">
              <span className="v-step__dot" aria-hidden="true"><Check size={20} strokeWidth={3} /></span>
              <h3>{w.title}</h3>
              <p>{w.text}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* V4 What to expect: on a wide screen it pins, the four steps lit one at a time while the photograph changes beside them */}
      <section className="v-sec v-light v-exp" aria-labelledby="v-st-title">
        <div className="v-sec__head">
          <h2 id="v-st-title" className="s-h2" data-head>What to <em>expect.</em></h2>
          <p>From the first tap to the last wipe.</p>
        </div>
        <div className="v-exp__stage">
          <ol className="v-exp__list">
            {processSteps.map((p) => (
              <li key={p.title} className="v-exp__item">
                <span className="v-exp__m" style={{ backgroundImage: 'url(' + p.image + ')' }} aria-hidden="true" />
                <h3>{p.title}</h3>
                <p>{p.text}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* V5 Reviews: one lane the scroll pulls sideways while the section stays pinned (a plain swipeable row on a phone) */}
      <section className="v-sec v-rev" aria-labelledby="v-rv-title">
        <div className="v-sec__head">
          <h2 id="v-rv-title" className="s-h2" data-head>Straight from <em>the owners.</em></h2>
          <p className="v-rating"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b> · {proof.customers}</p>
        </div>
        <div className="v-rev__row">
          <ScrollRow label="Reviews from owners" auto={4500} className="v-htrack">
            {testimonials.map((t) => (
              <li key={t.name}>
                <figure className="v-quote">
                  <span className="v-quote__mark" aria-hidden="true">“</span>
                  <blockquote>{t.quote}</blockquote>
                  <figcaption>{t.name}</figcaption>
                </figure>
              </li>
            ))}
          </ScrollRow>
        </div>
      </section>

      {/* V6 Another ride? The other nine as big type; each row fades in and draws its rule with the scroll */}
      <section className="v-sec v-light v-more" aria-labelledby="v-mo-title">
        <div className="v-sec__head">
          <h2 id="v-mo-title" className="s-h2" data-head>Something else <em>in the garage?</em></h2>
          <p>We care for more than {noun}{noun.endsWith('s') ? '' : 's'}. Pick another ride to see its packages.</p>
        </div>
        <ScrollRow label="Other vehicle types" tone="light" className="v-cards">
          {other.map((x) => {
            const f = live[x.key] ?? x.from;
            return (
              <li key={x.slug} className="v-card" data-in>
                <Link href={'/' + x.slug} className="v-card__img" tabIndex={-1} aria-hidden="true"><span style={{ backgroundImage: 'url(' + x.image + ')' }} /></Link>
                <div className="v-card__body">
                  <h3>{x.short}</h3>
                  <p>{x.summary}</p>
                  <div className="v-card__foot">
                    <span className="v-card__from">{f ? <><small>Starts from</small><b>${money(f)}</b></> : <><small>Pricing</small><b>Ask the team</b></>}</span>
                    <Link href={'/' + x.slug} className="v-card__go">See packages<ArrowRight size={18} aria-hidden="true" /></Link>
                  </div>
                </div>
              </li>
            );
          })}
        </ScrollRow>
      </section>

      {/* V7 Questions people ask about this service */}
      <section className="v-sec v-faq" aria-labelledby="v-faq-title">
        <div className="v-sec__head">
          <h2 id="v-faq-title" className="s-h2" data-head>Questions, <em>answered.</em></h2>
          <p>Not sure which package suits your {noun}? Call the team for personal advice.</p>
        </div>
        <div className="v-faq__list">
          {(faqs[c.key] ?? []).map((f) => (
            <details key={f.q} className="v-faq__item">
              <summary>{f.q}<Plus size={22} aria-hidden="true" /></summary>
              <p>{f.a}</p>
            </details>
          ))}
        </div>
      </section>

      <CtaBand id="v-close-title" lead={'Ready to give your ' + noun}  />
    </div>
  );
}

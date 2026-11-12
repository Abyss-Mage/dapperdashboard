'use client';
import { ScrollRow } from '../_components/ScrollRow';
import { CtaBand } from '../_components/CtaBand';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight, Check, Plus, Star } from 'lucide-react';
import { categories } from '../content';
import { faqs } from '../faq';
import { contact, cta, processSteps, proof, testimonials, why } from '../dapper';
import { usePublished } from '../_components/usePublished';
import { money, useLiveCatalog, useLiveFrom } from '../_components/useLiveFrom';
import { useReplaced } from '../_components/useHidden';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* One template for every service page, in the same darker "hard" direction as Services.
   V1 hero with the vehicle, price and a switcher for the other nine. V2 packages published by the team. V3 what to expect. V4 reviews. V5 closing band. */

const kind = (key: string) => (key === 'ceramic' ? 'Coating' : key === 'ppf' ? 'Paint protection film' : key === 'window-tint' ? 'Window tint' : 'Detailing');

/* What each package includes, in the studio's own words from dapperdetailing.ca. Used when the package's published summary is still the
   generic booking line; once the team writes a real summary in the dashboard (one item per line) that is shown instead. */
const GENERIC = /book online, or call the team/i;
const INCLUDES: [RegExp, string[]][] = [
  [/front ppf/i, ['Full hood', 'Full front bumper', 'Both front fenders', 'Professional installation and edge finishing']],
  [/full ppf/i, ['Full exterior paint coverage', 'Hood, bumpers, fenders and doors', 'Quarter panels, roof and trunk', 'Other painted exterior panels', 'Professional installation and edge finishing']],
  [/express interior/i, ['Full interior vacuum: seats, carpets and floor mats', 'Trunk or cargo area vacuumed', 'Dash, console and door panels wiped down', 'Cupholders, vents and crevices dusted out', 'Interior glass cleaned, streak-free', 'Floor mats pulled, shaken and cleaned', 'Light deodorize for a fresh finish']],
  [/armour up/i, ['Full 3-stage paint correction', 'Swirl marks, scratches, oxidation and paint imperfections removed', 'Premium ceramic coating applied', '5-Year Manufacturer Warranty', 'Before-and-after photos of the transformation']],
  [/^3 stage paint correction/i, ['Removes heavy swirl marks, scratches, oxidation and paint defects', 'Restores gloss, clarity and depth', 'Mirror-like reflections']],
  [/fortress/i, ['Premium-grade ceramic coating', 'Complete paint decontamination and surface preparation', 'Enhanced hydrophobic protection', 'Superior resistance to minor scratches', 'Protection against UV rays and chemical contaminants', 'Annual maintenance reminders', '5-Year Manufacturer Warranty']],
  [/shield package/i, ['Professional ceramic coating application', 'Complete paint decontamination and surface preparation', 'Hydrophobic water-repelling layer', 'Protection against harmful UV rays', '6-hour professional installation', 'Post-cure care guidance', '3-Year Manufacturer Warranty']],
  [/5 year ceramic/i, ['Professional ceramic coating', 'High-gloss hydrophobic finish', 'Protection against UV rays and contaminants', '5-Year Ceramic Coating Warranty']],
  [/single window/i, ['Replace or upgrade the tint on a single window', 'Ideal for small fixes and privacy', 'Heat and UV protection on a specific window']],
  [/4-window/i, ['Complete tinting for all four side windows', 'Enhances privacy', 'Reduces interior heat and blocks harmful UV rays', 'A sleek, uniform look']],
  [/^(boat|aircraft).*signature/i, ['Full interior detail', 'Full exterior detail', 'Surfaces cleaned, decontaminated and dried', 'Protective wax finish']],
  [/^(boat|aircraft).*paint correction/i, ['Surface washed and decontaminated', 'Machine polishing to reduce swirls and light scratches', 'Gloss restored across the paint']],
  [/^(boat|aircraft).*ceramic/i, ['Surface washed and decontaminated', 'Ceramic coating applied to the exterior', 'A glossy, water-repellent finish']],
  [/^(boat|aircraft).*interior/i, ['Thorough vacuum of every surface', 'Seats, carpet and upholstery cleaned', 'Panels, trim and vinyl wiped and conditioned', 'Glass and windows cleaned inside']],
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
  const { price, addons } = useLiveCatalog();
  const [tab, setTab] = useState('');
  // Cheapest first (unpriced last).
  const items = services.filter((s) => s.category === category).sort((a, b) => Number(!!b.starred) - Number(!!a.starred) || (price[b.serviceId] ?? -1) - (price[a.serviceId] ?? -1));
  // Film is priced per vehicle ("Sedan - Front PPF"): choose the vehicle first, then its packages open.
  const typeOf = (n: string) => (n.includes(' - ') ? n.split(' - ')[0].trim() : '');
  const sized = category === 'boat' || category === 'aircraft';
  const order = ['Small', 'Medium', 'Large'];
  const sizeOf = (t: string) => t.replace(/^(boat|aircraft)\s+/i, '');
  const types = [...new Set(category === 'ppf' || sized ? items.map((s) => typeOf(s.name)).filter(Boolean) : [])]
    .sort((a, b) => (sized ? order.indexOf(sizeOf(a)) - order.indexOf(sizeOf(b)) : 0));
  const active = types.includes(tab) ? tab : types[0] ?? '';
  const shown = types.length ? items.filter((s) => typeOf(s.name) === active) : items;
  // Add-ons that belong to any package on this page, once each by name.
  const ids = new Set(items.map((s) => s.serviceId));
  const extras = [...new Map(addons.filter((a) => a.parentIds.some((p) => ids.has(p))).map((a) => [a.name.trim().toLowerCase(), a])).values()].sort((a, b) => a.name.localeCompare(b.name));
  // The packages arrive after the page has measured itself; measure again so the scroll effects further down fire in the right places.
  useEffect(() => { if (status !== 'loading') requestAnimationFrame(() => ScrollTrigger.refresh()); }, [status, shown.length, extras.length]);
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
        <div className="v-tabs" role="tablist" aria-label={sized ? 'Choose the size' : 'Choose your vehicle'}>
          {types.map((t) => <button key={t} type="button" role="tab" aria-selected={t === active} className={t === active ? 'is-on' : undefined} onClick={() => setTab(t)}>{sized ? sizeOf(t) : t}</button>)}
        </div>
      )}
      <ul className="v-pkgs" aria-live="polite">
        {shown.map((s) => {
          const inc = includes(s.name, s.summary);
          return (
            <li key={s.serviceId} className={s.starred ? 'v-star' : undefined}>
              <div className={'v-row' + (s.starred ? ' is-star' : '')}>
              <div className="v-row__main">
                {s.starred && <p className="v-row__star"><Star size={14} fill="currentColor" strokeWidth={0} aria-hidden="true" />Bestseller</p>}
                <h3>{types.length || category === 'window-tint' ? s.name.replace(/^.*? - /, '') : s.name}</h3>
                {price[s.serviceId] !== undefined && <p className="v-row__price"><b>${money(price[s.serviceId])}</b></p>}
              </div>
              <div className="v-row__inc">
                {inc.list.length ? <><small>What’s included</small><ul>{inc.list.map((x) => <li key={x}><Check size={16} strokeWidth={3} aria-hidden="true" />{x}</li>)}</ul></> : inc.text ? <p>{inc.text}</p> : null}
              </div>
              <a className="v-row__book" href={'/book-appointment?package=' + encodeURIComponent(s.serviceId)}>Book now<ArrowRight size={18} aria-hidden="true" /></a>
              </div>
            </li>
          );
        })}
      </ul>
      {extras.length > 0 && (
        <div className="v-addons">
          <h3>Add-ons</h3>
          <p>Pick these in the booking when you choose your package.</p>
          <ul>{extras.map((x) => <li key={x.name}><span>{x.name}</span><b>+${money(x.priceCents / 100)}</b></li>)}</ul>
        </div>
      )}
    </>
  );
}

export default function VehiclePage({ slug }: { slug: string }) {
  const isOwn = useReplaced();
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
        .from('.v-buy > *', { opacity: 0, duration: .8, stagger: .08 }, .6);
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
        {c.photo || isOwn(c.image) ? null : <span className="s-tile__tag">Studio photo</span>}
        <div className="v-buy">
          {fromPrice
            ? <p className="v-price"><small>Starting from</small><b aria-label={'$' + money(fromPrice)}>${money(fromPrice)}</b></p>
            : <p className="v-price v-price--ask"><small>Pricing</small><b>Ask the team</b></p>}
          <a className="s-btn" href={'/book-appointment?category=' + c.key}><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
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

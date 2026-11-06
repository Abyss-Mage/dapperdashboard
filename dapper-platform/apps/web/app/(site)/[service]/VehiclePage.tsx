'use client';
import Link from 'next/link';
import { useRef } from 'react';
import { ArrowRight, Check, Star } from 'lucide-react';
import { categories } from '../content';
import { contact, cta, processSteps, proof, testimonials, why } from '../dapper';
import { usePublished } from '../_components/usePublished';
import { money, useLiveCatalog, useLiveFrom } from '../_components/useLiveFrom';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* One template for every service page, in the same darker "hard" direction as Services.
   V1 hero with the vehicle, price and a switcher for the other nine. V2 packages published by the team. V3 what to expect. V4 reviews. V5 closing band. */

const kind = (key: string) => (key === 'ceramic' ? 'Coating' : key === 'ppf' ? 'Paint protection film' : 'Detailing');

function Packages({ category, name }: { category: string; name: string }) {
  const { status, services, retry } = usePublished();
  const { price } = useLiveCatalog();
  // Cheapest first (unpriced last); the cheapest main package is the one the page's "starting from" price comes from.
  const items = services.filter((s) => s.category === category).sort((a, b) => (price[a.serviceId] ?? Infinity) - (price[b.serviceId] ?? Infinity));
  const lowest = items.find((s) => s.kind === 'main' && price[s.serviceId] !== undefined)?.serviceId;
  if (status === 'loading') return <p className="v-note" role="status">Loading packages…</p>;
  if (status === 'error') return <div className="v-note" role="status"><span>Current packages could not be loaded. Call the team or try again.</span><button type="button" className="v-ghost" onClick={retry}>Try again</button></div>;
  if (!items.length) return (
    <div className="v-note v-note--big" role="status">
      <span><b>No packages listed for {name.toLowerCase()} yet.</b> Tell us what you drive and what shape it’s in. We’ll tell you what it needs.</span>
      <a className="v-ghost" href={'tel:' + contact.tel}>Call {contact.phone}</a>
    </div>
  );
  return (
    <ul className="v-pkgs" aria-live="polite">
      {items.map((s) => (
        <li key={s.serviceId} className="v-row">
          <h3>{s.name}</h3>
          <p>{s.summary}</p>
          <div className="v-row__end">
            {price[s.serviceId] !== undefined && <span className="v-row__price">{s.serviceId === lowest && <small>Lowest price</small>}<b>${money(price[s.serviceId])}</b></span>}
            <a className="v-row__go" href="/account">View options<ArrowRight size={18} aria-hidden="true" /></a>
          </div>
        </li>
      ))}
    </ul>
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
    // The garage list: each row fades and slides in and draws its rule as it reaches the middle of the screen.
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      gsap.utils.toArray<HTMLElement>('.v-more li').forEach((li) => {
        gsap.fromTo(li.querySelector('a'), { opacity: .2, xPercent: -3 }, { opacity: 1, xPercent: 0, ease: 'none', scrollTrigger: { trigger: li, start: 'top 94%', end: 'top 60%', scrub: true } });
        gsap.fromTo(li.querySelector('.v-more__rule'), { scaleX: 0 }, { scaleX: 1, ease: 'none', scrollTrigger: { trigger: li, start: 'top 94%', end: 'top 64%', scrub: true } });
      });
    });
    // On a phone the steps simply rise in one after another.
    mm.add('(max-width: 899px) and (prefers-reduced-motion: no-preference)', () => {
      ScrollTrigger.batch('.v-exp__item', { start: 'top 92%', once: true,
        onEnter: (els) => gsap.from(els, { y: 36, opacity: 0, duration: .8, stagger: .08, ease: 'expo.out', overwrite: true }) });
    });
    // On a wide screen the reviews and the steps pin and play with the scroll.
    mm.add('(min-width: 900px) and (prefers-reduced-motion: no-preference)', () => {
      const page = root.current;
      if (!page) return;
      const undo: Array<() => void> = [];
      const rev = page.querySelector<HTMLElement>('.v-rev');
      const view = page.querySelector<HTMLElement>('.v-hview');
      const track = page.querySelector<HTMLElement>('.v-htrack');
      if (rev && view && track) {
        rev.classList.add('is-pinned');
        undo.push(() => rev.classList.remove('is-pinned'));
        const dist = () => Math.max(0, track.scrollWidth - view.clientWidth);
        gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: rev, start: 'top top', end: () => '+=' + dist(), pin: true, scrub: 1, invalidateOnRefresh: true, anticipatePin: 1, refreshPriority: 1 } })
          .to(track, { x: () => -dist() }, 0)
          .to('.v-hbar i', { scaleX: 1 }, 0);
      }
      const exp = page.querySelector<HTMLElement>('.v-exp');
      if (exp) {
        exp.classList.add('is-pinned');
        undo.push(() => exp.classList.remove('is-pinned'));
        const items = gsap.utils.toArray<HTMLElement>('.v-exp__item', exp);
        const imgs = gsap.utils.toArray<HTMLElement>('.v-exp__img', exp);
        gsap.set(items.slice(1), { opacity: .28 });
        gsap.set(imgs.slice(1), { clipPath: 'inset(100% 0% 0% 0%)' });
        const tl = gsap.timeline({ defaults: { ease: 'power2.inOut' }, scrollTrigger: { trigger: exp, start: 'top top', end: () => '+=' + Math.round(window.innerHeight * .85 * (items.length - 1)), pin: true, scrub: .8, invalidateOnRefresh: true, anticipatePin: 1, refreshPriority: 2 } });
        for (let i = 1; i < items.length; i++) {
          tl.to(imgs[i], { clipPath: 'inset(0% 0% 0% 0%)', duration: 1 }, i - 1)
            .to(items[i - 1], { opacity: .28, duration: .45 }, i - 1)
            .to(items[i], { opacity: 1, duration: .45 }, i - 1 + .4);
        }
        tl.to('.v-exp__bar i', { scaleY: 1, ease: 'none', duration: items.length - 1 }, 0);
      }
      return () => undo.forEach((fn) => fn());
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  const other = categories.filter((x) => x.slug !== c.slug);
  const noun = c.short.toLowerCase().replace(/s$/, '');
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
          <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
          <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
        </div>
      </section>

      {/* V2 Packages: exactly what the team has published for this category */}
      <section className="v-sec" aria-labelledby="v-pk-title">
        <div className="v-sec__head">
          <h2 id="v-pk-title" className="s-h2" data-head>Find your <em>care.</em></h2>
          <p>Packages are published by the Dapper team. Sign in to choose one for your own vehicle and request a time.</p>
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
      <section className="v-sec v-sec--alt v-exp" aria-labelledby="v-st-title">
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
          <div className="v-exp__media" aria-hidden="true">
            {processSteps.map((p) => <span key={p.title} className="v-exp__img" style={{ backgroundImage: 'url(' + p.image + ')' }} />)}
            <span className="v-exp__bar"><i /></span>
          </div>
        </div>
      </section>

      {/* V5 Reviews: one lane the scroll pulls sideways while the section stays pinned (a plain swipeable row on a phone) */}
      <section className="v-sec v-rev" aria-labelledby="v-rv-title">
        <div className="v-sec__head">
          <h2 id="v-rv-title" className="s-h2" data-head>Straight from <em>the owners.</em></h2>
          <p className="v-rating"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b> · {proof.customers}</p>
        </div>
        <div className="v-hview">
          <div className="v-htrack">
            {testimonials.map((t) => (
              <figure key={t.name} className="v-quote">
                <span className="v-quote__mark" aria-hidden="true">“</span>
                <blockquote>{t.quote}</blockquote>
                <figcaption>{t.name}</figcaption>
              </figure>
            ))}
          </div>
        </div>
        <div className="v-hbar" aria-hidden="true"><i /></div>
      </section>

      {/* V6 Another ride? The other nine as big type; each row fades in and draws its rule with the scroll */}
      <section className="v-sec v-more" aria-labelledby="v-mo-title">
        <div className="v-more__wrap">
          <div className="v-more__head">
            <h2 id="v-mo-title" className="s-h2" data-head>Something else <em>in the garage?</em></h2>
          </div>
          <ul>
            {other.map((x) => <li key={x.slug}><Link href={'/' + x.slug}><span>{x.short}</span><ArrowRight size={28} aria-hidden="true" /></Link><i className="v-more__rule" aria-hidden="true" /></li>)}
          </ul>
        </div>
      </section>

      {/* V7 Closing band: the finished car from the film behind the booking call */}
      <section className="v-end" aria-labelledby="v-close-title">
        <img className="v-end__img" src="/site/assets/frames/desktop/0287.webp" alt="" loading="lazy" />
        <div className="v-end__in">
          <h2 id="v-close-title" className="s-h2" data-head>Ready to give your {noun} <em>the care it deserves?</em></h2>
          <p className="v-end__lede">{cta.text}</p>
          <div className="v-end__act">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="v-end__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </div>
      </section>
    </div>
  );
}

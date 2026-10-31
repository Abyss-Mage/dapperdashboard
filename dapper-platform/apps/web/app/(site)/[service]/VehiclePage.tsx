'use client';
import Link from 'next/link';
import { useRef } from 'react';
import { ArrowRight, Star } from 'lucide-react';
import { categories } from '../content';
import { contact, cta, processSteps, proof, testimonials } from '../dapper';
import { usePublished } from '../_components/usePublished';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* One template for every service page, in the same darker "hard" direction as Services.
   V1 hero with the vehicle, price and a switcher for the other nine. V2 packages published by the team. V3 what to expect. V4 reviews. V5 closing band. */

const pad = (n: number) => String(n + 1).padStart(2, '0');
const kind = (key: string) => (key === 'ceramic' ? 'Coating' : key === 'ppf' ? 'Paint protection film' : 'Detailing');

function Packages({ category, name }: { category: string; name: string }) {
  const { status, services, retry } = usePublished();
  const items = services.filter((s) => s.category === category);
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
      {items.map((s, i) => (
        <li key={s.name} className="v-pkg">
          <span className="v-pkg__n">{pad(i)}</span>
          <h3>{s.name}</h3>
          <p>{s.summary}</p>
          <a className="v-pkg__go" href="/account">View options<ArrowRight size={18} aria-hidden="true" /></a>
        </li>
      ))}
    </ul>
  );
}

export default function VehiclePage({ slug }: { slug: string }) {
  const root = useRef<HTMLDivElement>(null);
  const c = categories.find((x) => x.slug === slug)!;
  const idx = categories.indexOf(c);
  const words = c.title.split(' ');
  const last = words.pop();

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.v-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.1, stagger: .09, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.v-crumb', { y: 14, opacity: 0, duration: .8 }, .15)
        .from('.v-lede, .v-buy > *', { y: 22, opacity: 0, duration: .9, stagger: .08 }, .5)
        .fromTo('.v-hero__mask', { xPercent: 100 }, { xPercent: 0, duration: 1.3, ease: 'expo.inOut' }, .1)
        .fromTo('.v-hero__mask img', { xPercent: -100, scale: 1.15 }, { xPercent: 0, scale: 1, duration: 1.3, ease: 'expo.inOut' }, .1)
        .from('.v-rides li', { y: 24, opacity: 0, duration: .7, stagger: .035 }, .7);
      // The price counts up to itself.
      const price = document.querySelector<HTMLElement>('.v-price b');
      if (price && c.from) {
        const o = { n: 0 };
        gsap.to(o, { n: c.from, duration: 1.4, ease: 'power3.out', delay: .6, onUpdate: () => { price.textContent = '$' + Math.round(o.n); } });
      }
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.05, stagger: .07, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      ScrollTrigger.batch('.v-pkg, .v-step', { start: 'top 90%', once: true,
        onEnter: (els) => gsap.from(els, { y: 48, opacity: 0, duration: .9, stagger: .09, ease: 'expo.out', overwrite: true }) });
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.s-close__media', start: 'top 88%', end: 'top 30%', scrub: true } })
        .fromTo('.s-close__mask', { xPercent: 100 }, { xPercent: 0 })
        .fromTo('.s-close__mask img', { xPercent: -100 }, { xPercent: 0 }, 0);
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="s-page v-page">
      {/* V1 Hero: the vehicle, what it costs, how to book it */}
      <section className="v-hero" aria-labelledby="v-title">
        <div className="v-hero__text">
          <p className="v-crumb"><Link href="/services">Services</Link><span aria-hidden="true">/</span>{c.short}</p>
          <h1 id="v-title" className="v-h1">{words.join(' ')} <em>{last}.</em></h1>
          <p className="v-lede">{c.summary}</p>
          <div className="v-buy">
            {c.from
              ? <p className="v-price"><small>Starting from</small><b aria-label={'$' + c.from}>${c.from}</b></p>
              : <p className="v-price v-price--ask"><small>Pricing</small><b>Ask the team</b></p>}
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </div>
        <div className="v-hero__media">
          <div className="v-hero__mask"><img src={c.image} alt={c.photo ? c.title + ' at Dapper' : 'Vehicle detailed by Dapper'} fetchPriority="high" /></div>
          {c.photo ? null : <span className="s-tile__tag">Studio photo</span>}
        </div>
      </section>

      {/* The other nine rides, one tap away */}
      <nav className="v-rides" aria-label="Service categories">
        <ul>
          {categories.map((x, i) => (
            <li key={x.slug}><Link href={'/' + x.slug} aria-current={x.slug === c.slug ? 'page' : undefined}><span>{pad(i)}</span>{x.short}</Link></li>
          ))}
        </ul>
      </nav>

      {/* V2 Packages: exactly what the team has published for this category */}
      <section className="v-sec" aria-labelledby="v-pk-title">
        <div className="v-sec__head">
          <h2 id="v-pk-title" className="s-h2" data-head>Find your <em>care.</em></h2>
          <p>Packages are published by the Dapper team. Sign in to choose one for your own vehicle and request a time.</p>
        </div>
        <Packages category={c.key} name={c.short} />
      </section>

      {/* V3 What to expect: the same four steps, laid out as a row */}
      <section className="v-sec v-sec--alt" aria-labelledby="v-st-title">
        <div className="v-sec__head">
          <h2 id="v-st-title" className="s-h2" data-head>What to <em>expect.</em></h2>
          <p>{kind(c.key)} for your {c.short.toLowerCase().replace(/s$/, '')}, from the first tap to the last wipe.</p>
        </div>
        <ol className="v-steps">
          {processSteps.map((p, i) => (
            <li key={p.title} className="v-step">
              <span className="v-step__img" style={{ backgroundImage: 'url(' + p.image + ')' }} aria-hidden="true" />
              <span className="v-step__n" aria-hidden="true">{pad(i)}</span>
              <h3>{p.title}</h3>
              <p>{p.text}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* V4 Reviews: what clients say, in their own words */}
      <section className="v-sec v-rev" aria-labelledby="v-rv-title">
        <div className="v-sec__head">
          <h2 id="v-rv-title" className="s-h2" data-head>Straight from <em>the owners.</em></h2>
          <p className="v-rating"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b> · {proof.customers}</p>
        </div>
        {[0, 1].map((row) => (
          <div key={row} className={'v-marq' + (row ? ' v-marq--rev' : '')}>
            <div className="v-marq__track">
              {[...testimonials.slice(row * 4, row * 4 + 4), ...testimonials.slice(row * 4, row * 4 + 4)].map((t, i) => (
                <figure key={t.name + i} className="v-quote" aria-hidden={i >= 4 ? true : undefined}>
                  <blockquote>“{t.quote}”</blockquote>
                  <figcaption>{t.name}</figcaption>
                </figure>
              ))}
            </div>
          </div>
        ))}
      </section>

      {/* V5 Closing band */}
      <section className="s-close" aria-labelledby="v-close-title">
        <div className="s-close__text">
          <h2 id="v-close-title" className="s-h2" data-head>Ready to give your {c.short.toLowerCase().replace(/s$/, '')} <em>the care it deserves?</em></h2>
          <p className="s-close__lede">{cta.text}</p>
          <div className="s-close__actions">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </div>
        <div className="s-close__media">
          <div className="s-close__mask"><img src={categories[(idx + 3) % categories.length].photo ? categories[(idx + 3) % categories.length].image : '/site/assets/portfolio/portfolio-07.jpg'} alt="" loading="lazy" /></div>
        </div>
      </section>
    </div>
  );
}

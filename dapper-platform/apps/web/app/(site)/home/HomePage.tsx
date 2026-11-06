'use client';
import { CtaBand } from '../_components/CtaBand';
import { FilmCarousel } from '../_components/FilmCarousel';
import Link from 'next/link';
import { ArrowLeft, ArrowRight, Check, Play, Star, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { contact, cta, hero, proof, servicesIntro, testimonials, transformations, values, why } from '../dapper';
import { Dealers } from '../_components/Dealers';
import { money, useLiveFrom } from '../_components/useLiveFrom';
import { categories } from '../content';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';
import { frameSequence } from './frameSequence';
import { STORY } from './story';

/* Home, built section by section with the owner's approval of each, in the darker "hard" direction.
   H1: the studio, the headline, the booking call (to become a scroll-driven video sequence when the footage arrives). H2: core values. H3: the five priced services. H4: commercial partners. H5: transformation films. H6: why owners trust Dapper. H7: reviews. H8: closing booking band. */

// The three facts the core-values section sets as giant figures. From the old site's wording: the owner confirms them before launch.
const facts = [
  { to: 3000, suffix: '+', label: 'Vehicles detailed', sr: '3,000 plus vehicles detailed' },
  { to: 5, suffix: 'star', label: 'Google reviews', sr: 'Five star Google reviews' },
  { to: 5, suffix: '+', label: 'Years in the trade', sr: 'Five plus years in the trade' },
];

export default function HomePage() {
  const root = useRef<HTMLDivElement>(null);
  const [rev, setRev] = useState(0);
  const [deckIdx, setDeckIdx] = useState(0);
  const live = useLiveFrom();
  const priceOf = (c: { key: string; from?: number }) => live[c.key] ?? c.from;
  const nRev = testimonials.length;
  const goRev = (to: number) => setRev(((to % nRev) + nRev) % nRev);
  const clips = transformations.clips;
  const box = useRef<HTMLDialogElement>(null);
  const [film, setFilm] = useState<number | null>(null);
  const stepFilm = (n: number) => setFilm((i) => (i === null ? i : (i + n + clips.length) % clips.length));
  // The films open in a modal dialog with sound and controls: focus is trapped, Escape closes it, the arrows step through.
  useEffect(() => {
    const d = box.current;
    if (!d) return;
    if (film !== null && !d.open) d.showModal();
    if (film === null && d.open) d.close();
  }, [film]);
  // A new review's lines rise into place (instant with reduced motion); with motion it moves on by itself every eight seconds, paused while
  // it is pointed at or focused, and the bar under it shows the time left.
  const paused = useRef(false);
  const timer = useRef<gsap.core.Tween | null>(null);
  useEffect(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const q = document.querySelector<HTMLElement>('.h-rev__q blockquote');
    let split: SplitText | undefined;
    if (q) {
      split = SplitText.create(q, { type: 'lines', mask: 'lines', linesClass: 'c-line' });
      gsap.from(split.lines, { yPercent: 110, duration: .9, stagger: .08, ease: 'expo.out' });
      gsap.from('.h-rev__q figcaption', { opacity: 0, y: 12, duration: .6, delay: .4, ease: 'expo.out' });
    }
    timer.current = gsap.fromTo('.h-rev__bar i', { scaleX: 0 }, { scaleX: 1, duration: 8, ease: 'none', paused: paused.current, onComplete: () => setRev((i) => (i + 1) % nRev) });
    return () => { timer.current?.kill(); split?.revert(); };
  }, [rev, nRev]);
  const hold = (on: boolean) => { paused.current = on; if (on) timer.current?.pause(); else timer.current?.resume(); };

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.h-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.2, stagger: .08, ease: 'expo.out', delay: .2 }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.h-hero__tag, .h-hero__lead, .h-hero__act > *, .h-hero__trust', { y: 22, opacity: 0, duration: .9, stagger: .08 }, .6);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .04, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      gsap.from('.h-val__text > p, .h-val__text > a', { y: 24, opacity: 0, duration: .9, stagger: .1, ease: 'expo.out', scrollTrigger: { trigger: '.h-val', start: 'top 70%', once: true } });
      gsap.fromTo('.h-steps__line i', { scaleY: 0 }, { scaleY: 1, ease: 'none', scrollTrigger: { trigger: '.h-steps', start: 'top 62%', end: 'bottom 62%', scrub: true } });
      gsap.utils.toArray<HTMLElement>('.h-step').forEach((el) => {
        ScrollTrigger.create({ trigger: el, start: 'top 62%', onEnter: () => el.classList.add('is-done'), onLeaveBack: () => el.classList.remove('is-done') });
        gsap.from(el.querySelectorAll('h3, p'), { y: 36, opacity: 0, duration: .9, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } });
      });
      // The closing call: the picture settles from a slight zoom as it arrives.
      gsap.fromTo('.h-end__img', { scale: 1.14 }, { scale: 1, ease: 'none', scrollTrigger: { trigger: '.h-end', start: 'top bottom', end: 'bottom bottom', scrub: true } });
      // The films play only while they are on screen.
      const io = new IntersectionObserver((es) => es.forEach((e) => { const v = e.target as HTMLVideoElement; if (e.isIntersecting) void v.play().catch(() => {}); else v.pause(); }), { threshold: .35 });
      gsap.utils.toArray<HTMLVideoElement>('.h-reel video').forEach((v) => io.observe(v));
      // The hero pins and plays the frame sequence with the scroll.
      const seq = root.current?.querySelector<HTMLElement>('.h-seq');
      const stop = seq ? frameSequence(seq) : undefined;
      return () => stop?.();
    });
    // Services: a pinned deck of ten cards. Each scroll step throws the top card off to the left and the rest settle forward into the pile.
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const cards = gsap.utils.toArray<HTMLElement>('.h-dcard');
      const n = cards.length;
      if (!n) return;
      const pose = (k: number, d: number) => ({ y: d * 20, scale: 1 - d * .05, rotation: (k % 2 ? 1 : -1) * Math.min(d, 4) * 1.6, opacity: d < 4 ? 1 : 0 });
      cards.forEach((c, k) => gsap.set(c, { xPercent: -50, yPercent: -50, zIndex: n - k, ...pose(k, k) }));
      const total = n - 2 + .7;
      const tl = gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.h-svc', start: 'top top', end: '+=' + Math.round(innerHeight * (n - 1) * .55), pin: true, scrub: .6, anticipatePin: 1, invalidateOnRefresh: true, refreshPriority: 1,
        onUpdate: (self) => setDeckIdx(Math.min(n - 1, Math.round(self.progress * (n - 1)))) } });
      for (let st = 0; st < n - 1; st++) {
        tl.to(cards[st], { x: () => innerWidth * .3, y: () => -innerHeight * .95, rotation: 12, opacity: 0, duration: .7, ease: 'power2.in' }, st);
        for (let j = st + 1; j < n; j++) tl.to(cards[j], { ...pose(j, j - st - 1), duration: .7, ease: 'power2.out' }, st);
      }
      tl.to('.h-deck__bar i', { scaleX: 1, duration: total }, 0);
      return () => setDeckIdx(0);
    });
    // Core values: the studio's roll-up door covers the section; the scroll rolls it up and the three figures count as the bay opens.
    // Static (no door, final numbers) on a phone and with reduced motion.
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const slats = gsap.utils.toArray<HTMLElement>('.h-door__slat');
      const nums = gsap.utils.toArray<HTMLElement>('.h-fact [data-to]');
      const show = (el: HTMLElement, v: number) => { el.textContent = Math.round(v).toLocaleString('en-CA'); };
      const c = facts.map(() => ({ v: 0 }));
      nums.forEach((n) => show(n, 0));
      const tl = gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.h-val', start: 'top top', end: '+=' + Math.round(innerHeight * 2.4), pin: true, scrub: .6, anticipatePin: 1, invalidateOnRefresh: true, refreshPriority: 2 } });
      // Each slat rises further than the one above it, so the door folds up into the top edge like a real sectional door.
      slats.forEach((sl, i) => tl.to(sl, { y: () => -(i + 1) * sl.offsetHeight, duration: 1 }, 0));
      tl.to('.h-door__sign, .h-door__hint', { y: () => -7.5 * slats[0].offsetHeight, duration: 1 }, 0)
        .fromTo('.h-val__text, .h-val__facts', { y: 48 }, { y: 0, duration: 1 }, 0)
        .to('.h-door', { autoAlpha: 0, duration: .01 }, 1);
      facts.forEach((f, i) => tl.to(c[i], { v: f.to, duration: .55, ease: 'power2.out', onUpdate: () => show(nums[i], c[i].v) }, .7 + i * .18));
      tl.to({}, { duration: .3 });
      return () => { nums.forEach((n, i) => show(n, facts[i].to)); };
    });
    mm.add('(prefers-reduced-motion: reduce)', () => {
      // With reduced motion a film stays paused, so nudge it to paint its first frame.
      gsap.utils.toArray<HTMLVideoElement>('.h-reel video').forEach((v) => { v.preload = 'auto'; v.currentTime = .1; });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="s-page h-page">
      <section className="h-hero h-seq" aria-labelledby="h-title">
        <picture>
          <source media="(max-width:720px)" srcSet="/site/assets/frames/mobile/0000.webp" />
          <img className="h-hero__img" src="/site/assets/frames/desktop/0000.webp" alt="A dark blue sports car in the Dapper studio (AI-generated image)" fetchPriority="high" />
        </picture>
        <canvas className="h-seq__canvas" aria-hidden="true" />
        <div className="h-hero__shade" aria-hidden="true" />
        <div className="h-hero__in">
          <p className="h-hero__tag">Peterborough · Ontario</p>
          <h1 id="h-title" className="h-h1">Detailing that <em>speaks for itself.</em></h1>
          <p className="h-hero__lead">{hero.lead}</p>
          <div className="h-hero__act">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
          <p className="h-hero__trust"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b><span>{proof.customers}</span></p>
        </div>
        <div className="h-seq__scrim" aria-hidden="true" />
        <div className="h-seq__story" aria-label="What we can do to your car">
          {STORY.map((st) => (
            <div key={st.at} className={'h-story h-story--' + st.side}>
              <span className="h-story__chip"><i style={{ background: st.swatch }} aria-hidden="true" />{st.chip}</span>
              <div className="h-story__body">
                <p className="h-tag">{st.tag}</p>
                <h2 className="s-h2">{st.title} <em>{st.em}</em></h2>
                <p className="h-story__p">{st.text}</p>
              </div>
            </div>
          ))}
        </div>
        <a className="s-btn h-seq__book" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
        <div className="h-seq__bar" aria-hidden="true"><i /></div>
      </section>

      {/* H2 Core values: the promise on the left, what we detail rolling past on the right */}
      <section className="h-val h-light" aria-labelledby="h-val-title">
        <div className="h-val__text">
          <p className="h-tag">Our core values</p>
          <h2 id="h-val-title" className="s-h2" data-head>Driven by quality, <em>powered by passion.</em></h2>
          <p>{values.text}</p>
          <a className="s-btn" href="/booking"><span className="s-roll"><span>Book your detail</span><span aria-hidden="true">Book your detail</span></span><ArrowRight size={22} aria-hidden="true" /></a>
        </div>
        <ul className="h-val__facts" aria-label="Dapper in numbers">
          {facts.map((f) => (
            <li className="h-fact" key={f.label}>
              <span className="h-fact__n" aria-hidden="true">
                <b data-to={f.to}>{f.to.toLocaleString('en-CA')}</b>
                {f.suffix === 'star' ? <Star className="h-fact__star" fill="currentColor" strokeWidth={0} /> : <i>{f.suffix}</i>}
              </span>
              <span className="h-fact__l"><span className="sr-only">{f.sr}. </span><span aria-hidden="true">{f.label}</span></span>
            </li>
          ))}
        </ul>
        <div className="h-door" aria-hidden="true">
          {Array.from({ length: 8 }, (_, i) => <span className="h-door__slat" key={i} />)}
          <p className="h-door__sign">Dapper Detailing Pros<br /><em>Step inside.</em></p>
          <p className="h-door__hint">Scroll to open the bay</p>
        </div>
      </section>

      {/* H3 Services: five panels, the one you point at opens */}
      <section className="h-svc" aria-labelledby="h-svc-title">
        <div className="h-hs">
          <ul className="h-hs__track">
            <li className="h-hs__intro">
              <h2 id="h-svc-title" className="s-h2" data-head>Detailing for <em>every vehicle.</em></h2>
              <p>{servicesIntro.text}</p>
              <Link href="/services" className="h-hs__all">All services<ArrowRight size={20} aria-hidden="true" /></Link>
              <p className="h-hs__hint" aria-hidden="true">Keep scrolling<ArrowRight size={22} /></p>
            </li>
            {categories.map((c) => (
              <li className="h-hcard" key={c.slug}>
                <Link href={'/' + c.slug}>
                  <span className="h-hcard__img" role="img" aria-label={c.photo ? c.title : 'Example photo from Dapper work'}>
                    <span className="h-hcard__ph" style={{ backgroundImage: 'url(' + c.image + ')' }} />
                    {c.photo ? null : <span className="h-hcard__note">Example photo</span>}
                  </span>
                  <span className="h-hcard__body">
                    <span className="h-hcard__name">{c.title}</span>
                    <span className="h-hcard__foot">
                      {priceOf(c) ? <span className="h-hcard__price">from<b>${money(priceOf(c)!)}</b></span> : null}
                      <span className="h-hcard__go">Check now<ArrowRight size={18} aria-hidden="true" /></span>
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
          <div className="h-hs__bar" aria-hidden="true"><i /></div>
        </div>
        <div className="h-deck">
          <div className="h-deck__intro">
            <h2 className="s-h2">Detailing for <em>every vehicle.</em></h2>
            <p>{servicesIntro.text}</p>
            <div className="h-deck__now" aria-live="polite">
              <Link key={categories[deckIdx].slug} href={'/' + categories[deckIdx].slug}>
                <span className="h-deck__nowname">{categories[deckIdx].title}</span>
                {priceOf(categories[deckIdx]) ? <span className="h-deck__nowprice">from <b>${money(priceOf(categories[deckIdx])!)}</b></span> : null}
                <span className="h-deck__go">Check now<ArrowRight size={18} aria-hidden="true" /></span>
              </Link>
            </div>
            <Link href="/services" className="h-deck__all">All services<ArrowRight size={20} aria-hidden="true" /></Link>
          </div>
          <div className="h-deck__stage">
            {categories.map((c) => (
              <article className="h-dcard" key={c.slug}>
                <Link href={'/' + c.slug}>
                  <span className="h-dcard__img" role="img" aria-label={c.photo ? c.title : 'Example photo from Dapper work'} style={{ backgroundImage: 'url(' + c.image + ')' }} />
                  {c.photo ? null : <span className="h-dcard__note">Example photo</span>}
                  <span className="h-dcard__cap">
                    <span className="h-dcard__name">{c.title}</span>
                    {priceOf(c) ? <span className="h-dcard__price">from <b>${money(priceOf(c)!)}</b></span> : null}
                  </span>
                </Link>
              </article>
            ))}
          </div>
          <div className="h-deck__bar" aria-hidden="true"><i /></div>
        </div>
      </section>

      {/* H4 Partners: the dealerships we work with */}
      <section className="h-par h-light" aria-labelledby="h-par-title">
        <div className="h-par__head">
          <p className="h-tag">Our partners</p>
          <h2 id="h-par-title" className="s-h2" data-head>Trusted by <em>the dealers.</em></h2>
          <p>The dealerships we work with.</p>
        </div>
        <Dealers />
      </section>

      {/* H5 Transformations: five real films, playing while they are in view */}
      <section className="h-films" id="work" aria-labelledby="h-films-title">
        <div className="fc-sec">
          <div className="fc-sec__text">
            <h2 id="h-films-title" className="s-h2" data-head>Real results. <em>Real vehicles.</em></h2>
            <p>Every vehicle tells a story and every transformation showcases our commitment to excellence. Select a film to watch it with sound.</p>
            <Link className="s-btn" href="/portfolio"><span className="s-roll"><span>Explore portfolio</span><span aria-hidden="true">Explore portfolio</span></span><ArrowRight size={22} aria-hidden="true" /></Link>
          </div>
          <FilmCarousel clips={clips} onOpen={setFilm} />
        </div>
      </section>

      {/* H6 Why trust Dapper: six reasons, word for word from the current site */}
      <section className="h-why h-light" aria-labelledby="h-why-title">
        <div className="h-why__side">
          <p className="h-tag">Why choose us</p>
          <h2 id="h-why-title" className="s-h2" data-head>Why owners <em>trust Dapper.</em></h2>
        </div>
        <ol className="h-steps">
          <span className="h-steps__line" aria-hidden="true"><i /></span>
          {why.map((w) => (
            <li key={w.title} className="h-step">
              <span className="h-step__dot" aria-hidden="true"><Check size={20} strokeWidth={3} /></span>
              <h3>{w.title}</h3>
              <p>{w.text}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* H7 Reviews: what clients say, one at a time, in their own words */}
      <section className="h-rev" aria-labelledby="h-rev-title" onMouseEnter={() => hold(true)} onMouseLeave={() => hold(false)} onFocus={() => hold(true)} onBlur={() => hold(false)}>
        <div className="h-rev__top">
          <p className="h-tag">Testimonials</p>
          <h2 id="h-rev-title" className="s-h2" data-head>Straight from <em>the owners.</em></h2>
          <p className="h-rev__rating"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b> · {proof.customers}</p>
        </div>
        <div className="h-rev__stage">
          <figure className="h-rev__q" aria-live="polite">
            <blockquote>“{testimonials[rev].quote}”</blockquote>
            <figcaption>{testimonials[rev].name}</figcaption>
          </figure>
          <div className="h-rev__ctrl">
            <button type="button" onClick={() => goRev(rev - 1)} aria-label="Previous review"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" onClick={() => goRev(rev + 1)} aria-label="Next review"><ArrowRight size={22} aria-hidden="true" /></button>
            <span className="h-rev__bar" aria-hidden="true"><i /></span>
          </div>
        </div>
      </section>

      <CtaBand id="h-close-title" />

      <dialog ref={box} className="h-film" aria-label={film !== null ? 'Transformation ' + (film + 1) + ' of ' + clips.length : 'Film'} onClose={() => setFilm(null)}
        onClick={(e) => { if (e.target === e.currentTarget) setFilm(null); }}
        onKeyDown={(e) => { if (e.key === 'ArrowRight') stepFilm(1); if (e.key === 'ArrowLeft') stepFilm(-1); }}>
        {film !== null && (
          <div className="h-film__in">
            <button type="button" className="h-film__btn h-film__close" onClick={() => setFilm(null)} aria-label="Close"><X size={22} aria-hidden="true" /></button>
            <video key={clips[film]} src={clips[film]} autoPlay loop playsInline controls preload="auto" ref={(el) => { if (el && el.paused) void el.play().catch(() => {}); }} />
            <p className="h-film__cap">Transformation {film + 1}<span>{film + 1} / {clips.length}</span></p>
            <button type="button" className="h-film__btn h-film__prev" onClick={() => stepFilm(-1)} aria-label="Previous film"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" className="h-film__btn h-film__next" onClick={() => stepFilm(1)} aria-label="Next film"><ArrowRight size={22} aria-hidden="true" /></button>
          </div>
        )}
      </dialog>
    </div>
  );
}

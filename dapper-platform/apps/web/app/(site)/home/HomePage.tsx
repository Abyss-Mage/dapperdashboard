'use client';
import Link from 'next/link';
import { ArrowLeft, ArrowRight, Play, Star, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { contact, cta, hero, partners, proof, servicesIntro, services, testimonials, transformations, values, why } from '../dapper';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Home, built section by section with the owner's approval of each, in the darker "hard" direction.
   H1: the studio, the headline, the booking call (to become a scroll-driven video sequence when the footage arrives). H2: core values. H3: the five priced services. H4: commercial partners. H5: transformation films. H6: why owners trust Dapper. H7: reviews. H8: closing booking band. */

const words = ['Sedan detailing', 'SUV detailing', 'Pickup detailing', 'Ceramic coating', 'Paint protection film', 'Aircraft detailing', 'Boat detailing'];

export default function HomePage() {
  const root = useRef<HTMLDivElement>(null);
  const [on, setOn] = useState(0);
  const [rev, setRev] = useState(0);
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
  // A new review slides up into place (instant with reduced motion).
  const first = useRef(true);
  useEffect(() => {
    if (first.current) { first.current = false; return; }
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    gsap.fromTo('.h-rev__q', { y: 28, opacity: 0 }, { y: 0, opacity: 1, duration: .7, ease: 'expo.out', overwrite: true });
  }, [rev]);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.h-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.2, stagger: .08, ease: 'expo.out', delay: .2 }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .fromTo('.h-hero__img', { scale: 1.18 }, { scale: 1, duration: 2.4, ease: 'power3.out' }, 0)
        .from('.h-hero__tag, .h-hero__lead, .h-hero__act > *, .h-hero__trust', { y: 22, opacity: 0, duration: .9, stagger: .08 }, .6);
      // The photo drifts slower than the page as it leaves.
      gsap.to('.h-hero__img', { yPercent: 12, ease: 'none', scrollTrigger: { trigger: '.h-hero', start: 'top top', end: 'bottom top', scrub: true } });
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.05, stagger: .07, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      gsap.from('.h-val__text > p, .h-val__text > a', { y: 24, opacity: 0, duration: .9, stagger: .1, ease: 'expo.out', scrollTrigger: { trigger: '.h-val', start: 'top 70%', once: true } });
      gsap.from('.h-svc__panel', { y: 60, opacity: 0, duration: 1, stagger: .09, ease: 'expo.out', scrollTrigger: { trigger: '.h-svc__row', start: 'top 85%', once: true } });
      gsap.from('.h-par__grid li', { y: 36, opacity: 0, duration: .9, stagger: .09, ease: 'expo.out', scrollTrigger: { trigger: '.h-par__grid', start: 'top 88%', once: true } });
      ScrollTrigger.batch('.h-why__item', { start: 'top 90%', once: true,
        onEnter: (els) => gsap.from(els, { y: 40, opacity: 0, duration: .9, stagger: .08, ease: 'expo.out', overwrite: true }) });
      // Image mask on scroll: the frame slides in from the right while the photo slides the other way, so the picture holds still.
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.s-close__media', start: 'top 88%', end: 'top 30%', scrub: true } })
        .fromTo('.s-close__mask', { xPercent: 100 }, { xPercent: 0 })
        .fromTo('.s-close__mask img', { xPercent: -100 }, { xPercent: 0 }, 0);
      // The films play only while they are on screen.
      const io = new IntersectionObserver((es) => es.forEach((e) => { const v = e.target as HTMLVideoElement; if (e.isIntersecting) void v.play().catch(() => {}); else v.pause(); }), { threshold: .35 });
      gsap.utils.toArray<HTMLVideoElement>('.h-reel video').forEach((v) => io.observe(v));
      gsap.from('.h-reel', { y: 70, opacity: 0, duration: 1, stagger: .09, ease: 'expo.out', scrollTrigger: { trigger: '.h-films__reels', start: 'top 88%', once: true } });
      // Only the word nearest the middle of the screen is lit.
      const items = gsap.utils.toArray<HTMLElement>('.h-val__words li');
      ScrollTrigger.create({ trigger: '.h-val__words', start: 'top bottom', end: 'bottom top', onUpdate: () => {
        const mid = innerHeight * .5;
        let best = 0, dist = Infinity;
        items.forEach((li, i) => { const r = li.getBoundingClientRect(); const d = Math.abs(r.top + r.height / 2 - mid); if (d < dist) { dist = d; best = i; } });
        items.forEach((li, i) => li.classList.toggle('is-on', i === best));
      } });
    });
    mm.add('(prefers-reduced-motion: reduce)', () => {
      document.querySelector('.h-val__words li')?.classList.add('is-on');
      // With reduced motion a film stays paused, so nudge it to paint its first frame.
      gsap.utils.toArray<HTMLVideoElement>('.h-reel video').forEach((v) => { v.preload = 'auto'; v.currentTime = .1; });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="s-page h-page">
      <section className="h-hero" aria-labelledby="h-title">
        <picture>
          <source media="(max-width:720px)" srcSet="/site/assets/hero-mobile.webp" />
          <img className="h-hero__img" src="/site/assets/hero.webp" alt="A black classic car with its headlights on, parked in the Dapper studio" fetchPriority="high" />
        </picture>
        <div className="h-hero__shade" aria-hidden="true" />
        <div className="h-hero__in">
          <p className="h-hero__tag">Peterborough · 660 The Kingsway</p>
          <h1 id="h-title" className="h-h1">Detailing that <em>speaks for itself.</em></h1>
          <p className="h-hero__lead">{hero.lead}</p>
          <div className="h-hero__act">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
          <p className="h-hero__trust"><span className="v-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={18} fill="currentColor" strokeWidth={0} />)}</span><b>{proof.rating}</b><span>{proof.customers}</span></p>
        </div>
      </section>

      {/* H2 Core values: the promise on the left, what we detail rolling past on the right */}
      <section className="h-val h-light" aria-labelledby="h-val-title">
        <div className="h-val__text">
          <p className="h-tag">Our core values</p>
          <h2 id="h-val-title" className="s-h2" data-head>Driven by quality, <em>powered by passion.</em></h2>
          <p>{values.text}</p>
          <a className="s-btn" href="/booking"><span className="s-roll"><span>Book your detail</span><span aria-hidden="true">Book your detail</span></span><ArrowRight size={22} aria-hidden="true" /></a>
        </div>
        <ul className="h-val__words" aria-label="What we detail">
          {words.map((w) => <li key={w}>{w}</li>)}
        </ul>
      </section>

      {/* H3 Services: five panels, the one you point at opens */}
      <section className="h-svc" aria-labelledby="h-svc-title">
        <div className="h-svc__head">
          <h2 id="h-svc-title" className="s-h2" data-head>Detailing for <em>every vehicle.</em></h2>
          <p>{servicesIntro.text}</p>
        </div>
        <ul className="h-svc__row">
          {services.map((v, i) => (
            <li key={v.slug} className={'h-svc__panel' + (i === on ? ' is-on' : '')} onMouseEnter={() => setOn(i)}>
              <Link href={'/' + v.slug} onFocus={() => setOn(i)}>
                <span className="h-svc__img" style={{ backgroundImage: 'url(' + v.image + ')' }} role="img" aria-label={v.name} />
                <span className="h-svc__n" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span>
                <span className="h-svc__name">{v.name}</span>
                <span className="h-svc__more">
                  {v.from ? <span className="h-svc__price">Starting from <b>${v.from}</b></span> : null}
                  <span className="h-svc__text">{v.text}</span>
                  <span className="h-svc__go">Check now<ArrowRight size={20} aria-hidden="true" /></span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
        <Link href="/services" className="h-svc__all">All ten vehicle types<ArrowRight size={20} aria-hidden="true" /></Link>
      </section>

      {/* H4 Partners: the dealerships we work with */}
      <section className="h-par h-light" aria-labelledby="h-par-title">
        <div className="h-par__head">
          <p className="h-tag">Our partners</p>
          <h2 id="h-par-title" className="s-h2" data-head>Trusted by <em>the dealers.</em></h2>
        </div>
        <ul className="h-par__grid">
          {partners.map((p) => (
            <li key={p.name}><img className={p.name.includes('McGee') ? 'is-crop' : undefined} src={p.src} alt={p.name} loading="lazy" /></li>
          ))}
        </ul>
      </section>

      {/* H5 Transformations: five real films, playing while they are in view */}
      <section className="h-films" id="work" aria-labelledby="h-films-title">
        <div className="h-films__head">
          <h2 id="h-films-title" className="s-h2" data-head>Real results. <em>Real vehicles.</em></h2>
          <div>
            <p>Every vehicle tells a story and every transformation showcases our commitment to excellence. Select a film to watch it with sound.</p>
            <Link className="s-btn" href="/portfolio"><span className="s-roll"><span>Explore portfolio</span><span aria-hidden="true">Explore portfolio</span></span><ArrowRight size={22} aria-hidden="true" /></Link>
          </div>
        </div>
        <ul className="h-films__reels">
          {clips.map((c, i) => (
            <li key={c}>
              <button type="button" className="h-reel" onClick={() => setFilm(i)} aria-label={'Watch transformation ' + (i + 1) + ' of ' + clips.length}>
                <video src={c} muted loop playsInline preload="metadata" />
                <span className="h-reel__n" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span>
                <span className="h-reel__play" aria-hidden="true"><Play size={20} fill="currentColor" /></span>
              </button>
            </li>
          ))}
        </ul>
      </section>

      {/* H6 Why trust Dapper: six reasons, word for word from the current site */}
      <section className="h-why h-light" aria-labelledby="h-why-title">
        <p className="h-tag">Why choose us</p>
        <h2 id="h-why-title" className="s-h2" data-head>Why owners <em>trust Dapper.</em></h2>
        <ul className="h-why__grid">
          {why.map((w, i) => (
            <li key={w.title} className="h-why__item">
              <span className="h-why__n" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span>
              <h3>{w.title}</h3>
              <p>{w.text}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* H7 Reviews: what clients say, one at a time, in their own words */}
      <section className="h-rev h-light" aria-labelledby="h-rev-title">
        <div className="h-rev__top">
          <div>
            <p className="h-tag">Testimonials</p>
            <h2 id="h-rev-title" className="s-h2" data-head>Straight from <em>the owners.</em></h2>
          </div>
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
          </div>
        </div>
        <ul className="h-rev__dots" aria-label="Reviews">
          {testimonials.map((t, i) => <li key={t.name}><button type="button" onClick={() => goRev(i)} aria-label={'Review ' + (i + 1) + ' of ' + nRev + ', ' + t.name} aria-current={i === rev ? 'true' : undefined} /></li>)}
        </ul>
      </section>

      {/* H8 Closing band: the booking call beside a photo that slides into its frame */}
      <section className="s-close" aria-labelledby="h-close-title">
        <div className="s-close__text">
          <h2 id="h-close-title" className="s-h2" data-head>Ready to give your car <em>the care it deserves?</em></h2>
          <p className="s-close__lede">{cta.text}</p>
          <div className="s-close__actions">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </div>
        <div className="s-close__media">
          <div className="s-close__mask"><img src="/site/assets/portfolio/portfolio-06.jpg" alt="A vehicle finished by the Dapper team" loading="lazy" /></div>
        </div>
      </section>

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

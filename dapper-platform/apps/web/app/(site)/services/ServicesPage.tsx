'use client';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { categories } from '../content';
import { contact, cta, processSteps } from '../dapper';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Services, built section by section with the owner's approval of each, in the darker "hard" direction the owner chose for it:
   black ground, heavy condensed capitals, blunt words. S1: pick your ride, the ten vehicle types. S2: how we do it, a card stack. S3: the closing booking band. */

// Where the cards sit in the stack: slot 0 is the card on top; each one behind is smaller, higher and further right.
const slot = (s: number) => ({ x: s * 30, y: -s * 24, scale: 1 - s * .05, opacity: 1 - s * .14, zIndex: 10 - s });
const pad = (n: number) => String(n + 1).padStart(2, '0');

export default function ServicesPage() {
  const root = useRef<HTMLDivElement>(null);
  const [cur, setCur] = useState(0);
  const prev = useRef(0);
  const n = processSteps.length;
  const go = (to: number) => setCur(((to % n) + n) % n);

  useGSAP(() => {
    // The stack is laid out first, so the entrance below ends exactly where the stack sits.
    gsap.utils.toArray<HTMLElement>('.s-card').forEach((el, i) => gsap.set(el, slot(i)));
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.s-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.1, stagger: .09, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.s-hero__note', { y: 20, opacity: 0, duration: .9 }, .45)
        .fromTo('.s-tile', { clipPath: 'inset(100% 0 0 0)' }, { clipPath: 'inset(0% 0 0 0)', duration: 1, stagger: .06 }, .3)
        .from('.s-tile__img', { scale: 1.2, duration: 1.5, stagger: .06, ease: 'power3.out' }, .3);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.05, stagger: .07, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      // Image mask on scroll: the frame slides in from the right while the photo slides the other way, so the picture holds still.
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.s-close__media', start: 'top 88%', end: 'top 30%', scrub: true } })
        .fromTo('.s-close__mask', { xPercent: 100 }, { xPercent: 0 })
        .fromTo('.s-close__mask img', { xPercent: -100 }, { xPercent: 0 }, 0);
      gsap.from('.s-card', { y: 60, opacity: 0, duration: 1, stagger: .1, ease: 'expo.out', scrollTrigger: { trigger: '.s-how__stack', start: 'top 85%', once: true } });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  // Card stack (demo 8): the card on top slides out to the left and drops to the back; the rest step forward. Going back reverses it.
  useEffect(() => {
    const el = root.current;
    if (!el || prev.current === cur) { prev.current = cur; return; }
    const cards = gsap.utils.toArray<HTMLElement>('.s-card', el);
    const quick = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const forward = ((cur - prev.current + n) % n) <= n / 2;
    const tl = gsap.timeline({ defaults: { duration: quick ? 0 : .6, ease: 'power3.inOut' } });
    cards.forEach((card, i) => {
      const from = (i - prev.current + n) % n;
      const to = (i - cur + n) % n;
      if (from === to) return;
      if (forward && from === 0) {
        // the old top card leaves, then settles at the back
        tl.to(card, { x: -(card.offsetWidth * 1.05), rotation: -7, duration: quick ? 0 : .38, ease: 'power2.in' }, 0)
          .set(card, { zIndex: 1 }, quick ? 0 : .38)
          .to(card, { ...slot(to), rotation: 0, duration: quick ? 0 : .55 }, quick ? 0 : .38);
      } else if (!forward && to === 0) {
        // going back: the card from the back flies in over the top
        tl.set(card, { zIndex: 20 }, 0).to(card, { x: -(card.offsetWidth * 1.05), rotation: -7, duration: quick ? 0 : .01 }, 0)
          .to(card, { ...slot(0), rotation: 0, duration: quick ? 0 : .6, ease: 'power3.out' }, quick ? 0 : .02);
      } else {
        tl.to(card, slot(to), 0);
      }
    });
    gsap.fromTo('.s-how__copy', { y: 16, opacity: 0 }, { y: 0, opacity: 1, duration: quick ? 0 : .6, ease: 'expo.out', overwrite: true });
    prev.current = cur;
    return () => { tl.progress(1); };
  }, [cur, n]);

  return (
    <div ref={root} className="s-page">
      {/* S1 Pick your ride: the ten vehicle types, each a way into its own page */}
      <section className="s-hero" aria-labelledby="s-title">
        <div className="s-hero__top">
          <h1 id="s-title" className="s-h1">Pick your <em>ride.</em></h1>
          <p className="s-hero__note"><b>No two vehicles are the same.</b> Choose yours. We’ll show you the care that fits it.</p>
        </div>
        <ul className="s-grid">
          {categories.map((c) => (
            <li key={c.slug}>
              <Link href={'/' + c.slug} className="s-tile">
                <span className="s-tile__img" style={{ backgroundImage: 'url(' + c.image + ')' }} role="img" aria-label={c.photo ? c.title : 'Vehicle detailed by Dapper'} />
                {c.from ? <span className="s-tile__price" aria-label={'Starting from $' + c.from}>${c.from}</span> : null}
                <span className="s-tile__name">{c.short}</span>
                {c.photo ? null : <span className="s-tile__tag">Studio photo</span>}
                <span className="s-tile__sub">{c.key === 'ceramic' ? 'Coating' : c.key === 'ppf' ? 'Paint protection film' : c.from ? 'Detailing' : 'Ask the team'}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      {/* S2 How we do it: four steps as a stack of cards; the top card is the current step */}
      <section className="s-how" aria-labelledby="s-how-title">
        <div className="s-how__text">
          <h2 id="s-how-title" className="s-h2" data-head>How we <em>do it.</em></h2>
          <ol className="s-how__steps" aria-label="Steps">
            {processSteps.map((p, i) => (
              <li key={p.title}><button type="button" className={i === cur ? 'is-on' : undefined} aria-current={i === cur ? 'step' : undefined} onClick={() => go(i)}><span>{pad(i)}</span>{p.title}</button></li>
            ))}
          </ol>
          <div className="s-how__copy" aria-live="polite">
            <h3>{processSteps[cur].title}</h3>
            <p>{processSteps[cur].text}</p>
          </div>
          <div className="s-how__ctrl">
            <button type="button" onClick={() => go(cur - 1)} aria-label="Previous step"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" onClick={() => go(cur + 1)} aria-label="Next step"><ArrowRight size={22} aria-hidden="true" /></button>
            <span aria-hidden="true">{pad(cur)} / {pad(n - 1)}</span>
          </div>
        </div>
        <div className="s-how__stack" aria-hidden="true">
          {processSteps.map((p, i) => (
            <div key={p.title} className="s-card" onClick={() => { if (i === cur) go(cur + 1); }}>
              <span className="s-card__img" style={{ backgroundImage: 'url(' + p.image + ')' }} />
              <span className="s-card__n">{pad(i)}</span>
              <span className="s-card__t">{p.title}</span>
            </div>
          ))}
        </div>
      </section>

      {/* S3 Closing band: the booking call beside a photo that slides into its frame */}
      <section className="s-close" aria-labelledby="s-close-title">
        <div className="s-close__text">
          <h2 id="s-close-title" className="s-h2" data-head>Ready to give your car <em>the care it deserves?</em></h2>
          <p className="s-close__lede">{cta.text}</p>
          <div className="s-close__actions">
            <a className="s-btn" href="/booking"><span className="s-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={22} aria-hidden="true" /></a>
            <a className="s-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </div>
        <div className="s-close__media">
          <div className="s-close__mask"><img src="/site/assets/portfolio/portfolio-03.jpg" alt="A black Ford F-150 pickup parked beside a house" loading="lazy" /></div>
        </div>
      </section>
    </div>
  );
}

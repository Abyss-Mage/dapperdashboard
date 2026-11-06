'use client';
import { ScrollRow } from '../_components/ScrollRow';
import { CtaBand } from '../_components/CtaBand';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { categories } from '../content';
import { contact, cta, processSteps, testimonials, why } from '../dapper';
import { Dealers } from '../_components/Dealers';
import { money, useLiveFrom } from '../_components/useLiveFrom';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Services, built section by section with the owner's approval of each, in the darker "hard" direction the owner chose for it.
   S1: pick your ride, the ten vehicle types. S2: how we do it, pinned: scrolling moves through the four steps, each with the
   screen of the real system it happens in. S3: the Dapper standard. S4: what customers say. S5: the dealerships. S6: the booking call. */

const Roll = ({ children }: { children: string }) => <span className="s-roll"><span>{children}</span><span aria-hidden="true">{children}</span></span>;

export default function ServicesPage() {
  const live = useLiveFrom();
  const root = useRef<HTMLDivElement>(null);
  const [cur, setCur] = useState(0);
  const prev = useRef(0);
  const pin = useRef<ScrollTrigger | null>(null);
  const z = useRef(2);
  const n = processSteps.length;
  const go = (to: number) => {
    const next = ((to % n) + n) % n;
    const st = pin.current;
    // While the section is pinned, a step is reached by scrolling to its share of the pin; otherwise the step simply changes.
    if (st && st.isActive) window.scrollTo({ top: st.start + ((next + 0.5) / n) * (st.end - st.start), behavior: 'smooth' });
    else setCur(next);
  };

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.s-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .05, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.s-hero__note', { y: 20, opacity: 0, duration: .55 }, .3)
        .fromTo('.s-tile', { clipPath: 'inset(100% 0 0 0)' }, { clipPath: 'inset(0% 0 0 0)', duration: .8, stagger: .05 }, .2)
        .from('.s-tile__img', { scale: 1.2, duration: 1.2, stagger: .05, ease: 'power3.out' }, .2);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .04, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      ScrollTrigger.batch('[data-in]', { start: 'top 92%', once: true, onEnter: (els) => gsap.from(els, { y: 24, opacity: 0, duration: .55, stagger: .06, ease: 'expo.out', overwrite: true }) });
    });
    // How we do it: on a desktop screen the section pins and each quarter of the scroll is one step.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const sec = root.current!.querySelector<HTMLElement>('.s-how')!;
      root.current!.classList.add('is-how-pinned');
      pin.current = ScrollTrigger.create({ trigger: sec, start: 'top top', end: () => '+=' + n * 60 + '%', pin: true, invalidateOnRefresh: true, refreshPriority: 1,
        onUpdate: (self) => setCur(Math.min(n - 1, Math.floor(self.progress * n))) });
      return () => { pin.current = null; root.current?.classList.remove('is-how-pinned'); };
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  // The step's screen opens over the last one from the top; the copy beside it fades up.
  useEffect(() => {
    const el = root.current;
    if (!el || prev.current === cur) { prev.current = cur; return; }
    const quick = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const shot = el.querySelectorAll<HTMLElement>('.s-shot')[cur];
    if (shot) {
      shot.style.zIndex = String(++z.current);
      gsap.fromTo(shot, { clipPath: 'inset(0 0 100% 0)' }, { clipPath: 'inset(0 0 0% 0)', duration: quick ? 0 : .55, ease: 'expo.out', overwrite: true });
      gsap.fromTo(shot.querySelector('.s-shot__img'), { scale: 1.12 }, { scale: 1, duration: quick ? 0 : .9, ease: 'expo.out', overwrite: true });
    }
    gsap.fromTo('.s-how__copy', { y: 14, opacity: 0 }, { y: 0, opacity: 1, duration: quick ? 0 : .45, ease: 'expo.out', overwrite: true });
    prev.current = cur;
  }, [cur]);

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
                {(live[c.key] ?? c.from) ? <span className="s-tile__price" aria-label={'Starting from $' + money((live[c.key] ?? c.from)!)}>${money((live[c.key] ?? c.from)!)}</span> : null}
                <span className="s-tile__name">{c.short}</span>
                {c.photo ? null : <span className="s-tile__tag">Studio photo</span>}
                <span className="s-tile__sub">{c.key === 'ceramic' ? 'Coating' : c.key === 'ppf' ? 'Paint protection film' : (live[c.key] ?? c.from) ? 'Detailing' : 'Ask the team'}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      {/* S2 How we do it: scroll (or press a step) and the screen beside it changes to the one that step happens in */}
      <section className="s-how s-light" aria-labelledby="s-how-title">
        <div className="s-how__text">
          <h2 id="s-how-title" className="s-h2" data-head>How we <em>do it.</em></h2>
          <ol className="s-how__steps" aria-label="Steps">
            {processSteps.map((p, i) => (
              <li key={p.title}><button type="button" className={i === cur ? 'is-on' : undefined} aria-current={i === cur ? 'step' : undefined} onClick={() => go(i)}>{p.title}</button></li>
            ))}
          </ol>
          <div className="s-how__copy" aria-live="polite">
            <h3>{processSteps[cur].title}</h3>
            <p>{processSteps[cur].text}</p>
          </div>
          <div className="s-how__ctrl">
            <button type="button" onClick={() => go(cur - 1)} aria-label="Previous step"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" onClick={() => go(cur + 1)} aria-label="Next step"><ArrowRight size={22} aria-hidden="true" /></button>
            <span aria-hidden="true">Step {cur + 1} of {n}</span>
          </div>
        </div>
        <div className="s-how__stage" aria-hidden="true">
          {processSteps.map((p, i) => (
            <div key={p.title} className="s-shot" style={{ zIndex: i === 0 ? 2 : 1 }} data-on={i === cur || undefined}>
              <span className="s-shot__img" style={{ backgroundImage: 'url(' + p.image + ')' }} />
              {p.shot && <img className="s-shot__real" src={p.shot} alt="" loading="lazy" />}
            </div>
          ))}
        </div>
      </section>

      {/* S3 The Dapper standard: six reasons, set as a list beside the heading */}
      <section className="s-std" aria-labelledby="s-std-title">
        <h2 id="s-std-title" className="s-h2" data-head>The Dapper <em>standard.</em></h2>
        <ul className="s-std__list">
          {why.map((w) => (
            <li key={w.title} data-in>
              <h3>{w.title}</h3>
              <p>{w.text}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* S4 What customers say: a row to swipe or scroll */}
      <section className="s-rev s-light" aria-labelledby="s-rev-title">
        <h2 id="s-rev-title" className="s-h2" data-head>What our customers <em>say.</em></h2>
        <ScrollRow label="Customer reviews" tone="light" auto={4500} className="s-rev__row">
          {testimonials.map((t) => (
            <li key={t.name}>
              <blockquote>{t.quote}</blockquote>
              <p>{t.name}</p>
            </li>
          ))}
        </ScrollRow>
      </section>

      {/* S5 The dealerships Dapper works with */}
      <section className="s-par" aria-labelledby="s-par-title">
        <h2 id="s-par-title" className="s-h2" data-head>Dealerships that <em>trust us.</em></h2>
        <Dealers />
      </section>

      <CtaBand id="s-close-title" />
    </div>
  );
}

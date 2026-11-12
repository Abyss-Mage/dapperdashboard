'use client';
import { useRef } from 'react';
import { ArrowRight } from 'lucide-react';
import { timeline } from '../content';
import { contact, founderPhotos, values, why } from '../dapper';
import { Dealers } from '../_components/Dealers';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* About, built section by section with the owner's approval of each. Same paper system as Contact (../paper.css).
   Hero: one full-bleed photograph with the headline over it. Founders: two portraits. Why: six cards that stack, each sliding over the last.
   Partners: the dealerships. Closing band: type only. */

const founders = [
  { role: 'CEO', title: 'Chief Executive Officer', name: 'Ayush Kumar Singh', initials: 'AS', text: 'Began detailing in 2022, alongside his studies in Canada.' },
  { role: 'CTO', title: 'Chief Technology Officer', name: 'Mohan Preet Singh Virk', initials: 'MV', text: 'Joined in 2023, bringing technology and digital systems into the business.' },
];

const words = ['Sedan detailing', 'SUV detailing', 'Pickup detailing', 'Ceramic coating', 'Paint protection film', 'Aircraft detailing', 'Boat detailing'];
const Roll = ({ children }: { children: string }) => <span className="c-roll"><span>{children}</span><span aria-hidden="true">{children}</span></span>;

export default function AboutPage() {
  const root = useRef<HTMLDivElement>(null);
  const wordRef = useRef<HTMLSpanElement>(null);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.c-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .04, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.ah__img', { scale: 1.14, duration: 1.3, ease: 'power3.out' }, 0)
        .from('.ah__aside > *', { y: 20, opacity: 0, duration: .55, stagger: .06 }, .3);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.lines, { yPercent: 110, duration: .6, stagger: .045, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      // Animate text: each role word arrives character by character from the right.
      gsap.utils.toArray<HTMLElement>('.af-role').forEach((el) => {
        const s = SplitText.create(el, { type: 'chars', smartWrap: true, aria: 'none' });
        gsap.from(s.chars, { x: 60, opacity: 0, duration: .5, ease: 'power4', stagger: .035, scrollTrigger: { trigger: el, start: 'top 88%', once: true } });
      });
      ScrollTrigger.batch('[data-in]', { start: 'top 92%', once: true, onEnter: (els) => gsap.from(els, { y: 24, opacity: 0, duration: .55, stagger: .05, ease: 'expo.out', overwrite: true }) });
      // The founders' portraits open upward as they arrive.
      // The picture wipes in, not its frame: the frame keeps the cut corner.
      gsap.utils.toArray<HTMLElement>('.af-frame > *').forEach((el) => gsap.fromTo(el, { clipPath: 'inset(100% 0 0 0)' }, { clipPath: 'inset(0% 0 0 0)', duration: .9, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }));

      // Text replacement: the service word morphs into the next one while the band is on screen.
      const word = wordRef.current!;
      let i = 0;
      const call = gsap.delayedCall(1.2, () => {
        i = (i + 1) % words.length;
        gsap.to(word, { duration: .45, ease: 'none', text: { value: words[i], type: 'diff' }, onComplete: () => { call.restart(true); } });
      }).pause();
      ScrollTrigger.create({ trigger: '.a-values', start: 'top 75%', end: 'bottom 25%', onToggle: (self) => (self.isActive ? call.restart(true) : call.pause()) });
      return () => { call.kill(); word.textContent = words[0]; };
    });

    // Why-trust cards: the one underneath shrinks back and dims as the next slides up over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const cards = gsap.utils.toArray<HTMLElement>('.aw-card');
      cards.forEach((card, i) => {
        if (i < cards.length - 1) gsap.to(card, { scale: .94, '--dim': .55, ease: 'none', scrollTrigger: { trigger: cards[i + 1], start: 'top 92%', end: 'top 28%', scrub: true } });
      });
    });

    // (The pin's refreshPriority is 1 so it refreshes first: higher numbers go first, and every trigger below it must measure
    // the page with its scroll space already added.)
    // Horizontal gallery: the story section pins and its strip slides left, a red line filling along the bottom. Without motion
    // the five years simply stack. Phones get the same sideways strip.
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const el = root.current!;
      const section = el.querySelector<HTMLElement>('.a-story')!;
      const strip = el.querySelector<HTMLElement>('.a-story__strip')!;
      const dist = () => Math.max(0, strip.scrollWidth - innerWidth);
      el.classList.add('is-pinned');
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: section, start: 'top top', end: () => '+=' + dist(), pin: true, scrub: .6, invalidateOnRefresh: true, refreshPriority: 1 } })
        .to(strip, { x: () => -dist() }, 0);
      return () => el.classList.remove('is-pinned');
    });
    // Pinned panel with overscroll (as on Contact): the hero stays put and settles back as the story section slides over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const hero = root.current!.querySelector<HTMLElement>('.c-hero')!;
      gsap.timeline({ scrollTrigger: { trigger: hero, start: () => (hero.offsetHeight > innerHeight ? 'bottom bottom' : 'top top'), end: () => '+=' + innerHeight, pin: true, pinSpacing: false, scrub: true, invalidateOnRefresh: true, refreshPriority: 2 } })
        .fromTo('.c-hero__stage', { scale: 1, opacity: 1, transformOrigin: '50% 30%' }, { scale: .92, opacity: .35, duration: .9, ease: 'none' })
        .to('.c-hero__stage', { opacity: 0, duration: .1, ease: 'none' });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="c-page a-page">
      {/* A1 Hero: one photograph, edge to edge, with the headline over it */}
      <section className="c-hero ah" aria-labelledby="a-title">
        <div className="c-hero__stage ah__stage">
          <img className="ah__img" src="/site/assets/portfolio/portfolio-01.jpg" alt="A black Range Rover at sunset after detailing" fetchPriority="high" />
          <div className="ah__scrim" aria-hidden="true" />
          <div className="ah__copy">
            <h1 id="a-title" className="c-h1 a-h1">Good people.<br /><em>Considered care.</em></h1>
            <div className="ah__aside">
              <p>Two childhood friends started Dapper as a mobile detailing business in 2022. Since 2026 it has had its own studio at {contact.street}, Peterborough. The care behind every vehicle has not changed.</p>
              <a className="c-btn c-btn--red" href="/book-appointment"><Roll>Book your detail</Roll><ArrowRight size={20} aria-hidden="true" /></a>
            </div>
          </div>
        </div>
      </section>

      {/* A2 Our story: five years on a strip that scrolls sideways while the page holds still */}
      <div className="c-after">
        <section className="a-story" aria-labelledby="a-story-title">
          <div className="a-story__strip">
            <div className="a-story__head">
              <h2 id="a-story-title" data-head>A journey worth <em>caring about.</em></h2>
              <p className="c-sub">Five years, from mobile work to a studio of our own.</p>
            </div>
            <ol className="a-story__items">
              {timeline.map((t) => (
                <li key={t.year} className="a-story__item">
                  <time className="a-story__year" dateTime={t.year}>{t.year}</time>
                  <h3>{t.title}</h3>
                  <p>{t.text}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        {/* A3 The founders: a portrait each, the role set large beside the name */}
        <section className="af" aria-labelledby="a-founders-title">
          <h2 id="a-founders-title" data-head>Meet the <em>founders.</em></h2>
          <div className="af-grid">
            {founders.map((f) => (
              <article key={f.name} className="af-card">
                <div className="af-frame">
                  {founderPhotos[f.name]
                    ? <img src={founderPhotos[f.name]} alt={f.name + ', ' + f.title} loading="lazy" />
                    : <span className="af-mono" aria-hidden="true">{f.initials}</span>}
                </div>
                <div className="af-text">
                  <p className="af-role" aria-label={f.title}><span aria-hidden="true">{f.role}</span></p>
                  <h3 data-in>{f.name}</h3>
                  <p className="c-muted" data-in>{f.text}</p>
                </div>
              </article>
            ))}
          </div>
        </section>

        {/* A4 Core values: an ink band; the service name in red morphs from one to the next */}
        <section className="a-values" aria-labelledby="a-values-title">
          <h2 id="a-values-title" data-head>{values.heading.replace(/, powered by passion$/, '')}, powered by <em>passion.</em></h2>
          <p className="a-values__word" aria-hidden="true"><span ref={wordRef}>{words[0]}</span></p>
          <ul className="a-values__list" aria-label="What we detail">{words.map((w) => <li key={w}>{w}</li>)}</ul>
          <div className="a-values__foot">
            <p data-in>{values.text}</p>
            <a className="c-btn c-btn--red" href="/book-appointment" data-in><Roll>Book your detail</Roll><ArrowRight size={20} aria-hidden="true" /></a>
          </div>
        </section>

        {/* A5 Why vehicle owners trust Dapper: six cards that stack, each sliding up over the one before */}
        <section className="aw" aria-labelledby="a-why-title">
          <h2 id="a-why-title" data-head>Why vehicle owners trust <em>Dapper.</em></h2>
          <div className="aw-stack">
            {why.map((w, i) => (
              <article key={w.title} className="aw-card" style={{ ['--i' as string]: i }}>
                <h3>{w.title}</h3>
                <p>{w.text}</p>
              </article>
            ))}
          </div>
        </section>

        {/* A6 Partners: the dealerships Dapper works with */}
        <section className="a-partners" aria-labelledby="a-partners-title">
          <h2 id="a-partners-title" data-head>The dealerships that <em>trust us</em> with their cars.</h2>
          <Dealers />
        </section>

        {/* A7 Closing band: the booking call, in type */}
        <section className="a-close" aria-labelledby="a-close-title">
          <h2 id="a-close-title" data-head>Ready for <em>precise detailing?</em></h2>
          <p className="c-muted a-close__lede" data-in>Choose mobile service or at-shop detailing and pick your date and time in seconds.</p>
          <div className="a-close__actions" data-in>
            <a className="c-btn c-btn--red" href="/book-appointment"><Roll>Book now</Roll><ArrowRight size={20} aria-hidden="true" /></a>
            <a className="a-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
          </div>
        </section>
      </div>
    </div>
  );
}

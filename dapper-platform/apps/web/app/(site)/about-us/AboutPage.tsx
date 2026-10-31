'use client';
import { useRef } from 'react';
import { ArrowRight } from 'lucide-react';
import { timeline } from '../content';
import { contact, partners, values, why } from '../dapper';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* About, built section by section with the owner's approval of each. Same paper system as Contact (../paper.css). Section A1: the hero. */

const founders = [
  { role: 'CEO', title: 'Chief Executive Officer', name: 'Ayush Kumar Singh', text: 'Began detailing in 2022, alongside his studies in Canada.' },
  { role: 'CTO', title: 'Chief Technology Officer', name: 'Mohan Preet Singh Virk', text: 'Joined in 2023, bringing technology and digital systems into the business.' },
];

const words = ['Sedan detailing', 'SUV detailing', 'Pickup detailing', 'Ceramic coating', 'Paint protection film', 'Aircraft detailing', 'Boat detailing'];
const INK = '#121212';
const MUTED = '#8a8780';
const Roll = ({ children }: { children: string }) => <span className="c-roll"><span>{children}</span><span aria-hidden="true">{children}</span></span>;

export default function AboutPage() {
  const root = useRef<HTMLDivElement>(null);
  const wordRef = useRef<HTMLSpanElement>(null);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.c-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.1, stagger: .07, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .fromTo('.c-hero__photo', { clipPath: 'inset(0 0 100% 0)' }, { clipPath: 'inset(0 0 0% 0)', duration: 1.2 }, .15)
        .from('.c-hero__photo img', { scale: 1.12, duration: 1.8, ease: 'power3.out' }, .15)
        .from('.c-tag', { xPercent: 100, duration: .9 }, .45)
        .from('.c-hero__info > *', { y: 20, opacity: 0, duration: .9, stagger: .08 }, .6)
        .to('.c-tag span', { duration: 1.1, ease: 'none', scrambleText: { text: 'SINCE 2022', chars: 'upperCase', speed: .6 } }, .6);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.lines, { yPercent: 110, duration: 1.05, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
      // Animate text: each role word arrives character by character from the right.
      gsap.utils.toArray<HTMLElement>('.a-founder__role span').forEach((el) => {
        const s = SplitText.create(el, { type: 'chars', smartWrap: true, aria: 'none' });
        gsap.from(s.chars, { x: 150, opacity: 0, duration: .7, ease: 'power4', stagger: .04, scrollTrigger: { trigger: el, start: 'top 85%', once: true } });
      });
      ScrollTrigger.batch('[data-in]', { start: 'top 92%', once: true, onEnter: (els) => gsap.from(els, { y: 24, opacity: 0, duration: .9, stagger: .07, ease: 'expo.out', overwrite: true }) });

      // Image mask on scroll: the frame slides in from the right while the photo slides the other way, so the picture holds still.
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.a-close__media', start: 'top 88%', end: 'top 30%', scrub: true } })
        .fromTo('.a-close__mask', { xPercent: 100 }, { xPercent: 0 })
        .fromTo('.a-close__mask img', { xPercent: -100 }, { xPercent: 0 }, 0);

      // Text replacement: the service word morphs into the next one while the band is on screen.
      const word = wordRef.current!;
      let i = 0;
      const call = gsap.delayedCall(2.2, () => {
        i = (i + 1) % words.length;
        gsap.to(word, { duration: .9, ease: 'none', text: { value: words[i], type: 'diff' }, onComplete: () => { call.restart(true); } });
      }).pause();
      ScrollTrigger.create({ trigger: '.a-values', start: 'top 75%', end: 'bottom 25%', onToggle: (self) => (self.isActive ? call.restart(true) : call.pause()) });
      return () => { call.kill(); word.textContent = words[0]; };
    });

    // (The pin's refreshPriority is 1 so it refreshes first: higher numbers go first, and every trigger below it must measure
    // the page with its scroll space already added.)
    // Horizontal gallery: the story section pins and its strip slides left, a red line filling along the bottom. Without motion
    // or on a narrow screen the five years simply stack.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const el = root.current!;
      const section = el.querySelector<HTMLElement>('.a-story')!;
      const strip = el.querySelector<HTMLElement>('.a-story__strip')!;
      const dist = () => Math.max(0, strip.scrollWidth - innerWidth);
      el.classList.add('is-pinned');
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: section, start: 'top top', end: () => '+=' + dist(), pin: true, scrub: .6, invalidateOnRefresh: true, refreshPriority: 1 } })
        .to(strip, { x: () => -dist() }, 0)
        .to('.a-story__bar i', { scaleX: 1 }, 0);
      return () => el.classList.remove('is-pinned');
    });
    // Lateral pin indicator: the reasons list pins; as you scroll, the active title lights up, a line fills beside the list and
    // the matching reason replaces the last.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const el = root.current!;
      const section = el.querySelector<HTMLElement>('.a-why')!;
      const items = gsap.utils.toArray<HTMLElement>('.a-why__list li', el);
      const slides = gsap.utils.toArray<HTMLElement>('.a-why__slide', el);
      el.classList.add('is-why');
      gsap.set(slides, { autoAlpha: 0, y: 30 });
      gsap.set(slides[0], { autoAlpha: 1, y: 0 });
      gsap.set(items[0], { color: INK });
      gsap.set('.a-why__fill i', { scaleY: 1 / items.length, transformOrigin: 'top' });
      const tl = gsap.timeline({ scrollTrigger: { trigger: section, start: 'top top', end: '+=' + items.length * 55 + '%', pin: true, scrub: true, invalidateOnRefresh: true, refreshPriority: 1 } });
      items.forEach((item, i) => {
        if (!i) return;
        tl.set(items[i - 1], { color: MUTED }, .5 * i)
          .set(item, { color: INK }, .5 * i)
          .to(slides[i - 1], { autoAlpha: 0, y: -30, duration: .2 }, .5 * i - .05)
          .to(slides[i], { autoAlpha: 1, y: 0, duration: .25 }, .5 * i);
      });
      tl.to('.a-why__fill i', { scaleY: 1, ease: 'none', duration: tl.duration() }, 0).to({}, { duration: .3 });
      return () => el.classList.remove('is-why');
    });
    // Pinned panel with overscroll (as on Contact): the hero stays put and settles back as the story section slides over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const hero = root.current!.querySelector<HTMLElement>('.c-hero')!;
      gsap.timeline({ scrollTrigger: { trigger: hero, start: () => (hero.offsetHeight > innerHeight ? 'bottom bottom' : 'top top'), end: () => '+=' + innerHeight, pin: true, pinSpacing: false, scrub: true, invalidateOnRefresh: true, refreshPriority: 2 } })
        .fromTo('.c-hero__stage', { scale: 1, opacity: 1, transformOrigin: '50% 30%' }, { scale: .88, opacity: .35, duration: .9, ease: 'none' })
        .to('.c-hero__stage', { opacity: 0, duration: .1, ease: 'none' });
    });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="c-page a-page">
      {/* A1 Hero */}
      <section className="c-hero" aria-labelledby="a-title">
        <div className="c-hero__stage">
          <h1 id="a-title" className="c-h1 a-h1">Good people.<br /><em>Considered care.</em></h1>
          <div className="c-hero__band">
            <div className="c-hero__photo"><img src="/site/assets/portfolio/portfolio-01.jpg" alt="A black Range Rover at sunset after detailing" fetchPriority="high" /></div>
            <div className="c-tag" aria-hidden="true"><span>SINCE 2022</span></div>
          </div>
          <div className="c-hero__info">
            <div className="c-hero__lead">
              <h2>Built by two childhood friends.</h2>
              <p>Dapper grew from a mobile detailing business into a dedicated Peterborough studio. The care behind every vehicle remains the same.</p>
            </div>
            <dl className="c-hero__reach a-facts">
              <div><dt>Began</dt><dd>2022, mobile detailing</dd></div>
              <div><dt>Studio</dt><dd>2026, {contact.street}</dd></div>
            </dl>
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
          <div className="a-story__bar" aria-hidden="true"><i /></div>
        </section>

        {/* A3 The founders: type only until portraits exist */}
        <section className="a-founders" aria-labelledby="a-founders-title">
          <h2 id="a-founders-title" data-head>Meet the <em>founders.</em></h2>
          <div className="a-founders__grid">
            {founders.map((f) => (
              <article key={f.name} className="a-founder">
                <p className="a-founder__role" aria-label={f.title}><span aria-hidden="true">{f.role}</span></p>
                <h3 data-in>{f.name}</h3>
                <p className="c-muted" data-in>{f.text}</p>
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
            <a className="c-btn c-btn--red" href="/booking" data-in><Roll>Book your detail</Roll><ArrowRight size={20} aria-hidden="true" /></a>
          </div>
        </section>

        {/* A5 Why vehicle owners trust Dapper: a list that pins and advances reason by reason; stacked without motion */}
        <section className="a-why" aria-labelledby="a-why-title">
          <div className="a-why__inner">
            <div className="a-why__side">
              <h2 id="a-why-title" data-head>Why vehicle owners trust <em>Dapper</em></h2>
              <div className="a-why__listwrap" aria-hidden="true">
                <div className="a-why__fill"><i /></div>
                <ol className="a-why__list">{why.map((w) => <li key={w.title}>{w.title}</li>)}</ol>
              </div>
            </div>
            <div className="a-why__slides">
              {why.map((w) => (
                <article key={w.title} className="a-why__slide">
                  <h3>{w.title}</h3>
                  <p>{w.text}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        {/* A6 Partners: on ink, because the supplied logos are made for dark grounds */}
        <section className="a-partners" aria-labelledby="a-partners-title">
          <h2 id="a-partners-title" data-head>Our trusted <em>commercial partners</em></h2>
          <ul className="a-partners__grid">
            {partners.map((p) => (
              <li key={p.name} data-in><img className={p.name.includes('McGee') ? 'is-crop' : undefined} src={p.src} alt={p.name} loading="lazy" /></li>
            ))}
          </ul>
        </section>

        {/* A7 Closing band: the booking call, with a photo that slides into its frame */}
        <section className="a-close" aria-labelledby="a-close-title">
          <div className="a-close__text">
            <h2 id="a-close-title" data-head>Ready for <em>precise detailing?</em></h2>
            <p className="c-muted a-close__lede" data-in>Choose mobile service or at-shop detailing and pick your date and time in seconds.</p>
            <div className="a-close__actions" data-in>
              <a className="c-btn c-btn--red" href="/booking"><Roll>Book now</Roll><ArrowRight size={20} aria-hidden="true" /></a>
              <a className="a-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
            </div>
          </div>
          <div className="a-close__media">
            <div className="a-close__mask"><img src="/site/assets/portfolio/portfolio-02.jpg" alt="A RAM pickup parked beside a light aircraft outside a hangar" loading="lazy" /></div>
          </div>
        </section>
      </div>
    </div>
  );
}

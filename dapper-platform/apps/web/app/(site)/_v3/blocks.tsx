'use client';
import Link from 'next/link';
import { useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { ArrowRight, Award, CalendarCheck, CalendarClock, Car, CheckCircle2, ClipboardList, FlaskConical, Phone, Play, ShieldCheck, Sparkles, Star, Wrench, X } from 'lucide-react';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsap';
import { contact, cta, processSteps, proof, testimonials, transformations, why } from '../dapper';
import { Dealers } from '../_components/Dealers';

/* Reusable blocks of the Dapper page system (the home page's sections, made into parts for the inner pages). Each one
   animates itself: titles rise out of a mask, cards rise in sequence, dark panels settle as they arrive. */

export const Stars = ({ size = 16 }: { size?: number }) => <span className="d-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={size} fill="currentColor" strokeWidth={0} />)}</span>;
export const Chip = ({ children, light }: { children: ReactNode; light?: boolean }) => <p className={'d-chip' + (light ? ' d-chip--light' : '')}><span className="d-dot" />{children}</p>;

// Motion shared by every block on a page: mount once inside the page.
export function PageMotion({ children }: { children: ReactNode }) {
  const root = useRef<HTMLDivElement>(null);
  useGSAP(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const hero = root.current!.querySelector('.d-page-hero');
    if (hero) {
      const h = SplitText.create(hero.querySelector('h1'), { type: 'lines', mask: 'lines' });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from(hero, { scale: .97, duration: 1.3 }, 0)
        .from(hero.querySelector('.d-page-hero__img'), { scale: 1.12, duration: 2, ease: 'power3.out' }, 0)
        .from(h.lines, { yPercent: 105, duration: 1.1, stagger: .08 }, .2)
        .from(hero.querySelectorAll('.d-page-hero__body > :not(h1)'), { y: 18, opacity: 0, stagger: .07, duration: .9 }, .5);
    }
    gsap.utils.toArray<HTMLElement>('[data-d-head]', root.current).forEach((el) => {
      const s = SplitText.create(el, { type: 'lines', mask: 'lines' });
      gsap.from(s.lines, { yPercent: 105, duration: 1.1, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } });
    });
    const cards = gsap.utils.toArray<HTMLElement>('[data-d-in]', root.current);
    if (cards.length) ScrollTrigger.batch(cards, { start: 'top 92%', once: true, onEnter: (els) => gsap.from(els, { y: 40, opacity: 0, duration: 1, stagger: .07, ease: 'expo.out' }) });
    gsap.utils.toArray<HTMLElement>('.d-panel', root.current).forEach((el) => gsap.from(el, { scale: .97, duration: 1.2, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 92%', once: true } }));
    const line = root.current!.querySelector('.d-steps__line i');
    if (line) gsap.from(line, { scaleX: 0, transformOrigin: 'left', ease: 'none', scrollTrigger: { trigger: '.d-steps', start: 'top 75%', end: 'bottom 60%', scrub: true } });
  }, { scope: root });
  return <div ref={root} className="d-home">{children}</div>;
}

export function PageHero({ chip, title, lead, image, alt, children }: { chip: string; title: ReactNode; lead: string; image: string; alt: string; children?: ReactNode }) {
  return (
    <section className="d-page-hero" aria-labelledby="page-title">
      <img className="d-page-hero__img" src={image} alt={alt} fetchPriority="high" />
      <div className="d-hero__shade" />
      <div className="d-page-hero__body">
        <Chip>{chip}</Chip>
        <h1 id="page-title">{title}</h1>
        <p className="d-hero__lead">{lead}</p>
        <div className="d-hero__actions">{children ?? <><a className="d-btn d-btn--red" href="/book-appointment">Book now<ArrowRight size={18} /></a><a className="d-btn d-btn--glass" href={'tel:' + contact.tel}><Phone size={16} />{contact.phone}</a></>}</div>
      </div>
    </section>
  );
}

export function SectionHead({ chip, title, text, center, dark }: { chip: string; title: string; text?: string; center?: boolean; dark?: boolean }) {
  return (
    <div className={'d-head' + (center ? ' d-head--center' : '')}>
      <Chip light={!dark}>{chip}</Chip>
      <h2 data-d-head>{title}</h2>
      {text && <p data-d-in>{text}</p>}
    </div>
  );
}

const whyIcons = [Award, FlaskConical, Car, ClipboardList, Wrench, CalendarClock];
export function WhyGrid() {
  return (
    <section className="d-section">
      <SectionHead chip="Why choose us" title="Why vehicle owners trust Dapper" center />
      <div className="d-why">
        {why.map((w, i) => { const Icon = whyIcons[i]; return <article key={w.title} data-d-in><span className="d-icon"><Icon size={22} /></span><h3>{w.title}</h3><p>{w.text}</p></article>; })}
      </div>
    </section>
  );
}

const stepIcons = [CalendarCheck, ShieldCheck, Sparkles, CheckCircle2];
export function ProcessStrip() {
  return (
    <section className="d-panel d-panel--soft">
      <SectionHead chip="Our process" title="How we do it" center dark />
      <div className="d-steps">
        <div className="d-steps__line" aria-hidden="true"><i /></div>
        {processSteps.map((p, i) => { const Icon = stepIcons[i]; return (
          <article key={p.title} data-d-in><span className="d-steps__n"><Icon size={22} /><b>{i + 1}</b></span><h3>{p.title}</h3><p>{p.text}</p></article>
        ); })}
      </div>
    </section>
  );
}

export function PartnersStrip() {
  return (
    <section className="d-section d-section--tight">
      <SectionHead chip="Our partners" title="Our trusted commercial partners" center />
      <Dealers />
    </section>
  );
}

export function Reels({ title = transformations.heading, text = transformations.text, button = true }: { title?: string; text?: string; button?: boolean }) {
  const [film, setFilm] = useState<string | null>(null);
  return (
    <section className="d-panel" id="work">
      <div className="d-head d-head--center">
        <Chip>Transformations</Chip>
        <h2 data-d-head>{title}</h2>
        <p data-d-in>{text}</p>
        <p className="d-rating" data-d-in><Stars /><b>5☆</b>Google Ratings</p>
      </div>
      <div className="d-reels">
        {transformations.clips.map((c, i) => (
          <button key={c} type="button" className="d-reel" onClick={() => setFilm(c)} aria-label={'Play transformation ' + (i + 1)} data-d-in>
            <video src={c} muted loop playsInline autoPlay preload="metadata" />
            <span className="d-reel__play"><Play size={18} fill="currentColor" /></span>
          </button>
        ))}
      </div>
      {button && <div className="d-center"><Link className="d-btn d-btn--red" href="/portfolio">Explore portfolio<ArrowRight size={18} /></Link></div>}
      {film && (
        <div className="d-film" role="dialog" aria-modal="true" aria-label="Transformation video" onClick={() => setFilm(null)}>
          <video src={film} autoPlay loop playsInline controls onClick={(e) => e.stopPropagation()} />
          <button type="button" className="d-film__close" onClick={() => setFilm(null)} aria-label="Close"><X size={22} /></button>
        </div>
      )}
    </section>
  );
}

export function Reviews() {
  return (
    <section className="d-section d-reviews">
      <div className="d-head d-head--center">
        <Chip light>Testimonials</Chip>
        <h2 data-d-head>What our clients say</h2>
        <p className="d-rating d-rating--dark" data-d-in><Stars /><b>5.0</b>{proof.customers}</p>
      </div>
      {[0, 1].map((row) => (
        <div key={row} className={'d-marquee' + (row ? ' d-marquee--rev' : '')}>
          <div className="d-marquee__track">
            {[...testimonials.slice(row * 4, row * 4 + 4), ...testimonials.slice(row * 4, row * 4 + 4)].map((t, i) => (
              <figure key={t.name + i} className="d-review" aria-hidden={i >= 4}>
                <Stars size={14} /><blockquote>“{t.quote}”</blockquote>
                <figcaption><span className="d-avatar">{t.name.charAt(0).toUpperCase()}</span>{t.name}</figcaption>
              </figure>
            ))}
          </div>
        </div>
      ))}
    </section>
  );
}

export function CallToBook({ title = cta.heading, text = cta.text, image = '/site/assets/portfolio/portfolio-06.jpg' }: { title?: string; text?: string; image?: string }) {
  return (
    <section className="d-cta">
      <img src={image} alt="" loading="lazy" />
      <div className="d-cta__content">
        <h2 data-d-head>{title}</h2>
        <p>{text}</p>
        <div className="d-hero__actions"><a className="d-btn d-btn--red" href="/book-appointment">Book now<ArrowRight size={18} /></a><a className="d-btn d-btn--glass" href={'tel:' + contact.tel}><Phone size={16} />{contact.phone}</a></div>
      </div>
    </section>
  );
}

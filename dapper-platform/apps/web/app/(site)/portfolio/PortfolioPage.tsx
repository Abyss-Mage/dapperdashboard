'use client';
import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, ArrowUpRight, Play, X } from 'lucide-react';
import { contact, cta, portfolio, transformations } from '../dapper';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Portfolio, built section by section with the owner's approval of each. Same paper system as Contact and About (../paper.css).
   Section P1: the hero, a photo mosaic beside the headline. Section P2: the gallery. */

const tiles = [
  { cls: 'a', src: '17', alt: 'A Harley-Davidson motorcycle with a tan and chrome finish' },
  { cls: 'b', src: '09', alt: 'A black classic Chevrolet Nova parked on gravel' },
  { cls: 'c', src: '05', alt: 'A white Dodge RAM pickup under a blue sky' },
  { cls: 'd', src: '14', alt: 'A grey Mazda SUV in a driveway' },
  { cls: 'e', src: '06', alt: 'A white Porsche 911 outside the Dapper studio' },
];

// The studio's channels; each handle is read from the address in the project's contact details.
const channels = [
  { name: 'Instagram', href: contact.instagram, handle: '@' + contact.instagram.split('/').filter(Boolean).pop() },
  { name: 'YouTube', href: contact.youtube, handle: contact.youtube.split('/').filter(Boolean).pop()! },
];

// Pixel sizes of the portfolio photos that are not portrait 3:4, so each frame holds its shape before its image loads
// (the length of the sideways strip depends on it).
const sizes: Record<string, [number, number]> = { '01': [900, 1600], '02': [1600, 900], '08': [1600, 900], '05': [1600, 1200], '06': [1600, 1200], '07': [1600, 1200], '09': [1600, 1200], '10': [1600, 1200], '14': [1600, 1200] };
const ratio = (src: string) => { const [w, h] = sizes[src.match(/(\d\d)\.jpg$/)![1]] ?? [1200, 1600]; return w + ' / ' + h; };

export default function PortfolioPage() {
  const root = useRef<HTMLDivElement>(null);
  const viewer = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState<number | null>(null);

  // The full-size viewer is a modal dialog: focus is trapped, Escape closes it, and the arrow keys move between photos.
  useEffect(() => {
    const d = viewer.current;
    if (!d) return;
    if (open !== null && !d.open) d.showModal();
    if (open === null && d.open) d.close();
  }, [open]);
  const step = (n: number) => setOpen((i) => (i === null ? i : (i + n + portfolio.length) % portfolio.length));

  // The films open in the same kind of modal dialog, with sound and controls.
  const filmBox = useRef<HTMLDialogElement>(null);
  const [film, setFilm] = useState<number | null>(null);
  const clips = transformations.clips;
  useEffect(() => {
    const d = filmBox.current;
    if (!d) return;
    if (film !== null && !d.open) d.showModal();
    if (film === null && d.open) d.close();
  }, [film]);
  const stepFilm = (n: number) => setFilm((i) => (i === null ? i : (i + n + clips.length) % clips.length));

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      SplitText.create('.p-h1', { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: 1.1, stagger: .07, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .fromTo('.p-tile', { clipPath: 'inset(100% 0 0 0)' }, { clipPath: 'inset(0% 0 0 0)', duration: 1.1, stagger: .09 }, .1)
        .from('.p-tile img', { scale: 1.18, duration: 1.7, stagger: .09, ease: 'power3.out' }, .1)
        .from('.c-tag', { xPercent: 100, duration: .9 }, .5)
        .from('.p-hero__info > *', { y: 20, opacity: 0, duration: .9, stagger: .08 }, .6)
        .to('.c-tag span', { duration: 1.1, ease: 'none', scrambleText: { text: 'REAL WORK', chars: 'upperCase', speed: .6 } }, .6);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.lines, { yPercent: 110, duration: 1.05, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
    });

    // Horizontal gallery: the section pins and the strip of photos slides left, a red line filling along the bottom. On a phone
    // or without motion the strip is an ordinary swipeable row. (refreshPriority 1: pins refresh first, higher numbers go first.)
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const el = root.current!;
      const section = el.querySelector<HTMLElement>('.p-gallery')!;
      const strip = el.querySelector<HTMLElement>('.p-strip__list')!;
      const dist = () => Math.max(0, strip.scrollWidth - innerWidth);
      el.classList.add('is-gallery');
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: section, start: 'top top', end: () => '+=' + dist(), pin: true, scrub: .6, invalidateOnRefresh: true, refreshPriority: 1 } })
        .to(strip, { x: () => -dist() }, 0)
        .to('.p-gallery__bar i', { scaleX: 1 }, 0);
      return () => el.classList.remove('is-gallery');
    });
    // Films play only while they are on screen (and not at all with reduced motion, where each shows its first frame); on a
    // desktop screen the five columns drift at different speeds as the section scrolls past.
    gsap.utils.toArray<HTMLVideoElement>('.p-reel video').forEach((v) => {
      // With reduced motion a film stays paused, so nudge it to paint its first frame instead of an empty box.
      const still = () => { if (matchMedia('(prefers-reduced-motion: reduce)').matches) { v.preload = 'auto'; v.currentTime = .1; } };
      if (v.readyState >= 1) still(); else v.addEventListener('loadedmetadata', still, { once: true });
      ScrollTrigger.create({ trigger: v, start: 'top 95%', end: 'bottom 5%', onToggle: (self) => {
        if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
        if (self.isActive) void v.play().catch(() => {}); else v.pause();
      } });
    });
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const drift = [0, -70, -20, -90, -40];
      gsap.utils.toArray<HTMLElement>('.p-reel').forEach((el, i) => gsap.to(el, { y: drift[i % drift.length], ease: 'none', scrollTrigger: { trigger: '.p-films__reels', start: 'top bottom', end: 'bottom top', scrub: true } }));
    });

    // Pinned panel with overscroll (as on Contact and About): the hero stays put and settles back as the gallery slides over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const hero = root.current!.querySelector<HTMLElement>('.p-hero')!;
      gsap.timeline({ scrollTrigger: { trigger: hero, start: () => (hero.offsetHeight > innerHeight ? 'bottom bottom' : 'top top'), end: () => '+=' + innerHeight, pin: true, pinSpacing: false, scrub: true, invalidateOnRefresh: true, refreshPriority: 2 } })
        .fromTo('.p-hero__copy, .p-hero__media', { scale: 1, opacity: 1, transformOrigin: '50% 30%' }, { scale: .9, opacity: .35, duration: .9, ease: 'none' })
        .to('.p-hero__copy, .p-hero__media', { opacity: 0, duration: .1, ease: 'none' });
    });

    // Image mask on scroll: the frame slides in from the left while the photo slides the other way, so the picture holds still.
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: '.p-close__media', start: 'top 88%', end: 'top 30%', scrub: true } })
        .fromTo('.p-close__mask', { xPercent: -100 }, { xPercent: 0 })
        .fromTo('.p-close__mask img', { xPercent: 100 }, { xPercent: 0 }, 0);
    });

    // The strip's photos are lazy, but once the gallery is about to arrive they all start loading, so none is still blank when the
    // sideways scroll reaches it.
    ScrollTrigger.create({ trigger: '.p-gallery', start: 'top bottom+=600', once: true, onEnter: () => gsap.utils.toArray<HTMLImageElement>('.p-photo img').forEach((img) => { img.loading = 'eager'; }) });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="c-page">
      {/* P1 Hero: headline and facts on the left, five photos tiled on the right */}
      <section className="p-hero" aria-labelledby="p-title">
        <div className="p-hero__copy">
          <h1 id="p-title" className="p-h1">Real results.<br /><em>Real vehicles.</em></h1>
          <div className="p-hero__info">
            <h2>Every vehicle tells a story.</h2>
            <p>Every transformation showcases our commitment to excellence.</p>
            <dl className="p-facts">
              <div><dt>Photos</dt><dd>{portfolio.length} vehicles and details</dd></div>
              <div><dt>Films</dt><dd>{transformations.clips.length} real transformations</dd></div>
            </dl>
          </div>
        </div>
        <div className="p-hero__media">
          <div className="p-mosaic">
            {tiles.map((t, i) => (
              <figure key={t.src} className={'p-tile p-tile--' + t.cls}>
                <img src={'/site/assets/portfolio/portfolio-' + t.src + '.jpg'} alt={t.alt} fetchPriority={i < 2 ? 'high' : undefined} loading="eager" />
              </figure>
            ))}
          </div>
          <div className="c-tag" aria-hidden="true"><span>REAL WORK</span></div>
        </div>
      </section>

      {/* P2 Gallery: every photo on one strip; select one to see it full size */}
      <div className="p-after">
        <section className="p-gallery" aria-labelledby="p-gallery-title">
          <div className="p-gallery__head">
            <h2 id="p-gallery-title" data-head>Work worth a <em>closer look.</em></h2>
            <p>Cars, trucks, classics and motorcycles cared for by the Dapper team. Select a photo to see it full size.</p>
          </div>
          <div className="p-strip">
            <ul className="p-strip__list">
              {portfolio.map((ph, i) => (
                <li key={ph.src}>
                  <button type="button" className="p-photo" onClick={() => setOpen(i)} aria-label={'Open photo: ' + ph.caption} style={{ aspectRatio: ratio(ph.src) }}>
                    <img src={ph.src} alt="" loading="lazy" decoding="async" />
                  </button>
                  <p>{ph.caption}</p>
                </li>
              ))}
            </ul>
          </div>
          <div className="p-gallery__bar" aria-hidden="true"><i /></div>
        </section>

        {/* P3 Films: five transformations, playing while they are in view; select one for sound and controls */}
        <section className="p-films" aria-labelledby="p-films-title">
          <div className="p-films__head">
            <h2 id="p-films-title" data-head>The finish, in <em>motion.</em></h2>
            <p>Real work from the Dapper studio. Select a film to watch it with sound.</p>
          </div>
          <ul className="p-films__reels">
            {clips.map((c, i) => (
              <li key={c}>
                <button type="button" className="p-reel" onClick={() => setFilm(i)} aria-label={'Watch transformation ' + (i + 1) + ' of ' + clips.length}>
                  <video src={c + '#t=0.1'} muted loop playsInline preload="metadata" aria-hidden="true" tabIndex={-1} />
                  <span className="p-reel__play" aria-hidden="true"><Play size={18} fill="currentColor" /></span>
                </button>
              </li>
            ))}
          </ul>
        </section>

        {/* P4 Follow the work: the studio's two channels */}
        <section className="p-social" aria-labelledby="p-social-title">
          <div className="p-social__head">
            <h2 id="p-social-title" data-head>Follow the <em>work.</em></h2>
            <p data-in>Follow along for more transformations.</p>
          </div>
          <ul className="p-social__list">
            {channels.map((c) => (
              <li key={c.name} data-in>
                <a href={c.href} target="_blank" rel="noopener noreferrer">
                  <span className="p-social__name">{c.name}</span>
                  <span className="p-social__handle">{c.handle}<span className="sr"> (opens in a new tab)</span></span>
                  <ArrowUpRight size={36} strokeWidth={1.6} aria-hidden="true" />
                </a>
              </li>
            ))}
          </ul>
        </section>

        {/* P5 Closing band: the booking call, beside a photo that slides into its frame */}
        <section className="p-close" aria-labelledby="p-close-title">
          <div className="p-close__media">
            <div className="p-close__mask"><img src="/site/assets/portfolio/portfolio-07.jpg" alt="A classic Mercedes with its bonnet open, showing the engine bay" loading="lazy" /></div>
          </div>
          <div className="p-close__text">
            <h2 id="p-close-title" data-head>Ready to give your car <em>the care it deserves?</em></h2>
            <p className="c-muted p-close__lede" data-in>{cta.text}</p>
            <div className="p-close__actions" data-in>
              <a className="c-btn c-btn--red" href="/booking"><span className="c-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={20} aria-hidden="true" /></a>
              <a className="p-close__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
            </div>
          </div>
        </section>
      </div>

      <dialog ref={viewer} className="p-viewer" aria-label={open !== null ? portfolio[open].caption : 'Photo'} onClose={() => setOpen(null)}
        onClick={(e) => { if (e.target === e.currentTarget) setOpen(null); }}
        onKeyDown={(e) => { if (e.key === 'ArrowRight') step(1); if (e.key === 'ArrowLeft') step(-1); }}>
        {open !== null && (
          <>
            <img src={portfolio[open].src} alt={portfolio[open].caption} />
            <p className="p-viewer__cap">{portfolio[open].caption}<span>{open + 1} / {portfolio.length}</span></p>
            <button type="button" className="p-viewer__btn p-viewer__close" onClick={() => setOpen(null)} aria-label="Close"><X size={22} aria-hidden="true" /></button>
            <button type="button" className="p-viewer__btn p-viewer__prev" onClick={() => step(-1)} aria-label="Previous photo"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" className="p-viewer__btn p-viewer__next" onClick={() => step(1)} aria-label="Next photo"><ArrowRight size={22} aria-hidden="true" /></button>
          </>
        )}
      </dialog>

      <dialog ref={filmBox} className="p-viewer p-viewer--film" aria-label={film !== null ? 'Transformation ' + (film + 1) + ' of ' + clips.length : 'Film'} onClose={() => setFilm(null)}
        onClick={(e) => { if (e.target === e.currentTarget) setFilm(null); }}
        onKeyDown={(e) => { if (e.key === 'ArrowRight') stepFilm(1); if (e.key === 'ArrowLeft') stepFilm(-1); }}>
        {film !== null && (
          <>
            <button type="button" className="p-viewer__btn p-viewer__close" onClick={() => setFilm(null)} aria-label="Close"><X size={22} aria-hidden="true" /></button>
            <video key={clips[film]} src={clips[film]} autoPlay loop playsInline controls preload="auto" ref={(el) => { if (el && el.paused) void el.play().catch(() => {}); }} />
            <p className="p-viewer__cap">Transformation {film + 1}<span>{film + 1} / {clips.length}</span></p>
            <button type="button" className="p-viewer__btn p-viewer__prev" onClick={() => stepFilm(-1)} aria-label="Previous film"><ArrowLeft size={22} aria-hidden="true" /></button>
            <button type="button" className="p-viewer__btn p-viewer__next" onClick={() => stepFilm(1)} aria-label="Next film"><ArrowRight size={22} aria-hidden="true" /></button>
          </>
        )}
      </dialog>
    </div>
  );
}

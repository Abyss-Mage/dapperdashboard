'use client';
import { CtaBand } from '../_components/CtaBand';
import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, ArrowUpRight, Play, X } from 'lucide-react';
import { contact, cta, portfolio, transformations } from '../dapper';
import { FilmCarousel } from '../_components/FilmCarousel';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsapContact';

/* Portfolio, built section by section with the owner's approval of each. Same paper system as Contact and About (../paper.css).
   Section P1: the hero, two rows of photographs drifting behind the headline. Section P2: the gallery. */

// The studio's channels; each handle is read from the address in the project's contact details.
const channels = [
  { name: 'Instagram', href: contact.instagram, handle: '@' + contact.instagram.split('/').filter(Boolean).pop() },
  { name: 'YouTube', href: contact.youtube, handle: contact.youtube.split('/').filter(Boolean).pop()! },
];

// Pixel sizes of the portfolio photos that are not portrait 3:4, so each frame holds its shape before its image loads
// (the length of the sideways strip depends on it).
const sizes: Record<string, [number, number]> = { '01': [900, 1600], '02': [1600, 900], '08': [1600, 900], '05': [1600, 1200], '06': [1600, 1200], '07': [1600, 1200], '09': [1600, 1200], '10': [1600, 1200], '14': [1600, 1200] };
const ratio = (src: string) => { const [w, h] = sizes[src.match(/(\d\d)\.jpg$/)![1]] ?? [1200, 1600]; return w + ' / ' + h; };

type Shot = { src: string; caption: string; alt: string; ratio: string };

export default function PortfolioPage() {
  const root = useRef<HTMLDivElement>(null);
  // Photos added in the dashboard (Settings > Website images) replace the built-in ones as soon as one is published.
  const [uploaded, setUploaded] = useState<Shot[] | null>(null);
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/portfolio', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((b: { items?: { mediaId: string; title: string; alt: string; width: number; height: number }[] } | null) => {
        if (!live || !b || !Array.isArray(b.items) || b.items.length === 0) return;
        setUploaded(b.items.map((i) => ({ src: '/api/v1/public/media/' + i.mediaId, caption: i.title, alt: i.alt || i.title, ratio: i.width > 0 && i.height > 0 ? i.width + ' / ' + i.height : '3 / 4' })));
      })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  const photos: Shot[] = uploaded ?? portfolio.map((ph) => ({ src: ph.src, caption: ph.caption, alt: '', ratio: ratio(ph.src) }));
  // Two rows of photographs drift in opposite directions behind the hero headline (each row is listed twice so the loop has no seam).
  const rowOf = (r: number) => photos.filter((_, i) => i % 2 === r).slice(0, 9);
  // The strip's length depends on the photos, so the scroll positions are measured again when they change.
  useEffect(() => { if (uploaded) { const id = setTimeout(() => ScrollTrigger.refresh(), 400); return () => clearTimeout(id); } }, [uploaded]);
  const viewer = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState<number | null>(null);

  // The full-size viewer is a modal dialog: focus is trapped, Escape closes it, and the arrow keys move between photos.
  useEffect(() => {
    const d = viewer.current;
    if (!d) return;
    if (open !== null && !d.open) d.showModal();
    if (open === null && d.open) d.close();
  }, [open]);
  const step = (n: number) => setOpen((i) => (i === null ? i : (i + n + photos.length) % photos.length));

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
        onSplit: (self) => gsap.from(self.words, { yPercent: 110, duration: .6, stagger: .04, ease: 'expo.out' }) });
      gsap.timeline({ defaults: { ease: 'expo.out' } })
        .from('.ph__rows', { opacity: 0, scale: 1.06, duration: 1.2, ease: 'power3.out' }, 0)
        .from('.ph__aside > *', { y: 20, opacity: 0, duration: .55, stagger: .06 }, .3);
      gsap.utils.toArray<HTMLElement>('[data-head]').forEach((el) => SplitText.create(el, { type: 'lines,words', mask: 'lines', linesClass: 'c-line', autoSplit: true,
        onSplit: (self) => gsap.from(self.lines, { yPercent: 110, duration: .6, stagger: .045, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } }) }));
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
    // Films play only while they are on screen (and not at all with reduced motion, where each shows its first frame).
    gsap.utils.toArray<HTMLVideoElement>('.p-reel video').forEach((v) => {
      // With reduced motion a film stays paused, so nudge it to paint its first frame instead of an empty box.
      const still = () => { if (matchMedia('(prefers-reduced-motion: reduce)').matches) { v.preload = 'auto'; v.currentTime = .1; } };
      if (v.readyState >= 1) still(); else v.addEventListener('loadedmetadata', still, { once: true });
      ScrollTrigger.create({ trigger: v, start: 'top 95%', end: 'bottom 5%', onToggle: (self) => {
        if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
        if (self.isActive) void v.play().catch(() => {}); else v.pause();
      } });
    });

    // Pinned panel with overscroll (as on Contact and About): the hero stays put and settles back as the gallery slides over it.
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const hero = root.current!.querySelector<HTMLElement>('.p-hero')!;
      gsap.timeline({ scrollTrigger: { trigger: hero, start: () => (hero.offsetHeight > innerHeight ? 'bottom bottom' : 'top top'), end: () => '+=' + innerHeight, pin: true, pinSpacing: false, scrub: true, invalidateOnRefresh: true, refreshPriority: 2 } })
        .fromTo('.ph__stage', { scale: 1, opacity: 1, transformOrigin: '50% 30%' }, { scale: .92, opacity: .35, duration: .9, ease: 'none' })
        .to('.ph__stage', { opacity: 0, duration: .1, ease: 'none' });
    });

    // The strip's photos are lazy, but once the gallery is about to arrive they all start loading, so none is still blank when the
    // sideways scroll reaches it.
    ScrollTrigger.create({ trigger: '.p-gallery', start: 'top bottom+=600', once: true, onEnter: () => gsap.utils.toArray<HTMLImageElement>('.p-photo img').forEach((img) => { img.loading = 'eager'; }) });
    document.fonts?.ready.then(() => ScrollTrigger.refresh());
    return () => mm.revert();
  }, { scope: root });

  return (
    <div ref={root} className="c-page">
      {/* P1 Hero: two rows of photographs drift behind the headline */}
      <section className="p-hero ph" aria-labelledby="p-title">
        <div className="ph__stage">
          <div className="ph__rows" aria-hidden="true">
            {[0, 1].map((r) => (
              <div key={r} className={'ph__row' + (r ? ' ph__row--rev' : '')}>
                <div className="ph__track">
                  {[...rowOf(r), ...rowOf(r)].map((ph, i) => <img key={ph.src + i} src={ph.src} alt="" style={{ aspectRatio: ph.ratio }} loading="eager" decoding="async" />)}
                </div>
              </div>
            ))}
          </div>
          <div className="ph__scrim" aria-hidden="true" />
          <div className="ph__copy">
            <h1 id="p-title" className="p-h1">Real results.<br /><em>Real vehicles.</em></h1>
            <div className="ph__aside">
              <p>Every transformation shows our commitment to excellence: {photos.length} vehicles and details, and {transformations.clips.length} films from the Dapper studio.</p>
              <a className="c-btn c-btn--red" href="#p-gallery-title"><span className="c-roll"><span>See the work</span><span aria-hidden="true">See the work</span></span><ArrowRight size={20} aria-hidden="true" /></a>
            </div>
          </div>
        </div>
      </section>

      {/* P2 Gallery: every photo on one strip; select one to see it full size */}
      <div className="p-after">
        <section className="p-gallery" id="gallery" aria-labelledby="p-gallery-title">
          <div className="p-gallery__head">
            <h2 id="p-gallery-title" data-head>Work worth a <em>closer look.</em></h2>
            <p>Cars, trucks, classics and motorcycles cared for by the Dapper team. Select a photo to see it full size.</p>
          </div>
          <div className="p-strip">
            <ul className="p-strip__list">
              {photos.map((ph, i) => (
                <li key={ph.src + i}>
                  <button type="button" className="p-photo" onClick={() => setOpen(i)} aria-label={'Open photo ' + (i + 1) + ' of ' + photos.length} style={{ aspectRatio: ph.ratio }}>
                    <img src={ph.src} alt="" loading="lazy" decoding="async" />
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div className="p-gallery__bar" aria-hidden="true"><i /></div>
        </section>

        {/* P3 Films: five transformations, playing while they are in view; select one for sound and controls */}
        <section className="p-films" aria-labelledby="p-films-title">
          <div className="fc-sec">
            <div className="fc-sec__text">
              <h2 id="p-films-title" data-head>The finish, in <em>motion.</em></h2>
              <p>Real work from the Dapper studio. Select a film to watch it with sound.</p>
              <a className="c-btn c-btn--red" href="/booking"><span className="c-roll"><span>Book now</span><span aria-hidden="true">Book now</span></span><ArrowRight size={20} aria-hidden="true" /></a>
            </div>
            <FilmCarousel clips={clips} onOpen={setFilm} />
          </div>
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

        <CtaBand id="p-close-title" />
      </div>

      <dialog ref={viewer} className="p-viewer" aria-label={open !== null ? 'Photo ' + (open + 1) + ' of ' + photos.length : 'Photo'} onClose={() => setOpen(null)}
        onClick={(e) => { if (e.target === e.currentTarget) setOpen(null); }}
        onKeyDown={(e) => { if (e.key === 'ArrowRight') step(1); if (e.key === 'ArrowLeft') step(-1); }}>
        {open !== null && (
          <>
            <img src={photos[open].src} alt={photos[open].alt} />
            <p className="p-viewer__cap"><span>{open + 1} / {photos.length}</span></p>
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

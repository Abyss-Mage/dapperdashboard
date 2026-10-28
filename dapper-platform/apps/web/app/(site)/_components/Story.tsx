'use client';
import { useEffect, useRef } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsap';

/* The home scroll story. This section keeps its own staging (the agreed exception to the tape-line look); it is the former
   index.html story and story.js, moved into React. GSAP ScrollTrigger pins the stage and scrubs through the steps, and the
   progress is published on window.dapperHome for the renderer: the photographic frame sequence when one is published
   (/site/assets/frames/manifest.json), otherwise the real-time 3D scene. Both are plain ES modules in /site. */

type Step = { key: string; hud: string; detail: string; swatch: string; title: [string, string]; text: string; tint?: boolean };

const steps: Step[] = [
  { key: 'road', hud: 'Arrival', detail: 'Road-worn', swatch: '#8a6d4a', title: ['Good cars.', 'Extraordinary care.'], text: 'Miles of coast road on the paint. Scroll to bring it into the Dapper garage.' },
  { key: 'garage', hud: 'Dapper garage', detail: 'Bay 01', swatch: '#fb1a2b', title: ['Into the', 'garage.'], text: 'Every car starts with a walk-around, so we know exactly what we are working with.' },
  { key: 'foam', hud: 'Detailing', detail: 'Snow foam', swatch: '#f3f5f7', title: ['Foam', 'first.'], text: 'Thick snow foam loosens the road dirt before anything touches the paint.' },
  { key: 'rinse', hud: 'Detailing', detail: 'Pressure rinse', swatch: '#7fd3ff', title: ['Rinse it', 'all away.'], text: 'A pressure rinse carries the foam and the grime off, front to back.' },
  { key: 'tintOut', hud: 'Window tint', detail: 'Glass prepped', swatch: '#3a3f45', title: ['Pick your', 'shade.'], text: 'The glass is cleaned and prepped so the film goes on flawless. Choose how dark you want it.', tint: true },
  { key: 'tintIn', hud: 'Window tint', detail: 'Film applied', swatch: '#16191d', title: ['On,', 'your way.'], text: 'The film goes on with the shade you picked.' },
  { key: 'coat', hud: 'Ceramic coating', detail: 'Applying', swatch: '#39e1ff', title: ['Panel by panel,', 'deep gloss.'], text: 'The coating is wiped on one section at a time and levelled to a mirror finish.' },
  { key: 'beads', hud: 'Ceramic coating', detail: 'Water test', swatch: '#39e1ff', title: ['The water', 'test.'], text: 'Pour water on it: tight beads that roll straight off.' },
  { key: 'film', hud: 'Paint protection film', detail: 'Fitted', swatch: '#39e1ff', title: ['Every panel,', 'cut to fit.'], text: 'Clear film comes down onto each panel and is trimmed edge to edge.' },
  { key: 'done', hud: 'Ready', detail: 'Door up', swatch: '#fb1a2b', title: ['Ready for', 'the road.'], text: 'Detailed, tinted, coated and protected. Next stop: the coast.' },
];
const tints = [70, 50, 35, 20, 5];

type Home = { progress: number; view: string | null; tint: number | null; keys: string[]; side: number };
declare global { interface Window { dapperHome?: Home; __dapperStage?: number } }

export default function Story() {
  const root = useRef<HTMLElement>(null);
  const tl = useRef<gsap.core.Timeline | null>(null);

  // The renderer module is loaded fresh for each mount and released on unmount.
  useEffect(() => {
    const n = (window.__dapperStage = (window.__dapperStage || 0) + 1);
    import(/* webpackIgnore: true */ '/site/home-stage.js?m=' + n).catch(() => root.current?.setAttribute('data-scene', 'fallback'));
    return () => { window.dispatchEvent(new Event('dapper:dispose')); };
  }, []);

  useGSAP(() => {
    const el = root.current!;
    const layers = [...el.querySelectorAll<HTMLElement>('.story-step')];
    const dots = [...el.querySelectorAll<HTMLButtonElement>('.story-dots button')];
    const hudLabel = el.querySelector('[data-hud-label]')!;
    const hudDetail = el.querySelector('[data-hud-detail]')!;
    const hudSwatch = el.querySelector<HTMLElement>('[data-hud-swatch]')!;
    const chip = el.querySelector<HTMLElement>('.story-hud')!;
    const home: Home = (window.dapperHome = { progress: 0, view: null, tint: 50, keys: steps.map((s) => s.key), side: 1 });
    const n = steps.length;
    const publish = (p: number) => { home.progress = p; dispatchEvent(new Event('dapper:home')); };

    let current = -1;
    const setActive = (i: number) => {
      if (i === current) return;
      current = i;
      layers.forEach((s, k) => { s.setAttribute('aria-hidden', String(k !== i)); s.classList.toggle('on', k === i); });
      el.dataset.side = i % 2 ? 'right' : 'left';
      home.side = i % 2 ? -1 : 1;
      dispatchEvent(new Event('dapper:step'));
      dots.forEach((d, k) => d.setAttribute('aria-current', String(k === i)));
      hudLabel.textContent = steps[i].hud;
      hudDetail.textContent = steps[i].detail;
      hudSwatch.style.background = steps[i].swatch;
      chip.classList.remove('pop'); void chip.offsetWidth; chip.classList.add('pop');
    };
    setActive(0);

    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      el.classList.add('is-pinned');
      // The timeline is only a clock from 0 to the last step; each step's words fade with CSS when it becomes active.
      const t = gsap.timeline({
        defaults: { ease: 'none' },
        scrollTrigger: {
          trigger: el,
          start: 'top top',
          end: () => '+=' + Math.round(innerHeight * (n - 1) * 0.85 + innerHeight * 0.6),
          pin: el.querySelector('.story-pin'),
          scrub: 0.6,
          anticipatePin: 1,
          invalidateOnRefresh: true,
          snap: { snapTo: 'labels', duration: { min: 0.25, max: 0.7 }, delay: 0.08, ease: 'power2.inOut' },
          onUpdate: () => {
            const p = t.time();
            publish(p);
            const k = Math.round(p);
            setActive(Math.min(n - 1, k));
            if (k < n - 1) home.view = null;
          },
        },
      });
      steps.forEach((_, i) => t.addLabel('s' + i, i));
      t.to({}, { duration: n - 1 }, 0);
      tl.current = t;
      return () => { el.classList.remove('is-pinned'); tl.current = null; };
    });
    mm.add('(prefers-reduced-motion: reduce)', () => {
      el.classList.add('is-static');
      layers.forEach((s, i) => ScrollTrigger.create({ trigger: s, start: 'top 60%', end: 'bottom 40%', onToggle: (self) => { if (self.isActive) { setActive(i); publish(i); } } }));
      return () => el.classList.remove('is-static');
    });
  }, { scope: root });

  const goTo = (i: number) => {
    const st = tl.current?.scrollTrigger;
    if (st) scrollTo({ top: st.start + (st.end - st.start) * (i / (steps.length - 1)), behavior: 'smooth' });
  };
  const pickTint = (v: number) => {
    const home = window.dapperHome;
    if (!home) return;
    home.tint = v;
    root.current?.querySelectorAll('[data-try-tint]').forEach((b) => b.setAttribute('aria-pressed', String(Number((b as HTMLElement).dataset.tryTint) === v)));
    const label = root.current?.querySelector('[data-hud-label]');
    const detail = root.current?.querySelector('[data-hud-detail]');
    if (label) label.textContent = 'Your build';
    if (detail) detail.textContent = v + '% VLT';
    dispatchEvent(new Event('dapper:home'));
  };

  return (
    <section ref={root} className="story story3d" id="story" data-scene="loading" aria-labelledby="story-heading">
      <h1 id="story-heading" className="sr">Dapper Detailing Pros: vehicle care in Peterborough, Ontario</h1>
      <div className="story-pin">
        <div className="story-poster" aria-hidden="true" />
        <div id="home-stage" className="home-stage" aria-hidden="true" />
        <div id="home-curtain" className="home-curtain" aria-hidden="true" />
        <div className="story-shade" aria-hidden="true" />
        <div className="story-loading" aria-hidden="true"><span />Warming up the studio</div>
        <div className="story-layers">
          {steps.map((s) => (
            <article key={s.key} className="story-step" data-key={s.key}>
              <div className="story-copy">
                <h2>{s.title[0]}<br /><em>{s.title[1]}</em></h2>
                <p>{s.text}</p>
                {s.tint && (
                  <div className="try" role="group" aria-label="Choose your tint">
                    <p className="try-label">Choose your shade (visible light)</p>
                    <div className="try-row">
                      {tints.map((v) => <button key={v} type="button" className="try-tint" data-try-tint={v} aria-pressed={v === 50} onClick={() => pickTint(v)}>{v}%</button>)}
                    </div>
                  </div>
                )}
              </div>
            </article>
          ))}
        </div>
        <div className="story-hud" aria-live="polite"><i data-hud-swatch /><b data-hud-label /><span data-hud-detail /></div>
        <p className="drag-hint" aria-hidden="true"><span>⟷</span>Drag to look around</p>
        <div className="story-dots" aria-label="Story progress">
          {steps.map((s, i) => <button key={s.key} type="button" aria-label={'Go to step ' + (i + 1)} onClick={() => goTo(i)} />)}
        </div>
        <div className="story-bar" role="group" aria-label="Quick actions">
          <a className="pill light" href="#quote">Instant quote <span aria-hidden="true">↗</span></a>
          <a className="pill red" href="/booking">Book an appointment <span aria-hidden="true">↗</span></a>
        </div>
        <p className="story-note">Real-time 3D visualisation of our process. Tint levels are illustrative; ask us what suits your vehicle.</p>
      </div>
    </section>
  );
}

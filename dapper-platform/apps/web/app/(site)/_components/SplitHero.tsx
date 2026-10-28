'use client';
import { useRef } from 'react';
import type { ReactNode } from 'react';
import { gsap, SplitText, useGSAP } from './gsap';
import { MoveHorizontal } from './Icons';

type Props = {
  image: string;
  alt: string;
  before: string;
  after: string;
  lead: string;
  children?: ReactNode;
  // True when the photo shows the studio rather than the vehicle type the page is about.
  studioPhoto?: boolean;
};

const clamp = gsap.utils.clamp(4, 96);

/* The 50/50 test panel. One photo, two finishes: the left of the tape is shown as it arrives (an illustration made from the
   same photo), the right as it leaves. The visitor drags the tape, or uses the arrow keys on it. */
export default function SplitHero({ image, alt, before, after, lead, children, studioPhoto }: Props) {
  const root = useRef<HTMLElement>(null);
  const tape = useRef<HTMLDivElement>(null);
  const pos = useRef({ x: 50 });

  const { contextSafe } = useGSAP(() => {
    const el = root.current!;
    const set = () => {
      el.style.setProperty('--x', pos.current.x + '%');
      tape.current?.setAttribute('aria-valuenow', String(Math.round(pos.current.x)));
    };
    const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) { set(); return; }

    // Lay the tape from the top, then sweep it from mostly-before to the middle so the change is seen happening.
    const split = SplitText.create(el.querySelectorAll('.split__title .line'), { type: 'lines', mask: 'lines' });
    pos.current.x = 88; set();
    const tl = gsap.timeline({ defaults: { ease: 'expo.out' } });
    tl.from(el.querySelectorAll('.split__img'), { scale: 1.12, duration: 2.2, ease: 'power3.out' }, 0)
      .from(tape.current, { scaleY: 0, duration: 1, ease: 'expo.inOut' }, 0.15)
      .to(pos.current, { x: 50, duration: 1.6, ease: 'power4.inOut', onUpdate: set }, 0.9)
      .from(split.lines, { yPercent: 105, duration: 1.2, stagger: 0.08 }, 1)
      .from(el.querySelectorAll('.split__label, .split__lead, .split__actions > *'), { y: 18, opacity: 0, stagger: 0.06, duration: 0.9 }, 1.4)
      .from(el.querySelectorAll('.split__grip'), { scale: 0, opacity: 0, duration: 0.7, ease: 'back.out(2)' }, 2.2);

    // Scrolling away lifts the photo slightly slower than the page.
    gsap.to(el.querySelectorAll('.split__img'), { yPercent: 8, ease: 'none', scrollTrigger: { trigger: el, start: 'top top', end: 'bottom top', scrub: true } });
  }, { scope: root });

  const moveTo = contextSafe((x: number, immediate = false) => {
    const el = root.current!;
    el.classList.add('is-used'); // the visitor has found the tape; the cue goes
    gsap.to(pos.current, {
      x: clamp(x), duration: immediate ? 0.25 : 0.6, ease: 'power3.out', overwrite: true,
      onUpdate: () => { el.style.setProperty('--x', pos.current.x + '%'); tape.current?.setAttribute('aria-valuenow', String(Math.round(pos.current.x))); },
    });
  });

  const onPointerDown = (e: React.PointerEvent) => {
    const el = root.current!;
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    const box = el.getBoundingClientRect();
    const move = (ev: PointerEvent) => moveTo(((ev.clientX - box.left) / box.width) * 100, true);
    const up = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', up); removeEventListener('pointercancel', up); };
    addEventListener('pointermove', move);
    addEventListener('pointerup', up);
    addEventListener('pointercancel', up);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const step = e.shiftKey ? 15 : 5;
    if (e.key === 'ArrowLeft') moveTo(pos.current.x - step);
    else if (e.key === 'ArrowRight') moveTo(pos.current.x + step);
    else if (e.key === 'Home') moveTo(4);
    else if (e.key === 'End') moveTo(96);
    else return;
    e.preventDefault();
  };

  return (
    <section ref={root} className="split" aria-labelledby="page-title">
      <div className="split__media">
      <img className="split__img split__after" src={image} alt={alt} fetchPriority="high" />
      <div className="split__before" aria-hidden="true"><img className="split__img" src={image} alt="" /></div>
      <div className="split__shade" aria-hidden="true" />
      <span className="tape tape--tilt split__label split__label--before">Before · illustration</span>
      <span className="tape tape--red tape--tilt-r split__label split__label--after">After</span>
      <div
        ref={tape}
        className="split__tape"
        role="slider"
        tabIndex={0}
        aria-label="Before and after divider"
        aria-valuemin={4}
        aria-valuemax={96}
        aria-valuenow={50}
        aria-valuetext="Drag to compare before and after"
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
      >
        <span aria-hidden="true">Dapper · 50/50</span>
        <i className="split__grip" aria-hidden="true"><MoveHorizontal size={22} /></i>
      </div>
      <span className="tape split__cue" aria-hidden="true">Drag the tape</span>
      </div>
      <div className="split__copy">
        <h1 id="page-title" className="split__title">
          <span className="b"><span className="line">{before}</span></span>
          <span className="a"><span className="line">{after}</span></span>
        </h1>
        <p className="split__lead">{lead}</p>
        <div className="split__actions">{children}</div>
      </div>
      {studioPhoto && <span className="split__credit">Photo: the Dapper studio, Peterborough</span>}
    </section>
  );
}

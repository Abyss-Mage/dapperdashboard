'use client';

import { useEffect, useLayoutEffect, useRef, type ReactNode } from 'react';
import gsap from 'gsap';

/* Height-animated show / hide. Closed content is hidden from sight and from assistive tech; the height tween is cleared when it
   finishes so the content lays out normally again. Reduced motion switches instantly. */

const useIsoLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;
const CLEAR = 'height,opacity,visibility,overflow';

export default function Collapse({ open, children, className }: { open: boolean; children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const first = useRef(true);
  useIsoLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (first.current) { first.current = false; el.style.display = open ? '' : 'none'; return; }
    gsap.killTweensOf(el);
    const instant = matchMedia('(prefers-reduced-motion: reduce)').matches;
    // Very tall content (hundreds of rows) fades and lifts instead of resizing every frame, which stays smooth on slow devices.
    if (open) el.style.display = '';
    const tall = el.scrollHeight > 1400; // measured after it is shown, when opening
    if (open) {
      if (instant) { gsap.set(el, { clearProps: CLEAR }); return; }
      if (tall) { gsap.fromTo(el, { autoAlpha: 0, y: -14 }, { autoAlpha: 1, y: 0, duration: 0.4, ease: 'power3.out', clearProps: 'opacity,visibility,transform' }); return; }
      gsap.fromTo(el, { height: 0, autoAlpha: 0, overflow: 'hidden' }, { height: 'auto', autoAlpha: 1, duration: 0.5, ease: 'power3.out', clearProps: CLEAR });
    } else if (instant) el.style.display = 'none';
    else if (tall) gsap.to(el, { autoAlpha: 0, y: -10, duration: 0.22, ease: 'power2.in', onComplete: () => { gsap.set(el, { clearProps: 'opacity,visibility,transform' }); el.style.display = 'none'; } });
    else gsap.to(el, { height: 0, autoAlpha: 0, overflow: 'hidden', duration: 0.38, ease: 'power3.inOut', onComplete: () => { gsap.set(el, { clearProps: CLEAR }); el.style.display = 'none'; } });
  }, [open]);
  return <div ref={ref} className={className} aria-hidden={!open || undefined} {...(!open ? { inert: true } : {})}>{children}</div>;
}

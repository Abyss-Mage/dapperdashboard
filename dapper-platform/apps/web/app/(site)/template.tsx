'use client';
import { useRef } from 'react';
import type { ReactNode } from 'react';
import { gsap, useGSAP } from './_components/gsap';

/* Each page arrives under a strip of tape that is peeled away to the right. The first load skips it so the home page opens on
   its own story. */
let seen = false;

export default function Template({ children }: { children: ReactNode }) {
  const strip = useRef<HTMLElement>(null);
  const first = useRef(!seen);

  useGSAP(() => {
    const was = first.current;
    seen = true;
    if (was || matchMedia('(prefers-reduced-motion: reduce)').matches) { gsap.set(strip.current, { display: 'none' }); return; }
    gsap.timeline()
      .set(strip.current, { scaleX: 1, transformOrigin: 'right' })
      .to(strip.current, { scaleX: 0, duration: 0.9, ease: 'expo.inOut' })
      .set(strip.current, { display: 'none' });
  });

  return (
    <>
      <div className="sweep" aria-hidden="true"><i ref={strip} /></div>
      {children}
    </>
  );
}

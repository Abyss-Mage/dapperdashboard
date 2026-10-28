'use client';
import { useEffect } from 'react';
import { usePathname } from 'next/navigation';
import Lenis from 'lenis';
import { gsap, ScrollTrigger } from './gsap';

/* Lenis smooths the wheel and tells ScrollTrigger on every frame, so pinned and scrubbed sections move with the page.
   Off for touch-first devices and for visitors who prefer reduced motion: they get the browser's own scrolling. */
let lenis: Lenis | null = null;

export default function SmoothScroll() {
  const pathname = usePathname();

  useEffect(() => {
    const reduced = matchMedia('(prefers-reduced-motion: reduce)');
    const fine = matchMedia('(pointer: fine)');
    const tick = (time: number) => lenis?.raf(time * 1000);
    const sync = () => {
      const on = !reduced.matches && fine.matches;
      if (on && !lenis) {
        lenis = new Lenis({ lerp: 0.09, smoothWheel: true, prevent: (node) => !!node.closest?.('dialog') });
        lenis.on('scroll', ScrollTrigger.update);
        gsap.ticker.add(tick);
        gsap.ticker.lagSmoothing(0);
      } else if (!on && lenis) {
        gsap.ticker.remove(tick);
        lenis.destroy();
        lenis = null;
      }
    };
    sync();
    reduced.addEventListener('change', sync);
    fine.addEventListener('change', sync);
    return () => {
      reduced.removeEventListener('change', sync);
      fine.removeEventListener('change', sync);
      gsap.ticker.remove(tick);
      lenis?.destroy();
      lenis = null;
    };
  }, []);

  // A new page starts at the top, and every trigger measures the new layout.
  useEffect(() => {
    lenis?.scrollTo(0, { immediate: true, force: true });
    const id = requestAnimationFrame(() => ScrollTrigger.refresh());
    return () => cancelAnimationFrame(id);
  }, [pathname]);

  return null;
}

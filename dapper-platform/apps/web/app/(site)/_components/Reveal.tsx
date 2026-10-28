'use client';
import { useRef } from 'react';
import type { ReactNode } from 'react';
import { gsap, LAY, ScrollTrigger, SplitText, useGSAP } from './gsap';

/* The site's motion grammar, applied to whatever a section marks up:
   [data-split]  headings rise line by line out of a mask
   [data-lay]    tape is laid from one end (data-lay="y" lays downward), with a slight press-down overshoot
   [data-peel]   photos are uncovered as if a strip were peeled off them
   [data-rise]   everything else settles up into place, in reading order
   [data-scrub]  tape that grows with the scroll (the timeline and process lines)
   Content is visible without JavaScript and for reduced motion; the animation only runs from the start state. */
export default function Reveal({ as: Tag = 'section', className, id, children, ...rest }: { as?: 'section' | 'div' | 'footer'; className?: string; id?: string; children: ReactNode; 'aria-labelledby'?: string }) {
  const root = useRef<HTMLElement>(null);

  useGSAP(() => {
    const el = root.current!;
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const at = (node: Element, start = 'top 85%') => ({ trigger: node, start, once: true });

    el.querySelectorAll<HTMLElement>('[data-split]').forEach((node) => {
      const split = SplitText.create(node, { type: 'lines', mask: 'lines', autoSplit: true, onSplit: (self) =>
        gsap.from(self.lines, { yPercent: 110, duration: 1.15, stagger: 0.09, scrollTrigger: at(node) }) });
      void split;
    });
    el.querySelectorAll<HTMLElement>('[data-lay]').forEach((node) => {
      const down = node.dataset.lay === 'y';
      gsap.from(node, { [down ? 'scaleY' : 'scaleX']: 0, transformOrigin: down ? 'top' : node.dataset.lay === 'right' ? 'right' : 'left', duration: 1.1, ease: down ? 'expo.inOut' : LAY, scrollTrigger: at(node, 'top 90%') });
    });
    el.querySelectorAll<HTMLElement>('[data-peel]').forEach((node) => {
      gsap.from(node, { clipPath: 'inset(0 100% 0 0)', duration: 1.4, ease: 'expo.inOut', scrollTrigger: at(node, 'top 88%') });
      const img = node.querySelector('img');
      if (img) gsap.from(img, { scale: 1.18, duration: 1.8, ease: 'power3.out', scrollTrigger: at(node, 'top 88%') });
    });
    el.querySelectorAll<HTMLElement>('[data-scrub]').forEach((node) => {
      const down = node.dataset.scrub === 'y';
      gsap.fromTo(node, { [down ? 'scaleY' : 'scaleX']: 0 }, { [down ? 'scaleY' : 'scaleX']: 1, ease: 'none', scrollTrigger: { trigger: node.parentElement, start: 'top 75%', end: 'bottom 70%', scrub: 0.6 } });
    });
    const rise = gsap.utils.toArray<HTMLElement>(el.querySelectorAll('[data-rise]'));
    if (rise.length) {
      gsap.set(rise, { y: 28, opacity: 0 });
      ScrollTrigger.batch(rise, { start: 'top 90%', once: true, onEnter: (batch) => gsap.to(batch, { y: 0, opacity: 1, duration: 1, stagger: 0.07, overwrite: true }) });
    }
  }, { scope: root });

  return <Tag ref={root as never} className={className} id={id} {...rest}>{children}</Tag>;
}

'use client';
import Link from 'next/link';
import { useRef } from 'react';
import { categories } from '../content';
import { gsap, useGSAP } from './gsap';
import { ArrowUpRight } from './Icons';

/* The service directory: ten rows of big type. On a pointer device a taped photo of the vehicle type follows the cursor. */
export default function ServiceRows() {
  const root = useRef<HTMLDivElement>(null);
  const peek = useRef<HTMLDivElement>(null);
  const img = useRef<HTMLImageElement>(null);

  const { contextSafe } = useGSAP(() => {
    if (!matchMedia('(hover: hover) and (pointer: fine)').matches || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const x = gsap.quickTo(peek.current, 'x', { duration: 0.55, ease: 'power3' });
    const y = gsap.quickTo(peek.current, 'y', { duration: 0.55, ease: 'power3' });
    const r = gsap.quickTo(peek.current, 'rotate', { duration: 0.8, ease: 'power3' });
    let lastX = 0;
    const move = (e: PointerEvent) => {
      x(e.clientX + 28); y(e.clientY - 120);
      r(gsap.utils.clamp(-8, 8, (e.clientX - lastX) * 0.6)); lastX = e.clientX;
    };
    root.current!.addEventListener('pointermove', move);
    return () => root.current?.removeEventListener('pointermove', move);
  }, { scope: root });

  const enter = contextSafe((src: string) => {
    if (!matchMedia('(hover: hover) and (pointer: fine)').matches) return;
    if (img.current && img.current.getAttribute('src') !== src) img.current.src = src;
    gsap.to(peek.current, { opacity: 1, scale: 1, duration: 0.5, ease: 'back.out(1.6)', overwrite: 'auto' });
  });
  const leave = contextSafe(() => gsap.to(peek.current, { opacity: 0, scale: 0.6, duration: 0.35, ease: 'power3.in', overwrite: 'auto' }));

  return (
    <div ref={root} className="rows" onPointerLeave={leave}>
      {categories.map((c) => (
        <Link key={c.slug} href={'/' + c.slug} className="row" data-rise onPointerEnter={() => enter(c.image)}>
          <h2>{c.title}</h2>
          <p>{c.summary}</p>
          <ArrowUpRight size={30} aria-hidden />
        </Link>
      ))}
      <div ref={peek} className="row-peek taped" aria-hidden="true"><img ref={img} src={categories[0].image} alt="" /></div>
    </div>
  );
}

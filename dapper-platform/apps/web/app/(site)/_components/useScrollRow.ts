'use client';
import { useEffect, useState, type RefObject } from 'react';

/* A row you can move three ways: swipe (touch, native), drag with the mouse, or let it step along by itself.
   - Dragging only starts once the pointer has moved a few pixels, so a plain click still opens the card or film under it.
   - `auto` is the pause in milliseconds between steps (0 = off). It stops while the pointer or focus is on the row, while the page
     is hidden or the row is off screen, and always with reduced motion; after reaching the end it winds back to the start.
   Returns whether the row is at its start or end, for the arrow buttons. */
export function useScrollRow(ref: RefObject<HTMLElement | null>, auto = 0) {
  const [edge, setEdge] = useState({ start: true, end: false });
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const measure = () => setEdge({ start: el.scrollLeft < 8, end: el.scrollLeft + el.clientWidth >= el.scrollWidth - 8 });
    let down = false, grabbed = false, startX = 0, startLeft = 0, moved = 0, hold = false, visible = true, resume = 0;
    const onDown = (e: PointerEvent) => { if (e.pointerType !== 'mouse' || e.button !== 0) return; down = true; grabbed = false; moved = 0; startX = e.clientX; startLeft = el.scrollLeft; };
    const onMove = (e: PointerEvent) => {
      if (!down) return;
      const dx = e.clientX - startX;
      moved = Math.max(moved, Math.abs(dx));
      if (moved > 5 && !grabbed) { grabbed = true; el.classList.add('is-dragging'); el.setPointerCapture(e.pointerId); }
      if (grabbed) el.scrollLeft = startLeft - dx;
    };
    const onUp = (e: PointerEvent) => { if (!down) return; down = false; el.classList.remove('is-dragging'); if (grabbed) { try { el.releasePointerCapture(e.pointerId); } catch { /* already released */ } } };
    const onClick = (e: MouseEvent) => { if (grabbed && moved > 5) { e.preventDefault(); e.stopPropagation(); } grabbed = false; };
    const onTouch = () => { hold = true; clearTimeout(resume); resume = window.setTimeout(() => { hold = false; }, 8000); };
    const enter = () => { hold = true; }, leave = () => { hold = false; };
    el.addEventListener('scroll', measure, { passive: true });
    el.addEventListener('pointerdown', onDown); el.addEventListener('pointermove', onMove); el.addEventListener('pointerup', onUp); el.addEventListener('pointercancel', onUp);
    el.addEventListener('click', onClick, true);
    el.addEventListener('touchstart', onTouch, { passive: true });
    el.addEventListener('pointerenter', (e) => { if ((e as PointerEvent).pointerType === 'mouse') enter(); }); el.addEventListener('pointerleave', leave);
    el.addEventListener('focusin', enter); el.addEventListener('focusout', leave);
    window.addEventListener('resize', measure);
    measure();
    const io = new IntersectionObserver(([en]) => { visible = en.isIntersecting; }, { threshold: .3 });
    io.observe(el);
    const timer = auto > 0 && !reduce ? window.setInterval(() => {
      if (hold || down || !visible || document.hidden) return;
      if (el.scrollLeft + el.clientWidth >= el.scrollWidth - 4) { el.scrollTo({ left: 0, behavior: 'smooth' }); return; }
      const a = el.children[0] as HTMLElement | undefined, b = el.children[1] as HTMLElement | undefined;
      el.scrollBy({ left: a && b ? b.offsetLeft - a.offsetLeft : el.clientWidth * .6, behavior: 'smooth' });
    }, auto) : 0;
    return () => {
      clearInterval(timer); clearTimeout(resume); io.disconnect();
      el.removeEventListener('scroll', measure); window.removeEventListener('resize', measure);
      el.removeEventListener('pointerdown', onDown); el.removeEventListener('pointermove', onMove); el.removeEventListener('pointerup', onUp); el.removeEventListener('pointercancel', onUp);
      el.removeEventListener('click', onClick, true); el.removeEventListener('touchstart', onTouch);
    };
  }, [ref, auto]);
  return edge;
}

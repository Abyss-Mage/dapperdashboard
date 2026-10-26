/* Smooth scrolling for the home page: Lenis drives the scroll and tells GSAP ScrollTrigger on every frame, so the pinned story and
   the page scroll feel like one continuous, fluid motion. Off for touch devices and when the visitor prefers reduced motion. */
import Lenis from './assets/three/lenis.mjs';

const reduced = matchMedia('(prefers-reduced-motion: reduce)');
const fine = matchMedia('(pointer: fine)');
let lenis = null;
let frame = 0;

function tick(time) {
  frame = 0;
  if (!lenis || document.hidden) return;
  lenis.raf(time);
  frame = requestAnimationFrame(tick);
}
function sync() {
  const on = !reduced.matches && fine.matches;
  if (on && !lenis) {
    lenis = new Lenis({ lerp: 0.085, smoothWheel: true, anchors: { offset: 0 }, prevent: (node) => node.closest('dialog') });
    lenis.on('scroll', () => window.ScrollTrigger?.update());
    if (!frame) frame = requestAnimationFrame(tick);
  } else if (!on && lenis) {
    lenis.destroy();
    lenis = null;
  }
}
reduced.addEventListener('change', sync);
document.addEventListener('visibilitychange', () => { if (!document.hidden && lenis && !frame) frame = requestAnimationFrame(tick); });
window.addEventListener('load', sync);
sync();

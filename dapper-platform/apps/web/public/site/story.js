/* Scroll story: a pinned, scrubbed sequence (GSAP ScrollTrigger on top of the site's Lenis smooth scroll). Each .story-step is a full
   layer (photo + words). The timeline crossfades layers, drifts the photo, draws the panel outlines and updates the HUD chip.
   Small screens and "reduce motion" skip the pin and show the same steps as a simple stacked list. */
(function () {
  const root = document.getElementById('story');
  if (!root) return;
  const steps = [...root.querySelectorAll('.story-step')];
  const hudLabel = root.querySelector('[data-hud-label]');
  const hudDetail = root.querySelector('[data-hud-detail]');
  const dots = [...root.querySelectorAll('.story-dots button')];
  if (!steps.length) return;

  function setActive(i) {
    steps.forEach((s, n) => s.setAttribute('aria-current', n === i ? 'step' : 'false'));
    dots.forEach((d, n) => d.setAttribute('aria-current', n === i ? 'true' : 'false'));
    if (hudLabel) hudLabel.textContent = steps[i].dataset.hud || '';
    if (hudDetail) hudDetail.textContent = steps[i].dataset.detail || '';
  }
  setActive(0);

  function start() {
    if (!window.gsap || !window.ScrollTrigger) return;
    gsap.registerPlugin(ScrollTrigger);
    const mm = gsap.matchMedia();
    mm.add('(min-width: 800px) and (prefers-reduced-motion: no-preference)', () => {
      root.classList.add('is-pinned');
      const n = steps.length;
      let current = 0;
      const imgs = steps.map((s) => s.querySelector('.story-photo'));
      const copy = steps.map((s) => s.querySelector('.story-copy'));
      const outline = steps.map((s) => [...s.querySelectorAll('.story-outline path')]);
      // Starting poses: only the first layer is visible.
      gsap.set(steps, { autoAlpha: 0 });
      gsap.set(steps[0], { autoAlpha: 1 });
      gsap.set(imgs, { scale: 1.07 });
      gsap.set(copy, { y: 36, autoAlpha: 0 });
      gsap.set(copy[0], { y: 0, autoAlpha: 1 });
      outline.flat().forEach((p) => { p.style.strokeDasharray = '1'; p.style.strokeDashoffset = '1'; });

      const tl = gsap.timeline({
        defaults: { ease: 'none' },
        scrollTrigger: {
          trigger: root,
          start: 'top top',
          end: () => '+=' + Math.round(window.innerHeight * n * 0.9),
          pin: root.querySelector('.story-pin'),
          scrub: 0.7,
          anticipatePin: 1,
          invalidateOnRefresh: true,
          snap: { snapTo: 'labels', duration: { min: 0.2, max: 0.6 }, delay: 0.06, ease: 'power1.inOut' },
          onUpdate: (self) => {
            const i = Math.round(self.progress * (n - 1));
            if (i !== current) { current = i; setActive(i); }
          },
        },
      });
      steps.forEach((step, i) => {
        tl.addLabel('s' + i, i);
        // the photo slowly settles while its step is on screen
        tl.to(imgs[i], { scale: 1, duration: 1, ease: 'none' }, i - 0.5);
        if (i === 0) return;
        const fade = 0.45;
        tl.to(copy[i - 1], { autoAlpha: 0, y: -28, duration: 0.3, ease: 'power2.in' }, i - fade);
        tl.to(steps[i - 1], { autoAlpha: 0, duration: fade, ease: 'power1.inOut' }, i - fade);
        tl.to(steps[i], { autoAlpha: 1, duration: fade, ease: 'power1.inOut' }, i - fade);
        tl.to(copy[i], { autoAlpha: 1, y: 0, duration: 0.4, ease: 'power3.out' }, i - 0.25);
        if (outline[i].length) {
          tl.to(outline[i], { strokeDashoffset: 0, duration: 0.5, stagger: 0.08, ease: 'power2.out' }, i - 0.1);
          tl.to(outline[i], { autoAlpha: 0, duration: 0.3 }, i + 0.55);
        }
      });
      tl.to({}, { duration: 0.001 }, n - 1); // make the last label reachable
      dots.forEach((d, i) => d.addEventListener('click', () => {
        const st = tl.scrollTrigger;
        const y = st.start + (st.end - st.start) * (i / (n - 1));
        window.scrollTo({ top: y, behavior: 'smooth' });
      }));
      return () => { root.classList.remove('is-pinned'); };
    });
  }
  // GSAP is loaded with `defer`; wait for it.
  if (document.readyState === 'complete') start(); else window.addEventListener('load', start, { once: true });
})();

/* Find your care: pick a vehicle, the photo changes and the next step is one tap away. */
(function () {
  const root = document.getElementById('care');
  if (!root) return;
  const tiles = [...root.querySelectorAll('.care-tile')];
  const photos = [...root.querySelectorAll('.care-photo')];
  const line = root.querySelector('[data-care-line]');
  function pick(i) {
    tiles.forEach((t, n) => t.setAttribute('aria-pressed', String(n === i)));
    photos.forEach((p, n) => p.classList.toggle('on', n === i));
    if (line) line.textContent = tiles[i].dataset.line || '';
  }
  tiles.forEach((t, i) => t.addEventListener('click', () => pick(i)));
  pick(0);
})();

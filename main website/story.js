/* Home scroll story. GSAP ScrollTrigger pins the 3D studio and scrubs through the steps; Lenis (home-motion.js) makes the scroll fluid.
   This file only handles words and controls: it fades each step's text in and out, keeps the status chip and dots in step, and
   publishes a continuous progress number that home-scene.js uses to move the camera and change the car. */
(function () {
  const root = document.getElementById('story');
  if (!root) return;
  const steps = [...root.querySelectorAll('.story-step')];
  const chip = root.querySelector('.story-hud');
  const hudLabel = root.querySelector('[data-hud-label]');
  const hudDetail = root.querySelector('[data-hud-detail]');
  const hudSwatch = root.querySelector('[data-hud-swatch]');
  const dots = [...root.querySelectorAll('.story-dots button')];
  const home = (window.dapperHome ||= { progress: 0, view: null, tint: null, keys: [], side: 1 });
  home.keys = steps.map((s) => s.dataset.key);
  home.tint ??= 50; // the shade pre-selected in the tint step
  const n = steps.length;
  if (!n) return;
  const publish = (p) => { home.progress = p; window.dispatchEvent(new Event('dapper:home')); };

  let current = -1;
  function setActive(i) {
    if (i === current) return;
    current = i;
    steps.forEach((s, k) => { s.setAttribute('aria-hidden', String(k !== i)); s.classList.toggle('on', k === i); });
    // words alternate sides each step; the 3D keeps the car on the opposite side
    root.dataset.side = i % 2 ? 'right' : 'left';
    home.side = i % 2 ? -1 : 1;
    window.dispatchEvent(new Event('dapper:step'));
    dots.forEach((d, k) => d.setAttribute('aria-current', String(k === i)));
    const s = steps[i];
    if (hudLabel) hudLabel.textContent = s.dataset.hud || '';
    if (hudDetail) hudDetail.textContent = s.dataset.detail || '';
    if (hudSwatch) hudSwatch.style.background = s.dataset.swatch || '#fb1a2b';
    if (chip) { chip.classList.remove('pop'); void chip.offsetWidth; chip.classList.add('pop'); }
  }
  setActive(0);

  /* "Try it on": the last step lets the visitor pick a service to look at and a tint level. */
  const tryLabel = () => {
    const v = root.querySelector('[data-try-view][aria-pressed=true]');
    if (hudLabel) hudLabel.textContent = 'Your build';
    if (hudDetail) hudDetail.textContent = [v?.textContent.trim(), home.tint ? home.tint + '% VLT' : ''].filter(Boolean).join(' · ') || 'Pick a service and a tint';
    if (hudSwatch && v) hudSwatch.style.background = v.dataset.swatch || '#fb1a2b';
  };
  root.querySelectorAll('[data-try-view]').forEach((b) => b.addEventListener('click', () => {
    home.view = b.dataset.tryView;
    root.querySelectorAll('[data-try-view]').forEach((x) => x.setAttribute('aria-pressed', String(x === b)));
    tryLabel();
    window.dispatchEvent(new Event('dapper:home'));
  }));
  root.querySelectorAll('[data-try-tint]').forEach((b) => b.addEventListener('click', () => {
    home.tint = Number(b.dataset.tryTint);
    root.querySelectorAll('[data-try-tint]').forEach((x) => x.setAttribute('aria-pressed', String(x === b)));
    tryLabel();
    window.dispatchEvent(new Event('dapper:home'));
  }));
  // Leaving the last step hands the car back to the story.
  const clearTry = () => { home.view = null; };

  function start() {
    if (!window.gsap || !window.ScrollTrigger) { root.classList.add('is-static'); return; }
    gsap.registerPlugin(ScrollTrigger);
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      root.classList.add('is-pinned');
      // The timeline is only a clock from 0 to the last step; words fade with CSS when their step becomes active,
      // so the text always matches the car and the status chip.
      const tl = gsap.timeline({
        defaults: { ease: 'none' },
        scrollTrigger: {
          trigger: root,
          start: 'top top',
          end: () => '+=' + Math.round(window.innerHeight * (n - 1) * 0.85 + window.innerHeight * 0.6),
          pin: root.querySelector('.story-pin'),
          scrub: 0.6,
          anticipatePin: 1,
          invalidateOnRefresh: true,
          snap: { snapTo: 'labels', duration: { min: 0.25, max: 0.7 }, delay: 0.08, ease: 'power2.inOut' },
          onUpdate: () => {
            const p = tl.time();
            publish(p);
            const k = Math.round(p);
            setActive(Math.min(n - 1, k));
            if (k < n - 1) clearTry();
          },
        },
      });
      steps.forEach((s, i) => tl.addLabel('s' + i, i));
      tl.to({}, { duration: n - 1 }, 0);
      dots.forEach((d, i) => d.addEventListener('click', () => {
        const st = tl.scrollTrigger;
        window.scrollTo({ top: st.start + (st.end - st.start) * (i / (n - 1)), behavior: 'smooth' });
      }));
      return () => root.classList.remove('is-pinned');
    });
    mm.add('(prefers-reduced-motion: reduce)', () => {
      // No pin and no camera travel: each step is shown in turn as it scrolls past.
      root.classList.add('is-static');
      steps.forEach((s, i) => ScrollTrigger.create({ trigger: s, start: 'top 60%', end: 'bottom 40%', onToggle: (self) => { if (self.isActive) { setActive(i); publish(i); } } }));
      return () => root.classList.remove('is-static');
    });
  }
  if (document.readyState === 'complete') start(); else window.addEventListener('load', start, { once: true });
})();

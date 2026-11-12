import { gsap } from '../_components/gsapContact';
import { STORY, STORY_SPAN } from './story';
import { MOTION } from './frameMotion';

/* The home hero plays a pre-made image sequence (AI-generated frames) with the scroll: the section pins, a canvas draws the frame the
   playhead is on, and the headline leaves and comes back. Frames live in /site/assets/frames/desktop and /mobile (0000.webp ...).
   What keeps it smooth:
   - The playhead follows the scroll with its own damped follower on the GSAP ticker (time based), so the picture glides at the display's
     refresh rate even when wheel or touch input arrives in steps.
   - The playhead is a fractional frame; the two frames either side of it are blended on the canvas.
   - Scroll speed through the film changes gradually (weights are low-pass filtered, and the slow-down at a stage eases in and out), so
     the pace never jumps.
   - Files are fetched nearest the playhead first, but drawing uses decoded ImageBitmaps for a window of frames around the playhead,
     already scaled to the canvas, so a draw never waits on image decoding and the browser never has to re-decode a frame it dropped.
   - The canvas is capped to the frames' own resolution, so no pixels are drawn that the frames do not have.
   Returns a cleanup. */

const COUNT = 288; // frames per set, 0000.webp to 0287.webp; both sets are 16:9 (1600x900 and 960x540)
const dir = (phone: boolean) => '/site/assets/frames/' + (phone ? 'mobile' : 'desktop') + '/';
const file = (i: number) => String(i).padStart(4, '0') + '.webp';

// Scroll each step between frames is worth: more where the picture changes more (MOTION, softened so the pace does not jitter), and up to four times
// that around a stage's words so they can be read, easing in and out over ten frames.
const soften = (a: number[], r: number) => a.map((_, i) => { let s = 0, n = 0; for (let k = i - r; k <= i + r; k++) if (k >= 0 && k < a.length) { s += a[k]; n++; } return s / n; });
const steps = soften(soften(Array.from({ length: COUNT - 1 }, (_, i) => MOTION[i] ?? 1), 8), 8);
const dwell = (c: number) => {
  let d = 99;
  for (const s of STORY) d = Math.min(d, Math.abs(c - s.at));
  const t = Math.min(1, Math.max(0, (STORY_SPAN + 10 - d) / 10));
  return 1 + 3 * t * t * (3 - 2 * t);
};
const cum: number[] = [0];
for (let f = 1; f < COUNT; f++) cum[f] = cum[f - 1] + steps[f - 1] * dwell(f - .5);
// Scroll progress (0..1) to a fractional frame, as a ratio of the last frame (a GSAP ease).
const ease = (p: number) => {
  const t = Math.min(1, Math.max(0, p)) * cum[COUNT - 1];
  let lo = 0, hi = COUNT - 1;
  while (hi - lo > 1) { const mid = (lo + hi) >> 1; if (cum[mid] <= t) lo = mid; else hi = mid; }
  return (lo + (t - cum[lo]) / (cum[hi] - cum[lo])) / (COUNT - 1);
};

export function frameSequence(section: HTMLElement): () => void {
  const canvas = section.querySelector<HTMLCanvasElement>('.h-seq__canvas');
  const ctx = canvas?.getContext('2d');
  if (!canvas || !ctx) return () => {};
  const conn = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection;
  if (conn?.saveData || typeof createImageBitmap !== 'function') return () => {}; // data saver: keep the still poster
  if (matchMedia('(max-width:720px)').matches) return () => {}; // phones play the film the page mounts instead
  // Portrait tablets draw the frames in a box under the header, as phones once did; the sharper desktop set is used either way.
  const phone = matchMedia('(orientation:portrait) and (max-width:1100px)').matches;
  const base = dir(false);
  const SRC_W = 1600, SRC_H = 900;
  const blobs: (Blob | undefined)[] = new Array(COUNT);
  const bmps: (ImageBitmap | undefined)[] = new Array(COUNT);
  const requested = new Set<number>();
  const making = new Set<number>();
  let target = 0, pos = 0, last = 0, busy = 0, dead = false, lit = -2, bmpW = 0, key = '';
  const items = Array.from(section.querySelectorAll<HTMLElement>('.h-story'));
  // The words for the stage the playhead is on (none between stages).
  const words = () => {
    let on = -1;
    STORY.forEach((st, i) => { if (Math.abs(pos - st.at) <= STORY_SPAN) on = i; });
    if (on === lit) return;
    lit = on;
    items.forEach((el, i) => el.classList.toggle('is-on', i === on));
    section.classList.toggle('story-on', on >= 0);
  };

  // On a phone the 16:9 picture fills a box under the header (cropped to its middle, edges faded into the page) so the car stays big and the
  // headline has the lower part of the screen.
  const box = () => { const k = canvas.width / canvas.clientWidth; return { top: 64 * k, height: Math.round(Math.min(canvas.width * .95, canvas.height * .6)) }; };
  const fit = () => {
    // Never more pixels than the frames have.
    const dpr = Math.min(devicePixelRatio || 1, 2, SRC_W / Math.max(1, canvas.clientWidth));
    const w = Math.round(canvas.clientWidth * dpr), h = Math.round(canvas.clientHeight * dpr);
    if (canvas.width !== w || canvas.height !== h) { canvas.width = w; canvas.height = h; key = ''; }
    // The width the decoded bitmaps need so they cover the drawing area exactly.
    const area = phone ? { w: canvas.width, h: box().height } : { w: canvas.width, h: canvas.height };
    const sc = Math.max(area.w / SRC_W, area.h / SRC_H);
    const need = Math.min(SRC_W, Math.ceil(SRC_W * sc));
    if (need !== bmpW) { bmpW = need; bmps.forEach((b, i) => { b?.close(); bmps[i] = undefined; }); key = ''; }
  };
  const nearest = (i: number) => {
    for (let d = 0; d < COUNT; d++) {
      if (bmps[i - d]) return i - d;
      if (bmps[i + d]) return i + d;
    }
    return -1;
  };
  const paint = (img: ImageBitmap, alpha: number) => {
    const cw = canvas.width, ch = canvas.height;
    ctx.globalAlpha = alpha;
    if (phone) {
      const { top, height } = box();
      const sc = Math.max(cw / img.width, height / img.height);
      const w = img.width * sc, h = img.height * sc;
      ctx.save();
      ctx.beginPath();
      ctx.rect(0, top, cw, height);
      ctx.clip();
      ctx.drawImage(img, (cw - w) / 2, top + (height - h) / 2, w, h);
      ctx.restore();
    } else {
      const sc = Math.max(cw / img.width, ch / img.height);
      const w = img.width * sc, h = img.height * sc;
      ctx.drawImage(img, (cw - w) / 2, (ch - h) / 2, w, h);
    }
    ctx.globalAlpha = 1;
  };
  const fadeEdges = () => {
    const { top, height } = box();
    const fade = Math.round(height * .14);
    for (const [y0, y1] of [[top, top + fade], [top + height, top + height - fade]] as const) {
      const g = ctx.createLinearGradient(0, y0, 0, y1);
      g.addColorStop(0, 'rgba(11,11,12,1)');
      g.addColorStop(1, 'rgba(11,11,12,0)');
      ctx.fillStyle = g;
      ctx.fillRect(0, Math.min(y0, y1), canvas.width, fade);
    }
  };
  const draw = () => {
    const f = Math.min(COUNT - 1, Math.max(0, pos));
    const lo = Math.floor(f), frac = f - lo;
    const a = nearest(lo);
    if (a < 0) return;
    const b = frac > .01 ? nearest(Math.min(COUNT - 1, lo + 1)) : a;
    // Smoothstep, so a frame takes over gently and never pops at the moment the playhead crosses it.
    const mix = frac * frac * (3 - 2 * frac);
    const k = a + '|' + b + '|' + Math.round(mix * 48) + '|' + canvas.width;
    if (k === key) return;
    key = k;
    ctx.fillStyle = '#0b0b0c';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    paint(bmps[a]!, 1);
    // The next frame fades in over this one by how far the playhead is between them.
    if (b >= 0 && b !== a) paint(bmps[b]!, mix);
    if (phone) fadeEdges();
    section.classList.add('is-playing');
  };

  // Keep decoded bitmaps for a window around the playhead (more ahead than behind) and let go of the rest.
  const keep = () => {
    const fwd = pos >= last;
    const from = Math.floor(pos) - (fwd ? 10 : 36), to = Math.ceil(pos) + (fwd ? 36 : 10);
    for (let i = 0; i < COUNT; i++) {
      if (bmps[i] && (i < from - 12 || i > to + 12)) { bmps[i]!.close(); bmps[i] = undefined; }
    }
    // Nearest to the playhead first.
    const order: number[] = [];
    for (let i = Math.max(0, from); i <= Math.min(COUNT - 1, to); i++) if (blobs[i] && !bmps[i] && !making.has(i)) order.push(i);
    order.sort((x, y) => Math.abs(x - pos) - Math.abs(y - pos));
    for (const i of order) {
      if (making.size >= 4) break;
      making.add(i);
      const blob = blobs[i]!;
      const opts: ImageBitmapOptions = bmpW < SRC_W ? { resizeWidth: bmpW, resizeQuality: 'high' } : {};
      createImageBitmap(blob, opts).then((bm) => {
        making.delete(i);
        if (dead || bm.width !== (bmpW < SRC_W ? bmpW : bm.width) || bmps[i]) { bm.close(); } else { bmps[i] = bm; }
        if (!dead) { key = ''; keep(); draw(); }
      }, () => { making.delete(i); });
    }
  };

  // Fetch files a few at a time, always the nearest one to the playhead that has not been asked for yet.
  const pump = () => {
    while (!dead && busy < 6) {
      const c = Math.round(target);
      let pick = -1;
      for (let d = 0; d < COUNT && pick < 0; d++) {
        if (c + d < COUNT && !requested.has(c + d)) pick = c + d;
        else if (c - d >= 0 && !requested.has(c - d)) pick = c - d;
      }
      if (pick < 0) return;
      requested.add(pick);
      busy++;
      const idx = pick;
      fetch(base + file(idx)).then((r) => (r.ok ? r.blob() : Promise.reject())).then((b) => { blobs[idx] = b; }, () => {}).then(() => {
        busy--;
        if (!dead) { keep(); pump(); }
      });
    }
  };

  // The playhead follows the scroll target with a time-based glide (about a tenth of a second behind), whatever the input rate.
  const tick = (_t: number, ms: number) => {
    const dt = Math.min(.1, ms / 1000);
    const diff = target - pos;
    if (Math.abs(diff) < .003) { if (diff !== 0) pos = target; else return; }
    else pos += diff * (1 - Math.exp(-dt * 9));
    keep();
    draw();
    words();
    pump();
    last = pos;
  };

  fit();
  const onResize = () => { fit(); keep(); draw(); };
  addEventListener('resize', onResize);
  pump();
  gsap.ticker.add(tick);

  const proxy = { f: 0 };
  const pin = Math.round(innerHeight * (phone ? 15 : 22));
  const tl = gsap.timeline({ defaults: { ease: 'none' }, scrollTrigger: { trigger: section, start: 'top top', end: '+=' + pin, pin: true, scrub: .3, anticipatePin: 1, invalidateOnRefresh: true, refreshPriority: 3 } });
  // The film runs to progress .9; the last tenth holds the final frame with its closing words and the booking call (the opening headline does not return, so the two never overlap).
  tl.to(proxy, { f: COUNT - 1, duration: .9, ease, onUpdate: () => { target = proxy.f; } }, 0)
    .to('.h-hero__in', { opacity: 0, y: -48, duration: .08, ease: 'power2.in' }, .02)
    // The dark shade is for the headline; while the film plays it lifts so the frames show at full strength, and returns with the headline.
    .to('.h-hero__shade', { opacity: .18, duration: .08 }, .02)
    .to('.h-seq__bar i', { scaleX: 1, duration: 1 }, 0);

  return () => {
    dead = true;
    gsap.ticker.remove(tick);
    removeEventListener('resize', onResize);
    tl.scrollTrigger?.kill();
    tl.kill();
    bmps.forEach((b) => b?.close());
    bmps.fill(undefined);
    blobs.fill(undefined);
  };
}

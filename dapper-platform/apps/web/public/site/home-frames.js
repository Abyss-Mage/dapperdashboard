/* Home story as a photographic frame sequence, scrubbed by scroll.
   story.js publishes window.dapperHome.progress (0 .. steps-1). Each story step has a frame number in manifest.json ("keys");
   between two steps the playhead moves from one frame number to the next, so the footage always rests exactly on the step's frame.
   Frames are drawn on a canvas, cover-fitted, with the next frame cross-faded in so a sparse sequence still moves smoothly.
   Loading is progressive: the step frames first, then every 6th, 3rd and finally every frame; the nearest loaded frame is shown meanwhile.
   Optional tint stills (manifest.tint) are cross-faded in on the "tintIn" step when the visitor picks a shade.
   Format and authoring rules: FRAME_SEQUENCE_SPEC.md. */
const stage = document.getElementById('home-stage');
const story = document.getElementById('story');
const home = (window.dapperHome ||= {progress: 0, view: null, tint: null, keys: [], side: 1});
const base = new URL('./assets/frames/', import.meta.url);
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const saveData = !!navigator.connection?.saveData;
const clamp01 = (v) => Math.min(1, Math.max(0, v)), ease = (t) => t * t * (3 - 2 * t);

const manifest = await (await fetch(new URL('manifest.json', base), {cache: 'no-cache'})).json();
const pad = manifest.pad || 4;
const small = () => stage.clientWidth < 760 && !!manifest.mobile;
const pick = () => (small() ? manifest.mobile : manifest.desktop || manifest.mobile);

const canvas = document.createElement('canvas'), ctx = canvas.getContext('2d', {alpha: false});
canvas.setAttribute('aria-hidden', 'true'); stage.append(canvas);

let seq = pick(), frames = new Map(), tints = new Map(), queue = [], active = 0, drawn = '', dirty = true, ready = false;
const urlOf = (dir, ext, name) => new URL(dir + name + '.' + ext, base);
const frameUrl = (i) => urlOf(seq.dir, seq.ext || 'webp', String(i).padStart(pad, '0'));

function fetchImage(url) {
  return new Promise((resolve) => {
    const img = new Image(); img.decoding = 'async';
    img.onload = () => resolve(img); img.onerror = () => resolve(null);
    img.src = url.href;
  });
}
function request(i) {
  if (i < 0 || i >= seq.count || frames.has(i)) return;
  frames.set(i, null); queue.push(i); pump();
}
function pump() {
  while (active < 6 && queue.length) {
    const i = queue.shift(), mine = seq; active++;
    fetchImage(frameUrl(i)).then((img) => {
      active--; if (mine !== seq) return;
      if (img) { frames.set(i, img); dirty = true; draw(); if (!ready && i === keyFrames()[0]) markReady(); } else frames.delete(i);
      pump();
    });
  }
}
/* the frame number each story step rests on; steps without a number are spread evenly between their neighbours */
function keyFrames() {
  const keys = home.keys.length ? home.keys : Object.keys(manifest.keys || {}), out = keys.map((k) => manifest.keys?.[k]);
  for (let i = 0; i < out.length; i++) if (out[i] == null) out[i] = Math.round((i / Math.max(1, out.length - 1)) * (seq.count - 1));
  return out;
}
function plan() {
  const need = new Set(), K = keyFrames(); K.forEach((f) => need.add(f));
  for (const stride of saveData ? [6] : [6, 3, 1]) for (let i = 0; i < seq.count; i += stride) need.add(i);
  need.add(seq.count - 1);
  [...need].forEach(request);
}
const markReady = () => { ready = true; story?.setAttribute('data-scene', 'ready'); };

function loadedNear(f) {
  const i = Math.round(f);
  for (let d = 0; d < seq.count; d++) { const a = frames.get(i - d), b = frames.get(i + d); if (a) return a; if (b) return b; }
  return null;
}
function cover(img, alpha) {
  const cw = canvas.width, ch = canvas.height, s = Math.max(cw / img.naturalWidth, ch / img.naturalHeight), w = img.naturalWidth * s, h = img.naturalHeight * s;
  ctx.globalAlpha = alpha; ctx.drawImage(img, (cw - w) / 2, (ch - h) / 2, w, h);
}
function playhead() {
  const K = keyFrames(), n = K.length, p = clamp01(home.progress / Math.max(1, n - 1)) * (n - 1), i = Math.min(n - 1, Math.floor(p)), j = Math.min(n - 1, i + 1);
  return {p, f: K[i] + (K[j] - K[i]) * (reduced ? Math.round(p - i) : ease(p - i))};
}
function tintStill() {
  const t = manifest.tint; if (!t || !home.tint) return null;
  const key = String(home.tint); if (!tints.has(key)) { tints.set(key, null); fetchImage(urlOf(t.dir, t.ext || 'webp', key)).then((img) => { if (img) { tints.set(key, img); dirty = true; draw(); } else tints.delete(key); }); }
  return tints.get(key) || null;
}
function draw() {
  if (!ready && !frames.size) return;
  const {p, f} = playhead(), a = Math.floor(f), t = f - a, A = frames.get(a), B = frames.get(Math.min(seq.count - 1, a + 1));
  const tintAt = manifest.tint ? home.keys.indexOf(manifest.tint.key || 'tintIn') : -1, tw = tintAt < 0 ? 0 : clamp01(1 - Math.abs(p - tintAt) * 2);
  const sig = [f.toFixed(2), canvas.width, home.tint, tw.toFixed(2), frames.size, tints.size].join('|');
  if (sig === drawn && !dirty) return; drawn = sig; dirty = false;
  ctx.globalCompositeOperation = 'source-over'; ctx.fillStyle = '#07080b'; ctx.globalAlpha = 1; ctx.fillRect(0, 0, canvas.width, canvas.height);
  const first = A || loadedNear(f); if (first) cover(first, 1);
  if (A && B && t > .02 && B !== A) cover(B, t);
  if (tw > .01) { const s = tintStill(); if (s) cover(s, tw); }
  ctx.globalAlpha = 1;
}
function resize() {
  const next = pick(); if (next !== seq) { seq = next; frames = new Map(); queue = []; ready = false; plan(); }
  const dpr = Math.min(devicePixelRatio || 1, 2), w = Math.max(1, Math.round(stage.clientWidth * dpr)), h = Math.max(1, Math.round(stage.clientHeight * dpr));
  if (canvas.width !== w || canvas.height !== h) { canvas.width = w; canvas.height = h; dirty = true; }
  draw();
}
let raf = 0;
const schedule = () => { if (!raf) raf = requestAnimationFrame(() => { raf = 0; draw(); }); };

stage.style.setProperty('--frames', '1'); story?.classList.add('has-frames');
canvas.style.cssText = 'display:block;width:100%;height:100%';
const observer = new ResizeObserver(resize); observer.observe(stage);
window.addEventListener('dapper:home', schedule); window.addEventListener('dapper:step', schedule);
// Sent by the Next.js home page when the story unmounts on client navigation.
window.addEventListener('dapper:dispose', () => {
  observer.disconnect(); cancelAnimationFrame(raf); raf = 1; queue.length = 0;
  window.removeEventListener('dapper:home', schedule); window.removeEventListener('dapper:step', schedule);
  frames.clear(); tints.clear(); canvas.remove();
}, {once: true});
resize(); plan();

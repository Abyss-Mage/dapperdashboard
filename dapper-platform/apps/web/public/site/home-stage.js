/* Chooses what draws the home story. If the studio has published a photographic frame sequence (assets/frames/manifest.json),
   home-frames.js plays it, scrubbed by scroll. Otherwise the real-time 3D scene (home-scene.js) runs. Both read the same
   window.dapperHome state that story.js writes, so the words, dots and controls never change. */
let manifest = null;
try {
  const res = await fetch(new URL('./assets/frames/manifest.json', import.meta.url), {cache: 'no-cache'});
  if (res.ok && /json/.test(res.headers.get('content-type') || '')) manifest = await res.json();
} catch { /* no sequence published */ }
if (manifest && manifest.version === 1 && (manifest.desktop || manifest.mobile)) await import('./home-frames.js'); else await import('./home-scene.js');

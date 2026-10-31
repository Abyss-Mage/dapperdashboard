'use client';
import { useEffect, useRef } from 'react';

/* ClearCoat: a real photograph rendered through WebGL as if it had a fresh ceramic coat.
   - A studio light-bar reflection drifts across the image and leans toward the pointer.
   - Moving the pointer rains water on it; the drops bead up (hydrophobic), refract the photo underneath, catch the light and
     roll off downward, the way water behaves on coated paint.
   Drops are simulated on the CPU (a few hundred at most) and drawn into a small normal map; one fragment shader composites
   the photo, the refraction and the highlights. Falls back to the plain photo without WebGL or with reduced motion. */

type Drop = { x: number; y: number; r: number; vy: number; life: number };

const VERT = `#version 300 es
in vec2 p; out vec2 uv;
void main(){ uv = p * .5 + .5; gl_Position = vec4(p, 0., 1.); }`;

const FRAG = `#version 300 es
precision highp float;
in vec2 uv; out vec4 o;
uniform sampler2D img, drops;
uniform vec2 res, imgSize, mouse;
uniform float t, cover;
vec2 coverUV(vec2 u){
  float ra = res.x / res.y, ia = imgSize.x / imgSize.y;
  vec2 s = ra > ia ? vec2(1., ia / ra) : vec2(ra / ia, 1.);
  return (u - .5) * s + .5;
}
void main(){
  vec2 u = vec2(uv.x, 1. - uv.y);
  vec4 d = texture(drops, u);                       // rg = surface normal, b = thickness, a = coverage
  vec2 n = (d.rg - .5) * 2.;
  // A bead is a tiny convex lens: it shows the scene around it flipped, so sample against the normal.
  vec2 iu = coverUV(u - n * .022 * d.a);
  vec3 col = texture(img, iu).rgb;
  // a thin contact shadow at the rim, a slightly brighter lens centre
  float rim = smoothstep(.55, .95, length(n));
  col *= mix(1., 1.06 - rim * .45, d.a);
  // the studio light bar: a soft diagonal band sliding across, pulled toward the pointer
  float k = (u.x * .85 + u.y * .35) - (fract(t * .035) * 2.2 - .6) - (mouse.x - .5) * .25;
  float band = exp(-k * k * 260.) * .28 + exp(-k * k * 18.) * .05;
  col += band * vec3(1., .98, .95) * (1. - u.y * .4);
  // each bead catches the light bar as a small sharp highlight
  vec3 L = normalize(vec3(-.4, -.6, .7)), N = normalize(vec3(n, .9));
  float spec = pow(max(dot(reflect(-L, N), vec3(0., 0., 1.)), 0.), 40.);
  col += spec * d.a * 1.2 + d.a * band * 1.5;
  o = vec4(col, 1.);
}`;

export default function ClearCoat({ src, className }: { src: string; className?: string }) {
  const canvas = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const el = canvas.current!;
    const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const gl = el.getContext('webgl2', { premultipliedAlpha: false, antialias: false });
    if (!gl || reduced) { el.dataset.fallback = '1'; return; }

    const shader = (type: number, code: string) => { const s = gl.createShader(type)!; gl.shaderSource(s, code); gl.compileShader(s); return s; };
    const prog = gl.createProgram()!;
    gl.attachShader(prog, shader(gl.VERTEX_SHADER, VERT));
    gl.attachShader(prog, shader(gl.FRAGMENT_SHADER, FRAG));
    gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) { el.dataset.fallback = '1'; return; }
    gl.useProgram(prog);
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    const loc = gl.getAttribLocation(prog, 'p');
    gl.enableVertexAttribArray(loc); gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
    const U = (n: string) => gl.getUniformLocation(prog, n);

    const tex = (unit: number) => { const t = gl.createTexture()!; gl.activeTexture(gl.TEXTURE0 + unit); gl.bindTexture(gl.TEXTURE_2D, t); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE); return t; };
    const imgTex = tex(0), dropTex = tex(1);
    gl.uniform1i(U('img'), 0); gl.uniform1i(U('drops'), 1);

    // The bead normal map: drawn on a small 2D canvas each frame and uploaded as a texture.
    const map = document.createElement('canvas'), mctx = map.getContext('2d')!;
    const sprite = document.createElement('canvas');
    sprite.width = sprite.height = 64;
    { // one bead: normal encoded in r/g, coverage in alpha (a domed lens)
      const s = sprite.getContext('2d')!, id = s.createImageData(64, 64);
      for (let y = 0; y < 64; y++) for (let x = 0; x < 64; x++) {
        const dx = (x - 31.5) / 31.5, dy = (y - 31.5) / 31.5, r2 = dx * dx + dy * dy, i = (y * 64 + x) * 4;
        if (r2 > 1) continue;
        id.data[i] = 128 + dx * 127; id.data[i + 1] = 128 + dy * 127; id.data[i + 2] = 255 * (1 - r2); id.data[i + 3] = 255 * Math.min(1, (1 - r2) * 6);
      }
      s.putImageData(id, 0, 0);
    }

    let W = 1, H = 1, iw = 1, ih = 1, ready = false, raf = 0, last = performance.now(), visible = true;
    const drops: Drop[] = [];
    const mouse = { x: .5, y: .5, mx: .5, active: false };

    const resize = () => {
      const dpr = Math.min(devicePixelRatio || 1, 1.75), r = el.getBoundingClientRect();
      W = Math.max(1, Math.round(r.width * dpr)); H = Math.max(1, Math.round(r.height * dpr));
      el.width = W; el.height = H; gl.viewport(0, 0, W, H);
      map.width = Math.max(1, Math.round(r.width / 3)); map.height = Math.max(1, Math.round(r.height / 3));
    };
    const img = new Image();
    img.decoding = 'async';
    img.onload = () => {
      iw = img.naturalWidth; ih = img.naturalHeight;
      gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, imgTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
      ready = true; el.dataset.ready = '1';
    };
    img.src = src;

    const spawn = (x: number, y: number, n: number) => {
      for (let i = 0; i < n && drops.length < 320; i++) {
        drops.push({ x: x + (Math.random() - .5) * .08, y: y + (Math.random() - .5) * .08, r: .004 + Math.random() * Math.random() * .014, vy: 0, life: 1 });
      }
    };
    const onMove = (e: PointerEvent) => {
      const r = el.getBoundingClientRect();
      const x = (e.clientX - r.left) / r.width, y = (e.clientY - r.top) / r.height;
      if (x < 0 || x > 1 || y < 0 || y > 1) return;
      const speed = Math.hypot(x - mouse.x, y - mouse.y);
      mouse.x = x; mouse.y = y; mouse.active = true;
      spawn(x, y, Math.min(6, 1 + Math.floor(speed * 120)));
    };
    addEventListener('pointermove', onMove, { passive: true });

    // a light idle shower so the surface is alive before the visitor moves
    let idle = 0;
    const frame = (now: number) => {
      raf = 0;
      if (!visible) return;
      const dt = Math.min(.05, (now - last) / 1000); last = now;
      idle += dt;
      if (!mouse.active && idle > .12) { idle = 0; spawn(.45 + Math.random() * .5, .2 + Math.random() * .5, 1); }
      // physics: beads stick until they are heavy enough, then roll down and merge-shrink
      for (let i = drops.length - 1; i >= 0; i--) {
        const d = drops[i];
        if (d.r > .011) { d.vy += dt * (d.r * 22); d.y += d.vy * dt; d.r -= dt * .003; }
        d.life -= dt * (d.r > .011 ? .08 : .09);
        if (d.y > 1.08 || d.life <= 0 || d.r <= .002) drops.splice(i, 1);
      }
      for (let i = 0; i < drops.length; i++) for (let j = i + 1; j < drops.length; j++) {
        const a = drops[i], b = drops[j], dx = (a.x - b.x) * (W / H), dy = a.y - b.y;
        if (dx * dx + dy * dy < (a.r + b.r) * (a.r + b.r) * .5) { a.r = Math.min(.018, Math.sqrt(a.r * a.r + b.r * b.r)); a.life = Math.max(a.life, b.life); b.life = 0; }
      }
      mctx.clearRect(0, 0, map.width, map.height);
      for (const d of drops) {
        const s = d.r * map.height * 2, x = d.x * map.width - s / 2, y = d.y * map.height - s / 2;
        mctx.globalAlpha = Math.min(1, d.life * 2);
        mctx.drawImage(sprite, x, y, s, s * (d.vy > 0 ? 1.15 : 1));
      }
      mctx.globalAlpha = 1;
      gl.activeTexture(gl.TEXTURE1); gl.bindTexture(gl.TEXTURE_2D, dropTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, map);
      mouse.mx += (mouse.x - mouse.mx) * Math.min(1, dt * 3);
      if (ready) {
        gl.uniform2f(U('res'), W, H); gl.uniform2f(U('imgSize'), iw, ih);
        gl.uniform2f(U('mouse'), mouse.mx, mouse.y); gl.uniform1f(U('t'), now / 1000);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      }
      raf = requestAnimationFrame(frame);
    };
    const io = new IntersectionObserver(([e]) => { visible = e.isIntersecting; if (visible && !raf) { last = performance.now(); raf = requestAnimationFrame(frame); } });
    io.observe(el);
    const ro = new ResizeObserver(resize); ro.observe(el);
    resize();
    raf = requestAnimationFrame(frame);
    // Free what this run created, but keep the context itself: React may mount the same canvas again straight away.
    return () => {
      cancelAnimationFrame(raf); io.disconnect(); ro.disconnect(); removeEventListener('pointermove', onMove);
      gl.deleteTexture(imgTex); gl.deleteTexture(dropTex); gl.deleteBuffer(buf); gl.deleteProgram(prog);
      delete el.dataset.ready;
    };
  }, [src]);

  return <canvas ref={canvas} className={className} aria-hidden="true" />;
}

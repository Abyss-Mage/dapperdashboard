/* Home page 3D story: a road-worn Mercedes GLS arrives on a coastal mountain road, drives into the Dapper garage and gets the work done
   in front of the visitor, step by step as they scroll:
     detailing: snow foam, then a pressure rinse that sweeps the foam and the road dirt off,
     window tint: the windows lift out, the visitor picks a shade, the windows go back in with that shade,
     ceramic coating: an applicator pass that leaves deep gloss, then the water droplet test,
     paint protection film: clear film comes down onto the body and the trim lines light up.
   story.js owns the scroll (GSAP ScrollTrigger + Lenis) and writes window.dapperHome.progress (0 .. steps-1), .side and .tint;
   this file reads them every frame and blends between the shots below. Real-time three.js; no pre-rendered images. */
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {HDRLoader} from 'three/addons/loaders/HDRLoader.js';
import {toCreasedNormals} from 'three/addons/utils/BufferGeometryUtils.js';
import {MeshoptDecoder} from './assets/three/meshopt_decoder.module.js';

const stage = document.getElementById('home-stage');
const story = document.getElementById('story');
const curtain = document.getElementById('home-curtain');
const home = (window.dapperHome ||= {progress: 0, view: null, tint: null, keys: [], side: 1});
const small = () => stage.clientWidth < 760;
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const mix = THREE.MathUtils.lerp, clamp01 = (v) => Math.min(1, Math.max(0, v)), ease = (t) => t * t * (3 - 2 * t);

/* Shots. cam/look are in the car's own frame (nose along +z). env: where the car is. The numbers drive the shader and the effects. */
// rinse defaults to 1 (washed) so the dirt never comes back once the car is clean; the steps before the rinse set it to 0
const BASE = {drive: 0, dirt: 0, foam: 0, rinse: 1, spray: 0, cannon: 0, glassOut: 0, vlt: 85, coat: 0, pad: 0, beads: 0, pour: 0, film: 0, door: 0};
const KEY = {
  road: {env: 'road', cam: [-4.9, 1.8, 6.4], look: [0, .8, .4], dirt: 1, rinse: 0},
  garage: {env: 'garage', cam: [-4.6, 1.4, 9.6], look: [.4, 2.1, -1.6], dirt: 1, rinse: 0},
  foam: {env: 'garage', cam: [-4.6, 1.7, 3.8], look: [0, .9, .3], dirt: 1, rinse: 0, foam: 1, cannon: 1},
  // the rinse and coat steps rest part-way through, so the tool is on the car and both halves are visible
  rinse: {env: 'garage', cam: [-6.4, 1.7, 1.2], look: [0, .8, -.2], dirt: 1, foam: 1, rinse: .55, spray: 1},
  tintOut: {env: 'garage', cam: [-5.8, 2.3, 3.6], look: [0, 1.25, -.2], glassOut: 1, vlt: 'pick'},
  tintIn: {env: 'garage', cam: [-4.9, 1.6, 2.4], look: [0, 1.15, 0], vlt: 'pick'},
  coat: {env: 'garage', cam: [-2.6, 2.6, 4.8], look: [0, 1.0, 1.4], vlt: 'pick', coat: .2, pad: 1},
  beads: {env: 'garage', cam: [-1.8, 2.1, 4.2], look: [0, 1.0, 1.6], vlt: 'pick', coat: 1, beads: 1, pour: 1},
  film: {env: 'garage', cam: [-6.2, 3.2, 5.2], look: [0, .9, 0], vlt: 'pick', coat: 1, film: .55},
  done: {env: 'garage', cam: [3.4, 1.5, 8.4], look: [-.2, 1.5, -4], vlt: 'pick', coat: 1, film: 1, door: 1},
};
const ROAD_X = 2.4, ROAD_FROM = 3, ROAD_TO = -2, ENTRY = 15, WHEEL_R = .39;
const GARAGE = new THREE.Vector3(200, 0, 0); // the garage set sits away from the road scene; the cut between them is hidden by a fade
const UP = new THREE.Vector3(0, 1, 0);

let renderer, scene, camera, car, body, wheels = [], glassPieces = [], glassMat, paintMat, filmPieces = [];
const particleMats = [];
let roadSet, garageSet, roadEnv, garageEnv, sun, garageKey, spray, cannon, cannonGun, pad, padHeight, beadsMesh, pourMesh, door;
const U = {foamZ: {value: -9}, out: {value: 0}, dirt: {value: 1}, foam: {value: 0}, rinse: {value: 0}, coat: {value: 0}, film: {value: 0}, time: {value: 0}};
const GLASS_DARK = new THREE.Color('#0a0c0f'), GLASS_LIGHT = new THREE.Color('#a9bfcc');

/* ---------- paint: one shader with road dirt, snow foam, a rinse front, and the ceramic gloss front ---------- */
function paintShader(mat) {
  mat.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, {uDirt: U.dirt, uFoam: U.foam, uFoamZ: U.foamZ, uRinse: U.rinse, uCoat: U.coat, uTime: U.time});
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position; transformed+=normal*.003;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', `#include <common>
      varying vec3 vL; uniform float uDirt,uFoam,uFoamZ,uRinse,uCoat,uTime;
      float h3(vec3 p){return fract(sin(dot(p,vec3(127.1,311.7,74.7)))*43758.5453);}
      float n3(vec3 p){vec3 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(mix(h3(i),h3(i+vec3(1,0,0)),f.x),mix(h3(i+vec3(0,1,0)),h3(i+vec3(1,1,0)),f.x),f.y),mix(mix(h3(i+vec3(0,0,1)),h3(i+vec3(1,0,1)),f.x),mix(h3(i+vec3(0,1,1)),h3(i+vec3(1,1,1)),f.x),f.y),f.z);}
      float washed(){ return smoothstep(-.25,.25,vL.z-mix(3.0,-3.0,uRinse)); }
      float coated(){ return smoothstep(-.2,.2,vL.z-mix(3.0,-3.0,uCoat)); }
      float dirtM=0.,foamM=0.;`)
      .replace('#include <color_fragment>', `#include <color_fragment>
      float w=washed();
      float g=n3(vL*9.)*.3+n3(vL*37.)*.35+n3(vL*110.)*.35;
      // road film: mud thickest low down and behind the wheels, dust over the rest, rain streaks, splatter flecks
      float low=1.0-smoothstep(.15,1.05,vL.y);
      float grime=smoothstep(.25,.85,clamp(low*1.1+g*.35-.1,0.,1.));
      float streak=smoothstep(.7,.92,n3(vec3(vL.x*80.,vL.y*1.1,vL.z*80.)))*(1.-smoothstep(.9,1.2,vL.y));
      float fleck=step(.86,n3(vL*170.))*(1.-smoothstep(.3,1.15,vL.y));
      float dust=.34+.1*g;
      dirtM=clamp(max(max(grime*.9,dust*(1.-.3*streak)),fleck*.7),0.,.88)*uDirt*(1.-w);
      vec3 mud=mix(vec3(.31,.28,.24),vec3(.15,.11,.08),low)*(.8+.4*g);
      diffuseColor.rgb=mix(diffuseColor.rgb,mud,dirtM*.9);
      float bub=n3(vL*60.+vec3(0.,uTime*.15,0.))*.6+n3(vL*14.)*.4;
      float drip=smoothstep(.35,.75,n3(vec3(vL.x*18.,vL.y*2.-uTime*.08,vL.z*18.)));
      foamM=uFoam*(1.-w)*smoothstep(.05,.42,bub+drip*.35+(vL.y-.4)*.25)*(1.-smoothstep(uFoamZ-.5,uFoamZ+.05,vL.z));
      diffuseColor.rgb=mix(diffuseColor.rgb,vec3(.96,.97,.98)*(.86+bub*.14),foamM);`)
      .replace('#include <roughnessmap_fragment>', `#include <roughnessmap_fragment>
      float c=coated();
      roughnessFactor=mix(mix(roughnessFactor,.5,(1.-c)*uCoatOn),.92,max(dirtM,foamM));`)
      .replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
      // a faint wet sheen where the coating is being levelled
      totalEmissiveRadiance+=vec3(.85,.93,1.)*(1.-smoothstep(0.,.07,abs(vL.z-mix(3.0,-3.0,uCoat))))*step(.002,uCoat)*step(uCoat,.995)*.14;`)
      .replace('#include <metalnessmap_fragment>', '#include <metalnessmap_fragment>\nmetalnessFactor=mix(metalnessFactor,.05,max(dirtM,foamM)*.85);')
      .replace('#include <lights_fragment_begin>', `material.clearcoat*=mix(1.-.8*uCoatOn,1.,coated())*(1.-max(dirtM,foamM));\n#include <lights_fragment_begin>`);
    // before the ceramic step the paint is plain; once it starts, the uncoated part reads hazy next to the coated gloss
    sh.fragmentShader = sh.fragmentShader.replace('float dirtM=0.,foamM=0.;', 'float dirtM=0.,foamM=0.,uCoatOn=0.;')
      .replace('float w=washed();', 'float w=washed(); uCoatOn=step(.002,uCoat);');
  };
  mat.customProgramCacheKey = () => 'dapper-paint-v9';
  return mat;
}
/* clear film revealed from the roof down, with a bright squeegee line at the edge */
function filmShader(mat) {
  mat.onBeforeCompile = (sh) => {
    sh.uniforms.uFilm = U.film;
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', '#include <common>\nvarying vec3 vL; uniform float uFilm;')
      .replace('#include <clipping_planes_fragment>', '#include <clipping_planes_fragment>\nfloat lvl=mix(2.1,-.1,uFilm); if(vL.y<lvl) discard; float band=(1.-smoothstep(0.,.045,vL.y-lvl))*step(uFilm,.995);')
      .replace('#include <dithering_fragment>', '#include <dithering_fragment>\ngl_FragColor.rgb+=vec3(.9,.95,1.)*band*.55; gl_FragColor.a=max(gl_FragColor.a,band*.5);');
  };
  mat.customProgramCacheKey = () => 'dapper-film-v1';
  return mat;
}

/* ---------- road scene: a real asphalt road above the sea, lit and backed by a CC0 coastal HDR ---------- */
function asphaltCanvas() {
  const [c, x] = canvas(1024, 1024);
  x.fillStyle = '#2b2c2f'; x.fillRect(0, 0, 1024, 1024);
  for (let n = 0; n < 160; n++) { const g = 30 + Math.random() * 22; x.fillStyle = `rgba(${g},${g},${g + 2},.09)`; x.beginPath(); x.arc(Math.random() * 1024, Math.random() * 1024, 30 + Math.random() * 110, 0, 6.3); x.fill(); }
  for (let n = 0; n < 34000; n++) { const g = 38 + Math.random() * 96 | 0; x.fillStyle = `rgba(${g},${g},${g + 3},${.2 + Math.random() * .45})`; x.fillRect(Math.random() * 1024, Math.random() * 1024, 1 + Math.random() * 2.2, 1 + Math.random() * 2.2); }
  for (let n = 0; n < 14; n++) {
    x.strokeStyle = `rgba(6,6,7,${.35 + Math.random() * .3})`; x.lineWidth = 1 + Math.random() * 1.8; x.beginPath();
    let px = Math.random() * 1024, py = Math.random() * 1024; x.moveTo(px, py);
    for (let k = 0; k < 14; k++) { px += (Math.random() - .5) * 60; py += 10 + Math.random() * 40; x.lineTo(px, py); }
    x.stroke();
  }
  return c;
}
function gravelCanvas() {
  const [c, x] = canvas(512, 512);
  x.fillStyle = '#4a463e'; x.fillRect(0, 0, 512, 512);
  for (let n = 0; n < 90; n++) { const g = 80 + Math.random() * 40 | 0; x.fillStyle = `rgba(${g},${g - 6},${g - 16},.14)`; x.beginPath(); x.arc(Math.random() * 512, Math.random() * 512, 14 + Math.random() * 50, 0, 6.3); x.fill(); }
  for (let n = 0; n < 16000; n++) { const g = 45 + Math.random() * 80 | 0; x.fillStyle = `rgba(${g},${g - 3},${g - 10},${.35 + Math.random() * .5})`; x.fillRect(Math.random() * 512, Math.random() * 512, 1 + Math.random() * 3, 1 + Math.random() * 3); }
  return c;
}
function wornCanvas() {
  const [c, x] = canvas(256, 256);
  x.fillStyle = '#fff'; x.fillRect(0, 0, 256, 256);
  for (let n = 0; n < 900; n++) { const g = Math.random() * 130 | 0; x.fillStyle = `rgba(${g},${g},${g},${.5 + Math.random() * .5})`; x.fillRect(Math.random() * 256, Math.random() * 256, 1 + Math.random() * 5, 1 + Math.random() * 5); }
  const t = new THREE.CanvasTexture(c); t.wrapS = t.wrapT = THREE.RepeatWrapping; return t;
}
/* The linear colour of the HDR at its horizon, so distant ground fades into the same haze the sky has. */
function horizonColor(tex) {
  const out = new THREE.Color('#9fb0bd'), img = tex.image;
  if (!img?.data) return out;
  const {data, width, height} = img, half = data instanceof Uint16Array, f = (v) => (half ? THREE.DataUtils.fromHalfFloat(v) : v);
  let r = 0, g = 0, b = 0, n = 0;
  // the road shots look toward -x, which is the seam of the equirect map: u = atan2(z, x) / 2pi + .5
  for (const row of [Math.floor(height * .5) - 6, Math.floor(height * .5) + 6]) for (let d = -40; d <= 40; d++) {
    const col = (Math.floor(width * (1 + d / 360)) % width + width) % width, i = (row * width + col) * 4; r += f(data[i]); g += f(data[i + 1]); b += f(data[i + 2]); n++;
  }
  return out.setRGB(Math.min(1.6, r / n), Math.min(1.6, g / n), Math.min(1.6, b / n));
}
let roadFog, garageFog, haze;
async function buildRoad() {
  const tex = await new HDRLoader().loadAsync(new URL('./assets/env/red_hill_curve_2k.hdr', import.meta.url).href);
  tex.mapping = THREE.EquirectangularReflectionMapping;
  roadEnv = tex; roadFog = new THREE.Fog(new THREE.Color('#6f8498').lerp(horizonColor(tex), .15), 14, 190); garageFog = new THREE.Fog(roadFog.color, 30, 170);
  roadSet = new THREE.Group(); scene.add(roadSet);
  const X0 = -3.4, X1 = 4.4, RW = X1 - X0, RC = (X0 + X1) / 2, LEN = 440;
  const flat = (w, h, mat, x, y, z) => { const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat); m.rotation.x = -Math.PI / 2; m.position.set(x, y, z); m.receiveShadow = true; roadSet.add(m); return m; };
  // gravel shoulders and rough ground out to the haze
  const gravelTex = repeatTex(gravelCanvas(), 300, 300);
  flat(900, 900, new THREE.MeshStandardMaterial({map: gravelTex, roughness: 1, bumpMap: gravelTex, bumpScale: 1.4}), RC, -.012, 0);
  // a band of haze round the horizon, so the flat ground melts into the base of the mountains instead of ending in a hard line
  const [hc, hg] = canvas(4, 256), hgrad = hg.createLinearGradient(0, 0, 0, 256);
  hgrad.addColorStop(0, 'rgba(255,255,255,0)'); hgrad.addColorStop(.35, 'rgba(255,255,255,.18)'); hgrad.addColorStop(.62, 'rgba(255,255,255,.7)'); hgrad.addColorStop(.8, 'rgba(255,255,255,1)'); hgrad.addColorStop(1, 'rgba(255,255,255,1)');
  hg.fillStyle = hgrad; hg.fillRect(0, 0, 4, 256);
  haze = new THREE.Mesh(new THREE.CylinderGeometry(430, 430, 90, 48, 1, true), new THREE.MeshBasicMaterial({map: new THREE.CanvasTexture(hc), color: roadFog.color, transparent: true, side: THREE.BackSide, depthWrite: false, fog: false}));
  haze.position.y = 32; haze.renderOrder = -1; roadSet.add(haze);
  // the road itself
  const asphaltTex = repeatTex(asphaltCanvas(), RW / 4, LEN / 4);
  flat(RW, LEN, new THREE.MeshStandardMaterial({map: asphaltTex, roughness: .88, metalness: 0, bumpMap: asphaltTex, bumpScale: 1.1}), RC, .004, 0);
  // two darker wheel paths down the lane the car drives in
  const wear = new THREE.CanvasTexture((() => { const [c, x] = canvas(64, 64); const g = x.createLinearGradient(0, 0, 64, 0); g.addColorStop(0, 'rgba(0,0,0,0)'); g.addColorStop(.5, 'rgba(0,0,0,.4)'); g.addColorStop(1, 'rgba(0,0,0,0)'); x.fillStyle = g; x.fillRect(0, 0, 64, 64); return c; })());
  for (const dx of [-.8, .8]) { const m = flat(.85, LEN, new THREE.MeshBasicMaterial({map: wear, transparent: true, depthWrite: false, opacity: .6}), 2.4 + dx, .007, 0); m.receiveShadow = false; }
  // lane paint, worn away in patches
  const worn = wornCanvas(), line = new THREE.MeshStandardMaterial({color: '#d8d8cf', roughness: .75, alphaMap: worn, alphaTest: .4, side: THREE.DoubleSide});
  worn.repeat.set(1, 120);
  for (const x of [X0 + .28, X1 - .28]) flat(.15, LEN, line, x, .009, 0);
  const dashes = new THREE.InstancedMesh(new THREE.PlaneGeometry(.13, 3), new THREE.MeshStandardMaterial({color: '#e2c04a', roughness: .75, alphaMap: wornCanvas(), alphaTest: .4}), 50), m4 = new THREE.Matrix4();
  for (let i = 0; i < 50; i++) { m4.compose(new THREE.Vector3(RC, .0095, -220 + i * 9), new THREE.Quaternion().setFromEuler(new THREE.Euler(-Math.PI / 2, 0, 0)), new THREE.Vector3(1, 1, 1)); dashes.setMatrixAt(i, m4); }
  dashes.receiveShadow = true; roadSet.add(dashes);
  // a galvanised crash barrier on the sea side, behind the car
  const steel = new THREE.MeshStandardMaterial({color: '#b6bcc1', metalness: .75, roughness: .42});
  const beam = new THREE.Mesh(new THREE.BoxGeometry(.05, .3, LEN), steel); beam.position.set(X0 - .9, .62, 0); beam.castShadow = true; roadSet.add(beam);
  const rib = new THREE.Mesh(new THREE.BoxGeometry(.07, .06, LEN), steel); rib.position.set(X0 - .9 + .01, .62, 0); roadSet.add(rib);
  const posts = new THREE.InstancedMesh(new THREE.BoxGeometry(.1, .78, .09), new THREE.MeshStandardMaterial({color: '#8d9298', metalness: .6, roughness: .5}), 110);
  for (let i = 0; i < 110; i++) { m4.makeTranslation(X0 - 1.0, .39, -200 + i * 4); posts.setMatrixAt(i, m4); }
  roadSet.add(posts);
}

/* ---------- the Dapper garage: modelled on the real shop from the owner's photos ----------
   black painted block walls, the hexagon LED clusters on the ceiling and the slat wall, warm vertical tubes and white
   fluorescents, ribbed interlocking floor tiles with three red bays, the plywood loft with its black sheeting, shelves of
   chemical jugs and spray bottles, the folding workbench, shop vacs, a generator, and the cream roller door with its three
   small windows. The car drives in through that door. Everything is built here (no images of the shop are used). */
const HALF_X = 7, HALF_Z = 11, HEIGHT = 5.2, DOOR_W = 5.2, DOOR_H = 4.2;

/* A box with softened edges: real objects are never razor-sharp cubes. */
function roundedBox(w, h, d, r = .02, seg = 3) {
  const b = Math.max(.002, Math.min(r, w / 2 - .001, h / 2 - .001, d / 2 - .001)), iw = w - 2 * b, ih = h - 2 * b, ir = Math.max(.001, Math.min(b, iw / 2, ih / 2) * .6);
  const sh = new THREE.Shape();
  sh.moveTo(-iw / 2 + ir, -ih / 2); sh.lineTo(iw / 2 - ir, -ih / 2); sh.absarc(iw / 2 - ir, -ih / 2 + ir, ir, -Math.PI / 2, 0);
  sh.lineTo(iw / 2, ih / 2 - ir); sh.absarc(iw / 2 - ir, ih / 2 - ir, ir, 0, Math.PI / 2); sh.lineTo(-iw / 2 + ir, ih / 2); sh.absarc(-iw / 2 + ir, ih / 2 - ir, ir, Math.PI / 2, Math.PI);
  sh.lineTo(-iw / 2, -ih / 2 + ir); sh.absarc(-iw / 2 + ir, -ih / 2 + ir, ir, Math.PI, Math.PI * 1.5);
  const g = new THREE.ExtrudeGeometry(sh, {depth: Math.max(.001, d - 2 * b), bevelEnabled: true, bevelThickness: b, bevelSize: b, bevelSegments: seg, curveSegments: 5});
  g.translate(0, 0, -(d - 2 * b) / 2); g.computeVertexNormals(); return g;
}
const repeatTex = (c, rx = 1, ry = 1) => {
  const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.repeat.set(rx, ry); t.anisotropy = 8; return t;
};
const canvas = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; return [c, c.getContext('2d')]; };

/* Painted cinder block: 40 x 20 cm blocks, running bond. One tile of this texture is 3.2 m x 1.6 m. */
function blockCanvas() {
  const [c, x] = canvas(1024, 512);
  x.fillStyle = '#2d2e32'; x.fillRect(0, 0, 1024, 512);
  for (let j = 0; j < 8; j++) for (let i = -1; i < 9; i++) {
    const shade = 13 + Math.random() * 11; x.fillStyle = `rgb(${shade},${shade + 1},${shade + 3})`; x.fillRect(i * 128 + (j % 2) * 64 + 3, j * 64 + 3, 122, 58);
  }
  for (let n = 0; n < 6000; n++) { const g = 28 + Math.random() * 44; x.fillStyle = `rgba(${g},${g},${g + 4},.2)`; x.fillRect(Math.random() * 1024, Math.random() * 512, 2, 2); }
  return c;
}
/* Ribbed interlocking floor tiles, 40 cm, alternating rib direction. One tile of this texture is 3.2 m. */
function tileCanvas(base, alt, rib, grime) {
  const [c, x] = canvas(1024, 1024);
  for (let j = 0; j < 8; j++) for (let i = 0; i < 8; i++) {
    const even = (i + j) % 2 === 0; x.fillStyle = even ? base : alt; x.fillRect(i * 128, j * 128, 128, 128); x.fillStyle = rib;
    for (let k = 0; k < 16; k++) { if (even) x.fillRect(i * 128, j * 128 + k * 8 + 5, 128, 2); else x.fillRect(i * 128 + k * 8 + 5, j * 128, 2, 128); }
    x.strokeStyle = 'rgba(0,0,0,.55)'; x.lineWidth = 2; x.strokeRect(i * 128 + 1, j * 128 + 1, 126, 126);
  }
  if (grime) for (let n = 0; n < 90; n++) { x.fillStyle = `rgba(0,0,0,${.03 + Math.random() * .06})`; x.beginPath(); x.arc(Math.random() * 1024, Math.random() * 1024, 10 + Math.random() * 60, 0, 6.3); x.fill(); }
  return c;
}
function slatCanvas() {
  const [c, x] = canvas(256, 256);
  x.fillStyle = '#0a0a0c'; x.fillRect(0, 0, 256, 256);
  for (let k = 0; k < 8; k++) { x.fillStyle = '#1d1e22'; x.fillRect(0, k * 32, 256, 4); x.fillStyle = '#34353a'; x.fillRect(0, k * 32 + 4, 256, 1); }
  return c;
}
function doorCanvas() {
  const [c, x] = canvas(1040, 840);
  x.fillStyle = '#d7cdb5'; x.fillRect(0, 0, 1040, 840);
  for (let k = 0; k < 4; k++) {
    const y = k * 210; const g = x.createLinearGradient(0, y, 0, y + 210); g.addColorStop(0, 'rgba(255,255,255,.18)'); g.addColorStop(1, 'rgba(0,0,0,.1)'); x.fillStyle = g; x.fillRect(0, y, 1040, 210);
    x.fillStyle = 'rgba(60,50,35,.55)'; x.fillRect(0, y, 1040, 5);
    for (const hx of [60, 520, 980]) { x.fillStyle = 'rgba(40,36,30,.8)'; x.fillRect(hx, y - 10, 22, 24); }
  }
  for (const cx of [260, 520, 780]) {
    x.fillStyle = '#0d0d0f'; x.beginPath(); x.roundRect(cx - 95, 408, 190, 86, 8); x.fill();
    const g = x.createLinearGradient(0, 418, 0, 484); g.addColorStop(0, '#a9c4dc'); g.addColorStop(.5, '#58738b'); g.addColorStop(1, '#8aa2b4'); x.fillStyle = g; x.fillRect(cx - 82, 420, 164, 62);
  }
  for (let n = 0; n < 260; n++) { x.fillStyle = `rgba(70,55,35,${Math.random() * .06})`; x.fillRect(Math.random() * 1040, 560 + Math.random() * 280, 60 + Math.random() * 120, 3 + Math.random() * 10); }
  return c;
}

/* Flat-top hexagon clusters like the LED rigs: cells are axial (q, s); shared edges are drawn once. */
function hexSegments(cells, r) {
  const segs = [], seen = new Set();
  for (const [q, s] of cells) {
    const cx = r * 1.5 * q, cz = r * Math.sqrt(3) * (s + q / 2);
    for (let k = 0; k < 6; k++) {
      const a0 = k * Math.PI / 3, a1 = (k + 1) * Math.PI / 3;
      const p0 = [cx + r * Math.cos(a0), cz + r * Math.sin(a0)], p1 = [cx + r * Math.cos(a1), cz + r * Math.sin(a1)];
      const key = [p0, p1].map((p) => p.map((v) => Math.round(v * 100)).join(',')).sort().join('|');
      if (!seen.has(key)) { seen.add(key); segs.push([p0, p1]); }
    }
  }
  return segs;
}
const tubeGeo = new THREE.CylinderGeometry(.034, .034, 1, 10).rotateZ(Math.PI / 2);
function tubeBetween(group, mat, A, B) {
  const m = new THREE.Mesh(tubeGeo, mat), d = B.clone().sub(A);
  m.position.copy(A).add(B).multiplyScalar(.5); m.scale.x = d.length(); m.quaternion.setFromUnitVectors(new THREE.Vector3(1, 0, 0), d.normalize()); group.add(m);
}
function hexRig(group, mat, cells, r, place) { for (const [a, b] of hexSegments(cells, r)) tubeBetween(group, mat, place(a[0], a[1]), place(b[0], b[1])); }
const fixture = (group, matTube, x, y, z, ry, len = 1.25) => {
  const g = new THREE.Group(), body = new THREE.Mesh(roundedBox(len, .08, .26, .03), new THREE.MeshStandardMaterial({color: '#b9bcc0', roughness: .55, metalness: .3}));
  g.add(body); for (const dz of [-.07, .07]) { const t = new THREE.Mesh(new THREE.CylinderGeometry(.035, .035, len - .08, 8), matTube); t.rotation.z = Math.PI / 2; t.position.set(0, -.07, dz); g.add(t); }
  g.position.set(x, y, z); g.rotation.y = ry; group.add(g);
};

function facadeCanvas(wall) {
  const [c, g] = canvas(256, 256); g.fillStyle = wall; g.fillRect(0, 0, 256, 256);
  for (let n = 0; n < 2400; n++) { g.fillStyle = `rgba(0,0,0,${Math.random() * .05})`; g.fillRect(Math.random() * 256, Math.random() * 256, 3, 3); }
  for (let r = 0; r < 4; r++) for (let q = 0; q < 4; q++) {
    const x = 14 + q * 62, y = 18 + r * 62, lit = Math.random() < .08, gr = g.createLinearGradient(0, y, 0, y + 40);
    gr.addColorStop(0, lit ? '#d8c8a0' : '#7c93a6'); gr.addColorStop(1, lit ? '#b89f6c' : '#3b4a58'); g.fillStyle = 'rgba(30,32,36,.9)'; g.fillRect(x - 2, y - 2, 46, 44); g.fillStyle = gr; g.fillRect(x, y, 42, 40);
    g.fillStyle = 'rgba(255,255,255,.12)'; g.fillRect(x, y, 42, 5);
  }
  const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 8; return t;
}
function outsideScenery(parent) {
  const flat = (w, h, mat, x, y, z) => { const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat); m.rotation.x = -Math.PI / 2; m.position.set(x, y, z); m.receiveShadow = true; parent.add(m); return m; };
  const gravel = repeatTex(gravelCanvas(), 260, 260), asphalt = repeatTex(asphaltCanvas(), 5, 80);
  flat(900, 900, new THREE.MeshStandardMaterial({map: gravel, color: '#8e9a74', roughness: 1}), 0, -.03, -HALF_Z - 200);
  flat(9, 12, new THREE.MeshStandardMaterial({color: '#8a8d90', roughness: .92}), 0, -.005, -HALF_Z - 6);
  flat(20, 320, new THREE.MeshStandardMaterial({map: asphalt, roughness: .9}), 0, -.012, -HALF_Z - 172);
  const kerb = new THREE.MeshStandardMaterial({color: '#a7a9a6', roughness: .9});
  for (const x of [-10.3, 10.3]) { const k = new THREE.Mesh(new THREE.BoxGeometry(.3, .16, 320), kerb); k.position.set(x, .06, -HALF_Z - 172); parent.add(k); }
  const dash = new THREE.MeshStandardMaterial({color: '#dcdccf', roughness: .8});
  for (let i = 0; i < 30; i++) { const d = new THREE.Mesh(new THREE.PlaneGeometry(.14, 3), dash); d.rotation.x = -Math.PI / 2; d.position.set(0, -.008, -HALF_Z - 12 - i * 9); parent.add(d); }
  // buildings along the far side of the street
  const walls = ['#b7b2a6', '#9aa0a5', '#a89b86', '#838e97', '#b0a495'];
  for (let i = 0; i < 9; i++) {
    const w = 22 + (i * 7 % 5) * 4, h = 10 + (i * 5 % 4) * 2.6, x = -130 + i * 30, tex = facadeCanvas(walls[i % walls.length]); tex.wrapS = tex.wrapT = THREE.RepeatWrapping; tex.repeat.set(w / 8, h / 8);
    const mat = new THREE.MeshStandardMaterial({map: tex, roughness: .85, emissive: '#202327', emissiveMap: tex, emissiveIntensity: .35});
    const bld = new THREE.Mesh(new THREE.BoxGeometry(w, h, 16), mat); bld.position.set(x, h / 2, -HALF_Z - 95 - (i % 3) * 9); parent.add(bld);
    const roof = new THREE.Mesh(new THREE.BoxGeometry(w + .6, .5, 16.6), new THREE.MeshStandardMaterial({color: '#4a4c50', roughness: .9})); roof.position.set(x, h + .25, bld.position.z); parent.add(roof);
  }
  // trees: trunk and a cluster of leaf masses, varied in size and tone
  const leafTex = (() => { const [c, g] = canvas(128, 128); g.fillStyle = '#5b7a4c'; g.fillRect(0, 0, 128, 128); for (let n = 0; n < 1400; n++) { const v = 40 + Math.random() * 110 | 0; g.fillStyle = `rgba(${v * .6 | 0},${v},${v * .5 | 0},.6)`; g.beginPath(); g.ellipse(Math.random() * 128, Math.random() * 128, 2 + Math.random() * 4, 1 + Math.random() * 2.5, Math.random() * 3, 0, 6.3); g.fill(); } const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.repeat.set(3, 2); return t; })();
  const trunk = new THREE.CylinderGeometry(.16, .24, 1, 8), leaf = new THREE.IcosahedronGeometry(1, 3);
  { const pa = leaf.attributes.position, v = new THREE.Vector3(); for (let i = 0; i < pa.count; i++) { v.fromBufferAttribute(pa, i); const f = 1 + (Math.sin(v.x * 7.3 + v.y * 3.1) * Math.cos(v.z * 6.1 + v.x * 2.3)) * .16; pa.setXYZ(i, v.x * f, v.y * f, v.z * f); } leaf.computeVertexNormals(); }
  const trunks = new THREE.InstancedMesh(trunk, new THREE.MeshStandardMaterial({color: '#4a3a2b', roughness: 1}), 64), crowns = new THREE.InstancedMesh(leaf, new THREE.MeshStandardMaterial({map: leafTex, color: '#ffffff', roughness: 1}), 64 * 7), m4 = new THREE.Matrix4(), col = new THREE.Color();
  for (let i = 0; i < 64; i++) {
    const side = i % 2 ? 1 : -1, x = side * (14 + (i * 13 % 70)), z = -HALF_Z - 28 - (i * 7 % 30) * 2.4, hgt = 4 + (i * 3 % 5) * .8;
    m4.compose(new THREE.Vector3(x, hgt / 2, z), new THREE.Quaternion(), new THREE.Vector3(1, hgt, 1)); trunks.setMatrixAt(i, m4);
    for (let j = 0; j < 7; j++) {
      const a = j * 2.4 + i, rr = j ? 1.5 : 0, r = 1.1 + ((i + j) % 3) * .35;
      m4.compose(new THREE.Vector3(x + Math.cos(a) * rr, hgt + .9 + (j % 3) * .8, z + Math.sin(a) * rr), new THREE.Quaternion(), new THREE.Vector3(r, r * .8, r));
      crowns.setMatrixAt(i * 7 + j, m4); crowns.setColorAt(i * 7 + j, col.set(['#34492f', '#3f5a36', '#2e4429', '#4a6a3e', '#3a5233'][(i + j) % 5]));
    }
  }
  trunks.castShadow = crowns.castShadow = true; parent.add(trunks, crowns);
}

/* The lights, built once for the visible shop and once for the reflection room, so the paint mirrors the real fixtures. */
function lightRig(target) {
  const white = new THREE.MeshBasicMaterial({color: new THREE.Color(1, 1, 1), toneMapped: false}), warm = new THREE.MeshBasicMaterial({color: new THREE.Color(1, .74, .42), toneMapped: false}), tube = new THREE.MeshBasicMaterial({color: new THREE.Color(1, 1, .97), toneMapped: false});
  const spin = (ox, y, oz, rot) => (u, v) => new THREE.Vector3(ox + u * Math.cos(rot) - v * Math.sin(rot), y, oz + u * Math.sin(rot) + v * Math.cos(rot));
  // ceiling: a large cluster and a lower, smaller one, as in the photos
  hexRig(target, white, [[-1, 0], [0, 0], [1, -1], [1, 0], [2, -1], [0, 1], [-1, 1]], 1.05, spin(-.4, 4.4, -2.2, .08));
  hexRig(target, white, [[0, 0], [1, 0], [0, 1], [-1, 1], [1, -1]], .8, spin(.6, 3.85, 3.4, -.22));
  // the slat wall's vertical hexagon panel (left wall)
  hexRig(target, white, [[0, 0], [0, 1], [-1, 1], [1, 0], [0, 2]], .55, (u, v) => new THREE.Vector3(-HALF_X + .06, 2.5 - v, -1.2 + u));
  // warm vertical tubes beside the door and on the shelf wall; warm strips under the shelves
  for (const [x, y, z] of [[-3.5, 2.4, -HALF_Z + .12], [HALF_X - .1, 2.6, -6.6], [HALF_X - .1, 2.6, 1.6]]) { const t = new THREE.Mesh(new THREE.BoxGeometry(.08, 1.9, .08), warm); t.position.set(x, y, z); target.add(t); }
  for (const [y, z0, z1] of [[1.62, -5.6, -1.4], [2.36, -5.6, -1.4]]) tubeBetween(target, warm, new THREE.Vector3(HALF_X - .3, y, z0), new THREE.Vector3(HALF_X - .3, y, z1));
  // white fluorescents on the shelf wall and above the door wall
  fixture(target, tube, HALF_X - .25, 3.7, -4.6, Math.PI / 2); fixture(target, tube, HALF_X - .25, 3.7, -2.9, Math.PI / 2);
  fixture(target, tube, 4.4, 3.6, -HALF_Z + .25, 0); fixture(target, tube, -4.4, 3.6, -HALF_Z + .25, 0);
  fixture(target, tube, 0, 3.9, HALF_Z - .25, Math.PI);
}

async function buildGarage() {
  garageSet = new THREE.Group(); garageSet.position.copy(GARAGE); scene.add(garageSet);
  const blockTex = repeatTex(blockCanvas()), floorTex = repeatTex(tileCanvas('#15161a', '#1f2024', 'rgba(255,255,255,.075)', true), (HALF_X * 2) / 3.2, (HALF_Z * 2) / 3.2);
  floorTex.repeat.set((HALF_X * 2) / 3.2, (HALF_Z * 2) / 3.2);
  const wallOf = (w, h) => { const t = blockTex.clone(); t.needsUpdate = true; t.repeat.set(w / 3.2, h / 1.6); return new THREE.MeshStandardMaterial({map: t, roughness: .93, metalness: .02}); };

  // reflections: the same lights in a dark room, so the paint mirrors the hexagon rigs
  const room = new THREE.Scene(); room.background = new THREE.Color('#060709');
  const roomBox = new THREE.Mesh(new THREE.BoxGeometry(HALF_X * 2, HEIGHT, HALF_Z * 2), new THREE.MeshBasicMaterial({color: '#0c0d10', side: THREE.BackSide})); roomBox.position.y = HEIGHT / 2; room.add(roomBox);
  lightRig(room); room.traverse((o) => { if (o.isMesh && o.material?.isMeshBasicMaterial && o.material.color.r >= .99) { o.material = o.material.clone(); o.material.color.setScalar(9); } });
  const doorGlow = new THREE.Mesh(new THREE.PlaneGeometry(DOOR_W, DOOR_H), new THREE.MeshBasicMaterial({color: new THREE.Color(1.5, 1.6, 1.8)})); doorGlow.position.set(0, DOOR_H / 2, -HALF_Z + .2); room.add(doorGlow);
  const pm = new THREE.PMREMGenerator(renderer); garageEnv = pm.fromScene(room, .02).texture; pm.dispose();

  // floor with the three red bays
  const floor = new THREE.Mesh(new THREE.PlaneGeometry(HALF_X * 2, HALF_Z * 2), new THREE.MeshStandardMaterial({map: floorTex, roughness: .4, metalness: .06}));
  floor.rotation.x = -Math.PI / 2; floor.receiveShadow = true; garageSet.add(floor);
  const redMat = new THREE.MeshStandardMaterial({map: repeatTex(tileCanvas('#a9171f', '#bd2029', 'rgba(0,0,0,.28)', false), 9 / 8, 2 / 8), roughness: .5, metalness: .1});
  for (const z of [-7.2, -4.6, 4.4]) { const band = new THREE.Mesh(new THREE.PlaneGeometry(3.6, .8), redMat); band.rotation.x = -Math.PI / 2; band.position.set(0, .004, z); band.receiveShadow = true; garageSet.add(band); }

  // walls: block, with the door opening in the end wall, the slat wall on the left and the loft behind
  const plane = (w, h, mat, x, y, z, ry) => { const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat); m.position.set(x, y, z); m.rotation.y = ry; m.receiveShadow = true; garageSet.add(m); return m; };
  plane(HALF_Z * 2, HEIGHT, wallOf(HALF_Z * 2, HEIGHT), HALF_X, HEIGHT / 2, 0, -Math.PI / 2);
  plane(HALF_Z * 2, HEIGHT, wallOf(HALF_Z * 2, HEIGHT), -HALF_X, HEIGHT / 2, 0, Math.PI / 2);
  plane(HALF_X * 2, HEIGHT, wallOf(HALF_X * 2, HEIGHT), 0, HEIGHT / 2, HALF_Z, Math.PI);
  const side = (HALF_X - DOOR_W / 2);
  plane(side, HEIGHT, wallOf(side, HEIGHT), -(HALF_X + DOOR_W / 2) / 2, HEIGHT / 2, -HALF_Z, 0);
  plane(side, HEIGHT, wallOf(side, HEIGHT), (HALF_X + DOOR_W / 2) / 2, HEIGHT / 2, -HALF_Z, 0);
  plane(DOOR_W, HEIGHT - DOOR_H, wallOf(DOOR_W, HEIGHT - DOOR_H), 0, DOOR_H + (HEIGHT - DOOR_H) / 2, -HALF_Z, 0);
  const frameMat = new THREE.MeshStandardMaterial({color: '#0b0b0d', roughness: .7});
  for (const [w, h, x, y] of [[.16, DOOR_H, -DOOR_W / 2, DOOR_H / 2], [.16, DOOR_H, DOOR_W / 2, DOOR_H / 2], [DOOR_W + .3, .16, 0, DOOR_H]]) { const f = new THREE.Mesh(new THREE.BoxGeometry(w, h, .22), frameMat); f.position.set(x, y, -HALF_Z + .05); garageSet.add(f); }
  const slatTex = repeatTex(slatCanvas(), 4.2, 3.2);
  plane(8, 3.2, new THREE.MeshStandardMaterial({map: slatTex, roughness: .55, metalness: .25}), -HALF_X + .02, 1.6, -1.2, Math.PI / 2);
  const ceilMat = new THREE.MeshStandardMaterial({color: '#08080a', roughness: 1});
  const ceil = new THREE.Mesh(new THREE.PlaneGeometry(HALF_X * 2, HALF_Z * 2), ceilMat); ceil.position.y = HEIGHT; ceil.rotation.x = Math.PI / 2; garageSet.add(ceil);
  for (const z of [-8, -3.5, 1, 5.5]) { const beam = new THREE.Mesh(new THREE.BoxGeometry(HALF_X * 2, .38, .22), new THREE.MeshStandardMaterial({color: '#0a0a0c', roughness: .9})); beam.position.set(0, HEIGHT - .22, z); garageSet.add(beam); }

  // outside the door: a concrete apron, the street, buildings and trees, under the coastal sky
  outsideScenery(garageSet);

  // the roller door: it sits just outside the opening and rolls up behind the lintel
  const doorTex = new THREE.CanvasTexture(doorCanvas()); doorTex.colorSpace = THREE.SRGBColorSpace; doorTex.anisotropy = 8;
  door = new THREE.Mesh(new THREE.PlaneGeometry(DOOR_W, DOOR_H), new THREE.MeshStandardMaterial({map: doorTex, roughness: .7, metalness: .15, side: THREE.DoubleSide}));
  door.position.set(0, DOOR_H / 2, -HALF_Z - .1); garageSet.add(door);

  lightRig(garageSet);

  // logo sign on the shelf wall
  try {
    const logo = await new THREE.TextureLoader().loadAsync(new URL('./assets/logo.webp', import.meta.url).href); logo.colorSpace = THREE.SRGBColorSpace;
    const sign = new THREE.Mesh(new THREE.PlaneGeometry(2.1, 2.1), new THREE.MeshBasicMaterial({map: logo, transparent: true})); sign.position.set(HALF_X - .03, 3.55, 3.4); sign.rotation.y = -Math.PI / 2; garageSet.add(sign);
  } catch { /* the sign is decoration */ }

  // ----- props -----
  const mats = new Map();
  const std = (color, rough = .6, metal = 0) => { const k = color + rough + metal; if (!mats.has(k)) mats.set(k, new THREE.MeshStandardMaterial({color, roughness: rough, metalness: metal})); return mats.get(k); };
  const box = (w, h, d, m, x, y, z, parent = garageSet) => { const o = new THREE.Mesh(Math.min(w, h, d) < .04 || Math.max(w, h, d) > 6 ? new THREE.BoxGeometry(w, h, d) : roundedBox(w, h, d, Math.min(.03, Math.min(w, h, d) * .28)), m); o.position.set(x, y, z); o.castShadow = o.receiveShadow = true; parent.add(o); return o; };
  const liquids = ['#8c4fb5', '#e7ecef', '#f1eee2', '#d7dadc', '#c64a5d', '#4aa3d8', '#e8a23a', '#3fb58a'];
  // detailing chemicals: HDPE jugs and trigger sprays with printed labels (abstract artwork, not any real brand)
  const labelTex = ['#c0392b', '#1f6f9c', '#d9a21b', '#2b2f33', '#1e8449', '#7d3c98'].map((accent) => {
    const [c, g] = canvas(256, 128);
    g.fillStyle = '#f2f3f4'; g.fillRect(0, 0, 256, 128); g.fillStyle = accent; g.fillRect(0, 0, 256, 30); g.fillRect(0, 104, 256, 24);
    g.fillStyle = '#15171a'; g.beginPath(); g.arc(50, 68, 21, 0, 6.3); g.fill(); g.fillStyle = accent; g.fillRect(41, 60, 18, 16);
    for (let r = 0; r < 4; r++) { g.fillStyle = r ? 'rgba(25,27,31,.7)' : '#15171a'; g.fillRect(86, 46 + r * 15, 118 - r * 22 + (r % 2) * 14, r ? 5 : 9); }
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 8; return t;
  });
  const labelMats = labelTex.map((map) => new THREE.MeshStandardMaterial({map, roughness: .5}));
  const lathe = (pts, seg = 28) => new THREE.LatheGeometry(pts.map(([r, y]) => new THREE.Vector2(r, y)), seg);
  const jugGeo = lathe([[0, 0], [.083, 0], [.094, .012], [.098, .07], [.098, .2], [.092, .245], [.062, .282], [.036, .298], [.034, .318], [0, .318]]);
  const capGeo = new THREE.CylinderGeometry(.037, .04, .034, 20), jugLabel = new THREE.CylinderGeometry(.0995, .0995, .15, 28, 1, true, -Math.PI / 2 - 1.2, 2.4);
  const handleGeo = new THREE.TorusGeometry(.04, .009, 8, 16, Math.PI), bottleGeo = lathe([[0, 0], [.034, 0], [.037, .01], [.037, .12], [.031, .148], [.022, .165], [0, .165]], 20);
  const liquidGeo = lathe([[0, .005], [.0315, .005], [.0345, .014], [.0345, .095], [0, .095]], 20), sprayLabel = new THREE.CylinderGeometry(.0378, .0378, .075, 20, 1, true, -1.3, 2.6);
  const shellMat = new THREE.MeshPhysicalMaterial({color: '#dfe9ee', transparent: true, opacity: .3, roughness: .08, clearcoat: 1, depthWrite: false});
  const caps = ['#1b1c1f', '#c0392b', '#d9a21b', '#1f6f9c', '#e8e8e8'].map((c) => std(c, .45));
  const jug = (x, y, z, i) => {
    const grp = new THREE.Group(), s = .82 + (i * 7 % 5) * .08;
    const body = new THREE.Mesh(jugGeo, new THREE.MeshPhysicalMaterial({color: new THREE.Color('#f1f3f4').lerp(new THREE.Color(liquids[i % liquids.length]), .1 + (i % 3) * .06), roughness: .38, clearcoat: .25}));
    const label = new THREE.Mesh(jugLabel, labelMats[i % labelMats.length]); label.position.y = .13;
    const cap = new THREE.Mesh(capGeo, caps[i % caps.length]); cap.position.y = .335;
    const handle = new THREE.Mesh(handleGeo, body.material); handle.position.set(.07, .27, 0); handle.rotation.set(0, Math.PI / 2, Math.PI / 2 + .3);
    grp.add(body, label, cap, handle); grp.position.set(x, y, z); grp.scale.setScalar(s); grp.rotation.y = ((i * 37) % 11 - 5) * .04; garageSet.add(grp);
  };
  const spray = (x, y, z, i) => {
    const grp = new THREE.Group();
    const shell = new THREE.Mesh(bottleGeo, shellMat), liquid = new THREE.Mesh(liquidGeo, new THREE.MeshStandardMaterial({color: liquids[(i * 3) % liquids.length], roughness: .15, transparent: true, opacity: .88}));
    const label = new THREE.Mesh(sprayLabel, labelMats[(i + 2) % labelMats.length]); label.position.y = .07;
    const head = std(i % 2 ? '#d9a21b' : '#1b1c1f', .4), neck = new THREE.Mesh(new THREE.CylinderGeometry(.021, .024, .03, 14), head); neck.position.y = .18;
    const hb = new THREE.Mesh(new THREE.BoxGeometry(.036, .045, .052), head); hb.position.set(0, .213, .006);
    const nozzle = new THREE.Mesh(new THREE.CylinderGeometry(.009, .011, .045, 10), head); nozzle.rotation.x = Math.PI / 2; nozzle.position.set(0, .216, .048);
    const trig = new THREE.Mesh(new THREE.BoxGeometry(.012, .05, .01), head); trig.position.set(0, .19, .033); trig.rotation.x = -.35;
    const tube = new THREE.Mesh(new THREE.CylinderGeometry(.0025, .0025, .15, 6), std('#d4dadd', .3)); tube.position.set(.01, .08, 0);
    grp.add(shell, liquid, label, neck, hb, nozzle, trig, tube); grp.position.set(x, y, z); grp.rotation.y = -Math.PI / 2 + ((i * 53) % 9 - 4) * .1; garageSet.add(grp);
  };
  const wood = std('#5b3d26', .75);
  for (const y of [1.7, 2.4]) box(.34, .045, 4.4, wood, HALF_X - .22, y, -3.5);
  for (let i = 0; i < 13; i++) jug(HALF_X - .22, 2.43, -5.5 + i * .33, i);
  for (let i = 0; i < 9; i++) { if (i % 3 === 0) jug(HALF_X - .22, 1.73, -5.4 + i * .45, i + 2); else spray(HALF_X - .22, 1.73, -5.4 + i * .45, i); }
  // the folding workbench with its row of bottles and the bins under it
  box(.82, .05, 3.2, std('#e6e3dc', .5), HALF_X - .75, .86, -1.2);
  for (const [dx, dz] of [[-.36, -1.45], [.36, -1.45], [-.36, 1.45], [.36, 1.45]]) box(.04, .84, .04, std('#9aa0a6', .4, .6), HALF_X - .75 + dx, .42, -1.2 + dz);
  for (let i = 0; i < 18; i++) spray(HALF_X - .45, .885, -2.6 + i * .17, i);
  for (let i = 0; i < 3; i++) { box(.5, .32, .62, std('#16171a', .6), HALF_X - 1.0, .2, -2.2 + i * .75); box(.5, .05, .62, std('#d9b300', .6), HALF_X - 1.0, .385, -2.2 + i * .75); }

  // microfibre towel stacks and two wash buckets with grit guards
  const towelTex = (() => { const [c, g] = canvas(64, 64); g.fillStyle = '#fff'; g.fillRect(0, 0, 64, 64); for (let n = 0; n < 900; n++) { const v = 200 + Math.random() * 55 | 0; g.fillStyle = `rgb(${v},${v},${v})`; g.fillRect(Math.random() * 64, Math.random() * 64, 1, 1); } for (let k = 0; k < 64; k += 4) { g.fillStyle = 'rgba(0,0,0,.08)'; g.fillRect(k, 0, 1, 64); g.fillRect(0, k, 64, 1); } const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.repeat.set(3, 3); return t; })();
  const towelStack = (x, y, z, colors) => colors.forEach((col, k) => { const t = new THREE.Mesh(new THREE.BoxGeometry(.36, .028, .3), new THREE.MeshStandardMaterial({color: col, map: towelTex, roughness: 1})); t.position.set(x + (k % 2) * .01, y + .014 + k * .029, z); t.rotation.y = (k - 1.5) * .06; t.castShadow = true; garageSet.add(t); });
  towelStack(HALF_X - 1.0, .885, -2.45, ['#c0392b', '#e8e8e8', '#1f6f9c', '#2b2f33']); towelStack(HALF_X - 1.0, .885, -1.8, ['#d9a21b', '#2b2f33', '#e8e8e8']); towelStack(HALF_X - 1.0, .885, -.55, ['#1e8449', '#e8e8e8', '#c0392b']);
  const bucketGeo = new THREE.LatheGeometry([[0, 0], [.105, 0], [.158, .29], [.168, .3], [.165, .312], [.154, .308], [.148, .29], [.1, .03], [0, .03]].map(([r, y]) => new THREE.Vector2(r, y)), 32);
  [['#1b1c1f', HALF_X - 1.3, -3.7], ['#b3121d', HALF_X - 1.55, -4.3]].forEach(([col, x, z]) => {
    const b = new THREE.Mesh(bucketGeo, new THREE.MeshStandardMaterial({color: col, roughness: .45, side: THREE.DoubleSide})); b.position.set(x, 0, z); b.castShadow = true; garageSet.add(b);
    const guard = new THREE.Mesh(new THREE.CylinderGeometry(.14, .14, .012, 28), std('#3a3d42', .6)); guard.position.set(x, .12, z); garageSet.add(guard);
    const water = new THREE.Mesh(new THREE.CircleGeometry(.152, 28), new THREE.MeshStandardMaterial({color: '#9fb8c4', roughness: .05, metalness: .2, transparent: true, opacity: .8})); water.rotation.x = -Math.PI / 2; water.position.set(x, .24, z); garageSet.add(water);
    const h = new THREE.Mesh(new THREE.TorusGeometry(.17, .005, 6, 24, Math.PI), std('#9aa0a6', .4, .8)); h.position.set(x, .3, z); h.rotation.set(0, 0.7, 0); garageSet.add(h);
  });
  // shop vacs, generator
  const vac = (x, z) => { const g = new THREE.Group(); const body = new THREE.Mesh(new THREE.CylinderGeometry(.28, .25, .52, 18), std('#c07416', .55)); body.position.y = .27; const lid = new THREE.Mesh(new THREE.CylinderGeometry(.29, .29, .08, 18), std('#17181a', .5)); lid.position.y = .56; const hose = new THREE.Mesh(new THREE.TorusGeometry(.3, .03, 8, 24, Math.PI * 1.2), std('#101012', .7)); hose.position.set(.35, .45, 0); hose.rotation.y = Math.PI / 2; g.add(body, lid, hose); g.position.set(x, 0, z); g.traverse((o) => { o.castShadow = true; }); garageSet.add(g); };
  vac(HALF_X - 1.5, 0.9); vac(-HALF_X + 1.2, 3.2); vac(-HALF_X + 1.0, -5.8);
  box(.74, .56, .5, std('#a3131b', .45), HALF_X - 1.4, .36, -5.9); box(.8, .05, .56, std('#111214', .6), HALF_X - 1.4, .66, -5.9);
  // the loft: plywood deck, timber posts and rails, black sheeting behind
  box(9.2, .16, 3.2, std('#17181b', .8), 0, 3.1, HALF_Z - 1.6);
  const timber = std('#b58853', .7);
  for (let i = 0; i < 10; i++) box(.08, 1.0, .08, timber, -4.5 + i, 3.68, HALF_Z - 3.18);
  box(9.2, .07, .07, timber, 0, 4.18, HALF_Z - 3.18); box(9.2, .07, .07, timber, 0, 3.75, HALF_Z - 3.18);
  plane(9.4, 2.4, new THREE.MeshStandardMaterial({color: '#050507', roughness: .3, metalness: .35}), 0, 4.05, HALF_Z - .04, Math.PI);
  // waiting chairs and the tall cabinet beneath the loft
  for (let i = 0; i < 4; i++) { const x = -4.6 + i * .55; box(.44, .04, .42, std('#2a2b2f', .6), x, .46, HALF_Z - 1.0); box(.44, .4, .03, std('#2a2b2f', .6), x, .72, HALF_Z - 1.2); for (const [dx, dz] of [[-.18, -.17], [.18, -.17], [-.18, .17], [.18, .17]]) box(.025, .44, .025, std('#555a60', .4, .7), x + dx, .22, HALF_Z - 1.0 + dz); }
  box(.95, 1.95, .5, std('#101113', .5, .3), 1.2, .98, HALF_Z - .5);
  for (let i = 0; i < 4; i++) for (let j = 0; j < 5; j++) box(.12, .1, .1, std(liquids[(i + j) % liquids.length], .5), .85 + j * .16, .35 + i * .45, HALF_Z - .72);

  garageKey = new THREE.DirectionalLight('#ffffff', 1.7); garageKey.position.set(GARAGE.x + 1.5, 9, 2); garageKey.castShadow = true;
  garageKey.shadow.mapSize.set(2048, 2048); Object.assign(garageKey.shadow.camera, {left: -6, right: 6, top: 6, bottom: -6, near: 1, far: 22}); garageKey.shadow.bias = -.0005; garageKey.shadow.normalBias = .03;
  garageKey.target.position.copy(GARAGE); scene.add(garageKey, garageKey.target);
  // light that comes from the fixtures themselves
  const a = new THREE.PointLight('#fff4e6', 70, 16, 2); a.position.set(-.2, 4.0, -2.2); const b = new THREE.PointLight('#f2f6ff', 48, 14, 2); b.position.set(.6, 3.4, 3.4);
  const c = new THREE.PointLight('#ffd7a3', 22, 10, 2); c.position.set(HALF_X - 1, 2.2, -3);
  garageSet.add(a, b, c, new THREE.HemisphereLight('#dfe8ff', '#0c0d10', .55));
}

/* ---------- the car ---------- */
function splitBy(geo, groupOf, count) {
  const g = geo.index ? geo.toNonIndexed() : geo;
  const lists = Array.from({length: count}, () => []), p = g.attributes.position, v = new THREE.Vector3(), t = new THREE.Vector3();
  for (let i = 0; i < p.count; i += 3) { v.set(0, 0, 0); for (let j = 0; j < 3; j++) v.add(t.fromBufferAttribute(p, i + j)); v.multiplyScalar(1 / 3); lists[groupOf(v)].push(i, i + 1, i + 2); }
  return lists.map((list) => {
    if (!list.length) return null;
    const out = new THREE.BufferGeometry();
    for (const [k, a] of Object.entries(g.attributes)) { const vals = new Float32Array(list.length * a.itemSize); list.forEach((idx, i) => { for (let j = 0; j < a.itemSize; j++) vals[i * a.itemSize + j] = a.getComponent(idx, j); }); out.setAttribute(k, new THREE.BufferAttribute(vals, a.itemSize)); }
    return out;
  });
}
/* Geometry in the car's frame with plain float attributes (the file is quantised, so attributes are decoded here). */
function geomOf(mesh) {
  const src = mesh.geometry, out = new THREE.BufferGeometry();
  for (const name of ['position', 'normal', 'uv']) {
    const a = src.attributes[name]; if (!a) continue;
    const vals = new Float32Array(a.count * a.itemSize);
    for (let i = 0; i < a.count; i++) for (let j = 0; j < a.itemSize; j++) vals[i * a.itemSize + j] = a.getComponent(i, j);
    out.setAttribute(name, new THREE.BufferAttribute(vals, a.itemSize));
  }
  if (src.index) out.setIndex(src.index.clone());
  out.applyMatrix4(mesh.matrixWorld);
  return out;
}

async function buildCar() {
  const gltf = await new GLTFLoader().setMeshoptDecoder(MeshoptDecoder).loadAsync(new URL('./assets/three/mercedes-gls.glb', import.meta.url).href);
  const model = gltf.scene; model.updateMatrixWorld(true);
  car = new THREE.Group(); body = new THREE.Group(); car.add(body); scene.add(car);
  paintMat = paintShader(new THREE.MeshPhysicalMaterial({color: '#0c0e12', metalness: .55, roughness: .22, clearcoat: 1, clearcoatRoughness: .03}));
  // the panels overlap the trim meshes underneath at the seams; the paint must always win those depth ties
  paintMat.polygonOffset = true; paintMat.polygonOffsetFactor = -3; paintMat.polygonOffsetUnits = -3;
  glassMat = glassDirt(new THREE.MeshPhysicalMaterial({color: '#0a0c0f', metalness: .1, roughness: .03, transparent: true, opacity: .3, clearcoat: 1, side: THREE.DoubleSide, depthWrite: false}));
  const look = {
    Tyres: new THREE.MeshStandardMaterial({color: '#141416', roughness: .82}),
    Interior: new THREE.MeshStandardMaterial({color: '#1a1714', roughness: .7}),
    UnderCarriage: new THREE.MeshStandardMaterial({color: '#0b0b0c', roughness: .9}),
    Grille: new THREE.MeshStandardMaterial({color: '#0a0a0b', metalness: .9, roughness: .25}),
    ChromeBlack_Windows: new THREE.MeshStandardMaterial({color: '#08090a', metalness: .9, roughness: .2}),
    Lights_Glass: new THREE.MeshPhysicalMaterial({color: '#e9eef2', roughness: .02, transparent: true, opacity: .35, metalness: .2}),
    Color_A07: new THREE.MeshStandardMaterial({color: '#7a0008', emissive: '#3a0003', roughness: .2}),
    Color_A08: new THREE.MeshStandardMaterial({color: '#5a0006', roughness: .2}),
  };
  const chrome = new THREE.MeshStandardMaterial({color: '#cfd3d8', metalness: 1, roughness: .18});
  const greys = new Map();
  const grey = (hex) => { if (!greys.has(hex)) greys.set(hex, new THREE.MeshStandardMaterial({color: hex, metalness: .85, roughness: .3})); return greys.get(hex); };
  const bodyBox = new THREE.Box3().setFromObject(model), half = bodyBox.getSize(new THREE.Vector3()).x / 2;
  // wheel centres from the tyres
  const all = []; model.traverse((n) => { if (n.isMesh) all.push(n); });
  const tyreMesh = all.find((m) => (m.material?.name || m.name) === 'Tyres'), wheelInfo = [];
  if (tyreMesh) {
    splitBy(geomOf(tyreMesh), (v) => (v.x > 0 ? 1 : 0) + (v.z > 0 ? 2 : 0), 4).forEach((g) => {
      if (!g) return; g.computeBoundingBox(); const b = g.boundingBox;
      wheelInfo.push({c: b.getCenter(new THREE.Vector3()), r: Math.max(b.max.y - b.min.y, b.max.z - b.min.z) / 2 + .02, pieces: []});
    });
  }
  const wheelOf = (v) => wheelInfo.findIndex((w) => Math.hypot(v.y - w.c.y, v.z - w.c.z) < w.r * 1.02 && Math.abs(v.x - w.c.x) < .25);
  for (const n of all) {
    const name = n.material?.name || n.name, g0 = geomOf(n), g = /^(Polar_White|Tyres|Color_M)/.test(name) ? toCreasedNormals(g0, .6) : g0; // the file's quantised normals zig-zag along the panel seams; rebuild them
    if (name === 'WindowsTint') {
      // the windows: the windscreen stays, the side and rear glass lift out for tinting
      const pieces = splitBy(g, (v) => (Math.abs(v.x) > half * .62 ? (v.x > 0 ? 1 : 2) : v.z < 0 ? 3 : 0), 4);
      const dirs = [null, new THREE.Vector3(1, .55, 0), new THREE.Vector3(-1, .55, 0), new THREE.Vector3(0, .5, -1)];
      pieces.forEach((pg, i) => { if (!pg) return; const m = new THREE.Mesh(pg, glassMat); m.renderOrder = 2; body.add(m); if (i) glassPieces.push({mesh: m, dir: dirs[i].normalize()}); });
      continue;
    }
    const mat = name === 'Polar_White' ? paintMat : look[name] || (/Color_M0[12]/.test(name) ? chrome : grey('#' + (n.material?.color?.getHexString?.() || '777777')));
    if (wheelInfo.length && name !== 'Polar_White' && /Tyres|Color_M/.test(name)) {
      // wheel parts (tyre, rim, brake) go into their own spinning pivots
      const nearest = (v) => { let bi = 0, bd = 1e9; wheelInfo.forEach((w, wi) => { const d = Math.hypot(v.x - w.c.x, v.y - w.c.y, v.z - w.c.z); if (d < bd) { bd = d; bi = wi; } }); return bi; };
      const parts = splitBy(g, name === 'Tyres' ? (v) => nearest(v) + 1 : (v) => wheelOf(v) + 1, wheelInfo.length + 1);
      parts.forEach((pg, i) => { if (!pg) return; if (i === 0) { const m = new THREE.Mesh(pg, mat); m.castShadow = m.receiveShadow = true; body.add(m); } else wheelInfo[i - 1].pieces.push({g: pg, mat}); });
      continue;
    }
    const m = new THREE.Mesh(g, mat); m.castShadow = true; m.receiveShadow = true; body.add(m);
    if (name === 'Polar_White') {
      // film panels for the PPF step: a slightly larger clear copy that comes down onto the body, with cyan trim lines
      const film = new THREE.Mesh(g, filmShader(new THREE.MeshPhysicalMaterial({color: '#dff7ff', roughness: .05, transparent: true, opacity: 0, clearcoat: 1, depthWrite: false})));
      film.scale.setScalar(1.006); film.renderOrder = 3; film.visible = false; body.add(film); filmPieces.push(film);
    }
  }
  for (const w of wheelInfo) {
    const pivot = new THREE.Group(); pivot.position.copy(w.c); body.add(pivot);
    w.pieces.forEach((p) => { p.g.translate(-w.c.x, -w.c.y, -w.c.z); const m = new THREE.Mesh(p.g, wheelDirt(p.mat.clone(), w.c.z, p.mat !== look.Tyres)); m.castShadow = true; pivot.add(m); });
    wheels.push(pivot);
  }
  buildEffects(bodyBox);
}

/* ---------- tools and water ---------- */
const NOISE_GLSL = `
  varying vec3 vL;
  float h3(vec3 p){return fract(sin(dot(p,vec3(127.1,311.7,74.7)))*43758.5453);}
  float n3(vec3 p){vec3 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(mix(h3(i),h3(i+vec3(1,0,0)),f.x),mix(h3(i+vec3(0,1,0)),h3(i+vec3(1,1,0)),f.x),f.y),mix(mix(h3(i+vec3(0,0,1)),h3(i+vec3(1,0,1)),f.x),mix(h3(i+vec3(0,1,1)),h3(i+vec3(1,1,1)),f.x),f.y),f.z);}`;
/* Road dirt on the tyres and wheels: dust, caked mud, brake dust on the rims; washed off by the same rinse front as the paint. */
function wheelDirt(mat, zc, rim) {
  const uZc = {value: zc};
  mat.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, {uDirt: U.dirt, uFoam: U.foam, uFoamZ: U.foamZ, uRinse: U.rinse, uZc});
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', `#include <common>\n${NOISE_GLSL}\nuniform float uDirt,uFoam,uFoamZ,uRinse,uZc; float dirtM=0.,foamM=0.;`)
      .replace('#include <color_fragment>', `#include <color_fragment>
      float w=smoothstep(-.25,.25,uZc-mix(3.0,-3.0,uRinse));
      float g=n3(vL*13.)*.5+n3(vL*41.)*.3+n3(vL*130.)*.2;
      float lump=smoothstep(.35,.95,n3(vL*90.+3.))*.35;
      dirtM=clamp(${rim ? '.5+g*.3' : '.3+g*.35+lump*.3'},0.,.8)*uDirt*(1.-w);
      vec3 mud=mix(vec3(.24,.21,.17),vec3(.12,.09,.065),lump+.2)*(.75+.5*g);
      ${rim ? 'mud=mix(mud,vec3(.11,.085,.065),.45);' : ''}
      diffuseColor.rgb=mix(diffuseColor.rgb,mud,dirtM*.94);
      foamM=uFoam*(1.-w)*smoothstep(.2,.55,n3(vL*38.)*.65+.3)*(1.-smoothstep(uFoamZ-.5,uFoamZ+.05,uZc));
      diffuseColor.rgb=mix(diffuseColor.rgb,vec3(.95,.96,.97),foamM);`)
      .replace('#include <roughnessmap_fragment>', '#include <roughnessmap_fragment>\nroughnessFactor=mix(roughnessFactor,.96,max(dirtM,foamM));')
      .replace('#include <metalnessmap_fragment>', '#include <metalnessmap_fragment>\nmetalnessFactor=mix(metalnessFactor,.04,max(dirtM,foamM)*.9);');
  };
  mat.customProgramCacheKey = () => (rim ? 'dapper-wheel-rim-v3' : 'dapper-wheel-tyre-v3');
  return mat;
}
/* Dusty, streaked road film on the glass that rinses off with the paint; panes that are out catch the light at their edge. */
function glassDirt(mat) {
  mat.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, {uDirt: U.dirt, uRinse: U.rinse, uOut: U.out});
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', `#include <common>\n${NOISE_GLSL}\nuniform float uDirt,uRinse,uOut;`)
      .replace('#include <color_fragment>', `#include <color_fragment>
      float w=smoothstep(-.25,.25,vL.z-mix(3.0,-3.0,uRinse));
      float g=n3(vL*16.)*.5+n3(vL*70.)*.5;
      float streak=smoothstep(.5,.9,n3(vec3(vL.x*30.,vL.y*1.5,vL.z*30.)));
      float gd=uDirt*(1.-w)*clamp(.34+.45*g+(1.-smoothstep(.5,1.5,vL.y))*.3-streak*.22,0.,1.);
      diffuseColor.rgb=mix(diffuseColor.rgb,vec3(.34,.3,.25),gd*.55);
      diffuseColor.a=mix(diffuseColor.a,.92,gd*.6);`)
      .replace('#include <opaque_fragment>', `#include <opaque_fragment>
      float edge=pow(1.-abs(dot(normalize(vNormal),normalize(vViewPosition))),3.);
      gl_FragColor.rgb+=vec3(.55,.68,.66)*edge*uOut*.0; gl_FragColor.a=max(gl_FragColor.a,edge*uOut*.0);`);
  };
  mat.customProgramCacheKey = () => 'dapper-glass-v1';
  return mat;
}

/* soft sprites for foam, mist and droplets */
function puffTexture(kind) {
  const [c, x] = canvas(128, 128);
  const rg = (a0, a1) => { const g = x.createRadialGradient(64, 64, 2, 64, 64, 62); g.addColorStop(0, `rgba(255,255,255,${a0})`); g.addColorStop(.55, `rgba(255,255,255,${a0 * .6})`); g.addColorStop(1, `rgba(255,255,255,${a1})`); return g; };
  if (kind === 'foam') {
    x.fillStyle = rg(.8, 0); x.fillRect(0, 0, 128, 128);
    for (let n = 0; n < 46; n++) { const a = Math.random() * 6.3, d = Math.random() * 46, r = 4 + Math.random() * 9; x.fillStyle = 'rgba(255,255,255,.55)'; x.strokeStyle = 'rgba(190,205,215,.55)'; x.beginPath(); x.arc(64 + Math.cos(a) * d, 64 + Math.sin(a) * d, r, 0, 6.3); x.fill(); x.stroke(); }
  } else if (kind === 'drop') {
    x.fillStyle = rg(.95, 0); x.fillRect(0, 0, 128, 128);
  } else {
    x.fillStyle = rg(.5, 0); x.fillRect(0, 0, 128, 128);
    for (let n = 0; n < 30; n++) { x.fillStyle = `rgba(255,255,255,${.04 + Math.random() * .05})`; x.beginPath(); x.arc(20 + Math.random() * 88, 20 + Math.random() * 88, 8 + Math.random() * 20, 0, 6.3); x.fill(); }
  }
  const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; return t;
}
function particles(n, map, color) {
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(n * 3), 3));
  geo.setAttribute('aSize', new THREE.BufferAttribute(new Float32Array(n), 1));
  geo.setAttribute('aAlpha', new THREE.BufferAttribute(new Float32Array(n), 1));
  const mat = new THREE.ShaderMaterial({
    uniforms: {uMap: {value: map}, uColor: {value: new THREE.Color(color)}, uScale: {value: 600}},
    vertexShader: 'attribute float aSize; attribute float aAlpha; varying float vA; uniform float uScale; void main(){ vA=aAlpha; vec4 mv=modelViewMatrix*vec4(position,1.); gl_PointSize=max(aSize*uScale/-mv.z,0.); gl_Position=projectionMatrix*mv; }',
    fragmentShader: 'uniform sampler2D uMap; uniform vec3 uColor; varying float vA; void main(){ vec4 t=texture2D(uMap,gl_PointCoord); gl_FragColor=vec4(uColor*t.rgb,t.a*vA);\n#include <tonemapping_fragment>\n#include <colorspace_fragment>\n}',
    transparent: true, depthWrite: false,
  });
  const pts = new THREE.Points(geo, mat); pts.frustumCulled = false; particleMats.push(mat);
  pts.userData.q = Array.from({length: n}, () => [Math.random(), Math.random(), Math.random()]);
  return pts;
}
const putParticle = (pts, i, x, y, z, size, alpha) => {
  const g = pts.geometry; g.attributes.position.setXYZ(i, x, y, z); g.attributes.aSize.setX(i, size); g.attributes.aAlpha.setX(i, alpha);
};
const flushParticles = (pts) => { const g = pts.geometry; g.attributes.position.needsUpdate = g.attributes.aSize.needsUpdate = g.attributes.aAlpha.needsUpdate = true; };

/* A pressure-washer gun with its wand, or a foam lance (a wand with a foam bottle). It sprays along +x from the origin (the nozzle);
   the hose runs back and down to the floor. */
function washerRig({accent, length, tilt, bottle, floor}) {
  const root = new THREE.Group(), gun = new THREE.Group(); gun.rotation.z = -tilt; root.add(gun);
  const steel = new THREE.MeshStandardMaterial({color: '#cdd2d7', metalness: 1, roughness: .2}), black = new THREE.MeshStandardMaterial({color: '#17181a', roughness: .5, metalness: .25});
  const paint = new THREE.MeshStandardMaterial({color: accent, roughness: .4, metalness: .15}), rubber = new THREE.MeshStandardMaterial({color: '#1a1b1e', roughness: .65});
  const cyl = (r0, r1, len, mat, x, y = 0) => { const m = new THREE.Mesh(new THREE.CylinderGeometry(r0, r1, len, 20), mat); m.rotation.z = Math.PI / 2; m.position.set(x, y, 0); m.castShadow = true; gun.add(m); return m; };
  cyl(.0105, .0105, length, steel, -length / 2); cyl(.019, .0145, .055, paint, -.012); cyl(.0145, .0145, .02, steel, .02);
  cyl(.017, .017, .06, black, -length + .06);
  const body = new THREE.Mesh(roundedBox(.17, .075, .05, .02), paint); body.position.set(-length - .1, .0, 0); body.castShadow = true; gun.add(body);
  const grip = new THREE.Mesh(new THREE.CapsuleGeometry(.02, .12, 4, 10), black); grip.position.set(-length - .13, -.1, 0); grip.rotation.z = .22; grip.castShadow = true; gun.add(grip);
  const trig = new THREE.Mesh(new THREE.CapsuleGeometry(.007, .05, 3, 6), black); trig.position.set(-length - .065, -.075, 0); trig.rotation.z = -.35; gun.add(trig);
  cyl(.014, .014, .07, steel, -length - .22);
  if (bottle) {
    const prof = [[0, 0], [.04, 0], [.048, .012], [.05, .2], [.044, .235], [.02, .252], [.02, .272], [0, .272]].map(([r, y]) => new THREE.Vector2(r, y));
    const shell = new THREE.Mesh(new THREE.LatheGeometry(prof, 24), new THREE.MeshPhysicalMaterial({color: '#e6eef2', transparent: true, opacity: .34, roughness: .08, side: THREE.DoubleSide, depthWrite: false}));
    const liquid = new THREE.Mesh(new THREE.CylinderGeometry(.043, .04, .12, 20), new THREE.MeshStandardMaterial({color: '#f1f4f6', roughness: .35, transparent: true, opacity: .92})); liquid.position.y = .075;
    // the bottle hangs below the wand by its neck, as on a real foam lance
    const holder = new THREE.Group(); holder.add(shell, liquid); holder.position.set(-length * .62, -.03 - .272, 0);
    const neckBrass = new THREE.Mesh(new THREE.CylinderGeometry(.025, .025, .05, 16), new THREE.MeshStandardMaterial({color: '#b08d3c', metalness: 1, roughness: .3})); neckBrass.position.set(-length * .62, -.012, 0);
    gun.add(holder, neckBrass);
  }
  // the hose, from the back of the gun, down to the floor and away
  const back = new THREE.Vector3(-Math.cos(tilt), Math.sin(tilt), 0).multiplyScalar(length + .3);
  const f = -floor;
  const curve = new THREE.CatmullRomCurve3([back, back.clone().add(new THREE.Vector3(-.2, -.1, -.08)), new THREE.Vector3(back.x - .55, f * .55, -.26), new THREE.Vector3(back.x - 1.0, f + .06, -.7), new THREE.Vector3(back.x - 1.9, f + .02, -1.4), new THREE.Vector3(back.x - 3.4, f + .02, -2.2)]);
  const hose = new THREE.Mesh(new THREE.TubeGeometry(curve, 48, .011, 10), rubber); hose.castShadow = true; root.add(hose);
  return {root, gun};
}
const FAN = (() => {
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.BufferAttribute(new Float32Array([0, 0, 0, 1, 0, -.5, 1, 0, .5]), 3));
  g.setAttribute('uv', new THREE.BufferAttribute(new Float32Array([0, .5, 1, 0, 1, 1]), 2)); return g;
})();
function streakTexture() {
  const [c, x] = canvas(256, 128);
  for (let n = 0; n < 70; n++) { x.fillStyle = `rgba(255,255,255,${.25 + Math.random() * .6})`; x.fillRect(Math.random() * 256, Math.random() * 128, 30 + Math.random() * 110, 1 + Math.random() * 2.4); }
  const t = new THREE.CanvasTexture(c); t.wrapS = t.wrapT = THREE.RepeatWrapping; t.colorSpace = THREE.SRGBColorSpace; return t;
}
function fadeTexture() {
  const [c, x] = canvas(128, 4); const g = x.createLinearGradient(0, 0, 128, 0);
  g.addColorStop(0, '#080808'); g.addColorStop(.18, '#d8d8d8'); g.addColorStop(.8, '#bcbcbc'); g.addColorStop(1, '#303030'); x.fillStyle = g; x.fillRect(0, 0, 128, 4);
  return new THREE.CanvasTexture(c);
}
function suedeTexture() {
  const [c, x] = canvas(128, 128); x.fillStyle = '#464b53'; x.fillRect(0, 0, 128, 128);
  for (let n = 0; n < 3200; n++) { const g = 52 + Math.random() * 40 | 0; x.fillStyle = `rgba(${g},${g + 4},${g + 12},.5)`; x.fillRect(Math.random() * 128, Math.random() * 128, 1, 2); }
  const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; return t;
}

const WASH_TILT = .45, FOAM_TILT = .25, WASH_TIP = new THREE.Vector3(-1.75, 1.2, 0), FOAM_TIP = new THREE.Vector3(-2.0, 1.4, 0);
let foamClock = 0, lastT = 0, foamFront = 0, beadGlints, mistPts, dropPts, foamPts, splashPts, fanMat, streakTex, gridH;
function buildEffects(box) {
  const top = box.max.y;
  const mistTex = puffTexture('mist'), foamTex = puffTexture('foam'), dropTex = puffTexture('drop');
  // pressure washer: gun, wand, hose, a fan of water and the mist it throws back off the paint
  const washer = washerRig({accent: '#d9a900', length: .8, tilt: WASH_TILT, floor: WASH_TIP.y});
  spray = washer.root; body.add(spray);
  streakTex = streakTexture();
  fanMat = new THREE.MeshBasicMaterial({map: streakTex, alphaMap: fadeTexture(), color: '#dff2fb', transparent: true, opacity: .5, side: THREE.DoubleSide, depthWrite: false});
  for (let k = 0; k < 3; k++) { const fan = new THREE.Mesh(FAN, fanMat); fan.scale.set(.92, 1, .5); fan.rotation.x = k * Math.PI / 3; washer.gun.add(fan); }
  mistPts = particles(260, mistTex, '#e8f3f8'); dropPts = particles(180, dropTex, '#f2fbff'); body.add(mistPts, dropPts);
  // foam lance: walks along the car, throwing a thick fan of foam onto the side
  const lance = washerRig({accent: '#b3121d', length: .85, tilt: FOAM_TILT, bottle: true, floor: FOAM_TIP.y});
  cannonGun = lance.root; body.add(cannonGun);
  cannon = particles(520, foamTex, '#eef2f4'); body.add(cannon);
  // ceramic applicator: a suede-wrapped foam block with a hard hand grip
  pad = new THREE.Group();
  const suede = new THREE.MeshStandardMaterial({map: suedeTexture(), roughness: .95}), hard = new THREE.MeshStandardMaterial({color: '#15171a', roughness: .5, metalness: .1}), foam = new THREE.MeshStandardMaterial({color: '#cfd3d6', roughness: .9});
  const nitrile = new THREE.MeshStandardMaterial({color: '#20384f', roughness: .32, metalness: 0}), cloth = new THREE.MeshStandardMaterial({color: '#23272d', roughness: .92});
  const sole = new THREE.Mesh(roundedBox(.125, .03, .08, .012), suede), cap = new THREE.Mesh(roundedBox(.105, .014, .066, .006), hard);
  sole.position.y = .015; cap.position.y = .037;
  const palm = new THREE.Mesh(roundedBox(.075, .03, .075, .013), nitrile); palm.position.set(0, .058, -.005);
  const fingers = []; for (let i = 0; i < 4; i++) { const f = new THREE.Mesh(new THREE.CapsuleGeometry(.0095, .05 - Math.abs(i - 1.5) * .004, 4, 8), nitrile); f.rotation.x = Math.PI / 2; f.position.set(-.027 + i * .018, .052, .048); fingers.push(f); }
  const thumb = new THREE.Mesh(new THREE.CapsuleGeometry(.011, .04, 4, 8), nitrile); thumb.rotation.set(Math.PI / 2, 0, -.5); thumb.position.set(-.05, .054, .018);
  const armDir = new THREE.Vector3(-.5, .5, .7).normalize(), arm = new THREE.Mesh(new THREE.CylinderGeometry(.033, .04, .7, 16), cloth); arm.quaternion.setFromUnitVectors(UP, armDir); arm.position.set(0, .075, -.02).addScaledVector(armDir, .35);
  const wet = new THREE.Mesh(new THREE.PlaneGeometry(.12, .075), new THREE.MeshBasicMaterial({color: '#cfe3ee', transparent: true, opacity: .16, depthWrite: false})); wet.rotation.x = -Math.PI / 2; wet.position.y = -.004;
  pad.add(sole, cap, palm, ...fingers, thumb, arm, wet); pad.children.forEach((c) => { c.castShadow = true; }); body.add(pad);
  // a coarse height map of the top of the body: the applicator rides on it and the water beads sit on it
  const ray = new THREE.Raycaster(), targets = [];
  body.updateMatrixWorld(true);
  body.traverse((n) => { if (n.isMesh && n.material === paintMat) targets.push(n); });
  const GX = 19, GZ = 61, STEP = .1, X0 = -.9, Z0 = -3;
  gridH = new Float32Array(GX * GZ);
  for (let a = 0; a < GX; a++) for (let b = 0; b < GZ; b++) {
    ray.set(new THREE.Vector3(X0 + a * STEP, top + 2, Z0 + b * STEP), new THREE.Vector3(0, -1, 0));
    gridH[a * GZ + b] = ray.intersectObjects(targets, false)[0]?.point.y ?? 0;
  }
  padHeight = (x, z) => {
    const fx = THREE.MathUtils.clamp((x - X0) / STEP, 0, GX - 1.001), fz = THREE.MathUtils.clamp((z - Z0) / STEP, 0, GZ - 1.001), a = Math.floor(fx), b = Math.floor(fz), u = fx - a, v = fz - b;
    return mix(mix(gridH[a * GZ + b], gridH[(a + 1) * GZ + b], u), mix(gridH[a * GZ + b + 1], gridH[(a + 1) * GZ + b + 1], u), v);
  };
  // droplet test: hundreds of beads of every size on the bonnet and roof, flat lenses with a bright edge
  const pts = [];
  for (let i = 0; i < 1600 && pts.length < 720; i++) {
    const x = (Math.random() - .5) * 1.7, z = .1 + Math.random() * 2.3, y = padHeight(x, z);
    if (y < top - .8) continue;
    const dx = padHeight(x + .05, z) - padHeight(x - .05, z), dz = padHeight(x, z + .05) - padHeight(x, z - .05);
    pts.push({p: new THREE.Vector3(x, y, z), n: new THREE.Vector3(-dx, .1, -dz).normalize(), s: .0022 + Math.random() ** 3 * .013, ph: Math.random()});
  }
  beadsMesh = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 20, 14), new THREE.MeshPhysicalMaterial({color: '#0b121a', roughness: .02, metalness: 0, ior: 1.33, envMapIntensity: 7, specularIntensity: 1, clearcoat: 1, clearcoatRoughness: 0}), Math.max(1, pts.length));
  beadsMesh.userData.pts = pts; body.add(beadsMesh);
  beadGlints = particles(Math.max(1, pts.length), dropTex, '#ffffff'); body.add(beadGlints);
  pourMesh = new THREE.Group();
  const streamMat = new THREE.MeshPhysicalMaterial({color: '#e8f4fa', transparent: true, opacity: 0, roughness: 0, clearcoat: 1, envMapIntensity: 4, depthWrite: false});
  for (const sx of [-.14, .04, .2]) { const st = new THREE.Mesh(new THREE.CylinderGeometry(.007, .011, 1, 14), streamMat); st.userData.x = sx; pourMesh.add(st); }
  pourMesh.userData.mat = streamMat; body.add(pourMesh);
  splashPts = particles(120, dropTex, '#f4fbff'); body.add(splashPts);
}
const dummy = new THREE.Object3D();
function stepEffects(k, t) {
  const dtE = Math.min(.1, Math.max(0, t - lastT)); lastT = t;
  if (k.cannon > .3) foamClock += dtE; else if (k.cannon < .05 && k.foam < .05) foamClock = 0;
  const target = k.cannon > .3 ? clamp01(foamClock / 8) : (k.foam > .3 ? 1 : 0);
  foamFront = target > foamFront ? Math.min(target, foamFront + dtE * (k.cannon > .3 ? 1 : 3)) : target;
  const sweep = foamClock < 8 ? mix(-3.4, 3.5, ease(foamFront)) : mix(3.5, Math.sin(t * .45) * 2.4, clamp01((foamClock - 8) / 1.5));
  U.foamZ.value = k.cannon > .3 ? sweep : mix(-3.4, 3.5, ease(foamFront));
  // pressure washer
  spray.visible = k.spray > .02; mistPts.visible = dropPts.visible = spray.visible;
  if (spray.visible) {
    const z = mix(3.0, -3.0, k.rinse);
    spray.position.set(WASH_TIP.x, WASH_TIP.y, z);
    streakTex.offset.x = -(t * 2.2 % 1);
    fanMat.opacity = .85 * clamp01(k.spray) * (.88 + Math.sin(t * 40) * .06);
    mistPts.userData.q.forEach((q, i) => { const life = (t * 1.1 + q[0]) % 1, s = q[1] - .5; putParticle(mistPts, i, -1.0 - life * .45, .55 + q[2] * .8 + life * .35, z + s * .9, .05 + life * .2, (1 - life) * (1 - life) * .55 * k.spray); });
    dropPts.userData.q.forEach((q, i) => { const life = (t * 2.2 + q[0]) % 1, s = q[1] - .5; putParticle(dropPts, i, -.97 - life * .7, .92 + (q[2] - .3) * .5 - life * life * .6 + life * .15, z + s * .85 * (.5 + life), .012, (1 - life) * .9 * k.spray); });
    flushParticles(mistPts); flushParticles(dropPts);
  }
  // foam lance
  cannon.visible = cannonGun.visible = k.cannon > .02;
  if (cannon.visible) {
    const gz = U.foamZ.value - .15; // the nozzle leads the foam from the rear of the car to the nose, then keeps working
    cannonGun.position.set(FOAM_TIP.x, FOAM_TIP.y, gz);
    cannon.userData.q.forEach((q, i) => {
      const life = (t * 1.25 + q[0]) % 1, spread = (q[1] - .5) * (.15 + life * .85);
      putParticle(cannon, i, FOAM_TIP.x + .1 + life * 1.0, FOAM_TIP.y - life * .15 - life * life * .55 + (q[2] - .5) * life * .3, gz + spread, (.04 + life * .11) * k.cannon, (1 - life * life) * .7 * k.cannon);
    });
    flushParticles(cannon);
  }
  pad.visible = k.pad > .02 && k.coat < .995;
  if (pad.visible) { const x = Math.sin(t * 3.2) * .65, z = mix(3.0, -3.0, k.coat) + .05; pad.position.set(x, padHeight(x, z) + .012, z); pad.rotation.y = Math.sin(t * 3.2) * .4; }
  const bt = k.beads;
  beadsMesh.visible = beadGlints.visible = bt > .02;
  if (beadsMesh.visible) {
    beadsMesh.userData.pts.forEach((b, i) => { const slide = b.s > .008 ? ((t * .03 + b.ph) % 1) * .05 * bt : 0; dummy.position.copy(b.p).addScaledVector(b.n, b.s * .25); dummy.position.z -= slide * .5; dummy.quaternion.setFromUnitVectors(UP, b.n); dummy.scale.set(b.s * bt, b.s * .8 * bt, b.s * (1 + b.ph * .15) * bt); dummy.updateMatrix(); beadsMesh.setMatrixAt(i, dummy.matrix); putParticle(beadGlints, i, dummy.position.x - b.s * .25, dummy.position.y + b.s * .55, dummy.position.z + b.s * .2, b.s * 1.1, .8 * bt); });
    beadsMesh.instanceMatrix.needsUpdate = true; flushParticles(beadGlints);
  }
  pourMesh.visible = splashPts.visible = k.pour > .02; pourMesh.userData.mat.opacity = .42 * k.pour;
  if (pourMesh.visible) {
    pourMesh.children.forEach((st, si) => { const hy = padHeight(st.userData.x, 1.5), h = 3.1 - hy; st.position.set(st.userData.x + Math.sin(t * 9 + si) * .004, hy + h / 2, 1.5); st.scale.set(1 + Math.sin(t * 31 + si * 2) * .12, h, 1); });
    for (let i = 0; i < 120; i++) { const q = splashPts.userData.q[i], si = i % 3, sx = pourMesh.children[si].userData.x, life = (t * 2.2 + q[0]) % 1, hy = padHeight(sx, 1.5); putParticle(splashPts, i, sx + (q[1] - .5) * (.04 + life * .22), hy + .005 + life * (.04 + q[2] * .08) - life * life * .03, 1.5 + (q[2] - .5) * (.04 + life * .2), .011, (1 - life) * .9 * k.pour); }
    flushParticles(splashPts);
  }
  const out = ease(clamp01(k.glassOut)); U.out.value = out;
    // while the panes are out they read as light, untinted glass; refitted they take the chosen shade
  glassMat.opacity = mix(.14 + (1 - k.vlt / 100) * .84, .1, out);
  glassMat.color.copy(GLASS_DARK).lerp(GLASS_LIGHT, out);
  // PPF: the clear film is laid from the roof down; a bright wet line marks the edge being pressed on
  U.film.value = clamp01(k.film);
  filmPieces.forEach((m) => { m.visible = k.film > .01; m.material.opacity = .2 * (1 - k.door * .6); });
}

/* ---------- blend two shots ---------- */
function resolve(key) {
  const k = {...BASE, ...key};
  if (k.vlt === 'pick') k.vlt = home.tint ?? 50;
  return k;
}
function blend(a, b, t) {
  const out = {env: t < .5 ? a.env : b.env};
  for (const name of Object.keys(BASE)) out[name] = mix(a[name], b[name], t);
  out.cam = a.cam.map((v, i) => mix(v, b.cam[i], t)); out.look = a.look.map((v, i) => mix(v, b.look[i], t));
  out.cut = a.env !== b.env ? 1 - Math.abs(t - .5) * 2 : 0; // 0..1..0: dark across the change of place
  if (a.env !== b.env) { const s = t < .5 ? a : b; out.cam = s.cam; out.look = s.look; }
  return out;
}

/* ---------- drag to look around ---------- */
let yaw = 0, yawTarget = 0, drag = null;
function interaction() {
  const el = renderer.domElement; el.style.touchAction = 'pan-y';
  el.addEventListener('pointerdown', (e) => { drag = {x: e.clientX, y: e.clientY, start: yawTarget, id: e.pointerId, moved: false}; });
  el.addEventListener('pointermove', (e) => {
    if (!drag) return; const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
    if (!drag.moved && Math.abs(dy) > Math.abs(dx)) { drag = null; return; }
    if (Math.abs(dx) > 4 && !drag.moved) { drag.moved = true; el.setPointerCapture?.(drag.id); story?.classList.add('dragged'); }
    if (drag.moved) { yawTarget = THREE.MathUtils.clamp(drag.start - dx * .006, -2.6, 2.6); schedule(); }
  });
  for (const ev of ['pointerup', 'pointercancel', 'lostpointercapture']) el.addEventListener(ev, () => { drag = null; });
  window.addEventListener('dapper:step', () => { if (!drag) yawTarget = 0; schedule(); });
}

/* ---------- frame ---------- */
const cam = new THREE.Vector3(), look = new THREE.Vector3();
let entryD = 1, last = 0, clock = 0, running = false, visible = true, entry = 0, offset = 0, roadPos = ENTRY, wheelTurn = 0, first = true, lastEnv = '';

function frame(now) {
  running = false;
  if (!visible || document.hidden) return;
  const dt = Math.min((now - (last || now)) / 1000, .05); last = now; clock += dt; U.time.value = clock;
  const keys = home.keys.length ? home.keys : ['road'];
  const n = keys.length, p = THREE.MathUtils.clamp(home.progress, 0, n - 1), i = Math.floor(p), j = Math.min(i + 1, n - 1);
  const k = blend(resolve(KEY[keys[i]] || KEY.road), resolve(KEY[keys[j]] || KEY.road), ease(p - i));

  const env = k.env;
  if (env !== lastEnv) {
    lastEnv = env; roadSet.visible = env === 'road'; garageSet.visible = env === 'garage';
    scene.environment = env === 'road' ? roadEnv : garageEnv; scene.environmentIntensity = env === 'road' ? 1 : .9;
    scene.background = roadEnv; scene.fog = env === 'road' ? roadFog : garageFog;
    sun.visible = env === 'road'; garageKey.visible = env === 'garage';
    renderer.toneMappingExposure = env === 'road' ? 1.12 : 1.0; first = true;
  }
  if (curtain) curtain.style.opacity = String(Math.min(1, k.cut * 1.7));

  // the car: on the road it drives in and rolls; in the garage it rolls in through the door and stops on the bay
  const prev = car.position.clone();
  if (env === 'road') {
    entry = reduced ? 1 : Math.min(1, entry + dt / 3.2);
    const target = mix(ENTRY, mix(ROAD_FROM, ROAD_TO, clamp01(p * 2)), 1 - (1 - entry) ** 3);
    roadPos = reduced ? target : mix(roadPos, target, 1 - Math.exp(-dt * 4));
    car.rotation.y = Math.PI; car.position.set(ROAD_X, 0, roadPos);
  } else {
    car.rotation.y = 0;
    const d = keys[i] === 'road' ? ease(clamp01((p - i - .5) * 2)) : 1;
    car.position.copy(GARAGE).add(new THREE.Vector3(0, 0, mix(-17, 0, d)));
    entryD = d;
  }
  const moved = car.position.distanceTo(prev);
  if (moved < 2) wheelTurn += moved / WHEEL_R;
  wheels.forEach((w) => { w.rotation.x = wheelTurn; });

  yaw = mix(yaw, yawTarget, reduced ? 1 : 1 - Math.exp(-dt * 6));
  const want = new THREE.Vector3(...k.cam); if (small()) want.multiplyScalar(1.5);
  want.applyAxisAngle(UP, car.rotation.y + yaw).add(car.position);
  const lookAt = new THREE.Vector3(...k.look).applyAxisAngle(UP, car.rotation.y).add(car.position);
  const follow = reduced || first ? 1 : 1 - Math.exp(-dt * 5); first = false;
  cam.lerp(want, follow); look.lerp(lookAt, follow);
  camera.position.copy(cam); camera.lookAt(look);
  const W = stage.clientWidth, H = stage.clientHeight;
  offset = mix(offset, small() ? 0 : -(home.side || 1) * W * .16, reduced ? 1 : 1 - Math.exp(-dt * 4));
  camera.setViewOffset(W, H, offset, small() ? H * .13 : 0, W, H);

  U.dirt.value = k.dirt; U.foam.value = k.foam; U.rinse.value = k.rinse; U.coat.value = k.coat;
  stepEffects(k, clock);
  if (door) {
    // open for the arrival (shuts once the car is inside) and again on the last step
    const arriving = env === 'garage' && keys[i] === 'road' ? 1 - ease(clamp01((entryD - .78) / .22)) : 0;
    const open = Math.max(k.door, arriving);
    door.position.y = DOOR_H / 2 + open * (DOOR_H + .3);
  }
  if (env === 'road') { haze.position.set(car.position.x, 32, car.position.z); sun.target.position.copy(car.position); sun.position.set(car.position.x - 6, 14, car.position.z + 9); }
  renderer.render(scene, camera);
  schedule();
}
function schedule() { if (!running && visible && !document.hidden) { running = true; requestAnimationFrame(frame); } }
function resize() {
  const w = Math.max(stage.clientWidth, 1), h = Math.max(stage.clientHeight, 1);
  renderer.setPixelRatio(Math.min(devicePixelRatio, small() ? 1.25 : 1.6)); renderer.setSize(w, h, false);
  camera.aspect = w / h; camera.fov = small() ? 44 : 32; camera.updateProjectionMatrix();
  const pxScale = h * renderer.getPixelRatio() / (2 * Math.tan(THREE.MathUtils.degToRad(camera.fov) / 2)); particleMats.forEach((m) => { m.uniforms.uScale.value = pxScale; });
}

async function start() {
  try { renderer = new THREE.WebGLRenderer({antialias: true, powerPreference: 'high-performance'}); } catch { story?.setAttribute('data-scene', 'fallback'); return; }
  renderer.outputColorSpace = THREE.SRGBColorSpace; renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = 1.1;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.domElement.setAttribute('aria-hidden', 'true'); stage.append(renderer.domElement);
  scene = new THREE.Scene(); camera = new THREE.PerspectiveCamera(32, 1, .3, 1200); resize();
  sun = new THREE.DirectionalLight('#fff1dc', 2.4); sun.castShadow = true; sun.shadow.mapSize.set(2048, 2048);
  Object.assign(sun.shadow.camera, {left: -4, right: 4, top: 4, bottom: -4, near: .5, far: 40}); sun.shadow.bias = -.0006; sun.shadow.normalBias = .03; scene.add(sun, sun.target);
  try { await Promise.all([buildRoad(), buildGarage(), buildCar()]); } catch (err) { console.warn('3D studio unavailable', err); story?.setAttribute('data-scene', 'fallback'); return; }
  resize();
  story?.setAttribute('data-scene', 'ready');
  interaction();
  new ResizeObserver(resize).observe(stage);
  new IntersectionObserver((e) => { visible = e[0].isIntersecting; schedule(); }).observe(stage);
  document.addEventListener('visibilitychange', schedule);
  window.addEventListener('dapper:home', schedule);
  renderer.domElement.addEventListener('webglcontextlost', (ev) => { ev.preventDefault(); story?.setAttribute('data-scene', 'fallback'); });
  schedule();
}
start();

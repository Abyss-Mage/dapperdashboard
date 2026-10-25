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
import {GroundedSkybox} from 'three/addons/objects/GroundedSkybox.js';
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
let roadSet, garageSet, roadEnv, garageEnv, sun, garageKey, spray, cannon, cannonGun, pad, padHeight, beadsMesh, pourMesh, door;
const U = {dirt: {value: 1}, foam: {value: 0}, rinse: {value: 0}, coat: {value: 0}, film: {value: 0}, time: {value: 0}};
const GLASS_DARK = new THREE.Color('#0a0c0f'), GLASS_LIGHT = new THREE.Color('#a9bfcc');

/* ---------- paint: one shader with road dirt, snow foam, a rinse front, and the ceramic gloss front ---------- */
function paintShader(mat) {
  mat.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, {uDirt: U.dirt, uFoam: U.foam, uRinse: U.rinse, uCoat: U.coat, uTime: U.time});
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', `#include <common>
      varying vec3 vL; uniform float uDirt,uFoam,uRinse,uCoat,uTime;
      float h3(vec3 p){return fract(sin(dot(p,vec3(127.1,311.7,74.7)))*43758.5453);}
      float n3(vec3 p){vec3 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(mix(h3(i),h3(i+vec3(1,0,0)),f.x),mix(h3(i+vec3(0,1,0)),h3(i+vec3(1,1,0)),f.x),f.y),mix(mix(h3(i+vec3(0,0,1)),h3(i+vec3(1,0,1)),f.x),mix(h3(i+vec3(0,1,1)),h3(i+vec3(1,1,1)),f.x),f.y),f.z);}
      float washed(){ return smoothstep(-.25,.25,vL.z-mix(3.0,-3.0,uRinse)); }
      float coated(){ return smoothstep(-.2,.2,vL.z-mix(3.0,-3.0,uCoat)); }
      float dirtM=0.,foamM=0.;`)
      .replace('#include <color_fragment>', `#include <color_fragment>
      float w=washed();
      float g=n3(vL*9.)*.5+n3(vL*37.)*.3+n3(vL*110.)*.2;
      // road grime thickest low down and behind the wheels, plus a thin even dust film over the rest
      float grime=smoothstep(.35,.95,clamp((1.0-vL.y)*1.15+g*.4,0.,1.));
      float dust=.22+.1*g;
      dirtM=max(grime,dust)*uDirt*(1.-w);
      diffuseColor.rgb=mix(diffuseColor.rgb,vec3(.29,.25,.2)*(.8+.4*g),dirtM*.85);
      float bub=n3(vL*60.+vec3(0.,uTime*.15,0.))*.6+n3(vL*14.)*.4;
      float drip=smoothstep(.35,.75,n3(vec3(vL.x*18.,vL.y*2.-uTime*.08,vL.z*18.)));
      foamM=uFoam*(1.-w)*smoothstep(.05,.42,bub+drip*.35+(vL.y-.4)*.25);
      diffuseColor.rgb=mix(diffuseColor.rgb,vec3(.96,.97,.98)*(.86+bub*.14),foamM);`)
      .replace('#include <roughnessmap_fragment>', `#include <roughnessmap_fragment>
      float c=coated();
      roughnessFactor=mix(mix(roughnessFactor,.5,(1.-c)*uCoatOn),.92,max(dirtM,foamM));`)
      .replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
      // a faint cyan line where the coating is being levelled
      totalEmissiveRadiance+=vec3(.22,.88,1.)*(1.-smoothstep(0.,.07,abs(vL.z-mix(3.0,-3.0,uCoat))))*step(.002,uCoat)*step(uCoat,.995)*.5;`)
      .replace('#include <lights_fragment_begin>', `material.clearcoat*=mix(1.-.8*uCoatOn,1.,coated())*(1.-max(dirtM,foamM));\n#include <lights_fragment_begin>`);
    // before the ceramic step the paint is plain; once it starts, the uncoated part reads hazy next to the coated gloss
    sh.fragmentShader = sh.fragmentShader.replace('float dirtM=0.,foamM=0.;', 'float dirtM=0.,foamM=0.,uCoatOn=0.;')
      .replace('float w=washed();', 'float w=washed(); uCoatOn=step(.002,uCoat);');
  };
  mat.customProgramCacheKey = () => 'dapper-paint-v5';
  return mat;
}
/* clear film revealed from the roof down, with a bright squeegee line at the edge */
function filmShader(mat) {
  mat.onBeforeCompile = (sh) => {
    sh.uniforms.uFilm = U.film;
    sh.vertexShader = sh.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 vL;').replace('#include <begin_vertex>', '#include <begin_vertex>\nvL=position;');
    sh.fragmentShader = sh.fragmentShader.replace('#include <common>', '#include <common>\nvarying vec3 vL; uniform float uFilm;')
      .replace('#include <clipping_planes_fragment>', '#include <clipping_planes_fragment>\nfloat lvl=mix(2.1,-.1,uFilm); if(vL.y<lvl) discard; float band=(1.-smoothstep(0.,.045,vL.y-lvl))*step(uFilm,.995);')
      .replace('#include <dithering_fragment>', '#include <dithering_fragment>\ngl_FragColor.rgb+=vec3(.22,.88,1.)*band*1.6; gl_FragColor.a=max(gl_FragColor.a,band*.85);');
  };
  mat.customProgramCacheKey = () => 'dapper-film-v1';
  return mat;
}

/* ---------- road scene: CC0 coastal road HDR, projected onto the ground ---------- */
async function buildRoad() {
  const tex = await new HDRLoader().loadAsync(new URL('./assets/env/red_hill_curve_2k.hdr', import.meta.url).href);
  tex.mapping = THREE.EquirectangularReflectionMapping;
  roadEnv = tex;
  roadSet = new THREE.Group(); scene.add(roadSet);
  const sky = new GroundedSkybox(tex, 2.2, 60); sky.position.y = 2.2 - .01; roadSet.add(sky);
  const catcher = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({opacity: .55}));
  catcher.rotation.x = -Math.PI / 2; catcher.position.y = .002; catcher.receiveShadow = true; roadSet.add(catcher);
}

/* ---------- the Dapper garage: modelled on the real shop from the owner's photos ----------
   black painted block walls, the hexagon LED clusters on the ceiling and the slat wall, warm vertical tubes and white
   fluorescents, ribbed interlocking floor tiles with three red bays, the plywood loft with its black sheeting, shelves of
   chemical jugs and spray bottles, the folding workbench, shop vacs, a generator, and the cream roller door with its three
   small windows. The car drives in through that door. Everything is built here (no images of the shop are used). */
const HALF_X = 7, HALF_Z = 11, HEIGHT = 5.2, DOOR_W = 5.2, DOOR_H = 4.2;

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
    const g = x.createLinearGradient(0, 418, 0, 484); g.addColorStop(0, '#5d6670'); g.addColorStop(.5, '#1a1d22'); g.addColorStop(1, '#3a4048'); x.fillStyle = g; x.fillRect(cx - 82, 420, 164, 62);
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
const tubeGeo = new THREE.BoxGeometry(1, .075, .075);
function tubeBetween(group, mat, A, B) {
  const m = new THREE.Mesh(tubeGeo, mat), d = B.clone().sub(A);
  m.position.copy(A).add(B).multiplyScalar(.5); m.scale.x = d.length(); m.quaternion.setFromUnitVectors(new THREE.Vector3(1, 0, 0), d.normalize()); group.add(m);
}
function hexRig(group, mat, cells, r, place) { for (const [a, b] of hexSegments(cells, r)) tubeBetween(group, mat, place(a[0], a[1]), place(b[0], b[1])); }
const fixture = (group, matTube, x, y, z, ry, len = 1.25) => {
  const g = new THREE.Group(), body = new THREE.Mesh(new THREE.BoxGeometry(len, .09, .3), new THREE.MeshStandardMaterial({color: '#b9bcc0', roughness: .6}));
  g.add(body); for (const dz of [-.07, .07]) { const t = new THREE.Mesh(new THREE.CylinderGeometry(.035, .035, len - .08, 8), matTube); t.rotation.z = Math.PI / 2; t.position.set(0, -.07, dz); g.add(t); }
  g.position.set(x, y, z); g.rotation.y = ry; group.add(g);
};

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
  const floor = new THREE.Mesh(new THREE.PlaneGeometry(HALF_X * 2, HALF_Z * 2), new THREE.MeshStandardMaterial({map: floorTex, roughness: .5, metalness: .12}));
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

  // outside the door: daylight and a concrete apron, so the car visibly arrives from outside
  const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 30), new THREE.MeshStandardMaterial({color: '#8b8e92', roughness: .95})); ground.rotation.x = -Math.PI / 2; ground.position.set(0, -.005, -HALF_Z - 15); ground.receiveShadow = true; garageSet.add(ground);
  const sky = new THREE.Mesh(new THREE.PlaneGeometry(90, 40), new THREE.MeshBasicMaterial({color: new THREE.Color(1.1, 1.2, 1.35), toneMapped: false})); sky.position.set(0, 12, -HALF_Z - 28); garageSet.add(sky);

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
  const box = (w, h, d, m, x, y, z, parent = garageSet) => { const o = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), m); o.position.set(x, y, z); o.castShadow = o.receiveShadow = true; parent.add(o); return o; };
  const liquids = ['#8c4fb5', '#e7ecef', '#f1eee2', '#d7dadc', '#c64a5d', '#4aa3d8', '#e8a23a', '#3fb58a'];
  const jug = (x, y, z, i) => { box(.2, .3, .17, std('#e9edf0', .35), x, y + .15, z); box(.205, .13, .175, std(liquids[i % liquids.length], .3), x, y + .07, z); box(.07, .04, .07, std('#d6d9db', .5), x - .04, y + .32, z); };
  const spray = (x, y, z, i) => { box(.09, .24, .09, std('#eceff0', .4), x, y + .12, z); box(.1, .1, .1, std(liquids[(i * 3) % liquids.length], .35), x, y + .06, z); box(.06, .06, .11, std(i % 2 ? '#e6c200' : '#111214', .4), x, y + .27, z + .02); };
  const wood = std('#5b3d26', .75);
  for (const y of [1.7, 2.4]) box(.34, .045, 4.4, wood, HALF_X - .22, y, -3.5);
  for (let i = 0; i < 13; i++) jug(HALF_X - .22, 2.43, -5.5 + i * .33, i);
  for (let i = 0; i < 9; i++) { if (i % 3 === 0) jug(HALF_X - .22, 1.73, -5.4 + i * .45, i + 2); else spray(HALF_X - .22, 1.73, -5.4 + i * .45, i); }
  // the folding workbench with its row of bottles and the bins under it
  box(.82, .05, 3.2, std('#e6e3dc', .5), HALF_X - .75, .86, -1.2);
  for (const [dx, dz] of [[-.36, -1.45], [.36, -1.45], [-.36, 1.45], [.36, 1.45]]) box(.04, .84, .04, std('#9aa0a6', .4, .6), HALF_X - .75 + dx, .42, -1.2 + dz);
  for (let i = 0; i < 18; i++) spray(HALF_X - .45, .885, -2.6 + i * .17, i);
  for (let i = 0; i < 3; i++) { box(.5, .32, .62, std('#16171a', .6), HALF_X - 1.0, .2, -2.2 + i * .75); box(.5, .05, .62, std('#d9b300', .6), HALF_X - 1.0, .385, -2.2 + i * .75); }
  // shop vacs, generator
  const vac = (x, z) => { const g = new THREE.Group(); const body = new THREE.Mesh(new THREE.CylinderGeometry(.28, .25, .52, 18), std('#e08a0b', .5)); body.position.y = .27; const lid = new THREE.Mesh(new THREE.CylinderGeometry(.29, .29, .08, 18), std('#17181a', .5)); lid.position.y = .56; const hose = new THREE.Mesh(new THREE.TorusGeometry(.3, .03, 8, 24, Math.PI * 1.2), std('#101012', .7)); hose.position.set(.35, .45, 0); hose.rotation.y = Math.PI / 2; g.add(body, lid, hose); g.position.set(x, 0, z); g.traverse((o) => { o.castShadow = true; }); garageSet.add(g); };
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
  glassMat = new THREE.MeshPhysicalMaterial({color: '#0a0c0f', metalness: .1, roughness: .03, transparent: true, opacity: .3, clearcoat: 1, side: THREE.DoubleSide, depthWrite: false});
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
    const name = n.material?.name || n.name, g = geomOf(n);
    if (name === 'WindowsTint') {
      // the windows: the windscreen stays, the side and rear glass lift out for tinting
      const pieces = splitBy(g, (v) => (Math.abs(v.x) > half * .62 ? (v.x > 0 ? 1 : 2) : v.z < 0 ? 3 : 0), 4);
      const dirs = [null, new THREE.Vector3(1, .55, 0), new THREE.Vector3(-1, .55, 0), new THREE.Vector3(0, .5, -1)];
      pieces.forEach((pg, i) => { if (!pg) return; const m = new THREE.Mesh(pg, glassMat); m.renderOrder = 2; body.add(m); if (i) glassPieces.push({mesh: m, dir: dirs[i].normalize()}); });
      continue;
    }
    const mat = name === 'Polar_White' ? paintMat : look[name] || (/Color_M0[12]/.test(name) ? chrome : grey('#' + (n.material?.color?.getHexString?.() || '777777')));
    if (wheelInfo.length && name !== 'Polar_White' && /Tyres|Color_M|Grille|UnderCarriage|material/.test(name)) {
      // wheel parts (tyre, rim, brake) go into their own spinning pivots
      const parts = splitBy(g, (v) => wheelOf(v) + 1, wheelInfo.length + 1);
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
    w.pieces.forEach((p) => { p.g.translate(-w.c.x, -w.c.y, -w.c.z); const m = new THREE.Mesh(p.g, p.mat); m.castShadow = true; pivot.add(m); });
    wheels.push(pivot);
  }
  buildEffects(bodyBox);
}

/* ---------- tools and water ---------- */
function buildEffects(box) {
  const top = box.max.y;
  // pressure washer lance: a nozzle and a fan of water, moving along the car with the rinse front
  spray = new THREE.Group();
  const lance = new THREE.Mesh(new THREE.CylinderGeometry(.02, .02, 1.1, 8), new THREE.MeshStandardMaterial({color: '#202328', metalness: .8, roughness: .3}));
  lance.rotation.z = Math.PI / 2.6; lance.position.set(-1.9, 1.5, 0); spray.add(lance);
  const jetMat = new THREE.MeshBasicMaterial({color: '#d9f1ff', transparent: true, opacity: .55, depthWrite: false});
  for (let i = 0; i < 18; i++) { const jet = new THREE.Mesh(new THREE.CylinderGeometry(.004, .012, 1.4, 5), jetMat); jet.userData.i = i; spray.add(jet); }
  const mist = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 6, 5), new THREE.MeshBasicMaterial({color: '#eaf7ff', transparent: true, opacity: .35, depthWrite: false}), 260);
  mist.userData.p = Array.from({length: 260}, () => [Math.random(), Math.random(), Math.random()]); spray.add(mist); spray.userData.mist = mist;
  body.add(spray);
  // foam cannon: a lance that walks along the car, throwing a fine fan of foam onto the side
  cannonGun = lance.clone(); cannonGun.material = new THREE.MeshStandardMaterial({color: '#b3121d', metalness: .4, roughness: .4}); body.add(cannonGun);
  cannon = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 6, 5), new THREE.MeshStandardMaterial({color: '#f3f5f7', roughness: .95, transparent: true, opacity: .8}), 600);
  cannon.userData.p = Array.from({length: 600}, () => [Math.random(), Math.random(), Math.random()]); body.add(cannon);
  // ceramic applicator pad
  pad = new THREE.Mesh(new THREE.BoxGeometry(.16, .05, .1), new THREE.MeshStandardMaterial({color: '#1d4f7a', roughness: .6}));
  body.add(pad);
  // droplet test: beads on the bonnet and a stream poured over it
  const ray = new THREE.Raycaster(), targets = [], pts = [];
  body.updateMatrixWorld(true);
  body.traverse((n) => { if (n.isMesh && n.material === paintMat) targets.push(n); });
  for (let i = 0; i < 900 && pts.length < 260; i++) {
    const x = (Math.random() - .5) * 1.5, z = 1.15 + Math.random() * 1.15;
    ray.set(new THREE.Vector3(x, top + 2, z), new THREE.Vector3(0, -1, 0)); const hit = ray.intersectObjects(targets, false)[0];
    if (hit && hit.point.y > top - .75) pts.push({p: hit.point, n: hit.face.normal.clone().transformDirection(hit.object.matrixWorld), s: .006 + Math.random() ** 2 * .016, ph: Math.random()});
  }
  beadsMesh = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 12, 10), new THREE.MeshPhysicalMaterial({color: '#f4fbff', roughness: .02, transmission: .96, thickness: .02, ior: 1.333, clearcoat: 1, transparent: true, opacity: 0}), Math.max(1, pts.length));
  beadsMesh.userData.pts = pts; body.add(beadsMesh);
  pourMesh = new THREE.InstancedMesh(new THREE.CylinderGeometry(.003, .003, .12, 4), new THREE.MeshBasicMaterial({color: '#dff1ff', transparent: true, opacity: 0, depthWrite: false}), 140);
  pourMesh.userData.p = Array.from({length: 140}, () => [(Math.random() - .5) * 1.3, 1.2 + Math.random() * 1.1, Math.random()]); body.add(pourMesh);
  // a coarse height map of the top of the body, so the applicator pad rides on the paint
  const GX = 13, GZ = 41, hs = new Float32Array(GX * GZ);
  for (let a = 0; a < GX; a++) for (let b = 0; b < GZ; b++) {
    ray.set(new THREE.Vector3(-.9 + a * .15, top + 2, -3 + b * .15), new THREE.Vector3(0, -1, 0));
    hs[a * GZ + b] = ray.intersectObjects(targets, false)[0]?.point.y ?? 0;
  }
  padHeight = (x, z) => {
    const fx = THREE.MathUtils.clamp((x + .9) / .15, 0, GX - 1.001), fz = THREE.MathUtils.clamp((z + 3) / .15, 0, GZ - 1.001), a = Math.floor(fx), b = Math.floor(fz), u = fx - a, v = fz - b;
    return mix(mix(hs[a * GZ + b], hs[(a + 1) * GZ + b], u), mix(hs[a * GZ + b + 1], hs[(a + 1) * GZ + b + 1], u), v);
  };
}
const dummy = new THREE.Object3D();
function stepEffects(k, t) {
  spray.visible = k.spray > .02;
  if (spray.visible) {
    spray.position.set(0, 0, mix(3.0, -3.0, k.rinse));
    spray.children.forEach((c) => { if (c.userData.i === undefined) return; const a = (c.userData.i / 17 - .5) * .9; c.position.set(-1.35, 1.25 + Math.sin(a) * .25, Math.sin(t * 30 + c.userData.i) * .01); c.rotation.set(a, 0, Math.PI / 2 + .35); });
    const mist = spray.userData.mist;
    mist.userData.p.forEach((q, i) => { const life = (t * 1.4 + q[0]) % 1; dummy.position.set(-1.0 + q[1] * .3 + life * .2, .3 + q[2] * 1.4 + life * .2, (q[0] - .5) * .9); dummy.scale.setScalar(.006 + life * .03); dummy.quaternion.identity(); dummy.updateMatrix(); mist.setMatrixAt(i, dummy.matrix); });
    mist.instanceMatrix.needsUpdate = true;
  }
  cannon.visible = cannonGun.visible = k.cannon > .02;
  if (cannon.visible) {
    const gz = Math.sin(t * .45) * 2.1; // the nozzle walks front to back and back again
    cannonGun.position.set(-2.45, 1.38, gz); cannonGun.rotation.set(0, 0, Math.PI / 2 - .25);
    cannon.userData.p.forEach((q, i) => { const life = (t * 1.3 + q[0]) % 1, z = gz + (q[1] - .5) * life * .9; dummy.position.set(-1.85 + life * .85, 1.32 - life * life * .55 + (q[2] - .5) * life * .5, z); dummy.scale.setScalar((.004 + life * .012) * k.cannon); dummy.quaternion.identity(); dummy.updateMatrix(); cannon.setMatrixAt(i, dummy.matrix); });
    cannon.instanceMatrix.needsUpdate = true;
  }
  pad.visible = k.pad > .02 && k.coat < .995;
  if (pad.visible) { const x = Math.sin(t * 3.2) * .65, z = mix(3.0, -3.0, k.coat) + .05; pad.position.set(x, padHeight(x, z) + .03, z); pad.rotation.y = Math.sin(t * 3.2) * .4; }
  const bt = k.beads;
  beadsMesh.material.opacity = bt; beadsMesh.visible = bt > .02;
  if (beadsMesh.visible) {
    beadsMesh.userData.pts.forEach((b, i) => { const slide = ((t * .05 + b.ph) % 1) * .12 * bt; dummy.position.copy(b.p).addScaledVector(b.n, b.s * .4); dummy.position.x += Math.sign(b.p.x || 1) * slide; dummy.quaternion.setFromUnitVectors(UP, b.n); dummy.scale.set(b.s, b.s * .6, b.s); dummy.updateMatrix(); beadsMesh.setMatrixAt(i, dummy.matrix); });
    beadsMesh.instanceMatrix.needsUpdate = true;
  }
  pourMesh.material.opacity = .5 * k.pour; pourMesh.visible = k.pour > .02;
  if (pourMesh.visible) { pourMesh.userData.p.forEach((q, i) => { dummy.position.set(q[0], 3.2 - ((t * 2.4 + q[2] * 2) % 2), q[1]); dummy.scale.setScalar(1); dummy.quaternion.identity(); dummy.updateMatrix(); pourMesh.setMatrixAt(i, dummy.matrix); }); pourMesh.instanceMatrix.needsUpdate = true; }
  const out = ease(clamp01(k.glassOut));
  glassPieces.forEach((g) => g.mesh.position.copy(g.dir).multiplyScalar(out * .7));
  // while the panes are out they read as light, untinted glass; refitted they take the chosen shade
  glassMat.opacity = mix(.14 + (1 - k.vlt / 100) * .84, .4, out);
  glassMat.color.copy(GLASS_DARK).lerp(GLASS_LIGHT, out);
  // PPF: the clear film is laid from the roof down; a cyan squeegee line marks the edge being pressed on
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
  if (env === 'road') { sun.target.position.copy(car.position); sun.position.set(car.position.x - 6, 14, car.position.z + 9); }
  renderer.render(scene, camera);
  schedule();
}
function schedule() { if (!running && visible && !document.hidden) { running = true; requestAnimationFrame(frame); } }
function resize() {
  const w = Math.max(stage.clientWidth, 1), h = Math.max(stage.clientHeight, 1);
  renderer.setPixelRatio(Math.min(devicePixelRatio, small() ? 1.25 : 1.6)); renderer.setSize(w, h, false);
  camera.aspect = w / h; camera.fov = small() ? 44 : 32; camera.updateProjectionMatrix();
}

async function start() {
  try { renderer = new THREE.WebGLRenderer({antialias: true, powerPreference: 'high-performance'}); } catch { story?.setAttribute('data-scene', 'fallback'); return; }
  renderer.outputColorSpace = THREE.SRGBColorSpace; renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = 1.1;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.domElement.setAttribute('aria-hidden', 'true'); stage.append(renderer.domElement);
  scene = new THREE.Scene(); camera = new THREE.PerspectiveCamera(32, 1, .05, 400); resize();
  sun = new THREE.DirectionalLight('#fff1dc', 2.4); sun.castShadow = true; sun.shadow.mapSize.set(2048, 2048);
  Object.assign(sun.shadow.camera, {left: -4, right: 4, top: 4, bottom: -4, near: .5, far: 40}); sun.shadow.bias = -.0006; sun.shadow.normalBias = .03; scene.add(sun, sun.target);
  try { await Promise.all([buildRoad(), buildGarage(), buildCar()]); } catch (err) { console.warn('3D studio unavailable', err); story?.setAttribute('data-scene', 'fallback'); return; }
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

import * as THREE from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from './assets/three/meshopt_decoder.module.js';

const stage=document.querySelector('#three-stage'), experience=document.querySelector('#experience'), status=document.querySelector('#scene-status');
const mobile=()=>stage.clientWidth<700;
const s=()=>window.dapperStudio||{service:'detailing',amount:1,tint:20,water:true,film:true,view:'front',orbit:false,unlocked:false,cabin:'door',motion:true};
let renderer,scene,camera,car,door,env,cabinLamp,frame=0,frames=0,visible=true,ready=false,lost=false,last=0,rotation=0;
let meshes=[],paints=[],glass=[],hood=[],filmSurfaces=[],tyres=[],doorAngle=0;
let rainMesh,rainData=[],rainTime=0,beadMesh,beadData=[],filmGroup,filmMaterial,filmEdges=[],headlights=[];
let scroll={value:0},actualCamera=new THREE.Vector3(6,2.6,7.4),actualLook=new THREE.Vector3(0,.6,0),pointer=new THREE.Vector2(),smoothPointer=new THREE.Vector2();
const materials=new Set(),geometries=new Set();
const dirtUniform={value:1};
const diagnostics={};Object.defineProperties(diagnostics,{state:{get:()=>experience.dataset.scene},frames:{get:()=>frames},service:{get:()=>s().service},dirt:{get:()=>dirtUniform.value},doorAngle:{get:()=>doorAngle},beads:{get:()=>beadMesh?.visible?beadMesh.count:0},film:{get:()=>filmGroup?.visible},glassOpacity:{get:()=>glass[0]?.opacity},camera:{get:()=>camera?.position.toArray()},meshes:{get:()=>meshes.length},tyres:{get:()=>tyres.length},rain:{get:()=>rainMesh?.visible?rainMesh.count:0},rainTime:{get:()=>rainTime}});window.dapperScene=Object.freeze(diagnostics);
const smooth=(a,b,t)=>THREE.MathUtils.lerp(a,b,t);
function shaderDirt(mat){
 mat.onBeforeCompile=shader=>{shader.uniforms.dapperDirt=dirtUniform;
 shader.vertexShader=shader.vertexShader.replace('#include <common>','#include <common>\nvarying vec3 vDapper;').replace('#include <begin_vertex>','#include <begin_vertex>\nvDapper=(modelMatrix*vec4(position,1.0)).xyz;');
 shader.fragmentShader=shader.fragmentShader.replace('#include <common>',`#include <common>
 varying vec3 vDapper; uniform float dapperDirt;
 float hashD(vec3 p){return fract(sin(dot(p,vec3(127.1,311.7,74.7)))*43758.5453);}
 float noiseD(vec3 p){vec3 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);return mix(mix(mix(hashD(i),hashD(i+vec3(1,0,0)),f.x),mix(hashD(i+vec3(0,1,0)),hashD(i+vec3(1,1,0)),f.x),f.y),mix(mix(hashD(i+vec3(0,0,1)),hashD(i+vec3(1,0,1)),f.x),mix(hashD(i+vec3(0,1,1)),hashD(i+vec3(1,1,1)),f.x),f.y),f.z);}`);
 shader.fragmentShader=shader.fragmentShader.replace('#include <color_fragment>',`#include <color_fragment>
 float grimeNoise=noiseD(vDapper*13.)*.5+noiseD(vDapper*52.)*.3+noiseD(vDapper*130.)*.2;
 float roadDirt=clamp((1.35-vDapper.y)*.6+grimeNoise*.8,.0,1.);
 float dirtMask=smoothstep(.18,.8,roadDirt)*dapperDirt;
 diffuseColor.rgb=mix(diffuseColor.rgb, mix(vec3(.14,.105,.064),vec3(.38,.30,.18),grimeNoise),dirtMask*.93);`);
 shader.fragmentShader=shader.fragmentShader.replace('#include <roughnessmap_fragment>','#include <roughnessmap_fragment>\nroughnessFactor=mix(roughnessFactor,.95,dirtMask);');
 };mat.customProgramCacheKey=()=> 'dapper-dirt-v1';return mat;
}
function garage(){
 scene.background=new THREE.Color('#0e0e11');scene.fog=new THREE.Fog('#0e0e11',14,33);
 // A dark reflection room with tall light cards, matching the reference studio.
 const room=new THREE.Scene();room.add(new THREE.Mesh(new THREE.BoxGeometry(40,20,40),new THREE.MeshBasicMaterial({color:'#151b24',side:THREE.BackSide})));
 const luminous=new THREE.MeshBasicMaterial({color:new THREE.Color(7,6.7,6.1),side:THREE.DoubleSide});
 for(const x of [-5,5])for(const z of [-5,0,5]){const card=new THREE.Mesh(new THREE.PlaneGeometry(.35,4.2),luminous);card.position.set(x,2.8,z);card.rotation.y=x>0?-Math.PI/2:Math.PI/2;room.add(card)}
 for(const z of [-3,2]){const card=new THREE.Mesh(new THREE.PlaneGeometry(6,.8),luminous);card.position.set(0,5,z);card.rotation.x=Math.PI/2;room.add(card)}
 const gen=new THREE.PMREMGenerator(renderer);env=gen.fromScene(room,.035);scene.environment=env.texture;scene.environmentIntensity=.7;room.traverse(n=>{n.geometry?.dispose()});luminous.dispose();gen.dispose();
 const floor=new THREE.Mesh(new THREE.PlaneGeometry(90,90),new THREE.MeshStandardMaterial({color:'#12161b',roughness:.6,metalness:.2}));floor.rotation.x=-Math.PI/2;floor.position.y=-.025;floor.receiveShadow=true;scene.add(floor);
 const floorTexture=garageFloorTexture();const pad=new THREE.Mesh(new THREE.PlaneGeometry(17,20),new THREE.MeshStandardMaterial({map:floorTexture,color:'#717171',roughness:.53,metalness:.22,bumpMap:floorTexture,bumpScale:.012}));pad.rotation.x=-Math.PI/2;pad.position.set(0,-.016,-.8);pad.receiveShadow=true;scene.add(pad);
 const blocks=garageWallTexture();const wallMaterial=new THREE.MeshStandardMaterial({map:blocks,color:'#9ca8b5',roughness:.85,bumpMap:blocks,bumpScale:.035});
 const back=new THREE.Mesh(new THREE.PlaneGeometry(20,6),wallMaterial);back.position.set(0,2.98,-8.5);scene.add(back);
 for(const x of [-9,9]){const wall=new THREE.Mesh(new THREE.PlaneGeometry(22,6),wallMaterial);wall.position.set(x,2.98,1.5);wall.rotation.y=x>0?-Math.PI/2:Math.PI/2;scene.add(wall)}
 const black=new THREE.MeshStandardMaterial({color:'#10151b',roughness:.58,metalness:.4});
 const addBox=(w,h,d,x,y,z,mat=black)=>{const m=new THREE.Mesh(new THREE.BoxGeometry(w,h,d),mat);m.position.set(x,y,z);m.castShadow=true;m.receiveShadow=true;scene.add(m);return m};
 // Structural ribs and ceiling rails, with the rear wall left clear behind the vehicle.
 for(const x of [-8.7,-4.5,0,4.5,8.7])addBox(.13,6,.16,x,3,-8.35);
 for(const z of [-7,-2,3,8])addBox(18,.14,.14,0,5.6,z);
 const warm=new THREE.MeshBasicMaterial({color:new THREE.Color(3.7,3.5,3.2)});
 for(const side of [-1,1])for(const z of [-6.5,-1,5]){
  addBox(.16,3.4,.18,side*8.75,2.9,z,black);addBox(.07,3.1,.08,side*8.63,2.9,z,warm);
  const light=new THREE.PointLight('#fff2db',12,12,2);light.position.set(side*7.8,3,z);scene.add(light);
 }
 for(const x of [-5.7,5.7]){addBox(.1,3.3,.12,x,2.65,-8.18,warm);const l=new THREE.PointLight('#e2eeff',9,13,2);l.position.set(x,3,-7.5);scene.add(l)}
 // Shelves and detailing bottles on the right, echoing the customer's work area.
 const shelfMat=new THREE.MeshStandardMaterial({color:'#252c33',roughness:.4,metalness:.5});
 for(const y of [1.5,2.4,3.35]){
  addBox(3.2,.07,.65,3.8,y,-6.95,shelfMat);
  for(let i=0;i<8;i++){
   const colours=['#bac9cd','#c0d6e4','#57849e','#d0d4be','#d59a24','#8aafbd','#c9cfce','#1a466d'];
   const bottle=new THREE.Mesh(new THREE.CylinderGeometry(.065,.078,.30+(i%3)*.035,8),new THREE.MeshStandardMaterial({color:colours[i],roughness:.25,metalness:.07}));bottle.position.set(2.45+i*.36,y+.20,-6.88);scene.add(bottle);addBox(.07,.06,.08,bottle.position.x,y+.40,-6.88,black);
   const label=new THREE.Mesh(new THREE.PlaneGeometry(.10,.10),new THREE.MeshBasicMaterial({color:'#d4e0de'}));label.position.set(bottle.position.x,y+.19,-6.79);scene.add(label);
  }
 }
 addBox(3.4,.09,1.1,4.4,1.08,-5.8,new THREE.MeshStandardMaterial({color:'#9b8b72',roughness:.7}));for(const x of [2.9,5.9])addBox(.09,1.05,.09,x,.52,-5.8);
 addBox(2.2,1.45,.8,-5.8,.72,-7.6,black);addBox(1.5,.85,.04,-5.8,2.1,-8.22,new THREE.MeshStandardMaterial({color:'#b6b7ae',roughness:.8}));
 for(let i=0;i<3;i++){addBox(.58,.12,.52,-2.6+i*.72,.52,-7.5,shelfMat);addBox(.56,.55,.1,-2.6+i*.72,.88,-7.73,shelfMat);for(const x of [-.22,.22])addBox(.035,.5,.035,-2.6+i*.72+x,.25,-7.5)}
 const key=new THREE.DirectionalLight('#edf4ff',3.5);key.position.set(3,6,5);key.castShadow=true;key.shadow.mapSize.set(1024,1024);key.shadow.camera.left=-5;key.shadow.camera.right=5;key.shadow.camera.top=5;key.shadow.camera.bottom=-5;key.shadow.bias=-.001;key.shadow.normalBias=.035;scene.add(key);
 const fill=new THREE.DirectionalLight('#b9d5ff',1.4);fill.position.set(-5,4,-3);scene.add(fill);scene.add(new THREE.AmbientLight('#c7d7ec',.45));
 const rim=new THREE.DirectionalLight('#fff2de',2.2);rim.position.set(1,5,-6);scene.add(rim);
 const shade=new THREE.Mesh(new THREE.PlaneGeometry(3.2,6.3),new THREE.MeshBasicMaterial({map:shadowTexture(),transparent:true,depthWrite:false,opacity:.65}));shade.rotation.x=-Math.PI/2;shade.position.y=-.008;scene.add(shade);
}
function garageFloorTexture(){
 const c=document.createElement('canvas');c.width=c.height=1024;const ctx=c.getContext('2d'),count=32,cell=32;
 for(let y=0;y<count;y++)for(let x=0;x<count;x++){
  const red=(y===10||y===11||y===21||y===22)&&x>6&&x<25;ctx.fillStyle=red?((x+y)%2?'#9b1420':'#c01a28'):((x+y)%2?'#414951':'#171c22');ctx.fillRect(x*cell,y*cell,cell,cell);
  ctx.save();ctx.beginPath();ctx.rect(x*cell,y*cell,cell,cell);ctx.clip();ctx.strokeStyle=red?'#d54d54':((x+y)%2?'#5d6771':'#303942');ctx.lineWidth=.8;
  for(let k=-cell;k<cell*2;k+=4){ctx.beginPath();if((x+y)%2){ctx.moveTo(x*cell+k,y*cell);ctx.lineTo(x*cell+k+cell,y*cell+cell)}else{ctx.moveTo(x*cell+k,y*cell);ctx.lineTo(x*cell+k-cell,y*cell+cell)}ctx.stroke()}ctx.restore();ctx.strokeStyle='#090c10';ctx.strokeRect(x*cell,y*cell,cell,cell);
 }
 const t=new THREE.CanvasTexture(c);t.colorSpace=THREE.SRGBColorSpace;t.anisotropy=Math.min(8,renderer.capabilities.getMaxAnisotropy());return t;
}
function garageWallTexture(){
 const c=document.createElement('canvas');c.width=1024;c.height=512;const ctx=c.getContext('2d');ctx.fillStyle='#14191f';ctx.fillRect(0,0,c.width,c.height);
 for(let y=0;y<8;y++)for(let x=-1;x<9;x++){const xx=x*128+(y%2)*64;ctx.fillStyle=(x+y)%3?'#171d24':'#1c222a';ctx.fillRect(xx+2,y*64+2,124,60);ctx.strokeStyle='#090c10';ctx.lineWidth=3;ctx.strokeRect(xx,y*64,128,64)}
 const t=new THREE.CanvasTexture(c);t.colorSpace=THREE.SRGBColorSpace;t.wrapS=t.wrapT=THREE.RepeatWrapping;t.repeat.set(5,3);return t;
}
function shadowTexture(){const c=document.createElement('canvas');c.width=c.height=128;const ctx=c.getContext('2d'),g=ctx.createRadialGradient(64,64,10,64,64,64);g.addColorStop(0,'rgba(0,0,0,.75)');g.addColorStop(1,'rgba(0,0,0,0)');ctx.fillStyle=g;ctx.fillRect(0,0,128,128);return new THREE.CanvasTexture(c)}
// Decode normalized attributes before moving vertices; keep authored normals and UVs.
function worldGeometry(mesh){
 const src=mesh.geometry.toNonIndexed(),geo=new THREE.BufferGeometry();
 for(const key of Object.keys(src.attributes)){const a=src.attributes[key];if(!a)continue;const out=new Float32Array(a.count*a.itemSize);for(let i=0;i<a.count;i++)for(let k=0;k<a.itemSize;k++)out[i*a.itemSize+k]=a.getComponent(i,k);geo.setAttribute(key,new THREE.BufferAttribute(out,a.itemSize));}
 geo.applyMatrix4(mesh.matrixWorld);return geo;
}
function smoothPaintNormals(geo){
 const p=geo.attributes.position,normal=new Float32Array(p.count*3),sums=new Map(),keys=[],a=new THREE.Vector3(),b=new THREE.Vector3(),c=new THREE.Vector3(),face=new THREE.Vector3();
 for(let i=0;i<p.count;i++){const key=[p.getX(i),p.getY(i),p.getZ(i)].map(v=>Math.round(v*10000)).join(',');keys.push(key);if(!sums.has(key))sums.set(key,new THREE.Vector3());}
 for(let i=0;i<p.count;i+=3){a.fromBufferAttribute(p,i);b.fromBufferAttribute(p,i+1);c.fromBufferAttribute(p,i+2);face.crossVectors(b.sub(a),c.sub(a));for(let j=0;j<3;j++)sums.get(keys[i+j]).add(face);}
 for(let i=0;i<p.count;i++){const n=sums.get(keys[i]).normalize();normal.set(n.toArray(),i*3);}geo.setAttribute('normal',new THREE.BufferAttribute(normal,3));
}
function splitSurface(geo,accept){
 const yes=[],no=[],p=geo.attributes.position,v=new THREE.Vector3();for(let i=0;i<p.count;i+=3){v.set(0,0,0);for(let j=0;j<3;j++)v.add(new THREE.Vector3().fromBufferAttribute(p,i+j));v.multiplyScalar(1/3);(accept(v)?yes:no).push(i,i+1,i+2);}
 return [yes,no].map(list=>{if(!list.length)return null;const out=new THREE.BufferGeometry();for(const [key,a]of Object.entries(geo.attributes)){const values=new Float32Array(list.length*a.itemSize);list.forEach((index,i)=>{for(let k=0;k<a.itemSize;k++)values[i*a.itemSize+k]=a.getComponent(index,k)});out.setAttribute(key,new THREE.BufferAttribute(values,a.itemSize));}return out;});
}
async function loadCar(){
 const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);const asset=await loader.loadAsync(new URL('./assets/three/porsche-carrera-gt.glb',import.meta.url).href);const model=asset.scene;model.updateMatrixWorld(true);
 const box=new THREE.Box3().setFromObject(model),sz=box.getSize(new THREE.Vector3()),center=box.getCenter(new THREE.Vector3()),scale=4.7/Math.max(sz.x,sz.z);model.scale.multiplyScalar(scale);model.position.set(-center.x*scale,-box.min.y*scale,-center.z*scale);model.updateMatrixWorld(true);
 car=new THREE.Group();scene.add(car);door=new THREE.Group();door.position.set(.78,.50,.67);car.add(door);
 const cache=new Map(),wheelBuckets=new Map(),tyreGroups=new Set();
 const add=(geo,mat,name,toDoor=false)=>{if(!geo)return;const m=new THREE.Mesh(geo,mat);m.name=name;m.castShadow=true;m.receiveShadow=true;meshes.push(m);car.add(m);if(toDoor)door.attach(m);if(mat.userData.finish){filmSurfaces.push(m);hood.push(m);}return m;};
 model.traverse(n=>{if(!n.isMesh)return;const old=n.material,name=old.name||'',geo=worldGeometry(n);if(name==='C1_Paint')smoothPaintNormals(geo);let mat=cache.get(old);
  if(!mat){mat=old.clone();mat.userData={...old.userData};materials.add(mat);
   if(name==='C1_Paint'){mat=new THREE.MeshPhysicalMaterial({color:'#9d1726',roughness:.38,metalness:.12,clearcoat:.7,clearcoatRoughness:.09});mat.userData.finish=true;shaderDirt(mat);paints.push(mat);materials.add(mat);}
   else if(name==='C1_Base'||name==='C1_Color_Plastico_Negro'){shaderDirt(mat);}
   else if(name==='C1_Carbon'){shaderDirt(mat);mat.userData.finish=true;}
   else if(/Wheel1A/.test(name)){shaderDirt(mat);}
   cache.set(old,mat);
  }
  if(/Wheel1A|Callipers/.test(name)){geo.computeBoundingBox();const c=geo.boundingBox.getCenter(new THREE.Vector3()),quadrant=(c.x>0?'L':'R')+(c.z>0?'F':'B'),key=name+'-'+quadrant;if(!wheelBuckets.has(key))wheelBuckets.set(key,{geos:[],mat});wheelBuckets.get(key).geos.push(geo);if(/Wheel1A/.test(name))tyreGroups.add(quadrant);return;}
  // The source combines panels. Separate the visible driver-door region for the demo hinge.
  const doorRegion=v=>v.x>.69&&v.y>.28&&v.y<.92&&v.z>-.50&&v.z<.66;
  if(/C1_Paint|C1_Interior|C1_Carbon|C1_Base|C1_Color_Plastico_Negro/.test(name)&&!/Engine/.test(n.name)){const [leaf,body]=splitSurface(geo,doorRegion);add(body,mat,n.name);add(leaf,mat,n.name+'-door',true);}
  else if(name==='C1_Vidrios_Tr'){
   const [window,lenses]=splitSurface(geo,v=>v.y>.8&&v.z<1.05&&Math.abs(v.x)<.78);
   add(lenses,mat,n.name+'-lenses');if(window){const glassMat=new THREE.MeshPhysicalMaterial({color:'#a5b7be',metalness:.05,roughness:.08,transparent:true,opacity:.22,depthWrite:false,side:THREE.DoubleSide});glass.push(glassMat);materials.add(glassMat);const [leaf,rest]=splitSurface(window,v=>v.x>.63&&v.z>-.47&&v.z<.6);add(rest,glassMat,n.name+'-glass');add(leaf,glassMat,n.name+'-door-glass',true);}
  }else add(geo,mat,n.name);
 });
 for(const [name,bucket]of wheelBuckets){const geo=mergeGeometries(bucket.geos,false);if(geo)add(geo,bucket.mat,name);bucket.geos.forEach(g=>g.dispose());}
 tyres=[...tyreGroups];car.updateMatrixWorld(true);createServiceEffects();
}
const ray=new THREE.Raycaster();
function hoodPoint(x,z){ray.set(new THREE.Vector3(x,4,z),new THREE.Vector3(0,-1,0));const hit=ray.intersectObjects(hood,false)[0];if(!hit)return null;return {point:hit.point,normal:hit.face.normal.clone().transformDirection(hit.object.matrixWorld)}}
function sidePoint(y,z){ray.set(new THREE.Vector3(2.5,y,z),new THREE.Vector3(-1,0,0));const hit=ray.intersectObjects(filmSurfaces,false).find(h=>h.face.normal.clone().transformDirection(h.object.matrixWorld).x>.55);if(!hit)return null;return {point:hit.point,normal:hit.face.normal.clone().transformDirection(hit.object.matrixWorld)}}
function createServiceEffects(){
 car.updateMatrixWorld(true);const hb=new THREE.Box3();hood.forEach(n=>hb.expandByObject(n));hb.min.x=-.58;hb.max.x=.58;hb.min.z=1.1;hb.max.z=1.85;const hs=hb.getSize(new THREE.Vector3());let seed=27;const rand=()=>{seed=seed*16807%2147483647;return seed/2147483647};
 const beadGeo=new THREE.SphereGeometry(1,10,8),beadMat=new THREE.MeshPhysicalMaterial({color:'#ecf6fc',metalness:0,roughness:.035,transmission:.98,thickness:.012,ior:1.333,clearcoat:1,clearcoatRoughness:.02,transparent:true,opacity:1,envMapIntensity:1.5});
 for(let i=0;i<150;i++){let point,x,z;for(let j=0;j<15&&!point;j++){x=hb.min.x+hs.x*(.08+rand()*.84);z=hb.min.z+hs.z*(.08+rand()*.84);point=hoodPoint(x,z)}if(point)beadData.push({kind:'hood',x,z,origin:z,size:.0015+rand()**2*.0045,phase:rand(),...point})}
 for(let i=0;i<150;i++){let point,y,z;for(let j=0;j<20&&!point;j++){y=.33+rand()*.68;z=-1.65+rand()*3.15;point=sidePoint(y,z)}if(point)beadData.push({kind:'side',y,z,origin:y,size:.0012+rand()**2*.0048,phase:rand(),...point})}
 beadMesh=new THREE.InstancedMesh(beadGeo,beadMat,beadData.length);beadMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);scene.add(beadMesh);updateBeads(0,false);
 rainMesh=new THREE.InstancedMesh(new THREE.SphereGeometry(1,6,5),new THREE.MeshPhysicalMaterial({color:'#d5e9f6',metalness:0,roughness:.03,transmission:.65,thickness:.015,ior:1.333,transparent:true,opacity:.7,envMapIntensity:1.5}),180);rainMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);scene.add(rainMesh);
 for(let i=0;i<180;i++)rainData.push({x:-.9+rand()*2.35,z:-2+rand()*4.2,phase:rand(),speed:2.1+rand()*1.2,size:.0015+rand()*.002});updateRain(0);
 filmGroup=new THREE.Group();scene.add(filmGroup);filmMaterial=new THREE.MeshPhysicalMaterial({color:'#a3ebdc',metalness:.05,roughness:.07,clearcoat:1,transparent:true,opacity:.12,depthWrite:false,polygonOffset:true,polygonOffsetFactor:-2,side:THREE.DoubleSide});
 filmSurfaces.forEach(n=>{const cover=new THREE.Mesh(n.geometry,filmMaterial);n.userData.filmCover=cover;filmGroup.add(cover);const edge=new THREE.LineSegments(new THREE.EdgesGeometry(n.geometry,35),new THREE.LineBasicMaterial({color:'#40b7a2',transparent:true,opacity:.65}));n.userData.filmEdge=edge;filmGroup.add(edge);filmEdges.push(edge)});
}
function updateRain(t){const d=new THREE.Object3D();rainData.forEach((r,i)=>{const y=3.5-((t*r.speed+r.phase*3.2)%3.2);d.position.set(r.x,y,r.z);d.scale.set(r.size,r.size*4.7,r.size);d.updateMatrix();rainMesh.setMatrixAt(i,d.matrix)});rainMesh.instanceMatrix.needsUpdate=true}
function updateBeads(time,animate){const dummy=new THREE.Object3D(),up=new THREE.Vector3(0,1,0);beadData.forEach((b,i)=>{const flow=animate?((time*.00012+b.phase)%1):b.phase;let pos=b.point;if(b.kind==='side'){const p=sidePoint(b.origin-flow*.22,b.z);if(p)pos=p.point;}else{const p=hoodPoint(b.x,b.origin+flow*.14);if(p)pos=p.point;}dummy.position.copy(pos).addScaledVector(b.normal,b.size*.4);dummy.quaternion.setFromUnitVectors(up,b.normal);dummy.scale.set(b.size*(b.kind==='side'?1.7:1),b.size*.6,b.size*1.12);dummy.updateMatrix();beadMesh.setMatrixAt(i,dummy.matrix)});beadMesh.instanceMatrix.needsUpdate=true}
function size(){if(!renderer)return;const w=Math.max(stage.clientWidth,1),h=Math.max(stage.clientHeight,1);renderer.setPixelRatio(Math.min(devicePixelRatio,mobile()?1.35:1.65));renderer.setSize(w,h,false);camera.aspect=w/h;camera.fov=mobile()?39:34;camera.updateProjectionMatrix();draw(performance.now(),true)}
function chapterProgress(){const lab=document.querySelector('#services').offsetTop,inside=document.querySelector('#inside').offsetTop;const y=scrollY;const service=THREE.MathUtils.smoothstep(y,lab*.35,lab*.92),interior=THREE.MathUtils.smoothstep(y,inside-stage.clientHeight*.4,inside);return {service,interior,lab,inside}}
const views={front:[5.8,2.45,6.6],side:[7.1,1.8,.3],rear:[5.5,2.4,-6.5],hood:[2.3,3.1,4.0],'ceramic-side':[5.1,1.8,1.0]};
let targetDirt=1,wasMoving=true,viewOffsetX=0,viewOffsetY=0,manualYaw=0,manualPitch=0,manualPan=new THREE.Vector3(),dragStart=null;
function draw(time,instant=false){if(!ready||lost)return;const dt=Math.min((time-(last||time-16))/1000,.06);last=time;const st=s(),p=chapterProgress();const ease=instant||!st.motion?1:1-Math.exp(-dt*4.5);
 const cabin=p.interior>.7&&st.unlocked; if(cabinLamp)cabinLamp.intensity=cabin?7:0; const detail=st.service==='detailing';
 targetDirt=(1-p.service)+(detail?(1-st.amount)*p.service:0);
 dirtUniform.value=smooth(dirtUniform.value,targetDirt,ease);
 paints.forEach(m=>{m.roughness=smooth(m.roughness,st.service==='ppf'?smooth(.30,.18,st.amount):st.service==='ceramic'?smooth(.30,.22,st.amount):.30,ease);m.clearcoat=st.service==='ppf'?1:.8;m.clearcoatRoughness=st.service==='ppf'?.025:.09});
 const tintOpacity=p.interior>.6?.18:st.service==='tint'?.14+(1-st.tint/100)*.85*st.amount:.22;
 glass.forEach(m=>m.opacity=smooth(m.opacity,tintOpacity,ease));
 const waterVisible=st.service==='ceramic'&&st.water&&p.service>.75&&p.interior<.6;beadMesh.visible=waterVisible;beadMesh.material.opacity=st.amount;rainMesh.visible=waterVisible;rainMesh.material.opacity=.7*st.amount;if(waterVisible){if(st.motion&&!instant)rainTime+=dt;updateBeads(rainTime*1000,st.motion);updateRain(rainTime);}
 filmGroup.visible=st.service==='ppf'&&p.service>.75&&p.interior<.6;filmMaterial.opacity=st.film?.14*st.amount:0;filmEdges.forEach(e=>e.visible=st.film&&st.amount>.01);
 let desired=new THREE.Vector3(...views[st.view]);const look=new THREE.Vector3(0,.67,0);if(st.view==='hood'){look.z=1.3;look.y=.72;}
 if(st.orbit&&st.motion&&p.service>.8&&p.interior<.3){rotation+=dt*.17;desired.applyAxisAngle(new THREE.Vector3(0,1,0),rotation)}
 
 const arrival=new THREE.Vector3(5.7,2.3,6.5);desired.lerp(arrival,1-p.service);
 if(cabin){if(st.cabin==='driver'){desired.set(.35,.98,.05);look.set(.35,.83,.94)}else if(st.cabin==='seats'){desired.set(1.9,1.6,.30);look.set(.1,.7,-.12)}else{desired.set(4.9,2.35,4.1);look.set(.25,.65,0)}}
 else if(p.interior>.05){const interiorView=new THREE.Vector3(5.7,2.5,3.7);desired.lerp(interiorView,p.interior)}
 const aspect=camera.aspect;if(mobile()&&!cabin){desired.multiplyScalar(Math.max(1.25,.7/aspect));}else if(!mobile()&&aspect<1.15&&!cabin)desired.multiplyScalar(1.15/aspect);
 if(mobile()&&cabin&&st.cabin==='door')desired.multiplyScalar(1.25);
 smoothPointer.lerp(st.motion?pointer:new THREE.Vector2(),ease);if(!cabin){desired.x+=smoothPointer.x*.17;desired.y+=smoothPointer.y*.1;}
 if(manualYaw||manualPitch){const offset=desired.clone().sub(look),spherical=new THREE.Spherical().setFromVector3(offset);spherical.theta+=manualYaw;spherical.phi=THREE.MathUtils.clamp(spherical.phi+manualPitch,.12,Math.PI-.12);desired.copy(look).add(new THREE.Vector3().setFromSpherical(spherical));}
 desired.add(manualPan);look.add(manualPan);
 actualCamera.lerp(desired,ease);actualLook.lerp(look,ease);camera.position.copy(actualCamera);camera.lookAt(actualLook);
 const desiredX=mobile()?0:p.service*.14-p.interior*.36;let desiredY=mobile()?-.01: -.07*(1-p.service);
 if(mobile()){desiredY=p.service*(.5-265/stage.clientHeight)-p.interior*.28;if(cabin&&st.cabin==='driver')desiredY=-.12;}
 viewOffsetX=smooth(viewOffsetX,desiredX,ease);viewOffsetY=smooth(viewOffsetY,desiredY,ease);camera.setViewOffset(stage.clientWidth,stage.clientHeight,stage.clientWidth*viewOffsetX,stage.clientHeight*viewOffsetY,stage.clientWidth,stage.clientHeight);
 doorAngle=smooth(doorAngle,cabin?-1.13:0,ease);door.rotation.y=doorAngle;car.updateMatrixWorld(true);
 filmSurfaces.forEach(n=>{for(const key of ['filmCover','filmEdge']){const o=n.userData[key];if(o){o.matrixAutoUpdate=false;o.matrix.copy(n.matrixWorld);}}});
 camera.fov=smooth(camera.fov,cabin&&st.cabin==='driver'?55:mobile()?39:34,ease);camera.updateProjectionMatrix();renderer.render(scene,camera);frames++;
 const chapter=p.interior>.5?'interior':p.service>.5?'service':'arrival';experience.dataset.chapter=chapter;
}
function canRun(){return ready&&!lost&&visible&&!document.hidden&&!document.body.classList.contains('modal-open')&&s().motion}
function tick(time){frame=0;if(!canRun())return;draw(time);frame=requestAnimationFrame(tick)}
function sync(){if(frame)cancelAnimationFrame(frame);frame=0;last=0;if(!ready||lost)return;if(canRun())frame=requestAnimationFrame(tick);else draw(performance.now(),true)}
async function start(){
 renderer=new THREE.WebGLRenderer({antialias:!mobile(),alpha:false,powerPreference:'high-performance'});renderer.outputColorSpace=THREE.SRGBColorSpace;renderer.toneMapping=THREE.ACESFilmicToneMapping;renderer.toneMappingExposure=1;renderer.shadowMap.enabled=!mobile();renderer.shadowMap.type=THREE.PCFSoftShadowMap;renderer.domElement.tabIndex=-1;renderer.domElement.setAttribute('aria-hidden','true');stage.append(renderer.domElement);
 scene=new THREE.Scene();camera=new THREE.PerspectiveCamera(34,1,.025,100);garage();cabinLamp=new THREE.PointLight('#d6e7ff',0,3,1);cabinLamp.position.set(0,1.2,.30);scene.add(cabinLamp);await loadCar();ready=true;experience.dataset.scene='ready';status.textContent='PORSCHE CARRERA GT / SCROLL TO TRANSFORM';size();sync();
 window.addEventListener('dapper:studio',()=>{manualYaw=0;manualPitch=0;manualPan.set(0,0,0);sync()});window.addEventListener('dapper:visibility',sync);document.addEventListener('visibilitychange',sync);window.addEventListener('scroll',()=>{if(!s().motion)draw(performance.now(),true)},{passive:true});
 const observer=new IntersectionObserver(entries=>{visible=entries[0].isIntersecting;sync()});observer.observe(experience);
 const resizer=new ResizeObserver(size);resizer.observe(stage);
 window.addEventListener('pointermove',e=>{if(e.pointerType==='mouse')pointer.set(e.clientX/innerWidth*2-1,1-e.clientY/innerHeight*2)},{passive:true});
 renderer.domElement.addEventListener('pointerdown',e=>{
  if(e.button!==0&&e.button!==1)return;e.preventDefault();
  dragStart={x:e.clientX,y:e.clientY,yaw:manualYaw,pitch:manualPitch,pan:manualPan.clone(),id:e.pointerId,button:e.button,touch:e.pointerType==='touch',right:new THREE.Vector3().setFromMatrixColumn(camera.matrix,0),up:new THREE.Vector3().setFromMatrixColumn(camera.matrix,1),scale:Math.max(.0007,camera.position.distanceTo(actualLook)*.0015)};
  if(e.pointerType==='mouse')renderer.domElement.setPointerCapture(e.pointerId);
 });
 renderer.domElement.addEventListener('pointermove',e=>{if(!dragStart)return;const d=dragStart,dx=e.clientX-d.x,dy=e.clientY-d.y;
  if(d.touch&&Math.abs(dy)>Math.abs(dx)&&Math.abs(dy)>8)return;
  if(d.button===1||d.touch){manualYaw=d.yaw-dx*.007;manualPitch=d.pitch-dy*.005;}else{manualPan.copy(d.pan).addScaledVector(d.right,-dx*d.scale).addScaledVector(d.up,dy*d.scale);}
  if(!s().motion)draw(performance.now(),true);
 });
 for(const event of ['pointerup','pointercancel','lostpointercapture'])renderer.domElement.addEventListener(event,()=>dragStart=null);
 renderer.domElement.addEventListener('auxclick',e=>{if(e.button===1)e.preventDefault()});
 window.addEventListener('dapper:camera-reset',()=>{manualYaw=manualPitch=0;manualPan.set(0,0,0);sync()});
 renderer.domElement.addEventListener('webglcontextlost',e=>{e.preventDefault();lost=true;experience.dataset.scene='lost';status.textContent='Photo mode · graphics interrupted';if(frame)cancelAnimationFrame(frame);frame=0});
 renderer.domElement.addEventListener('webglcontextrestored',()=>{lost=false;experience.dataset.scene='ready';status.textContent='Studio restored';sync()});
 window.addEventListener('pagehide',()=>{if(frame)cancelAnimationFrame(frame);observer.disconnect();resizer.disconnect();geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());env?.dispose();renderer.dispose()},{once:true});
}
await start();

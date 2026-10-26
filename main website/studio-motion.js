import Lenis from './assets/three/lenis.mjs';
let lenis,frame=0;const reduced=matchMedia('(prefers-reduced-motion: reduce)'),fine=matchMedia('(pointer:fine)');
function tick(time){frame=0;if(!lenis||document.hidden)return;lenis.raf(time);frame=requestAnimationFrame(tick)}
function sync(){const enabled=window.dapperStudio?.motion&&!reduced.matches&&fine.matches;if(enabled&&!lenis){lenis=new Lenis({lerp:.075,smoothWheel:true,anchors:{offset:0},prevent:node=>node.closest('dialog')});lenis.on('scroll',()=>window.ScrollTrigger?.update());}else if(!enabled&&lenis){lenis.destroy();lenis=null}if(!lenis||document.hidden){if(frame)cancelAnimationFrame(frame);frame=0}else if(!frame)frame=requestAnimationFrame(tick)}
window.addEventListener('dapper:studio',sync);document.addEventListener('visibilitychange',sync);window.addEventListener('dapper:visibility',()=>{if(document.body.classList.contains('modal-open'))lenis?.stop();else lenis?.start()});sync();

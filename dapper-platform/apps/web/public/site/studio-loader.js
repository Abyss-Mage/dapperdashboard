import './studio-ui.js';
import './studio-motion.js';
const experience=document.querySelector('#experience');
const status=document.querySelector('#scene-status');
const loadButton=document.querySelector('#load-studio');
let started=false;
async function loadStudio(){
 if(started)return;
 started=true;loadButton.hidden=true;experience.dataset.scene='loading';
 status.textContent='Loading the interactive studio…';
 const timer=setTimeout(()=>{experience.dataset.scene='fallback';status.textContent='Photo mode · the studio is taking longer to load';},15000);
 try{await import('./studio-scene.js');}
 catch(error){experience.dataset.scene='fallback';status.textContent='Photo mode · explore our services';console.warn('Interactive studio unavailable; service information remains available.',error);}
 finally{clearTimeout(timer)}
}
loadButton.addEventListener('click',loadStudio);
loadButton.hidden=false;
status.textContent='Photo mode · explore in 3D when you’re ready';
const optional=matchMedia('(pointer: coarse)').matches||matchMedia('(prefers-reduced-motion: reduce)').matches||navigator.connection?.saveData;
if(!optional){if('requestIdleCallback' in window)requestIdleCallback(loadStudio,{timeout:1800});else setTimeout(loadStudio,300);}

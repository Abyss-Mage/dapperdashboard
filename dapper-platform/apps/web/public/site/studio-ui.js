(() => {
 const $ = s => document.querySelector(s), $$ = s => [...document.querySelectorAll(s)];
 const reduce = matchMedia('(prefers-reduced-motion: reduce)');
 let saved = false; try {saved=sessionStorage.getItem('dapper-motion-off')==='true'} catch {}
 const state = {service:'detailing',amount:1,tint:20,water:true,film:true,view:'front',orbit:false,unlocked:false,cabin:'door',motion:!reduce.matches&&!saved};
 window.dapperStudio = state;
 function emit(){window.dispatchEvent(new CustomEvent('dapper:studio',{detail:{...state}}))}
 const content = {
 detailing:['A FRESH START','From road-worn to remarkable.','Remove the everyday build-up. Explore a clean, refined exterior, then unlock the cabin for a closer look inside.','Dirty exterior to a clean finish.'],
 ceramic:['WATER-REPELLENT CARE','A finish that lets water go.','Inspect the side panels in the rain, then move to the hood. Watch water bead and run across the clean finish, illustrating water repellency.','Clean, untreated surface to a coated water-beading preview.'],
 ppf:['AN INVISIBLE LAYER OF CARE','Protect the finish you love.','Explore a glossy finish with a clear-film layer across the full body. Highlight the coverage to see where the illustrative film sits.','Clean, unprotected finish to a full-body film preview.'],
 tint:['YOUR VIEW, REFINED','Find your shade.','Explore the look of window tint. Adjust visible light transmission from 20% to 95%; a lower VLT creates a darker appearance.','Original glass to the selected tint preview.']
 };
 function selectService(key,focus=false){$('#services').scrollIntoView({behavior:state.motion&&!focus?'smooth':'instant',block:'start'});state.service=key;state.view=key==='ceramic'?'ceramic-side':key==='tint'?'side':'front';state.orbit=false;
  $$('[data-service]').forEach(b=>{const active=b.dataset.service===key;b.setAttribute('aria-selected',active);b.tabIndex=active?0:-1;if(active&&focus)b.focus()});
  const c=content[key];$('#service-eyebrow').textContent=c[0];$('#service-title').textContent=c[1];$('#service-description').textContent=c[2];$('#comparison-help').textContent=c[3];$('#service-panel').setAttribute('aria-labelledby','tab-'+key);
  ['ceramic','ppf','tint'].forEach(k=>$('#'+k+'-controls').hidden=key!==k);
  state.amount=1;$('#comparison').value=100;$('#comparison-value').textContent='100% after';syncViews();emit();
 }
 $('#camera-reset').addEventListener('click',()=>window.dispatchEvent(new Event('dapper:camera-reset')));
 function syncViews(){$$('[data-water-view]').forEach(b=>b.setAttribute('aria-pressed',b.dataset.waterView===state.view));$$('[data-view]').forEach(b=>b.setAttribute('aria-pressed',b.dataset.view===state.view));$('#orbit').setAttribute('aria-pressed',state.orbit)}
 $$('[data-service]').forEach((b,i)=>{b.addEventListener('click',()=>selectService(b.dataset.service));b.addEventListener('keydown',e=>{const a=$$('[data-service]');let n;if(['ArrowRight','ArrowDown'].includes(e.key))n=(i+1)%a.length;if(['ArrowLeft','ArrowUp'].includes(e.key))n=(i+a.length-1)%a.length;if(e.key==='Home')n=0;if(e.key==='End')n=a.length-1;if(n!==undefined){e.preventDefault();selectService(a[n].dataset.service,true)}})});
 function compare(value){state.amount=Number(value)/100;$('#comparison').value=value;$('#comparison-value').textContent=value+'% after';emit()}
 $('#comparison').addEventListener('input',e=>compare(e.target.value));$$('[data-compare]').forEach(b=>b.addEventListener('click',()=>compare(b.dataset.compare)));
 $('#tint').addEventListener('input',e=>{state.tint=Number(e.target.value);$('#tint-value').textContent=state.tint+'% VLT';emit()});
 for(const [id,key] of [['water-test','water'],['film-toggle','film']])$('#'+id).addEventListener('click',()=>{state[key]=!state[key];$('#'+id).setAttribute('aria-pressed',state[key]);$('#'+id+' span').textContent=state[key]?'On':'Off';emit()});
 $$('[data-water-view]').forEach(b=>b.addEventListener('click',()=>{state.view=b.dataset.waterView;state.orbit=false;syncViews();emit()}));
 $$('[data-view]').forEach(b=>b.addEventListener('click',()=>{state.view=b.dataset.view;state.orbit=false;syncViews();emit()}));$('#orbit').addEventListener('click',()=>{state.orbit=!state.orbit;syncViews();emit()});
 $('#unlock').addEventListener('click',()=>{state.unlocked=!state.unlocked;if(state.unlocked)$('#inside').scrollIntoView({behavior:state.motion?'smooth':'instant',block:'start'});state.cabin='door';$('#unlock').setAttribute('aria-pressed',state.unlocked);$('#unlock').innerHTML=state.unlocked?'Close cabin <span>↗</span>':'Unlock & explore interior <span>↗</span>';$('#cabin-controls').hidden=!state.unlocked;$('#lock-status').textContent=state.unlocked?'CABIN UNLOCKED / EXPLORE THE DETAILS':'CABIN LOCKED / TAP TO EXPLORE';$$('[data-cabin]').forEach(b=>b.setAttribute('aria-pressed',b.dataset.cabin==='door'));emit()});
 $$('[data-cabin]').forEach(b=>b.addEventListener('click',()=>{state.cabin=b.dataset.cabin;$$('[data-cabin]').forEach(a=>a.setAttribute('aria-pressed',a===b));emit()}));
 function syncMotion(){document.documentElement.dataset.motion=state.motion?'on':'off';$('#motion').setAttribute('aria-pressed',!state.motion);$('#motion').innerHTML=state.motion?'Ⅱ <span>Pause motion</span>':'▷ <span>Motion paused</span>';emit()}
 $('#motion').addEventListener('click',()=>{state.motion=reduce.matches?false:!state.motion;saved=!state.motion;try{sessionStorage.setItem('dapper-motion-off',!state.motion)}catch{}syncMotion()});reduce.addEventListener('change',()=>{state.motion=!reduce.matches&&!saved;syncMotion()});syncMotion();
 if(window.gsap){gsap.registerPlugin(ScrollTrigger);const mm=gsap.matchMedia();mm.add('(prefers-reduced-motion: no-preference)',()=>{if(!state.motion)return;gsap.from('.hero-kicker,h1,.hero-note,.hero-bottom',{y:20,opacity:0,duration:1,stagger:.12,ease:'power3.out'});gsap.utils.toArray('.work-card,.steps article').forEach(el=>gsap.from(el,{y:30,opacity:0,duration:.9,ease:'power3.out',scrollTrigger:{trigger:el,start:'top 95%',once:true}}))})}
})();

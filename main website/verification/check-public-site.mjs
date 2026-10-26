// Focused behaviour checks without a browser. Visual/device acceptance is separate.
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const root = new URL('../', import.meta.url);
const source = await readFile(new URL('public-site.js',root),'utf8');
const handlers = new Map();
const classes = new Set();
const attributes = new Map([['aria-expanded','false']]);
let focused = false;
const toggle = {hidden:true, setAttribute:(k,v)=>attributes.set(k,v),getAttribute:k=>attributes.get(k),focus:()=>{focused=true},addEventListener:(k,v)=>handlers.set('toggle:'+k,v)};
const navigation = {classList:{remove:k=>classes.delete(k),toggle:(k,v)=>v?classes.add(k):classes.delete(k)},addEventListener:(k,v)=>handlers.set('nav:'+k,v)};
const document = {querySelector:s=>s==='.menu-toggle'?toggle:navigation,documentElement:{classList:{add:()=>{}}},addEventListener:(k,v)=>handlers.set('document:'+k,v)};
vm.runInNewContext(source,{document,matchMedia:()=>({addEventListener:(k,v)=>handlers.set('media:'+k,v)})});
assert.equal(toggle.hidden,false);
handlers.get('toggle:click')();assert.equal(attributes.get('aria-expanded'),'true');assert(classes.has('is-open'));
handlers.get('document:keydown')({key:'Escape'});assert.equal(attributes.get('aria-expanded'),'false');assert(focused);
handlers.get('toggle:click')();handlers.get('nav:click')({target:{closest:()=>true}});assert.equal(attributes.get('aria-expanded'),'false');
handlers.get('toggle:click')();handlers.get('document:focusin')({target:{closest:()=>null}});assert.equal(attributes.get('aria-expanded'),'false');
handlers.get('toggle:click')();handlers.get('media:change')();assert.equal(attributes.get('aria-expanded'),'false');
const loader = (await readFile(new URL('studio-loader.js',root),'utf8')).replace(/^import .*;\r?\n/gm,'');
for(const scenario of ['coarse','reduce','saveData','desktop']){
 let scheduled=false;
 const button={hidden:true,addEventListener:()=>{}};
 const experience={dataset:{scene:'poster'}};
 const status={textContent:''};
 vm.runInNewContext(loader,{
  document:{querySelector:s=>s==='#experience'?experience:s==='#scene-status'?status:button},
  matchMedia:q=>({matches:scenario==='coarse'?q.includes('coarse'):scenario==='reduce'?q.includes('reduce'):false}),
  navigator:{connection:{saveData:scenario==='saveData'}},window:{requestIdleCallback:true},
  requestIdleCallback:()=>{scheduled=true},setTimeout:()=>{scheduled=true},clearTimeout:()=>{},console
 });
 assert.equal(scheduled,scenario==='desktop',scenario);
 assert.equal(button.hidden,false,scenario);
 assert.equal(experience.dataset.scene,'poster',scenario);
}
console.log('PASS: menu disclosure, Escape/focus, navigation dismissal, focus exit, breakpoint reset; coarse/reduced/data-save defer graphics; desktop schedules graphics.');

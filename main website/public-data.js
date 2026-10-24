// Only published public projections enter this surface. No staff/customer records.
(() => {
 const serviceList=document.querySelector('[data-service-list]');
 const gallery=document.querySelector('#published-portfolio');
 const status=document.querySelector('#portfolio-status');
 if(!serviceList&&!gallery)return;
 let images=[],filter='all',opener,loaded=false;
 const filters=[...document.querySelectorAll('[data-filter]')];
 filters.forEach(button=>button.disabled=true);
 const dialog=document.querySelector('#portfolio-dialog');
 const element=(tag,text,className)=>{const node=document.createElement(tag);if(text)node.textContent=text;if(className)node.className=className;return node};
 function renderGallery(){
  if(!gallery)return;gallery.replaceChildren();
  const shown=images.filter(item=>filter==='all'||item.category===filter);
  for(const item of shown){
   const button=element('button',null,gallery.classList.contains('work-grid')?'work-card':'portfolio-item');button.type='button';button.dataset.image='/api/v1/public/media/'+encodeURIComponent(item.mediaId);button.dataset.title=item.title;
   const image=element('img');image.src=button.dataset.image;image.alt=item.alt;image.loading='lazy';image.width=1100;image.height=825;
   button.append(image,element('span',item.title));gallery.append(button);
  }
  if(status)status.textContent=shown.length?`${shown.length} published images${images.length===200?' · showing up to 200 images':''}.`:'No published images in this category yet.';
 }
 function unavailable(container,message){
  if(!container)return;const state=element('div',null,'publication-state');state.append(element('p',message));const retry=element('button','Try again');retry.type='button';retry.addEventListener('click',load,{once:true});state.append(retry);container.replaceChildren(state);
 }
 async function load(){
  try{
   const response=await fetch('/api/v1/public/sites/dapper',{cache:'no-store',headers:{Accept:'application/json'}});
   if(!response.ok)throw Error();const data=await response.json();
   if(!Array.isArray(data.services)||!Array.isArray(data.portfolio))throw Error();
   if(serviceList){
    const category=serviceList.dataset.category;const items=data.services.filter(s=>!category||s.category===category);serviceList.replaceChildren();
    if(!items.length)serviceList.append(element('p','Contact the team for current packages in this category.','publication-state'));
    for(const service of items){const article=element('article',null,'published-service');article.append(element('h3',service.name),element('p',service.summary));const link=element('a','View options in your account ↗','text-link');link.href='/account';article.append(link);serviceList.append(article);}
   }
   loaded=true;filters.forEach(button=>button.disabled=false);images=data.portfolio;renderGallery();
  }catch{
   unavailable(serviceList,'Current packages could not be loaded. Contact the team or try again.');
   if(status)status.textContent='Latest portfolio images could not be loaded.';
   loaded=false;filters.forEach(button=>button.disabled=true);
   unavailable(gallery,'The portfolio could not be loaded. Try again, or contact Dapper to discuss recent work.');
  }
 }
 filters.forEach(button=>button.addEventListener('click',()=>{if(!loaded)return;filter=button.dataset.filter;filters.forEach(item=>item.setAttribute('aria-pressed',String(item===button)));renderGallery()}));
 if(dialog){
  document.addEventListener('click',event=>{const button=event.target.closest('[data-image]');if(!button)return;opener=button;const image=dialog.querySelector('#full-image');image.src=button.dataset.image;image.alt=button.querySelector('img')?.alt??'';dialog.querySelector('#image-title').textContent=button.dataset.title;dialog.showModal();document.body.classList.add('modal-open');window.dispatchEvent(new Event('dapper:visibility'))});
  dialog.querySelector('.close-image').addEventListener('click',()=>dialog.close());dialog.addEventListener('close',()=>{document.body.classList.remove('modal-open');opener?.focus();window.dispatchEvent(new Event('dapper:visibility'))});
  dialog.addEventListener('click',event=>{if(event.target===dialog){const box=dialog.getBoundingClientRect();if(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom)dialog.close()}});
 }
 load();
})();

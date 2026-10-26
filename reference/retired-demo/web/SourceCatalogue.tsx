'use client';
import {useEffect,useState} from 'react';
import {Search,Layers,TriangleAlert,ChevronLeft,ChevronRight,PlusCircle} from 'lucide-react';

type SourceService={sourceId:string;categorySourceId:string;name:string;description:string;priceRaw:string;durationRaw:string;durationUnitRaw:string;reviewFlags:string[]};
type SourceExtra={sourceId:string;parentServiceSourceId:string;name:string;description:string;priceRaw:string;durationRaw:string;durationUnitRaw:string;minQuantityRaw:string;maxQuantityRaw:string;reviewFlags:string[]};
type Catalogue={capturedAt:string;sourceTimezone:string;categories:{sourceId:string;name:string}[];services:SourceService[];extras?:SourceExtra[];counts?:{services:number;categories:number;extras:number;extrasRequiringReview:number};reviewCount:number;transactionEnabled:false};

const issueLabels:Record<string,string>={
  one_minute_duration_review:'The source duration is one minute. Confirm this value before scheduling.',
  missing_or_zero_duration_review:'Source duration is zero/missing. Approved SOP duration required.',
  unmapped_parent_service:'The source parent service link needs review.',
  unmapped_category:'The source category link needs review.',
  unrecognized_price:'The source amount needs review.',
  invalid_duration:'The source duration needs review.',
  unrecognized_duration_unit:'The source duration unit needs review.',
  quantity_range_review:'The source quantity limits need review.'
};

export default function SourceCatalogue(){
  const [data,setData]=useState<Catalogue|null>(null);
  const [error,setError]=useState('');
  const [viewMode,setViewMode]=useState<'services'|'addons'>('services');
  const [query,setQuery]=useState('');
  const [category,setCategory]=useState('all');
  const [review,setReview]=useState(false);
  const [page,setPage]=useState(1);
  const [attempt,setAttempt]=useState(0);

  useEffect(()=>{
    let cancelled=false;
    fetch('/api/v1/catalogue/bookingpress')
      .then(async r=>{if(!r.ok)throw Error('Catalogue unavailable');return r.json()})
      .then(value=>{if(!cancelled){setData(value);setError('')}})
      .catch(()=>{if(!cancelled)setError('Could not load the source catalogue.')});
    return()=>{cancelled=true};
  },[attempt]);

  const extrasByParent=new Map<string,SourceExtra[]>();
  if(data?.extras){
    for(const extra of data.extras){
      const list=extrasByParent.get(extra.parentServiceSourceId)||[];
      list.push(extra);
      extrasByParent.set(extra.parentServiceSourceId,list);
    }
  }

  const filteredServices=data?.services.filter(s=>(category==='all'||s.categorySourceId===category)&&(!review||s.reviewFlags.length>0)&&`${s.name} ${s.description} ${s.sourceId}`.toLowerCase().includes(query.trim().toLowerCase()))??[];

  const filteredExtras=data?.extras?.filter(e=>{
    const parent=data.services.find(s=>s.sourceId===e.parentServiceSourceId);
    const matchesCat=category==='all'||(parent&&parent.categorySourceId===category);
    const matchesReview=!review||e.reviewFlags.length>0;
    const matchesQuery=`${e.name} ${e.description} ${e.sourceId} ${parent?.name??''}`.toLowerCase().includes(query.trim().toLowerCase());
    return matchesCat&&matchesReview&&matchesQuery;
  })??[];

  const items=viewMode==='services'?filteredServices:filteredExtras;
  const pages=Math.max(1,Math.ceil(items.length/12));

  return (
    <>
      <section className="panel detail-card source-intro">
        <div className="source-heading">
          <div>
            <p className="eyebrow">WORDPRESS · BOOKINGPRESS EXPORTER 0.1.3</p>
            <h2>Service catalogue & add-on review</h2>
            <p>Original service, category, and add-on mappings from BookingPress. All 228 source add-on durations are flagged zero/missing; amounts and durations remain reference-only until approved. Checkout is disabled.</p>
          </div>
          <span className="status quote">Approval pending</span>
        </div>

        {data&&<>
          <div className="source-metrics">
            <span><strong>{data.services.length}</strong> services</span>
            <span><strong>{data.categories.length}</strong> categories</span>
            <span><strong>{data.extras?.length??0}</strong> add-ons</span>
            <span><strong>{(data.counts?.extrasRequiringReview??0)+data.reviewCount}</strong> flagged for review</span>
            <span>Captured {new Date(data.capturedAt).toLocaleDateString('en-CA')}</span>
          </div>

          <div className="source-view-switch" role="group" aria-label="Catalogue items switch" style={{marginTop:'16px',marginBottom:'0'}}>
            <button
              className={viewMode==='services'?'selected':''}
              aria-pressed={viewMode==='services'}
              onClick={()=>{setViewMode('services');setPage(1);}}>
              Services ({filteredServices.length})
            </button>
            <button
              className={viewMode==='addons'?'selected':''}
              aria-pressed={viewMode==='addons'}
              onClick={()=>{setViewMode('addons');setPage(1);}}>
              Add-ons & Extras ({filteredExtras.length})
            </button>
          </div>

          <div className="catalogue-controls">
            <label className="search">
              <Search size={16}/>
              <input
                aria-label={viewMode==='services'?"Search source services":"Search source add-ons"}
                value={query}
                placeholder={viewMode==='services'?"Search services or source ID…":"Search add-ons or parent service…"}
                onChange={e=>{setQuery(e.target.value);setPage(1)}}
              />
            </label>
            <label className="catalogue-category">
              Category
              <select value={category} onChange={e=>{setCategory(e.target.value);setPage(1)}}>
                <option value="all">All categories</option>
                {data.categories.map(c=><option value={c.sourceId} key={c.sourceId}>{c.name}</option>)}
              </select>
            </label>
            <label className="catalogue-review">
              <input type="checkbox" checked={review} onChange={e=>{setReview(e.target.checked);setPage(1)}}/>
              Needs review only
            </label>
          </div>
        </>}

        {error&&<div role="alert"><p className="field-error">{error}</p><button className="secondary" onClick={()=>setAttempt(n=>n+1)}>Retry catalogue</button></div>}
        {!data&&!error&&<p role="status">Loading source catalogue…</p>}
      </section>

      {data&&<>
        <p role="status" className="catalogue-result-count">
          {items.length} matching {viewMode==='services'?'services':'add-ons'} · Currency and source durations require confirmation.
        </p>

        {viewMode==='services'?(
          <div className="catalogue-grid">
            {filteredServices.slice((page-1)*12,page*12).map(service=>{
              const serviceAddons=extrasByParent.get(service.sourceId)||[];
              return (
                <article className="panel source-service" key={service.sourceId}>
                  <div className="source-service-category">
                    <Layers size={15}/>
                    <span>{data.categories.find(c=>c.sourceId===service.categorySourceId)?.name??'Unmapped category'}</span>
                    <small>#{service.sourceId}</small>
                  </div>
                  <h2>{service.name}</h2>
                  <dl className="source-values">
                    <div>
                      <dt>Source amount</dt>
                      <dd>{service.priceRaw||'Not set'}</dd>
                    </div>
                    <div>
                      <dt>Source duration</dt>
                      <dd>{service.durationRaw} {service.durationUnitRaw}</dd>
                    </div>
                  </dl>
                  {service.reviewFlags.length>0&&<div className="source-review-flags">
                    <TriangleAlert size={15}/>
                    <div>{service.reviewFlags.map(flag=><p key={flag}>{issueLabels[flag]??'Source value needs review.'}</p>)}</div>
                  </div>}
                  {serviceAddons.length>0&&(
                    <details style={{marginBottom:'10px'}}>
                      <summary style={{fontWeight:600,color:'#d91627',cursor:'pointer'}}>
                        <PlusCircle size={14} style={{display:'inline',verticalAlign:'middle',marginRight:'5px'}}/>
                        {serviceAddons.length} linked add-ons available
                      </summary>
                      <ul style={{margin:'8px 0 0',paddingLeft:'18px',fontSize:'11px',lineHeight:'1.6'}}>
                        {serviceAddons.map(extra=>(
                          <li key={extra.sourceId} style={{marginBottom:'4px'}}>
                            <strong>{extra.name}</strong> (#{extra.sourceId}) · ${extra.priceRaw} · {extra.durationRaw}{extra.durationUnitRaw} (flagged)
                          </li>
                        ))}
                      </ul>
                    </details>
                  )}
                  <details>
                    <summary>Service description</summary>
                    <p className="source-description">{service.description||'No description in the source catalogue.'}</p>
                  </details>
                  <span className="source-reference-label">Reference only · not an approved offer</span>
                </article>
              );
            })}
          </div>
        ):(
          <div className="catalogue-grid">
            {filteredExtras.slice((page-1)*12,page*12).map(extra=>{
              const parent=data.services.find(s=>s.sourceId===extra.parentServiceSourceId);
              return (
                <article className="panel source-service" key={extra.sourceId}>
                  <div className="source-service-category">
                    <PlusCircle size={15} style={{color:'#d91627'}}/>
                    <span>Add-on for: <strong>{parent?.name??`Service #${extra.parentServiceSourceId}`}</strong></span>
                    <small>Extra #{extra.sourceId}</small>
                  </div>
                  <h2>{extra.name}</h2>
                  <dl className="source-values">
                    <div>
                      <dt>Source amount</dt>
                      <dd>${extra.priceRaw||'Not set'}</dd>
                    </div>
                    <div>
                      <dt>Source duration</dt>
                      <dd>{extra.durationRaw} {extra.durationUnitRaw}</dd>
                    </div>
                    <div>
                      <dt>Quantity bounds</dt>
                      <dd style={{fontSize:'13px',fontWeight:600}}>{extra.minQuantityRaw} – {extra.maxQuantityRaw}</dd>
                    </div>
                  </dl>
                  {extra.reviewFlags.length>0&&<div className="source-review-flags">
                    <TriangleAlert size={15}/>
                    <div>{extra.reviewFlags.map(flag=><p key={flag}>{issueLabels[flag]??'Source value needs review.'}</p>)}</div>
                  </div>}
                  <details>
                    <summary>Add-on description</summary>
                    <p className="source-description">{extra.description||'No description in source add-on.'}</p>
                  </details>
                  <span className="source-reference-label">Reference only · non-standalone add-on</span>
                </article>
              );
            })}
          </div>
        )}

        {items.length===0&&<div className="panel empty-state">
          <Search size={28}/>
          <h2>No matching {viewMode==='services'?'services':'add-ons'}</h2>
          <p>Try another search or category.</p>
          <button className="secondary" onClick={()=>{setQuery('');setCategory('all');setReview(false);setPage(1)}}>Clear filters</button>
        </div>}

        <nav className="catalogue-pagination" aria-label="Catalogue pages">
          <span>Page {page} of {pages}</span>
          <button className="secondary" disabled={page===1} onClick={()=>setPage(n=>n-1)}><ChevronLeft size={15}/> Previous</button>
          <button className="secondary" disabled={page>=pages} onClick={()=>setPage(n=>n+1)}>Next <ChevronRight size={15}/></button>
        </nav>
      </>}
    </>
  );
}

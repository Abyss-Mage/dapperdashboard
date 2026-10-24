export function publicDocumentResponse(document:string) {
  const indexing=process.env.DAPPER_PUBLIC_INDEXING==='enabled';
  return new Response(indexing?document.replace('noindex,nofollow','index,follow'):document,{headers:{
    'Content-Type':'text/html; charset=utf-8',
    'X-Robots-Tag':indexing?'index, follow':'noindex, nofollow',
    'X-Content-Type-Options':'nosniff',
    'Referrer-Policy':'strict-origin-when-cross-origin',
  }});
}

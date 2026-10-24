// Isolated local preview. Never proxies credentials or changes shared application routing.
import {createServer} from 'node:http';
import {readFile, stat} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const root = fileURLToPath(new URL('.', import.meta.url));
const types = {'.html':'text/html; charset=utf-8','.css':'text/css','.js':'text/javascript','.mjs':'text/javascript','.webp':'image/webp','.woff2':'font/woff2','.glb':'model/gltf-binary','.wasm':'application/wasm','.json':'application/json'};
createServer(async (req,res) => {
  res.setHeader('X-Robots-Tag','noindex, nofollow');
  res.setHeader('X-Content-Type-Options','nosniff');
  res.setHeader('Referrer-Policy','strict-origin-when-cross-origin');
  if (!['GET','HEAD'].includes(req.method)) {res.writeHead(405);res.end();return;}
  try {
    let pathname = decodeURIComponent(new URL(req.url,'http://localhost').pathname);
    if (['/book-appointment','/book-appointment/','/my-bookings','/my-bookings/'].includes(pathname)) {
      res.writeHead(307,{Location:pathname.startsWith('/my-bookings')?'/account':'/booking'});res.end();return;
    }
    const slug=pathname.replace(/^\//,'').replace(/\/$/,'');
    if (/^[a-z0-9-]+$/.test(slug)) {
      try {await stat(path.join(root,'pages',slug+'.html'));pathname='/pages/'+slug+'.html';} catch {}
    }
    if (pathname.startsWith('/site/')) pathname=pathname.slice(5);
    if (['/account','/booking'].includes(pathname)) {
      res.writeHead(503,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store'});
      res.end('<!doctype html><html lang="en"><meta name="viewport" content="width=device-width"><title>Customer service connection</title><link rel="stylesheet" href="/studio.css"><main class="connection-page"><h1>Customer service connection</h1><p>This isolated website preview is not connected to the authenticated application. Account and booking routes must be served by the existing Next.js application on the same secure origin before launch.</p><p>No account, appointment or payment has been created.</p><a href="/#contact">Contact Dapper</a> · <a href="/">Back to the studio</a></main></html>');return;
    }
    const target = path.resolve(root, '.' + (pathname === '/' ? '/index.html' : pathname));
    const relative = path.relative(root,target);
    if (relative.startsWith('..') || path.isAbsolute(relative) || /(^|[\\/])\./.test(relative)) throw Error();
    if (!(await stat(target)).isFile()) throw Error();
    const body = await readFile(target);
    res.writeHead(200,{'Content-Type':types[path.extname(target)] || 'application/octet-stream','Cache-Control':'no-cache'});
    res.end(req.method === 'HEAD' ? undefined : body);
  } catch {res.writeHead(404,{'Content-Type':'text/plain'});res.end('Page not found');}
}).listen(4174,'127.0.0.1',()=>console.log('Website preview: http://127.0.0.1:4174'));

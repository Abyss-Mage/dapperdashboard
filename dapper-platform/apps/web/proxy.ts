import { NextResponse, type NextRequest } from 'next/server';

/* The dashboard and the workspace on their own subdomains (owner 2026-11-13; error404 #87): each subdomain's home opens its app, and a
   page that belongs to the other app moves to the other subdomain (same path and query), so a link to /workspace from the dashboard
   lands on workspace.…. The IP address and the main website are left alone. */
const WORKSPACE_PAGES = ['/workspace', '/shop', '/shop-clock'];
const DASHBOARD_PAGES = ['/dashboard'];

const under = (path: string, roots: string[]) => roots.some((r) => path === r || path.startsWith(r + '/'));

export function proxy(request: NextRequest) {
  const host = (request.headers.get('host') ?? '').toLowerCase();
  const sub = host.startsWith('dashboard.') ? 'dashboard' : host.startsWith('workspace.') ? 'workspace' : '';
  if (!sub) return NextResponse.next();
  const parent = host.slice(sub.length + 1);
  const { pathname, search } = request.nextUrl;
  if (pathname === '/') return NextResponse.redirect(new URL(sub === 'dashboard' ? '/dashboard' : '/workspace', `https://${host}`));
  if (sub === 'dashboard' && under(pathname, WORKSPACE_PAGES)) return NextResponse.redirect(new URL(pathname + search, `https://workspace.${parent}`));
  if (sub === 'workspace' && under(pathname, DASHBOARD_PAGES)) return NextResponse.redirect(new URL(pathname + search, `https://dashboard.${parent}`));
  return NextResponse.next();
}

export const config = { matcher: ['/', '/workspace/:path*', '/shop/:path*', '/shop-clock/:path*', '/dashboard/:path*'] };

import { NextResponse, type NextRequest } from 'next/server';

/* The dashboard and the workspace on their own subdomains (owner 2026-11-13): their home address opens the app itself. */
export function proxy(request: NextRequest) {
  const host = (request.headers.get('host') ?? '').toLowerCase();
  const app = host.startsWith('dashboard.') ? '/dashboard' : host.startsWith('workspace.') ? '/workspace' : '';
  if (!app) return NextResponse.next();
  return NextResponse.redirect(new URL(app, `https://${host}`));
}

export const config = { matcher: '/' };

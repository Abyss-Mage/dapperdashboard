import type { NextConfig } from 'next';
// The HTTPS application gateway owns /api routing. There is no JSON demo
// fallback or browser-configured upstream that could bypass authentication.
// Two areas: /dashboard (owners and administrators) and /workspace (staff). The old /owner and /owners addresses redirect.
const config: NextConfig = {
  poweredByHeader: false,
  async redirects() {
    return [
      { source: '/owner', destination: '/dashboard', permanent: true },
      { source: '/owners', destination: '/dashboard', permanent: true },
      { source: '/owner/:path*', destination: '/dashboard/:path*', permanent: true },
      { source: '/owners/:path*', destination: '/dashboard/:path*', permanent: true },
      // Public website addresses from the static-page era.
      { source: '/home', destination: '/', permanent: true },
      { source: '/my-bookings', destination: '/account', permanent: false },
      { source: '/booking', destination: '/book-appointment', permanent: true },
      { source: '/booking/:path*', destination: '/book-appointment/:path*', permanent: true },
    ];
  },
};
export default config;

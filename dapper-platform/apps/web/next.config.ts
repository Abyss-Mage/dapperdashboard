import type { NextConfig } from 'next';
// The HTTPS application gateway owns /api routing. There is no JSON demo
// fallback or browser-configured upstream that could bypass authentication.
const config: NextConfig = { poweredByHeader: false };
export default config;

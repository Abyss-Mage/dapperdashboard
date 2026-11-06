/* The slug of the website entry the public site reads from the backend (Website images in the dashboard creates it). One place, so the
   catalogue, portfolio, dealers and booking all read the same entry. Override with NEXT_PUBLIC_DAPPER_SITE_SLUG at build time. */
export const SITE_SLUG = process.env.NEXT_PUBLIC_DAPPER_SITE_SLUG || 'dapper-detailing-pros-peterborough';

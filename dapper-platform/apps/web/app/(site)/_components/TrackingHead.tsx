/* The owner's Google Tag Manager container and every-page custom scripts, written into the page's HTML by the server (error404 #130),
   so Google's "test your website" check and tag assistants find them in the page source. Tracking (in the browser) then skips what is
   already here and still handles the Meta Pixel, Mailchimp and page-specific snippets. Read from the API on this machine, refreshed
   every minute; if it cannot be read, Tracking loads everything in the browser as before. */
type Snippet = { id: string; placement: 'head' | 'body'; code: string; paths: string[] };
type Settings = { gtmId?: string; scripts?: Snippet[] };

const GTM = /^GTM-[A-Z0-9]{4,12}$/;

// The script tags of a snippet: their attributes and inline code. Other markup is left to Tracking.
function scriptsOf(code: string) {
  const out: { attrs: Record<string, string>; body: string }[] = [];
  for (const m of code.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)) {
    const attrs: Record<string, string> = {};
    for (const a of m[1].matchAll(/([a-zA-Z_:][-a-zA-Z0-9_:.]*)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g)) {
      attrs[a[1].toLowerCase()] = a[2] ?? a[3] ?? a[4] ?? '';
    }
    out.push({ attrs, body: m[2] });
  }
  return out;
}

export default async function TrackingHead() {
  let s: Settings | null = null;
  try {
    const r = await fetch('http://127.0.0.1:8121/api/v1/public/tracking', { next: { revalidate: 60 }, headers: { Accept: 'application/json' } });
    if (r.ok) s = (await r.json()) as Settings;
  } catch { /* Tracking loads them in the browser */ }
  if (!s) return null;
  const gtm = s.gtmId && GTM.test(s.gtmId) ? s.gtmId : '';
  // A pasted copy of the same Tag Manager container is skipped, so GTM loads once and visits are not counted twice.
  const everyPage = (s.scripts ?? []).filter((sn) => sn.paths.length === 0 && !(gtm && sn.code.includes(gtm)));
  return (
    <>
      {gtm && <script id="dd-gtm-inline" dangerouslySetInnerHTML={{ __html: `(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src='https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);})(window,document,'script','dataLayer',${JSON.stringify(gtm)});` }} />}
      {everyPage.flatMap((sn) => scriptsOf(sn.code).map((sc, i) => {
        const { src, async: isAsync, defer, type, id } = sc.attrs;
        const common = { key: sn.id + i, 'data-dd-custom': sn.id, ...(id ? { id } : {}), ...(type ? { type } : {}) };
        return src
          ? <script {...common} src={src} async={isAsync !== undefined} defer={defer !== undefined} />
          : <script {...common} dangerouslySetInnerHTML={{ __html: sc.body }} />;
      }))}
    </>
  );
}

// The GTM fallback for visitors without JavaScript, right after <body> opens, as Google asks.
export async function TrackingNoscript() {
  let id = '';
  try {
    const r = await fetch('http://127.0.0.1:8121/api/v1/public/tracking', { next: { revalidate: 60 }, headers: { Accept: 'application/json' } });
    if (r.ok) id = ((await r.json()) as Settings).gtmId ?? '';
  } catch { /* none */ }
  if (!GTM.test(id)) return null;
  return <noscript><iframe src={'https://www.googletagmanager.com/ns.html?id=' + encodeURIComponent(id)} height="0" width="0" style={{ display: 'none', visibility: 'hidden' }} /></noscript>;
}

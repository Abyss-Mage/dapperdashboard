'use client';
import { usePublished } from '../_components/usePublished';

// The packages the Dapper team has published for one category. Prices and times live in the customer account.
export default function Packages3({ category }: { category: string }) {
  const { status, services, retry } = usePublished();
  const items = services.filter((s) => s.category === category);
  if (status === 'loading') return <p className="d-note" role="status">Loading packages…</p>;
  if (status === 'error') return <div className="d-note" role="status"><span>Current packages could not be loaded. Contact the team or try again.</span><button type="button" className="d-btn d-btn--dark d-btn--sm" onClick={retry}>Try again</button></div>;
  if (!items.length) return <div className="d-note" role="status"><span>Contact the team for current packages in this category, or book and we’ll help you choose.</span><a className="d-btn d-btn--red d-btn--sm" href="/booking">Book now</a></div>;
  return (
    <div className="d-packages" aria-live="polite">
      {items.map((s) => <article key={s.name} className="d-package"><h3>{s.name}</h3><p>{s.summary}</p><a className="d-link" href="/account">View options</a></article>)}
    </div>
  );
}

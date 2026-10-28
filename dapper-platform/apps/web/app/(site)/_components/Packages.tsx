'use client';
import { usePublished } from './usePublished';

// The packages the Dapper team has published for one category. Prices and times live in the customer account.
export default function Packages({ category }: { category: string }) {
  const { status, services, retry } = usePublished();
  const items = services.filter((s) => s.category === category);
  if (status === 'loading') return <p className="status" role="status">Loading packages…</p>;
  if (status === 'error') return <div className="state" role="status"><p>Current packages could not be loaded. Contact the team or try again.</p><button type="button" className="btn btn--tape" onClick={retry}>Try again</button></div>;
  if (!items.length) return <div className="state" role="status"><p>Contact the team for current packages in this category.</p></div>;
  return (
    <div className="packages" aria-live="polite">
      {items.map((s) => (
        <article key={s.name} className="package">
          <h3>{s.name}</h3>
          <p>{s.summary}</p>
          <a href="/account">View options ↗</a>
        </article>
      ))}
    </div>
  );
}

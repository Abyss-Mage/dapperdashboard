import Link from 'next/link';
import { notFound } from 'next/navigation';
import { categories } from '../content';
import { CallToBook, PageHero, PageMotion, ProcessStrip, Reviews, SectionHead } from '../_v3/blocks';
import Packages3 from '../_v3/Packages3';

export const dynamicParams = false;
export function generateStaticParams() { return categories.map((c) => ({ service: c.slug })); }

export async function generateMetadata({ params }: { params: Promise<{ service: string }> }) {
  const { service } = await params;
  const c = categories.find((x) => x.slug === service);
  return c ? { title: c.title, description: c.summary, alternates: { canonical: '/' + c.slug } } : {};
}

export default async function ServicePage({ params }: { params: Promise<{ service: string }> }) {
  const { service } = await params;
  const c = categories.find((x) => x.slug === service);
  if (!c) notFound();
  return (
    <PageMotion>
      <PageHero chip="Our services" title={c.title} lead={c.summary} image={c.image} alt={c.photo ? c.title + ' at Dapper' : 'Vehicle detailed by Dapper'}>
        {c.from ? <p className="d-price-tag">Starting from <b>${c.from}</b></p> : null}
        <a className="d-btn d-btn--red" href="/booking">Book now</a>
      </PageHero>
      <section className="d-section">
        <nav className="d-tabs" aria-label="Service categories">
          {categories.map((x) => <Link key={x.slug} href={'/' + x.slug} aria-current={x.slug === c.slug ? 'page' : undefined}>{x.title}</Link>)}
        </nav>
        <SectionHead chip="Packages" title="Find your care" text="Packages are published by the Dapper team. Sign in to choose one for your own vehicle and request a time." />
        <Packages3 category={c.key} />
      </section>
      <ProcessStrip />
      <Reviews />
      <CallToBook />
    </PageMotion>
  );
}

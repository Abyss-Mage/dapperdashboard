import { notFound } from 'next/navigation';
import { categories } from '../content';
import SplitHero from '../_components/SplitHero';
import Packages from '../_components/Packages';
import CategoryNav from '../_components/CategoryNav';
import Reveal from '../_components/Reveal';
import { Close } from '../_components/Chrome';
import { ArrowUpRight } from '../_components/Icons';

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
    <>
      <SplitHero image={c.image} studioPhoto={!c.photo} alt={c.photo ? c.title + " at the Dapper studio" : "Inside the Dapper studio, Peterborough"} before="Road-worn." after={c.title + '.'} lead={c.summary}>
        <a className="btn btn--red" href="/booking">Find an appointment<ArrowUpRight size={20} aria-hidden /></a>
      </SplitHero>
      <Reveal className="section" aria-labelledby="care-title">
        <div className="wrap cat-layout">
          <CategoryNav current={c.slug} />
          <div>
            <h2 id="care-title" className="headline" data-split>Find your<br /><span className="after">care.</span></h2>
            <p className="lede" data-rise>Packages are published by the Dapper team. Sign in to choose one for your own vehicle and request a time.</p>
            <Packages category={c.key} />
            <p style={{ marginTop: 32 }} data-rise><a className="btn btn--line" href="/account">Sign in to your account<ArrowUpRight size={20} aria-hidden /></a></p>
          </div>
        </div>
      </Reveal>
      <Close />
    </>
  );
}

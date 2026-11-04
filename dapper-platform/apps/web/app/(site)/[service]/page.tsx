import { notFound } from 'next/navigation';
import '../paper.css';
import '../services/services.css';
import './vehicle.css';
import { categories } from '../content';
import VehiclePage from './VehiclePage';

export const dynamicParams = false;
export function generateStaticParams() { return categories.map((c) => ({ service: c.slug })); }

export async function generateMetadata({ params }: { params: Promise<{ service: string }> }) {
  const { service } = await params;
  const c = categories.find((x) => x.slug === service);
  return c ? { title: c.title, description: c.summary, alternates: { canonical: '/' + c.slug } } : {};
}

export default async function ServicePage({ params }: { params: Promise<{ service: string }> }) {
  const { service } = await params;
  if (!categories.some((x) => x.slug === service)) notFound();
  return <VehiclePage slug={service} />;
}

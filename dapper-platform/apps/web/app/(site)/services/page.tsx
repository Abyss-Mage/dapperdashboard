import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { categories } from '../content';
import { servicesIntro } from '../dapper';
import { CallToBook, PageHero, PageMotion, SectionHead, WhyGrid } from '../_v3/blocks';

export const metadata = { title: 'Services', description: 'Detailing, ceramic coating and paint protection film for every vehicle, in Peterborough.', alternates: { canonical: '/services' } };

export default function Services() {
  return (
    <PageMotion>
      <PageHero chip="Our services" title={<>Detailing for <em>every vehicle.</em></>} lead={servicesIntro.text} image="/site/assets/services/suv.webp" alt="A black SUV in the Dapper studio" />
      <section className="d-section">
        <SectionHead chip="Choose your vehicle" title="Find the right care" />
        <div className="d-services">
          {categories.map((c) => (
            <Link key={c.slug} href={'/' + c.slug} className="d-service" data-d-in>
              <div className="d-service__img"><img src={c.image} alt={c.photo ? c.title : 'Vehicle detailed by Dapper'} loading="lazy" />{c.from ? <span className="d-price">Starting from <b>${c.from}</b></span> : null}</div>
              <div className="d-service__body"><h3>{c.title}</h3><p>{c.summary}</p><span className="d-link">Check now<ArrowRight size={16} /></span></div>
            </Link>
          ))}
        </div>
      </section>
      <WhyGrid />
      <CallToBook />
    </PageMotion>
  );
}

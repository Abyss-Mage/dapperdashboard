'use client';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight, Award, CalendarCheck, CalendarClock, Car, CheckCircle2, ClipboardList, FlaskConical, Phone, Play, ShieldCheck, Sparkles, Star, Wrench, X } from 'lucide-react';
import ClearCoat from '../_components/ClearCoat';
import { BookOnline, ContactForm, GlossFilm, MobileDetailing, ReadyBand } from './more';
import { gsap, ScrollTrigger, SplitText, useGSAP } from '../_components/gsap';
import { contact, cta, partners, processSteps, proof, services, servicesIntro, testimonials, transformations, values, why } from '../dapper';

/* Dapper home page: Delta-style dark/red panels on a warm off-white page, with Atelier-style polish (rounded stages, soft
   depth, a clear step strip, an app-like dock on phones). Every word, photo and film is Dapper's own. */

const whyIcons = [Award, FlaskConical, Car, ClipboardList, Wrench, CalendarClock];
const stepIcons = [CalendarCheck, ShieldCheck, Sparkles, CheckCircle2];
const words = ['Sedan detailing', 'SUV detailing', 'Pickup detailing', 'Ceramic coating', 'Paint protection film', 'Aircraft detailing', 'Boat detailing'];

const Stars = ({ size = 16 }: { size?: number }) => <span className="d-stars" aria-label="5 stars">{[0, 1, 2, 3, 4].map((i) => <Star key={i} size={size} fill="currentColor" strokeWidth={0} />)}</span>;

export default function Home3() {
  const root = useRef<HTMLDivElement>(null);
  const [word, setWord] = useState(0);
  const [film, setFilm] = useState<string | null>(null);

  // The service names in "Core values" take turns being the highlighted one.
  useEffect(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const id = setInterval(() => setWord((w) => (w + 1) % words.length), 1800);
    return () => clearInterval(id);
  }, []);

  useGSAP(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const h1 = SplitText.create('.d-hero h1', { type: 'lines', mask: 'lines' });
    gsap.timeline({ defaults: { ease: 'expo.out' } })
      .from('.d-hero', { scale: .96, borderRadius: 48, duration: 1.4 }, 0)
      .from(h1.lines, { yPercent: 105, duration: 1.1, stagger: .09 }, .25)
      .from('.d-hero__lead, .d-hero__actions > *, .d-hero__trust', { y: 18, opacity: 0, stagger: .07, duration: .9 }, .7)
      .from('.d-hero__card', { y: 30, opacity: 0, duration: 1 }, 1);

    gsap.utils.toArray<HTMLElement>('[data-d-head]').forEach((el) => {
      const s = SplitText.create(el, { type: 'lines', mask: 'lines' });
      gsap.from(s.lines, { yPercent: 105, duration: 1.1, stagger: .08, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 88%', once: true } });
    });
    ScrollTrigger.batch('[data-d-in]', { start: 'top 90%', once: true, onEnter: (els) => gsap.from(els, { y: 40, opacity: 0, duration: 1, stagger: .08, ease: 'expo.out' }) });
    // the step line draws itself across as the process strip comes into view
    gsap.from('.d-steps__line i', { scaleX: 0, transformOrigin: 'left', ease: 'none', scrollTrigger: { trigger: '.d-steps', start: 'top 75%', end: 'bottom 60%', scrub: true } });
    // dark panels lift slightly as they arrive
    gsap.utils.toArray<HTMLElement>('.d-panel').forEach((el) => gsap.from(el, { scale: .97, duration: 1.2, ease: 'expo.out', scrollTrigger: { trigger: el, start: 'top 92%', once: true } }));
  }, { scope: root });

  return (
    <div ref={root} className="d-home">
      {/* Hero */}
      <section className="d-hero" aria-labelledby="d-hero-title">
        <ClearCoat src="/site/assets/services/suv.webp" className="d-hero__gl" />
        <div className="d-hero__shade" />
        <div className="d-hero__content">
          <p className="d-chip"><span className="d-dot" />Peterborough · Detailing &amp; ceramic coating</p>
          <h1 id="d-hero-title">Detailing that<br />speaks for <em>itself.</em></h1>
          <p className="d-hero__lead">Detailing, ceramic coating, paint protection film and window tint at 660 The Kingsway, Peterborough. Book now or contact us to find the right care for your vehicle.</p>
          <div className="d-hero__actions">
            <a className="d-btn d-btn--red" href="/book-appointment">Book now<ArrowRight size={18} /></a>
            <Link className="d-btn d-btn--glass" href="/contact-us">Contact us</Link>
          </div>
          <p className="d-hero__trust"><Stars /><b>5.0</b><span>{proof.customers}</span></p>
        </div>
        <div className="d-hero__card" aria-label="Call or book">
          <p><b>Book or call us</b><span>We’ll come to you — no waiting rooms.</span></p>
          <a className="d-btn d-btn--dark" href={'tel:' + contact.tel}><Phone size={16} />{contact.phone}</a>
        </div>
      </section>

      {/* Core values */}
      <section className="d-values" aria-labelledby="d-values-title">
        <div className="d-values__text">
          <p className="d-chip d-chip--light"><span className="d-dot" />Our core values</p>
          <h2 id="d-values-title" data-d-head>{values.heading}</h2>
          <p data-d-in>{values.text}</p>
          <a className="d-btn d-btn--red" href="/book-appointment" data-d-in>Book your detail<ArrowRight size={18} /></a>
        </div>
        <ul className="d-words" aria-label="What we detail">
          {words.map((w, i) => <li key={w} className={i === word ? 'is-on' : undefined}>{w}</li>)}
        </ul>
      </section>

      {/* Services */}
      <section className="d-section" id="services" aria-labelledby="d-services-title">
        <div className="d-head">
          <p className="d-chip d-chip--light"><span className="d-dot" />Our services</p>
          <h2 id="d-services-title" data-d-head>{servicesIntro.heading}</h2>
          <p data-d-in>{servicesIntro.text}</p>
        </div>
        <div className="d-services">
          {services.map((s) => (
            <Link key={s.slug} href={'/' + s.slug} className="d-service" data-d-in>
              <div className="d-service__img"><img src={s.image} alt={s.name} loading="lazy" />{s.from ? <span className="d-price">Starting from <b>${s.from}</b></span> : null}</div>
              <div className="d-service__body"><h3>{s.name}</h3><p>{s.text}</p><span className="d-link">Check now<ArrowRight size={16} /></span></div>
            </Link>
          ))}
        </div>
      </section>

      {/* Partners */}
      <section className="d-section d-section--tight" aria-labelledby="d-partners-title">
        <div className="d-head d-head--center">
          <p className="d-chip d-chip--light"><span className="d-dot" />Our partners</p>
          <h2 id="d-partners-title" data-d-head>Our trusted commercial partners</h2>
        </div>
        <ul className="d-partners">{partners.map((p) => <li key={p.name} data-d-in>{p.src ? <img src={p.src} alt={p.name} /> : <span className="d-partners__name">{p.name}</span>}</li>)}</ul>
      </section>

      {/* Transformations */}
      <section className="d-panel" id="work" aria-labelledby="d-trans-title">
        <div className="d-head d-head--center">
          <p className="d-chip"><span className="d-dot" />Transformations</p>
          <h2 id="d-trans-title" data-d-head>{transformations.heading}</h2>
          <p data-d-in>{transformations.text}</p>
          <p className="d-rating" data-d-in><Stars /><b>5☆</b>Google Ratings</p>
        </div>
        <div className="d-reels">
          {transformations.clips.map((c, i) => (
            <button key={c} type="button" className="d-reel" onClick={() => setFilm(c)} aria-label={'Play transformation ' + (i + 1)} data-d-in>
              <video src={c} muted loop playsInline autoPlay preload="metadata" />
              <span className="d-reel__play"><Play size={18} fill="currentColor" /></span>
            </button>
          ))}
        </div>
        <div className="d-center"><Link className="d-btn d-btn--red" href="/portfolio">Explore portfolio<ArrowRight size={18} /></Link></div>
        {film && (
          <div className="d-film" role="dialog" aria-modal="true" aria-label="Transformation video" onClick={() => setFilm(null)}>
            <video src={film} autoPlay loop playsInline controls onClick={(e) => e.stopPropagation()} />
            <button type="button" className="d-film__close" onClick={() => setFilm(null)} aria-label="Close"><X size={22} /></button>
          </div>
        )}
      </section>

      {/* Why choose us */}
      <section className="d-section" aria-labelledby="d-why-title">
        <div className="d-head d-head--center">
          <p className="d-chip d-chip--light"><span className="d-dot" />Why choose us</p>
          <h2 id="d-why-title" data-d-head>Why vehicle owners trust Dapper</h2>
        </div>
        <div className="d-why">
          {why.map((w, i) => { const Icon = whyIcons[i]; return (
            <article key={w.title} data-d-in><span className="d-icon"><Icon size={22} /></span><h3>{w.title}</h3><p>{w.text}</p></article>
          ); })}
        </div>
      </section>

      <ReadyBand />
      <BookOnline />
      <MobileDetailing />

      {/* Process */}
      <section className="d-panel d-panel--soft" id="process" aria-labelledby="d-process-title">
        <div className="d-head d-head--center">
          <p className="d-chip"><span className="d-dot" />Our process</p>
          <h2 id="d-process-title" data-d-head>How we do it</h2>
        </div>
        <div className="d-steps">
          <div className="d-steps__line" aria-hidden="true"><i /></div>
          {processSteps.map((p, i) => { const Icon = stepIcons[i]; return (
            <article key={p.title} data-d-in>
              <span className="d-steps__n"><Icon size={22} /><b>{i + 1}</b></span>
              <h3>{p.title}</h3><p>{p.text}</p>
            </article>
          ); })}
        </div>
      </section>

      {/* Testimonials */}
      <section className="d-section d-reviews" aria-labelledby="d-reviews-title">
        <div className="d-head d-head--center">
          <p className="d-chip d-chip--light"><span className="d-dot" />Testimonials</p>
          <h2 id="d-reviews-title" data-d-head>What our clients say</h2>
          <p className="d-rating d-rating--dark" data-d-in><Stars /><b>5.0</b>{proof.customers}</p>
        </div>
        {[0, 1].map((row) => (
          <div key={row} className={'d-marquee' + (row ? ' d-marquee--rev' : '')}>
            <div className="d-marquee__track">
              {[...testimonials.slice(row * 4, row * 4 + 4), ...testimonials.slice(row * 4, row * 4 + 4)].map((t, i) => (
                <figure key={t.name + i} className="d-review" aria-hidden={i >= 4}>
                  <Stars size={14} />
                  <blockquote>“{t.quote}”</blockquote>
                  <figcaption><span className="d-avatar">{t.name.charAt(0).toUpperCase()}</span>{t.name}</figcaption>
                </figure>
              ))}
            </div>
          </div>
        ))}
      </section>

      <GlossFilm />

      {/* Call to book */}
      <section className="d-cta" aria-labelledby="d-cta-title">
        <img src="/site/assets/portfolio/portfolio-06.jpg" alt="" loading="lazy" />
        <div className="d-cta__content">
          <h2 id="d-cta-title" data-d-head>{cta.heading}</h2>
          <p>{cta.text}</p>
          <div className="d-hero__actions"><a className="d-btn d-btn--red" href="/book-appointment">Book now<ArrowRight size={18} /></a><a className="d-btn d-btn--glass" href={'tel:' + contact.tel}><Phone size={16} />{contact.phone}</a></div>
        </div>
      </section>
      <ContactForm />
    </div>
  );
}

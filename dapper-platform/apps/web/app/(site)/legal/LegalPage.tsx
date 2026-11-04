import Link from 'next/link';
import { Fragment } from 'react';
import { contact } from '../dapper';
import { legal, type LegalBlock } from '../legal';

/* A legal page on paper: a heading, then the contents beside the text. The wording is carried over from the published site. */

const link = /([\w.+-]+@[\w-]+\.[\w.-]+|https?:\/\/[^\s)]+)/g;
function Text({ s }: { s: string }) {
  return <>{s.split(link).map((part, i) => i % 2 === 1
    ? <a key={i} href={part.includes('@') ? 'mailto:' + part : part} {...(part.includes('@') ? {} : { target: '_blank', rel: 'noopener noreferrer' })}>{part}</a>
    : <Fragment key={i}>{part}</Fragment>)}</>;
}
const isSub = (b: LegalBlock, i: number) => 'p' in b && i > 0 && !b.p.includes('\n') && b.p.length <= 60 && !/[.:;,]$/.test(b.p);
const slug = (h: string) => 'l-' + h.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

export default function LegalPage({ page }: { page: keyof typeof legal }) {
  const d = legal[page];
  return (
    <div className="c-page l-page">
      <header className="l-head">
        <p className="l-crumb"><Link href="/">Home</Link><span aria-hidden="true">/</span>Legal</p>
        <h1>{d.title}</h1>
        <p className="l-lede">{d.lede}</p>
        <p className="l-eff">Effective {d.effective}</p>
      </header>
      <div className="l-body">
        <nav className="l-toc" aria-label="On this page">
          <ol>{d.sections.map((s) => <li key={s.h}><a href={'#' + slug(s.h)}>{s.h.replace(/^\d+\.\s*/, '')}</a></li>)}</ol>
          <p className="l-other">
            {(Object.keys(legal) as (keyof typeof legal)[]).filter((k) => k !== page).map((k) => <Link key={k} href={'/' + k}>{legal[k].title}</Link>)}
          </p>
        </nav>
        <article className="l-text">
          {d.intro.map((t) => <p key={t}><Text s={t} /></p>)}
          {d.sections.map((s) => (
            <section key={s.h} id={slug(s.h)} aria-labelledby={slug(s.h) + '-h'}>
              <h2 id={slug(s.h) + '-h'}>{s.h.replace(/^\d+\.\s*/, '')}</h2>
              {s.blocks.map((b, i) => 'ul' in b
                ? <ul key={i}>{b.ul.map((x) => <li key={x}><Text s={x} /></li>)}</ul>
                : isSub(b, i) ? <h3 key={i}>{b.p}</h3> : <p key={i}><Text s={b.p} /></p>)}
            </section>
          ))}
          <p className="l-end">Questions about this page? Email <a href={'mailto:' + contact.email}>{contact.email}</a> or call <a href={'tel:' + contact.tel}>+1 {contact.phone}</a>.</p>
        </article>
      </div>
    </div>
  );
}

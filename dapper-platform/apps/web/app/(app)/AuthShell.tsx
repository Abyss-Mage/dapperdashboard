import '@fontsource-variable/unbounded/index.css';
import ThemeToggle from './ThemeToggle';
import styles from './auth.module.css';

/* The shared look of every sign-in page: on a wide screen the left half is the film's last frame with the brand and a line (the
   website's look), the right half is the form; on a phone the picture is a short banner above the form. The form itself is
   passed in unchanged, so each page keeps its own sign-in logic. */
export default function AuthShell({ kind, title, lead, children }: { kind: string; title: string; lead?: string; children: React.ReactNode }) {
  return (
    <div className={styles.wrap}>
      <aside className={styles.art} aria-hidden="true">
        <a href="/" className={styles.brand} tabIndex={-1}>Dapper</a>
        <p className={styles.line}>Detailing that <em>speaks for itself.</em></p>
      </aside>
      <main className={styles.panel}>
        <div className={styles.top}>
          <a href="/" className={styles.back}>← Back to the website</a>
          <ThemeToggle compact className={styles.theme} />
        </div>
        <section className={styles.card} aria-labelledby="auth-title">
          <p className={styles.kind}>{kind}</p>
          <h1 id="auth-title">{title}</h1>
          {lead ? <p className={styles.lead}>{lead}</p> : null}
          <div className={styles.body}>{children}</div>
        </section>
        <p className={styles.foot}>© 2026 Dapper Detailing Pros Ltd.</p>
      </main>
    </div>
  );
}

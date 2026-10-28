'use client';

import { useEffect, useState } from 'react';

type PopupFormField = {
  name: string;
  type: string;
  label: string;
  required: boolean;
};

type PopupForm = {
  id: string;
  organizationId: string;
  locationId: string;
  name: string;
  title: string;
  description: string;
  headline: string;
  fields: PopupFormField[];
  displayDelaySeconds: number;
  selectedPages: string[];
  repeatDisplayLimitDays: number;
  successMessage: string;
  active: boolean;
};

export default function MarketingPopupModal({
  organizationId,
  locationId,
}: {
  organizationId?: string;
  locationId?: string;
}) {
  const [activePopup, setActivePopup] = useState<PopupForm | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [formData, setFormData] = useState<Record<string, string>>({});
  const [consentGiven, setConsentGiven] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!organizationId || !locationId) return;

    let timeoutId: NodeJS.Timeout;
    const currentPath = window.location.pathname || '/';

    async function checkForPopups() {
      try {
        const res = await fetch(
          `/api/v1/public/marketing/popups?organizationId=${organizationId}&locationId=${locationId}&path=${encodeURIComponent(currentPath)}`
        );
        if (!res.ok) return;
        const popups: PopupForm[] = await res.json();
        if (!Array.isArray(popups) || popups.length === 0) return;

        // Find the first popup that has not been dismissed within repeatDisplayLimitDays
        for (const popup of popups) {
          const dismissedAt = localStorage.getItem(`dapper_popup_dismissed_${popup.id}`);
          if (dismissedAt) {
            const daysSince = (Date.now() - Number(dismissedAt)) / (1000 * 60 * 60 * 24);
            if (daysSince < (popup.repeatDisplayLimitDays || 7)) {
              continue; // In cooldown period
            }
          }

          // Trigger display after configured delay
          const delayMs = (popup.displayDelaySeconds || 5) * 1000;
          timeoutId = setTimeout(() => {
            setActivePopup(popup);
            setIsOpen(true);
          }, delayMs);
          break;
        }
      } catch {
        // Public popups fail quietly without disrupting the user experience
      }
    }

    checkForPopups();

    return () => {
      if (timeoutId) clearTimeout(timeoutId);
    };
  }, [organizationId, locationId]);

  const handleDismiss = () => {
    if (activePopup) {
      localStorage.setItem(`dapper_popup_dismissed_${activePopup.id}`, String(Date.now()));
    }
    setIsOpen(false);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activePopup || !consentGiven) return;
    setSubmitting(true);
    setError('');

    try {
      const email = formData['email'] || '';
      const firstName = formData['first_name'] || '';
      const lastName = formData['last_name'] || '';

      const res = await fetch(`/api/v1/public/marketing/popups/${activePopup.id}/submit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email,
          firstName,
          lastName,
          metadata: {
            source_path: window.location.pathname,
            referrer: document.referrer || '',
          },
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to submit form.');
      }

      setSubmitted(true);
      localStorage.setItem(`dapper_popup_dismissed_${activePopup.id}`, String(Date.now()));

      // Auto close after 3 seconds
      setTimeout(() => {
        setIsOpen(false);
      }, 3000);
    } catch (err: unknown) {
      if (err instanceof Error) setError(err.message);
      else setError('Submission failed.');
    } finally {
      setSubmitting(false);
    }
  };

  if (!isOpen || !activePopup) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="popup-title"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        padding: '1rem',
      }}
    >
      <div
        style={{
          background: 'var(--card-bg, #ffffff)',
          color: 'var(--text-main, #1a0b0d)',
          maxWidth: '460px',
          width: '100%',
          borderRadius: '10px',
          padding: '2rem',
          position: 'relative',
          boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.1)',
        }}
      >
        <button
          type="button"
          onClick={handleDismiss}
          aria-label="Close dialog"
          style={{
            position: 'absolute',
            top: '0.75rem',
            right: '0.75rem',
            background: 'transparent',
            border: 'none',
            fontSize: '1.25rem',
            cursor: 'pointer',
            padding: '0.25rem 0.5rem',
            color: '#6b5558',
          }}
        >
          ✕
        </button>

        {submitted ? (
          <div style={{ textAlign: 'center', padding: '1rem 0' }}>
            <div style={{ fontSize: '2.5rem', marginBottom: '0.5rem', color: '#15804f' }}>✓</div>
            <h3 style={{ margin: '0 0 0.5rem 0' }}>Success!</h3>
            <p style={{ margin: 0, color: '#2b2023' }}>{activePopup.successMessage}</p>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            {activePopup.headline && (
              <span style={{ fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: '#d10f1f', fontWeight: 'bold' }}>
                {activePopup.headline}
              </span>
            )}
            <h2 id="popup-title" style={{ margin: '0.25rem 0 0.5rem 0', fontSize: '1.4rem' }}>
              {activePopup.title}
            </h2>
            {activePopup.description && (
              <p style={{ margin: '0 0 1.25rem 0', fontSize: '0.9rem', color: '#6b5558', lineHeight: '1.4' }}>
                {activePopup.description}
              </p>
            )}

            {error && (
              <div style={{ color: '#a30b18', background: '#ffe3e5', padding: '0.4rem', borderRadius: '4px', marginBottom: '0.75rem', fontSize: '0.85rem' }}>
                {error}
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', marginBottom: '1rem' }}>
              {(activePopup.fields || []).map((field) => (
                <div key={field.name}>
                  <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.25rem' }}>
                    {field.label} {field.required && <span style={{ color: '#a30b18' }}>*</span>}
                  </label>
                  <input
                    type={field.type || 'text'}
                    required={field.required}
                    value={formData[field.name] || ''}
                    onChange={(e) => setFormData({ ...formData, [field.name]: e.target.value })}
                    style={{
                      width: '100%',
                      padding: '0.5rem',
                      borderRadius: '4px',
                      border: '1px solid #f0dcde',
                      fontSize: '0.9rem',
                    }}
                  />
                </div>
              ))}
            </div>

            <div style={{ marginBottom: '1.25rem' }}>
              <label style={{ display: 'flex', alignItems: 'flex-start', gap: '0.5rem', fontSize: '0.75rem', color: '#6b5558' }}>
                <input
                  type="checkbox"
                  required
                  checked={consentGiven}
                  onChange={(e) => setConsentGiven(e.target.checked)}
                  style={{ marginTop: '0.15rem' }}
                />
                <span>
                  I agree to receive promotional emails, seasonal detailing specials, and vehicle care updates from Dapper Detailing. You may unsubscribe at any time.
                </span>
              </label>
            </div>

            <button
              type="submit"
              disabled={submitting || !consentGiven}
              style={{
                width: '100%',
                padding: '0.65rem',
                background: '#d10f1f',
                color: '#ffffff',
                border: 'none',
                borderRadius: '6px',
                fontWeight: 'bold',
                fontSize: '0.95rem',
                cursor: submitting ? 'wait' : 'pointer',
              }}
            >
              {submitting ? 'Subscribing...' : 'Claim Offer & Subscribe'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

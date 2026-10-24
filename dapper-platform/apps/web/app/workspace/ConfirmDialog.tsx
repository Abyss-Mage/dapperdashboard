'use client';
import { useEffect, useRef } from 'react';

export type ConfirmDialogProps = {
  isOpen: boolean;
  title: string;
  entityName: string;
  entityType: string;
  actionType: 'delete' | 'archive';
  explanation: string;
  impactNotice?: string;
  isBusy?: boolean;
  confirmLabel?: string;
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
};

export default function ConfirmDialog({
  isOpen,
  title,
  entityName,
  entityType,
  actionType,
  explanation,
  impactNotice,
  isBusy = false,
  confirmLabel,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const cancelBtnRef = useRef<HTMLButtonElement>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (isOpen) {
      previouslyFocusedRef.current = document.activeElement as HTMLElement | null;
      // Safe initial focus on Cancel to prevent accidental destruction
      const timer = setTimeout(() => {
        cancelBtnRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    } else if (previouslyFocusedRef.current) {
      // Return focus to triggering element on close
      previouslyFocusedRef.current.focus();
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape' && !isBusy) {
        e.preventDefault();
        onCancel();
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, isBusy, onCancel]);

  if (!isOpen) return null;

  const defaultConfirmLabel = actionType === 'delete' ? `Delete ${entityType}` : `Archive ${entityType}`;

  return (
    <div
      className="modal-overlay"
      role="presentation"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.75)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        padding: '1rem',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget && !isBusy) onCancel();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-desc"
        className="detail-card panel"
        style={{
          maxWidth: '520px',
          width: '100%',
          margin: 'auto',
          backgroundColor: '#17171b',
          color: '#ffffff',
          borderRadius: '8px',
          boxShadow: '0 8px 32px rgba(0, 0, 0, 0.5)',
          padding: '1.5rem',
        }}
      >
        <h2 id="confirm-dialog-title" style={{ marginTop: 0, fontSize: '1.25rem', color: '#f5f5f7' }}>
          {title}
        </h2>

        <div id="confirm-dialog-desc" style={{ marginTop: '0.75rem', fontSize: '0.9375rem', lineHeight: '1.5' }}>
          <p style={{ margin: '0 0 0.75rem 0' }}>
            Are you sure you want to {actionType === 'delete' ? 'delete' : 'archive'}{' '}
            <strong>{entityName}</strong>?
          </p>
          <p style={{ margin: '0 0 0.75rem 0', opacity: 0.9 }}>
            {explanation}
          </p>
          {impactNotice && (
            <div
              style={{
                margin: '0.75rem 0',
                padding: '0.75rem',
                backgroundColor: 'rgba(255, 107, 119, 0.15)',
                borderLeft: '3px solid #ff6b77',
                fontSize: '0.875rem',
                borderRadius: '4px',
              }}
            >
              <strong>Dependency notice:</strong> {impactNotice}
            </div>
          )}
        </div>

        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: '0.75rem',
            marginTop: '1.5rem',
          }}
        >
          <button
            ref={cancelBtnRef}
            type="button"
            className="secondary"
            onClick={onCancel}
            disabled={isBusy}
            style={{
              padding: '0.5rem 1rem',
              fontSize: '0.875rem',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            className="primary"
            onClick={onConfirm}
            disabled={isBusy}
            style={{
              padding: '0.5rem 1rem',
              fontSize: '0.875rem',
              backgroundColor: actionType === 'delete' ? '#d10f1f' : undefined,
              borderColor: actionType === 'delete' ? '#a30b18' : undefined,
              cursor: isBusy ? 'wait' : 'pointer',
            }}
          >
            {isBusy ? 'Processing…' : (confirmLabel || defaultConfirmLabel)}
          </button>
        </div>
      </div>
    </div>
  );
}

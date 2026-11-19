import React, { useRef } from 'react';
import { AlertCircle, CheckCircle2, Info, Loader2, TriangleAlert } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/ui/dialog';
import { cn } from '@/shared/lib/utils';

// Feature-local pieces for the admin console. They compose only shared primitives, status
// tokens and Lucide icons. Two things are missing from @/shared/components/ui and are flagged
// in the PR: an inline alert/callout primitive (Banner below) and a Button `isLoading` prop
// (Busy below, used inside a disabled Button).

export type Tone = 'info' | 'ok' | 'attn' | 'err';

const TONE_CLASS: Record<Tone, string> = {
  info: 'border-border bg-surface-subtle text-text',
  ok: 'border-status-success-border bg-status-success-bg text-status-success-ink',
  attn: 'border-status-warning-border bg-status-warning-bg text-status-warning-ink',
  err: 'border-status-error-border bg-status-error-bg text-status-error-ink',
};

const TONE_ICON: Record<Tone, React.ReactNode> = {
  info: <Info size={15} strokeWidth={2} aria-hidden="true" />,
  ok: <CheckCircle2 size={15} strokeWidth={2} aria-hidden="true" />,
  attn: <TriangleAlert size={15} strokeWidth={2} aria-hidden="true" />,
  err: <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />,
};

interface BannerProps {
  tone: Tone;
  children: React.ReactNode;
  /** Reference id from the server (correlation id), shown so support can find the request. */
  reference?: string;
  /** Optional action, for example a Retry button. */
  action?: React.ReactNode;
  testId?: string;
}

/** Inline message. Errors use role="alert"; everything else is a polite status. */
export const Banner: React.FC<BannerProps> = ({ tone, children, reference, action, testId }) => (
  <div
    role={tone === 'err' ? 'alert' : 'status'}
    data-testid={testId}
    className={cn('flex flex-col gap-2 rounded-sm border p-3 text-sm', TONE_CLASS[tone])}
  >
    <div className="flex items-start gap-2">
      <span className="mt-0.5 shrink-0">{TONE_ICON[tone]}</span>
      <span>{children}</span>
    </div>
    {reference && <div className="font-mono text-xs tabular-nums">Reference: {reference}</div>}
    {action}
  </div>
);

/** Spinner plus a visible verb, for a disabled Button while a request is pending. */
export const Busy: React.FC<{ label: string }> = ({ label }) => (
  <>
    <Loader2 size={15} strokeWidth={2} className="animate-spin" aria-hidden="true" />
    <span>{label}</span>
  </>
);

/** Small red message under a field. */
export const FieldError: React.FC<{ id?: string; children: React.ReactNode }> = ({ id, children }) => (
  <p id={id} className="text-xs text-status-error-ink">
    {children}
  </p>
);

/** Helper text under a field. */
export const FieldHint: React.FC<{ id?: string; children: React.ReactNode }> = ({ id, children }) => (
  <p id={id} className="text-xs text-text-muted">
    {children}
  </p>
);

interface AdminDialogProps {
  title: string;
  description?: string;
  onClose: () => void;
  /** While a request is pending, Escape, outside clicks and the close button do nothing. */
  busy?: boolean;
  children: React.ReactNode;
  testId?: string;
}

/**
 * Named dialog built on the shared Dialog primitive: it traps focus, closes on Escape
 * (unless busy) and returns focus to the control that opened it.
 */
export const AdminDialog: React.FC<AdminDialogProps> = ({ title, description, onClose, busy = false, children, testId }) => {
  // The shared Dialog returns focus only to a DialogTrigger. These dialogs are opened by state,
  // so remember the control that was focused when the dialog first rendered and refocus it on close.
  const opener = useRef<HTMLElement | null>(document.activeElement as HTMLElement | null);

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
    >
      <DialogContent
        data-testid={testId}
        showClose={!busy}
        onEscapeKeyDown={(e) => {
          if (busy) e.preventDefault();
        }}
        onInteractOutside={(e) => {
          if (busy) e.preventDefault();
        }}
        onCloseAutoFocus={(e) => {
          e.preventDefault();
          const el = opener.current;
          if (el && document.contains(el)) el.focus();
        }}
        className="max-h-screen overflow-y-auto"
        {...(description ? {} : { 'aria-describedby': undefined })}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );
};

/** Formats an ISO timestamp for Sri Lanka, for example "12 Nov 2026, 7:45 pm". */
export function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
    timeZone: 'Asia/Colombo',
  });
}

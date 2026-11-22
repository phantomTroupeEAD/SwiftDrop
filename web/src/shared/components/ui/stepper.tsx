import { Check } from 'lucide-react';
import { cn } from '@/shared/lib/utils';

// Connected step rail: done steps are ink with a tick, the current step has a signal ring,
// steps ahead are outlined and joined by a dashed line. Supports 3 or 4 steps; `label` names
// the list for screen readers (defaults to the booking wizard's name).
export function Stepper({ steps, current, label = 'Booking progress' }: { steps: string[]; current: number; label?: string }) {
  return <ol className={cn('mb-5 grid gap-2', steps.length === 3 ? 'grid-cols-3' : 'grid-cols-4')} aria-label={label}>
    {steps.map((step, index) => {
      const done = index < current;
      const active = index === current;
      return <li key={step} aria-current={active ? 'step' : undefined} data-completed={done}
        className={cn('flex flex-col items-center gap-1.5 text-center text-xs sm:flex-row sm:gap-2 sm:text-left sm:text-sm',
          active ? 'font-semibold text-text' : done ? 'font-medium text-text' : 'font-medium text-text-muted')}>
        <span aria-hidden="true" className={cn('grid size-7 shrink-0 place-items-center rounded-full font-mono text-[13px]',
          done ? 'bg-chrome text-chrome-foreground' : active ? 'border-2 border-primary bg-surface text-text' : 'border border-border-strong bg-surface text-text-muted')}>
          {done ? <Check size={14} strokeWidth={3} /> : index + 1}
        </span>
        <span>{step}{done && <span className="sr-only">, done</span>}</span>
        {index < steps.length - 1 && <span aria-hidden="true" className={cn('hidden min-w-4 flex-1 border-t-2 sm:block', done ? 'border-text' : 'border-dashed border-border-strong')} />}
      </li>;
    })}
  </ol>;
}

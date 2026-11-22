import { Check } from 'lucide-react';
import { cn } from '@/shared/lib/utils';

// Connected step rail: done steps are ink with a tick, the current step has a signal ring,
// steps ahead are outlined and joined by a dashed line.
export function Stepper({ steps, current }: { steps: string[]; current: number }) {
  return <ol className="mb-5 grid grid-cols-4 gap-2" aria-label="Booking progress">
    {steps.map((label, index) => {
      const done = index < current;
      const active = index === current;
      return <li key={label} aria-current={active ? 'step' : undefined} data-completed={done}
        className={cn('flex flex-col items-center gap-1.5 text-center text-xs sm:flex-row sm:gap-2 sm:text-left sm:text-sm',
          active ? 'font-semibold text-text' : done ? 'font-medium text-text' : 'font-medium text-text-muted')}>
        <span aria-hidden="true" className={cn('grid size-7 shrink-0 place-items-center rounded-full font-mono text-[13px]',
          done ? 'bg-chrome text-chrome-foreground' : active ? 'border-2 border-primary bg-surface text-text' : 'border border-border-strong bg-surface text-text-muted')}>
          {done ? <Check size={14} strokeWidth={3} /> : index + 1}
        </span>
        <span>{label}{done && <span className="sr-only">, done</span>}</span>
        {index < steps.length - 1 && <span aria-hidden="true" className={cn('hidden min-w-4 flex-1 border-t-2 sm:block', done ? 'border-text' : 'border-dashed border-border-strong')} />}
      </li>;
    })}
  </ol>;
}

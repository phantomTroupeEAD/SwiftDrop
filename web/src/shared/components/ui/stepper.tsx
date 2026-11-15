export function Stepper({ steps, current }: { steps: string[]; current: number }) {
  return <ol className="mb-5 grid grid-cols-4 gap-2" aria-label="Booking progress">
    {steps.map((label, index) => <li key={label} className={`flex flex-col items-center gap-2 rounded-xl border px-1 py-3 text-center text-xs font-medium sm:flex-row sm:px-3 sm:text-sm ${index === current ? 'border-primary bg-primary-tint text-primary' : 'border-border bg-surface text-text-muted'}`} aria-current={current === index ? 'step' : undefined}
      data-completed={index < current}><span aria-hidden="true" className="grid size-6 place-items-center rounded-full bg-surface-subtle font-semibold">{index < current ? '✓' : index + 1}</span>{label}</li>)}
  </ol>;
}

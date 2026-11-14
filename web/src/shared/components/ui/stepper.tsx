export function Stepper({ steps, current }: { steps: string[]; current: number }) {
  return <ol className="m6-steps" aria-label="Booking progress">
    {steps.map((label, index) => <li key={label} aria-current={current === index ? 'step' : undefined}
      data-completed={index < current}>{index < current ? '✓' : index + 1} {label}</li>)}
  </ol>;
}

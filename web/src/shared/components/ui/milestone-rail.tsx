import type { ReactNode } from 'react';

export function MilestoneRail({ entries }: { entries: { id: string; title: string; time: string; detail?: ReactNode }[] }) {
  return <ol className="m6-timeline" data-testid="tracking-timeline">
    {entries.map(entry => <li key={entry.id}>
      <strong className="m6-timeline-label">{entry.title}</strong>
      <time dateTime={entry.time}>{new Date(entry.time).toLocaleString('en-LK')}</time>
      {entry.detail}
    </li>)}
  </ol>;
}

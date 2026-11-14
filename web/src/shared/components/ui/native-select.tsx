import type { ComponentProps } from 'react';

export function NativeSelect({ className, ...props }: ComponentProps<'select'>) {
  return <select className={`min-h-10 rounded-lg border border-border bg-surface px-3 text-text ${className ?? ''}`} {...props} />;
}

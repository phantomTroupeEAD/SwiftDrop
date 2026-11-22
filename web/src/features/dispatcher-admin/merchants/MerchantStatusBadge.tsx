import React from 'react';
import { Badge } from '@/shared/components/ui/badge';

const LABEL: Record<string, string> = {
  PENDING: 'Pending review',
  APPROVED: 'Approved',
  REJECTED: 'Rejected',
  SUSPENDED: 'Suspended',
  DISABLED: 'Disabled',
};

const VARIANT: Record<string, 'success' | 'warning' | 'error' | 'neutral'> = {
  PENDING: 'warning',
  APPROVED: 'success',
  REJECTED: 'error',
  SUSPENDED: 'error',
  DISABLED: 'neutral',
};

/** Always a text label, never colour alone. */
export const MerchantStatusBadge: React.FC<{ status: string }> = ({ status }) => <Badge variant={VARIANT[status] ?? 'neutral'}>{LABEL[status] ?? status}</Badge>;

export function statusLabel(status: string): string {
  return LABEL[status] ?? status;
}

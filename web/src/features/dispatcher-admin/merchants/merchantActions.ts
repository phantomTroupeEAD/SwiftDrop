import type { MerchantAction } from './merchantsApi';

// What an admin may do to a merchant in each status. This mirrors the merchant state machine so the
// screen shows only buttons that can work; the server still decides and answers
// 409 MERCHANT_INVALID_TRANSITION if the status changed meanwhile.

export interface ActionInfo {
  action: MerchantAction;
  /** Button label: verb and object. */
  label: string;
  /** Dialog title. */
  title: string;
  /** What happens, in words. */
  consequence: string;
  /** The server requires a reason of 5 to 500 characters. For the other actions it ignores any reason. */
  reasonRequired: boolean;
  /** Reject, suspend and disable get a warning style and name their effect. */
  destructive: boolean;
  /** Text on the confirm button while sending. */
  busy: string;
  /** Success message, with the business name filled in. */
  done: (name: string) => string;
}

export const ACTION_INFO: Record<MerchantAction, ActionInfo> = {
  approve: {
    action: 'approve',
    label: 'Approve merchant',
    title: 'Approve this merchant',
    consequence: 'The merchant becomes Approved, and its owner account is given the Merchant role for this merchant.',
    reasonRequired: false,
    destructive: false,
    busy: 'Approving...',
    done: (n) => `${n} approved.`,
  },
  reject: {
    action: 'reject',
    label: 'Reject application',
    title: 'Reject this application',
    consequence: 'The application becomes Rejected. This is final for this application: it cannot be reopened.',
    reasonRequired: true,
    destructive: true,
    busy: 'Rejecting...',
    done: (n) => `${n} rejected.`,
  },
  suspend: {
    action: 'suspend',
    label: 'Suspend merchant',
    title: 'Suspend this merchant',
    consequence: 'The merchant becomes Suspended and is no longer treated as approved. You can reinstate it later.',
    reasonRequired: true,
    destructive: true,
    busy: 'Suspending...',
    done: (n) => `${n} suspended.`,
  },
  reinstate: {
    action: 'reinstate',
    label: 'Reinstate merchant',
    title: 'Reinstate this merchant',
    consequence: 'The merchant becomes Approved again.',
    reasonRequired: false,
    destructive: false,
    busy: 'Reinstating...',
    done: (n) => `${n} reinstated.`,
  },
  disable: {
    action: 'disable',
    label: 'Disable merchant',
    title: 'Disable this merchant',
    consequence: 'The merchant becomes Disabled for good. Every API key is revoked and every webhook is switched off. A disabled merchant cannot be reinstated.',
    reasonRequired: true,
    destructive: true,
    busy: 'Disabling...',
    done: (n) => `${n} disabled.`,
  },
};

const ALLOWED: Record<string, MerchantAction[]> = {
  PENDING: ['approve', 'reject'],
  APPROVED: ['suspend', 'disable'],
  SUSPENDED: ['reinstate', 'disable'],
  REJECTED: [],
  DISABLED: [],
};

/** The actions that can be taken from a status, in the order they appear. */
export function actionsFor(status: string): ActionInfo[] {
  return (ALLOWED[status] ?? []).map((a) => ACTION_INFO[a]);
}

export const REASON_MIN = 5;
export const REASON_MAX = 500;

/** Characters as people count them (the server counts code points, not bytes). */
export function reasonLength(reason: string): number {
  return [...reason.trim()].length;
}

export function validateReason(reason: string): string | null {
  const n = reasonLength(reason);
  if (n === 0) return 'Give a reason. It is saved in the audit log.';
  if (n < REASON_MIN) return `Use at least ${REASON_MIN} characters.`;
  if (n > REASON_MAX) return `Use at most ${REASON_MAX} characters.`;
  return null;
}

/** Why a finished merchant has no actions. */
export function noActionsText(status: string): string {
  if (status === 'REJECTED') return 'This application was rejected. A rejected application cannot be changed.';
  if (status === 'DISABLED') return 'This merchant was disabled. A disabled merchant cannot be changed.';
  return 'No status changes are available for this merchant.';
}

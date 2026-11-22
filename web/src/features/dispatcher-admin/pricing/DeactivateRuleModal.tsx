import React, { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { ApiError } from '../api';
import { AdminDialog, Banner, Busy } from '../admin-console/ui';
import { deactivateRule, type PricingRule } from './pricingApi';

interface DeactivateRuleModalProps {
  token: string | null;
  rule: PricingRule;
  routeLabel: string;
  onClose: () => void;
  onSaved: (rule: PricingRule, message: string) => void;
  onStale: () => void;
}

/** Switches a rule off. It cannot be switched on again from this screen, so the window says so. */
export const DeactivateRuleModal: React.FC<DeactivateRuleModalProps> = ({ token, rule, routeLabel, onClose, onSaved, onStale }) => {
  const [banner, setBanner] = useState<ApiError | null>(null);
  const [stale, setStale] = useState(false);
  const [saving, setSaving] = useState(false);

  const confirm = async () => {
    setBanner(null);
    setSaving(true);
    try {
      const saved = await deactivateRule(token, rule.id, rule.version);
      onSaved(saved, `Rule ${saved.id} (${routeLabel}) is now inactive.`);
    } catch (err) {
      const api = err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't deactivate the rule. Try again.");
      setStale(api.code === 'CONCURRENT_MODIFICATION' || api.code === 'PRICING_RULE_INACTIVE');
      setBanner(api);
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog title={`Deactivate rule ${rule.id}`} description={routeLabel} onClose={onClose} busy={saving} testId="pricing-deactivate-dialog">
      <div className="grid gap-4 px-5 py-5">
        {banner && (
          <Banner
            tone={stale ? 'attn' : 'err'}
            reference={banner.correlationId}
            testId="pricing-deactivate-error"
            action={
              stale ? (
                <div>
                  <Button type="button" size="sm" onClick={onStale} data-testid="pricing-stale-reload-btn">
                    Close and reload the list
                  </Button>
                </div>
              ) : undefined
            }
          >
            {banner.status === 403 ? 'Your account is not allowed to change pricing rules.' : banner.message}
          </Banner>
        )}
        <p className="text-sm" data-testid="pricing-deactivate-consequence">
          New quotes on this route stop using this rule. Quotes and parcels that already used it keep their price. An inactive rule cannot be switched back on here; add a new rule instead.
        </p>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button type="button" variant="destructive" onClick={() => void confirm()} disabled={saving || stale} data-testid="pricing-deactivate-confirm-btn">
          {saving ? <Busy label="Deactivating..." /> : 'Deactivate rule'}
        </Button>
      </DialogFooter>
    </AdminDialog>
  );
};

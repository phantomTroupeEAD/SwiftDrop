import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader } from '@/shared/components/ui/card';
import { Badge } from '@/shared/components/ui/badge';
import { errorMessage, portalRequest } from './api';

export type MerchantProfile = {
  merchant_id: string;
  business_name: string;
  status: string;
  contact_email?: string;
  contact_phone?: string;
};

export function MerchantProfileBanner({ token }: { token: string }) {
  const [result, setResult] = useState<{ profile: MerchantProfile } | { error: string }>();
  useEffect(() => {
    const controller = new AbortController();
    void portalRequest<MerchantProfile>('/merchants/me', { token, signal: controller.signal })
      .then(profile => {
        if (!profile?.merchant_id || !profile.business_name || !profile.status) {
          throw new Error('Invalid merchant profile');
        }
        if (!controller.signal.aborted) setResult({ profile });
      })
      .catch(cause => {
        if (!controller.signal.aborted) setResult({ error: errorMessage(cause) });
      });
    return () => controller.abort();
  }, [token]);

  return (
    <Card className="mb-4">
      <CardHeader><h2 className="text-lg font-semibold">Merchant profile</h2></CardHeader>
      <CardContent>
        {!result ? <p role="status">Loading merchant profile…</p> : 'error' in result ? (
          <p role="alert">Couldn’t load merchant profile. {result.error}</p>
        ) : (
          <dl className="space-y-2 break-words text-sm">
            <div><dt className="text-text-muted">Business</dt><dd className="font-medium">{result.profile.business_name}</dd></div>
            <div><dt className="text-text-muted">Merchant tenant ID</dt><dd>{result.profile.merchant_id}</dd></div>
            <div><dt className="text-text-muted">Status</dt><dd><Badge variant={result.profile.status === 'APPROVED' ? 'success' : 'secondary'}>{result.profile.status}</Badge></dd></div>
            {result.profile.contact_email && <div><dt className="text-text-muted">Email</dt><dd>{result.profile.contact_email}</dd></div>}
            {result.profile.contact_phone && <div><dt className="text-text-muted">Phone</dt><dd>{result.profile.contact_phone}</dd></div>}
          </dl>
        )}
      </CardContent>
    </Card>
  );
}

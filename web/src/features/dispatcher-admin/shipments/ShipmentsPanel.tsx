import React, { useCallback, useEffect, useState } from 'react';
import {
  ArrowRight,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock,
  Eye,
  MapPin,
  Package,
  RefreshCw,
  SearchX,
  Tag,
} from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/ui/dialog';
import { Label } from '@/shared/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/components/ui/select';
import { Skeleton } from '@/shared/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/shared/components/ui/table';
import { ApiError } from '../api';
import { Banner, Busy, formatWhen } from '../admin-console/ui';
import { listZoneOptions, type ZoneOption } from '../fleet/fleetApi';
import {
  getParcelTracking,
  type AuthenticatedTracking,
} from '../tracking/trackingApi';
import {
  getParcelHistory,
  listShipments,
  type ParcelSummary,
  type ShipmentsData,
} from './shipmentsApi';

const ANY_ZONE = 'ANY';

const STATUS_FILTERS: Array<{ id: string; label: string }> = [
  { id: 'ALL', label: 'All' },
  { id: 'CREATED', label: 'Created' },
  { id: 'ASSIGNED', label: 'Assigned' },
  { id: 'PICKED_UP', label: 'Picked Up' },
  { id: 'IN_TRANSIT', label: 'In Transit' },
  { id: 'OUT_FOR_DELIVERY', label: 'Out for Delivery' },
  { id: 'DELIVERED', label: 'Delivered' },
  { id: 'ATTEMPT_FAILED', label: 'Failed' },
  { id: 'RETURNED', label: 'Returned' },
  { id: 'CANCELLED', label: 'Cancelled' },
];

const STATUS_VARIANT: Record<string, 'success' | 'neutral' | 'warning' | 'error'> = {
  DELIVERED: 'success',
  OUT_FOR_DELIVERY: 'neutral',
  IN_TRANSIT: 'neutral',
  PICKED_UP: 'neutral',
  ASSIGNED: 'neutral',
  CREATED: 'neutral',
  ATTEMPT_FAILED: 'warning',
  RETURNED: 'error',
  CANCELLED: 'error',
};

function humanizeStatus(status: string): string {
  return status
    .split('_')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
}

function formatCurrency(cents: number, currency: string): string {
  const major = (cents / 100).toFixed(2);
  return `${currency} ${major}`;
}

interface ShipmentsPanelProps {
  token: string | null;
  onNotice?: (message: string) => void;
}

export const ShipmentsPanel: React.FC<ShipmentsPanelProps> = ({ token }) => {
  const [zones, setZones] = useState<ZoneOption[]>([]);
  const [selectedZone, setSelectedZone] = useState<string>(ANY_ZONE);
  const [selectedStatus, setSelectedStatus] = useState<string>('ALL');
  const [page, setPage] = useState<number>(1);
  const pageSize = 20;

  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [data, setData] = useState<ShipmentsData | null>(null);

  // Journey Dialog state
  const [viewingParcel, setViewingParcel] = useState<ParcelSummary | null>(null);
  const [journeyLoading, setJourneyLoading] = useState<boolean>(false);
  const [journeyError, setJourneyError] = useState<ApiError | null>(null);
  const [journeyData, setJourneyData] = useState<AuthenticatedTracking | null>(null);

  // Load zone options once on mount
  useEffect(() => {
    let active = true;
    listZoneOptions()
      .then((z) => {
        if (active) setZones(z);
      })
      .catch(() => {
        // Non-blocking: zone selector remains optional
      });
    return () => {
      active = false;
    };
  }, []);

  const loadShipments = useCallback(
    async (isBackground = false) => {
      if (!isBackground) {
        setLoading(true);
      } else {
        setRefreshing(true);
      }
      setError(null);

      try {
        const res = await listShipments(token, {
          page,
          per_page: pageSize,
          status: selectedStatus,
          zone_id: selectedZone !== ANY_ZONE ? selectedZone : undefined,
        });
        setData(res.data);
      } catch (err) {
        setError(
          err instanceof ApiError
            ? err
            : new ApiError(0, 'UNKNOWN', 'Failed to retrieve shipments. Please check your network and try again.'),
        );
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [token, page, pageSize, selectedStatus, selectedZone],
  );

  useEffect(() => {
    void loadShipments(false);
  }, [loadShipments]);

  // Open journey inspection modal
  const handleOpenJourney = async (parcel: ParcelSummary) => {
    setViewingParcel(parcel);
    setJourneyLoading(true);
    setJourneyError(null);
    setJourneyData(null);

    try {
      try {
        const tracking = await getParcelTracking(token, parcel.tracking_number);
        setJourneyData(tracking);
        return;
      } catch {
        // Fallback to Core API authoritative parcel status history (/api/v1/parcels/{id}/history)
        const histData = await getParcelHistory(token, parcel.id);
        const mappedTracking: AuthenticatedTracking = {
          tracking_number: histData.tracking_number,
          parcel_id: histData.parcel_id,
          status: histData.status,
          status_description: humanizeStatus(histData.status),
          destination: {
            city: parcel.dropoff?.city ?? '',
            postal_code: parcel.dropoff?.postal_code ?? '',
            zone_id: parcel.destination_zone_id ?? '',
          },
          milestones: histData.history.map((h) => ({
            status: h.to_status,
            description:
              h.reason ||
              (h.from_status
                ? `Transitioned from ${h.from_status} to ${h.to_status}`
                : 'Shipment created and registered in system'),
            actor_role: h.actor_role || h.actor_type,
            timestamp: h.occurred_at,
          })),
        };
        setJourneyData(mappedTracking);
      }
    } catch (err) {
      setJourneyError(
        err instanceof ApiError
          ? err
          : new ApiError(0, 'UNKNOWN', 'Failed to retrieve milestone journey for this parcel.'),
      );
    } finally {
      setJourneyLoading(false);
    }
  };

  const handleCloseJourney = () => {
    setViewingParcel(null);
    setJourneyData(null);
    setJourneyError(null);
  };

  const totalPages = data ? Math.max(1, Math.ceil(data.total / pageSize)) : 1;

  return (
    <div className="space-y-6" data-role="DISPATCHER">
      {error && (
        <Banner
          tone="err"
          action={
            <Button
              variant="outline"
              size="sm"
              onClick={() => void loadShipments(false)}
              className="touch:min-h-[44px] text-xs"
            >
              Retry
            </Button>
          }
        >
          {error.message}
        </Banner>
      )}

      {/* Filters Card */}
      <Card>
        <CardContent className="pt-6 space-y-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            {/* Status Tabs */}
            <div className="flex flex-wrap items-center gap-1.5" role="tablist" aria-label="Shipment status filter">
              {STATUS_FILTERS.map((f) => {
                const isActive = selectedStatus === f.id;
                return (
                  <Button
                    key={f.id}
                    type="button"
                    role="tab"
                    aria-selected={isActive}
                    variant={isActive ? 'default' : 'ghost'}
                    size="sm"
                    className="touch:min-h-[44px] text-xs font-medium"
                    onClick={() => {
                      if (selectedStatus !== f.id) {
                        setSelectedStatus(f.id);
                        setPage(1);
                      }
                    }}
                  >
                    {f.label}
                  </Button>
                );
              })}
            </div>

            {/* Zone Filter & Refresh */}
            <div className="flex items-center gap-2">
              <Label htmlFor="zone-select" className="text-xs font-medium text-text-muted whitespace-nowrap">
                Zone:
              </Label>
              <Select
                value={selectedZone}
                onValueChange={(val) => {
                  setSelectedZone(val);
                  setPage(1);
                }}
              >
                <SelectTrigger id="zone-select" className="h-9 text-xs touch:min-h-[44px] min-w-[150px]">
                  <SelectValue placeholder="All zones" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY_ZONE}>All zones</SelectItem>
                  {zones.map((z) => (
                    <SelectItem key={z.id} value={z.id}>
                      {z.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void loadShipments(true)}
                disabled={loading || refreshing}
                className="flex items-center gap-1.5 touch:min-h-[44px]"
                aria-label="Refresh shipments"
              >
                <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} aria-hidden="true" />
                <span className="hidden sm:inline">Refresh</span>
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Parcels Table Card */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base font-semibold">Parcels</CardTitle>
              <CardDescription>
                {data
                  ? `Showing ${data.items.length === 0 ? 0 : (page - 1) * pageSize + 1} to ${Math.min(
                      page * pageSize,
                      data.total,
                    )} of ${data.total} parcels`
                  : 'Loading shipments...'}
              </CardDescription>
            </div>
            {refreshing && (
              <span className="text-xs text-text-muted flex items-center gap-1.5">
                <Busy label="Updating shipments..." />
              </span>
            )}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="p-6 space-y-3">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : !data || data.items.length === 0 ? (
            <div className="p-12 text-center">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-surface-subtle text-text-muted">
                <SearchX size={24} aria-hidden="true" />
              </div>
              <h3 className="mt-3 text-sm font-semibold text-text">No shipments found</h3>
              <p className="mt-1 text-sm text-text-muted">
                {selectedStatus !== 'ALL' || selectedZone !== ANY_ZONE
                  ? 'No parcels match the current status and zone filters. Try selecting a different filter.'
                  : 'No parcels have been created in the system yet.'}
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Tracking #</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Route (Origin &rarr; Destination)</TableHead>
                    <TableHead>Recipient</TableHead>
                    <TableHead>Service & Fee</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((parcel) => (
                    <TableRow key={parcel.id}>
                      <TableCell className="font-mono text-xs font-semibold tabular-nums text-text">
                        {parcel.tracking_number}
                      </TableCell>
                      <TableCell>
                        <Badge variant={STATUS_VARIANT[parcel.status] ?? 'neutral'}>
                          {humanizeStatus(parcel.status)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="text-xs space-y-0.5">
                          <div className="flex items-center gap-1 text-text font-medium">
                            <span>{parcel.origin_zone_id}</span>
                            <ArrowRight size={12} className="text-text-muted" aria-hidden="true" />
                            <span>{parcel.destination_zone_id}</span>
                          </div>
                          <div className="text-text-muted">
                            {parcel.dropoff?.city
                              ? `${parcel.dropoff.city} (${parcel.dropoff.postal_code})`
                              : parcel.destination_zone_id}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-xs space-y-0.5">
                          <div className="font-medium text-text">{parcel.recipient_name}</div>
                          <div className="text-text-muted tabular-nums">{parcel.recipient_phone}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-xs space-y-0.5">
                          <div className="font-medium text-text">{parcel.service_level}</div>
                          <div className="text-text-muted tabular-nums">
                            {formatCurrency(parcel.delivery_fee_cents ?? 0, parcel.currency ?? 'LKR')}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-xs text-text-muted whitespace-nowrap tabular-nums">
                        {formatWhen(parcel.created_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="outline"
                          size="sm"
                          className="touch:min-h-[44px] text-xs flex items-center gap-1.5 ml-auto"
                          onClick={() => void handleOpenJourney(parcel)}
                        >
                          <Eye size={14} aria-hidden="true" />
                          <span>View Journey</span>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Pagination Controls */}
          {data && data.total > 0 && (
            <div className="flex items-center justify-between border-t border-border px-6 py-4">
              <div className="text-xs text-text-muted">
                Page <span className="font-semibold text-text tabular-nums">{page}</span> of{' '}
                <span className="font-semibold text-text tabular-nums">{totalPages}</span>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1 || loading}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className="touch:min-h-[44px] text-xs flex items-center gap-1"
                >
                  <ChevronLeft size={14} aria-hidden="true" />
                  <span>Previous</span>
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages || loading}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  className="touch:min-h-[44px] text-xs flex items-center gap-1"
                >
                  <span>Next</span>
                  <ChevronRight size={14} aria-hidden="true" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Milestone Journey Dialog */}
      <Dialog open={!!viewingParcel} onOpenChange={(open) => !open && handleCloseJourney()}>
        <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <div className="flex items-center gap-2">
              <DialogTitle className="text-lg font-bold">
                Shipment Journey
              </DialogTitle>
              {viewingParcel && (
                <Badge variant={STATUS_VARIANT[viewingParcel.status] ?? 'neutral'}>
                  {humanizeStatus(viewingParcel.status)}
                </Badge>
              )}
            </div>
            <DialogDescription className="font-mono text-xs tabular-nums text-text-muted">
              {viewingParcel?.tracking_number}
            </DialogDescription>
          </DialogHeader>

          {journeyLoading ? (
            <div className="py-8 text-center space-y-3">
              <Busy label="Loading tracking timeline..." />
            </div>
          ) : journeyError ? (
            <Banner
              tone="err"
              action={
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => viewingParcel && void handleOpenJourney(viewingParcel)}
                  className="touch:min-h-[44px] text-xs"
                >
                  Retry
                </Button>
              }
            >
              {journeyError.message}
            </Banner>
          ) : journeyData ? (
            <div className="space-y-6 pt-2">
              {/* Destination & Routing Overview */}
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div className="rounded-md border border-border bg-surface-subtle p-3">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-text-muted">
                    <MapPin size={14} aria-hidden="true" /> City
                  </div>
                  <div className="mt-1 text-sm font-semibold text-text">
                    {journeyData.destination.city || 'Not specified'}
                  </div>
                </div>

                <div className="rounded-md border border-border bg-surface-subtle p-3">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-text-muted">
                    <Tag size={14} aria-hidden="true" /> Postal Code
                  </div>
                  <div className="mt-1 font-mono text-sm font-semibold tabular-nums text-text">
                    {journeyData.destination.postal_code || '—'}
                  </div>
                </div>

                <div className="rounded-md border border-border bg-surface-subtle p-3">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-text-muted">
                    <Package size={14} aria-hidden="true" /> Routing Zone
                  </div>
                  <div className="mt-1 font-mono text-sm font-semibold tabular-nums text-text">
                    {journeyData.destination.zone_id || '—'}
                  </div>
                </div>
              </div>

              {/* Milestones timeline */}
              <div>
                <h4 className="text-sm font-semibold text-text mb-3">Milestone Timeline</h4>
                {journeyData.milestones.length === 0 ? (
                  <p className="text-sm text-text-muted">No milestone history available for this shipment.</p>
                ) : (
                  <ol className="relative border-l border-border pl-6 space-y-6" aria-label="Tracking milestones">
                    {journeyData.milestones.map((m, index) => {
                      const isLatest = index === journeyData.milestones.length - 1;
                      return (
                        <li key={`${m.status}-${m.timestamp}-${index}`} className="relative">
                          <span
                            className={`absolute -left-[31px] mt-1 flex h-4 w-4 items-center justify-center rounded-full border border-border ${
                              isLatest ? 'bg-primary text-primary-foreground' : 'bg-surface text-text-muted'
                            }`}
                            aria-hidden="true"
                          >
                            <CheckCircle2 size={10} strokeWidth={2.5} />
                          </span>

                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-semibold text-xs text-text">
                                {humanizeStatus(m.status)}
                              </span>
                              <Badge variant={STATUS_VARIANT[m.status] ?? 'neutral'}>
                                {m.status}
                              </Badge>
                              {m.actor_role && (
                                <span className="rounded bg-surface-subtle px-1.5 py-0.5 text-xs text-text-muted">
                                  By {m.actor_role}
                                </span>
                              )}
                              {m.reason_code && (
                                <Badge variant="warning">
                                  {m.reason_code}
                                </Badge>
                              )}
                            </div>
                            <div className="flex items-center gap-1 font-mono text-xs tabular-nums text-text-muted">
                              <Clock size={12} aria-hidden="true" />
                              {formatWhen(m.timestamp)}
                            </div>
                          </div>

                          <p className="mt-1 text-xs text-text-muted">
                            {m.description}
                          </p>
                        </li>
                      );
                    })}
                  </ol>
                )}
              </div>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  );
};

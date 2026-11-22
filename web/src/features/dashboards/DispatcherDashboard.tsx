import { useEffect, useState } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { portalRequest, errorMessage, statusLabel } from '@/features/customer-merchant/portal/api';
import { PortalHeader } from '@/features/customer-merchant/portal/PortalLayout';
import { Card, CardHeader, CardContent } from '@/shared/components/ui/card';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { Input } from '@/shared/components/ui/input';
import { Textarea } from '@/shared/components/ui/textarea';
import { Label } from '@/shared/components/ui/label';
import { NativeSelect } from '@/shared/components/ui/native-select';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/shared/components/ui/dialog';

type Parcel = {
  id: string;
  tracking_number: string;
  recipient_city: string;
  weight_kg: number;
  status?: string;
  created_at?: string;
};
type Courier = {
  courier_id: string;
  name: string;
  employee_code: string;
  status: string;
  max_capacity: number;
  active_assignments: number;
  user_id?: string;
  courier_name?: string;
  current_zone_id?: string;
  active_count?: number;
  vehicle_type?: string;
};
type AssignedParcel = Parcel & { courier_id: string; status: string };
type Board = { assigned_parcels?: AssignedParcel[]; unassigned_parcels: Parcel[] | null; courier_loads: Courier[] | null };
type ZoneOption = { id: string; code?: string; name: string };
type CourierCandidate = { user_id: string; display_name: string; email: string; phone: string };

const DEFAULT_ZONES: ZoneOption[] = [
  { id: 'ZONE_COLOMBO_01', name: 'Colombo Central (CMB-01)' },
  { id: 'ZONE_COLOMBO_02', name: 'Colombo South (CMB-02)' },
  { id: 'ZONE_COLOMBO_03', name: 'Colombo East (CMB-03)' },
  { id: 'ZONE_KANDY_01', name: 'Kandy (KDY-01)' },
  { id: 'ZONE_GALLE_01', name: 'Galle (GAL-01)' },
  { id: 'ZONE_NEGOMBO_01', name: 'Negombo (NEG-01)' },
];

const DEFAULT_CAPACITIES: Record<string, number> = {
  MOTORCYCLE: 25,
  VAN: 60,
  TRUCK: 150,
};

export function DispatcherDashboard() {
  const { accessToken, refreshSession, logout } = useAuth();
  const [board, setBoard] = useState<Board>();
  const [couriers, setCouriers] = useState<Courier[]>([]);
  const [selectedZone, setSelectedZone] = useState<string>('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [revision, setRevision] = useState(0);
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [pending, setPending] = useState(false);

  // Cancel Assignment Modal State
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelParcel, setCancelParcel] = useState<AssignedParcel | null>(null);
  const [cancelReason, setCancelReason] = useState('');
  const [cancelPending, setCancelPending] = useState(false);
  const [cancelError, setCancelError] = useState('');

  // Reschedule Failed Parcel Modal State
  const [rescheduleOpen, setRescheduleOpen] = useState(false);
  const [rescheduleParcel, setRescheduleParcel] = useState<Parcel | null>(null);
  const [rescheduleCourierId, setRescheduleCourierId] = useState('');
  const [rescheduleNotes, setRescheduleNotes] = useState('');
  const [reschedulePending, setReschedulePending] = useState(false);
  const [rescheduleError, setRescheduleError] = useState('');

  // Shift Management State
  const [shiftToggling, setShiftToggling] = useState<string | null>(null);

  // Courier Registration Modal State
  const [registerOpen, setRegisterOpen] = useState(false);
  const [registerError, setRegisterError] = useState('');
  const [registerPending, setRegisterPending] = useState(false);
  const [zones, setZones] = useState<ZoneOption[]>(DEFAULT_ZONES);
  const [candidates, setCandidates] = useState<CourierCandidate[]>([]);
  const [candidatesLoading, setCandidatesLoading] = useState(false);
  const [manualEntry, setManualEntry] = useState(false);
  const [courierForm, setCourierForm] = useState({
    user_id: '',
    employee_code: '',
    vehicle_type: 'MOTORCYCLE',
    max_capacity: 25,
    current_zone_id: 'ZONE_COLOMBO_01',
  });

  // Fetch delivery zones on mount
  useEffect(() => {
    const controller = new AbortController();
    void portalRequest<ZoneOption[]>('/zones', { token: accessToken, signal: controller.signal })
      .then(result => {
        if (!controller.signal.aborted && Array.isArray(result) && result.length > 0) {
          setZones(result);
        }
      })
      .catch(() => {
        // Fallback default zones remain
      });
    return () => controller.abort();
  }, [accessToken]);

  // Fetch board and zone-filtered couriers
  useEffect(() => {
    const controller = new AbortController();
    const zoneQuery = selectedZone ? `?zone_id=${encodeURIComponent(selectedZone)}` : '';
    const boardPath = `/dispatch/board${zoneQuery}`;
    const couriersPath = `/dispatch/couriers${zoneQuery}`;

    Promise.allSettled([
      portalRequest<Board>(boardPath, { token: accessToken, signal: controller.signal }),
      portalRequest<Courier[]>(couriersPath, { token: accessToken, signal: controller.signal }),
    ]).then(([boardRes, couriersRes]) => {
      if (controller.signal.aborted) return;

      if (boardRes.status === 'fulfilled') {
        setBoard(boardRes.value);
      } else {
        setError(errorMessage(boardRes.reason));
      }

      if (couriersRes.status === 'fulfilled' && Array.isArray(couriersRes.value)) {
        setCouriers(couriersRes.value);
      } else if (boardRes.status === 'fulfilled' && boardRes.value?.courier_loads) {
        setCouriers(boardRes.value.courier_loads);
      }
    });

    return () => controller.abort();
  }, [accessToken, revision, selectedZone]);

  useEffect(() => {
    if (!registerOpen) return;
    const controller = new AbortController();

    // Fetch eligible courier candidates
    setCandidatesLoading(true);
    void portalRequest<CourierCandidate[]>('/dispatch/couriers/candidates', { token: accessToken, signal: controller.signal })
      .then(result => {
        if (!controller.signal.aborted && Array.isArray(result)) {
          setCandidates(result);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) setCandidates([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setCandidatesLoading(false);
      });

    return () => controller.abort();
  }, [registerOpen, accessToken]);

  async function refresh() {
    setError('');
    setBoard(undefined);
    setCouriers([]);
    setSelected({});
    if (refreshSession) {
      await refreshSession().catch(() => null);
    }
    setRevision(current => current + 1);
  }

  function openCancelModal(parcel: AssignedParcel) {
    setCancelParcel(parcel);
    setCancelReason('');
    setCancelError('');
    setCancelOpen(true);
  }

  async function submitCancelAssignment(e: React.FormEvent) {
    e.preventDefault();
    if (!cancelParcel) return;
    const trimmedReason = cancelReason.trim();
    if (!trimmedReason) {
      setCancelError('Please provide an operational cancellation reason.');
      return;
    }

    setCancelPending(true);
    setCancelError('');

    try {
      await portalRequest(`/dispatch/assignments/${encodeURIComponent(cancelParcel.id)}/cancel`, {
        method: 'POST',
        token: accessToken,
        body: { reason: trimmedReason },
      });
      setNotice(`Assignment cancelled for ${cancelParcel.tracking_number}. Parcel returned to unassigned queue.`);
      setCancelOpen(false);
      setCancelParcel(null);
      setCancelReason('');
      await refresh();
    } catch (cause) {
      setCancelError(errorMessage(cause));
    } finally {
      setCancelPending(false);
    }
  }

  function openRescheduleModal(parcel: Parcel) {
    setRescheduleParcel(parcel);
    setRescheduleCourierId('');
    setRescheduleNotes('');
    setRescheduleError('');
    setRescheduleOpen(true);
  }

  async function submitRescheduleParcel(e: React.FormEvent) {
    e.preventDefault();
    if (!rescheduleParcel) return;
    if (!rescheduleCourierId) {
      setRescheduleError('Please choose an on-shift courier for reattempt.');
      return;
    }

    setReschedulePending(true);
    setRescheduleError('');

    try {
      await portalRequest(`/dispatch/parcels/${encodeURIComponent(rescheduleParcel.id)}/reschedule`, {
        method: 'POST',
        token: accessToken,
        body: {
          courier_id: rescheduleCourierId,
          notes: rescheduleNotes.trim(),
        },
      });
      const courierObj = couriers.find(c => (c.courier_id || c.user_id) === rescheduleCourierId);
      const courierName = courierObj?.name || courierObj?.courier_name || courierObj?.employee_code || 'courier';
      setNotice(`Parcel ${rescheduleParcel.tracking_number} rescheduled and assigned to ${courierName}.`);
      setRescheduleOpen(false);
      setRescheduleParcel(null);
      setRescheduleCourierId('');
      setRescheduleNotes('');
      await refresh();
    } catch (cause) {
      setRescheduleError(errorMessage(cause));
    } finally {
      setReschedulePending(false);
    }
  }

  function handleVehicleTypeChange(newType: string) {
    setCourierForm(prev => ({
      ...prev,
      vehicle_type: newType,
      max_capacity: DEFAULT_CAPACITIES[newType] ?? prev.max_capacity,
    }));
  }

  async function submitRegisterCourier(e: React.FormEvent) {
    e.preventDefault();
    setRegisterPending(true);
    setRegisterError('');

    const trimmedUserID = courierForm.user_id.trim();
    const trimmedCode = courierForm.employee_code.trim();

    if (!trimmedUserID) {
      setRegisterError('User ID is required.');
      setRegisterPending(false);
      return;
    }
    const uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
    if (!uuidRegex.test(trimmedUserID)) {
      setRegisterError('User ID must be a valid UUID (e.g. 018f4a7c-1003-7000-8000-000000000003).');
      setRegisterPending(false);
      return;
    }
    if (!trimmedCode) {
      setRegisterError('Employee code is required (e.g. CR-7001).');
      setRegisterPending(false);
      return;
    }

    try {
      await portalRequest('/dispatch/couriers', {
        method: 'POST',
        token: accessToken,
        body: {
          user_id: trimmedUserID,
          employee_code: trimmedCode,
          vehicle_type: courierForm.vehicle_type,
          max_capacity: Number(courierForm.max_capacity) || 25,
          current_zone_id: courierForm.current_zone_id,
        },
      });

      setNotice(`Courier profile registered for ${trimmedCode}.`);
      setRegisterOpen(false);
      setManualEntry(false);
      setCourierForm({
        user_id: '',
        employee_code: '',
        vehicle_type: 'MOTORCYCLE',
        max_capacity: 25,
        current_zone_id: 'ZONE_COLOMBO_01',
      });
      refresh();
    } catch (cause) {
      setRegisterError(errorMessage(cause));
    } finally {
      setRegisterPending(false);
    }
  }

  async function assign(parcel: Parcel) {
    setPending(true);
    setNotice('');
    try {
      await portalRequest('/dispatch/assignments', {
        token: accessToken, method: 'POST', body: { parcel_id: parcel.id, courier_id: selected[parcel.id] },
      });
      setNotice(parcel.tracking_number + ' assigned.');
      refresh();
    } catch (cause) {
      setNotice(errorMessage(cause));
    } finally {
      setPending(false);
    }
  }

  async function toggleCourierShift(courier: Courier, targetStatus: string) {
    const courierUID = courier.user_id || courier.courier_id;
    if (!courierUID) return;

    setShiftToggling(courierUID);
    setNotice('');

    try {
      await portalRequest('/courier/shift', {
        method: 'POST',
        token: accessToken,
        body: {
          courier_id: courierUID,
          target_status: targetStatus,
        },
      });
      const cName = courier.name || courier.courier_name || courier.employee_code;
      setNotice(`Shift updated for ${cName}: ${statusLabel(targetStatus)}.`);
      await refresh();
    } catch (cause) {
      setNotice(errorMessage(cause));
    } finally {
      setShiftToggling(null);
    }
  }

  const parcels = board?.unassigned_parcels ?? [];
  const available = couriers.filter(
    courier => courier.status === 'ON_SHIFT' && (courier.active_assignments ?? courier.active_count ?? 0) < courier.max_capacity
  );

  return <div data-role="DISPATCHER" className="min-h-screen bg-canvas text-text">
    <PortalHeader profile="dispatcher" />
    <main className="m6-main" id="main-content">
      <section className="portal-hero" aria-labelledby="dispatch-title">
        <img src="/login-bg.jpg" alt="" fetchPriority="high" />
        <div><h1 id="dispatch-title" className="text-4xl font-semibold tracking-tight sm:text-5xl">Dispatch board</h1>
          <p className="mt-3 max-w-sm">Review parcels awaiting assignment and check courier capacity.</p></div>
      </section>
      {notice && <p className="mb-4" role="status">{notice}</p>}

      {/* Operating Zone Filter Toolbar */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-surface p-4 shadow-sm">
        <div className="flex flex-wrap items-center gap-3">
          <Label htmlFor="zone-filter" className="text-sm font-medium text-text">Operating Zone</Label>
          <NativeSelect
            id="zone-filter"
            aria-label="Filter by delivery zone"
            value={selectedZone}
            onChange={e => {
              setSelectedZone(e.target.value);
              setSelected({});
            }}
            className="w-56"
          >
            <option value="">All Operational Zones</option>
            {zones.map(z => (
              <option key={z.id} value={z.id}>
                {z.name} {z.code ? `(${z.code})` : ''}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex items-center gap-2">
          {selectedZone && (
            <Badge variant="neutral">
              Zone: {zones.find(z => z.id === selectedZone)?.name ?? selectedZone}
            </Badge>
          )}
          <Button variant="outline" size="sm" onClick={refresh}>Refresh</Button>
        </div>
      </div>

      {error ? (
        <Card>
          <CardContent className="space-y-3 p-5">
            <p role="alert">Couldn’t load the dispatch board. {error}</p>
            <div className="flex flex-wrap items-center gap-3">
              <Button variant="outline" onClick={refresh} data-testid="dispatcher-retry-btn">Retry</Button>
              {error.toLowerCase().includes('sign in') && (
                <Button variant="default" onClick={() => logout()}>Sign in again</Button>
              )}
            </div>
          </CardContent>
        </Card>
      ) : !board ? <p role="status">Loading dispatch board…</p> :
        <div className="grid items-start gap-4 lg:grid-cols-3" data-testid="dispatcher-board">
          <Card className="lg:col-span-2">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <h2 className="text-lg font-semibold">Unassigned parcels</h2>
              <Badge variant="neutral" showDot={false}>{parcels.length} waiting</Badge>
            </CardHeader>
            <CardContent>
              {!parcels.length && <p className="text-sm text-text-muted">No parcels are waiting for assignment.</p>}
              <ul className="divide-y divide-border">
                {parcels.map(parcel => {
                  const isFailed = parcel.status === 'ATTEMPT_FAILED';
                  return (
                    <li key={parcel.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <p className="font-mono font-semibold">{parcel.tracking_number}</p>
                          {isFailed && (
                            <Badge variant="warning" className="capitalize">
                              Attempt Failed
                            </Badge>
                          )}
                        </div>
                        <p className="text-sm text-text-muted">{parcel.recipient_city} · {parcel.weight_kg} kg</p>
                      </div>
                      {available.length > 0 && (
                        <div className="flex flex-wrap items-center gap-2">
                          {isFailed ? (
                            <Button
                              variant="outline"
                              size="sm"
                              data-testid="reschedule-btn"
                              onClick={() => openRescheduleModal(parcel)}
                            >
                              Reschedule
                            </Button>
                          ) : (
                            <>
                              <NativeSelect
                                aria-label={'Courier for ' + parcel.tracking_number}
                                data-testid="dispatcher-courier-select"
                                value={selected[parcel.id] ?? ''}
                                disabled={pending}
                                onChange={event => setSelected(current => ({ ...current, [parcel.id]: event.target.value }))}
                              >
                                <option value="">Choose courier</option>
                                {available.map(courier => {
                                  const cID = courier.courier_id || courier.user_id || '';
                                  const cName = courier.name || courier.courier_name || courier.employee_code;
                                  return (
                                    <option key={cID} value={cID}>
                                      {cName} ({courier.employee_code})
                                    </option>
                                  );
                                })}
                              </NativeSelect>
                              <Button
                                disabled={pending || !selected[parcel.id]}
                                data-testid="dispatcher-assign-btn"
                                onClick={() => void assign(parcel)}
                              >
                                Assign courier
                              </Button>
                            </>
                          )}
                        </div>
                      )}
                    </li>
                  );
                })}
              </ul>
              {parcels.length > 0 && !available.length && <p className="text-sm text-text-muted">No on-shift couriers have spare capacity.</p>}
            </CardContent>
          </Card>
          <Card className="lg:col-start-1 lg:col-span-2 lg:row-start-2">
            <CardHeader><h2 className="text-lg font-semibold">Assigned parcels</h2></CardHeader>
            <CardContent data-testid="dispatcher-assigned-parcels">
              {board.assigned_parcels === undefined ? <p className="text-sm text-text-muted">Assigned parcels are unavailable right now.</p> :
                !board.assigned_parcels.length ? <p className="text-sm text-text-muted">No active assignments.</p> :
                <ul className="divide-y divide-border">
                  {board.assigned_parcels.map(parcel => {
                    const courier = couriers.find(item => (item.courier_id || item.user_id) === parcel.courier_id);
                    return <li key={parcel.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                      <div>
                        <p className="font-mono font-semibold">{parcel.tracking_number}</p>
                        <p className="text-sm text-text-muted">{parcel.recipient_city} · {parcel.weight_kg} kg</p>
                      </div>
                      <p className="text-sm">{courier?.name || courier?.courier_name || courier?.employee_code || 'Courier details unavailable'}</p>
                      <div className="flex items-center gap-2">
                        <Badge variant="active" className="capitalize">{statusLabel(parcel.status)}</Badge>
                        <Button
                          variant="outline"
                          size="sm"
                          data-testid="unassign-btn"
                          onClick={() => openCancelModal(parcel)}
                        >
                          Unassign
                        </Button>
                      </div>
                    </li>;
                  })}
                </ul>}
            </CardContent>
          </Card>
          <Card className="lg:col-start-3 lg:row-start-1 lg:row-span-2">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <h2 className="text-lg font-semibold">Courier roster</h2>
                <p className="text-xs text-text-muted">
                  {selectedZone ? (zones.find(z => z.id === selectedZone)?.name ?? selectedZone) : 'All Zones'} · {couriers.length} couriers
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                data-testid="onboard-courier-btn"
                onClick={() => setRegisterOpen(true)}
              >
                + Add Courier
              </Button>
            </CardHeader>
            <CardContent>
              {!couriers.length && (
                <p className="text-sm text-text-muted">
                  {selectedZone ? 'No active couriers operating in this zone.' : 'No couriers are listed on this board.'}
                </p>
              )}
              <ul className="divide-y divide-border">
                {couriers.map(courier => {
                  const cID = courier.courier_id || courier.user_id || courier.employee_code;
                  const cName = courier.name || courier.courier_name || courier.employee_code;
                  const activeAssignments = courier.active_assignments ?? courier.active_count ?? 0;
                  const zoneObj = zones.find(z => z.id === courier.current_zone_id);
                  return (
                    <li key={cID} className="py-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <p className="font-medium">{cName}</p>
                          <p className="text-xs text-text-muted">
                            {courier.employee_code}
                            {courier.vehicle_type ? ` · ${courier.vehicle_type}` : ''}
                            {!selectedZone && zoneObj ? ` · ${zoneObj.name}` : ''}
                          </p>
                        </div>
                        <Badge
                          variant={
                            courier.status === 'ON_SHIFT'
                              ? 'success'
                              : courier.status === 'ON_BREAK'
                              ? 'warning'
                              : 'neutral'
                          }
                          className="capitalize"
                        >
                          {statusLabel(courier.status)}
                        </Badge>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                        <p className="text-sm text-text-muted">
                          {activeAssignments} of {courier.max_capacity} assignments
                        </p>
                        <div className="flex flex-wrap items-center gap-1.5">
                          {courier.status === 'OFF_SHIFT' && (
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={shiftToggling === (courier.user_id || courier.courier_id)}
                              onClick={() => void toggleCourierShift(courier, 'ON_SHIFT')}
                              data-testid={`toggle-shift-${courier.employee_code}`}
                            >
                              {shiftToggling === (courier.user_id || courier.courier_id) ? 'Updating…' : 'Start shift'}
                            </Button>
                          )}
                          {courier.status === 'ON_SHIFT' && (
                            <>
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={shiftToggling === (courier.user_id || courier.courier_id)}
                                onClick={() => void toggleCourierShift(courier, 'ON_BREAK')}
                                data-testid={`toggle-break-${courier.employee_code}`}
                              >
                                {shiftToggling === (courier.user_id || courier.courier_id) ? 'Updating…' : 'Break'}
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={shiftToggling === (courier.user_id || courier.courier_id)}
                                onClick={() => void toggleCourierShift(courier, 'OFF_SHIFT')}
                                data-testid={`toggle-off-${courier.employee_code}`}
                              >
                                {shiftToggling === (courier.user_id || courier.courier_id) ? 'Updating…' : 'End shift'}
                              </Button>
                            </>
                          )}
                          {courier.status === 'ON_BREAK' && (
                            <>
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={shiftToggling === (courier.user_id || courier.courier_id)}
                                onClick={() => void toggleCourierShift(courier, 'ON_SHIFT')}
                                data-testid={`toggle-shift-${courier.employee_code}`}
                              >
                                {shiftToggling === (courier.user_id || courier.courier_id) ? 'Updating…' : 'Resume shift'}
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={shiftToggling === (courier.user_id || courier.courier_id)}
                                onClick={() => void toggleCourierShift(courier, 'OFF_SHIFT')}
                                data-testid={`toggle-off-${courier.employee_code}`}
                              >
                                {shiftToggling === (courier.user_id || courier.courier_id) ? 'Updating…' : 'End shift'}
                              </Button>
                            </>
                          )}
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ul>
            </CardContent>
          </Card>
        </div>}

      <Dialog open={registerOpen} onOpenChange={setRegisterOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Register Courier Profile</DialogTitle>
            <DialogDescription>
              Onboard an existing user account into the operational courier fleet.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submitRegisterCourier} className="space-y-4 p-5">
            {registerError && (
              <p role="alert" className="text-sm font-medium text-status-error-ink bg-status-error-bg p-3 rounded-md border border-status-error-border">
                {registerError}
              </p>
            )}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor={manualEntry ? 'courier-user-id' : 'courier-user-select'}>Courier Account</Label>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-auto p-0 text-xs text-text-muted hover:text-text"
                  onClick={() => setManualEntry(prev => !prev)}
                >
                  {manualEntry ? 'Select from active accounts' : 'Enter UUID manually'}
                </Button>
              </div>

              {manualEntry ? (
                <>
                  <Input
                    id="courier-user-id"
                    name="user_id"
                    placeholder="e.g. 018f4a7c-1003-7000-8000-000000000003"
                    value={courierForm.user_id}
                    onChange={e => setCourierForm(prev => ({ ...prev, user_id: e.target.value }))}
                    disabled={registerPending}
                    required
                  />
                  <p className="text-xs text-text-muted">Must be an existing user account with the COURIER role.</p>
                </>
              ) : candidatesLoading ? (
                <p className="text-sm text-text-muted">Loading eligible courier accounts…</p>
              ) : candidates.length > 0 ? (
                <NativeSelect
                  id="courier-user-select"
                  value={courierForm.user_id}
                  onChange={e => {
                    const selectedUID = e.target.value;
                    setCourierForm(prev => ({
                      ...prev,
                      user_id: selectedUID,
                    }));
                  }}
                  disabled={registerPending}
                  required
                >
                  <option value="">Choose an eligible courier account...</option>
                  {candidates.map(c => (
                    <option key={c.user_id} value={c.user_id}>
                      {c.display_name || c.email} ({c.email}{c.phone ? ` · ${c.phone}` : ''})
                    </option>
                  ))}
                </NativeSelect>
              ) : (
                <div className="rounded-md border border-border bg-surface-subtle p-3 text-sm text-text-muted">
                  No unassigned courier accounts found. New accounts with the <span className="font-semibold text-text">COURIER</span> role must be created before provisioning fleet profiles.
                </div>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="courier-emp-code">Employee Code</Label>
              <Input
                id="courier-emp-code"
                name="employee_code"
                placeholder="e.g. CR-7001"
                value={courierForm.employee_code}
                onChange={e => setCourierForm(prev => ({ ...prev, employee_code: e.target.value }))}
                disabled={registerPending}
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="courier-vehicle-type">Vehicle Type</Label>
                <NativeSelect
                  id="courier-vehicle-type"
                  value={courierForm.vehicle_type}
                  onChange={e => handleVehicleTypeChange(e.target.value)}
                  disabled={registerPending}
                >
                  <option value="MOTORCYCLE">Motorcycle</option>
                  <option value="VAN">Van</option>
                  <option value="TRUCK">Truck</option>
                </NativeSelect>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="courier-max-capacity">Max Capacity</Label>
                <Input
                  id="courier-max-capacity"
                  type="number"
                  min={1}
                  max={200}
                  value={courierForm.max_capacity}
                  onChange={e => setCourierForm(prev => ({ ...prev, max_capacity: Number(e.target.value) }))}
                  disabled={registerPending}
                  required
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="courier-zone">Operating Zone</Label>
              <NativeSelect
                id="courier-zone"
                value={courierForm.current_zone_id}
                onChange={e => setCourierForm(prev => ({ ...prev, current_zone_id: e.target.value }))}
                disabled={registerPending}
              >
                {zones.map(z => (
                  <option key={z.id} value={z.id}>
                    {z.name || z.code || z.id}
                  </option>
                ))}
              </NativeSelect>
            </div>
            <DialogFooter className="pt-3">
              <Button
                type="button"
                variant="outline"
                disabled={registerPending}
                onClick={() => setRegisterOpen(false)}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={registerPending} data-testid="register-courier-submit-btn">
                {registerPending ? 'Registering…' : 'Register Courier'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Cancel Courier Assignment Modal */}
      <Dialog open={cancelOpen} onOpenChange={setCancelOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Cancel Courier Assignment</DialogTitle>
            <DialogDescription>
              Cancel the active courier assignment for parcel{' '}
              <span className="font-mono font-semibold">{cancelParcel?.tracking_number}</span>.
              Driver capacity will be restored and the parcel will return to the unassigned queue.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submitCancelAssignment} className="space-y-4 p-5">
            {cancelError && (
              <p role="alert" className="text-sm font-medium text-status-error-ink bg-status-error-bg p-3 rounded-md border border-status-error-border">
                {cancelError}
              </p>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="cancel-reason">Cancellation Reason</Label>
              <Textarea
                id="cancel-reason"
                name="reason"
                placeholder="e.g. Courier vehicle flat tire on Galle Road"
                value={cancelReason}
                onChange={e => setCancelReason(e.target.value)}
                disabled={cancelPending}
                required
                rows={3}
              />
              <p className="text-xs text-text-muted">An operational reason is required for the audit log.</p>
            </div>
            <DialogFooter className="pt-3">
              <Button
                type="button"
                variant="outline"
                disabled={cancelPending}
                onClick={() => setCancelOpen(false)}
              >
                Keep Assignment
              </Button>
              <Button
                type="submit"
                variant="destructive"
                disabled={cancelPending || !cancelReason.trim()}
                data-testid="confirm-cancel-assignment-btn"
              >
                {cancelPending ? 'Cancelling…' : 'Cancel Assignment'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Reschedule Failed Parcel Modal */}
      <Dialog open={rescheduleOpen} onOpenChange={setRescheduleOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Reschedule Failed Delivery</DialogTitle>
            <DialogDescription>
              Re-dispatch parcel{' '}
              <span className="font-mono font-semibold">{rescheduleParcel?.tracking_number}</span> ({rescheduleParcel?.recipient_city})
              for a supervised reattempt run.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submitRescheduleParcel} className="space-y-4 p-5">
            {rescheduleError && (
              <p role="alert" className="text-sm font-medium text-status-error-ink bg-status-error-bg p-3 rounded-md border border-status-error-border">
                {rescheduleError}
              </p>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="reschedule-courier">Assign To Courier</Label>
              <NativeSelect
                id="reschedule-courier"
                value={rescheduleCourierId}
                onChange={e => setRescheduleCourierId(e.target.value)}
                disabled={reschedulePending}
                required
              >
                <option value="">Choose on-shift courier</option>
                {available.map(courier => {
                  const cID = courier.courier_id || courier.user_id || '';
                  const cName = courier.name || courier.courier_name || courier.employee_code;
                  return (
                    <option key={cID} value={cID}>
                      {cName} ({courier.employee_code}) · {courier.vehicle_type || 'Courier'}
                    </option>
                  );
                })}
              </NativeSelect>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="reschedule-notes">
                Reattempt Instructions / Notes <span className="font-normal text-text-muted">(Optional)</span>
              </Label>
              <Textarea
                id="reschedule-notes"
                name="notes"
                placeholder="e.g. Customer confirmed home between 2 PM and 5 PM"
                value={rescheduleNotes}
                onChange={e => setRescheduleNotes(e.target.value)}
                disabled={reschedulePending}
                rows={3}
              />
              <p className="text-xs text-text-muted">Optional supervisor notes recorded for the courier and audit trail.</p>
            </div>
            <DialogFooter className="pt-3">
              <Button
                type="button"
                variant="outline"
                disabled={reschedulePending}
                onClick={() => setRescheduleOpen(false)}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={reschedulePending || !rescheduleCourierId}
                data-testid="confirm-reschedule-btn"
              >
                {reschedulePending ? 'Rescheduling…' : 'Authorize Reattempt'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </main>
  </div>;
}

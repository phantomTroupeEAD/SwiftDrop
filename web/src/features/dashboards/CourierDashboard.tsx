import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useAuth } from '@/app/providers/AuthContext';
import { PortalHeader } from '@/features/customer-merchant/portal/PortalLayout';
import { portalRequest, errorMessage, statusLabel } from '@/features/customer-merchant/portal/api';
import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { Card, CardHeader, CardContent } from '@/shared/components/ui/card';
import { Input } from '@/shared/components/ui/input';
import { NativeSelect } from '@/shared/components/ui/native-select';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/shared/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/shared/components/ui/dialog';

type Task = {
  parcel_id: string; tracking_number: string; current_status: string;
  recipient_name: string; recipient_phone: string; delivery_street: string; delivery_city: string;
  handling_instructions?: string;
  attempt_count?: number;
};
const reasons = ['RECIPIENT_UNAVAILABLE', 'CUSTOMER_REQUESTED_RESCHEDULE', 'ACCESS_RESTRICTED',
  'INVALID_ADDRESS', 'REFUSED_BY_RECIPIENT', 'DAMAGED_IN_TRANSIT', 'OTHER'];

type DeliveryNotice = {
  title: string;
  description: string;
  trackingNumber?: string;
};

type AttemptRecord = {
  attempt_id?: string;
  id?: string;
  attempt_number: number;
  outcome: string;
  failure_reason_code?: string;
  reason_code?: string;
  notes?: string;
  proof_type?: string;
  proof_attachment_url?: string;
  received_by_name?: string;
  attempted_at: string;
};

type DeliveryHistory = {
  parcel_id: string;
  current_status: string;
  attempt_count: number;
  attempts: AttemptRecord[];
};

export function CourierDashboard() {
  const { accessToken } = useAuth();
  const [tasks, setTasks] = useState<Task[]>();
  const [revision, setRevision] = useState(0);
  const [loadError, setLoadError] = useState('');
  const [actionError, setActionError] = useState('');
  const [notice, setNotice] = useState('');
  const [deliveryNotice, setDeliveryNotice] = useState<DeliveryNotice | null>(null);
  const [shift, setShift] = useState('');
  const [pending, setPending] = useState(false);
  const lock = useRef(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [panel, setPanel] = useState<{ id: string; type: 'proof' | 'failure' }>();
  const proof = useRef<{ file: File; id: string; url: string; key: string }>();

  // Proof mode: PHOTO or SIGNATURE
  const [proofMode, setProofMode] = useState<'PHOTO' | 'SIGNATURE'>('PHOTO');

  // Live Camera state
  const [cameraActive, setCameraActive] = useState(false);
  const [capturedPhoto, setCapturedPhoto] = useState<File | null>(null);
  const [capturedPhotoUrl, setCapturedPhotoUrl] = useState('');
  const [cameraError, setCameraError] = useState('');
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);

  // Signature state
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const isDrawing = useRef(false);
  const [hasSignature, setHasSignature] = useState(false);
  const [signatureData, setSignatureData] = useState('');
  const svgPathData = useRef('');

  // Delivery attempt history modal state
  const [historyTask, setHistoryTask] = useState<Task | null>(null);
  const [historyData, setHistoryData] = useState<DeliveryHistory | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState('');

  async function openHistoryModal(task: Task) {
    setHistoryTask(task);
    setHistoryData(null);
    setHistoryLoading(true);
    setHistoryError('');
    try {
      const res = await portalRequest<DeliveryHistory>('/deliveries/parcels/' + encodeURIComponent(task.parcel_id), {
        token: accessToken,
      });
      setHistoryData(res);
    } catch (cause) {
      const msg = errorMessage(cause);
      if (msg.toLowerCase().includes('not found') || msg.toLowerCase().includes('404')) {
        setHistoryError('No delivery attempts have been recorded for this parcel yet.');
      } else {
        setHistoryError(msg);
      }
    } finally {
      setHistoryLoading(false);
    }
  }

  useEffect(() => {
    return () => {
      if (streamRef.current) {
        streamRef.current.getTracks().forEach(t => t.stop());
        streamRef.current = null;
      }
    };
  }, []);

  // Ensure stream is bound to video element as soon as it mounts
  useEffect(() => {
    if (cameraActive && videoRef.current && streamRef.current) {
      const video = videoRef.current;
      video.srcObject = streamRef.current;
      video.onloadedmetadata = () => {
        void video.play().catch(() => {});
      };
      void video.play().catch(() => {});
    }
  }, [cameraActive]);

  function stopCamera() {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach(t => t.stop());
      streamRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
    setCameraActive(false);
  }

  async function startCamera() {
    setCameraError('');
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: { ideal: 'environment' } },
      });
      streamRef.current = stream;
      setCameraActive(true);
    } catch {
      setCameraError('Camera access unavailable. You can upload an image file instead.');
    }
  }

  function captureSnapshot(parcelId: string) {
    if (!videoRef.current) return;
    const video = videoRef.current;
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth || 640;
    canvas.height = video.videoHeight || 480;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
    canvas.toBlob((blob) => {
      if (blob) {
        const file = new File([blob], `proof_${parcelId}_${Date.now()}.jpg`, { type: 'image/jpeg' });
        const previewUrl = URL.createObjectURL(blob);
        setCapturedPhoto(file);
        setCapturedPhotoUrl(previewUrl);
        stopCamera();
      }
    }, 'image/jpeg', 0.85);
  }

  function clearSignature() {
    const canvas = canvasRef.current;
    if (canvas) {
      const ctx = canvas.getContext('2d');
      if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
    }
    svgPathData.current = '';
    setSignatureData('');
    setHasSignature(false);
  }

  function handlePointerDown(e: React.PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current;
    if (!canvas) return;
    canvas.setPointerCapture(e.pointerId);
    const rect = canvas.getBoundingClientRect();
    const x = Math.round(e.clientX - rect.left);
    const y = Math.round(e.clientY - rect.top);
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.lineWidth = 2.5;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.strokeStyle = 'black';
    ctx.beginPath();
    ctx.moveTo(x, y);
    isDrawing.current = true;
    svgPathData.current += ` M ${x} ${y}`;
  }

  function handlePointerMove(e: React.PointerEvent<HTMLCanvasElement>) {
    if (!isDrawing.current) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = Math.round(e.clientX - rect.left);
    const y = Math.round(e.clientY - rect.top);
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.lineTo(x, y);
    ctx.stroke();
    svgPathData.current += ` L ${x} ${y}`;
  }

  function handlePointerUp() {
    if (!isDrawing.current) return;
    isDrawing.current = false;
    const canvas = canvasRef.current;
    if (!canvas) return;
    if (svgPathData.current.length > 25) {
      const svgStr = `<svg xmlns="http://www.w3.org/2000/svg" width="${canvas.width}" height="${canvas.height}" viewBox="0 0 ${canvas.width} ${canvas.height}"><path d="${svgPathData.current.trim()}" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
      const b64 = 'data:image/svg+xml;base64,' + btoa(svgStr);
      setSignatureData(b64);
      setHasSignature(true);
    }
  }

  function closePanel() {
    stopCamera();
    if (capturedPhotoUrl) {
      URL.revokeObjectURL(capturedPhotoUrl);
      setCapturedPhotoUrl('');
    }
    setCapturedPhoto(null);
    clearSignature();
    setProofMode('PHOTO');
    setPanel(undefined);
  }

  useEffect(() => {
    const controller = new AbortController();
    void portalRequest<Task[] | null>('/courier/parcels', { token: accessToken, signal: controller.signal })
      .then(result => { if (!controller.signal.aborted) { setTasks(result ?? []); setLoadError(''); } })
      .catch(cause => { if (!controller.signal.aborted) setLoadError(errorMessage(cause)); });
    return () => controller.abort();
  }, [accessToken, revision]);

  async function run(action: () => Promise<void>) {
    if (lock.current) return;
    lock.current = true;
    setPending(true);
    setActionError('');
    setNotice('');
    try { await action(); }
    catch (cause) { setActionError(errorMessage(cause)); }
    finally { lock.current = false; setPending(false); }
  }

  function updated(message: string) {
    setNotice(message);
    closePanel();
    setSelected([]);
    setRevision(value => value + 1);
  }

  function submitAttempt(event: FormEvent<HTMLFormElement>, task: Task, type: 'proof' | 'failure') {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const path = '/deliveries/' + encodeURIComponent(task.parcel_id);
    void run(async () => {
      if (type === 'failure') {
        const reason = String(fields.get('reason') || '');
        const notes = String(fields.get('notes') || '').trim();
        if (reason === 'OTHER' && notes.length < 15) {
          setActionError('Reason OTHER requires a detailed explanation of at least 15 characters.');
          return;
        }

        const res = await portalRequest<{ parcel_status: string; attempt_number: number; is_terminal: boolean }>(path + '/failed', {
          method: 'POST', token: accessToken, body: { failure_reason_code: reason, notes },
        });
        setDeliveryNotice({
          title: res.is_terminal ? 'Parcel marked as returned to hub' : 'Failed attempt recorded',
          description: `Attempt #${res.attempt_number} logged (${statusLabel(reason)}). Current status: ${statusLabel(res.parcel_status)}.`,
          trackingNumber: task.tracking_number,
        });
        updated(`Delivery attempt recorded for ${task.tracking_number}.`);
        return;
      }

      const name = String(fields.get('name') ?? '').trim();
      if (!name || name.length < 2) {
        setActionError('Please provide the recipient name (at least 2 characters).');
        return;
      }

      if (proofMode === 'PHOTO') {
        const file = capturedPhoto || fields.get('photo');
        if (!(file instanceof File) || !file.size) {
          setActionError('Please capture a live photo or select an image file as proof of delivery.');
          return;
        }

        if (proof.current?.file !== file || proof.current.id !== task.parcel_id) {
          const payload = new FormData();
          payload.set('file', file);
          const uploaded = await portalRequest<{ photo_url: string }>(path + '/proof-photo', {
            method: 'POST', token: accessToken, body: payload,
          });
          proof.current = { file, id: task.parcel_id, url: uploaded.photo_url, key: crypto.randomUUID() };
        }

        await portalRequest(path + '/complete', {
          method: 'POST', token: accessToken, idempotencyKey: proof.current.key,
          body: { proof_type: 'PHOTO', proof_url: proof.current.url, received_by_name: name },
        });
        proof.current = undefined;
      } else {
        if (!hasSignature || signatureData.length < 50) {
          setActionError('Please have the recipient draw their signature in the box below.');
          return;
        }

        await portalRequest(path + '/complete', {
          method: 'POST', token: accessToken, idempotencyKey: crypto.randomUUID(),
          body: { proof_type: 'SIGNATURE', signature_data: signatureData, received_by_name: name },
        });
      }

      setDeliveryNotice({
        title: 'Delivery confirmed successfully',
        description: `Parcel handed over to ${name} with verified ${proofMode === 'PHOTO' ? 'photo' : 'signature'} proof.`,
        trackingNumber: task.tracking_number,
      });
      updated(`Delivery completed for ${task.tracking_number}.`);
    });
  }

  const batchIds = (tasks ?? []).filter(task => task.current_status === 'ASSIGNED' && selected.includes(task.parcel_id)).map(task => task.parcel_id);

  return <div data-role="COURIER" className="min-h-screen bg-canvas text-text">
    <PortalHeader profile="courier" />
    <main id="main-content" className="mx-auto max-w-5xl space-y-4 px-4 py-5 sm:px-6">
      <section className="portal-hero" aria-labelledby="courier-title">
        <img src="/courier-delivery-hero.jpg" alt="" fetchPriority="high" />
        <div><h1 id="courier-title" className="text-4xl font-semibold tracking-tight">Delivery tasks</h1>
          <p className="mt-2">Manage your shift and assigned deliveries.</p></div>
      </section>
      <Card><CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
        <div><h2 className="font-semibold">Shift</h2>{shift && <p className="capitalize text-text-muted">{statusLabel(shift)}</p>}</div>
        <div className="flex flex-wrap gap-2">
          {['ON_SHIFT', 'OFF_SHIFT'].filter(status => status !== shift).map(status =>
            <Button key={status} variant="outline" className="min-h-11" disabled={pending} onClick={() => void run(async () => {
              const result = await portalRequest<{ new_status: string }>('/courier/shift', {
                method: 'POST', token: accessToken, body: { target_status: status },
              });
              setShift(result.new_status);
            })}>{status === 'ON_SHIFT' ? 'Start shift' : 'End shift'}</Button>)}
        </div>
      </CardContent></Card>

      {deliveryNotice && (
        <Card className="border-border bg-surface p-4" data-testid="delivery-success-alert">
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-status-success-bg text-status-success-text font-bold">
                ✓
              </div>
              <div>
                <h3 className="font-semibold text-text">{deliveryNotice.title}</h3>
                <p className="text-sm text-text-muted">{deliveryNotice.description}</p>
                {deliveryNotice.trackingNumber && (
                  <p className="font-mono text-xs text-text-muted mt-0.5">Tracking: {deliveryNotice.trackingNumber}</p>
                )}
              </div>
            </div>
            <Button variant="ghost" size="sm" onClick={() => setDeliveryNotice(null)}>Dismiss</Button>
          </div>
        </Card>
      )}

      {notice && !deliveryNotice && <p role="status">{notice}</p>}
      {actionError && <p role="alert">{actionError}</p>}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">Assigned deliveries</h2>
        <Button variant="outline" className="min-h-11" disabled={pending}
          onClick={() => setRevision(value => value + 1)}>Refresh tasks</Button>
      </div>
      {loadError ? <p role="alert">{loadError}</p> : !tasks ? <p role="status">Loading deliveries…</p> :
        !tasks.length && <p>No deliveries are assigned to you.</p>}
      {batchIds.length > 0 && <Button className="min-h-11 w-full" disabled={pending} onClick={() => void run(async () => {
        await portalRequest('/courier/runs/start-batch', {
          method: 'POST', token: accessToken, body: { parcel_ids: batchIds },
        });
        updated('Selected deliveries started.');
      })}>Start selected deliveries</Button>}
      {tasks?.map(task => {
        const active = panel?.id === task.parcel_id ? panel.type : undefined;
        const path = encodeURIComponent(task.parcel_id);
        const address = [task.delivery_street, task.delivery_city].filter(Boolean).join(', ');
        return <Card key={task.parcel_id} data-testid="courier-task">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <h3 className="break-all font-mono font-semibold">{task.tracking_number}</h3>
            <Badge variant="active" className="capitalize">{statusLabel(task.current_status)}</Badge>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="font-medium">{task.recipient_name}</p>
            <p className="text-text-muted">{address}</p>
            {task.handling_instructions && <p>{task.handling_instructions}</p>}
            <div className="grid grid-cols-2 gap-2">
              {task.recipient_phone && <Button asChild variant="outline" className="min-h-11"><a href={'tel:' + task.recipient_phone}>Call recipient</a></Button>}
              {address && <Button asChild variant="outline" className="min-h-11"><a href={'https://www.google.com/maps/search/?api=1&query=' + encodeURIComponent(address)} target="_blank" rel="noreferrer">Directions</a></Button>}
            </div>
            {((task.attempt_count ?? 0) > 0 || task.current_status === 'ATTEMPT_FAILED') && (
              <Button
                variant="outline"
                size="sm"
                className="w-full min-h-10 text-xs"
                disabled={pending}
                data-testid="view-attempt-history-btn"
                onClick={() => void openHistoryModal(task)}
              >
                📋 View attempt history {task.attempt_count ? `(${task.attempt_count})` : ''}
              </Button>
            )}
            {task.current_status === 'ASSIGNED' && <div className="grid gap-2 sm:grid-cols-2">
              <label className="flex min-h-11 items-center gap-2"><Input type="checkbox" className="!h-5 !w-5"
                disabled={pending} checked={selected.includes(task.parcel_id)}
                onChange={event => setSelected(current => event.target.checked ? [...current, task.parcel_id] : current.filter(id => id !== task.parcel_id))} />
                Select for batch</label>
              <Button className="min-h-11" disabled={pending || !!loadError} data-testid="courier-start-run" onClick={() => void run(async () => {
                await portalRequest('/courier/parcels/' + path + '/start-run', { method: 'POST', token: accessToken });
                updated('Delivery started.');
              })}>Start delivery</Button>
            </div>}
            {task.current_status === 'OUT_FOR_DELIVERY' && <div className="grid gap-2 sm:grid-cols-2">
              <Button className="min-h-11" disabled={pending} onClick={() => {
                closePanel();
                setPanel({ id: task.parcel_id, type: 'proof' });
              }}>Record delivery</Button>
              <Button className="min-h-11" variant="outline" disabled={pending} onClick={() => {
                closePanel();
                setPanel({ id: task.parcel_id, type: 'failure' });
              }}>Report failed attempt</Button>
            </div>}
            {(active === 'proof' || active === 'failure') && <form key={active} className="space-y-3"
              onSubmit={event => submitAttempt(event, task, active)}>
              <fieldset disabled={pending} className="space-y-3">
                {active === 'proof' ? <>
                  <label className="m6-field">Recipient name<Input className="min-h-11" name="name" required placeholder="Full name of person receiving parcel" /></label>

                  <div className="space-y-2">
                    <p className="text-sm font-medium">Proof of delivery method</p>
                    <Tabs value={proofMode} onValueChange={val => {
                      setProofMode(val as 'PHOTO' | 'SIGNATURE');
                      setActionError('');
                    }}>
                      <TabsList className="grid w-full grid-cols-2">
                        <TabsTrigger value="PHOTO">📷 Photo proof</TabsTrigger>
                        <TabsTrigger value="SIGNATURE">✍ Customer signature</TabsTrigger>
                      </TabsList>

                      <TabsContent value="PHOTO" className="space-y-3 pt-2">
                        {cameraError && <p className="text-sm text-text-muted">{cameraError}</p>}

                        {/* Live camera feed */}
                        {cameraActive ? (
                          <div className="space-y-2 rounded-lg border border-border p-3 bg-surface-subtle">
                            <div className="relative aspect-video w-full overflow-hidden rounded-md bg-black">
                              <video
                                ref={node => {
                                  videoRef.current = node;
                                  if (node && streamRef.current && node.srcObject !== streamRef.current) {
                                    node.srcObject = streamRef.current;
                                    node.onloadedmetadata = () => {
                                      void node.play().catch(() => {});
                                    };
                                    void node.play().catch(() => {});
                                  }
                                }}
                                playsInline
                                autoPlay
                                muted
                                className="h-full w-full object-cover"
                              />
                            </div>
                            <div className="flex gap-2">
                              <Button
                                type="button"
                                className="min-h-11 flex-1"
                                onClick={() => captureSnapshot(task.parcel_id)}
                              >
                                Capture snapshot
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                className="min-h-11"
                                onClick={stopCamera}
                              >
                                Cancel camera
                              </Button>
                            </div>
                          </div>
                        ) : capturedPhoto && capturedPhotoUrl ? (
                          <div className="space-y-2 rounded-lg border border-border p-3 bg-surface-subtle">
                            <div className="relative aspect-video w-full overflow-hidden rounded-md border border-border bg-black">
                              <img src={capturedPhotoUrl} alt="Captured proof snapshot" className="h-full w-full object-cover" />
                            </div>
                            <div className="flex items-center justify-between">
                              <p className="text-sm font-medium text-status-success-text">✓ Photo ready ({Math.round(capturedPhoto.size / 1024)} KB)</p>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                  if (capturedPhotoUrl) URL.revokeObjectURL(capturedPhotoUrl);
                                  setCapturedPhotoUrl('');
                                  setCapturedPhoto(null);
                                  void startCamera();
                                }}
                              >
                                Retake photo
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <div className="flex flex-col gap-2">
                            <Button
                              type="button"
                              variant="outline"
                              className="min-h-11"
                              onClick={() => void startCamera()}
                            >
                              Open live camera
                            </Button>
                            <label className="m6-field">
                              Or upload image file
                              <Input
                                className="min-h-11"
                                name="photo"
                                type="file"
                                accept="image/jpeg,image/png,image/webp"
                                capture="environment"
                              />
                            </label>
                          </div>
                        )}
                      </TabsContent>

                      <TabsContent value="SIGNATURE" className="space-y-3 pt-2">
                        <p className="text-sm text-text-muted">Sign on the touch pad below using finger, stylus, or mouse:</p>
                        <div className="overflow-hidden rounded-lg border border-border bg-white">
                          <canvas
                            ref={canvasRef}
                            width={360}
                            height={150}
                            className="w-full h-36 touch-none cursor-crosshair"
                            onPointerDown={handlePointerDown}
                            onPointerMove={handlePointerMove}
                            onPointerUp={handlePointerUp}
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <p className="text-xs text-text-muted">
                            {hasSignature ? '✓ Signature recorded' : 'Touchpad ready'}
                          </p>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={clearSignature}
                          >
                            Clear signature
                          </Button>
                        </div>
                      </TabsContent>
                    </Tabs>
                  </div>
                </> : <>
                  <label className="m6-field">Reason<NativeSelect name="reason">{reasons.map(reason =>
                    <option key={reason} value={reason}>{statusLabel(reason)}</option>)}</NativeSelect></label>
                  <label className="m6-field">Notes<Input className="min-h-11" name="notes" /></label>
                </>}
                <Button className="min-h-11 w-full" disabled={pending}>{pending ? 'Saving…' : active === 'proof' ? 'Confirm delivery' : 'Save attempt'}</Button>
              </fieldset>
            </form>}
            {active && <Button className="min-h-11" variant="ghost" disabled={pending} onClick={closePanel}>Close details</Button>}
          </CardContent>
        </Card>;
      })}

      <Dialog open={!!historyTask} onOpenChange={open => { if (!open) setHistoryTask(null); }}>
        <DialogContent className="max-w-lg p-0">
          <DialogHeader className="p-4 border-b border-border">
            <DialogTitle className="text-base font-semibold">Delivery attempt history</DialogTitle>
            <DialogDescription className="font-mono text-xs text-text-muted">
              Tracking: {historyTask?.tracking_number} · {historyTask?.delivery_city}
            </DialogDescription>
          </DialogHeader>

          <div className="max-h-[60vh] overflow-y-auto p-4 space-y-3">
            {historyLoading ? (
              <p className="text-sm text-text-muted">Loading attempt history…</p>
            ) : historyError ? (
              <p role="alert" className="text-sm text-text-muted">{historyError}</p>
            ) : !historyData?.attempts?.length ? (
              <p className="text-sm text-text-muted">No delivery attempts recorded for this parcel.</p>
            ) : (
              <div className="space-y-3">
                {historyData.attempts.map((attempt, index) => {
                  const isSuccess = attempt.outcome === 'SUCCESS';
                  const dateStr = attempt.attempted_at ? new Date(attempt.attempted_at).toLocaleString() : '';
                  return (
                    <div
                      key={attempt.attempt_id || attempt.id || index}
                      className="rounded-lg border border-border bg-surface-subtle p-3 space-y-2 text-sm"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-semibold text-text">
                          Attempt {attempt.attempt_number} of 3
                        </span>
                        <Badge variant={isSuccess ? 'success' : 'warning'}>
                          {isSuccess ? 'Delivered' : 'Attempt Failed'}
                        </Badge>
                      </div>

                      {(attempt.failure_reason_code || attempt.reason_code) && (
                        <p className="text-xs">
                          <span className="font-medium text-text">Reason: </span>
                          <span className="text-text-muted">{statusLabel(attempt.failure_reason_code || attempt.reason_code || '')}</span>
                        </p>
                      )}

                      {attempt.notes && (
                        <p className="text-xs">
                          <span className="font-medium text-text">Notes: </span>
                          <span className="text-text-muted">{attempt.notes}</span>
                        </p>
                      )}

                      {attempt.received_by_name && (
                        <p className="text-xs">
                          <span className="font-medium text-text">Received by: </span>
                          <span className="text-text-muted">{attempt.received_by_name}</span>
                        </p>
                      )}

                      {attempt.proof_type && (
                        <p className="text-xs">
                          <span className="font-medium text-text">Proof type: </span>
                          <span className="text-text-muted">{statusLabel(attempt.proof_type)}</span>
                        </p>
                      )}

                      {attempt.proof_attachment_url && (
                        <div className="pt-1">
                          <p className="text-xs font-medium text-text mb-1">Proof attachment:</p>
                          <div className="relative aspect-video w-48 overflow-hidden rounded-md border border-border bg-black">
                            <img
                              src={attempt.proof_attachment_url}
                              alt="Proof of delivery"
                              className="h-full w-full object-cover"
                            />
                          </div>
                        </div>
                      )}

                      {dateStr && (
                        <p className="text-xs text-text-muted pt-1 border-t border-border">
                          {dateStr}
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <DialogFooter className="p-3 border-t border-border bg-surface-subtle flex justify-end">
            <Button variant="outline" size="sm" onClick={() => setHistoryTask(null)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  </div>;
}

import React, { useState } from 'react';
import { DashboardLayout } from './DashboardLayout';
import {
  CheckCircle2,
  MapPin,
  Phone,
  Camera,
  AlertTriangle,
  Navigation,
  PenTool,
  Clock,
} from 'lucide-react';

interface DeliveryTask {
  id: string;
  trackingNumber: string;
  recipientName: string;
  phone: string;
  address: string;
  city: string;
  codAmount: string;
  status: 'ASSIGNED' | 'PICKED_UP' | 'OUT_FOR_DELIVERY' | 'DELIVERED' | 'ATTEMPT_FAILED';
  proofType?: string;
}

export const CourierDashboard: React.FC = () => {
  const [tasks, setTasks] = useState<DeliveryTask[]>([
    {
      id: 'tsk_01',
      trackingNumber: 'SD-20261030-99801',
      recipientName: 'Devinda Perera',
      phone: '+94 77 123 4567',
      address: '42 Galle Road, Bambalapitiya',
      city: 'Colombo 04',
      codAmount: 'PAID (Online)',
      status: 'OUT_FOR_DELIVERY',
    },
    {
      id: 'tsk_02',
      trackingNumber: 'SD-20261030-99802',
      recipientName: 'Kasun Wickramasinghe',
      phone: '+94 71 987 6543',
      address: '15/A Havelock Road',
      city: 'Colombo 05',
      codAmount: 'LKR 2,350.00',
      status: 'PICKED_UP',
    },
    {
      id: 'tsk_03',
      trackingNumber: 'SD-20261030-99803',
      recipientName: 'Priyani Fernando',
      phone: '+94 76 555 4321',
      address: '88 Duplication Road',
      city: 'Colombo 03',
      codAmount: 'PAID (Online)',
      status: 'DELIVERED',
      proofType: 'Signature Captured (P. Fernando)',
    },
  ]);

  const [onDuty, setOnDuty] = useState(true);
  const [activeModal, setActiveModal] = useState<{ id: string; type: 'POD' | 'FAIL' } | null>(null);
  const [failReason, setFailReason] = useState('RECIPIENT_UNAVAILABLE');
  const [signatureText, setSignatureText] = useState('');

  const updateTaskStatus = (id: string, newStatus: DeliveryTask['status'], proof?: string) => {
    setTasks((prev) =>
      prev.map((t) =>
        t.id === id ? { ...t, status: newStatus, proofType: proof || t.proofType } : t
      )
    );
    setActiveModal(null);
  };

  return (
    <DashboardLayout
      activeRole="COURIER"
      title="Courier Mobile Driver Route & POD Console"
      subtitle="Mobile-responsive delivery run, transit milestones, electronic signature, and photo Proof of Delivery (POD)."
    >
      {/* Driver Shift & Quick Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {/* Shift Card */}
        <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div className="space-y-1">
            <span className="text-xs text-slate-400">Shift Status</span>
            <div className="text-base font-bold text-white flex items-center gap-2">
              <span
                className={`w-2.5 h-2.5 rounded-full ${
                  onDuty ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                }`}
              />
              {onDuty ? 'ON DUTY' : 'OFF DUTY'}
            </div>
            <div className="text-[11px] text-slate-400">Vehicle: Bike #BK-204</div>
          </div>
          <button
            onClick={() => setOnDuty(!onDuty)}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition cursor-pointer ${
              onDuty
                ? 'bg-rose-950/40 text-rose-300 border-rose-800/50 hover:bg-rose-900/50'
                : 'bg-emerald-950/40 text-emerald-300 border-emerald-800/50 hover:bg-emerald-900/50'
            }`}
          >
            {onDuty ? 'End Shift' : 'Start Shift'}
          </button>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400">Assigned Run</div>
            <div className="text-2xl font-bold text-white mt-1">3 Stops</div>
            <div className="text-[11px] text-emerald-400">Zone: Colombo 03-05</div>
          </div>
          <div className="p-2.5 bg-blue-500/10 border border-blue-500/20 rounded-xl text-blue-400">
            <Navigation className="w-5 h-5" />
          </div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400">Delivered Today</div>
            <div className="text-2xl font-bold text-emerald-400 mt-1">1 / 3</div>
            <div className="text-[11px] text-slate-400">On-Time SLA: 100%</div>
          </div>
          <div className="p-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-xl text-emerald-400">
            <CheckCircle2 className="w-5 h-5" />
          </div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400">COD Collected</div>
            <div className="text-xl font-bold text-amber-400 mt-1">LKR 0.00</div>
            <div className="text-[11px] text-slate-400">1 COD pending</div>
          </div>
          <div className="p-2.5 bg-amber-500/10 border border-amber-500/20 rounded-xl text-amber-400">
            <Clock className="w-5 h-5" />
          </div>
        </div>
      </div>

      {/* Delivery Task Cards List */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold text-white">Today's Stop-by-Stop Run</h2>
          <span className="text-xs text-slate-400">Tap actions as you reach recipients</span>
        </div>

        <div className="space-y-3">
          {tasks.map((task, index) => (
            <div
              key={task.id}
              className={`p-5 rounded-2xl border transition-all ${
                task.status === 'DELIVERED'
                  ? 'bg-slate-950/50 border-slate-800/50 opacity-75'
                  : task.status === 'ATTEMPT_FAILED'
                  ? 'bg-rose-950/20 border-rose-900/50'
                  : 'bg-slate-900/80 border-slate-800 shadow-md'
              }`}
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="w-5 h-5 rounded-full bg-slate-800 text-[11px] font-bold text-slate-300 flex items-center justify-center">
                      {index + 1}
                    </span>
                    <span className="font-mono font-bold text-sm text-blue-400">
                      {task.trackingNumber}
                    </span>
                    <span
                      className={`text-[10px] font-bold px-2 py-0.5 rounded border ${
                        task.status === 'DELIVERED'
                          ? 'bg-emerald-950/60 text-emerald-300 border-emerald-800/40'
                          : task.status === 'OUT_FOR_DELIVERY'
                          ? 'bg-blue-950/60 text-blue-300 border-blue-800/40'
                          : task.status === 'ATTEMPT_FAILED'
                          ? 'bg-rose-950/60 text-rose-300 border-rose-800/40'
                          : 'bg-amber-950/60 text-amber-300 border-amber-800/40'
                      }`}
                    >
                      {task.status.replace(/_/g, ' ')}
                    </span>
                  </div>

                  <div className="text-sm font-semibold text-slate-200">
                    {task.recipientName}
                  </div>

                  <div className="text-xs text-slate-400 flex items-center gap-1.5">
                    <MapPin className="w-3.5 h-3.5 text-slate-500 shrink-0" />
                    <span>{task.address}, {task.city}</span>
                  </div>

                  <div className="text-xs text-slate-400 flex items-center gap-3 pt-1">
                    <span className="flex items-center gap-1 text-slate-300 font-mono">
                      <Phone className="w-3 h-3 text-slate-500" />
                      {task.phone}
                    </span>
                    <span className="text-amber-400 font-semibold">
                      COD: {task.codAmount}
                    </span>
                  </div>

                  {task.proofType && (
                    <div className="text-[11px] text-emerald-400 flex items-center gap-1 pt-1 font-mono">
                      <CheckCircle2 className="w-3 h-3" />
                      {task.proofType}
                    </div>
                  )}
                </div>

                {/* Milestone Action Buttons */}
                <div className="flex flex-wrap sm:flex-col gap-2 shrink-0 justify-end pt-2 sm:pt-0">
                  {task.status === 'ASSIGNED' && (
                    <button
                      onClick={() => updateTaskStatus(task.id, 'PICKED_UP')}
                      className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-semibold border border-slate-700 cursor-pointer"
                    >
                      Mark Picked Up
                    </button>
                  )}

                  {task.status === 'PICKED_UP' && (
                    <button
                      onClick={() => updateTaskStatus(task.id, 'OUT_FOR_DELIVERY')}
                      className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-sm cursor-pointer"
                    >
                      Out for Delivery
                    </button>
                  )}

                  {task.status === 'OUT_FOR_DELIVERY' && (
                    <>
                      <button
                        onClick={() => setActiveModal({ id: task.id, type: 'POD' })}
                        className="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold shadow-sm flex items-center gap-1.5 cursor-pointer"
                      >
                        <PenTool className="w-3.5 h-3.5" />
                        Capture POD
                      </button>
                      <button
                        onClick={() => setActiveModal({ id: task.id, type: 'FAIL' })}
                        className="px-3 py-1.5 bg-slate-800 hover:bg-rose-950/40 text-slate-400 hover:text-rose-400 rounded-lg text-xs font-semibold border border-slate-700 cursor-pointer"
                      >
                        Failed Attempt
                      </button>
                    </>
                  )}

                  {task.status === 'DELIVERED' && (
                    <span className="text-xs text-emerald-400 flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> Drop Completed
                    </span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Proof of Delivery (POD) Signature Modal Mock */}
      {activeModal && activeModal.type === 'POD' && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
            <h3 className="text-base font-bold text-white flex items-center gap-2">
              <Camera className="w-5 h-5 text-emerald-400" />
              Capture Proof of Delivery (POD)
            </h3>
            <p className="text-xs text-slate-400">
              Record a recipient electronic signature or photo attachment to complete delivery.
            </p>

            <div className="space-y-2">
              <label className="text-xs text-slate-300 font-medium">Recipient Signature Name</label>
              <input
                type="text"
                placeholder="e.g. Kasun W."
                value={signatureText}
                onChange={(e) => setSignatureText(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white outline-none focus:border-blue-500"
              />
            </div>

            <div className="h-28 rounded-xl bg-slate-950 border border-dashed border-slate-800 flex flex-col items-center justify-center text-slate-500 text-xs gap-1">
              <PenTool className="w-6 h-6 stroke-1 text-slate-400" />
              <span>Digital Signature Pad Active</span>
              <span className="text-[10px] text-slate-600">Touch or stylus canvas area</span>
            </div>

            <div className="flex gap-2 pt-2">
              <button
                onClick={() =>
                  updateTaskStatus(
                    activeModal.id,
                    'DELIVERED',
                    `Signed by ${signatureText || 'Recipient'}`
                  )
                }
                className="flex-1 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl text-xs font-semibold cursor-pointer"
              >
                Confirm Delivery
              </button>
              <button
                onClick={() => setActiveModal(null)}
                className="py-2 px-4 bg-slate-800 hover:bg-slate-700 text-slate-400 rounded-xl text-xs font-semibold cursor-pointer"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Failed Delivery Attempt Reason Modal Mock */}
      {activeModal && activeModal.type === 'FAIL' && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
            <h3 className="text-base font-bold text-white flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-rose-400" />
              Record Delivery Attempt Failure
            </h3>
            <p className="text-xs text-slate-400">
              Select failure reason to reschedule delivery route.
            </p>

            <select
              value={failReason}
              onChange={(e) => setFailReason(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white outline-none"
            >
              <option value="RECIPIENT_UNAVAILABLE">RECIPIENT_UNAVAILABLE (Doorbell unanswered)</option>
              <option value="INVALID_ADDRESS">INVALID_ADDRESS (House number not found)</option>
              <option value="SECURITY_ACCESS_DENIED">SECURITY_ACCESS_DENIED (Gated building restriction)</option>
              <option value="RECIPIENT_RESCHEDULED">RECIPIENT_RESCHEDULED (Customer requested evening)</option>
            </select>

            <div className="flex gap-2 pt-2">
              <button
                onClick={() =>
                  updateTaskStatus(
                    activeModal.id,
                    'ATTEMPT_FAILED',
                    `Attempt Failed: ${failReason}`
                  )
                }
                className="flex-1 py-2 bg-rose-600 hover:bg-rose-500 text-white rounded-xl text-xs font-semibold cursor-pointer"
              >
                Log Attempt & Reschedule
              </button>
              <button
                onClick={() => setActiveModal(null)}
                className="py-2 px-4 bg-slate-800 hover:bg-slate-700 text-slate-400 rounded-xl text-xs font-semibold cursor-pointer"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </DashboardLayout>
  );
};

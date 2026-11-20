import { Route } from 'react-router-dom';
import { OpsLayout } from './OpsLayout';
import { AuditPage, DispatchPage, FailedDeliveriesPage, FleetPage, MerchantsPage, OpsIndex, OpsNotFound, PricingPage, ReportsPage, SettingsPage, ShipmentsPage, TrackingPage, UsersPage, ZonesPage } from './OpsPages';

/**
 * Route tree for the ops workspace. App.tsx mounts it with one line: {opsRoutes}.
 * Roll back by deleting that line and its import.
 */
export const opsRoutes = (
  <Route path="/ops" element={<OpsLayout />}>
    <Route index element={<OpsIndex />} />
    <Route path="dispatch" element={<DispatchPage />} />
    <Route path="shipments" element={<ShipmentsPage />} />
    <Route path="couriers" element={<FleetPage />} />
    <Route path="failed-deliveries" element={<FailedDeliveriesPage />} />
    <Route path="merchants" element={<MerchantsPage />} />
    <Route path="merchants/:id" element={<MerchantsPage />} />
    <Route path="reports" element={<ReportsPage />} />
    <Route path="tracking" element={<TrackingPage />} />
    <Route path="users" element={<UsersPage />} />
    <Route path="zones" element={<ZonesPage />} />
    <Route path="pricing" element={<PricingPage />} />
    <Route path="settings" element={<SettingsPage />} />
    <Route path="audit" element={<AuditPage />} />
    <Route path="*" element={<OpsNotFound />} />
  </Route>
);

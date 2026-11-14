import { Route } from 'react-router-dom';
import { OpsLayout } from './OpsLayout';
import { AuditPage, NotBuiltPage, OpsIndex, OpsNotFound, SettingsPage, UsersPage, ZonesPage } from './OpsPages';

/**
 * Route tree for the ops workspace. App.tsx mounts it with one line: {opsRoutes}.
 * Roll back by deleting that line and its import.
 */
export const opsRoutes = (
  <Route path="/ops" element={<OpsLayout />}>
    <Route index element={<OpsIndex />} />
    <Route path="dispatch" element={<NotBuiltPage id="dispatch" />} />
    <Route path="couriers" element={<NotBuiltPage id="couriers" />} />
    <Route path="failed-deliveries" element={<NotBuiltPage id="failed-deliveries" />} />
    <Route path="merchants" element={<NotBuiltPage id="merchants" />} />
    <Route path="merchants/:id" element={<NotBuiltPage id="merchants" />} />
    <Route path="reports" element={<NotBuiltPage id="reports" />} />
    <Route path="users" element={<UsersPage />} />
    <Route path="zones" element={<ZonesPage />} />
    <Route path="pricing" element={<NotBuiltPage id="pricing" />} />
    <Route path="settings" element={<SettingsPage />} />
    <Route path="audit" element={<AuditPage />} />
    <Route path="*" element={<OpsNotFound />} />
  </Route>
);

import React from 'react';
import { AdminConsole } from '@/features/dispatcher-admin/admin-console/AdminConsole';

// The admin console the ADMIN role lands on after sign-in (route /admin/dashboard).
// The screen lives in features/dispatcher-admin/admin-console and follows web/design.mdx.
export const AdminDashboard: React.FC = () => <AdminConsole />;

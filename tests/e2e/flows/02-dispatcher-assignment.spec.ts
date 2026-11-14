import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Flow 2: Dispatcher Assignment Board & Capacity Management
 *
 * Invariants:
 * - Dispatcher authenticates and accesses the operational dispatch board.
 * - Displays unassigned parcels queue and real-time courier roster.
 * - Disallows assigning couriers that are OFF_SHIFT or exceeding max_capacity.
 * - Selecting an eligible courier and clicking assign sends POST /dispatch/assignments.
 * - Updates the board state reactively (decrementing unassigned parcels count).
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Flow 2: Dispatcher Assignment Board E2E Flow', () => {
  test('should display unassigned parcels, choose an on-shift courier, and execute assignment', async ({
    dispatcherPage,
  }) => {
    let assigned = false;

    // 1. Mock REST API routes for Dispatcher
    await dispatcherPage.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-dispatcher-jwt-token',
            refresh_token: 'mock-dispatcher-refresh-token',
            user: {
              id: 'd0010000-0000-0000-0000-000000000001',
              email: 'dispatcher@swiftdrop.io',
              role: 'DISPATCHER',
              full_name: 'Amara Gunawardena',
            },
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/board', async (route) => {
      if (!assigned) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              unassigned_parcels: [
                {
                  id: 'pcl-unassigned-001',
                  tracking_number: 'SD-20261114-11001',
                  recipient_city: 'Colombo 04',
                  weight_kg: 3.2,
                },
              ],
              courier_loads: [
                {
                  courier_id: 'cr-001',
                  name: 'Saman Kumara',
                  employee_code: 'CR-701',
                  status: 'ON_SHIFT',
                  max_capacity: 5,
                  active_assignments: 1,
                },
                {
                  courier_id: 'cr-002',
                  name: 'Nimal Silva',
                  employee_code: 'CR-702',
                  status: 'OFF_SHIFT',
                  max_capacity: 5,
                  active_assignments: 0,
                },
              ],
            },
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              unassigned_parcels: [],
              courier_loads: [
                {
                  courier_id: 'cr-001',
                  name: 'Saman Kumara',
                  employee_code: 'CR-701',
                  status: 'ON_SHIFT',
                  max_capacity: 5,
                  active_assignments: 2,
                },
              ],
            },
          }),
        });
      }
    });

    await dispatcherPage.route('**/api/v1/dispatch/assignments', async (route) => {
      if (route.request().method() === 'POST') {
        const body = route.request().postDataJSON();
        expect(body.parcel_id).toBe('pcl-unassigned-001');
        expect(body.courier_id).toBe('cr-001');
        assigned = true;

        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              assignment_id: 'asg-9901',
              parcel_id: 'pcl-unassigned-001',
              courier_id: 'cr-001',
              status: 'ASSIGNED',
              assigned_at: new Date().toISOString(),
            },
          }),
        });
      } else {
        await route.fallback();
      }
    });

    // 2. Dispatcher Sign-In
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    // 3. Navigate to Dispatcher Dashboard
    await dispatcherPage.goto('/dispatcher/dashboard');

    // 4. Verify Dispatcher Board is loaded
    const boardLocator = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.DISPATCHER_BOARD));
    await expect(boardLocator).toBeVisible();

    // Verify unassigned parcel is listed
    await expect(dispatcherPage.getByText('SD-20261114-11001')).toBeVisible();
    await expect(dispatcherPage.getByText('Colombo 04 · 3.2 kg')).toBeVisible();

    // 5. Select on-shift courier
    const courierSelect = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT));
    await expect(courierSelect).toBeVisible();
    await courierSelect.selectOption('cr-001');

    // 6. Execute Assignment
    const assignBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ASSIGN_BTN));
    await expect(assignBtn).toBeEnabled();
    await assignBtn.click();

    // 7. Verify Assignment notification and Board update
    await expect(dispatcherPage.getByRole('status')).toContainText('SD-20261114-11001 assigned.');
    await expect(dispatcherPage.getByText('No parcels are waiting for assignment.')).toBeVisible();
  });
});

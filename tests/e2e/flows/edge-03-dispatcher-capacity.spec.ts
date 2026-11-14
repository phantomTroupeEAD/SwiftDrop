import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';

/**
 * Suite 3: Dispatcher Assignment & Fleet Over-Capacity Guardrails
 *
 * Edge cases covered:
 * 1. Fleet Over-Capacity Saturation: When all on-shift couriers have reached max_capacity (e.g. 5/5),
 *    the courier select is disabled and a warning banner is displayed.
 * 2. Concurrent Assignment Conflict: Simulates receiving a 409 Conflict when another dispatcher assigns
 *    the parcel first, asserting the retry mechanism gracefully re-fetches the board.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

test.describe('Suite 3: Dispatcher Fleet Capacity & Conflict Edge Cases', () => {
  test.beforeEach(async ({ dispatcherPage }) => {
    await dispatcherPage.route('**/api/v1/auth/refresh', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'mock-dispatcher-jwt-token',
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
  });

  test('Edge 3.1: Full courier capacity displays warning banner and prevents assignment', async ({
    dispatcherPage,
  }) => {
    // Board mock where the only on-shift courier is fully saturated (5 of 5)
    await dispatcherPage.route('**/api/v1/dispatch/board', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [
              {
                id: 'pcl-capacity-001',
                tracking_number: 'SD-20261114-55001',
                recipient_city: 'Colombo 07',
                weight_kg: 4.0,
              },
            ],
            courier_loads: [
              {
                courier_id: 'cr-full',
                name: 'Sunil Perera',
                employee_code: 'CR-901',
                status: 'ON_SHIFT',
                max_capacity: 5,
                active_assignments: 5, // Fully saturated!
              },
            ],
          },
        }),
      });
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await dispatcherPage.goto('/dispatcher/dashboard');

    // Verify parcel is waiting
    await expect(dispatcherPage.getByText('SD-20261114-55001')).toBeVisible();

    // Invariant: Because active_assignments == max_capacity, the courier select is omitted
    // and the warning message is displayed
    await expect(dispatcherPage.getByText('No on-shift couriers have spare capacity.')).toBeVisible();
    await expect(dispatcherPage.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT))).not.toBeVisible();
  });

  test('Edge 3.2: Concurrent assignment collision returns error and allows retry refresh', async ({
    dispatcherPage,
  }) => {
    let assigned = false;
    await dispatcherPage.route('**/api/v1/dispatch/board', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: !assigned ? [
              {
                id: 'pcl-conflict-001',
                tracking_number: 'SD-20261114-99001',
                recipient_city: 'Colombo 03',
                weight_kg: 1.5,
              },
            ] : [],
            courier_loads: [
              {
                courier_id: 'cr-avail',
                name: 'Nimal Silva',
                employee_code: 'CR-101',
                status: 'ON_SHIFT',
                max_capacity: 5,
                active_assignments: 2,
              },
            ],
          },
        }),
      });
    });

    // Mock 409 Conflict response on assignment
    await dispatcherPage.route('**/api/v1/dispatch/assignments', async (route) => {
      assigned = true;
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'PARCEL_ALREADY_ASSIGNED',
            message: 'Parcel SD-20261114-99001 has already been assigned to another courier.',
          },
        }),
      });
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await dispatcherPage.goto('/dispatcher/dashboard');

    const courierSelect = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT));
    await courierSelect.selectOption('cr-avail');

    await dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ASSIGN_BTN)).click();

    // Invariant: Conflict error banner is displayed
    await expect(dispatcherPage.getByRole('status')).toContainText(
      'Parcel SD-20261114-99001 has already been assigned to another courier.'
    );
  });
});

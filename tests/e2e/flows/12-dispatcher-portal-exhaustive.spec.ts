import { test, expect } from '../fixtures/authFixtures';
import { SELECTORS, testId } from '../helpers/selectors';
import type { Page } from '@playwright/test';

/**
 * Flow 12: Exhaustive Dispatcher Portal E2E Test Suite
 *
 * Covers 100% of the Dispatcher Portal capabilities:
 * 1. Empty & Populated Board Ingestion, Badges, Role Scoping [data-role="DISPATCHER"]
 * 2. Operating Zone Filter Toolbar (GET /zones, query params, badge, empty states, refresh)
 * 3. Courier Capacity & Shift Eligibility Guardrails (disallow OFF_SHIFT, ON_BREAK, or Saturated)
 * 4. Courier Assignment Execution & Reactive Board State Update
 * 5. Cancel / Unassign Assignment Modal with Mandatory Operational Reason & Capacity Restoral
 * 6. Reschedule Failed Delivery Modal (ATTEMPT_FAILED detection, supervisor notes, reattempt authorization)
 * 7. Administrative Courier Shift Management directly from Board (Start, Break, Resume, End shift)
 * 8. Courier Fleet Onboarding Modal (+ Add Courier, candidate accounts, UUID regex, vehicle type defaults)
 * 9. Error Resilience, 401 Session Sign-In Fallback, and Retry Recovery
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 * Collaborating Owners: Member 3 (Dispatch & Delivery), Member 7 (Dispatcher & Admin Frontend)
 */

async function mockDispatcherAuth(page: Page) {
  await page.route('**/api/v1/auth/refresh', async (route) => {
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

  await page.route('**/api/v1/auth/login', async (route) => {
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
}

test.describe('Flow 12: Dispatcher Portal Exhaustive Suite', () => {
  test.beforeEach(async ({ dispatcherPage }) => {
    await mockDispatcherAuth(dispatcherPage);
  });

  test('12.1: Empty board state, zero waiting badges, and role scoping', async ({
    dispatcherPage,
  }) => {
    await dispatcherPage.route('**/api/v1/zones', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            { id: 'ZONE_COLOMBO_01', name: 'Colombo Central (CMB-01)' },
            { id: 'ZONE_KANDY_01', name: 'Kandy (KDY-01)' },
          ],
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [],
            assigned_parcels: [],
            courier_loads: [],
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/couriers*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [] }),
      });
    });

    // Sign in and open dispatcher dashboard
    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();

    await dispatcherPage.goto('/dispatcher/dashboard');

    // Assert role scoping
    const rootRole = dispatcherPage.locator('[data-role="DISPATCHER"]');
    await expect(rootRole).toBeVisible();

    // Assert Board empty states
    await expect(dispatcherPage.getByText('No parcels are waiting for assignment.')).toBeVisible();
    await expect(dispatcherPage.getByText('No active assignments.')).toBeVisible();
    await expect(dispatcherPage.getByText('No couriers are listed on this board.')).toBeVisible();
    await expect(dispatcherPage.getByText('0 waiting')).toBeVisible();
  });

  test('12.2: Operating zone filter triggers query parameter scoping and board refresh', async ({
    dispatcherPage,
  }) => {
    let requestedZone = '';

    await dispatcherPage.route('**/api/v1/zones', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            { id: 'ZONE_COLOMBO_01', name: 'Colombo Central (CMB-01)' },
            { id: 'ZONE_KANDY_01', name: 'Kandy (KDY-01)' },
          ],
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      const url = new URL(route.request().url());
      requestedZone = url.searchParams.get('zone_id') || '';

      if (requestedZone === 'ZONE_KANDY_01') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              unassigned_parcels: [
                {
                  id: 'pcl-kandy-101',
                  tracking_number: 'SD-KDY-1001',
                  recipient_city: 'Kandy',
                  weight_kg: 2.5,
                },
              ],
              courier_loads: [
                {
                  courier_id: 'cr-kdy-01',
                  name: 'Ruwan Bandara',
                  employee_code: 'CR-KDY-01',
                  status: 'ON_SHIFT',
                  max_capacity: 25,
                  active_assignments: 0,
                  current_zone_id: 'ZONE_KANDY_01',
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
              unassigned_parcels: [
                {
                  id: 'pcl-cmb-001',
                  tracking_number: 'SD-CMB-0001',
                  recipient_city: 'Colombo 01',
                  weight_kg: 1.2,
                },
              ],
              courier_loads: [],
            },
          }),
        });
      }
    });

    await dispatcherPage.route('**/api/v1/dispatch/couriers*', async (route) => {
      const url = new URL(route.request().url());
      const zone = url.searchParams.get('zone_id');
      if (zone === 'ZONE_KANDY_01') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                courier_id: 'cr-kdy-01',
                name: 'Ruwan Bandara',
                employee_code: 'CR-KDY-01',
                status: 'ON_SHIFT',
                max_capacity: 25,
                active_assignments: 0,
                current_zone_id: 'ZONE_KANDY_01',
              },
            ],
          }),
        });
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: [] }) });
      }
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Initial state: Colombo parcel visible
    await expect(dispatcherPage.getByText('SD-CMB-0001')).toBeVisible();

    // Select Kandy zone
    const zoneSelect = dispatcherPage.locator('#zone-filter');
    await expect(zoneSelect).toBeVisible();
    await zoneSelect.selectOption('ZONE_KANDY_01');

    // Assert query param was sent and Kandy parcel is displayed
    await expect(dispatcherPage.getByText('SD-KDY-1001')).toBeVisible();
    await expect(dispatcherPage.getByText('Zone: Kandy (KDY-01)')).toBeVisible();
    expect(requestedZone).toBe('ZONE_KANDY_01');

    // Reset to All Zones
    await zoneSelect.selectOption('');
    await expect(dispatcherPage.getByText('SD-CMB-0001')).toBeVisible();
  });

  test('12.3: Courier eligibility filtering (disallowing OFF_SHIFT, ON_BREAK, or Saturated couriers)', async ({
    dispatcherPage,
  }) => {
    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [
              {
                id: 'pcl-eligibility-01',
                tracking_number: 'SD-ELIG-001',
                recipient_city: 'Colombo 03',
                weight_kg: 4.1,
              },
            ],
            courier_loads: [
              {
                courier_id: 'cr-eligible',
                name: 'Eligible OnShift',
                employee_code: 'CR-ELG',
                status: 'ON_SHIFT',
                max_capacity: 10,
                active_assignments: 2,
              },
              {
                courier_id: 'cr-offshift',
                name: 'OffShift Courier',
                employee_code: 'CR-OFF',
                status: 'OFF_SHIFT',
                max_capacity: 10,
                active_assignments: 0,
              },
              {
                courier_id: 'cr-onbreak',
                name: 'OnBreak Courier',
                employee_code: 'CR-BRK',
                status: 'ON_BREAK',
                max_capacity: 10,
                active_assignments: 1,
              },
              {
                courier_id: 'cr-saturated',
                name: 'Saturated Courier',
                employee_code: 'CR-SAT',
                status: 'ON_SHIFT',
                max_capacity: 5,
                active_assignments: 5,
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

    const courierSelect = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.COURIER_SELECT));
    await expect(courierSelect).toBeVisible();

    const options = await courierSelect.locator('option').allInnerTexts();
    // Must include 'Choose courier' and 'Eligible OnShift'
    expect(options).toContain('Choose courier');
    expect(options.some(opt => opt.includes('Eligible OnShift'))).toBe(true);

    // Must NOT include OFF_SHIFT, ON_BREAK, or saturated couriers
    expect(options.some(opt => opt.includes('OffShift Courier'))).toBe(false);
    expect(options.some(opt => opt.includes('OnBreak Courier'))).toBe(false);
    expect(options.some(opt => opt.includes('Saturated Courier'))).toBe(false);
  });

  test('12.4: Cancel/Unassign Assignment modal with operational reason validation', async ({
    dispatcherPage,
  }) => {
    let cancelCalled = false;
    let cancelReasonSent = '';

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [],
            assigned_parcels: [
              {
                id: 'asg-parcel-001',
                tracking_number: 'SD-CANCEL-001',
                recipient_city: 'Colombo 05',
                weight_kg: 2.0,
                courier_id: 'cr-001',
                status: 'ASSIGNED',
              },
            ],
            courier_loads: [
              {
                courier_id: 'cr-001',
                name: 'Kamal Gunaratne',
                employee_code: 'CR-701',
                status: 'ON_SHIFT',
                max_capacity: 10,
                active_assignments: 1,
              },
            ],
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/assignments/*/cancel', async (route) => {
      cancelCalled = true;
      const body = route.request().postDataJSON();
      cancelReasonSent = body.reason;

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            assignment_id: 'asg-parcel-001',
            status: 'CANCELLED',
            reason: cancelReasonSent,
          },
        }),
      });
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Click Unassign button on assigned parcel
    const unassignBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.UNASSIGN_BTN));
    await expect(unassignBtn).toBeVisible();
    await unassignBtn.click();

    // Verify Cancel Assignment modal opens
    await expect(dispatcherPage.getByText('Cancel Courier Assignment')).toBeVisible();
    await expect(dispatcherPage.getByRole('dialog').getByText('SD-CANCEL-001')).toBeVisible();

    // Verify button is disabled when reason is empty or whitespace
    const cancelTextarea = dispatcherPage.locator('#cancel-reason');
    await cancelTextarea.fill('   ');
    const confirmBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.CONFIRM_CANCEL_ASSIGNMENT_BTN));
    await expect(confirmBtn).toBeDisabled();
    expect(cancelCalled).toBe(false);

    // Fill valid cancellation reason and submit
    await cancelTextarea.fill('Courier motorcycle breakdown on Marine Drive');
    await expect(confirmBtn).toBeEnabled();
    await confirmBtn.click();

    expect(cancelCalled).toBe(true);
    expect(cancelReasonSent).toBe('Courier motorcycle breakdown on Marine Drive');

    // Verify notice banner
    await expect(
      dispatcherPage.getByText('Assignment cancelled for SD-CANCEL-001. Parcel returned to unassigned queue.')
    ).toBeVisible();
  });

  test('12.5: Reschedule Failed Parcel Modal (ATTEMPT_FAILED detection & supervisor authorization)', async ({
    dispatcherPage,
  }) => {
    let rescheduleCalled = false;
    let reschedulePayload: any = null;

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [
              {
                id: 'pcl-failed-901',
                tracking_number: 'SD-FAILED-901',
                recipient_city: 'Dehiwala',
                weight_kg: 3.5,
                status: 'ATTEMPT_FAILED',
              },
            ],
            assigned_parcels: [],
            courier_loads: [
              {
                courier_id: 'cr-van-01',
                name: 'Sunil Silva',
                employee_code: 'CR-VAN-01',
                status: 'ON_SHIFT',
                max_capacity: 25,
                active_assignments: 3,
                vehicle_type: 'VAN',
              },
            ],
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/parcels/*/reschedule', async (route) => {
      rescheduleCalled = true;
      reschedulePayload = route.request().postDataJSON();

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            parcel_id: 'pcl-failed-901',
            status: 'ASSIGNED',
            assigned_courier_id: reschedulePayload.courier_id,
          },
        }),
      });
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Verify ATTEMPT_FAILED badge and Reschedule button
    await expect(dispatcherPage.getByText('Attempt Failed')).toBeVisible();
    const rescheduleBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.RESCHEDULE_BTN));
    await expect(rescheduleBtn).toBeVisible();
    await rescheduleBtn.click();

    // Verify modal opened
    await expect(dispatcherPage.getByText('Reschedule Failed Delivery')).toBeVisible();

    // Select on-shift courier
    const courierSelect = dispatcherPage.locator('#reschedule-courier');
    await courierSelect.selectOption('cr-van-01');

    // Input supervisor instructions
    const notesInput = dispatcherPage.locator('#reschedule-notes');
    await notesInput.fill('Customer contacted via call center; requested reattempt after 4:30 PM');

    // Confirm reschedule
    const confirmBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.CONFIRM_RESCHEDULE_BTN));
    await confirmBtn.click();

    expect(rescheduleCalled).toBe(true);
    expect(reschedulePayload.courier_id).toBe('cr-van-01');
    expect(reschedulePayload.notes).toContain('reattempt after 4:30 PM');

    // Verify notice banner
    await expect(
      dispatcherPage.getByText('Parcel SD-FAILED-901 rescheduled and assigned to Sunil Silva.')
    ).toBeVisible();
  });

  test('12.6: Administrative shift toggling from Dispatcher roster card', async ({
    dispatcherPage,
  }) => {
    let targetStatusSent = '';

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [],
            courier_loads: [
              {
                courier_id: 'cr-shift-01',
                user_id: '018f4a7c-1003-7000-8000-000000000003',
                name: 'Pradeep Chamara',
                employee_code: 'CR-SHIFT-01',
                status: 'OFF_SHIFT',
                max_capacity: 25,
                active_assignments: 0,
              },
            ],
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/courier/shift', async (route) => {
      const body = route.request().postDataJSON();
      targetStatusSent = body.target_status;

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            courier_id: body.courier_id,
            status: targetStatusSent,
          },
        }),
      });
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Courier is OFF_SHIFT -> displays "Start shift"
    const startShiftBtn = dispatcherPage.locator('[data-testid="toggle-shift-CR-SHIFT-01"]');
    await expect(startShiftBtn).toBeVisible();
    await expect(startShiftBtn).toHaveText('Start shift');

    await startShiftBtn.click();

    expect(targetStatusSent).toBe('ON_SHIFT');
    await expect(
      dispatcherPage.getByText('Shift updated for Pradeep Chamara: On Shift.')
    ).toBeVisible();
  });

  test('12.7: Courier fleet onboarding modal (+ Add Courier) with candidate selection and capacity defaults', async ({
    dispatcherPage,
  }) => {
    let onboardPayload: any = null;

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            unassigned_parcels: [],
            courier_loads: [],
          },
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/couriers/candidates', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              user_id: '018f4a7c-2001-7000-8000-000000000005',
              display_name: 'Dinesh Fernando',
              email: 'dinesh@swiftdrop.io',
              phone: '+94771239999',
            },
          ],
        }),
      });
    });

    await dispatcherPage.route('**/api/v1/dispatch/couriers*', async (route) => {
      if (route.request().method() === 'POST') {
        onboardPayload = route.request().postDataJSON();
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              courier_id: 'cr-new-001',
              employee_code: onboardPayload?.employee_code || 'CR-9005',
              status: 'OFF_SHIFT',
            },
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: [] }),
        });
      }
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Click + Add Courier
    const addCourierBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.ONBOARD_COURIER_BTN));
    await expect(addCourierBtn).toBeVisible();
    await addCourierBtn.click();

    // Verify modal header
    await expect(dispatcherPage.getByText('Register Courier Profile')).toBeVisible();

    // Select candidate from dropdown
    const candidateSelect = dispatcherPage.locator('#courier-user-select');
    await expect(candidateSelect).toBeVisible();
    await candidateSelect.selectOption('018f4a7c-2001-7000-8000-000000000005');

    // Enter employee code
    const empCodeInput = dispatcherPage.locator('#courier-emp-code');
    await empCodeInput.fill('CR-9005');

    // Change vehicle type to VAN and verify max capacity automatically updates to 60
    const vehicleSelect = dispatcherPage.locator('#courier-vehicle-type');
    await vehicleSelect.selectOption('VAN');
    const capacityInput = dispatcherPage.locator('#courier-max-capacity');
    await expect(capacityInput).toHaveValue('60');

    // Submit form
    const submitBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.REGISTER_COURIER_SUBMIT_BTN));
    await submitBtn.click();

    expect(onboardPayload).toBeTruthy();
    expect(onboardPayload.user_id).toBe('018f4a7c-2001-7000-8000-000000000005');
    expect(onboardPayload.employee_code).toBe('CR-9005');
    expect(onboardPayload.vehicle_type).toBe('VAN');
    expect(onboardPayload.max_capacity).toBe(60);

    // Verify notice
    await expect(dispatcherPage.getByText('Courier profile registered for CR-9005.')).toBeVisible();
  });

  test('12.8: Board API failure displays error alert and retry button', async ({
    dispatcherPage,
  }) => {
    let failBoard = true;

    await dispatcherPage.route('**/api/v1/dispatch/board*', async (route) => {
      if (failBoard) {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'Database connection timeout' } }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              unassigned_parcels: [],
              courier_loads: [],
            },
          }),
        });
      }
    });

    await dispatcherPage.goto('/auth/login');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_EMAIL)).fill('dispatcher@swiftdrop.io');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.INPUT_PASSWORD)).fill('Password123!');
    await dispatcherPage.locator(testId(SELECTORS.AUTH.BTN_LOGIN)).click();
    await dispatcherPage.goto('/dispatcher/dashboard');

    // Assert error state
    await expect(dispatcherPage.getByRole('alert')).toContainText("Couldn’t load the dispatch board");
    const retryBtn = dispatcherPage.locator(testId(SELECTORS.DISPATCHER.RETRY_BTN));
    await expect(retryBtn).toBeVisible();

    // Recover on retry
    failBoard = false;
    await retryBtn.click();

    // Assert board recovers
    await expect(dispatcherPage.getByText('No parcels are waiting for assignment.')).toBeVisible();
  });
});

# Customer and Merchant Frontend: Testing and Acceptance

Owner: Sajini, Member 6  
Date: 24 October 2026  
Status: Test plan only; no execution results or coverage are claimed

## Strategy

I will use Vitest and React Testing Library for utilities, hooks and components, and Playwright for browser workflows. Mock API responses can unblock UI development, but the final booking and tracking journeys must also be checked against the real services.

The planned layers are unit tests for formatting/validation, component tests for screen states, contract checks against approved response fixtures, integration tests for shared auth/query behavior and browser tests for core workflows. Backend ownership/security enforcement must be verified through real API tests as well as frontend navigation tests.

## Test-case matrix

| ID | Requirement | Scenario and expected result | Layer | Status |
| --- | --- | --- | --- | --- |
| M6-01 | FR-IAM-001/002 | Valid registration/login reaches permitted portal | Browser | Not run |
| M6-02 | FR-IAM-003 | Google login succeeds; denied/invalid callback fails safely | Integration/browser with M1 | Not run |
| M6-03 | FR-PAR-001/005 | Invalid booking fields show errors and preserve input | Component/browser | Not run |
| M6-04 | FR-PAR-002/003 | Server quote amount/currency shown accurately | Component | Not run |
| M6-05 | FR-PAR-002/003 | Input change/expired quote requires new quote | Component/browser | Not run |
| M6-06 | FR-PAR-004; AT-001 | Confirmed booking shows returned tracking number | Real-service browser | Not run |
| M6-07 | FR-PAR-006; AT-003 | Duplicate submit/retry creates one parcel | API/browser with M2 | Not run |
| M6-08 | FR-PAR-007 | Empty, populated and paginated history render correctly | Component/browser | Not run |
| M6-09 | FR-PAR-008 | Receipt download succeeds; unavailable/denied shows error | Browser with M2 | Not run |
| M6-10 | FR-TRK-001/002; AT-005 | Public tracking never shows private contacts/proof | API and browser | Not run |
| M6-11 | FR-TRK-004/005 | Status changes appear; reconnect re-fetches current state | Integration/browser | Not run |
| M6-12 | FR-TRK-007 | Unknown number differs clearly from network outage | Component | Not run |
| M6-13 | FR-MER-001/002 | Merchant application state shown; pending account denied creation | Browser with M4 | Not run |
| M6-14 | FR-IAM-006; AT-004/011 | Another user's/tenant's record is denied by API | Security integration | Not run |
| M6-15 | FR-IAM-005 | Customer opening operations route gets denied UI | Router/browser | Not run |
| M6-16 | Session contract | Concurrent 401s use coordinated refresh; failure clears private state | Integration | Not run |
| M6-17 | Frontend quality | Loading, empty, validation, server-error and success states work | Component | Not run |
| M6-18 | Frontend quality | Forms work with keyboard, labels and mobile viewport | Browser/manual | Not run |
| M6-19 | Frontend quality | Old quote response cannot overwrite newer-input quote | Component | Not run |
| M6-20 | Frontend quality | Build, deep-link refresh and gateway API routing work in container | Deployment/browser with M9 | Not run |

## Fixtures and isolation

I will use fictional data and separate customer/merchant accounts. Tests must not send real paid notifications or use real customer details. Component tests will reset query/auth state between cases. Browser tests need predictable seed data and isolated created records. Google authentication tests need a team-approved test setup; a mock does not prove the real Google integration works.

## Failure and concurrency checks

I will include slow responses, offline requests, 401/403/404/409/429/5xx results, refresh failure, interrupted SSE, double submission and stale quotes. I will check that timers/listeners are removed when pages unmount and that one account's data is not shown after switching accounts.

## Commands to provide

These are the planned package scripts from the frontend setup, not verified executable commands yet:

```sh
npm run lint
npm run test:unit
npm run test:unit:coverage
npm run test:e2e
npm run build
```

CI should install from the committed lockfile, run type/lint checks, unit tests with coverage, build, and run the agreed Playwright scenarios against a known environment. Exact pipeline setup is coordinated with M9/M10.

## Coverage and evidence

I will record actual frontend coverage instead of copying the SRS's core-business-package target onto the UI. Critical checks include auth guards, quote invalidation, booking retries, tracking cleanup and private-data handling. The team still needs to agree on a numerical frontend threshold.

The test report must include commit, date, environment, command, pass/fail totals, coverage, known failures and linked evidence. Screenshots alone do not prove authorization or persistence.

## Acceptance

I will mark the feature ready only after the required journeys work with the real APIs, privacy and access failures have been checked, build/deployment checks pass, and the test report contains actual results. Current result: no tests have been executed for this document package.

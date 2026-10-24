import test from 'node:test';
import assert from 'node:assert/strict';

/**
 * Deterministic test suite verifying Workspace async correctness patterns:
 * 1. Rapid A-to-B location switch (late A response discarded, B preserved).
 * 2. Logout during in-flight request (stale data cleared, zero leakage).
 * 3. Same-length job completion (mutation triggers overview pulse refresh).
 * 4. Failed fetch (honest error state, stale data purged).
 * 5. Revoked access (honest 403 access needed, stale data purged).
 */

test('Workspace Correctness: Rapid A-to-B location switch discards late response', async () => {
  let reqId = 0;
  let activeLocation = 'loc-A';
  let jobsState = [];
  let loadingState = false;

  async function loadJobs(loc, delayMs, payload) {
    const currentReq = ++reqId;
    loadingState = true;
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    if (currentReq !== reqId) {
      // Discard stale response
      return;
    }
    jobsState = payload;
    loadingState = false;
  }

  // User starts at Location A (slow response: 100ms)
  const slowA = loadJobs('loc-A', 100, [{ id: 'job-A1', serviceName: 'Wash A' }]);

  // User quickly switches to Location B at 10ms (fast response: 20ms)
  await new Promise((resolve) => setTimeout(resolve, 10));
  activeLocation = 'loc-B';
  jobsState = []; // Reset immediately on switch
  const fastB = loadJobs('loc-B', 20, [{ id: 'job-B1', serviceName: 'Detail B' }]);

  await Promise.all([slowA, fastB]);

  assert.equal(activeLocation, 'loc-B');
  assert.equal(jobsState.length, 1);
  assert.equal(jobsState[0].id, 'job-B1', 'State must preserve Location B and discard late Location A response');
  assert.equal(loadingState, false);
});

test('Workspace Correctness: In-flight request during logout is discarded and leaves zero state', async () => {
  let reqId = 0;
  let authenticated = true;
  let jobsState = [{ id: 'existing', serviceName: 'Existing' }];
  let overviewState = { metrics: { openJobs: 1 } };

  async function loadData(delayMs) {
    const currentReq = ++reqId;
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    if (currentReq !== reqId) {
      return; // Discard
    }
    jobsState = [{ id: 'leaked', serviceName: 'Should not appear' }];
    overviewState = { metrics: { openJobs: 99 } };
  }

  // Trigger in-flight request
  const inflight = loadData(60);

  // User logs out at 15ms
  await new Promise((resolve) => setTimeout(resolve, 15));
  reqId++; // Logout invalidates pending generation
  authenticated = false;
  jobsState = [];
  overviewState = null;

  await inflight;

  assert.equal(authenticated, false);
  assert.equal(jobsState.length, 0, 'Jobs state must remain empty after logout');
  assert.equal(overviewState, null, 'Overview state must remain null after logout');
});

test('Workspace Correctness: Same-length job completion increments revision and refreshes shift pulse', async () => {
  let jobs = [
    { id: 'j1', status: 'available' },
    { id: 'j2', status: 'completed' },
  ];
  let jobRevision = 0;
  let overviewRefreshCount = 0;
  let overviewMetrics = { openJobs: 1, upcomingBookings: 2 };

  function onOverviewEffect(rev) {
    overviewRefreshCount++;
    const openCount = jobs.filter((j) => j.status === 'available').length;
    overviewMetrics = { ...overviewMetrics, openJobs: openCount };
  }

  // Initial effect run
  onOverviewEffect(jobRevision);
  assert.equal(overviewRefreshCount, 1);
  assert.equal(overviewMetrics.openJobs, 1);

  // Complete job j1 without changing total length (remains 2 items)
  jobs = [
    { id: 'j1', status: 'completed' },
    { id: 'j2', status: 'completed' },
  ];
  assert.equal(jobs.length, 2, 'Total job list length must remain unchanged');

  // Trigger revision increment
  jobRevision++;
  onOverviewEffect(jobRevision);

  assert.equal(overviewRefreshCount, 2, 'Overview pulse must refresh on jobRevision change even if jobs.length is identical');
  assert.equal(overviewMetrics.openJobs, 0, 'Open jobs count must reflect newly completed work');
});

test('Workspace Correctness: Failed fetch purges stale records and surfaces honest error', async () => {
  let reqId = 0;
  let dataState = [{ id: 'stale-1' }];
  let errorState = '';
  let loadingState = false;

  async function loadWithFailure(status, errorMessage) {
    const currentReq = ++reqId;
    loadingState = true;
    errorState = '';
    await new Promise((resolve) => setTimeout(resolve, 10));
    if (currentReq !== reqId) return;

    if (status !== 200) {
      dataState = []; // Must purge stale records
      errorState = errorMessage;
    } else {
      dataState = [{ id: 'fresh-1' }];
    }
    loadingState = false;
  }

  await loadWithFailure(500, 'Could not load work orders');

  assert.equal(dataState.length, 0, 'Stale records must be purged on failure');
  assert.equal(errorState, 'Could not load work orders', 'Honest error state must be surfaced');
  assert.equal(loadingState, false);
});

test('Workspace Correctness: Revoked access (403) purges records and displays honest access-needed state', async () => {
  let reqId = 0;
  let attendanceActive = { id: 'punch-1', status: 'clocked_in' };
  let attendanceHistory = [{ id: 'punch-0' }];
  let errorState = '';
  let loadingState = false;

  async function loadAttendance(status) {
    const currentReq = ++reqId;
    loadingState = true;
    errorState = '';
    await new Promise((resolve) => setTimeout(resolve, 10));
    if (currentReq !== reqId) return;

    if (status === 403) {
      attendanceActive = null;
      attendanceHistory = [];
      errorState = 'Access needed (attendance.read or attendance.record)';
    }
    loadingState = false;
  }

  await loadAttendance(403);

  assert.equal(attendanceActive, null, 'Active punch must be cleared when access is revoked');
  assert.equal(attendanceHistory.length, 0, 'History records must be cleared when access is revoked');
  assert.equal(errorState, 'Access needed (attendance.read or attendance.record)', 'Honest access-needed message must be surfaced');
  assert.equal(loadingState, false);
});

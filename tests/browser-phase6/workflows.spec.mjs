import { test, expect } from '@playwright/test';

const configured = !!(process.env.PHASE6_BASE_URL && process.env.PHASE6_LECTURER_EMAIL && process.env.PHASE6_LECTURER_PASSWORD && process.env.PHASE6_STUDENT_EMAIL && process.env.PHASE6_STUDENT_PASSWORD && process.env.PHASE6_ASSIGNED_OFFERING_ID && process.env.PHASE6_UNASSIGNED_OFFERING_ID);
test.skip(!configured, 'A running development gateway and dedicated synthetic accounts are required.');

async function login(page, email, password) {
  await page.goto('/login');
  await page.getByLabel('Email address').fill(email);
  await page.getByLabel('Password').fill(password);
  const response = page.waitForResponse(value => value.url().endsWith('/api/v1/auth/login'));
  await page.getByRole('button', { name: 'Sign In' }).click();
  expect((await response).status()).toBe(200);
  await expect(page).toHaveURL(/\/courses$/);
}

test('anonymous routes and backend operations deny access', async ({ page, request }) => {
  for (const path of ['/lecturer/offerings', '/student/enrollments', '/admin/dashboard']) {
    await page.goto(path);
    await expect(page).toHaveURL(/\/login$/);
  }
  const response = await request.get('/api/v1/offerings/assigned');
  expect(response.status()).toBe(401);
});

test('lecturer sees only assigned offering and can inspect live analytics', async ({ page }) => {
  const calls = [];
  page.on('response', response => { if (response.url().includes('/api/v1/')) calls.push(response.url()); });
  await login(page, process.env.PHASE6_LECTURER_EMAIL, process.env.PHASE6_LECTURER_PASSWORD);
  await page.goto('/lecturer/offerings');
  await expect(page.getByRole('heading', { name: 'My assigned offerings' })).toBeVisible();
  await expect(page.locator(`a[href="/lecturer/offerings/${process.env.PHASE6_ASSIGNED_OFFERING_ID}"]`)).toBeVisible();
  await page.goto(`/lecturer/offerings/${process.env.PHASE6_ASSIGNED_OFFERING_ID}`);
  await expect(page.getByRole('heading', { name: 'Assessments' })).toBeVisible();
  await page.goto(`/lecturer/offerings/${process.env.PHASE6_ASSIGNED_OFFERING_ID}/analytics`);
  await expect(page.getByRole('heading', { name: 'Offering analytics' })).toBeVisible();
  expect(calls.some(url => url.includes('/offerings/assigned'))).toBeTruthy();
  expect(calls.some(url => url.includes('/analytics/lecturer/offerings/'))).toBeTruthy();
  await page.goto(`/lecturer/offerings/${process.env.PHASE6_UNASSIGNED_OFFERING_ID}`);
  await expect(page.getByRole('alert')).toContainText(/not assigned|denied|forbidden/i);
  const forbidden = await page.context().request.get(`/api/v1/offerings/${process.env.PHASE6_UNASSIGNED_OFFERING_ID}/enrollments`);
  expect(forbidden.status()).toBe(403);
  await page.getByRole('button', { name: 'Log out' }).click();
  await page.goto('/lecturer/offerings');
  await expect(page).toHaveURL(/\/login$/);
});

test('student enrolments, report and lecturer/admin denial use live APIs', async ({ page }) => {
  const calls = [];
  page.on('response', response => { if (response.url().includes('/api/v1/')) calls.push(response.url()); });
  await login(page, process.env.PHASE6_STUDENT_EMAIL, process.env.PHASE6_STUDENT_PASSWORD);
  await page.goto('/student/enrollments');
  await expect(page.getByRole('heading', { name: 'My enrolments' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'My enrolments' })).toBeVisible();
  await page.goto('/student/report');
  await expect(page.getByRole('heading', { name: 'Academic report' })).toBeVisible();
  expect((await page.context().request.get('/api/v1/analytics/admin/summary')).status()).toBe(403);
  await page.goto('/lecturer/offerings');
  await expect(page.getByRole('alert')).toContainText('Access denied');
  await page.goto('/admin/dashboard');
  await expect(page.getByRole('alert')).toContainText('Access denied');
  expect(calls.some(url => url.includes('/students/') && url.includes('/enrollments'))).toBeTruthy();
  expect(calls.some(url => url.includes('/analytics/students/me'))).toBeTruthy();
  const cookies = await page.context().cookies();
  const retained = cookies.filter(cookie => !cookie.name.endsWith('access_token'));
  await page.context().clearCookies();
  await page.context().addCookies(retained);
  const refresh = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/refresh'));
  await page.goto('/student/enrollments');
  expect((await refresh).status()).toBe(200);
  await expect(page.getByRole('heading', { name: 'My enrolments' })).toBeVisible();
  await page.context().clearCookies();
  await page.reload();
  await expect(page).toHaveURL(/\/login$/);
});

test('student file upload persists and lecturer can download, grade and release it', async ({ browser, request }) => {
  const lecturer = await browser.newContext(); const lecturerPage = await lecturer.newPage();
  await login(lecturerPage, process.env.PHASE6_LECTURER_EMAIL, process.env.PHASE6_LECTURER_PASSWORD);
  await lecturerPage.goto(`/lecturer/offerings/${process.env.PHASE6_ASSIGNED_OFFERING_ID}/assessments/create`);
  await lecturerPage.getByLabel('Title').fill(`Phase 6 synthetic ${Date.now()}`);
  await lecturerPage.getByLabel('Description').fill('Synthetic end-to-end assessment');
  await lecturerPage.getByLabel('Due date').fill(new Date(Date.now() + 86400000).toISOString().slice(0, 16));
  await lecturerPage.getByLabel('Maximum mark').fill('100');
  await lecturerPage.getByLabel('Weight percent').fill('1');
  const created = lecturerPage.waitForResponse(response => response.url().includes('/assessments') && response.request().method() === 'POST');
  await lecturerPage.getByRole('button', { name: 'Create draft' }).click();
  expect((await created).status()).toBe(201);
  await expect(lecturerPage).toHaveURL(/\/lecturer\/assessments\//);
  const assessmentId = lecturerPage.url().split('/').pop();
  const published = lecturerPage.waitForResponse(response => response.url().endsWith(`/assessments/${assessmentId}/publish`));
  await lecturerPage.getByRole('button', { name: 'Publish assessment' }).click();
  expect((await published).status()).toBe(200);
  await lecturerPage.reload();
  await expect(lecturerPage.getByText(/Status: published/)).toBeVisible();
  const student = await browser.newContext(); const studentPage = await student.newPage();
  await login(studentPage, process.env.PHASE6_STUDENT_EMAIL, process.env.PHASE6_STUDENT_PASSWORD);
  await studentPage.goto(`/student/assessments/${assessmentId}`);
  const bytes = Buffer.from('%PDF-1.4\n1 0 obj <<>> endobj\n%%EOF\n');
  await studentPage.getByLabel('Submission file').setInputFiles({ name: 'phase6-synthetic.pdf', mimeType: 'application/pdf', buffer: bytes });
  const upload = studentPage.waitForResponse(response => response.url().includes('/submissions') && response.request().method() === 'POST');
  await studentPage.getByRole('button', { name: 'Upload submission' }).click();
  const uploadResponse = await upload;
  expect(uploadResponse.status()).toBe(201);
  expect((await uploadResponse.request().allHeaders())['x-csrf-token']).toBeTruthy();
  await expect(studentPage).toHaveURL(/\/student\/submissions\//);
  const submissionId = studentPage.url().split('/').pop();
  await studentPage.reload();
  await expect(studentPage.getByText('phase6-synthetic.pdf')).toBeVisible();
  await lecturerPage.goto(`/lecturer/assessments/${assessmentId}`);
  await expect(lecturerPage.locator(`a[href="/lecturer/submissions/${submissionId}"]`)).toBeVisible();
  await lecturerPage.goto(`/lecturer/submissions/${submissionId}`);
  const download = lecturerPage.waitForResponse(response => response.url().endsWith(`/submissions/${submissionId}/file`));
  await lecturerPage.getByRole('button', { name: 'Download protected file' }).click();
  expect((await download).status()).toBe(200);
  await lecturerPage.getByLabel('Mark').fill('75');
  await lecturerPage.getByLabel('Feedback').fill('Synthetic Phase 6 browser test');
  await lecturerPage.getByRole('button', { name: 'Save grade' }).click();
  await expect(lecturerPage.getByText(/Saved mark: 75/)).toBeVisible();
  await lecturerPage.getByRole('button', { name: 'Release grade' }).click();
  await expect(lecturerPage.getByText(/released/)).toBeVisible();
  await studentPage.reload();
  await expect(studentPage.getByText(/Grade: 75/)).toBeVisible();
  expect((await request.get(`/api/v1/submissions/${submissionId}`)).status()).toBe(401);
  await lecturer.close(); await student.close();
});

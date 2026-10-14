import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

test('dashboard and all modules render without runtime errors', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Operations overview' })).toBeVisible();
  await page.screenshot({ path: 'test-results/dashboard-desktop.png', fullPage: true });
  for (const [path, title] of [
    ['patients', 'Patients'],
    ['therapy', 'Therapy orders'],
    ['insurance', 'Insurance authorizations'],
    ['pharmacy', 'Pharmacy readiness'],
    ['appointments', 'Appointment scheduling'],
    ['billing', 'Billing & claims'],
    ['workflows', 'Workflow executions'],
    ['scheduler', 'Job scheduler'],
    ['designer', 'Workflow designer'],
    ['databases', 'Database connections'],
    ['concurrency', 'Worker pools & concurrency'],
    ['memory', 'Memory & resource governor'],
    ['events', 'Kafka event delivery'],
    ['callbacks', 'Callback monitoring'],
    ['health', 'System health'],
    ['configuration', 'Workspace configuration'],
    ['users', 'Users & access'],
    ['audit', 'Audit logs'],
    ['alerts', 'Alerts & notifications'],
  ]) {
    await page.goto(`/${path}`);
    await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
  }
  expect(errors).toEqual([]);
});

test('registration stays isolated when switching workspaces', async ({ page }) => {
  await page.goto('/patients');
  await page.getByRole('button', { name: 'Register patient' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Full name', { exact: true }).fill('Synthetic QA Patient');
  await dialog.getByLabel('Patient reference', { exact: true }).fill('QA-ISOLATION-1');
  await dialog.getByLabel('Date of birth', { exact: true }).fill('1992-01-01');
  await dialog.getByRole('combobox', { name: 'Care facility' }).click();
  await page.getByRole('option', { name: 'Downtown Infusion Center', exact: true }).click();
  await dialog.getByRole('button', { name: 'Save patient' }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole('textbox', { name: 'Search patients' }).fill('Synthetic QA');
  await expect(page.getByText('Synthetic QA Patient', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Northstar Health/ }).click();
  await page.getByRole('menuitem', { name: 'Southridge Care', exact: true }).click();
  await page.getByRole('link', { name: 'Patients', exact: true }).click();
  await page.getByRole('textbox', { name: 'Search patients' }).fill('Synthetic QA');
  await expect(page.getByText('Synthetic QA Patient', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: /Southridge Care/ }).click();
  await page.getByRole('menuitem', { name: 'Northstar Health', exact: true }).click();
  await page.getByRole('link', { name: 'Patients', exact: true }).click();
  await page.getByRole('textbox', { name: 'Search patients' }).fill('Synthetic QA');
  await expect(page.getByText('Synthetic QA Patient', { exact: true })).toBeVisible();
});

test('authorization, booking and billing advance the same workflow', async ({ page }) => {
  await page.goto('/workflows/nh-w001');
  let dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: 'Approve', exact: true }).click();
  await page.getByRole('button', { name: 'Record outcome', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(1);
  await dialog.getByRole('button', { name: 'Mark available', exact: true }).click();
  await page.getByRole('button', { name: 'Record outcome', exact: true }).click();
  await expect(dialog.getByText('Ready to schedule', { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: 'Close record details' }).click();
  await page.getByRole('link', { name: 'Appointments', exact: true }).click();
  await page.getByRole('button', { name: 'Book appointment' }).click();
  dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox', { name: 'Eligible therapy order' }).click();
  await page.getByRole('option', { name: 'Avery Morgan · Immunoglobulin (IVIG)', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'Infusion-qualified nurse' }).click();
  await page
    .getByRole('option', { name: 'Olivia Carter, RN · Downtown Infusion Center', exact: true })
    .click();
  await dialog.getByRole('combobox', { name: 'Chair or room' }).click();
  await page
    .getByRole('option', { name: 'Infusion chair 1 · Downtown Infusion Center', exact: true })
    .click();
  const future = new Date();
  future.setDate(future.getDate() + 7);
  const day = `${future.getFullYear()}-${String(future.getMonth() + 1).padStart(2, '0')}-${String(future.getDate()).padStart(2, '0')}`;
  await dialog.getByLabel('Starts at', { exact: true }).fill(`${day}T09:00`);
  await dialog.getByLabel('Ends at', { exact: true }).fill(`${day}T10:00`);
  await dialog.getByRole('button', { name: 'Confirm booking' }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole('link', { name: 'Workflows', exact: true }).click();
  await page.getByRole('button', { name: 'View record nh-w001' }).click();
  dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: 'Record completed' }).click();
  await page.getByRole('button', { name: 'Record outcome', exact: true }).click();
  await expect(dialog.getByText('Awaiting billing', { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: 'Record submitted' }).click();
  await page.getByRole('button', { name: 'Record outcome', exact: true }).click();
  await expect(dialog.getByText('Completed', { exact: true }).first()).toBeVisible();
});

test('role navigation and direct routes both respect permissions', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Open user menu' }).click();
  await page.getByRole('menuitem', { name: 'Preview Auditor role' }).click();
  await expect(page.getByRole('link', { name: 'Patients', exact: true })).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Job scheduler', exact: true })).toBeVisible();
  await page.evaluate(() => {
    history.pushState(null, '', '/patients');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await expect(page.getByText('This area is restricted', { exact: true })).toBeVisible();
});

test('privacy masking hides demographics and dark theme remains usable', async ({ page }) => {
  await page.goto('/patients');
  await expect(page.getByText('Avery Morgan', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Hide patient details', exact: true }).click();
  await expect(page.getByText('Avery Morgan', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Toggle theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.screenshot({ path: 'test-results/patients-dark.png', fullPage: true });
});

test('mobile navigation and dashboard avoid horizontal overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Operations overview' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: 'test-results/dashboard-mobile.png', fullPage: true });
  await page.getByRole('button', { name: 'Open navigation' }).click();
  await page.getByRole('link', { name: 'Patients', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Patients', exact: true })).toBeVisible();
});

test('workflow drafts validate and save without changing active therapies', async ({ page }) => {
  await page.goto('/designer');
  await page.getByRole('button', { name: 'Validate workflow' }).click();
  await expect(page.getByText('Workflow is valid. All steps are connected without cycles.')).toBeVisible();
  await page.getByRole('button', { name: 'Save draft' }).click();
  await expect(
    page.getByText('Draft saved on this device. Active therapy workflows are unchanged.'),
  ).toBeVisible();
  await expect(page.getByText('Draft v2', { exact: true })).toBeVisible();
});

test('dashboard meets automated accessibility checks in both themes', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Operations overview' })).toBeVisible();
  for (const mode of ['light', 'dark']) {
    if (mode === 'dark') await page.getByRole('button', { name: 'Toggle theme' }).click();
    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'])
      .analyze();
    expect(results.violations.map((v) => ({ mode, id: v.id, nodes: v.nodes.map((n) => n.target) }))).toEqual([]);
  }
});

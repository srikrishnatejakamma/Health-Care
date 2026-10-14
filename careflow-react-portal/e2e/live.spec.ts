import { test, expect } from '@playwright/test';

test.use({ trace: 'off', screenshot: 'off' });

test.describe('live Go API', () => {
  test('creates and edits real tenant records and renders outbox events', async ({ page }) => {
    test.skip(
      !process.env.CAREFLOW_TEST_TOKEN,
      'Supply a local synthetic-tenant token to verify the Go API.',
    );
    const suffix = Date.now().toString();
    const facility = `UI verification ${suffix}`;
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.goto('/login');
    await page.getByRole('textbox', { name: 'Workspace ID', exact: true }).fill('north-health');
    await page.getByLabel(/Access token/).fill(process.env.CAREFLOW_TEST_TOKEN!);
    await page.getByRole('button', { name: 'Connect workspace', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Operations overview' })).toBeVisible();
    await expect(page.getByText('LIVE', { exact: true })).toBeVisible();
    await page.getByRole('link', { name: 'Configuration', exact: true }).click();
    await page.getByRole('tab', { name: 'Facilities & resources' }).click();
    await page.getByRole('button', { name: 'Add facility', exact: true }).click();
    let dialog = page.getByRole('dialog');
    await dialog.getByLabel('Facility name', { exact: true }).fill(facility);
    await dialog.getByRole('button', { name: 'Save facility', exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(facility, { exact: true })).toBeVisible();
    await page.getByRole('link', { name: 'Patients', exact: true }).click();
    await page.getByRole('button', { name: 'Register patient', exact: true }).click();
    dialog = page.getByRole('dialog');
    await dialog.getByLabel('Full name', { exact: true }).fill('Synthetic Live UI');
    await dialog.getByLabel('Patient reference', { exact: true }).fill(`UI-${suffix}`);
    await dialog.getByLabel('Date of birth', { exact: true }).fill('1992-01-01');
    await dialog.getByRole('combobox', { name: 'Care facility' }).click();
    await page.getByRole('option', { name: facility, exact: true }).click();
    await dialog.getByRole('button', { name: 'Save patient', exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await page.getByRole('textbox', { name: 'Search patients' }).fill(`UI-${suffix}`);
    await page.getByText('Synthetic Live UI', { exact: true }).click();
    dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Edit patient', exact: true }).click();
    const edit = page
      .getByRole('dialog')
      .filter({ has: page.getByRole('heading', { name: 'Edit patient' }) });
    await edit.getByLabel('Full name', { exact: true }).fill('Synthetic Live Edited');
    await edit.getByRole('button', { name: 'Save changes', exact: true }).click();
    await expect(edit).not.toBeVisible();
    await expect(dialog.getByText('Synthetic Live Edited', { exact: true })).toBeVisible();
    await dialog.getByRole('button', { name: 'Close record details' }).click();
    await page.getByRole('link', { name: 'Kafka events', exact: true }).click();
    await expect(page.getByRole('grid')).toBeVisible();
    await expect(page.getByText('Patient Updated', { exact: true })).toBeVisible();
    await page.getByRole('link', { name: 'Memory', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Memory & resource governor' })).toBeVisible();
    const persisted = await page.evaluate(() =>
      JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }),
    );
    expect(persisted.includes(process.env.CAREFLOW_TEST_TOKEN!)).toBe(false);
    expect(persisted).not.toContain('Synthetic Live Edited');
    expect(errors).toEqual([]);
  });
});

import { test, expect } from '@playwright/test';

test.describe('Subscription Flow', () => {
  const TEST_EMAIL = 'test-e2e@example.com';
  const TEST_REPO = 'golang/go';
  const TEST_API_KEY = 'test-api-key-12345';

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should have the correct title', async ({ page }) => {
    await expect(page).toHaveTitle(/Release Watch/);
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Release Watch');
  });

  test('should allow saving an API key', async ({ page }) => {
    const apiKeyInput = page.locator('#api-key');
    const saveButton = page.locator('#save-api-key');
    const status = page.locator('#api-key-status');

    await apiKeyInput.fill(TEST_API_KEY);
    await saveButton.click();

    await expect(status).toHaveText('API key saved.');
    await expect(status).toHaveClass(/ok/);

    // Refresh and check if it's still there (localStorage)
    await page.reload();
    await expect(apiKeyInput).toHaveValue(TEST_API_KEY);
    await expect(status).toHaveText('Using saved API key.');
  });

  test('should show validation errors for empty subscription form', async ({ page }) => {
    const submitButton = page.locator('#subscribe-form button[type="submit"]');
    
    // Playwright doesn't easily check HTML5 validation bubbles, 
    // but we can check if the form was NOT submitted by checking status
    await submitButton.click();
    
    const status = page.locator('#subscribe-status');
    await expect(status).toBeEmpty();
  });

  test('should handle a successful subscription request', async ({ page }) => {
    // First save the API key to avoid 401
    await page.locator('#api-key').fill(TEST_API_KEY);
    await page.locator('#save-api-key').click();

    // Mock the API response to avoid dependency on GitHub/Email in this specific test
    // or just let it run if the environment is set up with mocks
    await page.locator('#subscribe-email').fill(TEST_EMAIL);
    await page.locator('#subscribe-repo').fill(TEST_REPO);

    // We can intercept the request if we want to be purely UI focused
    await page.route('**/api/subscribe', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'subscription created; confirmation email sent' }),
      });
    });

    await page.locator('#subscribe-form button[type="submit"]').click();

    const status = page.locator('#subscribe-status');
    await expect(status).toHaveText(/subscription created/i);
    await expect(status).toHaveClass(/ok/);
    
    // Check if email was auto-filled in lookup form
    await expect(page.locator('#lookup-email')).toHaveValue(TEST_EMAIL);
  });

  test('should handle loading subscriptions', async ({ page }) => {
    await page.locator('#api-key').fill(TEST_API_KEY);
    await page.locator('#save-api-key').click();

    await page.locator('#lookup-email').fill(TEST_EMAIL);

    await page.route('**/ui/subscriptions**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            email: TEST_EMAIL,
            repo: TEST_REPO,
            confirmed: true,
            last_seen_tag: 'v1.22.0',
            unsubscribe_token: 'test-token-123'
          }
        ]),
      });
    });

    await page.locator('#lookup-form button[type="submit"]').click();

    const results = page.locator('#lookup-results');
    await expect(results.locator('.subscription')).toBeVisible();
    await expect(results.locator('.repo')).toHaveText(TEST_REPO);
    await expect(results.locator('.meta').first()).toHaveText(/Confirmed: yes/);
  });
});

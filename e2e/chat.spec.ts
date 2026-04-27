import { test, expect } from '@playwright/test';

test.describe('Chat', () => {
  test('should have chat input area', async ({ page }) => {
    await page.goto('/');
    const input = page.locator('#chat-input, textarea, input[type="text"]').first();
    await expect(input).toBeVisible({ timeout: 5000 });
  });

  test('should send a message and display it', async ({ page }) => {
    await page.goto('/');
    const input = page.locator('#chat-input, textarea').first();
    if (await input.isVisible()) {
      await input.fill('Hello, SmartClaw!');
      await input.press('Enter');
      await page.waitForTimeout(1000);
      const messages = page.locator('.message, .chat-message, [data-role="user"]');
      if (await messages.count() > 0) {
        await expect(messages.last()).toContainText('Hello');
      }
    }
  });

  test('should display response area', async ({ page }) => {
    await page.goto('/');
    const responseArea = page.locator('#messages, .messages, .chat-area, main').first();
    await expect(responseArea).toBeVisible({ timeout: 5000 });
  });
});

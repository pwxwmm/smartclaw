import { test, expect } from '@playwright/test';

test.describe('WebSocket Connection', () => {
  test('should load the main page', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('body')).toBeVisible();
  });

  test('should establish WebSocket connection', async ({ page }) => {
    const wsConnected = await page.evaluate(() => {
      return new Promise<boolean>((resolve) => {
        const ws = new WebSocket(`ws://localhost:8080/ws`);
        ws.onopen = () => {
          ws.close();
          resolve(true);
        };
        ws.onerror = () => resolve(false);
        setTimeout(() => resolve(false), 5000);
      });
    });
    expect(wsConnected).toBe(true);
  });

  test('should reconnect after disconnect', async ({ page }) => {
    await page.goto('/');
    const reconnectCount = await page.evaluate(() => {
      return new Promise<number>((resolve) => {
        let count = 0;
        const ws1 = new WebSocket(`ws://localhost:8080/ws`);
        ws1.onopen = () => {
          ws1.close();
          const ws2 = new WebSocket(`ws://localhost:8080/ws`);
          ws2.onopen = () => {
            count++;
            ws2.close();
            resolve(count);
          };
          ws2.onerror = () => resolve(count);
        };
        ws1.onerror = () => resolve(count);
        setTimeout(() => resolve(count), 10000);
      });
    });
    expect(reconnectCount).toBe(1);
  });
});

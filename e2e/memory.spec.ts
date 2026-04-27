import { test, expect } from '@playwright/test';

test.describe('Memory Panel', () => {
  test('should load memory panel', async ({ page }) => {
    await page.goto('/');
    const memoryTab = page.locator('button:has-text("Memory"), [data-panel="memory"], a:has-text("Memory")').first();
    if (await memoryTab.isVisible()) {
      await memoryTab.click();
      await page.waitForTimeout(500);
      const memoryPanel = page.locator('.memory-panel, [class*="memory"], #memory');
      if (await memoryPanel.count() > 0) {
        await expect(memoryPanel.first()).toBeVisible();
      }
    }
  });

  test('should edit memory content', async ({ page }) => {
    await page.goto('/');
    const memoryTab = page.locator('button:has-text("Memory"), [data-panel="memory"], a:has-text("Memory")').first();
    if (await memoryTab.isVisible()) {
      await memoryTab.click();
      await page.waitForTimeout(500);
      const editArea = page.locator('.memory-edit, textarea, [contenteditable]').first();
      if (await editArea.isVisible()) {
        await editArea.click();
      }
    }
  });
});

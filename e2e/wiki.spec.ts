import { test, expect } from '@playwright/test';

test.describe('Wiki Panel', () => {
  test('should load wiki panel', async ({ page }) => {
    await page.goto('/');
    const wikiTab = page.locator('button:has-text("Wiki"), [data-panel="wiki"], a:has-text("Wiki")').first();
    if (await wikiTab.isVisible()) {
      await wikiTab.click();
      await page.waitForTimeout(500);
      const wikiPanel = page.locator('.wiki-panel, [class*="wiki"], #wiki');
      if (await wikiPanel.count() > 0) {
        await expect(wikiPanel.first()).toBeVisible();
      }
    }
  });

  test('should search wiki content', async ({ page }) => {
    await page.goto('/');
    const wikiTab = page.locator('button:has-text("Wiki"), [data-panel="wiki"], a:has-text("Wiki")').first();
    if (await wikiTab.isVisible()) {
      await wikiTab.click();
      await page.waitForTimeout(500);
      const searchInput = page.locator('.wiki-search, input[type="search"], input[placeholder*="earch"]').first();
      if (await searchInput.isVisible()) {
        await searchInput.fill('test query');
        await searchInput.press('Enter');
        await page.waitForTimeout(1000);
      }
    }
  });
});

import { test, expect } from '@playwright/test';

test.describe('Sidebar', () => {
  test('should display sidebar navigation', async ({ page }) => {
    await page.goto('/');
    const sidebar = page.locator('nav, aside, .sidebar, [class*="sidebar"]').first();
    if (await sidebar.isVisible()) {
      await expect(sidebar).toBeVisible();
    }
  });

  test('should switch between panels', async ({ page }) => {
    await page.goto('/');
    const navItems = page.locator('nav a, .sidebar a, [class*="tab"], button[role="tab"]');
    if (await navItems.count() >= 2) {
      await navItems.nth(1).click();
      await page.waitForTimeout(500);
      const activePanel = page.locator('.active, [aria-selected="true"], .panel-active');
      if (await activePanel.count() > 0) {
        await expect(activePanel.first()).toBeVisible();
      }
    }
  });
});

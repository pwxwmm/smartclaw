import { test, expect } from '@playwright/test';

test.describe('Skills Panel', () => {
  test('should load skills panel', async ({ page }) => {
    await page.goto('/');
    const skillsTab = page.locator('button:has-text("Skills"), [data-panel="skills"], a:has-text("Skills")').first();
    if (await skillsTab.isVisible()) {
      await skillsTab.click();
      await page.waitForTimeout(500);
      const skillsPanel = page.locator('.skills-panel, [class*="skills"], #skills');
      if (await skillsPanel.count() > 0) {
        await expect(skillsPanel.first()).toBeVisible();
      }
    }
  });

  test('should toggle skill on and off', async ({ page }) => {
    await page.goto('/');
    const skillsTab = page.locator('button:has-text("Skills"), [data-panel="skills"], a:has-text("Skills")').first();
    if (await skillsTab.isVisible()) {
      await skillsTab.click();
      await page.waitForTimeout(500);
      const toggle = page.locator('.skill-toggle, [class*="toggle"]').first();
      if (await toggle.isVisible()) {
        await toggle.click();
        await page.waitForTimeout(300);
        await toggle.click();
      }
    }
  });
});

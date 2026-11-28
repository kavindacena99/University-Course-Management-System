import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: '.',
  timeout: 60000,
  use: { baseURL: process.env.PHASE6_BASE_URL, channel: process.env.PHASE6_BROWSER_CHANNEL || 'chrome', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'artifacts/report' }]],
  outputDir: 'artifacts/results',
});

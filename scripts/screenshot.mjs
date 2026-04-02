const { chromium } = (await import('playwright')).default
  ? (await import('playwright')).default
  : await import('playwright');

const url = process.env.SCREENSHOT_URL || 'http://localhost:8080';
console.log(`📸 Screenshot: ${url}`);
const browser = await chromium.launch({ args: ['--no-sandbox', '--disable-dev-shm-usage'] });
const page = await browser.newPage();
await page.setViewportSize({ width: 1280, height: 900 });
try {
  await page.goto(url, { timeout: 30000, waitUntil: 'networkidle' });
} catch {
  try { await page.goto(url, { timeout: 15000 }); } catch { console.log('Timeout, taking screenshot anyway'); }
}
await page.screenshot({ path: '/tmp/demo-app.png', fullPage: true });
console.log('✅ Screenshot gespeichert');
await browser.close();

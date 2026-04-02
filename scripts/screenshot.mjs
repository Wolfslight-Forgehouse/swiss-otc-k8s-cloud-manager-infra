import { chromium } from 'playwright';
const url = `http://${process.env.ELB_IP}`;
console.log(`📸 Screenshot: ${url}`);
const browser = await chromium.launch({ args: ['--no-sandbox'] });
const page = await browser.newPage();
await page.setViewportSize({ width: 1280, height: 900 });
try {
  await page.goto(url, { timeout: 30000, waitUntil: 'networkidle' });
} catch {
  await page.goto(url, { timeout: 15000 });
}
await page.screenshot({ path: '/tmp/demo-app.png', fullPage: true });
console.log('✅ Screenshot gespeichert');
await browser.close();

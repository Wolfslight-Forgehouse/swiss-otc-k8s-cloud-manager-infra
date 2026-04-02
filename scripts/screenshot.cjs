const playwright = require('/tmp/node_modules/playwright');

(async () => {
  const url = process.env.SCREENSHOT_URL || 'http://localhost:8080';
  console.log(`📸 Screenshot: ${url}`);
  const browser = await playwright.chromium.launch({
    args: ['--no-sandbox', '--disable-dev-shm-usage']
  });
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 900 });
  try {
    await page.goto(url, { timeout: 30000, waitUntil: 'networkidle' });
  } catch (e) {
    console.log('networkidle timeout, continuing...');
    try { await page.goto(url, { timeout: 15000 }); } catch (_) {}
  }
  await page.screenshot({ path: '/tmp/demo-app.png', fullPage: true });
  console.log('✅ Screenshot gespeichert: /tmp/demo-app.png');
  await browser.close();
})();

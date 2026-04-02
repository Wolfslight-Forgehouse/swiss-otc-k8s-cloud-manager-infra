const playwright = require('/tmp/node_modules/playwright');

(async () => {
  const url = process.env.SCREENSHOT_URL || 'http://localhost:8080';
  const outPath = process.env.SCREENSHOT_OUT || '/tmp/demo-app.png';
  console.log(`📸 Screenshot: ${url}`);
  const browser = await playwright.chromium.launch({
    args: ['--no-sandbox', '--disable-dev-shm-usage']
  });
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 900 });
  try {
    await page.goto(url, { timeout: 45000, waitUntil: 'domcontentloaded' });
    // Extra 3s für JS/CSS rendering
    await page.waitForTimeout(3000);
  } catch (e) {
    console.log(`goto error: ${e.message}`);
  }
  await page.screenshot({ path: outPath, fullPage: true });
  console.log(`✅ Screenshot gespeichert: ${outPath}`);
  await browser.close();
})();

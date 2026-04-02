const pw = require('/tmp/node_modules/playwright');
const path = require('path');
const fs = require('fs');

const htmlFile = process.env.HTML_FILE || '/tmp/demo-app.html';
const outFile  = process.env.SCREENSHOT_OUT || '/tmp/demo-app.png';

(async () => {
  if (!fs.existsSync(htmlFile)) {
    console.error(`❌ HTML file not found: ${htmlFile}`);
    process.exit(1);
  }
  const fileSize = fs.statSync(htmlFile).size;
  console.log(`📄 HTML: ${htmlFile} (${fileSize} bytes)`);

  const url = process.env.SCREENSHOT_URL || ('file://' + path.resolve(htmlFile));
  console.log(`📸 Rendering: ${url}`);

  const browser = await pw.chromium.launch({
    args: ['--no-sandbox', '--disable-dev-shm-usage', '--allow-file-access-from-files']
  });
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 900 });
  try {
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
    await page.waitForTimeout(2000);
  } catch (e) {
    console.log(`goto info: ${e.message}`);
  }
  await page.screenshot({ path: outFile, fullPage: true });
  const outSize = fs.statSync(outFile).size;
  console.log(`✅ Screenshot: ${outFile} (${outSize} bytes)`);
  await browser.close();
})();

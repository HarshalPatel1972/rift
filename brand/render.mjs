// Renders every icon RIFT ships from brand/logo.svg and brand/logo-mono.svg.
//
//   npm i -g puppeteer-core          (once; uses your installed Edge/Chrome)
//   node brand/render.mjs
//   go run ./cmd/icongen -o cmd/rift/rift.ico brand/png/icon-{16,20,24,32,40,48,64,128,256}.png
//
// Set BROWSER to a Chrome/Edge executable if it isn't found automatically, or
// PUPPETEER to puppeteer-core's entry file if it isn't resolvable.
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const puppeteer = (await import(process.env.PUPPETEER ? pathToFileURL(process.env.PUPPETEER).href : 'puppeteer-core')).default;

const browsers = [
  process.env.BROWSER,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome',
].filter(Boolean);
const executablePath = browsers.find((p) => existsSync(p));

const logo = readFileSync(join(here, 'logo.svg'));
const mono = readFileSync(join(here, 'logo-mono.svg'));

// [svg, size, output path relative to repo root]
const outputs = [
  ...[16, 20, 24, 32, 40, 48, 64, 128, 256].map((s) => [logo, s, `brand/png/icon-${s}.png`]),
  [logo, 1024, 'brand/png/icon-1024.png'],
  [logo, 512, 'web/icon.png'],
  [logo, 192, 'web/phone/icon-192.png'],
  [logo, 512, 'web/phone/icon-512.png'],
  [logo, 192, 'site/img/icon.png'],
  [logo, 64, 'site/img/favicon.png'],
  [mono, 44, 'cmd/rift/tray_template.png'],
];

const browser = await puppeteer.launch({ executablePath, headless: 'new' });
const page = await browser.newPage();
for (const [svg, size, out] of outputs) {
  await page.setViewport({ width: size, height: size, deviceScaleFactor: 1 });
  await page.setContent(
    `<html><body style="margin:0;background:transparent">` +
      `<img src="data:image/svg+xml;base64,${svg.toString('base64')}" width="${size}" height="${size}" style="display:block"></body></html>`,
  );
  await page.waitForSelector('img');
  await page.evaluate(() => document.querySelector('img').decode());
  const png = await page.screenshot({ omitBackground: true, clip: { x: 0, y: 0, width: size, height: size } });
  mkdirSync(dirname(join(root, out)), { recursive: true });
  writeFileSync(join(root, out), png);
  console.log(`${String(size).padStart(4)}px → ${out}`);
}
await browser.close();

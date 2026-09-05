#!/usr/bin/env node

/**
 * generate_datamatrix_pdf.js
 *
 * Standalone Node.js script to generate a printable A4 PDF containing a grid
 * of random, unique PocketBase-compatible Data Matrix barcodes.
 *
 * Requirements:
 * - PocketBase ID format: 15 characters, lowercase alphanumeric ([a-z0-9]{15}).
 * - Barcode dimensions: 15mm x 15mm vector Data Matrix (via bwip-js).
 * - Page layout: Standard A4 portrait (210mm x 297mm) with subtle cutting guides.
 * - Monospace ID printed cleanly under each barcode.
 * - PDF conversion via headless Google Chrome / Chromium.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { execSync } = require('child_process');

// 1. Resolve bwip-js dependency
function loadBwipJs() {
  const candidates = [
    'bwip-js',
    path.resolve(__dirname, '../web/node_modules/bwip-js'),
    path.resolve(process.cwd(), 'web/node_modules/bwip-js'),
    path.resolve(__dirname, '../node_modules/bwip-js')
  ];

  for (const candidate of candidates) {
    try {
      return require(candidate);
    } catch {
      // Continue searching
    }
  }

  console.error('Error: Could not locate bwip-js module.');
  console.error('Expected at ./web/node_modules/bwip-js or in NODE_PATH.');
  process.exit(1);
}

const bwipjs = loadBwipJs();

// 2. Locate Chrome / Chromium executable
function findChromeBinary() {
  const candidates = [
    process.env.CHROME_BIN,
    'google-chrome',
    'google-chrome-stable',
    'chromium',
    'chromium-browser',
    '/usr/bin/google-chrome',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser'
  ].filter(Boolean);

  for (const bin of candidates) {
    try {
      execSync(`which "${bin}" 2>/dev/null`);
      return bin;
    } catch {
      // Continue searching
    }
  }

  return null;
}

// 3. Generate PocketBase-compatible unique IDs: [a-z0-9]{15}
const ALPHABET = 'abcdefghijklmnopqrstuvwxyz0123456789';

function generatePocketBaseId() {
  let id = '';
  for (let i = 0; i < 15; i++) {
    const randIdx = crypto.randomInt(0, ALPHABET.length);
    id += ALPHABET[randIdx];
  }
  return id;
}

function generateUniquePocketBaseIds(count) {
  const ids = new Set();
  while (ids.size < count) {
    ids.add(generatePocketBaseId());
  }
  return Array.from(ids);
}

// 4. Parse CLI arguments
function parseArgs() {
  const args = process.argv.slice(2);
  const options = {
    pages: 1,
    cols: 8,
    rows: 11,
    output: 'datamatrix_a4_grid.pdf',
    keepHtml: false,
    help: false
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--help' || arg === '-h') {
      options.help = true;
    } else if ((arg === '--pages' || arg === '-p') && args[i + 1]) {
      options.pages = parseInt(args[++i], 10);
    } else if ((arg === '--output' || arg === '-o') && args[i + 1]) {
      options.output = args[++i];
    } else if ((arg === '--cols' || arg === '-c') && args[i + 1]) {
      options.cols = parseInt(args[++i], 10);
    } else if ((arg === '--rows' || arg === '-r') && args[i + 1]) {
      options.rows = parseInt(args[++i], 10);
    } else if (arg === '--keep-html') {
      options.keepHtml = true;
    }
  }

  return options;
}

function printUsage() {
  console.log(`
Usage: node scripts/generate_datamatrix_pdf.js [options]

Options:
  -p, --pages <number>   Number of A4 pages to generate (default: 1)
  -o, --output <path>    Output PDF file path (default: datamatrix_a4_grid.pdf)
  -c, --cols <number>    Number of columns per page (default: 8)
  -r, --rows <number>    Number of rows per page (default: 11)
      --keep-html        Keep intermediate HTML template alongside PDF
  -h, --help             Show this help message

Default Grid Layout:
  - 8 columns x 11 rows = 88 barcodes per page
  - Cell size: 24mm x 25mm (with 0.5px dashed cutting guides)
  - Barcode: 15mm x 15mm vector Data Matrix
  - Text label: 15-character PocketBase ID in 5.5pt monospace
  - Page size: A4 (210mm x 297mm) with 9mm horizontal & 11mm vertical margins
`);
}

// 5. Build HTML content
function buildHtml({ pages, cols, rows }) {
  const codesPerPage = cols * rows;
  const totalCodes = codesPerPage * pages;
  const allIds = generateUniquePocketBaseIds(totalCodes);

  // Cell & Margin calculations (defaults: 8x11 -> 24mm x 25mm, margins 9mm x 11mm)
  const cellWidthMm = (cols === 8) ? 24 : +(192 / cols).toFixed(2);
  const cellHeightMm = (rows === 11) ? 25 : +(275 / rows).toFixed(2);
  const gridWidthMm = +(cellWidthMm * cols).toFixed(2);
  const gridHeightMm = +(cellHeightMm * rows).toFixed(2);
  const horizontalMarginMm = +((210 - gridWidthMm) / 2).toFixed(2);
  const verticalMarginMm = +((297 - gridHeightMm) / 2).toFixed(2);

  let pageDivs = '';
  let idIndex = 0;

  for (let p = 0; p < pages; p++) {
    let rowsHtml = '';
    for (let r = 0; r < rows; r++) {
      let cellsHtml = '';
      for (let c = 0; c < cols; c++) {
        const id = allIds[idIndex++];
        const svg = bwipjs.toSVG({
          bcid: 'datamatrix',
          text: id,
          width: 15,
          height: 15
        });

        cellsHtml += `
          <td class="cell">
            <div class="cell-content">
              <div class="barcode">${svg}</div>
              <div class="code-id">${id}</div>
            </div>
          </td>`;
      }
      rowsHtml += `<tr>${cellsHtml}</tr>`;
    }

    pageDivs += `
      <div class="page">
        <table class="grid">
          ${rowsHtml}
        </table>
      </div>`;
  }

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>PocketBase Data Matrix Barcodes</title>
  <style>
    @page {
      size: A4 portrait;
      margin: 0;
    }
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }
    html, body {
      width: 210mm;
      background: #ffffff;
      -webkit-print-color-adjust: exact;
      print-color-adjust: exact;
    }
    .page {
      width: 210mm;
      height: 297mm;
      padding: ${verticalMarginMm}mm ${horizontalMarginMm}mm;
      page-break-after: always;
      break-after: page;
      overflow: hidden;
      display: flex;
      justify-content: center;
      align-items: center;
      background: #ffffff;
    }
    .page:last-child {
      page-break-after: avoid;
      break-after: avoid;
    }
    table.grid {
      border-collapse: collapse;
      width: ${gridWidthMm}mm;
      height: ${gridHeightMm}mm;
      table-layout: fixed;
      margin: 0 auto;
    }
    table.grid td.cell {
      width: ${cellWidthMm}mm;
      height: ${cellHeightMm}mm;
      padding: 0;
      border: 0.5px dashed #cccccc;
      text-align: center;
      vertical-align: middle;
      background: #ffffff;
    }
    .cell-content {
      width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
    }
    .barcode {
      width: 15mm;
      height: 15mm;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .barcode svg {
      width: 15mm;
      height: 15mm;
      display: block;
    }
    .code-id {
      font-family: 'Liberation Mono', 'DejaVu Sans Mono', 'Consolas', 'Courier New', monospace;
      font-size: 5.5pt;
      font-weight: 500;
      line-height: 1.15;
      letter-spacing: 0.35px;
      color: #111111;
      margin-top: 1.5mm;
      text-align: center;
      white-space: nowrap;
    }
  </style>
</head>
<body>
  ${pageDivs}
</body>
</html>`;
}

// 6. Main execution flow
function main() {
  const options = parseArgs();

  if (options.help) {
    printUsage();
    process.exit(0);
  }

  const chromeBin = findChromeBinary();
  if (!chromeBin) {
    console.error('Error: Google Chrome or Chromium executable not found in PATH.');
    console.error('Please ensure google-chrome or chromium is installed.');
    process.exit(1);
  }

  const startTime = Date.now();
  const codesPerPage = options.cols * options.rows;
  const totalCodes = codesPerPage * options.pages;
  const outputPath = path.resolve(process.cwd(), options.output);

  console.log('----------------------------------------------------');
  console.log(' PocketBase Data Matrix A4 Sheet Generator');
  console.log('----------------------------------------------------');
  console.log(`Grid dimensions    : ${options.cols} columns x ${options.rows} rows`);
  console.log(`Codes per page     : ${codesPerPage}`);
  console.log(`Total pages        : ${options.pages}`);
  console.log(`Total unique codes : ${totalCodes}`);
  console.log(`Barcode size       : 15mm x 15mm (Vector SVG)`);
  console.log(`Cutting guides     : 0.5px dashed border`);
  console.log(`Chrome binary      : ${chromeBin}`);
  console.log(`Output destination : ${outputPath}`);
  console.log('----------------------------------------------------');

  // Generate HTML
  console.log('Generating vector Data Matrix codes and HTML layout...');
  const htmlContent = buildHtml(options);

  const tmpHtmlPath = path.join(
    os.tmpdir(),
    `datamatrix_${Date.now()}_${process.pid}.html`
  );

  fs.writeFileSync(tmpHtmlPath, htmlContent, 'utf8');

  try {
    console.log('Rendering PDF via Headless Chrome...');
    const chromeCmd = `"${chromeBin}" --headless=new --disable-gpu --no-pdf-header-footer --print-to-pdf="${outputPath}" "${tmpHtmlPath}"`;
    execSync(chromeCmd, { stdio: ['pipe', 'pipe', 'pipe'] });

    if (!fs.existsSync(outputPath)) {
      throw new Error(`Output file ${outputPath} was not created.`);
    }

    const stats = fs.statSync(outputPath);
    if (stats.size === 0) {
      throw new Error(`Output file ${outputPath} is empty (0 bytes).`);
    }

    const elapsed = ((Date.now() - startTime) / 1000).toFixed(2);
    console.log(`\n✔ PDF generated successfully in ${elapsed}s!`);
    console.log(`  File size : ${(stats.size / 1024).toFixed(1)} KB (${stats.size} bytes)`);
    console.log(`  Saved to  : ${outputPath}`);

    if (options.keepHtml) {
      const savedHtmlPath = outputPath.replace(/\\.pdf$/i, '.html');
      fs.copyFileSync(tmpHtmlPath, savedHtmlPath);
      console.log(`  Saved HTML: ${savedHtmlPath}`);
    }
  } catch (err) {
    console.error('Error rendering PDF:', err.message);
    process.exit(1);
  } finally {
    if (fs.existsSync(tmpHtmlPath)) {
      fs.unlinkSync(tmpHtmlPath);
    }
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  generatePocketBaseId,
  generateUniquePocketBaseIds,
  buildHtml,
  loadBwipJs
};

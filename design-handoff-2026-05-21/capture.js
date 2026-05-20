// capture.js — headless Playwright screenshot script for archigraph design handoff
// Usage: node capture.js
// Requires: npx playwright (chromium already installed)

const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE_URL = 'http://127.0.0.1:47274';
const SCREENSHOTS_DIR = path.join(__dirname, 'screenshots');
const GROUP = 'upvate'; // the live group with real data
const FIXTURE_GROUP = 'fixture-946'; // second group, may be empty

// Ensure screenshots dir exists
if (!fs.existsSync(SCREENSHOTS_DIR)) fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });

// Results log
const results = [];

async function shot(page, name, description) {
  const filePath = path.join(SCREENSHOTS_DIR, `${name}.png`);
  await page.screenshot({ path: filePath, fullPage: false });
  results.push({ name, description, file: filePath, ok: true });
  console.log(`  ✓ ${name}`);
  return filePath;
}

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('theme', t);
    document.documentElement.classList.remove('dark', 'light');
    if (t === 'dark') document.documentElement.classList.add('dark');
  }, theme);
  // Wait a tick for repaint
  await page.waitForTimeout(300);
}

async function waitForContent(page, timeout = 8000) {
  // Wait until no loading spinners / skeleton indicators
  try {
    await page.waitForFunction(() => {
      const spinners = document.querySelectorAll('[class*="animate-spin"], [class*="animate-pulse"]');
      return spinners.length === 0;
    }, { timeout });
  } catch (_) {
    // timeout is ok — some surfaces always have animated elements
  }
  await page.waitForTimeout(400);
}

async function captureTheme(browser, theme, prefix) {
  console.log(`\n── Theme: ${theme} ──`);
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();

  // Suppress console noise
  page.on('console', () => {});
  page.on('pageerror', () => {});

  // ── Landing ──────────────────────────────────────────────────────────────
  console.log('Landing...');
  await page.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}01-landing-base`, 'Landing page default card grid');

  // Hover a card
  const cards = await page.locator('[data-testid="group-card"], .group\\/card, a[href*="/graph/"]').all();
  if (cards.length > 0) {
    await cards[0].hover();
    await page.waitForTimeout(400);
    await shot(page, `${prefix}02-landing-card-hover`, 'Landing card hover state');
  } else {
    // Try hovering first link that looks like a card
    const firstCard = page.locator('a').first();
    await firstCard.hover();
    await page.waitForTimeout(400);
    await shot(page, `${prefix}02-landing-card-hover`, 'Landing card hover (fallback)');
  }

  // Tooltip on unresolved edges icon
  const tooltipIcon = page.locator('[data-testid="tooltip"], [aria-label*="Unresolved"], button[title*="edge"], [class*="tooltip"]').first();
  try {
    await tooltipIcon.hover({ timeout: 3000 });
    await page.waitForTimeout(500);
    await shot(page, `${prefix}03-landing-tooltip`, 'Landing tooltip on unresolved edges icon');
  } catch (_) {
    // Fallback: hover any info/help icon
    const infoBtn = page.locator('button, [role="button"]').filter({ hasText: /info|help|edges/i }).first();
    try {
      await infoBtn.hover({ timeout: 2000 });
      await page.waitForTimeout(400);
    } catch (_2) { /* skip hover */ }
    await shot(page, `${prefix}03-landing-tooltip`, 'Landing — tooltip area (best effort)');
  }

  // ── Graph ──────────────────────────────────────────────────────────────
  console.log('Graph...');
  await page.goto(BASE_URL + `/graph/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page, 10000);
  await shot(page, `${prefix}10-graph-3d-default`, 'Graph 3D default view — dense LOD with repo islands');

  // 2D layout button
  try {
    const layout2d = page.locator('button, [role="button"]').filter({ hasText: /2d|flat/i }).first();
    await layout2d.click({ timeout: 3000 });
    await page.waitForTimeout(1200);
    await shot(page, `${prefix}11-graph-2d`, 'Graph 2D layout active');
    // Reset back to 3D if button exists
    const layout3d = page.locator('button, [role="button"]').filter({ hasText: /3d/i }).first();
    try { await layout3d.click({ timeout: 2000 }); await page.waitForTimeout(800); } catch (_) {}
  } catch (_) {
    await shot(page, `${prefix}11-graph-2d`, 'Graph — 2D button not found, base view captured');
  }

  // Tree layout
  try {
    const treeBtn = page.locator('button, [role="button"]').filter({ hasText: /tree/i }).first();
    await treeBtn.click({ timeout: 3000 });
    await page.waitForTimeout(1200);
    await shot(page, `${prefix}12-graph-tree`, 'Graph Tree layout active');
    // Reset
    const layout3d = page.locator('button, [role="button"]').filter({ hasText: /3d/i }).first();
    try { await layout3d.click({ timeout: 2000 }); await page.waitForTimeout(800); } catch (_) {}
  } catch (_) {
    await shot(page, `${prefix}12-graph-tree`, 'Graph — Tree button not found, base view captured');
  }

  // Click a node to open inspector
  try {
    // Canvas element — click center-ish area
    const canvas = page.locator('canvas').first();
    const box = await canvas.boundingBox();
    if (box) {
      await page.mouse.click(box.x + box.width * 0.5, box.y + box.height * 0.4);
      await page.waitForTimeout(1000);
    }
    await shot(page, `${prefix}13-graph-node-selected`, 'Graph node selected — inspector panel visible');
  } catch (_) {
    await shot(page, `${prefix}13-graph-node-selected`, 'Graph — node click attempted, base captured');
  }

  // Community drill-in via sidebar
  try {
    const communityItem = page.locator('[data-testid*="community"], [class*="community"], aside li, [role="listitem"]').first();
    await communityItem.click({ timeout: 3000 });
    await page.waitForTimeout(1000);
    await shot(page, `${prefix}14-graph-community-drilled`, 'Graph community drilled with breadcrumb');
  } catch (_) {
    await shot(page, `${prefix}14-graph-community-drilled`, 'Graph — community drill fallback');
  }

  // Repo filter toggle
  try {
    const repoFilter = page.locator('input[type="checkbox"], button[data-repo]').first();
    await repoFilter.click({ timeout: 3000 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}15-graph-repo-filter`, 'Graph — one repo toggled off via filter');
    await repoFilter.click({ timeout: 2000 }); // reset
  } catch (_) {
    await shot(page, `${prefix}15-graph-repo-filter`, 'Graph — repo filter fallback');
  }

  // Zoom out
  try {
    await page.keyboard.press('-');
    await page.keyboard.press('-');
    await page.keyboard.press('-');
    await page.waitForTimeout(600);
    await shot(page, `${prefix}16-graph-zoom-out`, 'Graph zoomed out to centroid LoD');
  } catch (_) {
    await shot(page, `${prefix}16-graph-zoom-out`, 'Graph zoom out fallback');
  }

  // Search
  try {
    const searchInput = page.locator('input[type="search"], input[placeholder*="search" i], input[placeholder*="Search" i]').first();
    await searchInput.click({ timeout: 3000 });
    await searchInput.type('User', { delay: 80 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}17-graph-search-active`, 'Graph search input populated with results');
    await searchInput.clear();
  } catch (_) {
    await shot(page, `${prefix}17-graph-search-active`, 'Graph — search fallback');
  }

  // ── Flows ──────────────────────────────────────────────────────────────
  console.log('Flows...');
  await page.goto(BASE_URL + `/flows/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}20-flows-list`, 'Flows list view — all processes');

  // Click first flow
  try {
    const firstFlow = page.locator('[data-testid*="flow"], tr, [role="row"], li').filter({ hasText: /→|->|BuildingDetails|proc:/ }).first();
    await firstFlow.click({ timeout: 3000 });
    await page.waitForTimeout(1000);
    await shot(page, `${prefix}21-flows-detail`, 'Flows — flow selected with swim-lane/step detail');
  } catch (_) {
    // Try clicking first list item
    try {
      const firstItem = page.locator('main li, main tr, main [role="row"]').first();
      await firstItem.click({ timeout: 3000 });
      await page.waitForTimeout(800);
    } catch (_2) {}
    await shot(page, `${prefix}21-flows-detail`, 'Flows — detail fallback');
  }

  // ── Topology ──────────────────────────────────────────────────────────
  console.log('Topology...');
  await page.goto(BASE_URL + `/topology/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}30-topology-map`, 'Topology map/force-graph view');

  // List view toggle
  try {
    const listViewBtn = page.locator('button, [role="button"]').filter({ hasText: /list/i }).first();
    await listViewBtn.click({ timeout: 3000 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}31-topology-list`, 'Topology list view active');
    // Toggle back
    const mapBtn = page.locator('button, [role="button"]').filter({ hasText: /map|graph/i }).first();
    try { await mapBtn.click({ timeout: 2000 }); } catch (_) {}
  } catch (_) {
    await shot(page, `${prefix}31-topology-list`, 'Topology — list toggle fallback');
  }

  // Protocol filter
  try {
    const protocolChip = page.locator('[data-testid*="protocol"], button[data-protocol], [class*="chip"], [class*="badge"]').first();
    await protocolChip.click({ timeout: 3000 });
    await page.waitForTimeout(600);
    await shot(page, `${prefix}32-topology-protocol-filtered`, 'Topology — protocol chip toggled off');
    await protocolChip.click({ timeout: 2000 });
  } catch (_) {
    await shot(page, `${prefix}32-topology-protocol-filtered`, 'Topology — protocol filter fallback');
  }

  // Topic selected
  try {
    const topicItem = page.locator('[data-testid*="topic"], [class*="topic"], main li, main tr').first();
    await topicItem.click({ timeout: 3000 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}33-topology-topic-selected`, 'Topology topic selected — detail panel visible');
  } catch (_) {
    await shot(page, `${prefix}33-topology-topic-selected`, 'Topology — topic selection fallback');
  }

  // ── Paths ──────────────────────────────────────────────────────────────
  console.log('Paths...');
  await page.goto(BASE_URL + `/paths/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}40-paths-collapsed`, 'Paths — all groups collapsed default view');

  // Expand one group
  try {
    const groupHeader = page.locator('[data-testid*="group"], button[aria-expanded], [class*="accordion"], details summary, [role="button"]').first();
    await groupHeader.click({ timeout: 3000 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}41-paths-group-expanded`, 'Paths — one group expanded showing endpoints');
  } catch (_) {
    // Try clicking first visible clickable row
    try {
      const firstRow = page.locator('main button, main [role="button"]').first();
      await firstRow.click({ timeout: 3000 });
      await page.waitForTimeout(800);
    } catch (_2) {}
    await shot(page, `${prefix}41-paths-group-expanded`, 'Paths — expand fallback');
  }

  // Filter input
  try {
    const filterInput = page.locator('input[type="text"], input[type="search"], input[placeholder*="filter" i], input[placeholder*="search" i]').first();
    await filterInput.click({ timeout: 3000 });
    await filterInput.type('/api/', { delay: 80 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}42-paths-filter-active`, 'Paths — filter input populated with /api/');
    await filterInput.clear();
    await page.waitForTimeout(400);
  } catch (_) {
    await shot(page, `${prefix}42-paths-filter-active`, 'Paths — filter fallback');
  }

  // Click an endpoint for detail panel
  try {
    const endpointItem = page.locator('main a, main li, main [role="row"], main tr').first();
    await endpointItem.click({ timeout: 3000 });
    await page.waitForTimeout(1000);
    await shot(page, `${prefix}43-paths-detail`, 'Paths — endpoint detail panel visible on right');
  } catch (_) {
    await shot(page, `${prefix}43-paths-detail`, 'Paths — detail panel fallback');
  }

  // Flat list toggle
  try {
    const flatBtn = page.locator('button, [role="button"]').filter({ hasText: /flat|all|list/i }).first();
    await flatBtn.click({ timeout: 3000 });
    await page.waitForTimeout(800);
    await shot(page, `${prefix}44-paths-flat-list`, 'Paths — flat list view active');
  } catch (_) {
    await shot(page, `${prefix}44-paths-flat-list`, 'Paths — flat list fallback');
  }

  // ── Docs ──────────────────────────────────────────────────────────────
  console.log('Docs...');
  await page.goto(BASE_URL + `/docs/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}50-docs-empty`, `Docs — empty state (${GROUP} has no generated docs)`);

  // ── Pending ──────────────────────────────────────────────────────────
  console.log('Pending...');
  await page.goto(BASE_URL + `/pending/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}60-pending-repairs`, 'Pending — repair candidates tab');

  // Click enrichments tab
  try {
    const enrichTab = page.locator('[role="tab"], button').filter({ hasText: /enrich/i }).first();
    await enrichTab.click({ timeout: 3000 });
    await page.waitForTimeout(600);
    await shot(page, `${prefix}61-pending-enrichments`, 'Pending — enrichment candidates tab');
  } catch (_) {
    await shot(page, `${prefix}61-pending-enrichments`, 'Pending — enrichment tab fallback');
  }

  // ── Top nav / global ──────────────────────────────────────────────────
  console.log('Top nav / global...');
  await page.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(page, theme);
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);

  // Version popover
  try {
    const versionBtn = page.locator('button, [role="button"]').filter({ hasText: /version|v\d|0\.0\.|info/i }).first();
    await versionBtn.click({ timeout: 3000 });
    await page.waitForTimeout(500);
    await shot(page, `${prefix}70-version-popover`, 'Version popover open');
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);
  } catch (_) {
    // Try clicking any small version text
    try {
      const versionText = page.locator('text=/v0\\.|version/i').first();
      await versionText.click({ timeout: 2000 });
      await page.waitForTimeout(400);
    } catch (_2) {}
    await shot(page, `${prefix}70-version-popover`, 'Version info area — popover fallback');
    try { await page.keyboard.press('Escape'); } catch (_3) {}
  }

  // Group selector
  try {
    // Look for the group selector dropdown
    const groupSelector = page.locator('button, [role="combobox"], [role="button"]').filter({ hasText: new RegExp(GROUP + '|fixture|group', 'i') }).first();
    await groupSelector.click({ timeout: 3000 });
    await page.waitForTimeout(500);
    await shot(page, `${prefix}71-group-selector-open`, 'Group selector dropdown open');
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);
  } catch (_) {
    await shot(page, `${prefix}71-group-selector-open`, 'Group selector — fallback');
    try { await page.keyboard.press('Escape'); } catch (_2) {}
  }

  // Theme toggle light
  await setTheme(page, 'light');
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}72-theme-toggle-light`, 'Theme: light mode landing');

  // Theme toggle dark
  await setTheme(page, 'dark');
  await page.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(page);
  await shot(page, `${prefix}73-theme-toggle-dark`, 'Theme: dark mode landing');

  // Mobile viewport
  await context.close();
  const mobileContext = await browser.newContext({ viewport: { width: 375, height: 812 } });
  const mobilePage = await mobileContext.newPage();
  mobilePage.on('console', () => {});

  await mobilePage.goto(BASE_URL + '/', { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(mobilePage, theme);
  await mobilePage.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(mobilePage);
  await shot(mobilePage, `${prefix}74-mobile-landing`, 'Mobile 375×812 — landing page');

  await mobilePage.goto(BASE_URL + `/graph/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(mobilePage, theme);
  await mobilePage.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(mobilePage, 10000);
  await shot(mobilePage, `${prefix}75-mobile-graph`, 'Mobile 375×812 — graph surface');

  await mobilePage.goto(BASE_URL + `/pending/${GROUP}`, { waitUntil: 'networkidle', timeout: 15000 });
  await setTheme(mobilePage, theme);
  await mobilePage.reload({ waitUntil: 'networkidle', timeout: 15000 });
  await waitForContent(mobilePage);
  await shot(mobilePage, `${prefix}76-mobile-pending`, 'Mobile 375×812 — pending surface');

  await mobileContext.close();
}

async function main() {
  console.log('Launching Playwright (headless)...');
  const browser = await chromium.launch({ headless: true });

  try {
    await captureTheme(browser, 'dark', 'dark-');
    await captureTheme(browser, 'light', 'light-');
  } finally {
    await browser.close();
  }

  // Write results JSON
  const resultsPath = path.join(__dirname, 'screenshots', '_results.json');
  fs.writeFileSync(resultsPath, JSON.stringify(results, null, 2));

  console.log(`\n✓ Done. ${results.length} screenshots captured.`);
  console.log(`  Output: ${SCREENSHOTS_DIR}`);
  return results;
}

main().catch(e => {
  console.error('Error:', e.message);
  process.exit(1);
});

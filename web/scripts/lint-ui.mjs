#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, '..');

console.log('--------------------------------------------------');
console.log('  SwiftDrop UI Guardrail: Running lint:ui checks');
console.log('--------------------------------------------------');

let hasErrors = false;

// 1. Radix UI Dependency Freeze Assertion
const packageJsonPath = path.join(rootDir, 'package.json');
const pkg = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));

const APPROVED_RADIX_DEPS = [
  '@radix-ui/react-accordion',
  '@radix-ui/react-avatar',
  '@radix-ui/react-checkbox',
  '@radix-ui/react-dialog',
  '@radix-ui/react-dropdown-menu',
  '@radix-ui/react-label',
  '@radix-ui/react-popover',
  '@radix-ui/react-scroll-area',
  '@radix-ui/react-select',
  '@radix-ui/react-separator',
  '@radix-ui/react-slot',
  '@radix-ui/react-switch',
  '@radix-ui/react-tabs',
  '@radix-ui/react-tooltip',
].sort();

const currentRadixDeps = Object.keys(pkg.dependencies || {})
  .filter((dep) => dep.startsWith('@radix-ui/'))
  .sort();

const depDiffAdded = currentRadixDeps.filter((dep) => !APPROVED_RADIX_DEPS.includes(dep));
const depDiffRemoved = APPROVED_RADIX_DEPS.filter((dep) => !currentRadixDeps.includes(dep));

if (depDiffAdded.length > 0 || depDiffRemoved.length > 0) {
  console.error('\n❌ [DEPENDENCY FREEZE VIOLATION] Unapproved changes to @radix-ui/* dependencies:');
  if (depDiffAdded.length > 0) {
    console.error(`  - Added unauthorized packages: ${depDiffAdded.join(', ')}`);
  }
  if (depDiffRemoved.length > 0) {
    console.error(`  - Missing required packages: ${depDiffRemoved.join(', ')}`);
  }
  console.error('  Running `shadcn add` or adding Radix dependencies is strictly forbidden (AGENTS.md Rule 7).');
  hasErrors = true;
} else {
  console.log(`✅ [1/3] Dependency freeze intact: exactly ${APPROVED_RADIX_DEPS.length} approved @radix-ui/* packages verified.`);
}

// 2. Scan Feature Files for Prohibited Styling Patterns
// Primitives in src/shared/components/ui/ are the substrate and are exempt.
// Legacy pre-shadcn files are isolated in a tracked exemption ledger until migrated.
const LEGACY_PRE_SHADCN_FILES = new Set([
  'src/features/auth/LoginPage.tsx',
  'src/features/auth/RegisterPage.tsx',
  'src/features/customer-merchant/portal/BookingPage.tsx',
  'src/features/customer-merchant/portal/MerchantOrderImport.tsx',
  'src/features/customer-merchant/portal/MerchantPage.tsx',
  'src/features/customer-merchant/portal/PortalLayout.tsx',
  'src/features/customer-merchant/portal/RegisterPage.tsx',
  'src/features/customer-merchant/portal/TrackingPage.tsx',
  'src/features/customer-merchant/preview/DesignPreviewPage.tsx',
  'src/features/customer-merchant/preview/SwiftSignatureTheme.tsx',
  'src/features/dashboards/AdminDashboard.tsx',
  'src/features/dashboards/CourierDashboard.tsx',
  'src/features/dashboards/CustomerDashboard.tsx',
  'src/features/dashboards/DashboardLayout.tsx',
  'src/features/dashboards/DispatcherDashboard.tsx',
  'src/features/dashboards/MerchantDashboard.tsx',
].map((p) => path.normalize(p)));

const RAW_PALETTE_REGEX = /\b(?:[a-z0-9-]+:)*(?:bg|text|border|ring|fill|stroke|from|to|via)-(?:red|rose|pink|fuchsia|purple|violet|indigo|blue|sky|cyan|teal|emerald|green|lime|amber|yellow|orange)(?:-\d+)?\b/i;
const ARBITRARY_BRACKET_REGEX = /\b(?:[a-z0-9-]+:)*(?:bg|text|border|ring|shadow)-\[[^\]]+\]/i;
const RAW_HEX_REGEX = /#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})\b/;
const INLINE_STYLE_COLOR_REGEX = /style=\{\{[^}]*?(?:color|backgroundColor|borderColor|background)\s*:\s*['"][^'"]+['"][^}]*?\}\}/i;
const RAW_ELEMENT_REGEX = /<(?:button|input|select|textarea|table|dialog)\b/;
const RADIX_IMPORT_REGEX = /from\s+['"]@radix-ui\/[^'"]+['"]/;

function getAllFiles(dir, fileList = []) {
  if (!fs.existsSync(dir)) return fileList;
  const files = fs.readdirSync(dir);
  for (const file of files) {
    const filePath = path.join(dir, file);
    const stat = fs.statSync(filePath);
    if (stat.isDirectory()) {
      if (file !== 'node_modules' && file !== 'dist' && file !== 'ui') {
        getAllFiles(filePath, fileList);
      }
    } else if (file.endsWith('.tsx') || file.endsWith('.ts')) {
      fileList.push(filePath);
    }
  }
  return fileList;
}

const srcDir = path.join(rootDir, 'src');
const allSourceFiles = getAllFiles(srcDir);
const filesToScan = allSourceFiles.filter((filePath) => {
  const relPath = path.relative(rootDir, filePath);
  // Exempt primitives in shared/components/ui
  if (relPath.includes(path.join('shared', 'components', 'ui'))) {
    return false;
  }
  // Exempt tracked legacy prototype files until migrated
  if (LEGACY_PRE_SHADCN_FILES.has(path.normalize(relPath))) {
    return false;
  }
  return true;
});

let styleViolations = 0;

for (const filePath of filesToScan) {
  const relPath = path.relative(rootDir, filePath).replace(/\\/g, '/');
  const content = fs.readFileSync(filePath, 'utf8');
  const lines = content.split('\n');

  lines.forEach((line, index) => {
    const lineNum = index + 1;
    const trimmed = line.trim();

    // Check 1: Raw intrinsic HTML element
    if (RAW_ELEMENT_REGEX.test(line)) {
      console.error(`❌ [RAW ELEMENT BAN] ${relPath}:${lineNum}`);
      console.error(`   Found intrinsic element: ${trimmed}`);
      console.error(`   Remedy: Replace with primitives from @/shared/components/ui/ (AGENTS.md Rule 3)`);
      styleViolations++;
      hasErrors = true;
    }

    // Check 2: Direct Radix UI import
    if (RADIX_IMPORT_REGEX.test(line)) {
      console.error(`❌ [DIRECT RADIX IMPORT BAN] ${relPath}:${lineNum}`);
      console.error(`   Direct @radix-ui import: ${trimmed}`);
      console.error(`   Remedy: Import exclusively from @/shared/components/ui/ (AGENTS.md Rule 2)`);
      styleViolations++;
      hasErrors = true;
    }

    // Check 3: Raw Tailwind palette family
    if (RAW_PALETTE_REGEX.test(line)) {
      console.error(`❌ [TAILWIND PALETTE BAN] ${relPath}:${lineNum}`);
      console.error(`   Unapproved color class: ${trimmed}`);
      console.error(`   Remedy: Use semantic tokens (bg-surface, text-text, text-primary, bg-status-success-bg) (AGENTS.md Rule 5)`);
      styleViolations++;
      hasErrors = true;
    }

    // Check 4: Arbitrary value bracket
    if (ARBITRARY_BRACKET_REGEX.test(line)) {
      console.error(`❌ [ARBITRARY VALUE BAN] ${relPath}:${lineNum}`);
      console.error(`   Arbitrary value bracket: ${trimmed}`);
      console.error(`   Remedy: Consume semantic design tokens or theme utilities (AGENTS.md Rule 5)`);
      styleViolations++;
      hasErrors = true;
    }

    // Check 5: Raw hex code
    if (RAW_HEX_REGEX.test(line) && !line.includes('//') && !line.includes('/*')) {
      console.error(`❌ [RAW HEX BAN] ${relPath}:${lineNum}`);
      console.error(`   Hardcoded hex color: ${trimmed}`);
      console.error(`   Remedy: Use semantic CSS custom properties or token classes (AGENTS.md Rule 5)`);
      styleViolations++;
      hasErrors = true;
    }

    // Check 6: Inline style with color properties
    if (INLINE_STYLE_COLOR_REGEX.test(line)) {
      console.error(`❌ [INLINE STYLE COLOR BAN] ${relPath}:${lineNum}`);
      console.error(`   Inline color style: ${trimmed}`);
      console.error(`   Remedy: Use Tailwind semantic token classes (AGENTS.md Rule 5)`);
      styleViolations++;
      hasErrors = true;
    }
  });
}

if (styleViolations === 0) {
  console.log(`✅ [2/3] Pattern analysis passed: ${filesToScan.length} non-exempt files analyzed, 0 violations.`);
} else {
  console.error(`\n❌ Total violations detected in feature code: ${styleViolations}`);
}

if (hasErrors) {
  console.error('\n💥 [UI LINT FAILED] Build aborted due to design governance violations.');
  process.exit(1);
} else {
  console.log('✅ [3/3] UI design governance passed cleanly.');
  console.log('--------------------------------------------------');
  process.exit(0);
}

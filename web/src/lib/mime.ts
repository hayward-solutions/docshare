const EDITABLE_APPLICATION_MIMES = new Set([
  'application/json',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/yaml',
]);

const MARKDOWN_MIMES = new Set(['text/markdown', 'text/x-markdown']);

const CSV_MIMES = new Set(['text/csv', 'application/csv']);

// XLSX-only for now. ExcelJS doesn't parse .xls (BIFF) or .ods, and
// silently re-emitting those formats as XLSX on save would lose data or
// confuse mime/extension expectations. Re-add when format-specific
// bridges land.
const SPREADSHEET_BINARY_MIMES = new Set([
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
]);

export const XLSX_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

/**
 * Strip any parameters (charset, boundary, etc.) and lowercase the bare
 * type/subtype. The backend's resolveMimeType can hand us
 * `text/csv; charset=utf-8` (from Go's mime.TypeByExtension), and our
 * Set/equality checks would otherwise miss it.
 */
function normalize(mimeType: string | undefined | null): string {
  if (!mimeType) return '';
  const semi = mimeType.indexOf(';');
  const base = semi >= 0 ? mimeType.slice(0, semi) : mimeType;
  return base.trim().toLowerCase();
}

export function isEditableMime(mimeType: string | undefined | null): boolean {
  const m = normalize(mimeType);
  if (!m) return false;
  if (m.startsWith('text/')) return true;
  return EDITABLE_APPLICATION_MIMES.has(m);
}

export function isMarkdownMime(mimeType: string | undefined | null): boolean {
  const m = normalize(mimeType);
  return !!m && MARKDOWN_MIMES.has(m);
}

export function isCsvMime(mimeType: string | undefined | null): boolean {
  const m = normalize(mimeType);
  return !!m && CSV_MIMES.has(m);
}

export function isSpreadsheetBinaryMime(mimeType: string | undefined | null): boolean {
  const m = normalize(mimeType);
  return !!m && SPREADSHEET_BINARY_MIMES.has(m);
}

export function isSpreadsheetMime(mimeType: string | undefined | null): boolean {
  return isCsvMime(mimeType) || isSpreadsheetBinaryMime(mimeType);
}

export function isAnyEditableMime(mimeType: string | undefined | null): boolean {
  // CSV gets its own check because application/csv (the rare-but-valid
  // alternate spelling) isn't text/* and isn't in the editable
  // application set — without this OR, the viewer would hide Edit for a
  // CSV that the spreadsheet editor is perfectly happy to open.
  return isEditableMime(mimeType) || isCsvMime(mimeType) || isSpreadsheetBinaryMime(mimeType);
}

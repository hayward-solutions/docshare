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

const SPREADSHEET_BINARY_MIMES = new Set([
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'application/vnd.ms-excel',
  'application/vnd.oasis.opendocument.spreadsheet',
]);

export const XLSX_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

export function isEditableMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  if (mimeType.startsWith('text/')) return true;
  return EDITABLE_APPLICATION_MIMES.has(mimeType);
}

export function isMarkdownMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  return MARKDOWN_MIMES.has(mimeType);
}

export function isCsvMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  return CSV_MIMES.has(mimeType);
}

export function isSpreadsheetBinaryMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  return SPREADSHEET_BINARY_MIMES.has(mimeType);
}

export function isSpreadsheetMime(mimeType: string | undefined | null): boolean {
  return isCsvMime(mimeType) || isSpreadsheetBinaryMime(mimeType);
}

export function isAnyEditableMime(mimeType: string | undefined | null): boolean {
  return isEditableMime(mimeType) || isSpreadsheetBinaryMime(mimeType);
}

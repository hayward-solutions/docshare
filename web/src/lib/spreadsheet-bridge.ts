import Papa from 'papaparse';
import ExcelJS, { type CellValue } from 'exceljs';

export interface UniverCell {
  v: string | number | boolean | null;
}

export type UniverRow = Record<number, UniverCell>;
export type UniverCellData = Record<number, UniverRow>;

export interface UniverSheetSnapshot {
  id: string;
  name: string;
  rowCount: number;
  columnCount: number;
  cellData: UniverCellData;
}

export interface UniverWorkbookSnapshot {
  id: string;
  name: string;
  sheetOrder: string[];
  sheets: Record<string, UniverSheetSnapshot>;
}

const DEFAULT_ROWS = 100;
const DEFAULT_COLS = 26;
const DEFAULT_SHEET_ID = 'sheet-1';
const DEFAULT_SHEET_NAME = 'Sheet1';

function makeEmptySheet(id = DEFAULT_SHEET_ID, name = DEFAULT_SHEET_NAME): UniverSheetSnapshot {
  return { id, name, rowCount: DEFAULT_ROWS, columnCount: DEFAULT_COLS, cellData: {} };
}

export function emptyWorkbook(workbookName = 'Spreadsheet'): UniverWorkbookSnapshot {
  const sheet = makeEmptySheet();
  return {
    id: 'workbook-1',
    name: workbookName,
    sheetOrder: [sheet.id],
    sheets: { [sheet.id]: sheet },
  };
}

function rowsToSheet(rows: (string | number | boolean | null)[][], sheetName = DEFAULT_SHEET_NAME): UniverSheetSnapshot {
  const cellData: UniverCellData = {};
  let maxCol = 0;
  rows.forEach((row, rowIdx) => {
    if (row.length > maxCol) maxCol = row.length;
    row.forEach((value, colIdx) => {
      if (value === null || value === undefined || value === '') return;
      if (!cellData[rowIdx]) cellData[rowIdx] = {};
      cellData[rowIdx][colIdx] = { v: value };
    });
  });
  return {
    id: DEFAULT_SHEET_ID,
    name: sheetName,
    rowCount: Math.max(rows.length, DEFAULT_ROWS),
    columnCount: Math.max(maxCol, DEFAULT_COLS),
    cellData,
  };
}

function sheetToRows(sheet: UniverSheetSnapshot): (string | number | boolean | null)[][] {
  const rowEntries = Object.entries(sheet.cellData).map(([k]) => Number(k));
  const maxRow = rowEntries.length ? Math.max(...rowEntries) : -1;
  let maxCol = -1;
  for (const row of Object.values(sheet.cellData)) {
    for (const colKey of Object.keys(row)) {
      const c = Number(colKey);
      if (c > maxCol) maxCol = c;
    }
  }
  const out: (string | number | boolean | null)[][] = [];
  for (let r = 0; r <= maxRow; r++) {
    const row: (string | number | boolean | null)[] = [];
    const cells = sheet.cellData[r] ?? {};
    for (let c = 0; c <= maxCol; c++) {
      const cell = cells[c];
      row.push(cell?.v ?? null);
    }
    out.push(row);
  }
  return out;
}

export function csvToWorkbook(text: string, workbookName = 'Spreadsheet'): UniverWorkbookSnapshot {
  if (!text.trim()) return emptyWorkbook(workbookName);
  const parsed = Papa.parse<string[]>(text, { skipEmptyLines: false });
  const sheet = rowsToSheet(parsed.data, DEFAULT_SHEET_NAME);
  return {
    id: 'workbook-1',
    name: workbookName,
    sheetOrder: [sheet.id],
    sheets: { [sheet.id]: sheet },
  };
}

export function workbookToCSV(snapshot: UniverWorkbookSnapshot): string {
  const firstSheetId = snapshot.sheetOrder[0];
  const sheet = snapshot.sheets[firstSheetId];
  if (!sheet) return '';
  const rows = sheetToRows(sheet);
  if (rows.length === 0) return '';
  return Papa.unparse(rows, { newline: '\n' });
}

// Counts sheets beyond the first that have at least one cell with a value.
// CSV can only represent one sheet, so the spreadsheet editor uses this to
// refuse a save that would silently discard data the user added on a second
// sheet via Univer's sheet controls.
export function extraNonEmptySheetCount(snapshot: UniverWorkbookSnapshot): number {
  let count = 0;
  for (let i = 1; i < snapshot.sheetOrder.length; i++) {
    const sheet = snapshot.sheets[snapshot.sheetOrder[i]];
    if (!sheet) continue;
    const cellData = sheet.cellData ?? {};
    const hasData = Object.values(cellData).some((row) =>
      Object.values(row ?? {}).some((cell) => cell?.v !== null && cell?.v !== undefined && cell?.v !== ''),
    );
    if (hasData) count += 1;
  }
  return count;
}

function excelValueToScalar(value: CellValue): string | number | boolean | null {
  if (value === null || value === undefined) return null;
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value;
  // RichText
  if (typeof value === 'object' && 'richText' in value && Array.isArray(value.richText)) {
    return value.richText.map((part) => part.text).join('');
  }
  // Formula result
  if (typeof value === 'object' && 'result' in value) {
    return excelValueToScalar(value.result as CellValue);
  }
  // Hyperlink: { text, hyperlink }
  if (typeof value === 'object' && 'text' in value && typeof (value as { text: unknown }).text === 'string') {
    return (value as { text: string }).text;
  }
  // Error value: { error: '#N/A' | '#REF!' | ... } — render the marker
  // text so the grid shows "#N/A" instead of "[object Object]".
  if (typeof value === 'object' && 'error' in value && typeof (value as { error: unknown }).error === 'string') {
    return (value as { error: string }).error;
  }
  // Date
  if (value instanceof Date) return value.toISOString();
  // Fallback to string representation
  try {
    return String(value);
  } catch {
    return null;
  }
}

export interface XlsxImportResult {
  workbook: UniverWorkbookSnapshot;
  // True when the source XLSX contains structure (formulas, custom styles,
  // merged cells, multiple sheets, etc.) that the bridge can't round-trip
  // cleanly. The editor uses this to surface a "save will lose X" banner.
  hasComplexFormatting: boolean;
}

export async function xlsxBufferToWorkbook(
  buffer: ArrayBuffer,
  workbookName = 'Spreadsheet',
): Promise<XlsxImportResult> {
  if (buffer.byteLength === 0) {
    return { workbook: emptyWorkbook(workbookName), hasComplexFormatting: false };
  }
  const wb = new ExcelJS.Workbook();
  await wb.xlsx.load(buffer);
  const sheetOrder: string[] = [];
  const sheets: Record<string, UniverSheetSnapshot> = {};
  let hasComplexFormatting = false;
  if (wb.worksheets.length > 1) hasComplexFormatting = true;

  wb.worksheets.forEach((ws, idx) => {
    const id = `sheet-${idx + 1}`;
    sheetOrder.push(id);
    const cellData: UniverCellData = {};
    let maxRow = 0;
    let maxCol = 0;
    const merges = (ws as unknown as { _merges?: Record<string, unknown> })._merges;
    if (merges && Object.keys(merges).length > 0) hasComplexFormatting = true;
    ws.eachRow({ includeEmpty: false }, (row, rowNumber) => {
      const rowIdx = rowNumber - 1;
      if (rowIdx > maxRow) maxRow = rowIdx;
      row.eachCell({ includeEmpty: false }, (cell, colNumber) => {
        const colIdx = colNumber - 1;
        if (colIdx > maxCol) maxCol = colIdx;
        // Detect lossy structure: formulas reduce to their cached result on
        // save, custom styles drop entirely. One hit is enough to flip the
        // banner; we don't need to enumerate every cell.
        if (!hasComplexFormatting) {
          if (cell.formula) hasComplexFormatting = true;
          else if (cell.style && (cell.style.font || cell.style.fill || cell.style.border)) {
            hasComplexFormatting = true;
          }
        }
        const scalar = excelValueToScalar(cell.value);
        if (scalar === null || scalar === '') return;
        if (!cellData[rowIdx]) cellData[rowIdx] = {};
        cellData[rowIdx][colIdx] = { v: scalar };
      });
    });
    sheets[id] = {
      id,
      name: ws.name || `Sheet${idx + 1}`,
      rowCount: Math.max(maxRow + 1, DEFAULT_ROWS),
      columnCount: Math.max(maxCol + 1, DEFAULT_COLS),
      cellData,
    };
  });

  if (sheetOrder.length === 0) {
    const sheet = makeEmptySheet();
    sheetOrder.push(sheet.id);
    sheets[sheet.id] = sheet;
  }

  return {
    workbook: { id: 'workbook-1', name: workbookName, sheetOrder, sheets },
    hasComplexFormatting,
  };
}

export async function workbookToXLSXBuffer(snapshot: UniverWorkbookSnapshot): Promise<ArrayBuffer> {
  const wb = new ExcelJS.Workbook();
  for (const sheetId of snapshot.sheetOrder) {
    const sheet = snapshot.sheets[sheetId];
    if (!sheet) continue;
    const ws = wb.addWorksheet(sheet.name);
    // Defensive: Univer can hand back a snapshot where cellData or an
    // individual row is undefined when the sheet is empty. Guarding here
    // keeps the export from crashing on a brand-new workbook.
    for (const [rowKey, row] of Object.entries(sheet.cellData ?? {})) {
      const rowIdx = Number(rowKey);
      for (const [colKey, cell] of Object.entries(row ?? {})) {
        const colIdx = Number(colKey);
        if (cell?.v === null || cell?.v === undefined) continue;
        // ExcelJS is 1-indexed
        ws.getCell(rowIdx + 1, colIdx + 1).value = cell.v as CellValue;
      }
    }
  }
  if (wb.worksheets.length === 0) wb.addWorksheet(DEFAULT_SHEET_NAME);
  const buf = await wb.xlsx.writeBuffer();
  // ExcelJS returns a Node Buffer in node, ArrayBuffer-like in browser. Normalize
  // to a plain ArrayBuffer the fetch body can take without SharedArrayBuffer
  // ambiguity.
  const u8 = new Uint8Array(buf as ArrayBufferLike);
  const out = new ArrayBuffer(u8.byteLength);
  new Uint8Array(out).set(u8);
  return out;
}

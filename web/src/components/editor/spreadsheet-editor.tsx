'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Loader2, AlertCircle, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import '@univerjs/preset-sheets-core/lib/index.css';
import { filesAPI } from '@/lib/api';
import { isCsvMime, XLSX_MIME } from '@/lib/mime';
import {
  csvToWorkbook,
  workbookToCSV,
  xlsxBufferToWorkbook,
  workbookToXLSXBuffer,
  emptyWorkbook,
  extraNonEmptySheetCount,
  type UniverWorkbookSnapshot,
} from '@/lib/spreadsheet-bridge';
import { EditorShell, useCmdS, useUnsavedWarning, type SaveState } from './editor-shell';

interface SpreadsheetEditorProps {
  fileId: string;
  name: string;
  mimeType: string;
}

interface UniverHandle {
  dispose: () => void;
  saveSnapshot: () => UniverWorkbookSnapshot;
}

export function SpreadsheetEditor({ fileId, name, mimeType }: SpreadsheetEditorProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const handleRef = useRef<UniverHandle | null>(null);
  const lastSavedRef = useRef<string>('');

  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [canEdit, setCanEdit] = useState(false);
  const [isDirty, setIsDirty] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [lossyImport, setLossyImport] = useState(false);

  const isCsv = isCsvMime(mimeType);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setLoadError(null);
    setCanEdit(false);
    setIsDirty(false);
    setSaveState('idle');
    setSaveError(null);

    (async () => {
      try {
        let snapshot: UniverWorkbookSnapshot;
        let editable = false;
        let seedEmptyXlsx = false;
        let complex = false;

        if (isCsv) {
          const res = await filesAPI.getContent(fileId);
          if (cancelled) return;
          if (!res.success) throw new Error(res.error || 'Failed to load CSV');
          snapshot = csvToWorkbook(res.data.content, res.data.name);
          editable = res.data.canEdit;
        } else {
          // The backend echoes the edit-permission decision via the
          // X-Can-Edit header on the /binary response so view-only shares
          // open Univer read-only.
          const binRes = await filesAPI.getBinary(fileId);
          if (cancelled) return;
          editable = binRes.canEdit;
          if (binRes.size === 0) {
            snapshot = emptyWorkbook(name);
            // CreateDoc stores a zero-byte placeholder; if we don't seed
            // real XLSX bytes after mount, the file is unusable from any
            // other surface (download/preview). Defer the seed-save until
            // after Univer has mounted so a re-export reflects the
            // canonical empty workbook.
            seedEmptyXlsx = editable;
          } else {
            const imported = await xlsxBufferToWorkbook(binRes.bytes, name);
            snapshot = imported.workbook;
            complex = imported.hasComplexFormatting;
          }
        }

        if (cancelled) return;
        setCanEdit(editable);
        setLossyImport(complex);

        const container = containerRef.current;
        if (!container) return;
        handleRef.current = await mountUniver(container, snapshot, () => {
          // Don't trust the bare "a command fired" signal — Univer fires a
          // long tail of init/layout commands after createWorkbook that
          // would otherwise leave the editor permanently "unsaved." Read
          // the current snapshot and compare to lastSavedRef so we only
          // flip dirty when bytes actually changed.
          const h = handleRef.current;
          if (!h) return;
          try {
            const current = JSON.stringify(h.saveSnapshot());
            const dirty = current !== lastSavedRef.current;
            setIsDirty(dirty);
            if (dirty) setSaveState((s) => (s === 'saved' ? 'idle' : s));
          } catch {
            // Snapshot can throw mid-init; fall back to optimistic dirty.
            setIsDirty(true);
          }
        }, editable);

        // Establish the baseline AFTER mount — Univer's createWorkbook
        // normalizes the snapshot (adds default styles, empty rows, hidden
        // metadata), so comparing later snapshots to the pre-mount one
        // would always show a diff. The post-mount call gives us the
        // canonical "fresh load" shape to diff against.
        if (handleRef.current) {
          lastSavedRef.current = JSON.stringify(handleRef.current.saveSnapshot());
        }

        // Seed a freshly-created blank XLSX with real bytes so a user who
        // bounces out before making changes still leaves a valid workbook
        // in storage (preview/download won't get a 0-byte placeholder).
        if (seedEmptyXlsx && handleRef.current && !cancelled) {
          try {
            const buf = await workbookToXLSXBuffer(handleRef.current.saveSnapshot());
            if (!cancelled) {
              await filesAPI.saveBinary(fileId, buf, XLSX_MIME);
            }
          } catch (err) {
            // Best-effort: log but don't fail the editor open. The user
            // can still type and save normally.
            console.warn('Failed to seed empty XLSX', err);
          }
        }
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : 'Failed to load file';
        setLoadError(message);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();

    return () => {
      cancelled = true;
      const handle = handleRef.current;
      handleRef.current = null;
      if (handle) {
        try { handle.dispose(); } catch { /* noop */ }
      }
    };
  }, [fileId, isCsv, name]);

  const handleSave = useCallback(async () => {
    const handle = handleRef.current;
    if (!handle || !canEdit) return;
    setSaveState('saving');
    setSaveError(null);
    // Capture the snapshot at save-start. If the user keeps typing during
    // the PUT we don't want to clear isDirty when the response lands —
    // their newer edits aren't saved yet.
    const snapshot = handle.saveSnapshot();
    const savedKey = JSON.stringify(snapshot);
    try {
      if (isCsv) {
        // CSV is one-sheet by definition. Refuse the save if Univer's
        // multi-sheet UI was used to add data to a second sheet — better
        // a clear error than a silent drop of the user's work.
        const extra = extraNonEmptySheetCount(snapshot);
        if (extra > 0) {
          throw new Error(
            `CSV files only support one sheet. Move data from the extra sheet${extra > 1 ? 's' : ''} into the first sheet, or delete ${extra > 1 ? 'them' : 'it'}, before saving.`,
          );
        }
        const csv = workbookToCSV(snapshot);
        const res = await filesAPI.saveContent(fileId, csv);
        if (!res.success) throw new Error(res.error || 'Save failed');
      } else {
        const buf = await workbookToXLSXBuffer(snapshot);
        const res = await filesAPI.saveBinary(fileId, buf, mimeType);
        if (!res.success) throw new Error(res.error || 'Save failed');
      }
      lastSavedRef.current = savedKey;
      // Only clear dirty if the editor's current snapshot still matches
      // what we just saved. Otherwise the user typed during the PUT and
      // the new content is still un-persisted.
      const liveHandle = handleRef.current;
      if (liveHandle) {
        const liveKey = JSON.stringify(liveHandle.saveSnapshot());
        setIsDirty(liveKey !== savedKey);
      } else {
        setIsDirty(false);
      }
      setSaveState('saved');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Save failed';
      setSaveError(message);
      setSaveState('error');
      toast.error(message);
    }
  }, [fileId, isCsv, mimeType, canEdit]);

  useUnsavedWarning(isDirty);
  useCmdS(handleSave, canEdit);

  if (loadError) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border bg-muted p-12 text-center">
        <AlertCircle className="h-10 w-10 text-muted-foreground" />
        <div>
          <p className="font-medium">Can&rsquo;t open this spreadsheet</p>
          <p className="mt-1 text-sm text-muted-foreground">{loadError}</p>
        </div>
      </div>
    );
  }

  return (
    <EditorShell
      fileId={fileId}
      name={name}
      canEdit={canEdit}
      isDirty={isDirty}
      saveState={saveState}
      saveError={saveError}
      onSave={handleSave}
      mimeBadge={isCsv ? 'CSV' : 'Spreadsheet'}
    >
      {lossyImport && canEdit && (
        <div className="flex items-start gap-3 rounded-md border border-amber-300/60 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-700/50 dark:bg-amber-950/40 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            This workbook contains formulas, custom styles, merged cells, or multiple sheets.
            Saving here will keep cell values but <span className="font-medium">drop those structures</span>.
            Download a copy first if you need the original.
          </div>
        </div>
      )}
      <div className="relative h-[75vh] rounded-lg border bg-card shadow-xs overflow-hidden">
        {isLoading && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-card">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        )}
        <div ref={containerRef} className="absolute inset-0" />
      </div>
    </EditorShell>
  );
}

async function mountUniver(
  container: HTMLDivElement,
  snapshot: UniverWorkbookSnapshot,
  onChange: () => void,
  editable: boolean,
): Promise<UniverHandle> {
  // Dynamic import to keep Univer out of the markdown editor's bundle and to
  // ensure it never executes on the server.
  const [{ createUniver, LocaleType, mergeLocales }, { UniverSheetsCorePreset }, localeMod] = await Promise.all([
    import('@univerjs/presets'),
    import('@univerjs/preset-sheets-core'),
    import('@univerjs/preset-sheets-core/locales/en-US'),
  ]);

  const { univerAPI, univer } = createUniver({
    locale: LocaleType.EN_US,
    locales: {
      [LocaleType.EN_US]: mergeLocales(
        (localeMod as { default: Record<string, unknown> }).default,
      ),
    },
    presets: [
      UniverSheetsCorePreset({ container }),
    ],
  });

  type WorkbookWithEdit = {
    save: () => UniverWorkbookSnapshot;
    setEditable?: (editable: boolean) => void;
  };
  // Univer's createWorkbook accepts our snapshot shape directly — id, name,
  // sheetOrder, sheets are all canonical IWorkbookData fields.
  const fWorkbook = univerAPI.createWorkbook(snapshot as unknown as Parameters<typeof univerAPI.createWorkbook>[0]) as unknown as WorkbookWithEdit;

  if (!editable && fWorkbook?.setEditable) {
    fWorkbook.setEditable(false);
  }

  // Treat any executed command as a change. Univer uses the command pattern
  // for every edit, so this gives us a single hook for dirty tracking
  // without subscribing to per-cell events. The first few commands fire
  // during initial workbook setup — debounce-style filter would help, but
  // we just clear isDirty after save via lastSavedRef comparison.
  const apiWithEvents = univerAPI as unknown as {
    addEvent?: (eventType: unknown, cb: () => void) => { dispose?: () => void } | undefined;
    Event?: Record<string, unknown>;
  };
  let unsubscribe: (() => void) | undefined;
  const commandEventType = apiWithEvents.Event?.CommandExecuted;
  if (apiWithEvents.addEvent && commandEventType !== undefined) {
    const sub = apiWithEvents.addEvent(commandEventType, () => {
      onChange();
    });
    unsubscribe = () => {
      if (sub?.dispose) sub.dispose();
    };
  }

  return {
    dispose: () => {
      try { unsubscribe?.(); } catch { /* noop */ }
      try { univer?.dispose(); } catch { /* noop */ }
      try { (univerAPI as unknown as { dispose?: () => void }).dispose?.(); } catch { /* noop */ }
    },
    saveSnapshot: () => fWorkbook.save() as UniverWorkbookSnapshot,
  };
}

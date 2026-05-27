'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Loader2, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import '@univerjs/preset-sheets-core/lib/index.css';
import { filesAPI } from '@/lib/api';
import { isCsvMime } from '@/lib/mime';
import {
  csvToWorkbook,
  workbookToCSV,
  xlsxBufferToWorkbook,
  workbookToXLSXBuffer,
  emptyWorkbook,
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

        if (isCsv) {
          const res = await filesAPI.getContent(fileId);
          if (cancelled) return;
          if (!res.success) throw new Error(res.error || 'Failed to load CSV');
          snapshot = csvToWorkbook(res.data.content, res.data.name);
          editable = res.data.canEdit;
        } else {
          // Need editability from the metadata endpoint since /binary doesn't return it
          const [binRes, metaRes] = await Promise.all([
            filesAPI.getBinary(fileId),
            filesAPI.getMeta(fileId),
          ]);
          if (cancelled) return;
          if (!metaRes.success) throw new Error(metaRes.error || 'Failed to load metadata');
          snapshot = binRes.size === 0
            ? emptyWorkbook(metaRes.data.name)
            : await xlsxBufferToWorkbook(binRes.bytes, metaRes.data.name);
          // canEdit: owner OR has edit share. For the editor route, treat
          // owner as editable; the backend will still gate the save.
          editable = true;
        }

        if (cancelled) return;
        setCanEdit(editable);

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
  }, [fileId, isCsv]);

  const handleSave = useCallback(async () => {
    const handle = handleRef.current;
    if (!handle || !canEdit) return;
    setSaveState('saving');
    setSaveError(null);
    try {
      const snapshot = handle.saveSnapshot();
      if (isCsv) {
        const csv = workbookToCSV(snapshot);
        const res = await filesAPI.saveContent(fileId, csv);
        if (!res.success) throw new Error(res.error || 'Save failed');
      } else {
        const buf = await workbookToXLSXBuffer(snapshot);
        const res = await filesAPI.saveBinary(fileId, buf, mimeType);
        if (!res.success) throw new Error(res.error || 'Save failed');
      }
      lastSavedRef.current = JSON.stringify(snapshot);
      setIsDirty(false);
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

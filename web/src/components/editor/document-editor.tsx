'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { EditorContent, useEditor, type Editor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import LinkExt from '@tiptap/extension-link';
import Placeholder from '@tiptap/extension-placeholder';
import { Table } from '@tiptap/extension-table';
import { TableRow } from '@tiptap/extension-table-row';
import { TableCell } from '@tiptap/extension-table-cell';
import { TableHeader } from '@tiptap/extension-table-header';
import { TaskList } from '@tiptap/extension-task-list';
import { TaskItem } from '@tiptap/extension-task-item';
import { Markdown } from 'tiptap-markdown';
import {
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Save,
  Eye,
} from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Loading } from '@/components/loading';
import { filesAPI } from '@/lib/api';
import { cn } from '@/lib/utils';
import { EditorToolbar } from './toolbar';

const MARKDOWN_MIMES = new Set(['text/markdown', 'text/x-markdown']);

interface DocumentEditorProps {
  fileId: string;
}

type EditorMode = 'markdown' | 'plain';

interface LoadedFile {
  name: string;
  mimeType: string;
  canEdit: boolean;
  content: string;
}

export function DocumentEditor({ fileId }: DocumentEditorProps) {
  const router = useRouter();
  const [loaded, setLoaded] = useState<LoadedFile | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setLoadError(null);
    setLoaded(null);

    (async () => {
      try {
        const res = await filesAPI.getContent(fileId);
        if (cancelled) return;
        if (!res.success) {
          setLoadError(res.error || 'Failed to load file');
          return;
        }
        setLoaded({
          name: res.data.name,
          mimeType: res.data.mimeType,
          canEdit: res.data.canEdit,
          content: res.data.content,
        });
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
    };
  }, [fileId]);

  if (isLoading) return <Loading />;

  if (loadError) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border bg-muted p-12 text-center">
        <AlertCircle className="h-10 w-10 text-muted-foreground" />
        <div>
          <p className="font-medium">Can&rsquo;t open this file in the editor</p>
          <p className="mt-1 text-sm text-muted-foreground">{loadError}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => router.back()}>
            Go back
          </Button>
          <Button asChild>
            <Link href={`/files/${fileId}`}>Open viewer</Link>
          </Button>
        </div>
      </div>
    );
  }

  if (!loaded) return null;

  const mode: EditorMode = MARKDOWN_MIMES.has(loaded.mimeType) ? 'markdown' : 'plain';

  return mode === 'markdown' ? (
    <MarkdownEditor fileId={fileId} initial={loaded} />
  ) : (
    <PlainTextEditor fileId={fileId} initial={loaded} />
  );
}

interface EditorVariantProps {
  fileId: string;
  initial: LoadedFile;
}

type SaveState = 'idle' | 'saving' | 'saved' | 'error';

function MarkdownEditor({ fileId, initial }: EditorVariantProps) {
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const lastSavedRef = useRef<string>(initial.content);

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        link: false,
      }),
      LinkExt.configure({
        openOnClick: false,
        autolink: true,
        defaultProtocol: 'https',
      }),
      Placeholder.configure({
        placeholder: 'Start writing… (markdown shortcuts work — try **bold**, # heading, - list)',
      }),
      Table.configure({ resizable: true }),
      TableRow,
      TableHeader,
      TableCell,
      TaskList,
      TaskItem.configure({ nested: true }),
      Markdown.configure({
        html: false,
        tightLists: true,
        bulletListMarker: '-',
        breaks: false,
        transformPastedText: true,
        transformCopiedText: true,
      }),
    ],
    [],
  );

  const editor = useEditor({
    extensions,
    content: initial.content,
    editable: initial.canEdit,
    immediatelyRender: false,
    onUpdate: ({ editor }) => {
      const next = readMarkdown(editor);
      const dirty = next !== lastSavedRef.current;
      setIsDirty(dirty);
      if (dirty && saveState === 'saved') setSaveState('idle');
    },
  });

  const handleSave = useCallback(async () => {
    if (!editor || !initial.canEdit) return;
    const content = readMarkdown(editor);
    setSaveState('saving');
    setSaveError(null);
    try {
      const res = await filesAPI.saveContent(fileId, content);
      if (!res.success) throw new Error(res.error || 'Save failed');
      lastSavedRef.current = content;
      setIsDirty(false);
      setSaveState('saved');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Save failed';
      setSaveError(message);
      setSaveState('error');
      toast.error(message);
    }
  }, [editor, fileId, initial.canEdit]);

  useUnsavedWarning(isDirty);
  useCmdS(handleSave, !!editor && initial.canEdit);

  return (
    <EditorShell
      fileId={fileId}
      name={initial.name}
      canEdit={initial.canEdit}
      isDirty={isDirty}
      saveState={saveState}
      saveError={saveError}
      onSave={handleSave}
      mimeBadge="Markdown"
    >
      {editor ? (
        <>
          <EditorToolbar editor={editor} disabled={!initial.canEdit} />
          <div
            className="editor-content rounded-lg border bg-card p-6 shadow-xs"
            onClick={() => editor.chain().focus().run()}
          >
            <EditorContent editor={editor} />
          </div>
        </>
      ) : (
        <Loading />
      )}
    </EditorShell>
  );
}

function PlainTextEditor({ fileId, initial }: EditorVariantProps) {
  const [value, setValue] = useState(initial.content);
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveError, setSaveError] = useState<string | null>(null);
  const lastSavedRef = useRef<string>(initial.content);
  const isDirty = value !== lastSavedRef.current;

  const handleSave = useCallback(async () => {
    if (!initial.canEdit) return;
    setSaveState('saving');
    setSaveError(null);
    try {
      const res = await filesAPI.saveContent(fileId, value);
      if (!res.success) throw new Error(res.error || 'Save failed');
      lastSavedRef.current = value;
      setSaveState('saved');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Save failed';
      setSaveError(message);
      setSaveState('error');
      toast.error(message);
    }
  }, [fileId, value, initial.canEdit]);

  useEffect(() => {
    if (saveState === 'saved' && isDirty) setSaveState('idle');
  }, [saveState, isDirty]);

  useUnsavedWarning(isDirty);
  useCmdS(handleSave, initial.canEdit);

  const badge =
    initial.mimeType === 'text/plain'
      ? 'Plain text'
      : initial.mimeType.replace(/^(text|application)\//, '').toUpperCase();

  return (
    <EditorShell
      fileId={fileId}
      name={initial.name}
      canEdit={initial.canEdit}
      isDirty={isDirty}
      saveState={saveState}
      saveError={saveError}
      onSave={handleSave}
      mimeBadge={badge}
    >
      <textarea
        value={value}
        readOnly={!initial.canEdit}
        spellCheck={initial.mimeType === 'text/plain'}
        onChange={(e) => setValue(e.target.value)}
        className={cn(
          'h-[70vh] w-full resize-none rounded-lg border bg-card p-6 font-mono text-sm leading-relaxed shadow-xs',
          'focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/40 focus-visible:border-ring',
        )}
        placeholder={initial.canEdit ? 'Start typing…' : ''}
      />
    </EditorShell>
  );
}

interface EditorShellProps {
  fileId: string;
  name: string;
  canEdit: boolean;
  isDirty: boolean;
  saveState: SaveState;
  saveError: string | null;
  onSave: () => void;
  mimeBadge: string;
  children: React.ReactNode;
}

function EditorShell({
  fileId,
  name,
  canEdit,
  isDirty,
  saveState,
  saveError,
  onSave,
  mimeBadge,
  children,
}: EditorShellProps) {
  const router = useRouter();

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <Button variant="ghost" size="icon" onClick={() => router.back()} aria-label="Back">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="truncate text-xl font-semibold" title={name}>
            {name}
          </h1>
          <Badge variant="secondary" className="ml-2 shrink-0">
            {mimeBadge}
          </Badge>
          {!canEdit && (
            <Badge variant="outline" className="ml-1 shrink-0">
              Read-only
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-3">
          <SaveStatus state={saveState} dirty={isDirty} error={saveError} canEdit={canEdit} />
          <Button variant="outline" size="sm" asChild>
            <Link href={`/files/${fileId}`}>
              <Eye className="mr-2 h-4 w-4" />
              View
            </Link>
          </Button>
          {canEdit && (
            <Button size="sm" onClick={onSave} disabled={!isDirty || saveState === 'saving'}>
              {saveState === 'saving' ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Save className="mr-2 h-4 w-4" />
              )}
              Save
            </Button>
          )}
        </div>
      </div>
      <EditorChrome>{children}</EditorChrome>
    </div>
  );
}

function EditorChrome({ children }: { children: React.ReactNode }) {
  return <div className="flex flex-col gap-3">{children}</div>;
}

function SaveStatus({
  state,
  dirty,
  error,
  canEdit,
}: {
  state: SaveState;
  dirty: boolean;
  error: string | null;
  canEdit: boolean;
}) {
  if (!canEdit) return null;
  if (state === 'saving') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Saving…
      </span>
    );
  }
  if (state === 'error') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-destructive" title={error ?? undefined}>
        <AlertCircle className="h-3.5 w-3.5" />
        Save failed
      </span>
    );
  }
  if (dirty) {
    return <span className="text-xs text-muted-foreground">Unsaved changes</span>;
  }
  if (state === 'saved') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
        Saved
      </span>
    );
  }
  return null;
}

function readMarkdown(editor: Editor): string {
  const storage = editor.storage as { markdown?: { getMarkdown: () => string } };
  return storage.markdown?.getMarkdown() ?? editor.getText();
}

function useUnsavedWarning(isDirty: boolean) {
  useEffect(() => {
    if (!isDirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [isDirty]);
}

function useCmdS(onSave: () => void, enabled: boolean) {
  useEffect(() => {
    if (!enabled) return;
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        onSave();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onSave, enabled]);
}


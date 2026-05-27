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
import { CodeBlockLowlight } from '@tiptap/extension-code-block-lowlight';
import { Image as ImageExtension } from '@tiptap/extension-image';
import { Markdown } from 'tiptap-markdown';
import { lowlight } from '@/lib/lowlight';
import { fileToDataURI, isImageFile } from '@/lib/editor-images';
import { AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Loading } from '@/components/loading';
import { filesAPI } from '@/lib/api';
import { cn } from '@/lib/utils';
import { isSpreadsheetMime } from '@/lib/mime';
import { EditorToolbar } from './toolbar';
import { EditorShell, useCmdS, useUnsavedWarning, type SaveState } from './editor-shell';
import { SpreadsheetEditor } from './spreadsheet-editor';
import { SlashCommandExtension } from './slash-command';

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

interface FileMetaResponse {
  id: string;
  name: string;
  mimeType: string;
  ownerID: string;
  size: number;
}

export function DocumentEditor({ fileId }: DocumentEditorProps) {
  const router = useRouter();
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isResolving, setIsResolving] = useState(true);
  const [meta, setMeta] = useState<FileMetaResponse | null>(null);
  const [loadedText, setLoadedText] = useState<LoadedFile | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsResolving(true);
    setLoadError(null);
    setMeta(null);
    setLoadedText(null);

    (async () => {
      try {
        const metaRes = await filesAPI.getMeta(fileId);
        if (cancelled) return;
        if (!metaRes.success) {
          setLoadError(metaRes.error || 'Failed to load file');
          return;
        }
        setMeta(metaRes.data);

        if (isSpreadsheetMime(metaRes.data.mimeType)) {
          // Spreadsheet editor handles its own content fetch (text or binary).
          return;
        }

        const contentRes = await filesAPI.getContent(fileId);
        if (cancelled) return;
        if (!contentRes.success) {
          setLoadError(contentRes.error || 'Failed to load file content');
          return;
        }
        setLoadedText({
          name: contentRes.data.name,
          mimeType: contentRes.data.mimeType,
          canEdit: contentRes.data.canEdit,
          content: contentRes.data.content,
        });
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : 'Failed to load file';
        setLoadError(message);
      } finally {
        if (!cancelled) setIsResolving(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [fileId]);

  if (isResolving) return <Loading />;

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

  if (meta && isSpreadsheetMime(meta.mimeType)) {
    return <SpreadsheetEditor fileId={fileId} name={meta.name} mimeType={meta.mimeType} />;
  }

  if (!loadedText) return null;

  const mode: EditorMode = MARKDOWN_MIMES.has(loadedText.mimeType) ? 'markdown' : 'plain';

  return mode === 'markdown' ? (
    <MarkdownEditor fileId={fileId} initial={loadedText} />
  ) : (
    <PlainTextEditor fileId={fileId} initial={loadedText} />
  );
}

interface EditorVariantProps {
  fileId: string;
  initial: LoadedFile;
}

function MarkdownEditor({ fileId, initial }: EditorVariantProps) {
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const lastSavedRef = useRef<string>(initial.content);

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        link: false,
        // Replaced by CodeBlockLowlight below so we get syntax highlighting
        // instead of a bare <pre><code>.
        codeBlock: false,
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
      CodeBlockLowlight.configure({
        lowlight,
        defaultLanguage: 'plaintext',
      }),
      ImageExtension.configure({
        allowBase64: true,
        inline: false,
        HTMLAttributes: { class: 'editor-image' },
      }),
      SlashCommandExtension,
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
    editorProps: {
      handlePaste: (view, event) => {
        if (!initial.canEdit) return false;
        const files = Array.from(event.clipboardData?.files ?? []).filter(isImageFile);
        if (files.length === 0) return false;
        event.preventDefault();
        void insertImageFiles(view, files);
        return true;
      },
      handleDrop: (view, event, _slice, moved) => {
        if (moved || !initial.canEdit) return false;
        const files = Array.from(event.dataTransfer?.files ?? []).filter(isImageFile);
        if (files.length === 0) return false;
        event.preventDefault();
        void insertImageFiles(view, files, { x: event.clientX, y: event.clientY });
        return true;
      },
    },
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

function readMarkdown(editor: Editor): string {
  const storage = editor.storage as { markdown?: { getMarkdown: () => string } };
  return storage.markdown?.getMarkdown() ?? editor.getText();
}

async function insertImageFiles(
  view: import('@tiptap/pm/view').EditorView,
  files: File[],
  dropCoords?: { x: number; y: number },
) {
  const datas = await Promise.all(files.map(fileToDataURI));
  const valid = datas.filter((d): d is string => !!d);
  if (valid.length === 0) return;
  // The view can be torn down while we were awaiting FileReader for large
  // files. Dispatching on a destroyed view throws — bail out cleanly.
  if (view.isDestroyed) return;

  const schema = view.state.schema;
  const imageType = schema.nodes.image;
  if (!imageType) return;

  // For a drop, anchor the insertion at the drop location; otherwise insert
  // at the current selection.
  let pos = view.state.selection.from;
  if (dropCoords) {
    const coordPos = view.posAtCoords({ left: dropCoords.x, top: dropCoords.y });
    if (coordPos) pos = coordPos.pos;
  }

  const tr = view.state.tr;
  for (const src of valid) {
    const node = imageType.create({ src });
    tr.insert(pos, node);
    pos += node.nodeSize;
  }
  if (view.isDestroyed) return;
  view.dispatch(tr);
}

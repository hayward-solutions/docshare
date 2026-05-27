'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Eye,
  Loader2,
  Save,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

export type SaveState = 'idle' | 'saving' | 'saved' | 'error';

export interface EditorShellProps {
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

export function EditorShell({
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
        <div className="flex min-w-0 items-center gap-2">
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
      <div className="flex flex-col gap-3">{children}</div>
    </div>
  );
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

export function useUnsavedWarning(isDirty: boolean) {
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

export function useCmdS(onSave: () => void, enabled: boolean) {
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

'use client';

import { useState } from 'react';
import { useShallow } from 'zustand/react/shallow';
import {
  ChevronDown,
  ChevronUp,
  Plus,
  X,
  Loader2,
  CheckCircle2,
  AlertCircle,
  RotateCw,
  CircleX,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  useUploadStore,
  selectVisibleItems,
  selectActiveCount,
  selectHasItems,
  type UploadItem,
  type UploadStatus,
} from '@/lib/upload-store';
import { cn } from '@/lib/utils';

const STATUS_TEXT: Record<UploadStatus, string> = {
  queued: 'Waiting',
  presigning: 'Preparing',
  uploading: 'Uploading',
  finalizing: 'Finishing',
  completed: 'Done',
  error: 'Failed',
  cancelled: 'Cancelled',
};

export function UploadDock() {
  const items = useUploadStore(useShallow(selectVisibleItems));
  const activeCount = useUploadStore(selectActiveCount);
  const hasItems = useUploadStore(selectHasItems);
  const isModalOpen = useUploadStore((s) => s.isModalOpen);
  const canUpload = useUploadStore((s) => s.currentContext?.canUpload ?? false);

  const cancel = useUploadStore((s) => s.cancel);
  const retry = useUploadStore((s) => s.retry);
  const remove = useUploadStore((s) => s.remove);
  const clearFinished = useUploadStore((s) => s.clearFinished);
  const openModal = useUploadStore((s) => s.openModal);

  const [collapsed, setCollapsed] = useState(false);

  if (!hasItems || isModalOpen) return null;

  const completedCount = items.filter((i) => i.status === 'completed').length;
  const errorCount = items.filter((i) => i.status === 'error').length;
  const dismissableCount = items.filter(
    (i) => i.status === 'completed' || i.status === 'cancelled',
  ).length;
  const total = items.length;

  let title: string;
  if (activeCount > 0) {
    title = `Uploading ${activeCount} of ${total}`;
  } else if (errorCount > 0) {
    title = `${errorCount} failed`;
  } else {
    title = `Uploaded ${completedCount} file${completedCount === 1 ? '' : 's'}`;
  }

  return (
    <div
      className={cn(
        'fixed bottom-6 right-6 z-40 w-[360px] max-w-[calc(100vw-3rem)] rounded-lg border bg-card shadow-xl shadow-black/20',
      )}
      role="region"
      aria-label="Upload progress"
    >
      <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
        <div className="flex items-center gap-2 min-w-0">
          {activeCount > 0 && (
            <Loader2 className="h-4 w-4 animate-spin text-primary shrink-0" />
          )}
          <span className="text-sm font-medium truncate">{title}</span>
        </div>
        <div className="flex items-center gap-0.5">
          {canUpload && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={openModal}
              aria-label="Add more files"
            >
              <Plus className="h-4 w-4" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? 'Expand' : 'Collapse'}
          >
            {collapsed ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={clearFinished}
            disabled={dismissableCount === 0}
            aria-label="Dismiss finished"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {!collapsed && (
        <ul className="max-h-[50vh] overflow-y-auto divide-y">
          {items.map((item) => (
            <UploadDockRow
              key={item.id}
              item={item}
              onCancel={() => cancel(item.id)}
              onRetry={() => retry(item.id)}
              onRemove={() => remove(item.id)}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

interface RowProps {
  item: UploadItem;
  onCancel: () => void;
  onRetry: () => void;
  onRemove: () => void;
}

function UploadDockRow({ item, onCancel, onRetry, onRemove }: RowProps) {
  const active =
    item.status === 'presigning' ||
    item.status === 'uploading' ||
    item.status === 'finalizing';
  const queued = item.status === 'queued';
  const failed = item.status === 'error' || item.status === 'cancelled';
  const done = item.status === 'completed';

  return (
    <li className="px-3 py-2.5">
      <div className="flex items-center gap-2 min-w-0">
        <StatusIcon status={item.status} />
        <div className="flex-1 min-w-0">
          <p className="truncate text-sm" title={item.name}>
            {item.name}
          </p>
          {item.parentLabel && (
            <p className="truncate text-xs text-muted-foreground">
              to {item.parentLabel}
            </p>
          )}
        </div>
        <RowAction
          status={item.status}
          onCancel={onCancel}
          onRetry={onRetry}
          onRemove={onRemove}
        />
      </div>
      <div className="mt-1.5 flex items-center gap-2">
        <Progress
          value={done ? 100 : item.progress}
          className={cn('h-1.5 flex-1', failed && 'opacity-50')}
        />
        <span className="text-xs text-muted-foreground w-14 text-right shrink-0">
          {active
            ? `${item.progress}%`
            : queued
              ? STATUS_TEXT.queued
              : STATUS_TEXT[item.status]}
        </span>
      </div>
      {item.error && (
        <p className="mt-1 text-xs text-red-600 dark:text-red-400 truncate" title={item.error}>
          {item.error}
        </p>
      )}
    </li>
  );
}

function StatusIcon({ status }: { status: UploadStatus }) {
  switch (status) {
    case 'completed':
      return <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400 shrink-0" />;
    case 'error':
      return <AlertCircle className="h-4 w-4 text-red-600 dark:text-red-400 shrink-0" />;
    case 'cancelled':
      return <CircleX className="h-4 w-4 text-muted-foreground shrink-0" />;
    case 'presigning':
    case 'uploading':
    case 'finalizing':
      return <Loader2 className="h-4 w-4 animate-spin text-primary shrink-0" />;
    case 'queued':
    default:
      return <Loader2 className="h-4 w-4 text-muted-foreground shrink-0" />;
  }
}

function RowAction({
  status,
  onCancel,
  onRetry,
  onRemove,
}: {
  status: UploadStatus;
  onCancel: () => void;
  onRetry: () => void;
  onRemove: () => void;
}) {
  if (status === 'error' || status === 'cancelled') {
    return (
      <div className="flex items-center gap-0.5 shrink-0">
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={onRetry}
          aria-label="Retry upload"
        >
          <RotateCw className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={onRemove}
          aria-label="Dismiss"
        >
          <X className="h-3.5 w-3.5" />
        </Button>
      </div>
    );
  }
  if (status === 'completed') {
    return (
      <Button
        variant="ghost"
        size="icon"
        className="h-6 w-6 shrink-0"
        onClick={onRemove}
        aria-label="Dismiss"
      >
        <X className="h-3.5 w-3.5" />
      </Button>
    );
  }
  return (
    <Button
      variant="ghost"
      size="icon"
      className="h-6 w-6 shrink-0"
      onClick={onCancel}
      aria-label="Cancel upload"
    >
      <X className="h-3.5 w-3.5" />
    </Button>
  );
}

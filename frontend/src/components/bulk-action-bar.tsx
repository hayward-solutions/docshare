'use client';

import { File } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Share2, Move, Trash2, X } from 'lucide-react';

interface BulkActionBarProps {
  selectedFiles: File[];
  onShare: () => void;
  onMove: () => void;
  onDelete: () => void;
  onClear: () => void;
  /** Hide the share button when the user doesn't own all selected files */
  canShare?: boolean;
}

export function BulkActionBar({
  selectedFiles,
  onShare,
  onMove,
  onDelete,
  onClear,
  canShare = true,
}: BulkActionBarProps) {
  if (selectedFiles.length === 0) return null;

  return (
    <div className="sticky bottom-6 z-40 flex justify-center pointer-events-none">
      <div className="pointer-events-auto flex items-center gap-3 rounded-lg border bg-white px-4 py-3 shadow-lg">
        <span className="text-sm font-medium text-slate-700">
          {selectedFiles.length} selected
        </span>

        <div className="h-5 w-px bg-slate-200" />

        {canShare && (
          <Button variant="outline" size="sm" onClick={onShare}>
            <Share2 className="mr-2 h-4 w-4" />
            Share
          </Button>
        )}
        <Button variant="outline" size="sm" onClick={onMove}>
          <Move className="mr-2 h-4 w-4" />
          Move
        </Button>
        <Button variant="outline" size="sm" className="text-red-600 hover:text-red-700 hover:bg-red-50" onClick={onDelete}>
          <Trash2 className="mr-2 h-4 w-4" />
          Delete
        </Button>

        <div className="h-5 w-px bg-slate-200" />

        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClear}>
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

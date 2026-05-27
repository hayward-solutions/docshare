'use client';

import { Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { useUploadStore, selectHasItems } from '@/lib/upload-store';
import { cn } from '@/lib/utils';

export function UploadFAB() {
  const canUpload = useUploadStore((s) => s.currentContext?.canUpload ?? false);
  const isModalOpen = useUploadStore((s) => s.isModalOpen);
  const hasItems = useUploadStore(selectHasItems);
  const openModal = useUploadStore((s) => s.openModal);
  const label = useUploadStore((s) => s.currentContext?.label);

  if (!canUpload || isModalOpen || hasItems) return null;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          size="icon"
          onClick={openModal}
          className={cn(
            'fixed bottom-6 right-6 z-40 h-14 w-14 rounded-full shadow-lg shadow-black/20',
            'hover:shadow-xl transition-shadow',
          )}
          aria-label="Upload files"
        >
          <Upload className="h-6 w-6" />
        </Button>
      </TooltipTrigger>
      <TooltipContent side="left">
        Upload {label ? `to ${label}` : 'files'}
      </TooltipContent>
    </Tooltip>
  );
}

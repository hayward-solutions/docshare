'use client';

import { useCallback } from 'react';
import { useDropzone } from 'react-dropzone';
import { Upload, FolderOpen } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { useUploadStore } from '@/lib/upload-store';
import { cn } from '@/lib/utils';

export function UploadModal() {
  const isOpen = useUploadStore((s) => s.isModalOpen);
  const closeModal = useUploadStore((s) => s.closeModal);
  const addFiles = useUploadStore((s) => s.addFiles);
  const parentLabel = useUploadStore((s) => s.modalParentLabel);

  const onDrop = useCallback(
    (accepted: File[]) => {
      if (accepted.length === 0) return;
      addFiles(accepted);
    },
    [addFiles],
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop });

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(next) => {
        if (!next) closeModal();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Upload files</DialogTitle>
          <DialogDescription className="flex items-center gap-1.5">
            <FolderOpen className="h-3.5 w-3.5" />
            <span>Uploading to {parentLabel ?? 'My Files'}</span>
          </DialogDescription>
        </DialogHeader>

        <div
          {...getRootProps()}
          className={cn(
            'border-2 border-dashed rounded-lg p-10 text-center cursor-pointer transition-colors',
            isDragActive
              ? 'border-primary bg-primary/5'
              : 'border-border hover:border-primary/50',
          )}
        >
          <input {...getInputProps()} />
          <div className="flex flex-col items-center gap-2 text-muted-foreground">
            <Upload className="h-10 w-10" />
            <p className="text-sm font-medium">
              {isDragActive
                ? 'Drop files here'
                : 'Drag & drop files here, or click to select'}
            </p>
            <p className="text-xs">Select one or more files to upload.</p>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

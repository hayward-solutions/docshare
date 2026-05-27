'use client';

import { useState } from 'react';
import { FileText, FileType, Plus, Table2, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useUploadStore, selectHasItems } from '@/lib/upload-store';
import { cn } from '@/lib/utils';
import { NewDocumentDialog, type NewDocType } from '@/components/new-document-dialog';

export function UploadFAB() {
  const canUpload = useUploadStore((s) => s.currentContext?.canUpload ?? false);
  const isModalOpen = useUploadStore((s) => s.isModalOpen);
  const hasItems = useUploadStore(selectHasItems);
  const openModal = useUploadStore((s) => s.openModal);
  const label = useUploadStore((s) => s.currentContext?.label);
  const parentID = useUploadStore((s) => s.currentContext?.parentID ?? null);

  const [newDocOpen, setNewDocOpen] = useState(false);
  const [newDocType, setNewDocType] = useState<NewDocType>('markdown');

  if (!canUpload) return null;

  const fabHidden = isModalOpen || hasItems || newDocOpen;

  const openNewDoc = (type: NewDocType) => {
    setNewDocType(type);
    setNewDocOpen(true);
  };

  return (
    <>
      {!fabHidden && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              size="icon"
              className={cn(
                'fixed bottom-6 right-6 z-40 h-14 w-14 rounded-full shadow-lg shadow-black/20',
                'hover:shadow-xl transition-shadow',
              )}
              aria-label="Create or upload"
            >
              <Plus className="h-6 w-6" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="end" className="w-56">
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              Create in {label ?? 'My Files'}
            </DropdownMenuLabel>
            <DropdownMenuItem onClick={() => openNewDoc('markdown')}>
              <FileType className="h-4 w-4" />
              New markdown document
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => openNewDoc('spreadsheet')}>
              <Table2 className="h-4 w-4" />
              New spreadsheet
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => openNewDoc('text')}>
              <FileText className="h-4 w-4" />
              New text file
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={openModal}>
              <Upload className="h-4 w-4" />
              Upload files…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <NewDocumentDialog
        open={newDocOpen}
        onOpenChange={setNewDocOpen}
        docType={newDocType}
        parentID={parentID}
        parentLabel={label}
      />
    </>
  );
}

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Loader2 } from 'lucide-react';
import { filesAPI } from '@/lib/api';
import { toast } from 'sonner';

export type NewDocType = 'markdown' | 'text';

interface NewDocumentDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  docType: NewDocType;
  parentID: string | null;
  parentLabel?: string;
}

const DEFAULTS: Record<NewDocType, { name: string; mimeType: string; title: string }> = {
  markdown: {
    name: 'Untitled.md',
    mimeType: 'text/markdown',
    title: 'New markdown document',
  },
  text: {
    name: 'Untitled.txt',
    mimeType: 'text/plain',
    title: 'New text file',
  },
};

const EXTENSIONS: Record<NewDocType, string[]> = {
  markdown: ['.md', '.markdown'],
  text: ['.txt'],
};

function ensureExtension(name: string, docType: NewDocType): string {
  const trimmed = name.trim();
  if (!trimmed) return DEFAULTS[docType].name;
  const lower = trimmed.toLowerCase();
  const hasExt = EXTENSIONS[docType].some((ext) => lower.endsWith(ext));
  if (hasExt) return trimmed;
  return `${trimmed}${EXTENSIONS[docType][0]}`;
}

export function NewDocumentDialog({
  open,
  onOpenChange,
  docType,
  parentID,
  parentLabel,
}: NewDocumentDialogProps) {
  const router = useRouter();
  const [name, setName] = useState(DEFAULTS[docType].name);
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    if (open) {
      setName(DEFAULTS[docType].name);
      setIsCreating(false);
    }
  }, [open, docType]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isCreating) return;
    setIsCreating(true);
    try {
      const finalName = ensureExtension(name, docType);
      const res = await filesAPI.createDoc({
        name: finalName,
        mimeType: DEFAULTS[docType].mimeType,
        parentID,
      });
      if (!res.success) throw new Error(res.error || 'Failed to create document');
      onOpenChange(false);
      router.push(`/edit/${res.data.id}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create document';
      toast.error(message);
      setIsCreating(false);
    }
  };

  const placement = parentLabel ?? 'My Files';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>{DEFAULTS[docType].title}</DialogTitle>
          <DialogDescription>
            Will be created in <span className="font-medium">{placement}</span>.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="doc-name" className="text-right">
                Name
              </Label>
              <Input
                id="doc-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="col-span-3"
                autoFocus
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={isCreating || !name.trim()}>
              {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Create &amp; open
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

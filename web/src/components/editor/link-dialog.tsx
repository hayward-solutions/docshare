'use client';

import { useState } from 'react';
import { Trash2 } from 'lucide-react';
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
import { Checkbox } from '@/components/ui/checkbox';

export interface LinkDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialHref: string;
  initialOpenInNewTab: boolean;
  hasExistingLink: boolean;
  onSubmit: (args: { href: string; openInNewTab: boolean }) => void;
  onRemove: () => void;
}

function normalizeHref(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  // Treat anything without a scheme that starts with a word char as https
  // (the most common user intent is "type the domain, not the protocol").
  // Preserve mailto:, tel:, /relative, #anchor, etc.
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(trimmed)) return trimmed;
  if (trimmed.startsWith('/') || trimmed.startsWith('#') || trimmed.startsWith('?')) return trimmed;
  return `https://${trimmed}`;
}

export function LinkDialog({
  open,
  onOpenChange,
  initialHref,
  initialOpenInNewTab,
  hasExistingLink,
  onSubmit,
  onRemove,
}: LinkDialogProps) {
  const [href, setHref] = useState(initialHref);
  const [openInNewTab, setOpenInNewTab] = useState(initialOpenInNewTab);
  const [error, setError] = useState<string | null>(null);

  // React 19 render-phase reset: when the dialog opens, snap the form to
  // the latest props rather than calling setState in an effect.
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setHref(initialHref);
      setOpenInNewTab(initialOpenInNewTab);
      setError(null);
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const normalized = normalizeHref(href);
    if (!normalized) {
      setError('Enter a URL');
      return;
    }
    onSubmit({ href: normalized, openInNewTab });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{hasExistingLink ? 'Edit link' : 'Insert link'}</DialogTitle>
          <DialogDescription>
            Bare domains get an https:// prefix. Use mailto: or tel: for those targets.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="grid gap-2">
            <Label htmlFor="link-url">URL</Label>
            <Input
              id="link-url"
              value={href}
              onChange={(e) => {
                setHref(e.target.value);
                if (error) setError(null);
              }}
              placeholder="example.com or https://example.com/path"
              autoFocus
              autoComplete="off"
              spellCheck={false}
            />
            {error && <p className="text-xs text-destructive">{error}</p>}
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="link-new-tab"
              checked={openInNewTab}
              onCheckedChange={(v) => setOpenInNewTab(v === true)}
            />
            <Label htmlFor="link-new-tab" className="text-sm font-normal">
              Open in a new tab
            </Label>
          </div>
          <DialogFooter className="gap-2 sm:gap-2">
            {hasExistingLink && (
              <Button
                type="button"
                variant="ghost"
                className="mr-auto text-destructive hover:text-destructive"
                onClick={() => {
                  onRemove();
                  onOpenChange(false);
                }}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                Remove link
              </Button>
            )}
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit">{hasExistingLink ? 'Update' : 'Insert'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

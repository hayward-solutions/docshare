'use client';

import { useState } from 'react';
import { Download, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { filesAPI } from '@/lib/api';

export type ExportFormat = 'pdf' | 'docx' | 'odt' | 'rtf' | 'html' | 'epub' | 'md' | 'txt';

// Mirror of services.maxConvertedBytes in the Go API. The backend
// refuses sources larger than this — surfacing a menu the user can
// click only to get a generic 500 is worse than hiding it.
export const MAX_EXPORTABLE_BYTES = 10 * 1024 * 1024;

// Mirror of services.IsExportableSource in the Go API: markdown and any
// plain-text MIME can be exported. Used by the viewer to decide whether
// to render the menu at all. Accepts null/undefined so callers can pass
// straight from a partially-loaded File object without an extra check.
export function isExportableSourceMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  const m = mimeType.toLowerCase().split(';')[0].trim();
  return m === 'text/markdown' || m === 'text/x-markdown' || m.startsWith('text/');
}

// canExportFile combines the MIME check, the backend size cap, and the
// caller's download-permission flag so the viewer/editor only render an
// Export control when every backend gate would actually pass.
export function canExportFile(file: { mimeType: string; size?: number; canDownload?: boolean }): boolean {
  if (file.canDownload === false) return false;
  if (!isExportableSourceMime(file.mimeType)) return false;
  if (typeof file.size === 'number' && file.size > MAX_EXPORTABLE_BYTES) return false;
  return true;
}

interface ExportOption {
  format: ExportFormat;
  label: string;
  description: string;
}

const MARKDOWN_OPTIONS: ExportOption[] = [
  { format: 'pdf', label: 'PDF', description: 'Portable Document Format' },
  { format: 'docx', label: 'Word (.docx)', description: 'Microsoft Word' },
  { format: 'odt', label: 'OpenDocument (.odt)', description: 'LibreOffice / OpenOffice' },
  { format: 'rtf', label: 'Rich Text (.rtf)', description: 'Compatible with most editors' },
  { format: 'html', label: 'HTML', description: 'Self-contained web page' },
  { format: 'epub', label: 'EPUB', description: 'E-book format' },
  { format: 'md', label: 'Markdown (.md)', description: 'Original source' },
];

const TEXT_OPTIONS: ExportOption[] = [
  { format: 'pdf', label: 'PDF', description: 'Portable Document Format' },
  { format: 'docx', label: 'Word (.docx)', description: 'Microsoft Word' },
  { format: 'txt', label: 'Plain text (.txt)', description: 'Original source' },
];

interface ExportMenuProps {
  fileId: string;
  sourceMime: string;
  disabled?: boolean;
}

export function ExportMenu({ fileId, sourceMime, disabled }: ExportMenuProps) {
  const [isExporting, setIsExporting] = useState<ExportFormat | null>(null);

  const mime = sourceMime.toLowerCase();
  const options = mime.startsWith('text/markdown') || mime.startsWith('text/x-markdown')
    ? MARKDOWN_OPTIONS
    : TEXT_OPTIONS;

  const handleExport = async (format: ExportFormat) => {
    if (isExporting) return;
    setIsExporting(format);
    try {
      const { blob, filename } = await filesAPI.exportFile(fileId, format);
      // Trigger the browser save dialog by clicking a transient <a download>.
      // Using object URLs (not data URLs) so we don't blow the URL-length cap
      // on multi-MB exports.
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      // Defer revocation so the browser has time to start the download in
      // the rare case where the anchor click fires asynchronously.
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Export failed';
      toast.error(message);
    } finally {
      setIsExporting(null);
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" disabled={disabled || isExporting !== null}>
          {isExporting ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Download className="mr-2 h-4 w-4" />
          )}
          Export
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel>Export as…</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {options.map((option) => (
          <DropdownMenuItem
            key={option.format}
            onSelect={() => handleExport(option.format)}
            disabled={isExporting !== null}
          >
            <div className="flex flex-col">
              <span>{option.label}</span>
              <span className="text-xs text-muted-foreground">{option.description}</span>
            </div>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

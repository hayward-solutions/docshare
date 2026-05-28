import { toast } from 'sonner';

export const MAX_IMAGE_BYTES = 2 * 1024 * 1024;

// Mirror of editableContentMaxBytes in api/internal/handlers/files_content.go.
// Used to refuse image inserts that would push the rendered markdown past
// the backend save cap. base64-encoded images inflate ~1.37×, and the
// JSON save body itself adds escape overhead, so the backend caps the
// decoded content at 4 MiB to keep worst-case JSON under the
// 8 MiB SmallBodyLimitForNonUploadRoutes middleware.
export const MAX_CONTENT_BYTES = 4 * 1024 * 1024;

// Overhead for the `![](...)` wrapper Markdown adds around each image src.
const MARKDOWN_IMAGE_OVERHEAD_BYTES = 8;

const ALLOWED_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml']);

/**
 * Reads `file` as a data URI, enforcing both the per-image cap and an
 * optional running document budget. When `docBudgetBytes` is supplied the
 * function refuses an image that would push the markdown past the save
 * cap, showing a toast rather than letting the user discover the 413 at
 * save time.
 */
export async function fileToDataURI(file: File, docBudgetBytes?: number): Promise<string | null> {
  if (!ALLOWED_TYPES.has(file.type)) {
    toast.error(`Unsupported image type: ${file.type || 'unknown'}`);
    return null;
  }
  if (file.size > MAX_IMAGE_BYTES) {
    toast.error(`Image is ${(file.size / 1024 / 1024).toFixed(1)} MiB — limit is 2 MiB. Upload as a separate file and link to it instead.`);
    return null;
  }
  const dataUri = await new Promise<string | null>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null);
    reader.onerror = () => reject(reader.error ?? new Error('failed to read image'));
    reader.readAsDataURL(file);
  });
  if (!dataUri) return null;
  if (docBudgetBytes !== undefined) {
    const insertCost = dataUri.length + MARKDOWN_IMAGE_OVERHEAD_BYTES;
    if (insertCost > docBudgetBytes) {
      toast.error(
        `Inserting this image would exceed the ${(MAX_CONTENT_BYTES / 1024 / 1024) | 0} MiB document save limit. Remove some content first, or upload the image as a separate file.`,
      );
      return null;
    }
  }
  return dataUri;
}

export function isImageFile(file: File): boolean {
  return file.type.startsWith('image/');
}

// UTF-8 byte length of the current markdown content — the unit the backend
// compares against editableContentMaxBytes.
export function markdownByteLength(markdown: string): number {
  if (typeof TextEncoder !== 'undefined') {
    return new TextEncoder().encode(markdown).length;
  }
  // SSR fallback; the editor is client-only but keep this safe just in case.
  return markdown.length;
}

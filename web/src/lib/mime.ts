const EDITABLE_APPLICATION_MIMES = new Set([
  'application/json',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/yaml',
]);

const MARKDOWN_MIMES = new Set(['text/markdown', 'text/x-markdown']);

export function isEditableMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  if (mimeType.startsWith('text/')) return true;
  return EDITABLE_APPLICATION_MIMES.has(mimeType);
}

export function isMarkdownMime(mimeType: string | undefined | null): boolean {
  if (!mimeType) return false;
  return MARKDOWN_MIMES.has(mimeType);
}

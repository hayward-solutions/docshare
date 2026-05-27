import type { File } from './types';

export type SortKey = 'name' | 'size' | 'modified';
export type SortDirection = 'asc' | 'desc';

const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: 'base',
});

export function sortFiles(
  files: File[],
  key: SortKey,
  direction: SortDirection,
): File[] {
  const dirMul = direction === 'asc' ? 1 : -1;
  return [...files].sort((a, b) => {
    if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;

    let cmp = 0;
    switch (key) {
      case 'name':
        cmp = collator.compare(a.name, b.name);
        break;
      case 'size':
        cmp = a.size - b.size;
        break;
      case 'modified':
        cmp = new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime();
        break;
    }
    if (cmp === 0) cmp = collator.compare(a.name, b.name);
    return cmp * dirMul;
  });
}

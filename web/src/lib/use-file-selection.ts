import { useState, useCallback, useMemo } from 'react';
import { File } from '@/lib/types';

export function useFileSelection(files: File[]) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const toggle = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelectedIds(new Set(files.map((f) => f.id)));
  }, [files]);

  const deselectAll = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  const isSelected = useCallback(
    (id: string) => selectedIds.has(id),
    [selectedIds],
  );

  const selectedFiles = useMemo(
    () => files.filter((f) => selectedIds.has(f.id)),
    [files, selectedIds],
  );

  const allSelected = files.length > 0 && selectedIds.size === files.length;
  const someSelected = selectedIds.size > 0 && !allSelected;
  const count = selectedIds.size;

  const toggleAll = useCallback(() => {
    if (allSelected) {
      deselectAll();
    } else {
      selectAll();
    }
  }, [allSelected, deselectAll, selectAll]);

  return {
    selectedIds,
    selectedFiles,
    toggle,
    selectAll,
    deselectAll,
    toggleAll,
    isSelected,
    allSelected,
    someSelected,
    count,
  };
}

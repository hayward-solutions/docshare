'use client';

import { ReactNode } from 'react';
import { TableHead } from '@/components/ui/table';
import { usePreferences } from '@/lib/preferences';
import type { SortKey } from '@/lib/file-sort';
import { ArrowDown, ArrowUp } from 'lucide-react';

interface Props {
  sortKey: SortKey;
  children: ReactNode;
  className?: string;
}

export function SortableTableHead({ sortKey: key, children, className }: Props) {
  const { sortKey, sortDirection, setSort } = usePreferences();
  const isActive = sortKey === key;
  const ariaSort: 'ascending' | 'descending' | 'none' = isActive
    ? sortDirection === 'asc'
      ? 'ascending'
      : 'descending'
    : 'none';

  return (
    <TableHead className={className} aria-sort={ariaSort}>
      <button
        type="button"
        onClick={() =>
          setSort(key, isActive && sortDirection === 'asc' ? 'desc' : 'asc')
        }
        className="-mx-2 flex items-center gap-1 rounded px-2 py-1 transition-colors hover:bg-muted"
      >
        {children}
        {isActive ? (
          sortDirection === 'asc' ? (
            <ArrowUp className="h-3.5 w-3.5" />
          ) : (
            <ArrowDown className="h-3.5 w-3.5" />
          )
        ) : null}
      </button>
    </TableHead>
  );
}

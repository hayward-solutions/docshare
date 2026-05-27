'use client';

import { usePreferences } from '@/lib/preferences';
import type { SortKey } from '@/lib/file-sort';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { ArrowDownUp, ArrowDown, ArrowUp } from 'lucide-react';

const OPTIONS: { key: SortKey; label: string }[] = [
  { key: 'name', label: 'Name' },
  { key: 'size', label: 'Size' },
  { key: 'modified', label: 'Modified' },
];

export function FileSortMenu() {
  const { sortKey, sortDirection, setSort } = usePreferences();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="icon" aria-label="Sort">
          <ArrowDownUp className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuLabel>Sort by</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {OPTIONS.map(({ key, label }) => {
          const isActive = sortKey === key;
          return (
            <DropdownMenuItem
              key={key}
              onClick={() =>
                setSort(
                  key,
                  isActive ? (sortDirection === 'asc' ? 'desc' : 'asc') : 'asc',
                )
              }
            >
              <span className="flex-1">{label}</span>
              {isActive ? (
                sortDirection === 'asc' ? (
                  <ArrowUp className="ml-2 h-4 w-4" />
                ) : (
                  <ArrowDown className="ml-2 h-4 w-4" />
                )
              ) : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

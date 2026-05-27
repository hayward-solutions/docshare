import { create } from 'zustand';
import type { SortKey, SortDirection } from './file-sort';

type ViewMode = 'grid' | 'list';

interface PreferencesState {
  viewMode: ViewMode;
  setViewMode: (mode: ViewMode) => void;
  sortKey: SortKey;
  sortDirection: SortDirection;
  setSort: (key: SortKey, direction: SortDirection) => void;
}

const VIEW_STORAGE_KEY = 'docshare-view-mode';
const SORT_STORAGE_KEY = 'docshare-sort';

function getStoredViewMode(): ViewMode {
  if (typeof window === 'undefined') return 'grid';
  const stored = localStorage.getItem(VIEW_STORAGE_KEY);
  return stored === 'list' ? 'list' : 'grid';
}

function getStoredSort(): { sortKey: SortKey; sortDirection: SortDirection } {
  const fallback = { sortKey: 'name' as SortKey, sortDirection: 'asc' as SortDirection };
  if (typeof window === 'undefined') return fallback;
  try {
    const raw = localStorage.getItem(SORT_STORAGE_KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as { sortKey?: unknown; sortDirection?: unknown };
    const key: SortKey =
      parsed.sortKey === 'size' || parsed.sortKey === 'modified' ? parsed.sortKey : 'name';
    const dir: SortDirection = parsed.sortDirection === 'desc' ? 'desc' : 'asc';
    return { sortKey: key, sortDirection: dir };
  } catch {
    return fallback;
  }
}

const initialSort = getStoredSort();

export const usePreferences = create<PreferencesState>((set) => ({
  viewMode: getStoredViewMode(),
  setViewMode: (mode) => {
    localStorage.setItem(VIEW_STORAGE_KEY, mode);
    set({ viewMode: mode });
  },
  sortKey: initialSort.sortKey,
  sortDirection: initialSort.sortDirection,
  setSort: (key, direction) => {
    localStorage.setItem(
      SORT_STORAGE_KEY,
      JSON.stringify({ sortKey: key, sortDirection: direction }),
    );
    set({ sortKey: key, sortDirection: direction });
  },
}));

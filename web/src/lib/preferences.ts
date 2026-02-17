import { create } from 'zustand';

type ViewMode = 'grid' | 'list';

interface PreferencesState {
  viewMode: ViewMode;
  setViewMode: (mode: ViewMode) => void;
}

const STORAGE_KEY = 'docshare-view-mode';

function getStoredViewMode(): ViewMode {
  if (typeof window === 'undefined') return 'grid';
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === 'list' ? 'list' : 'grid';
}

export const usePreferences = create<PreferencesState>((set) => ({
  viewMode: getStoredViewMode(),
  setViewMode: (mode) => {
    localStorage.setItem(STORAGE_KEY, mode);
    set({ viewMode: mode });
  },
}));

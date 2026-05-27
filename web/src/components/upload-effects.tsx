'use client';

import { useEffect, useRef } from 'react';
import { useUploadStore, selectActiveCount } from '@/lib/upload-store';
import { useActivity } from '@/contexts/activity-context';

export function UploadEffects() {
  const totalCompletions = useUploadStore((s) => s.totalCompletions);
  const activeCount = useUploadStore(selectActiveCount);
  const { refreshActivityCount } = useActivity();

  const lastSeen = useRef(0);
  useEffect(() => {
    if (totalCompletions > lastSeen.current) {
      lastSeen.current = totalCompletions;
      refreshActivityCount();
    }
  }, [totalCompletions, refreshActivityCount]);

  useEffect(() => {
    if (activeCount === 0) return;
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [activeCount]);

  return null;
}

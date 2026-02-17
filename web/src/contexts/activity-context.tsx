'use client';

import { createContext, useContext, useState, useCallback, ReactNode } from 'react';

interface ActivityContextType {
  refreshActivityCount: () => void;
  triggerRefresh: number;
}

const ActivityContext = createContext<ActivityContextType | undefined>(undefined);

export function ActivityProvider({ children }: { children: ReactNode }) {
  const [triggerRefresh, setTriggerRefresh] = useState(0);

  const refreshActivityCount = useCallback(() => {
    setTriggerRefresh(prev => prev + 1);
  }, []);

  return (
    <ActivityContext.Provider value={{ refreshActivityCount, triggerRefresh }}>
      {children}
    </ActivityContext.Provider>
  );
}

export function useActivity() {
  const context = useContext(ActivityContext);
  if (context === undefined) {
    throw new Error('useActivity must be used within an ActivityProvider');
  }
  return context;
}

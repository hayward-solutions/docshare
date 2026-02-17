'use client';

import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { ThemeProvider } from '@/components/theme-provider';
import { useEffect } from 'react';
import { useAuth } from '@/lib/auth';
import { useTheme } from 'next-themes';

function ThemeSync() {
  const user = useAuth((state) => state.user);
  const { setTheme } = useTheme();

  useEffect(() => {
    if (user?.theme) {
      setTheme(user.theme);
    }
  }, [user?.theme, setTheme]);

  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const loadUser = useAuth((state) => state.loadUser);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      <TooltipProvider>
        {children}
        <ThemeSync />
        <Toaster />
      </TooltipProvider>
    </ThemeProvider>
  );
}

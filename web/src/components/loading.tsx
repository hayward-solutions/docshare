import { Loader2 } from 'lucide-react';

export function Loading() {
  return (
    <div className="flex h-full w-full items-center justify-center p-8">
      <Loader2 className="h-8 w-8 animate-spin text-primary" />
      <span className="sr-only">Loading...</span>
    </div>
  );
}

export function LoadingPage() {
  return (
    <div className="flex h-screen w-screen items-center justify-center bg-background">
      <Loader2 className="h-10 w-10 animate-spin text-primary" />
    </div>
  );
}

'use client';

import { useEffect, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { Loader2 } from 'lucide-react';

function CallbackHandler() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { loginWithToken } = useAuth();

  useEffect(() => {
    const token = searchParams.get('token');
    const errorParam = searchParams.get('error');

    if (errorParam) {
      router.push('/login?error=' + encodeURIComponent(errorParam));
      return;
    }

    if (!token) {
      router.push('/login?error=oauth_failed');
      return;
    }

    loginWithToken(token)
      .then(() => {
        router.push('/files');
      })
      .catch((err) => {
        router.push('/login?error=' + encodeURIComponent(err.message || 'Authentication failed'));
      });
  }, [searchParams, router, loginWithToken]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50">
      <div className="text-center">
        <Loader2 className="mx-auto h-8 w-8 animate-spin text-muted-foreground" />
        <p className="mt-4 text-sm text-muted-foreground">Completing sign in...</p>
      </div>
    </div>
  );
}

export default function AuthCallbackPage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center bg-slate-50"><Loader2 className="h-8 w-8 animate-spin text-muted-foreground" /></div>}>
      <CallbackHandler />
    </Suspense>
  );
}

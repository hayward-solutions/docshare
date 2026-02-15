'use client';

import { useState, useEffect, Suspense } from 'react';
import { useAuth } from '@/lib/auth';
import { APP_VERSION, ssoAPI } from '@/lib/api';
import { SSOProvider } from '@/lib/types';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';

function LoginForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [ldapUsername, setLdapUsername] = useState('');
  const [ldapPassword, setLdapPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [providers, setProviders] = useState<SSOProvider[]>([]);
  const [providersLoading, setProvidersLoading] = useState(true);
  const { login, ldapLogin } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const error = searchParams.get('error');
    if (error) {
      toast.error(decodeURIComponent(error));
      router.replace('/login');
    }
  }, [searchParams, router]);

  useEffect(() => {
    ssoAPI.listProviders()
      .then((res) => {
        if (res.success && res.data) {
          setProviders(res.data);
        }
      })
      .catch(console.error)
      .finally(() => setProvidersLoading(false));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    try {
      await login(email, password);
      toast.success('Logged in successfully');
      router.push('/files');
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to login');
    } finally {
      setIsLoading(false);
    }
  };

  const handleLdapSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    try {
      await ldapLogin(ldapUsername, ldapPassword);
      toast.success('Logged in successfully');
      router.push('/files');
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to login');
    } finally {
      setIsLoading(false);
    }
  };

  const handleOAuthLogin = async (provider: string) => {
    try {
      const res = await ssoAPI.getOAuthUrl(provider);
      if (res.success && res.data) {
        window.location.href = res.data.url;
      }
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to initiate OAuth login');
    }
  };

  const oauthProviders = providers.filter(p => p.type === 'oauth' || p.type === 'oidc');
  const hasLdap = providers.some(p => p.type === 'ldap');
  const hasSaml = providers.some(p => p.type === 'saml');

  if (!providersLoading && hasSaml && !email && !ldapUsername) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-[400px]">
          <CardHeader>
            <CardTitle className="text-2xl text-center">Enterprise SSO</CardTitle>
            <CardDescription className="text-center">
              Redirecting to your identity provider...
            </CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </CardContent>
        </Card>
      </div>
    );
  }

  const handleSignIn = (e: React.FormEvent) => {
    if (hasLdap) {
      handleLdapSubmit(e);
    } else {
      handleSubmit(e);
    }
  };

  const showLdap = hasLdap && !hasSaml;
  const showDividerBeforeProviders = !providersLoading && oauthProviders.length > 0;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl text-center">Welcome back</CardTitle>
          <CardDescription className="text-center">
            {showLdap ? 'Sign in with your corporate account' : 'Sign in to your account'}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          <form id="login-form" onSubmit={handleSignIn}>
            <div className="space-y-4">
              {showLdap ? (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="ldap-username">Username</Label>
                    <Input
                      id="ldap-username"
                      type="text"
                      required
                      value={ldapUsername}
                      onChange={(e) => setLdapUsername(e.target.value)}
                      autoComplete="username"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ldap-password">Password</Label>
                    <Input
                      id="ldap-password"
                      type="password"
                      required
                      value={ldapPassword}
                      onChange={(e) => setLdapPassword(e.target.value)}
                      autoComplete="current-password"
                    />
                  </div>
                </>
              ) : (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="email">Email</Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder="m@example.com"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      autoComplete="email"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="password">Password</Label>
                    <Input
                      id="password"
                      type="password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      autoComplete="current-password"
                    />
                  </div>
                </>
              )}
            </div>
          </form>
        </CardContent>
        
        <CardFooter className="flex flex-col space-y-4">
          <Button 
            type="submit" 
            form="login-form"
            className="w-full" 
            disabled={isLoading}
          >
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Sign In
          </Button>

          {showDividerBeforeProviders && (
            <>
              <div className="relative w-full">
                <div className="absolute inset-0 flex items-center">
                  <Separator className="w-full" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-background px-2 text-muted-foreground">Or continue with</span>
                </div>
              </div>

              <div className={oauthProviders.length === 1 ? "flex justify-center w-full" : "grid grid-cols-2 gap-3 w-full"}>
                {oauthProviders.map((provider) => (
                  <Button
                    key={provider.name}
                    variant="outline"
                    onClick={() => handleOAuthLogin(provider.name)}
                    className={oauthProviders.length === 1 ? "" : "w-full"}
                  >
                    {provider.name === 'google' && (
                      <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" aria-hidden="true">
                        <path
                          d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                          fill="#4285F4"
                        />
                        <path
                          d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                          fill="#34A853"
                        />
                        <path
                          d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                          fill="#FBBC05"
                        />
                        <path
                          d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                          fill="#EA4335"
                        />
                      </svg>
                    )}
                    {provider.name === 'github' && (
                      <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                      </svg>
                    )}
                    Sign in with {provider.displayName}
                  </Button>
                ))}
              </div>
            </>
          )}

          {!showLdap && (
            <div className="text-sm text-center text-muted-foreground">
              Don&apos;t have an account?{' '}
              <Link href="/register" className="text-primary hover:underline">
                Sign up
              </Link>
            </div>
          )}
        </CardFooter>
      </Card>
      <p className="text-center text-xs text-muted-foreground">{APP_VERSION}</p>
    </>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center"><Loader2 className="h-6 w-6 animate-spin" /></div>}>
      <LoginForm />
    </Suspense>
  );
}

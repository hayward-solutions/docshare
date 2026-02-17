'use client';

import { useState, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { mfaAPI } from '@/lib/api';
import { decodePublicKeyCredentialRequestOptions, encodeCredentialRequestResponse } from '@/lib/webauthn';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { KeyRound, Fingerprint, ShieldAlert } from 'lucide-react';
import { useWebAuthnSupport } from '@/hooks/use-webauthn-support';

export default function MFAChallengePage() {
  const router = useRouter();
  const { mfaToken, mfaMethods, mfaPending, completeMFALogin, clearMFA } = useAuth();
  const { isSupported: webauthnSupported } = useWebAuthnSupport();
  const [totpCode, setTotpCode] = useState('');
  const [recoveryCode, setRecoveryCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showRecovery, setShowRecovery] = useState(false);

  useEffect(() => {
    if (!mfaPending || !mfaToken) {
      router.push('/login');
    }
  }, [mfaPending, mfaToken, router]);

  const handleTOTPSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    if (!mfaToken || !totpCode) return;
    setLoading(true);
    setError('');
    try {
      const res = await mfaAPI.verifyTOTP(mfaToken, totpCode);
      if (res.success) {
        completeMFALogin(res.data.token, res.data.user);
        router.push('/files');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid code');
    } finally {
      setLoading(false);
    }
  }, [mfaToken, totpCode, completeMFALogin, router]);

  const handleWebAuthn = useCallback(async () => {
    if (!mfaToken) return;
    setLoading(true);
    setError('');
    try {
      const beginRes = await mfaAPI.verifyWebAuthnBegin(mfaToken);
      if (!beginRes.success) throw new Error('Failed to start verification');

      const options = decodePublicKeyCredentialRequestOptions(
        beginRes.data.options as Record<string, unknown>
      );

      const credential = await navigator.credentials.get({
        publicKey: options,
      }) as PublicKeyCredential;

      if (!credential) throw new Error('No credential returned');

      const encoded = encodeCredentialRequestResponse(credential);
      const finishRes = await mfaAPI.verifyWebAuthnFinish(mfaToken, encoded);
      if (finishRes.success) {
        completeMFALogin(finishRes.data.token, finishRes.data.user);
        router.push('/files');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Passkey verification failed');
    } finally {
      setLoading(false);
    }
  }, [mfaToken, completeMFALogin, router]);

  const handleRecoverySubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    if (!mfaToken || !recoveryCode) return;
    setLoading(true);
    setError('');
    try {
      const res = await mfaAPI.verifyRecovery(mfaToken, recoveryCode);
      if (res.success) {
        completeMFALogin(res.data.token, res.data.user);
        router.push('/files');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid recovery code');
    } finally {
      setLoading(false);
    }
  }, [mfaToken, recoveryCode, completeMFALogin, router]);

  const hasTOTP = mfaMethods.includes('totp');
  const hasWebAuthn = mfaMethods.includes('webauthn') && webauthnSupported;
  const defaultTab = hasTOTP ? 'totp' : 'webauthn';

  if (!mfaPending || !mfaToken) {
    return null;
  }

  return (
    <Card>
      <CardHeader className="text-center">
        <CardTitle className="text-2xl">Two-Factor Authentication</CardTitle>
        <CardDescription>
          Verify your identity to continue signing in
        </CardDescription>
      </CardHeader>
      <CardContent>
        {error && (
          <Alert variant="destructive" className="mb-4">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {showRecovery ? (
          <div className="space-y-4">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <ShieldAlert className="h-4 w-4" />
              <span>Enter a recovery code</span>
            </div>
            <form onSubmit={handleRecoverySubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="recovery-code">Recovery Code</Label>
                <Input
                  id="recovery-code"
                  type="text"
                  value={recoveryCode}
                  onChange={(e) => setRecoveryCode(e.target.value)}
                  placeholder="Enter 8-character code"
                  autoFocus
                  disabled={loading}
                />
              </div>
              <Button type="submit" className="w-full" disabled={loading || !recoveryCode}>
                {loading ? 'Verifying...' : 'Use Recovery Code'}
              </Button>
            </form>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => setShowRecovery(false)}
            >
              Back to verification
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            {hasTOTP && hasWebAuthn ? (
              <Tabs defaultValue={defaultTab} className="w-full">
                <TabsList className="grid w-full grid-cols-2">
                  <TabsTrigger value="totp" className="flex items-center gap-2">
                    <KeyRound className="h-4 w-4" />
                    Authenticator
                  </TabsTrigger>
                  <TabsTrigger value="webauthn" className="flex items-center gap-2">
                    <Fingerprint className="h-4 w-4" />
                    Passkey
                  </TabsTrigger>
                </TabsList>
                <TabsContent value="totp" className="mt-4">
                  <form onSubmit={handleTOTPSubmit} className="space-y-4">
                    <div className="space-y-2">
                      <Label htmlFor="totp-code">Authentication Code</Label>
                      <Input
                        id="totp-code"
                        type="text"
                        inputMode="numeric"
                        pattern="[0-9]*"
                        maxLength={6}
                        value={totpCode}
                        onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
                        placeholder="000000"
                        autoComplete="one-time-code"
                        autoFocus
                        disabled={loading}
                      />
                    </div>
                    <Button type="submit" className="w-full" disabled={loading || totpCode.length !== 6}>
                      {loading ? 'Verifying...' : 'Verify'}
                    </Button>
                  </form>
                </TabsContent>
                <TabsContent value="webauthn" className="mt-4">
                  <div className="space-y-4 text-center">
                    <p className="text-sm text-muted-foreground">
                      Use your passkey to verify your identity
                    </p>
                    <Button onClick={handleWebAuthn} className="w-full" disabled={loading}>
                      <Fingerprint className="mr-2 h-4 w-4" />
                      {loading ? 'Waiting for passkey...' : 'Use Passkey'}
                    </Button>
                  </div>
                </TabsContent>
              </Tabs>
            ) : hasTOTP ? (
              <form onSubmit={handleTOTPSubmit} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="totp-code-single">Authentication Code</Label>
                  <Input
                    id="totp-code-single"
                    type="text"
                    inputMode="numeric"
                    pattern="[0-9]*"
                    maxLength={6}
                    value={totpCode}
                    onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
                    placeholder="000000"
                    autoComplete="one-time-code"
                    autoFocus
                    disabled={loading}
                  />
                </div>
                <Button type="submit" className="w-full" disabled={loading || totpCode.length !== 6}>
                  {loading ? 'Verifying...' : 'Verify'}
                </Button>
              </form>
            ) : (
              <div className="space-y-4 text-center">
                <p className="text-sm text-muted-foreground">
                  Use your passkey to verify your identity
                </p>
                <Button onClick={handleWebAuthn} className="w-full" disabled={loading}>
                  <Fingerprint className="mr-2 h-4 w-4" />
                  {loading ? 'Waiting for passkey...' : 'Use Passkey'}
                </Button>
              </div>
            )}

            <div className="pt-2 text-center">
              <button
                type="button"
                className="text-sm text-muted-foreground hover:text-foreground underline"
                onClick={() => setShowRecovery(true)}
              >
                Lost your device? Use a recovery code
              </button>
            </div>
          </div>
        )}

        <div className="mt-4 pt-4 border-t text-center">
          <button
            type="button"
            className="text-sm text-muted-foreground hover:text-foreground"
            onClick={() => {
              clearMFA();
              router.push('/login');
            }}
          >
            Cancel and return to login
          </button>
        </div>
      </CardContent>
    </Card>
  );
}

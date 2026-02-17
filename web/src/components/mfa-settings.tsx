'use client';

import { useState, useEffect, useCallback } from 'react';
import { mfaAPI, passkeyAPI } from '@/lib/api';
import { MFAStatus, WebAuthnCredentialInfo } from '@/lib/types';
import { decodePublicKeyCredentialCreationOptions, encodeCredentialCreationResponse } from '@/lib/webauthn';
import { useWebAuthnSupport } from '@/hooks/use-webauthn-support';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { Shield, KeyRound, Fingerprint, Plus, Trash2, Copy, AlertTriangle, Check } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { formatDistanceToNow } from 'date-fns';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Alert, AlertDescription } from '@/components/ui/alert';

export function MFASettings() {
  const [status, setStatus] = useState<MFAStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [passkeys, setPasskeys] = useState<WebAuthnCredentialInfo[]>([]);
  const { isSupported: webauthnSupported } = useWebAuthnSupport();

  const [totpSetupOpen, setTotpSetupOpen] = useState(false);
  const [totpSecret, setTotpSecret] = useState('');
  const [totpQrUri, setTotpQrUri] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpSetupLoading, setTotpSetupLoading] = useState(false);

  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [recoveryDialogOpen, setRecoveryDialogOpen] = useState(false);

  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [disablePassword, setDisablePassword] = useState('');

  const [passkeyNameDialogOpen, setPasskeyNameDialogOpen] = useState(false);
  const [newPasskeyName, setNewPasskeyName] = useState('');
  const [passkeyLoading, setPasskeyLoading] = useState(false);

  const [deletePasskeyDialogOpen, setDeletePasskeyDialogOpen] = useState(false);
  const [passkeyToDelete, setPasskeyToDelete] = useState<string | null>(null);

  const [regenDialogOpen, setRegenDialogOpen] = useState(false);
  const [regenPassword, setRegenPassword] = useState('');

  const fetchStatus = useCallback(async () => {
    try {
      const [statusRes, passkeysRes] = await Promise.all([
        mfaAPI.getStatus(),
        passkeyAPI.list(),
      ]);
      if (statusRes.success) setStatus(statusRes.data);
      if (passkeysRes.success) setPasskeys(passkeysRes.data);
    } catch {
      // Silently fail on initial load
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const handleTOTPSetup = async () => {
    try {
      const res = await mfaAPI.setupTOTP();
      if (res.success) {
        setTotpSecret(res.data.secret);
        setTotpQrUri(res.data.qrUri);
        setTotpCode('');
        setTotpSetupOpen(true);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to start TOTP setup');
    }
  };

  const handleTOTPVerify = async () => {
    setTotpSetupLoading(true);
    try {
      const res = await mfaAPI.verifyTOTPSetup(totpCode);
      if (res.success) {
        setTotpSetupOpen(false);
        setRecoveryCodes(res.data.recoveryCodes);
        setRecoveryDialogOpen(true);
        toast.success('Authenticator app enabled');
        fetchStatus();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Invalid code');
    } finally {
      setTotpSetupLoading(false);
    }
  };

  const handleTOTPDisable = async () => {
    try {
      const res = await mfaAPI.disableTOTP(disablePassword);
      if (res.success) {
        setDisableDialogOpen(false);
        setDisablePassword('');
        toast.success('Authenticator app disabled');
        fetchStatus();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to disable TOTP');
    }
  };

  const handlePasskeyRegister = async () => {
    setPasskeyLoading(true);
    try {
      const beginRes = await passkeyAPI.registerBegin();
      if (!beginRes.success) throw new Error('Failed to start registration');

      const options = decodePublicKeyCredentialCreationOptions(
        beginRes.data.options as Record<string, unknown>
      );

      const credential = await navigator.credentials.create({
        publicKey: options,
      }) as PublicKeyCredential;

      if (!credential) throw new Error('No credential returned');

      const encoded = encodeCredentialCreationResponse(credential);
      const finishRes = await passkeyAPI.registerFinish(newPasskeyName || 'Passkey', encoded);
      if (finishRes.success) {
        setPasskeyNameDialogOpen(false);
        setNewPasskeyName('');
        toast.success('Passkey registered');
        if (finishRes.data.recoveryCodes) {
          setRecoveryCodes(finishRes.data.recoveryCodes);
          setRecoveryDialogOpen(true);
        }
        fetchStatus();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Passkey registration failed');
    } finally {
      setPasskeyLoading(false);
    }
  };

  const handlePasskeyDeleteClick = (id: string) => {
    setPasskeyToDelete(id);
    setDeletePasskeyDialogOpen(true);
  };

  const confirmPasskeyDelete = async () => {
    if (!passkeyToDelete) return;
    try {
      const res = await passkeyAPI.delete(passkeyToDelete);
      if (res.success) {
        toast.success('Passkey removed');
        fetchStatus();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove passkey');
    } finally {
      setDeletePasskeyDialogOpen(false);
      setPasskeyToDelete(null);
    }
  };

  const handleRegenerateRecovery = async () => {
    try {
      const res = await mfaAPI.regenerateRecovery(regenPassword);
      if (res.success) {
        setRegenDialogOpen(false);
        setRegenPassword('');
        setRecoveryCodes(res.data.recoveryCodes);
        setRecoveryDialogOpen(true);
        fetchStatus();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to regenerate codes');
    }
  };

  const copyRecoveryCodes = () => {
    navigator.clipboard.writeText(recoveryCodes.join('\n'));
    toast.success('Recovery codes copied to clipboard');
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Multi-Factor Authentication
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Loading...</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Multi-Factor Authentication
          </CardTitle>
          <CardDescription>
            Add an extra layer of security to your account
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* TOTP Section */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <KeyRound className="h-4 w-4" />
                <span className="font-medium">Authenticator App</span>
                {status?.totpEnabled ? (
                  <Badge variant="default" className="bg-green-600 dark:bg-green-500">Enabled</Badge>
                ) : (
                  <Badge variant="secondary">Disabled</Badge>
                )}
              </div>
              {status?.totpEnabled ? (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => setDisableDialogOpen(true)}
                >
                  Disable
                </Button>
              ) : (
                <Button size="sm" onClick={handleTOTPSetup}>
                  Set Up
                </Button>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              Use an authenticator app like Google Authenticator or Authy to generate verification codes.
            </p>
          </div>

          <div className="border-t" />

          {/* Passkeys Section */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Fingerprint className="h-4 w-4" />
                <span className="font-medium">Passkeys</span>
                {passkeys.length > 0 ? (
                  <Badge variant="default" className="bg-green-600 dark:bg-green-500">{passkeys.length} registered</Badge>
                ) : (
                  <Badge variant="secondary">None</Badge>
                )}
              </div>
              {webauthnSupported && (
                <Button
                  size="sm"
                  onClick={() => {
                    setNewPasskeyName('');
                    setPasskeyNameDialogOpen(true);
                  }}
                >
                  <Plus className="mr-1 h-3 w-3" />
                  Add Passkey
                </Button>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              Use biometrics, security keys, or your device to sign in without a password.
            </p>

            {passkeys.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Added</TableHead>
                    <TableHead>Last Used</TableHead>
                    <TableHead className="w-[50px]" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {passkeys.map((pk) => (
                    <TableRow key={pk.id}>
                      <TableCell className="font-medium">{pk.name}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDistanceToNow(new Date(pk.createdAt), { addSuffix: true })}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {pk.lastUsedAt
                          ? formatDistanceToNow(new Date(pk.lastUsedAt), { addSuffix: true })
                          : 'Never'}
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-destructive"
                          onClick={() => handlePasskeyDeleteClick(pk.id)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>

          {/* Recovery Codes Section */}
          {status?.mfaEnabled && (
            <>
              <div className="border-t" />
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <span className="font-medium">Recovery Codes</span>
                    <p className="text-sm text-muted-foreground">
                      {status.recoveryCodesRemaining} codes remaining
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setRegenDialogOpen(true)}
                  >
                    Regenerate
                  </Button>
                </div>
                {status.recoveryCodesRemaining <= 3 && (
                  <Alert variant="destructive">
                    <AlertTriangle className="h-4 w-4" />
                    <AlertDescription>
                      You have {status.recoveryCodesRemaining} recovery code{status.recoveryCodesRemaining !== 1 ? 's' : ''} remaining.
                      Consider regenerating new codes.
                    </AlertDescription>
                  </Alert>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* TOTP Setup Dialog */}
      <Dialog open={totpSetupOpen} onOpenChange={setTotpSetupOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Set Up Authenticator App</DialogTitle>
            <DialogDescription>
              Scan the QR code with your authenticator app, then enter the verification code.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {totpQrUri && (
              <div className="flex justify-center p-4 bg-card rounded-lg">
                <QRCodeSVG value={totpQrUri} size={200} />
              </div>
            )}
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Manual entry key</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded bg-muted px-2 py-1 text-xs font-mono break-all">
                  {totpSecret}
                </code>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => {
                    navigator.clipboard.writeText(totpSecret);
                    toast.success('Secret copied');
                  }}
                >
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="totp-verify">Verification Code</Label>
              <Input
                id="totp-verify"
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
                placeholder="000000"
                autoComplete="one-time-code"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={handleTOTPVerify}
              disabled={totpSetupLoading || totpCode.length !== 6}
            >
              {totpSetupLoading ? 'Verifying...' : 'Verify & Enable'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Disable TOTP Dialog */}
      <Dialog open={disableDialogOpen} onOpenChange={setDisableDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Disable Authenticator App</DialogTitle>
            <DialogDescription>
              Enter your password to confirm disabling the authenticator app.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="disable-password">Password</Label>
            <Input
              id="disable-password"
              type="password"
              value={disablePassword}
              onChange={(e) => setDisablePassword(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDisableDialogOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={handleTOTPDisable} disabled={!disablePassword}>
              Disable
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Passkey Name Dialog */}
      <Dialog open={passkeyNameDialogOpen} onOpenChange={setPasskeyNameDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add a Passkey</DialogTitle>
            <DialogDescription>
              Give your passkey a name to identify it later, then follow your browser prompts.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="passkey-name">Passkey Name</Label>
            <Input
              id="passkey-name"
              type="text"
              value={newPasskeyName}
              onChange={(e) => setNewPasskeyName(e.target.value)}
              placeholder="e.g. MacBook Touch ID"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasskeyNameDialogOpen(false)}>Cancel</Button>
            <Button onClick={handlePasskeyRegister} disabled={passkeyLoading}>
              {passkeyLoading ? 'Waiting...' : 'Continue'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Passkey Dialog */}
      <Dialog open={deletePasskeyDialogOpen} onOpenChange={setDeletePasskeyDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove Passkey</DialogTitle>
            <DialogDescription>
              Are you sure you want to remove this passkey? You will no longer be able to use it to sign in.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeletePasskeyDialogOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={confirmPasskeyDelete}>
              Remove Passkey
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Recovery Codes Dialog */}
      <Dialog open={recoveryDialogOpen} onOpenChange={setRecoveryDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Recovery Codes</DialogTitle>
            <DialogDescription>
              Save these recovery codes in a safe place. Each code can only be used once.
              You will not be able to see them again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <Alert>
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>
                If you lose your authenticator device, you can use these codes to access your account.
              </AlertDescription>
            </Alert>
            <div className="rounded-lg bg-muted p-4">
              <div className="grid grid-cols-2 gap-2">
                {recoveryCodes.map((code) => (
                  <code key={code} className="text-sm font-mono">{code}</code>
                ))}
              </div>
            </div>
          </div>
          <DialogFooter className="flex-col gap-2 sm:flex-row">
            <Button variant="outline" onClick={copyRecoveryCodes} className="w-full sm:w-auto">
              <Copy className="mr-2 h-4 w-4" />
              Copy Codes
            </Button>
            <Button onClick={() => setRecoveryDialogOpen(false)} className="w-full sm:w-auto">
              <Check className="mr-2 h-4 w-4" />
              I&apos;ve Saved These Codes
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Regenerate Recovery Dialog */}
      <Dialog open={regenDialogOpen} onOpenChange={setRegenDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Regenerate Recovery Codes</DialogTitle>
            <DialogDescription>
              This will invalidate all existing recovery codes. Enter your password to continue.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="regen-password">Password</Label>
            <Input
              id="regen-password"
              type="password"
              value={regenPassword}
              onChange={(e) => setRegenPassword(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRegenDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleRegenerateRecovery} disabled={!regenPassword}>
              Regenerate
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

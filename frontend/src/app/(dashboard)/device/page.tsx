'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { deviceAPI } from '@/lib/api';
import { DeviceCodeVerification } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { Monitor, CheckCircle2, XCircle, Loader2, ShieldCheck, ArrowRight } from 'lucide-react';

export default function DeviceVerificationPage() {
  const searchParams = useSearchParams();
  const { user } = useAuth();
  
  const [code, setCode] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isApproving, setIsApproving] = useState(false);
  const [verification, setVerification] = useState<DeviceCodeVerification | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);

  const handleVerify = useCallback(async (codeToVerify: string) => {
    if (!codeToVerify || codeToVerify.length < 8) return;
    
    setIsLoading(true);
    setVerification(null);
    
    try {
      const res = await deviceAPI.verify(codeToVerify);
      if (res.success && res.data) {
        setVerification(res.data);
      } else {
        toast.error(res.error || 'Invalid device code');
        setVerification(null);
      }
    } catch (error) {
      console.error('Verification error:', error);
      toast.error('Failed to verify code');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    const urlCode = searchParams.get('code');
    if (urlCode) {
      setCode(urlCode);
      handleVerify(urlCode);
    }
  }, [searchParams, handleVerify]);

  const handleCodeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    let value = e.target.value.toUpperCase();
    
    if (value.length === 4 && code.length === 3) {
      value = value + '-';
    }
    
    // Limit length to 9 (XXXX-XXXX)
    if (value.length <= 9) {
      setCode(value);
    }
  };

  const handleApprove = async () => {
    if (!verification) return;

    setIsApproving(true);
    try {
      const res = await deviceAPI.approve(verification.userCode);
      if (res.success) {
        setIsSuccess(true);
        toast.success('Device authorized successfully');
      } else {
        toast.error(res.error || 'Failed to authorize device');
      }
    } catch (error) {
      console.error('Approval error:', error);
      toast.error('An error occurred while authorizing');
    } finally {
      setIsApproving(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return <Badge variant="outline" className="bg-yellow-50 text-yellow-700 border-yellow-200">Pending Approval</Badge>;
      case 'approved':
        return <Badge variant="outline" className="bg-green-50 text-green-700 border-green-200">Already Approved</Badge>;
      case 'denied':
        return <Badge variant="outline" className="bg-red-50 text-red-700 border-red-200">Denied</Badge>;
      case 'expired':
        return <Badge variant="outline" className="bg-gray-50 text-gray-700 border-gray-200">Expired</Badge>;
      default:
        return <Badge variant="outline">{status}</Badge>;
    }
  };

  if (isSuccess) {
    return (
      <div className="flex items-center justify-center min-h-[60vh] p-4">
        <Card className="w-full max-w-md border-green-100 shadow-lg shadow-green-50/50">
          <CardHeader className="text-center pb-2">
            <div className="mx-auto w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mb-4">
              <CheckCircle2 className="w-8 h-8 text-green-600" />
            </div>
            <CardTitle className="text-2xl text-green-700">Device Authorized!</CardTitle>
            <CardDescription className="text-base pt-2">
              You have successfully logged in to the device.
            </CardDescription>
          </CardHeader>
          <CardContent className="text-center space-y-6 pt-4">
            <p className="text-muted-foreground">
              You can now close this window and return to your CLI or application.
            </p>
            <Button 
              variant="outline" 
              className="w-full"
              onClick={() => window.close()}
            >
              Close Window
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center min-h-[60vh] p-4">
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader className="text-center space-y-2">
          <div className="mx-auto w-12 h-12 bg-primary/10 rounded-full flex items-center justify-center mb-2">
            <Monitor className="w-6 h-6 text-primary" />
          </div>
          <CardTitle className="text-2xl">Connect Device</CardTitle>
          <CardDescription className="text-base">
            Enter the code displayed on your device to authorize access to your account.
          </CardDescription>
        </CardHeader>
        
        <CardContent className="space-y-6">
          {!verification ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Input
                  className="text-center text-2xl font-mono tracking-widest uppercase h-14"
                  placeholder="XXXX-XXXX"
                  value={code}
                  onChange={handleCodeChange}
                  maxLength={9}
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleVerify(code);
                  }}
                />
                <p className="text-xs text-center text-muted-foreground">
                  Enter the 8-character code from your device
                </p>
              </div>
              
              <Button 
                className="w-full h-12 text-base" 
                onClick={() => handleVerify(code)}
                disabled={isLoading || code.length < 8}
              >
                {isLoading ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Verifying...
                  </>
                ) : (
                  'Verify Code'
                )}
              </Button>
            </div>
          ) : (
            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-300">
              <div className="bg-muted/50 rounded-lg p-4 text-center space-y-2 border">
                <p className="text-sm text-muted-foreground uppercase tracking-wider font-medium">Device Code</p>
                <p className="text-3xl font-mono font-bold tracking-widest">{verification.userCode}</p>
                <div className="flex justify-center pt-2">
                  {getStatusBadge(verification.status)}
                </div>
              </div>

              {verification.status === 'pending' && !verification.expired ? (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 p-3 bg-blue-50 text-blue-900 rounded-md border border-blue-100">
                    <ShieldCheck className="w-5 h-5 text-blue-600 shrink-0" />
                    <div className="text-sm">
                      <p className="font-medium">Authorize as {user?.email}</p>
                      <p className="text-blue-700/80 text-xs">This device will have full access to your account.</p>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <Button 
                      variant="outline" 
                      onClick={() => {
                        setVerification(null);
                        setCode('');
                      }}
                    >
                      Cancel
                    </Button>
                    <Button 
                      onClick={handleApprove}
                      disabled={isApproving}
                      className="bg-primary hover:bg-primary/90"
                    >
                      {isApproving ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          Authorizing...
                        </>
                      ) : (
                        <>
                          Authorize
                          <ArrowRight className="ml-2 h-4 w-4" />
                        </>
                      )}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 p-3 bg-red-50 text-red-900 rounded-md border border-red-100">
                    <XCircle className="w-5 h-5 text-red-600 shrink-0" />
                    <div className="text-sm">
                      <p className="font-medium">Cannot Authorize</p>
                      <p className="text-red-700/80 text-xs">
                        {verification.expired 
                          ? 'This code has expired. Please generate a new one.' 
                          : `This code is ${verification.status}.`}
                      </p>
                    </div>
                  </div>
                  <Button 
                    variant="outline" 
                    className="w-full"
                    onClick={() => {
                      setVerification(null);
                      setCode('');
                    }}
                  >
                    Try Another Code
                  </Button>
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

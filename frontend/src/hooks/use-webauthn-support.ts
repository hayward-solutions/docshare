'use client';

import { useEffect, useState } from 'react';

interface WebAuthnSupport {
  isSupported: boolean;
  isChecking: boolean;
}

export function useWebAuthnSupport(): WebAuthnSupport {
  const [support, setSupport] = useState<WebAuthnSupport>({
    isSupported: false,
    isChecking: true,
  });

  useEffect(() => {
    const checkSupport = async () => {
      try {
        if (typeof window === 'undefined') {
          setSupport({ isSupported: false, isChecking: false });
          return;
        }

        const webauthnSupported =
          typeof window.PublicKeyCredential !== 'undefined';

        let platformAuthenticatorAvailable = false;
        if (webauthnSupported && window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable) {
          platformAuthenticatorAvailable =
            await window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable();
        }

        setSupport({
          isSupported: webauthnSupported && platformAuthenticatorAvailable,
          isChecking: false,
        });
      } catch {
        setSupport({ isSupported: false, isChecking: false });
      }
    };

    checkSupport();
  }, []);

  return support;
}

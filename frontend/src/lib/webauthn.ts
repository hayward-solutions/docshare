export function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let str = '';
  for (const byte of bytes) {
    str += String.fromCharCode(byte);
  }
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
  const pad = base64.length % 4;
  const padded = pad ? base64 + '='.repeat(4 - pad) : base64;
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

export function encodeCredentialCreationResponse(credential: PublicKeyCredential): Record<string, unknown> {
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      attestationObject: bufferToBase64url(response.attestationObject),
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
    },
  };
}

export function encodeCredentialRequestResponse(credential: PublicKeyCredential): Record<string, unknown> {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      authenticatorData: bufferToBase64url(response.authenticatorData),
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : null,
    },
  };
}

export function decodePublicKeyCredentialCreationOptions(
  options: Record<string, unknown>
): PublicKeyCredentialCreationOptions {
  const opts = options as Record<string, unknown>;
  
  // The go-webauthn library returns options wrapped in a "publicKey" property
  // This is standard WebAuthn format (protocol.CredentialCreation)
  const pk = (opts.publicKey ?? opts) as Record<string, unknown>;
  
  const challenge = pk.challenge as string;
  const userObj = pk.user as Record<string, unknown>;
  const excludeCredentials = (pk.excludeCredentials as Array<Record<string, unknown>>) || [];

  return {
    ...pk,
    challenge: base64urlToBuffer(challenge),
    user: {
      ...userObj,
      id: base64urlToBuffer(userObj.id as string),
    },
    excludeCredentials: excludeCredentials.map((cred) => ({
      ...cred,
      id: base64urlToBuffer(cred.id as string),
    })),
  } as PublicKeyCredentialCreationOptions;
}

export function decodePublicKeyCredentialRequestOptions(
  options: Record<string, unknown>
): PublicKeyCredentialRequestOptions {
  const opts = options as Record<string, unknown>;
  
  // The go-webauthn library returns options wrapped in a "publicKey" property
  const pk = (opts.publicKey ?? opts) as Record<string, unknown>;
  
  const challenge = pk.challenge as string;
  const allowCredentials = (pk.allowCredentials as Array<Record<string, unknown>>) || [];

  return {
    ...pk,
    challenge: base64urlToBuffer(challenge),
    allowCredentials: allowCredentials.map((cred) => ({
      ...cred,
      id: base64urlToBuffer(cred.id as string),
    })),
  } as PublicKeyCredentialRequestOptions;
}

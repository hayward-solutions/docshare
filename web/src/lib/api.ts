import { Activity, APIToken, APITokenCreateResponse, ApiResponse, DeviceCodeVerification, Group, LinkedAccount, MFAStatus, PasskeyRegisterResponse, PreviewJob, RecoveryCodesResponse, SSOProvider, TOTPSetupResponse, User, WebAuthnCredentialInfo } from './types';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';
export const APP_VERSION = process.env.NEXT_PUBLIC_APP_VERSION || 'dev';

interface FetchOptions extends RequestInit {
  params?: Record<string, string | number | boolean | undefined>;
}

export async function api<T>(endpoint: string, options: FetchOptions = {}): Promise<ApiResponse<T>> {
  const { params, ...init } = options;
  
  let url = `${API_URL}${endpoint}`;
  if (params) {
    const searchParams = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined) {
        searchParams.append(key, String(value));
      }
    });
    url += `?${searchParams.toString()}`;
  }

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...init.headers,
  };

  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(url, {
      ...init,
      headers,
    });

    if (response.status === 401) {
      if (typeof window !== 'undefined') {
        localStorage.removeItem('token');
        window.location.href = '/login';
      }
      throw new Error('Unauthorized');
    }

    const data = await response.json();
    
    if (!response.ok) {
      throw new Error(data.error || 'An error occurred');
    }

    return data;
  } catch (error) {
    console.error('API Error:', error);
    throw error;
  }
}

export const apiMethods = {
  get: <T>(endpoint: string, params?: Record<string, string | number | boolean | undefined>) => api<T>(endpoint, { method: 'GET', params }),
  post: <T>(endpoint: string, body: Record<string, unknown>) => api<T>(endpoint, { method: 'POST', body: JSON.stringify(body) }),
  put: <T>(endpoint: string, body: Record<string, unknown>) => api<T>(endpoint, { method: 'PUT', body: JSON.stringify(body) }),
  delete: <T>(endpoint: string) => api<T>(endpoint, { method: 'DELETE' }),
  upload: <T>(endpoint: string, formData: FormData) => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    const headers: Record<string, string> = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    // Don't set Content-Type for FormData, let browser set it with boundary
    return fetch(`${API_URL}${endpoint}`, {
      method: 'POST',
      headers,
      body: formData,
    }).then(res => res.json() as Promise<ApiResponse<T>>);
  }
};

export const activityAPI = {
  list: async (page = 1, limit = 20) =>
    apiMethods.get<Activity[]>('/activities', { page, limit }),
  unreadCount: async () =>
    apiMethods.get<{ count: number }>('/activities/unread-count'),
  markRead: async (id: string) =>
    apiMethods.put('/activities/' + id + '/read', {}),
  markAllRead: async () =>
    apiMethods.put('/activities/read-all', {}),
};

export const auditAPI = {
  download: async (format: 'csv' | 'json' = 'csv') => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    const headers: Record<string, string> = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const response = await fetch(`${API_URL}/audit-log/export?format=${format}`, { headers });
    if (!response.ok) throw new Error('Failed to download audit log');
    return response.blob();
  },
};

export const tokenAPI = {
  list: async () => apiMethods.get<APIToken[]>('/auth/tokens'),
  create: async (data: { name: string; expiresIn?: string }) =>
    apiMethods.post<APITokenCreateResponse>('/auth/tokens', data),
  revoke: async (id: string) => apiMethods.delete('/auth/tokens/' + id),
};

export const deviceAPI = {
  verify: async (code: string) =>
    apiMethods.get<DeviceCodeVerification>('/auth/device/verify', { code }),
  approve: async (userCode: string) =>
    apiMethods.post<{ message: string }>('/auth/device/approve', { userCode }),
};

export const userAPI = {
  updateProfile: async (data: { firstName?: string; lastName?: string; avatarURL?: string | null; theme?: string }) =>
    apiMethods.put<User>('/auth/me', data),
  changePassword: async (data: { oldPassword: string; newPassword: string }) =>
    apiMethods.put('/auth/password', data),
  getCurrentUser: async () => apiMethods.get<User>('/auth/me'),
  getGroups: async () => apiMethods.get<Group[]>('/groups'),
  uploadAvatar: async (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('parentID', '');
    
    const res = await apiMethods.upload<{ id: string; storagePath?: string }>('/files/upload', formData);
    if (res.success) {
      return { success: true, url: res.data.storagePath ? `${API_URL}/files/${res.data.id}/download-url` : null };
    }
    return { success: false, error: 'Upload failed' };
  }
};

export const versionAPI = {
  get: async () => apiMethods.get<{ version: string; apiVersion: string }>('/version'),
};

export const previewAPI = {
  convert: async (fileId: string) =>
    apiMethods.post<{ job: PreviewJob }>('/files/' + fileId + '/convert-preview', {}),
  getStatus: async (fileId: string) =>
    apiMethods.get<{ job: PreviewJob | null; file: unknown }>('/files/' + fileId + '/preview-status'),
  retry: async (fileId: string) =>
    apiMethods.post<{ job: PreviewJob }>('/files/' + fileId + '/retry-preview', {}),
};

export const ssoAPI = {
  listProviders: async () =>
    apiMethods.get<SSOProvider[]>('/auth/sso/providers'),
  getOAuthUrl: async (provider: string) =>
    apiMethods.get<{ url: string }>('/auth/sso/oauth/' + provider),
  ldapLogin: async (data: { username: string; password: string }) =>
    apiMethods.post<{ token: string; user: User }>('/auth/sso/ldap/login', data),
  listLinkedAccounts: async () =>
    apiMethods.get<LinkedAccount[]>('/auth/linked-accounts'),
  unlinkAccount: async (id: string) =>
    apiMethods.delete('/auth/linked-accounts/' + id),
};

export const mfaAPI = {
  getStatus: async () =>
    apiMethods.get<MFAStatus>('/auth/mfa/status'),
  setupTOTP: async () =>
    apiMethods.post<TOTPSetupResponse>('/auth/mfa/totp/setup', {}),
  verifyTOTPSetup: async (code: string) =>
    apiMethods.post<RecoveryCodesResponse>('/auth/mfa/totp/verify-setup', { code }),
  disableTOTP: async (password: string) =>
    apiMethods.post<{ message: string }>('/auth/mfa/totp/disable', { password }),
  verifyTOTP: async (mfaToken: string, code: string) =>
    apiMethods.post<{ token: string; user: User }>('/auth/mfa/verify/totp', { mfaToken, code }),
  verifyRecovery: async (mfaToken: string, code: string) =>
    apiMethods.post<{ token: string; user: User }>('/auth/mfa/verify/recovery', { mfaToken, code }),
  verifyWebAuthnBegin: async (mfaToken: string) =>
    apiMethods.post<{ options: Record<string, unknown> }>('/auth/mfa/verify/webauthn/begin', { mfaToken }),
  verifyWebAuthnFinish: async (mfaToken: string, response: Record<string, unknown>) =>
    apiMethods.post<{ token: string; user: User }>('/auth/mfa/verify/webauthn/finish', { mfaToken, response }),
  regenerateRecovery: async (password: string) =>
    apiMethods.post<RecoveryCodesResponse>('/auth/mfa/recovery/regenerate', { password }),
};

export const passkeyAPI = {
  registerBegin: async () =>
    apiMethods.post<{ options: Record<string, unknown> }>('/auth/passkey/register/begin', {}),
  registerFinish: async (name: string, response: Record<string, unknown>) =>
    apiMethods.post<PasskeyRegisterResponse>('/auth/passkey/register/finish', { name, response }),
  loginBegin: async () =>
    apiMethods.post<{ options: Record<string, unknown>; challengeID: string }>('/auth/passkey/login/begin', {}),
  loginFinish: async (challengeID: string, response: Record<string, unknown>) =>
    apiMethods.post<{ token: string; user: User }>('/auth/passkey/login/finish', { challengeID, response }),
  list: async () =>
    apiMethods.get<WebAuthnCredentialInfo[]>('/auth/passkeys'),
  rename: async (id: string, name: string) =>
    apiMethods.put<WebAuthnCredentialInfo>('/auth/passkeys/' + id, { name }),
  delete: async (id: string) =>
    apiMethods.delete('/auth/passkeys/' + id),
};

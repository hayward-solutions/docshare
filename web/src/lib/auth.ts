import { create } from 'zustand';
import { User, LoginResponse, MFALoginResponse } from './types';
import { apiMethods } from './api';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  mfaToken: string | null;
  mfaMethods: ('totp' | 'webauthn')[];
  mfaPending: boolean;
  login: (email: string, password: string) => Promise<void>;
  ldapLogin: (username: string, password: string) => Promise<void>;
  loginWithToken: (token: string) => Promise<void>;
  register: (data: { email: string; password: string; firstName: string; lastName: string }) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
  setMFAChallenge: (mfaToken: string, methods: ('totp' | 'webauthn')[]) => void;
  completeMFALogin: (token: string, user: User) => void;
  clearMFA: () => void;
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  token: typeof window !== 'undefined' ? localStorage.getItem('token') : null,
  isAuthenticated: false,
  isLoading: true,
  mfaToken: null,
  mfaMethods: [],
  mfaPending: false,

  login: async (email, password) => {
    try {
      const res = await apiMethods.post<LoginResponse | MFALoginResponse>('/auth/login', { email, password });
      if (res.success) {
        const data = res.data;
        if ('mfaRequired' in data && data.mfaRequired) {
          set({
            mfaToken: data.mfaToken,
            mfaMethods: data.methods,
            mfaPending: true,
          });
          return;
        }
        const loginData = data as LoginResponse;
        localStorage.setItem('token', loginData.token);
        set({ token: loginData.token, user: loginData.user, isAuthenticated: true });
      } else {
        throw new Error(res.error || 'Login failed');
      }
    } catch (error) {
      throw error;
    }
  },

  ldapLogin: async (username, password) => {
    try {
      const res = await apiMethods.post<LoginResponse | MFALoginResponse>('/auth/sso/ldap/login', { username, password });
      if (res.success) {
        const data = res.data;
        if ('mfaRequired' in data && data.mfaRequired) {
          set({
            mfaToken: data.mfaToken,
            mfaMethods: data.methods,
            mfaPending: true,
          });
          return;
        }
        const loginData = data as LoginResponse;
        localStorage.setItem('token', loginData.token);
        set({ token: loginData.token, user: loginData.user, isAuthenticated: true });
      } else {
        throw new Error(res.error || 'LDAP login failed');
      }
    } catch (error) {
      throw error;
    }
  },

  loginWithToken: async (token: string) => {
    localStorage.setItem('token', token);
    set({ token, isAuthenticated: true });

    const res = await apiMethods.get<User>('/auth/me');
    if (res.success) {
      set({ user: res.data });
    } else {
      localStorage.removeItem('token');
      set({ token: null, user: null, isAuthenticated: false });
      throw new Error('Failed to fetch user');
    }
  },

  register: async (data) => {
    try {
      const res = await apiMethods.post<LoginResponse>('/auth/register', data);
      if (res.success) {
        localStorage.setItem('token', res.data.token);
        set({ token: res.data.token, user: res.data.user, isAuthenticated: true });
      } else {
        throw new Error(res.error || 'Registration failed');
      }
    } catch (error) {
      throw error;
    }
  },

  logout: () => {
    localStorage.removeItem('token');
    set({ token: null, user: null, isAuthenticated: false, mfaToken: null, mfaMethods: [], mfaPending: false });
    window.location.href = '/login';
  },

  loadUser: async () => {
    const token = localStorage.getItem('token');
    if (!token) {
      set({ isLoading: false, isAuthenticated: false });
      return;
    }

    try {
      const res = await apiMethods.get<User>('/auth/me');
      if (res.success) {
        set({ user: res.data, isAuthenticated: true, isLoading: false });
      } else {
        set({ token: null, isAuthenticated: false, isLoading: false });
        localStorage.removeItem('token');
      }
    } catch {
      set({ token: null, isAuthenticated: false, isLoading: false });
      localStorage.removeItem('token');
    }
  },

  setMFAChallenge: (mfaToken, methods) => {
    set({ mfaToken, mfaMethods: methods, mfaPending: true });
  },

  completeMFALogin: (token, user) => {
    localStorage.setItem('token', token);
    set({
      token,
      user,
      isAuthenticated: true,
      mfaToken: null,
      mfaMethods: [],
      mfaPending: false,
    });
  },

  clearMFA: () => {
    set({ mfaToken: null, mfaMethods: [], mfaPending: false });
  },
}));

import { create } from 'zustand';
import { User, LoginResponse } from './types';
import { apiMethods } from './api';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  ldapLogin: (username: string, password: string) => Promise<void>;
  loginWithToken: (token: string) => Promise<void>;
  register: (data: { email: string; password: string; firstName: string; lastName: string }) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  token: typeof window !== 'undefined' ? localStorage.getItem('token') : null,
  isAuthenticated: false,
  isLoading: true,

  login: async (email, password) => {
    try {
      const res = await apiMethods.post<LoginResponse>('/api/auth/login', { email, password });
      if (res.success) {
        localStorage.setItem('token', res.data.token);
        set({ token: res.data.token, user: res.data.user, isAuthenticated: true });
      }
    } catch (error) {
      throw error;
    }
  },

  ldapLogin: async (username, password) => {
    try {
      const res = await apiMethods.post<LoginResponse>('/api/auth/sso/ldap/login', { username, password });
      if (res.success) {
        localStorage.setItem('token', res.data.token);
        set({ token: res.data.token, user: res.data.user, isAuthenticated: true });
      }
    } catch (error) {
      throw error;
    }
  },

  loginWithToken: async (token: string) => {
    localStorage.setItem('token', token);
    set({ token, isAuthenticated: true });

    const res = await apiMethods.get<User>('/api/auth/me');
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
      const res = await apiMethods.post<LoginResponse>('/api/auth/register', data);
      if (res.success) {
        localStorage.setItem('token', res.data.token);
        set({ token: res.data.token, user: res.data.user, isAuthenticated: true });
      }
    } catch (error) {
      throw error;
    }
  },

  logout: () => {
    localStorage.removeItem('token');
    set({ token: null, user: null, isAuthenticated: false });
    window.location.href = '/login';
  },

  loadUser: async () => {
    const token = localStorage.getItem('token');
    if (!token) {
      set({ isLoading: false, isAuthenticated: false });
      return;
    }

    try {
      const res = await apiMethods.get<User>('/api/auth/me');
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
}));

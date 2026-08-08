import { create } from 'zustand';
import { apiClient } from '@/lib/api';

interface AuthState {
  username: string | null;
  isAuthenticated: boolean;
  isInitialized: boolean;
  login: (username: string) => void;
  logout: () => Promise<void>;
  initialize: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  username: null,
  isAuthenticated: false,
  isInitialized: false,

  login: (username: string) => {
    // The token lives in an httpOnly cookie set by the server; nothing to store.
    set({ username, isAuthenticated: true, isInitialized: true });
  },

  logout: async () => {
    try {
      await apiClient.logout();
    } catch {
      // Ignore network errors during logout - clear local state regardless
    }
    set({ username: null, isAuthenticated: false });
  },

  initialize: async () => {
    try {
      const me = await apiClient.me();
      set({
        username: me.username || null,
        isAuthenticated: true,
        isInitialized: true,
      });
    } catch {
      set({ username: null, isAuthenticated: false, isInitialized: true });
    }
  },
}));

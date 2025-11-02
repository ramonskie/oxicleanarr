import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { apiClient } from '@/lib/api';

interface AuthState {
  token: string | null;
  username: string | null;
  isAuthenticated: boolean;
  login: (token: string, username: string) => void;
  logout: () => void;
  initialize: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      username: null,
      isAuthenticated: false,
      
      login: (token: string, username: string) => {
        apiClient.setToken(token);
        set({ token, username, isAuthenticated: true });
      },
      
      logout: () => {
        apiClient.setToken(null);
        set({ token: null, username: null, isAuthenticated: false });
      },
      
      initialize: () => {
        const state = get();
        if (state.token) {
          apiClient.setToken(state.token);
          set({ isAuthenticated: true });
        }
      },
    }),
    {
      name: 'prunarr-auth',
    }
  )
);

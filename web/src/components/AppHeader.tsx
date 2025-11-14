import { useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { useThemeStore } from '@/store/theme';
import { Button } from '@/components/ui/button';
import { Clock, LogOut, Settings, LayoutDashboard, Calendar, Library, Trash2, History, Moon, Sun } from 'lucide-react';
import type { ReactNode } from 'react';

interface AppLayoutProps {
  children: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const logout = useAuthStore((state) => state.logout);
  const { theme, toggleTheme } = useThemeStore();

  const { data: syncStatus } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => apiClient.getSyncStatus(),
    refetchInterval: 5000,
  });

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const isActive = (path: string) => location.pathname === path;

  const navItems = [
    { path: '/', label: 'Dashboard', icon: LayoutDashboard },
    { path: '/timeline', label: 'Timeline', icon: Calendar },
    { path: '/library', label: 'Library', icon: Library },
    { path: '/scheduled-deletions', label: 'Scheduled Deletions', icon: Trash2 },
    { path: '/job-history', label: 'Job History', icon: History },
  ];

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {/* Left Sidebar */}
      <aside className="w-64 border-r bg-card flex flex-col">
        {/* Logo/Brand */}
        <div className="p-6 border-b">
          <h1 className="text-2xl font-bold text-primary">OxiCleanarr</h1>
          <p className="text-xs text-muted-foreground mt-1">Media Lifecycle Manager</p>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = isActive(item.path);
            return (
              <Button
                key={item.path}
                variant={active ? 'secondary' : 'ghost'}
                className={`w-full justify-start ${active ? 'bg-accent' : ''}`}
                onClick={() => navigate(item.path)}
              >
                <Icon className="h-4 w-4 mr-3" />
                {item.label}
              </Button>
            );
          })}
        </nav>

        {/* Bottom Section - Sync Status, Theme Toggle, Settings, Logout */}
        <div className="p-4 border-t space-y-2">
          {syncStatus?.in_progress && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground px-3 py-2 bg-accent/50 rounded">
              <Clock className="h-3 w-3 animate-spin" />
              Syncing...
            </div>
          )}
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={toggleTheme}
          >
            {theme === 'dark' ? (
              <>
                <Sun className="h-4 w-4 mr-3" />
                Light Mode
              </>
            ) : (
              <>
                <Moon className="h-4 w-4 mr-3" />
                Dark Mode
              </>
            )}
          </Button>
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={() => navigate('/configuration')}
          >
            <Settings className="h-4 w-4 mr-3" />
            Configuration
          </Button>
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={handleLogout}
          >
            <LogOut className="h-4 w-4 mr-3" />
            Logout
          </Button>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 overflow-y-auto">
        {children}
      </main>
    </div>
  );
}

// Keep AppHeader export for backwards compatibility (wrapper around AppLayout)
export function AppHeader() {
  return null;
}

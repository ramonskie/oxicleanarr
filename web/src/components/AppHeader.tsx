import { useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Clock, LogOut, Settings } from 'lucide-react';

export default function AppHeader() {
  const navigate = useNavigate();
  const location = useLocation();
  const logout = useAuthStore((state) => state.logout);

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

  return (
    <header className="border-b">
      <div className="container mx-auto px-4 py-4 flex items-center justify-between">
        <div className="flex items-center gap-6">
          <h1 className="text-2xl font-bold">OxiCleanarr</h1>
          <nav className="flex gap-4">
            <Button
              variant="ghost"
              className={isActive('/') ? 'bg-accent' : ''}
              onClick={() => navigate('/')}
            >
              Dashboard
            </Button>
            <Button
              variant="ghost"
              className={isActive('/timeline') ? 'bg-accent' : ''}
              onClick={() => navigate('/timeline')}
            >
              Timeline
            </Button>
            <Button
              variant="ghost"
              className={isActive('/library') ? 'bg-accent' : ''}
              onClick={() => navigate('/library')}
            >
              Library
            </Button>
            <Button
              variant="ghost"
              className={isActive('/scheduled-deletions') ? 'bg-accent' : ''}
              onClick={() => navigate('/scheduled-deletions')}
            >
              Scheduled Deletions
            </Button>
            <Button
              variant="ghost"
              className={isActive('/job-history') ? 'bg-accent' : ''}
              onClick={() => navigate('/job-history')}
            >
              Job History
            </Button>
          </nav>
        </div>
        <div className="flex items-center gap-4">
          {syncStatus?.in_progress && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Clock className="h-4 w-4 animate-spin" />
              Syncing...
            </div>
          )}
          <Button variant="ghost" size="sm" onClick={() => navigate('/configuration')}>
            <Settings className="h-4 w-4 mr-2" />
            Configuration
          </Button>
          <Button variant="ghost" size="sm" onClick={handleLogout}>
            <LogOut className="h-4 w-4 mr-2" />
            Logout
          </Button>
        </div>
      </div>
    </header>
  );
}

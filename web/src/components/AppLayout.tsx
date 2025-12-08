// This file is now named AppLayout.tsx
import { useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Clock, LogOut, Settings, LayoutDashboard, Calendar, Library, Trash2, History, ChevronDown, ChevronRight, Sliders, Wrench, Plug, Link, Shield } from 'lucide-react';
import type { ReactNode } from 'react';
import { useState } from 'react';

interface AppLayoutProps {
  children: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps) {
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

  const [settingsExpanded, setSettingsExpanded] = useState(
    location.pathname.startsWith('/settings/') || location.pathname === '/rules' || location.pathname === '/configuration'
  );

  const isActive = (path: string) => location.pathname === path;
  const isSettingsActive = location.pathname.startsWith('/settings/') || location.pathname === '/rules' || location.pathname === '/configuration';

  const navItems = [
    { path: '/', label: 'Dashboard', icon: LayoutDashboard },
    { path: '/library', label: 'Library', icon: Library },
    { path: '/timeline', label: 'Timeline', icon: Calendar },
    { path: '/scheduled-deletions', label: 'Deletions', icon: Trash2 },
    { path: '/job-history', label: 'Activity', icon: History },
  ];

  const settingsSubItems = [
    { path: '/settings/general', label: 'General', icon: Wrench },
    { path: '/settings/integrations', label: 'Integrations', icon: Plug },
    { path: '/settings/symlink', label: 'Symlink Library', icon: Link },
    { path: '/settings/admin', label: 'Server & Admin', icon: Shield },
    { path: '/rules', label: 'Advanced Rules', icon: Sliders },
  ];

  return (
    <div className="flex h-screen overflow-hidden bg-[#1e1e1e] text-gray-200 font-sans">
      {/* Left Sidebar - Arr Style */}
      <aside className="w-[240px] flex-shrink-0 flex flex-col bg-[#262626] border-r border-[#333]">
        {/* Logo Area */}
        <div className="h-[60px] flex items-center px-6 border-b border-[#333]">
           {/* Placeholder for an SVG logo if you have one, using text for now */}
          <span className="text-xl font-bold text-white tracking-tight">
            <span className="text-primary">Oxi</span>CleanArr
          </span>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-4">
          <ul className="space-y-0.5 px-3">
            {navItems.map((item) => {
              const Icon = item.icon;
              const active = isActive(item.path);
              return (
                <li key={item.path}>
                  <button
                    onClick={() => navigate(item.path)}
                    className={`
                      w-full flex items-center px-4 py-2.5 text-sm font-medium rounded-md transition-colors
                      ${active 
                        ? 'bg-primary/20 text-primary border-l-4 border-primary pl-3' 
                        : 'text-gray-400 hover:bg-[#333] hover:text-white border-l-4 border-transparent pl-3'
                      }
                    `}
                  >
                    <Icon className={`h-4 w-4 mr-3 ${active ? 'text-primary' : 'text-gray-400'}`} />
                    {item.label}
                  </button>
                </li>
              );
            })}

            {/* Settings Section with Subsections */}
            <li>
              <button
                onClick={() => setSettingsExpanded(!settingsExpanded)}
                className={`
                  w-full flex items-center justify-between px-4 py-2.5 text-sm font-medium rounded-md transition-colors
                  ${isSettingsActive 
                    ? 'bg-primary/20 text-primary border-l-4 border-primary pl-3' 
                    : 'text-gray-400 hover:bg-[#333] hover:text-white border-l-4 border-transparent pl-3'
                  }
                `}
              >
                <div className="flex items-center">
                  <Settings className={`h-4 w-4 mr-3 ${isSettingsActive ? 'text-primary' : 'text-gray-400'}`} />
                  Settings
                </div>
                {settingsExpanded ? (
                  <ChevronDown className="h-4 w-4" />
                ) : (
                  <ChevronRight className="h-4 w-4" />
                )}
              </button>

              {/* Settings Subsections */}
              {settingsExpanded && (
                <ul className="mt-1 space-y-0.5">
                  {settingsSubItems.map((subItem) => {
                    const SubIcon = subItem.icon;
                    const active = isActive(subItem.path);
                    return (
                      <li key={subItem.path}>
                        <button
                          onClick={() => navigate(subItem.path)}
                          className={`
                            w-full flex items-center px-4 py-2 text-sm font-medium rounded-md transition-colors pl-11
                            ${active 
                              ? 'bg-primary/10 text-primary' 
                              : 'text-gray-400 hover:bg-[#333] hover:text-gray-300'
                            }
                          `}
                        >
                          <SubIcon className={`h-3.5 w-3.5 mr-3 ${active ? 'text-primary' : 'text-gray-500'}`} />
                          {subItem.label}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </li>
          </ul>
        </nav>

        {/* Bottom Actions */}
        <div className="p-4 border-t border-[#333] space-y-2">
            {/* Sync Status - Arr Style Status Bar */}
          {syncStatus?.in_progress && (
            <div className="flex items-center gap-2 text-xs text-blue-400 bg-blue-900/30 px-3 py-2 rounded mb-2 border border-blue-900/50">
              <Clock className="h-3 w-3 animate-spin" />
              <span>Sync in progress...</span>
            </div>
          )}

          <div className="flex items-center justify-center text-gray-400">
              <Button
                variant="ghost"
                size="icon"
                onClick={handleLogout}
                className="hover:text-red-400 hover:bg-[#333]"
                title="Logout"
              >
                <LogOut className="h-4 w-4" />
              </Button>
          </div>
          <div className="text-[10px] text-center text-gray-600 mt-2">
            v0.1.0-alpha
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col overflow-hidden bg-[#141414]">
        <main className="flex-1 overflow-y-auto p-6 scrollbar-thin scrollbar-thumb-[#333] scrollbar-track-transparent">
          {children}
        </main>
      </div>
    </div>
  );
}

// Keep AppHeader export for backwards compatibility (wrapper around AppLayout)
export function AppHeader() {
  return null;
}

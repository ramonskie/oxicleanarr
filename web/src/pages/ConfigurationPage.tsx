import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { LogOut, Save, Settings } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useState } from 'react';
import type { Config, UpdateConfigRequest } from '@/lib/types';

export default function ConfigurationPage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: config, isLoading } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiClient.getConfig(),
  });

  const [formData, setFormData] = useState<Partial<Config>>(config || {});

  // Update formData when config loads
  useState(() => {
    if (config) {
      setFormData(config);
    }
  });

  const updateConfigMutation = useMutation({
    mutationFn: (data: UpdateConfigRequest) => apiClient.updateConfig(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config'] });
      queryClient.invalidateQueries({ queryKey: ['sync-status'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      toast({
        title: 'Success',
        description: 'Configuration updated successfully. Changes will take effect immediately.',
      });
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message,
        variant: 'destructive',
      });
    },
  });

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const handleSave = () => {
    if (!formData) return;
    
    const updateReq: UpdateConfigRequest = {
      app: formData.app,
      sync: formData.sync,
      rules: formData.rules,
    };
    
    updateConfigMutation.mutate(updateReq);
  };

  const handleInputChange = (section: keyof Config, field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [section]: {
        ...(prev[section] as any),
        [field]: value,
      },
    }));
  };

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50">
        <header className="bg-white shadow">
          <div className="mx-auto px-4 sm:px-6 lg:px-8 py-6 flex justify-between items-center">
            <h1 className="text-3xl font-bold text-gray-900">Configuration</h1>
          </div>
        </header>
        <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center">Loading configuration...</div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow">
        <div className="mx-auto px-4 sm:px-6 lg:px-8 py-6 flex justify-between items-center">
          <div className="flex items-center gap-4">
            <Button variant="ghost" onClick={() => navigate('/')}>
              ← Back
            </Button>
            <h1 className="text-3xl font-bold text-gray-900">Configuration</h1>
          </div>
          <div className="flex gap-2">
            <Button onClick={() => navigate('/rules')} variant="outline">
              <Settings className="h-4 w-4 mr-2" />
              Advanced Rules
            </Button>
            <Button 
              onClick={handleSave} 
              disabled={updateConfigMutation.isPending}
            >
              <Save className="h-4 w-4 mr-2" />
              {updateConfigMutation.isPending ? 'Saving...' : 'Save Configuration'}
            </Button>
            <Button onClick={handleLogout} variant="outline">
              <LogOut className="h-4 w-4 mr-2" />
              Logout
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
        <div className="space-y-6">
          {/* App Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Application Settings</CardTitle>
              <CardDescription>
                Core application behavior and safety controls
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Dry Run Mode</label>
                    <p className="text-sm text-gray-500">
                      When enabled, no actual deletions will occur
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.app?.dry_run || false}
                    onChange={(e) => handleInputChange('app', 'dry_run', e.target.checked)}
                    className="h-4 w-4"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Enable Automatic Deletions</label>
                    <p className="text-sm text-gray-500">
                      Allow automatic deletions during sync (requires dry_run to be false)
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.app?.enable_deletion || false}
                    onChange={(e) => handleInputChange('app', 'enable_deletion', e.target.checked)}
                    className="h-4 w-4"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Leaving Soon Days</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Items within this many days of deletion are considered "leaving soon"
                  </p>
                  <Input
                    type="number"
                    value={formData.app?.leaving_soon_days || 7}
                    onChange={(e) => handleInputChange('app', 'leaving_soon_days', parseInt(e.target.value))}
                    min="1"
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Sync Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Sync Settings</CardTitle>
              <CardDescription>
                Sync scheduler configuration (intervals in minutes)
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4">
                <div>
                  <label className="text-sm font-medium">Full Sync Interval (minutes)</label>
                  <p className="text-sm text-gray-500 mb-2">
                    How often to perform a complete library sync
                  </p>
                  <Input
                    type="number"
                    value={formData.sync?.full_interval || 60}
                    onChange={(e) => handleInputChange('sync', 'full_interval', parseInt(e.target.value))}
                    min="5"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Incremental Sync Interval (minutes)</label>
                  <p className="text-sm text-gray-500 mb-2">
                    How often to perform quick updates for changed items
                  </p>
                  <Input
                    type="number"
                    value={formData.sync?.incremental_interval || 15}
                    onChange={(e) => handleInputChange('sync', 'incremental_interval', parseInt(e.target.value))}
                    min="1"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Auto Start</label>
                    <p className="text-sm text-gray-500">
                      Automatically start sync scheduler on application startup
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.sync?.auto_start || false}
                    onChange={(e) => handleInputChange('sync', 'auto_start', e.target.checked)}
                    className="h-4 w-4"
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Retention Rules */}
          <Card>
            <CardHeader>
              <CardTitle>Default Retention Rules</CardTitle>
              <CardDescription>
                Base retention periods for movies and TV shows (e.g., "30d", "90d")
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4">
                <div>
                  <label className="text-sm font-medium">Movie Retention</label>
                  <p className="text-sm text-gray-500 mb-2">
                    How long to keep movies after last watch (e.g., "30d", "90d", "180d")
                  </p>
                  <Input
                    type="text"
                    value={formData.rules?.movie_retention || '90d'}
                    onChange={(e) => handleInputChange('rules', 'movie_retention', e.target.value)}
                    placeholder="90d"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">TV Show Retention</label>
                  <p className="text-sm text-gray-500 mb-2">
                    How long to keep TV shows after last watch (e.g., "30d", "60d", "90d")
                  </p>
                  <Input
                    type="text"
                    value={formData.rules?.tv_retention || '60d'}
                    onChange={(e) => handleInputChange('rules', 'tv_retention', e.target.value)}
                    placeholder="60d"
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Info Card */}
          <Card className="bg-blue-50 border-blue-200">
            <CardHeader>
              <CardTitle className="text-blue-900">Configuration Notes</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-blue-800 space-y-2">
              <p>• Integration settings (Jellyfin, Radarr, Sonarr, etc.) can only be changed via the config file</p>
              <p>• For advanced rules (tag-based, episode limits, user-based), use the Advanced Rules page</p>
              <p>• Changes take effect immediately and will trigger a config reload</p>
              <p>• Config file location: <code className="bg-blue-100 px-1 rounded">/app/config/prunarr.yaml</code></p>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}

import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Save, Clock, RefreshCw, Info, Eye, EyeOff, HardDrive } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useState, useEffect } from 'react';
import type { Config, UpdateConfigRequest, BaseIntegration, DiskThresholdConfig } from '@/lib/types';
import AppLayout from '@/components/AppLayout';
import { ServiceStatusCard } from '@/components/ServiceStatusCard';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

export default function ConfigurationPage() {
  const { section = 'general' } = useParams<{ section: string }>();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: config, isLoading } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiClient.getConfig(),
  });

  const [formData, setFormData] = useState<Partial<Config>>({});
  const [showRestartDialog, setShowRestartDialog] = useState(false);
  const [forceRestart, setForceRestart] = useState(false);
  // Section is now controlled by URL params, not local state
  
  // Track which API keys to show/hide
  const [showApiKeys, setShowApiKeys] = useState<Record<string, boolean>>({
    jellyfin: false,
    radarr: false,
    sonarr: false,
    jellyseerr: false,
    jellystat: false,
    streamystats: false,
  });

  // Track API keys being updated (not sent from backend for security)
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({
    jellyfin: '',
    radarr: '',
    sonarr: '',
    jellyseerr: '',
    jellystat: '',
    streamystats: '',
  });

  // Track password change
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');



  // Update formData when config loads
  useEffect(() => {
    if (config) {
      setFormData(config);
    }
  }, [config]);

  const updateConfigMutation = useMutation({
    mutationFn: (data: UpdateConfigRequest) => apiClient.updateConfig(data),
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ['config'] });
      queryClient.invalidateQueries({ queryKey: ['sync-status'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      // Invalidate media queries to refresh deletion dates after retention rule changes
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      
      // Clear sensitive fields after save
      setApiKeys({
        jellyfin: '',
        radarr: '',
        sonarr: '',
        jellyseerr: '',
        jellystat: '',
        streamystats: '',
      });
      setNewPassword('');
      setConfirmPassword('');
      
      toast({
        title: 'Success',
        description: response.message || 'Configuration updated successfully.',
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

  const restartMutation = useMutation({
    mutationFn: (force: boolean) => apiClient.restartApplication(force),
    onSuccess: () => {
      toast({
        title: 'Restarting',
        description: 'Application is restarting. Please wait a moment...',
      });
      setShowRestartDialog(false);
      
      // Optionally redirect to login or show a loading state
      setTimeout(() => {
        window.location.reload();
      }, 3000);
    },
    onError: (error: Error) => {
      toast({
        title: 'Restart Failed',
        description: error.message,
        variant: 'destructive',
      });
    },
  });



  const handleSave = () => {
    if (!formData) return;
    
    // Validate password if provided
    if (newPassword && newPassword !== confirmPassword) {
      toast({
        title: 'Error',
        description: 'Passwords do not match',
        variant: 'destructive',
      });
      return;
    }
    
    const updateReq: UpdateConfigRequest = {
      app: {
        ...formData.app,
        disk_threshold: formData.app?.disk_threshold,
      },
      sync: formData.sync,
      rules: formData.rules,
      server: formData.server,
      admin: {
        ...formData.admin,
        ...(newPassword ? { password: newPassword } : {}),
      },
      integrations: {
        jellyfin: {
          ...formData.integrations?.jellyfin,
          ...(apiKeys.jellyfin ? { api_key: apiKeys.jellyfin } : {}),
        },
        radarr: {
          ...formData.integrations?.radarr,
          ...(apiKeys.radarr ? { api_key: apiKeys.radarr } : {}),
        },
        sonarr: {
          ...formData.integrations?.sonarr,
          ...(apiKeys.sonarr ? { api_key: apiKeys.sonarr } : {}),
        },
        jellyseerr: {
          ...formData.integrations?.jellyseerr,
          ...(apiKeys.jellyseerr ? { api_key: apiKeys.jellyseerr } : {}),
        },
        jellystat: {
          ...formData.integrations?.jellystat,
          ...(apiKeys.jellystat ? { api_key: apiKeys.jellystat } : {}),
        },
        streamystats: {
          ...formData.integrations?.streamystats,
          ...(apiKeys.streamystats ? { api_key: apiKeys.streamystats } : {}),
        },
      },
    };
    
    updateConfigMutation.mutate(updateReq);
  };

  const handleRestart = () => {
    restartMutation.mutate(forceRestart);
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

  const handleIntegrationChange = (integration: string, field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      integrations: {
        ...prev.integrations,
        [integration]: {
          ...(prev.integrations?.[integration as keyof typeof prev.integrations] as any),
          [field]: value,
        },
      } as any,
    }));
  };

  const handleSymlinkLibraryChange = (field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      integrations: {
        ...prev.integrations,
        jellyfin: {
          ...prev.integrations?.jellyfin,
          symlink_library: {
            ...prev.integrations?.jellyfin?.symlink_library,
            [field]: value,
          } as any,
        } as any,
      } as any,
    }));
  };

  const handleDiskThresholdChange = (field: keyof DiskThresholdConfig, value: any) => {
    setFormData(prev => ({
      ...prev,
      app: {
        ...prev.app,
        disk_threshold: {
          ...prev.app?.disk_threshold,
          [field]: value,
        } as DiskThresholdConfig,
      } as any,
    }));
  };

  const toggleApiKeyVisibility = (integration: string) => {
    setShowApiKeys(prev => ({
      ...prev,
      [integration]: !prev[integration],
    }));
  };

  if (isLoading) {
    return (
      <AppLayout>
        <div className="container mx-auto px-4 py-8">
          <div className="flex items-center justify-center">
            <Clock className="h-8 w-8 animate-spin text-muted-foreground" />
            <span className="ml-2">Loading configuration...</span>
          </div>
        </div>
      </AppLayout>
    );
  }

  const renderIntegrationSection = (
    title: string,
    integration: string,
    data: BaseIntegration | undefined
  ) => (
    <Card key={integration}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          Configure {title} integration settings
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <label className="text-sm font-medium">Enabled</label>
            <p className="text-sm text-gray-500">
              Enable {title} integration
            </p>
          </div>
          <input
            type="checkbox"
            checked={data?.enabled || false}
            onChange={(e) => handleIntegrationChange(integration, 'enabled', e.target.checked)}
            className="h-4 w-4"
          />
        </div>

        <div>
          <label className="text-sm font-medium">URL</label>
          <p className="text-sm text-gray-500 mb-2">
            Base URL for {title} API
          </p>
          <Input
            type="text"
            value={data?.url || ''}
            onChange={(e) => handleIntegrationChange(integration, 'url', e.target.value)}
            placeholder={`https://${integration}.example.com`}
          />
        </div>

        <div>
          <label className="text-sm font-medium">API Key</label>
          <p className="text-sm text-gray-500 mb-2">
            {data?.has_api_key ? 'API key is configured (leave blank to keep current)' : 'No API key configured'}
          </p>
          <div className="flex gap-2">
            <Input
              type={showApiKeys[integration] ? 'text' : 'password'}
              value={apiKeys[integration]}
              onChange={(e) => setApiKeys(prev => ({ ...prev, [integration]: e.target.value }))}
              placeholder={data?.has_api_key ? '••••••••••••••••' : 'Enter API key'}
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              onClick={() => toggleApiKeyVisibility(integration)}
            >
              {showApiKeys[integration] ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </div>
        </div>

        <div>
          <label className="text-sm font-medium">Timeout</label>
          <p className="text-sm text-gray-500 mb-2">
            Request timeout (e.g., "30s", "1m")
          </p>
          <Input
            type="text"
            value={data?.timeout || '30s'}
            onChange={(e) => handleIntegrationChange(integration, 'timeout', e.target.value)}
            placeholder="30s"
          />
        </div>
      </CardContent>
    </Card>
  );

  return (
    <AppLayout>
      <div className="container mx-auto px-4 py-6">
        <div className="flex justify-between items-center mb-6">
          <div className="flex items-center gap-4">
            <h1 className="text-3xl font-bold">
              {section === 'general' && 'General Settings'}
              {section === 'integrations' && 'Integrations'}
              {section === 'symlink' && 'Symlink Library'}
              {section === 'admin' && 'Server & Admin'}
            </h1>
          </div>
          <div className="flex gap-2">
            <Button 
              onClick={() => setShowRestartDialog(true)} 
              variant="outline"
              className="border-orange-500 text-orange-600 hover:bg-orange-50"
            >
              <RefreshCw className="h-4 w-4 mr-2" />
              Restart Application
            </Button>
            <Button 
              onClick={handleSave} 
              disabled={updateConfigMutation.isPending}
            >
              <Save className="h-4 w-4 mr-2" />
              {updateConfigMutation.isPending ? 'Saving...' : 'Save Configuration'}
            </Button>
          </div>
        </div>

        {/* Compact Info Banner */}
        <div className="mb-6 p-3 bg-[#1a1a1a] border border-[#333] rounded-md text-sm text-gray-400">
          <div className="flex items-start gap-3">
            <Info className="h-4 w-4 mt-0.5 text-blue-400 flex-shrink-0" />
            <div className="flex-1">
              <span className="text-gray-300">Most settings apply immediately.</span>
              <span className="text-orange-400 ml-2">Integrations, Symlink Library, and Server/Admin changes require restart.</span>
            </div>
          </div>
        </div>

        {/* General Section */}
        {section === 'general' && (
          <div className="space-y-6">
            {/* App Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Application Settings</CardTitle>
              <CardDescription>
                Core application behavior and safety controls (hot-reload enabled)
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
                Sync scheduler configuration (intervals in minutes, hot-reload enabled)
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
                Base retention periods for movies and TV shows (e.g., "30d", "90d", hot-reload enabled)
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

                <div>
                  <label className="text-sm font-medium">Retention Base</label>
                  <p className="text-sm text-gray-500 mb-2">
                    When to start the retention countdown.
                    <span className="block mt-1 text-xs">
                      <strong>last_watched_or_added</strong> — use last watch date, fall back to added date (default)<br />
                      <strong>last_watched</strong> — only start retention after first watch<br />
                      <strong>added</strong> — pure age-based cleanup, ignores watch activity
                    </span>
                  </p>
                  <select
                    value={formData.rules?.retention_base || ''}
                    onChange={(e) => handleInputChange('rules', 'retention_base', e.target.value || undefined)}
                    className="w-full border border-input rounded-md px-3 py-2 bg-background text-sm"
                  >
                    <option value="">last_watched_or_added (default)</option>
                    <option value="last_watched_or_added">last_watched_or_added</option>
                    <option value="last_watched">last_watched</option>
                    <option value="added">added</option>
                  </select>
                </div>

                <div>
                  <label className="text-sm font-medium">Unwatched Behaviour</label>
                  <p className="text-sm text-gray-500 mb-2">
                    What to do with items that have never been watched.
                    Only relevant when Retention Base is <strong>last_watched</strong>.
                    <span className="block mt-1 text-xs">
                      <strong>added</strong> — use added date for unwatched items (default)<br />
                      <strong>never</strong> — never delete unwatched items
                    </span>
                  </p>
                  <select
                    value={formData.rules?.unwatched_behavior || ''}
                    onChange={(e) => handleInputChange('rules', 'unwatched_behavior', e.target.value || undefined)}
                    disabled={formData.rules?.retention_base !== 'last_watched'}
                    className="w-full border border-input rounded-md px-3 py-2 bg-background text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <option value="">added (default)</option>
                    <option value="added">added</option>
                    <option value="never">never</option>
                  </select>
                </div>

                <div>
                  <label className="text-sm font-medium">Unwatched Retention</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Separate retention period for unwatched items (e.g., "180d").
                    Only used when Retention Base is <strong>last_watched</strong> and Unwatched Behaviour is <strong>added</strong>.
                  </p>
                  <Input
                    type="text"
                    value={formData.rules?.unwatched_retention || ''}
                    onChange={(e) => handleInputChange('rules', 'unwatched_retention', e.target.value || undefined)}
                    placeholder="180d"
                    disabled={
                      formData.rules?.retention_base !== 'last_watched' ||
                      (formData.rules?.unwatched_behavior !== undefined && formData.rules?.unwatched_behavior !== 'added')
                    }
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Disk Threshold */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <HardDrive className="h-5 w-5" />
                Disk Threshold
              </CardTitle>
              <CardDescription>
                Gate deletions based on available disk space. When enabled, retention rules only trigger
                deletions when free space falls below the threshold (hot-reload enabled).
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Enable Disk Threshold</label>
                    <p className="text-sm text-gray-500">
                      Only delete media when free disk space is below the threshold
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.app?.disk_threshold?.enabled || false}
                    onChange={(e) => handleDiskThresholdChange('enabled', e.target.checked)}
                    className="h-4 w-4"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Free Space Threshold (GB)</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Deletions are gated until free space drops below this value
                  </p>
                  <Input
                    type="number"
                    value={formData.app?.disk_threshold?.free_space_gb || 100}
                    onChange={(e) => handleDiskThresholdChange('free_space_gb', parseInt(e.target.value))}
                    min="1"
                    disabled={!formData.app?.disk_threshold?.enabled}
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Check Source</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Which service to query for disk space. Use "lowest" to pick the most constrained volume across both.
                  </p>
                  <select
                    value={formData.app?.disk_threshold?.check_source || 'radarr'}
                    onChange={(e) => handleDiskThresholdChange('check_source', e.target.value)}
                    disabled={!formData.app?.disk_threshold?.enabled}
                    className="w-full h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <option value="radarr">Radarr</option>
                    <option value="sonarr">Sonarr</option>
                    <option value="lowest">Lowest (most constrained)</option>
                  </select>
                </div>
              </div>

              {formData.app?.disk_threshold?.enabled && (
                <div className="mt-2 p-3 bg-blue-950/30 border border-blue-900/40 rounded-md text-sm text-blue-300">
                  <p>
                    When disk threshold is active, media will only be deleted when free space on the{' '}
                    <strong>{formData.app.disk_threshold.check_source || 'radarr'}</strong> volume drops below{' '}
                    <strong>{formData.app.disk_threshold.free_space_gb || 100} GB</strong>.
                    The "Leaving Soon" library always shows upcoming deletions regardless of disk state.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
          </div>
        )}

        {/* Integrations Section */}
        {section === 'integrations' && (
          <div className="space-y-6">
            <div className="mb-4">
              <h2 className="text-xl font-bold mb-2">Integration Settings</h2>
              <p className="text-sm text-muted-foreground">
                Configure external service integrations. Changes require application restart.
              </p>
            </div>

            {/* Services Status */}
            <div className="mb-6">
              <ServiceStatusCard />
            </div>

            {/* Jellyfin Integration */}
          {renderIntegrationSection('Jellyfin', 'jellyfin', formData.integrations?.jellyfin)}

          {/* Other Integrations */}
          {renderIntegrationSection('Radarr', 'radarr', formData.integrations?.radarr)}
          {renderIntegrationSection('Sonarr', 'sonarr', formData.integrations?.sonarr)}
          {renderIntegrationSection('Jellyseerr', 'jellyseerr', formData.integrations?.jellyseerr)}
          {renderIntegrationSection('Jellystat', 'jellystat', formData.integrations?.jellystat)}

          {/* Mutual exclusivity warning */}
          {formData.integrations?.jellystat?.enabled && formData.integrations?.streamystats?.enabled && (
            <div className="rounded-md border border-yellow-400 bg-yellow-50 dark:bg-yellow-950 p-4 text-sm text-yellow-800 dark:text-yellow-200">
              <strong>Warning:</strong> Both Jellystat and Streamystats are enabled. Only one stats provider can be active at a time. Disable one before saving.
            </div>
          )}

          {/* Streamystats Integration */}
          <Card>
            <CardHeader>
              <CardTitle>Streamystats</CardTitle>
              <CardDescription>Configure Streamystats integration settings</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <label className="text-sm font-medium">Enabled</label>
                  <p className="text-sm text-gray-500">Enable Streamystats integration</p>
                </div>
                <input
                  type="checkbox"
                  checked={formData.integrations?.streamystats?.enabled || false}
                  onChange={(e) => handleIntegrationChange('streamystats', 'enabled', e.target.checked)}
                  className="h-4 w-4"
                />
              </div>

              <div>
                <label className="text-sm font-medium">URL</label>
                <p className="text-sm text-gray-500 mb-2">Base URL for Streamystats API</p>
                <Input
                  type="text"
                  value={formData.integrations?.streamystats?.url || ''}
                  onChange={(e) => handleIntegrationChange('streamystats', 'url', e.target.value)}
                  placeholder="https://streamystats.example.com"
                />
              </div>

              <div>
                <label className="text-sm font-medium">API Key (Jellyfin API Key)</label>
                <p className="text-sm text-gray-500 mb-2">
                  {formData.integrations?.streamystats?.has_api_key
                    ? 'API key is configured (leave blank to keep current)'
                    : 'No API key configured'}
                </p>
                <div className="flex gap-2">
                  <Input
                    type={showApiKeys.streamystats ? 'text' : 'password'}
                    value={apiKeys.streamystats}
                    onChange={(e) => setApiKeys(prev => ({ ...prev, streamystats: e.target.value }))}
                    placeholder={formData.integrations?.streamystats?.has_api_key ? '••••••••••••••••' : 'Enter Jellyfin API key'}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => toggleApiKeyVisibility('streamystats')}
                  >
                    {showApiKeys.streamystats ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>

              <div>
                <label className="text-sm font-medium">Server ID</label>
                <p className="text-sm text-gray-500 mb-2">
                  {formData.integrations?.streamystats?.has_server_id
                    ? 'Server ID is configured'
                    : 'Streamystats server UUID (find it in Streamystats → Servers)'}
                </p>
                <Input
                  type="text"
                  value={formData.integrations?.streamystats?.server_id || ''}
                  onChange={(e) => handleIntegrationChange('streamystats', 'server_id', e.target.value)}
                  placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                />
              </div>

              <div>
                <label className="text-sm font-medium">Timeout</label>
                <p className="text-sm text-gray-500 mb-2">Request timeout (e.g., "30s", "1m")</p>
                <Input
                  type="text"
                  value={formData.integrations?.streamystats?.timeout || '30s'}
                  onChange={(e) => handleIntegrationChange('streamystats', 'timeout', e.target.value)}
                  placeholder="30s"
                />
              </div>
            </CardContent>
          </Card>
          </div>
        )}



        {/* Symlink Library Section */}
        {section === 'symlink' && (
          <div className="space-y-6">
            <div className="mb-4">
              <h2 className="text-xl font-bold mb-2">Symlink Library Configuration</h2>
              <p className="text-sm text-muted-foreground">
                Configure the "Leaving Soon" symlink library for Jellyfin. Changes require application restart.
              </p>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>Symlink Library Settings</CardTitle>
                <CardDescription>
                  Create a "Leaving Soon" library with symlinks to media scheduled for deletion
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Enable Symlink Library</label>
                    <p className="text-sm text-gray-500">
                      Create a "Leaving Soon" library with symlinks to media
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.integrations?.jellyfin?.symlink_library?.enabled || false}
                    onChange={(e) => handleSymlinkLibraryChange('enabled', e.target.checked)}
                    className="h-4 w-4"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Base Path</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Root directory for symlink library
                  </p>
                  <Input
                    type="text"
                    value={formData.integrations?.jellyfin?.symlink_library?.base_path || ''}
                    onChange={(e) => handleSymlinkLibraryChange('base_path', e.target.value)}
                    placeholder="/path/to/leaving-soon"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">Movies Library Name</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Name for the movies leaving soon library
                  </p>
                  <Input
                    type="text"
                    value={formData.integrations?.jellyfin?.symlink_library?.movies_library_name || ''}
                    onChange={(e) => handleSymlinkLibraryChange('movies_library_name', e.target.value)}
                    placeholder="Movies - Leaving Soon"
                  />
                </div>

                <div>
                  <label className="text-sm font-medium">TV Shows Library Name</label>
                  <p className="text-sm text-gray-500 mb-2">
                    Name for the TV shows leaving soon library
                  </p>
                  <Input
                    type="text"
                    value={formData.integrations?.jellyfin?.symlink_library?.tv_library_name || ''}
                    onChange={(e) => handleSymlinkLibraryChange('tv_library_name', e.target.value)}
                    placeholder="TV Shows - Leaving Soon"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium">Hide When Empty</label>
                    <p className="text-sm text-gray-500">
                      Hide library when there are no items leaving soon
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={formData.integrations?.jellyfin?.symlink_library?.hide_when_empty || false}
                    onChange={(e) => handleSymlinkLibraryChange('hide_when_empty', e.target.checked)}
                    className="h-4 w-4"
                  />
              </div>
            </CardContent>
          </Card>
          </div>
        )}

        {/* Server & Admin Section */}
        {section === 'admin' && (
          <div className="space-y-6">
            <div className="mb-4">
              <h2 className="text-xl font-bold mb-2">Server & Admin Settings</h2>
              <p className="text-sm text-muted-foreground">
                Configure server settings and admin credentials. Changes require application restart.
              </p>
            </div>

          <Card>
            <CardHeader>
              <CardTitle>Server Configuration</CardTitle>
              <CardDescription>
                HTTP server binding settings
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium">Host</label>
                <p className="text-sm text-gray-500 mb-2">
                  Host address to bind to (e.g., "0.0.0.0" for all interfaces)
                </p>
                <Input
                  type="text"
                  value={formData.server?.host || '0.0.0.0'}
                  onChange={(e) => handleInputChange('server', 'host', e.target.value)}
                  placeholder="0.0.0.0"
                />
              </div>

              <div>
                <label className="text-sm font-medium">Port</label>
                <p className="text-sm text-gray-500 mb-2">
                  Port number to listen on
                </p>
                <Input
                  type="number"
                  value={formData.server?.port || 8080}
                  onChange={(e) => handleInputChange('server', 'port', parseInt(e.target.value))}
                  min="1"
                  max="65535"
                />
              </div>
            </CardContent>
          </Card>

          {/* Admin Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Authentication & Admin</CardTitle>
              <CardDescription>
                Manage admin credentials and authentication settings
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium">Admin Username</label>
                <p className="text-sm text-gray-500 mb-2">
                  Username for admin login
                </p>
                <Input
                  type="text"
                  value={formData.admin?.username || ''}
                  onChange={(e) => handleInputChange('admin', 'username', e.target.value)}
                  placeholder="admin"
                />
              </div>

              <div>
                <label className="text-sm font-medium">New Password</label>
                <p className="text-sm text-gray-500 mb-2">
                  Leave blank to keep current password
                </p>
                <Input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Enter new password"
                />
              </div>

              <div>
                <label className="text-sm font-medium">Confirm Password</label>
                <p className="text-sm text-gray-500 mb-2">
                  Re-enter new password to confirm
                </p>
                <Input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Confirm new password"
                />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <label className="text-sm font-medium">Disable Authentication</label>
                  <p className="text-sm text-gray-500">
                    Disable login requirement (not recommended for production)
                  </p>
                </div>
                <input
                  type="checkbox"
                  checked={formData.admin?.disable_auth || false}
                  onChange={(e) => handleInputChange('admin', 'disable_auth', e.target.checked)}
                  className="h-4 w-4"
                />
              </div>
            </CardContent>
          </Card>
          </div>
        )}
      </div>

      {/* Restart Confirmation Dialog */}
      <Dialog open={showRestartDialog} onOpenChange={setShowRestartDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restart Application</DialogTitle>
            <DialogDescription>
              Are you sure you want to restart the application? This will disconnect all active users and take a few seconds.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <div className="flex items-center space-x-2">
              <input
                type="checkbox"
                id="force-restart"
                checked={forceRestart}
                onChange={(e) => setForceRestart(e.target.checked)}
                className="h-4 w-4"
              />
              <label htmlFor="force-restart" className="text-sm font-medium">
                Force restart even if sync is running
              </label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowRestartDialog(false)}>
              Cancel
            </Button>
            <Button 
              onClick={handleRestart}
              disabled={restartMutation.isPending}
              className="bg-orange-500 hover:bg-orange-600"
            >
              <RefreshCw className="h-4 w-4 mr-2" />
              {restartMutation.isPending ? 'Restarting...' : 'Restart Now'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout>
  );
}

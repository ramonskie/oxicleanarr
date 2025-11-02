import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Film, Tv, Clock, LogOut, Shield, ShieldOff, Calendar } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import type { MediaItem } from '@/lib/types';

interface GroupedMedia {
  date: string;
  items: MediaItem[];
}

export default function TimelinePage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: leavingSoon, isLoading } = useQuery({
    queryKey: ['leaving-soon-all'],
    queryFn: () => apiClient.listLeavingSoon({ limit: 1000 }), // Get all leaving soon items
  });

  const { data: syncStatus } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => apiClient.getSyncStatus(),
    refetchInterval: 5000,
  });

  // Group media items by deletion date
  const groupedByDate = useMemo(() => {
    if (!leavingSoon?.items) return [];

    const groups: Record<string, MediaItem[]> = {};

    leavingSoon.items.forEach((item) => {
      if (!item.deletion_date) return;

      // Extract just the date part (YYYY-MM-DD)
      const date = item.deletion_date.split('T')[0];
      
      if (!groups[date]) {
        groups[date] = [];
      }
      groups[date].push(item);
    });

    // Convert to array and sort by date
    const result: GroupedMedia[] = Object.entries(groups)
      .map(([date, items]) => ({ date, items }))
      .sort((a, b) => a.date.localeCompare(b.date));

    return result;
  }, [leavingSoon]);

  const excludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.addExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      toast({
        title: 'Excluded',
        description: 'Item has been added to the exclusion list',
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

  const unexcludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.removeExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      toast({
        title: 'Unexcluded',
        description: 'Item has been removed from the exclusion list',
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

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const today = new Date();
    const tomorrow = new Date(today);
    tomorrow.setDate(tomorrow.getDate() + 1);

    // Reset time for comparison
    today.setHours(0, 0, 0, 0);
    tomorrow.setHours(0, 0, 0, 0);
    date.setHours(0, 0, 0, 0);

    if (date.getTime() === today.getTime()) {
      return 'Today';
    } else if (date.getTime() === tomorrow.getTime()) {
      return 'Tomorrow';
    } else {
      return date.toLocaleDateString('en-US', {
        weekday: 'long',
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      });
    }
  };

  const getDaysUntil = (dateString: string) => {
    const date = new Date(dateString);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    date.setHours(0, 0, 0, 0);
    
    const diffTime = date.getTime() - today.getTime();
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
    
    return diffDays;
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return 'N/A';
    const gb = bytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(2)} GB`;
  };

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <h1 className="text-2xl font-bold">Prunarr</h1>
            <nav className="flex gap-4">
              <Button variant="ghost" onClick={() => navigate('/')}>
                Dashboard
              </Button>
              <Button variant="ghost" className="bg-accent">
                Timeline
              </Button>
              <Button variant="ghost" onClick={() => navigate('/library')}>
                Library
              </Button>
              <Button variant="ghost" onClick={() => navigate('/scheduled-deletions')}>
                Scheduled Deletions
              </Button>
              <Button variant="ghost" onClick={() => navigate('/job-history')}>
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
            <Button variant="ghost" size="sm" onClick={handleLogout}>
              <LogOut className="h-4 w-4 mr-2" />
              Logout
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        <div className="mb-6">
          <h2 className="text-3xl font-bold mb-2">Deletion Timeline</h2>
          <p className="text-muted-foreground">
            Media items scheduled for deletion, grouped by date
          </p>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Clock className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : groupedByDate.length === 0 ? (
          <Card>
            <CardContent className="py-12">
              <div className="text-center text-muted-foreground">
                <Calendar className="h-12 w-12 mx-auto mb-4 opacity-20" />
                <p className="text-lg font-medium mb-1">No items scheduled for deletion</p>
                <p className="text-sm">All media items are within retention policy</p>
              </div>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-8">
            {groupedByDate.map((group) => {
              const daysUntil = getDaysUntil(group.date);
              const totalSize = group.items.reduce((sum, item) => sum + (item.file_size || 0), 0);
              
              return (
                <div key={group.date} className="space-y-4">
                  {/* Date Header */}
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="text-xl font-semibold flex items-center gap-2">
                        <Calendar className="h-5 w-5" />
                        {formatDate(group.date)}
                      </h3>
                      <p className="text-sm text-muted-foreground mt-1">
                        {daysUntil === 0 ? 'Today' : daysUntil === 1 ? '1 day away' : `${daysUntil} days away`} • {group.items.length} item{group.items.length !== 1 ? 's' : ''} • {formatFileSize(totalSize)} to be freed
                      </p>
                    </div>
                  </div>

                  {/* Items Card */}
                  <Card>
                    <CardContent className="p-0">
                      <div className="divide-y">
                        {group.items.map((item) => (
                          <div
                            key={item.id}
                            className="p-4 hover:bg-accent transition-colors"
                          >
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-4 flex-1 min-w-0">
                                {item.type === 'movie' ? (
                                  <Film className="h-8 w-8 text-muted-foreground flex-shrink-0" />
                                ) : (
                                  <Tv className="h-8 w-8 text-muted-foreground flex-shrink-0" />
                                )}
                                <div className="flex-1 min-w-0">
                                  <h4 className="font-medium truncate">{item.title}</h4>
                                  <div className="flex items-center gap-2 mt-1">
                                    {item.year && (
                                      <span className="text-sm text-muted-foreground">
                                        {item.year}
                                      </span>
                                    )}
                                    {item.file_size && (
                                      <>
                                        <span className="text-muted-foreground">•</span>
                                        <span className="text-sm text-muted-foreground">
                                          {formatFileSize(item.file_size)}
                                        </span>
                                      </>
                                    )}
                                  </div>
                                  {item.deletion_reason && (
                                    <p className="text-xs text-muted-foreground mt-1 line-clamp-1">
                                      {item.deletion_reason}
                                    </p>
                                  )}
                                </div>
                              </div>
                              <div className="flex items-center gap-3 flex-shrink-0">
                                <Badge variant={item.type === 'movie' ? 'movie' : 'show'}>
                                  {item.type === 'movie' ? 'Movie' : 'TV Show'}
                                </Badge>
                                {item.excluded ? (
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => unexcludeMutation.mutate(item.id)}
                                    disabled={unexcludeMutation.isPending}
                                  >
                                    <ShieldOff className="h-4 w-4 mr-2" />
                                    Unexclude
                                  </Button>
                                ) : (
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => excludeMutation.mutate(item.id)}
                                    disabled={excludeMutation.isPending}
                                  >
                                    <Shield className="h-4 w-4 mr-2" />
                                    Exclude
                                  </Button>
                                )}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              );
            })}
          </div>
        )}
      </main>
    </div>
  );
}

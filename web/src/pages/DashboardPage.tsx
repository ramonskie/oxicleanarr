import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Film, Tv, Clock, LogOut, Shield, ShieldOff } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';

export default function DashboardPage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: leavingSoon } = useQuery({
    queryKey: ['leaving-soon'],
    queryFn: () => apiClient.listLeavingSoon({ limit: 10 }),
  });

  const { data: excluded } = useQuery({
    queryKey: ['excluded'],
    queryFn: () => apiClient.listExcluded({ limit: 20 }),
  });

  const { data: movies } = useQuery({
    queryKey: ['movies'],
    queryFn: () => apiClient.listMovies({ limit: 5 }),
  });

  const { data: shows } = useQuery({
    queryKey: ['shows'],
    queryFn: () => apiClient.listShows({ limit: 5 }),
  });

  const { data: syncStatus } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => apiClient.getSyncStatus(),
    refetchInterval: 5000, // Poll every 5 seconds
  });

  const excludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.addExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['excluded'] });
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
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
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['excluded'] });
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
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

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <h1 className="text-2xl font-bold">Prunarr</h1>
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
        <div className="grid gap-6">
          {/* Stats Cards */}
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Movies</CardTitle>
                <Film className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{movies?.total || 0}</div>
                <p className="text-xs text-muted-foreground">
                  Total movies in library
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">TV Shows</CardTitle>
                <Tv className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{shows?.total || 0}</div>
                <p className="text-xs text-muted-foreground">
                  Total shows in library
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Leaving Soon</CardTitle>
                <Clock className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{leavingSoon?.total || 0}</div>
                <p className="text-xs text-muted-foreground">
                  Items to be deleted
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Leaving Soon Section */}
          {leavingSoon && leavingSoon.items.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>Leaving Soon</CardTitle>
                <CardDescription>
                  Media items scheduled for deletion
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {leavingSoon.items.map((item) => (
                    <div
                      key={item.id}
                      className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent transition-colors"
                    >
                      <div className="flex items-center gap-4">
                        {item.type === 'movie' ? (
                          <Film className="h-8 w-8 text-muted-foreground" />
                        ) : (
                          <Tv className="h-8 w-8 text-muted-foreground" />
                        )}
                        <div>
                          <h3 className="font-medium">{item.title}</h3>
                          {item.year && (
                            <p className="text-sm text-muted-foreground">
                              {item.year}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center gap-4">
                        <Badge variant={item.type === 'movie' ? 'movie' : 'show'}>
                          {item.type === 'movie' ? 'Movie' : 'TV Show'}
                        </Badge>
                        <div className="text-right">
                          {item.days_until_deletion !== undefined && (
                            <p className="text-sm font-medium">
                              {item.days_until_deletion} days
                            </p>
                          )}
                          <p className="text-xs text-muted-foreground">
                            until deletion
                          </p>
                        </div>
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
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Excluded Items Section */}
          <Card>
            <CardHeader>
              <CardTitle>Excluded Items</CardTitle>
              <CardDescription>
                Media items protected from automatic deletion
              </CardDescription>
            </CardHeader>
            <CardContent>
              {excluded && excluded.items.length > 0 ? (
                <div className="space-y-4">
                  {excluded.items.map((item) => (
                    <div
                      key={item.id}
                      className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent transition-colors"
                    >
                      <div className="flex items-center gap-4">
                        {item.type === 'movie' ? (
                          <Film className="h-8 w-8 text-muted-foreground" />
                        ) : (
                          <Tv className="h-8 w-8 text-muted-foreground" />
                        )}
                        <div>
                          <h3 className="font-medium">{item.title}</h3>
                          {item.year && (
                            <p className="text-sm text-muted-foreground">
                              {item.year}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center gap-4">
                        <Badge variant={item.type === 'movie' ? 'movie' : 'show'}>
                          {item.type === 'movie' ? 'Movie' : 'TV Show'}
                        </Badge>
                        <div className="flex items-center gap-2 text-sm text-muted-foreground">
                          <Shield className="h-4 w-4 text-green-600" />
                          <span>Protected</span>
                        </div>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => unexcludeMutation.mutate(item.id)}
                          disabled={unexcludeMutation.isPending}
                        >
                          <ShieldOff className="h-4 w-4 mr-2" />
                          Unexclude
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-center py-8 text-muted-foreground">
                  <Shield className="h-12 w-12 mx-auto mb-2 opacity-20" />
                  <p>No excluded items</p>
                  <p className="text-sm">Click "Exclude" on items to protect them from deletion</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}

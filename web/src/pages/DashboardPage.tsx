import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Film, Shield, ShieldOff, AlertTriangle, Clock, Monitor, Timer, TimerOff } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useMemo, useState } from 'react';
import AppLayout from '@/components/AppLayout';
import { MediaPoster } from '@/components/MediaPoster';
import { hasPoster } from '@/lib/imageUtils';

export default function DashboardPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  
  // Confirmation dialogs
  const [excludeConfirm, setExcludeConfirm] = useState<{ id: string; title: string } | null>(null);
  const [unexcludeConfirm, setUnexcludeConfirm] = useState<{ id: string; title: string } | null>(null);
  const [manualLeavingSoonConfirm, setManualLeavingSoonConfirm] = useState<{ id: string; title: string } | null>(null);
  const [removeManualLeavingSoonConfirm, setRemoveManualLeavingSoonConfirm] = useState<{ id: string; title: string } | null>(null);

  const { data: leavingSoon } = useQuery({
    queryKey: ['leaving-soon'],
    queryFn: () => apiClient.listLeavingSoon({ limit: 10 }),
  });

  const { data: excluded } = useQuery({
    queryKey: ['excluded'],
    queryFn: () => apiClient.listExcluded({ limit: 20 }),
  });

  const { data: movies, isLoading: moviesLoading } = useQuery({
    queryKey: ['movies'],
    queryFn: () => apiClient.listMovies(),
  });

  const { data: shows, isLoading: showsLoading } = useQuery({
    queryKey: ['shows'],
    queryFn: () => apiClient.listShows(),
  });

  const { data: unmatched, isLoading: unmatchedLoading } = useQuery({
    queryKey: ['unmatched'],
    queryFn: () => apiClient.listUnmatched(),
  });

  // Keep jobs query active for cache warming (invalidated by config/rule changes)
  useQuery({
    queryKey: ['jobs'],
    queryFn: () => apiClient.listJobs(),
  });

  // Memoize scheduled deletions count calculation
  const scheduledDeletionsCount = useMemo(() => {
    const now = new Date();
    const allItems = [
      ...(movies?.items || []),
      ...(shows?.items || []),
    ];
    
    return allItems.filter(item => {
      if (!item.deletion_date || item.deletion_date === '0001-01-01T00:00:00Z') return false;
      if (item.excluded) return false;
      const deletionDate = new Date(item.deletion_date);
      return deletionDate < now; // Overdue items only
    }).length;
  }, [movies?.items, shows?.items]);

  const excludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.addExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['excluded'] });
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      setExcludeConfirm(null);
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
      setUnexcludeConfirm(null);
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

  const addManualLeavingSoonMutation = useMutation({
    mutationFn: (id: string) => apiClient.addManualLeavingSoon(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      setManualLeavingSoonConfirm(null);
      toast({
        title: 'Leaving Soon',
        description: 'Item has been manually flagged as leaving soon',
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

  const removeManualLeavingSoonMutation = useMutation({
    mutationFn: (id: string) => apiClient.removeManualLeavingSoon(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      setRemoveManualLeavingSoonConfirm(null);
      toast({
        title: 'Flag Removed',
        description: 'Manual leaving soon flag has been removed',
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
  
  const confirmExclude = () => {
    if (excludeConfirm) {
      excludeMutation.mutate(excludeConfirm.id);
    }
  };
  
  const confirmUnexclude = () => {
    if (unexcludeConfirm) {
      unexcludeMutation.mutate(unexcludeConfirm.id);
    }
  };

  const confirmManualLeavingSoon = () => {
    if (manualLeavingSoonConfirm) {
      addManualLeavingSoonMutation.mutate(manualLeavingSoonConfirm.id);
    }
  };

  const confirmRemoveManualLeavingSoon = () => {
    if (removeManualLeavingSoonConfirm) {
      removeManualLeavingSoonMutation.mutate(removeManualLeavingSoonConfirm.id);
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'N/A';
    const date = new Date(dateStr);
    if (date.getFullYear() <= 1970 && date.getMonth() === 0 && date.getDate() === 1) {
      return 'N/A';
    }
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  return (
    <AppLayout>
      <div className="container mx-auto max-w-[1600px] px-4 py-6">
        
        {/* Status Bar - Arr Style */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
            {/* Movies Card */}
            {moviesLoading ? (
              <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-4">
                <Skeleton className="h-4 w-20 mb-2" />
                <Skeleton className="h-8 w-16" />
              </div>
            ) : (
              <div 
                  className="bg-[#1a1a1a] border border-[#333] rounded-md p-4 flex items-center justify-between shadow-sm cursor-pointer hover:border-blue-900/50 transition-colors group"
                  onClick={() => navigate('/library?type=movie')}
              >
                  <div>
                      <p className="text-xs text-gray-400 uppercase tracking-wider font-semibold group-hover:text-blue-400 transition-colors">Movies</p>
                      <p className="text-2xl font-bold text-white mt-1">{movies?.total || 0}</p>
                  </div>
                  <div className="h-10 w-10 bg-[#262626] rounded-full flex items-center justify-center group-hover:bg-blue-900/20 transition-colors">
                      <Film className="h-5 w-5 text-blue-500" />
                  </div>
              </div>
            )}

            {/* TV Shows Card */}
            {showsLoading ? (
              <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-4">
                <Skeleton className="h-4 w-20 mb-2" />
                <Skeleton className="h-8 w-16" />
              </div>
            ) : (
              <div 
                  className="bg-[#1a1a1a] border border-[#333] rounded-md p-4 flex items-center justify-between shadow-sm cursor-pointer hover:border-purple-900/50 transition-colors group"
                  onClick={() => navigate('/library?type=show')}
              >
                  <div>
                      <p className="text-xs text-gray-400 uppercase tracking-wider font-semibold group-hover:text-purple-400 transition-colors">TV Shows</p>
                      <p className="text-2xl font-bold text-white mt-1">{shows?.total || 0}</p>
                  </div>
                  <div className="h-10 w-10 bg-[#262626] rounded-full flex items-center justify-center group-hover:bg-purple-900/20 transition-colors">
                      <Monitor className="h-5 w-5 text-purple-500" />
                  </div>
              </div>
            )}

            {/* Pending Deletion Card */}
            {moviesLoading || showsLoading ? (
              <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-4">
                <Skeleton className="h-4 w-28 mb-2" />
                <Skeleton className="h-8 w-16" />
              </div>
            ) : (
              <div 
                  className="bg-[#1a1a1a] border border-[#333] rounded-md p-4 flex items-center justify-between shadow-sm cursor-pointer hover:border-red-900/50 transition-colors group"
                  onClick={() => navigate('/scheduled-deletions')}
              >
                  <div>
                      <p className="text-xs text-gray-400 uppercase tracking-wider font-semibold group-hover:text-red-400 transition-colors">Pending Deletion</p>
                      <p className="text-2xl font-bold text-white mt-1">{scheduledDeletionsCount}</p>
                  </div>
                  <div className="h-10 w-10 bg-[#262626] rounded-full flex items-center justify-center group-hover:bg-red-900/20 transition-colors">
                      <Clock className="h-5 w-5 text-red-500" />
                  </div>
              </div>
            )}

            {/* Unmatched Card */}
            {unmatchedLoading ? (
              <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-4">
                <Skeleton className="h-4 w-24 mb-2" />
                <Skeleton className="h-8 w-16" />
              </div>
            ) : (
              <div 
                  className="bg-[#1a1a1a] border border-[#333] rounded-md p-4 flex items-center justify-between shadow-sm cursor-pointer hover:border-yellow-900/50 transition-colors group"
                  onClick={() => navigate('/library?unmatched=true')}
              >
                  <div>
                      <p className="text-xs text-gray-400 uppercase tracking-wider font-semibold group-hover:text-yellow-400 transition-colors">Unmatched</p>
                      <p className="text-2xl font-bold text-white mt-1">{unmatched?.total || 0}</p>
                  </div>
                  <div className="h-10 w-10 bg-[#262626] rounded-full flex items-center justify-center group-hover:bg-yellow-900/20 transition-colors">
                      <AlertTriangle className="h-5 w-5 text-yellow-500" />
                  </div>
              </div>
            )}
        </div>

        <div className="grid gap-8">

          
          {/* Leaving Soon Section */}
          {leavingSoon && leavingSoon.items.length > 0 && (
            <div className="space-y-4">
               <div className="flex items-center justify-between">
                 <h2 className="text-xl font-bold text-white flex items-center gap-2">
                    <Clock className="h-5 w-5 text-red-400" />
                    Leaving Soon
                 </h2>
                 {leavingSoon.total > 10 && (
                    <Button variant="ghost" size="sm" onClick={() => navigate('/timeline')} className="text-sm text-blue-400 hover:text-blue-300">
                        View All
                    </Button>
                 )}
               </div>

               <div className="rounded-md border border-[#333] bg-[#1a1a1a] overflow-hidden">
                    <table className="w-full text-sm text-left">
                        <thead className="text-xs text-gray-400 uppercase bg-[#262626] border-b border-[#333]">
                            <tr>
                                <th className="px-6 py-3 font-medium">Title</th>
                                <th className="px-6 py-3 font-medium">Type</th>
                                <th className="px-6 py-3 font-medium">Scheduled Deletion</th>
                                <th className="px-6 py-3 font-medium">Reason</th>
                                <th className="px-6 py-3 font-medium text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-[#333]">
                            {leavingSoon.items.map((item) => (
                                <tr key={item.id} className="hover:bg-[#262626] transition-colors">
                                    <td className="px-6 py-4 font-medium text-white">
                                        <div className="flex items-center gap-3">
                                            <MediaPoster
                                              mediaId={item.id}
                                              mediaType={item.type}
                                              hasPoster={hasPoster(item)}
                                              size="tiny"
                                            />
                                            <div>
                                                <div className="font-medium text-white">{item.title}</div>
                                                <div className="text-xs text-gray-500">{item.year}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4">
                                        <Badge variant="secondary" className="bg-[#333] text-gray-400 border border-[#444] hover:bg-[#444]">
                                            {item.type === 'movie' ? 'Movie' : 'TV Show'}
                                        </Badge>
                                    </td>
                                    <td className="px-6 py-4">
                                        <div className="flex flex-col gap-1">
                                            <div className="flex items-center gap-1.5">
                                                <span className="text-gray-300 font-medium">{formatDate(item.deletion_date)}</span>
                                                {item.manual_leaving_soon && (
                                                    <Badge variant="outline" className="text-[10px] px-1.5 h-4 bg-orange-900/20 text-orange-400 border-orange-900/50">Manual</Badge>
                                                )}
                                            </div>
                                            <Badge variant="outline" className={`w-fit text-[10px] px-1.5 h-5
                                                ${(item.days_until_deletion || 0) <= 3 ? 'bg-red-900/20 text-red-400 border-red-900/50' : 'bg-orange-900/20 text-orange-400 border-orange-900/50'}
                                            `}>
                                                {item.days_until_deletion === 0 ? 'Today' : `${item.days_until_deletion} days left`}
                                            </Badge>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4 text-gray-400 text-xs max-w-[200px] truncate" title={item.deletion_reason}>
                                        {item.deletion_reason || 'Standard retention policy'}
                                    </td>
                                    <td className="px-6 py-4 text-right">
                                        <div className="flex items-center justify-end gap-1">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-8 w-8 p-0 text-gray-400 hover:text-white"
                                                onClick={() => setExcludeConfirm({ id: item.id, title: item.title })}
                                                disabled={excludeMutation.isPending}
                                                title="Protect from deletion"
                                            >
                                                <Shield className="h-4 w-4" />
                                            </Button>
                                            {item.manual_leaving_soon ? (
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-8 w-8 p-0 text-orange-400 hover:text-orange-300"
                                                    onClick={() => setRemoveManualLeavingSoonConfirm({ id: item.id, title: item.title })}
                                                    title="Remove leaving soon flag"
                                                >
                                                    <TimerOff className="h-4 w-4" />
                                                </Button>
                                            ) : (
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-8 w-8 p-0 text-gray-400 hover:text-orange-400"
                                                    onClick={() => setManualLeavingSoonConfirm({ id: item.id, title: item.title })}
                                                    title="Flag as leaving soon"
                                                    disabled={item.excluded}
                                                >
                                                    <Timer className="h-4 w-4" />
                                                </Button>
                                            )}
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
               </div>
            </div>
          )}

          {/* Excluded Items Section */}
          <div className="space-y-4">
             <h2 className="text-xl font-bold text-white flex items-center gap-2">
                <Shield className="h-5 w-5 text-green-400" />
                Protected Content
             </h2>
             
             <div className="rounded-md border border-[#333] bg-[#1a1a1a] overflow-hidden">
                {excluded && excluded.items.length > 0 ? (
                    <table className="w-full text-sm text-left">
                        <thead className="text-xs text-gray-400 uppercase bg-[#262626] border-b border-[#333]">
                            <tr>
                                <th className="px-6 py-3 font-medium">Title</th>
                                <th className="px-6 py-3 font-medium">Type</th>
                                <th className="px-6 py-3 font-medium">Status</th>
                                <th className="px-6 py-3 font-medium text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-[#333]">
                            {excluded.items.map((item) => (
                                <tr key={item.id} className="hover:bg-[#262626] transition-colors">
                                    <td className="px-6 py-4 font-medium text-white">
                                        <div className="flex items-center gap-3">
                                            <MediaPoster
                                              mediaId={item.id}
                                              mediaType={item.type}
                                              hasPoster={hasPoster(item)}
                                              size="tiny"
                                            />
                                            <div>
                                                <div className="font-medium text-white">{item.title}</div>
                                                <div className="text-xs text-gray-500">{item.year}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4">
                                        <Badge variant="secondary" className="bg-[#333] text-gray-400 border border-[#444] hover:bg-[#444]">
                                            {item.type === 'movie' ? 'Movie' : 'TV Show'}
                                        </Badge>
                                    </td>
                                    <td className="px-6 py-4">
                                        <Badge variant="outline" className="bg-green-900/20 text-green-400 border-green-900/50">
                                            Protected
                                        </Badge>
                                    </td>
                                    <td className="px-6 py-4 text-right">
                                        <div className="flex items-center justify-end gap-1">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-8 w-8 p-0 text-gray-400 hover:text-white"
                                                onClick={() => setUnexcludeConfirm({ id: item.id, title: item.title })}
                                                disabled={unexcludeMutation.isPending}
                                                title="Remove protection"
                                            >
                                                <ShieldOff className="h-4 w-4" />
                                            </Button>
                                            {/* Timer disabled for excluded items */}
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="h-8 w-8 p-0 text-gray-400 opacity-40 cursor-not-allowed"
                                                disabled
                                                title="Remove protection first"
                                            >
                                                <Timer className="h-4 w-4" />
                                            </Button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                 ) : (
                    <div className="text-center py-12 text-muted-foreground">
                        <Shield className="h-10 w-10 mx-auto mb-3 opacity-20" />
                        <p className="font-medium text-white">No protected content</p>
                        <p className="text-xs mt-1 text-gray-500 mb-3">Protected items will appear here when you exclude them from deletion</p>
                        <p className="text-xs text-gray-400">Use the shield icon in the Library to protect content</p>
                    </div>
                 )}
             </div>
          </div>

        </div>
      </div>

      {/* Exclude Confirmation Dialog */}
      <Dialog open={!!excludeConfirm} onOpenChange={() => setExcludeConfirm(null)}>
        <DialogContent className="bg-[#1a1a1a] border-[#333]">
          <DialogHeader>
            <DialogTitle className="text-white">Protect from deletion?</DialogTitle>
            <DialogDescription className="text-gray-400">
              Are you sure you want to protect "{excludeConfirm?.title}" from deletion? This item will never be automatically deleted.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setExcludeConfirm(null)} className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333]">
              Cancel
            </Button>
            <Button 
              onClick={confirmExclude}
              disabled={excludeMutation.isPending}
              className="bg-green-600 hover:bg-green-700 text-white"
            >
              {excludeMutation.isPending ? 'Protecting...' : 'Protect'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Unexclude Confirmation Dialog */}
      <Dialog open={!!unexcludeConfirm} onOpenChange={() => setUnexcludeConfirm(null)}>
        <DialogContent className="bg-[#1a1a1a] border-[#333]">
          <DialogHeader>
            <DialogTitle className="text-white">Remove protection?</DialogTitle>
            <DialogDescription className="text-gray-400">
              Are you sure you want to remove protection from "{unexcludeConfirm?.title}"? This item will be subject to automatic deletion rules again.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUnexcludeConfirm(null)} className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333]">
              Cancel
            </Button>
            <Button 
              onClick={confirmUnexclude}
              disabled={unexcludeMutation.isPending}
              variant="destructive"
            >
              {unexcludeMutation.isPending ? 'Removing...' : 'Remove Protection'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Manual Leaving Soon Confirmation Dialog */}
      <Dialog open={!!manualLeavingSoonConfirm} onOpenChange={() => setManualLeavingSoonConfirm(null)}>
        <DialogContent className="bg-[#1a1a1a] border-[#333]">
          <DialogHeader>
            <DialogTitle className="text-white">Flag as Leaving Soon?</DialogTitle>
            <DialogDescription className="text-gray-400">
              Are you sure you want to manually flag "{manualLeavingSoonConfirm?.title}" as leaving soon? It will appear in the leaving soon list and be scheduled for deletion.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setManualLeavingSoonConfirm(null)} className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333]">
              Cancel
            </Button>
            <Button
              onClick={confirmManualLeavingSoon}
              disabled={addManualLeavingSoonMutation.isPending}
              className="bg-orange-600 hover:bg-orange-700 text-white"
            >
              {addManualLeavingSoonMutation.isPending ? 'Flagging...' : 'Flag as Leaving Soon'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove Manual Leaving Soon Confirmation Dialog */}
      <Dialog open={!!removeManualLeavingSoonConfirm} onOpenChange={() => setRemoveManualLeavingSoonConfirm(null)}>
        <DialogContent className="bg-[#1a1a1a] border-[#333]">
          <DialogHeader>
            <DialogTitle className="text-white">Remove Leaving Soon Flag?</DialogTitle>
            <DialogDescription className="text-gray-400">
              Are you sure you want to remove the leaving soon flag from "{removeManualLeavingSoonConfirm?.title}"? It will return to normal rule evaluation.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveManualLeavingSoonConfirm(null)} className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333]">
              Cancel
            </Button>
            <Button
              onClick={confirmRemoveManualLeavingSoon}
              disabled={removeManualLeavingSoonMutation.isPending}
              variant="destructive"
            >
              {removeManualLeavingSoonMutation.isPending ? 'Removing...' : 'Remove Flag'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout>
  );
}

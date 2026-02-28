import { useMemo, useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Clock, Shield, ShieldOff, Calendar, Timer, TimerOff } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import type { MediaItem } from '@/lib/types';
import AppLayout from '@/components/AppLayout';
import { MediaPoster } from '@/components/MediaPoster';
import { hasPoster } from '@/lib/imageUtils';

interface GroupedMedia {
  date: string;
  items: MediaItem[];
}

export default function TimelinePage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [manualLeavingSoonConfirm, setManualLeavingSoonConfirm] = useState<{ id: string; title: string } | null>(null);
  const [removeManualLeavingSoonConfirm, setRemoveManualLeavingSoonConfirm] = useState<{ id: string; title: string } | null>(null);

  const { data: leavingSoon, isLoading } = useQuery({
    queryKey: ['leaving-soon-all'],
    queryFn: () => apiClient.listLeavingSoon({ limit: 1000 }), // Get all leaving soon items
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

  const addManualLeavingSoonMutation = useMutation({
    mutationFn: (id: string) => apiClient.addManualLeavingSoon(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
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
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
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

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    
    // Check for zero time values (Jan 1, 0001 or Jan 1, 1970)
    // Use "N/A" for deletion dates (not scheduled) vs "Never" for watch dates
    if (date.getFullYear() <= 1970 && date.getMonth() === 0 && date.getDate() === 1) {
      return 'N/A';
    }
    
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
    <AppLayout>
      <div className="container mx-auto max-w-[1600px] px-4 py-6">
        <div className="mb-6">
          <h2 className="text-3xl font-bold mb-2 text-white">Deletion Timeline</h2>
          <p className="text-gray-400">
            Media items scheduled for deletion, grouped by date
          </p>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Clock className="h-8 w-8 animate-spin text-gray-500" />
          </div>
        ) : groupedByDate.length === 0 ? (
          <div className="bg-[#1a1a1a] border border-[#333] rounded-md py-12">
            <div className="text-center text-gray-400">
              <Calendar className="h-12 w-12 mx-auto mb-4 opacity-20" />
              <p className="text-lg font-medium mb-1 text-white">No items scheduled for deletion</p>
              <p className="text-sm">All media items are within retention policy</p>
            </div>
          </div>
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
                      <h3 className="text-xl font-semibold flex items-center gap-2 text-white">
                        <Calendar className="h-5 w-5 text-orange-500" />
                        {formatDate(group.date)}
                      </h3>
                      <p className="text-sm text-gray-400 mt-1">
                        {daysUntil === 0 ? 'Today' : daysUntil === 1 ? '1 day away' : `${daysUntil} days away`} • {group.items.length} item{group.items.length !== 1 ? 's' : ''} • {formatFileSize(totalSize)} to be freed
                      </p>
                    </div>
                  </div>

                  {/* Items Card */}
                  <div className="bg-[#1a1a1a] border border-[#333] rounded-md overflow-hidden">
                    <div className="divide-y divide-[#333]">
                      {group.items.map((item) => (
                        <div
                          key={item.id}
                          className="p-4 hover:bg-[#262626] transition-colors"
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-4 flex-1 min-w-0">
                              <MediaPoster
                                mediaId={item.id}
                                mediaType={item.type}
                                hasPoster={hasPoster(item)}
                                size="small"
                              />
                              <div className="flex-1 min-w-0">
                                <h4 className="font-medium truncate text-white">{item.title}</h4>
                                <div className="flex items-center gap-2 mt-1">
                                  {item.year && (
                                    <span className="text-sm text-gray-400">
                                      {item.year}
                                    </span>
                                  )}
                                  {item.file_size && (
                                    <>
                                      <span className="text-gray-500">•</span>
                                      <span className="text-sm text-gray-400">
                                        {formatFileSize(item.file_size)}
                                      </span>
                                    </>
                                  )}
                                </div>
                                {/* Tags */}
                                {item.tags && item.tags.length > 0 && (
                                  <div className="flex flex-wrap gap-1 mt-1">
                                    {item.tags.map((tag) => (
                                      <Badge key={tag} variant="outline" className="text-xs bg-[#262626] text-gray-400 border-[#444]">
                                        {tag}
                                      </Badge>
                                    ))}
                                  </div>
                                )}
                                {/* Jellyfin Match Warning */}
                                {item.jellyfin_match_status && item.jellyfin_match_status !== 'matched' && (
                                  <div className="mt-1">
                                    <Badge 
                                      variant="outline"
                                      className={`text-xs ${
                                        item.jellyfin_match_status === 'metadata_mismatch'
                                          ? 'bg-red-900/20 text-red-400 border-red-900/50'
                                          : 'bg-yellow-900/20 text-yellow-400 border-yellow-900/50'
                                      }`}
                                    >
                                      ⚠️ {item.jellyfin_match_status === 'metadata_mismatch' ? 'Metadata Mismatch' : 'Not in Jellyfin'}
                                    </Badge>
                                  </div>
                                )}
                                {item.deletion_reason && (
                                  <p className="text-xs text-gray-500 mt-1 line-clamp-1">
                                    {item.deletion_reason}
                                  </p>
                                )}
                                {item.manual_leaving_soon && (
                                  <Badge variant="outline" className="text-[10px] px-1.5 h-4 bg-orange-900/20 text-orange-400 border-orange-900/50 mt-1">Manual</Badge>
                                )}
                                {item.is_requested && item.requested_by_username && (
                                  <p className="text-xs text-gray-500 mt-1">
                                    Requested by: {item.requested_by_username}
                                    {item.requested_by_email && ` (${item.requested_by_email})`}
                                  </p>
                                )}
                              </div>
                            </div>
                            <div className="flex items-center gap-2 flex-shrink-0">
                              <Badge variant="outline" className="bg-[#262626] text-gray-300 border-[#444]">
                                {item.type === 'movie' ? 'Movie' : 'TV Show'}
                              </Badge>
                              {item.excluded ? (
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => unexcludeMutation.mutate(item.id)}
                                  disabled={unexcludeMutation.isPending}
                                  className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
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
                                  className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
                                >
                                  <Shield className="h-4 w-4 mr-2" />
                                  Exclude
                                </Button>
                              )}
                              {/* Manual Leaving Soon — disabled for excluded items */}
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
                                  title={item.excluded ? 'Remove protection first' : 'Flag as leaving soon'}
                                  disabled={item.excluded}
                                >
                                  <Timer className="h-4 w-4" />
                                </Button>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

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

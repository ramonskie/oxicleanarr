import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { DeletionCandidate, MediaItem } from '@/lib/types';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Film, Tv, HardDrive, AlertTriangle, Info, Trash2, Clock } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import AppHeader from '@/components/AppHeader';

type MediaType = 'all' | 'movies' | 'shows';
type SortField = 'title' | 'year' | 'days_overdue' | 'file_size';
type SortOrder = 'asc' | 'desc';

const ITEMS_PER_PAGE = 50;

export default function ScheduledDeletionsPage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  
  const [mediaType, setMediaType] = useState<MediaType>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('days_overdue');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [currentPage, setCurrentPage] = useState(1);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  // Fetch movies
  const { data: moviesData, isLoading: moviesLoading } = useQuery({
    queryKey: ['movies'],
    queryFn: () => apiClient.listMovies(),
    enabled: mediaType === 'all' || mediaType === 'movies',
  });

  // Fetch shows
  const { data: showsData, isLoading: showsLoading } = useQuery({
    queryKey: ['shows'],
    queryFn: () => apiClient.listShows(),
    enabled: mediaType === 'all' || mediaType === 'shows',
  });

  // Fetch config
  const { data: configData } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiClient.getConfig(),
  });

  const isLoading = moviesLoading || showsLoading;

  // Execute deletions mutation
  const executeDeletionsMutation = useMutation({
    mutationFn: () => apiClient.executeDeletions(false),
    onSuccess: (data) => {
      toast({
        title: 'Deletions Executed',
        description: `Successfully deleted ${data.deleted_count} items. ${data.failed_count || 0} failed.`,
        variant: data.failed_count && data.failed_count > 0 ? 'destructive' : 'default',
      });
      // Refetch media to update the list
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      setShowDeleteDialog(false);
    },
    onError: (error: Error) => {
      toast({
        title: 'Deletion Failed',
        description: error.message || 'Failed to execute deletions',
        variant: 'destructive',
      });
      setShowDeleteDialog(false);
    },
  });

  const handleExecuteDeletions = () => {
    executeDeletionsMutation.mutate();
  };

  // Get scheduled deletions from media items (overdue items)
  const scheduledDeletions: DeletionCandidate[] = (() => {
    const now = new Date();
    let items: MediaItem[] = [];
    
    if (mediaType === 'all') {
      items = [
        ...(moviesData?.items || []),
        ...(showsData?.items || []),
      ];
    } else if (mediaType === 'movies') {
      items = moviesData?.items || [];
    } else {
      items = showsData?.items || [];
    }

    // Filter for non-excluded items with deletion dates in the past (overdue)
    return items
      .filter(item => {
        if (!item.deletion_date || item.deletion_date === '0001-01-01T00:00:00Z') return false;
        if (item.excluded) return false;
        const deletionDate = new Date(item.deletion_date!);
        return deletionDate < now;
      })
      .map(item => {
        const deletionDate = new Date(item.deletion_date!);
        const daysOverdue = Math.floor((now.getTime() - deletionDate.getTime()) / (1000 * 60 * 60 * 24));
        
        return {
          id: item.id,
          title: item.title,
          year: item.year,
          type: item.type,
          file_size: item.file_size,
          delete_after: item.deletion_date,
          days_overdue: daysOverdue,
          reason: item.deletion_reason || 'No reason specified',
          last_watched: item.last_watched,
          is_requested: item.is_requested,
          requested_by_user_id: item.requested_by_user_id,
          requested_by_username: item.requested_by_username,
          requested_by_email: item.requested_by_email,
          tags: item.tags,
        } as DeletionCandidate;
      });
  })();

  // Check if app is in dry-run mode (default to true for safety)
  const isDryRunMode = configData?.app?.dry_run ?? true;

  // Combine and filter deletion candidates
  const allItems: DeletionCandidate[] = (() => {
    let items = [...scheduledDeletions];
    
    // Apply media type filter
    if (mediaType === 'movies') {
      items = items.filter(item => item.type === 'movie');
    } else if (mediaType === 'shows') {
      items = items.filter(item => item.type === 'tv_show');
    }

    // Apply search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      items = items.filter(item =>
        item.title.toLowerCase().includes(query) ||
        item.year?.toString().includes(query)
      );
    }

    // Apply sorting
    items.sort((a, b) => {
      let aVal: any;
      let bVal: any;

      switch (sortField) {
        case 'title':
          aVal = a.title.toLowerCase();
          bVal = b.title.toLowerCase();
          break;
        case 'year':
          aVal = a.year || 0;
          bVal = b.year || 0;
          break;
        case 'days_overdue':
          aVal = a.days_overdue || 0;
          bVal = b.days_overdue || 0;
          break;
        case 'file_size':
          aVal = a.file_size || 0;
          bVal = b.file_size || 0;
          break;
      }

      if (sortOrder === 'asc') {
        return aVal < bVal ? -1 : aVal > bVal ? 1 : 0;
      } else {
        return aVal > bVal ? -1 : aVal < bVal ? 1 : 0;
      }
    });

    return items;
  })();

  // Count by type
  const movieCount = scheduledDeletions.filter(item => item.type === 'movie').length;
  const showCount = scheduledDeletions.filter(item => item.type === 'tv_show').length;

  // Pagination
  const totalPages = Math.ceil(allItems.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const endIndex = startIndex + ITEMS_PER_PAGE;
  const paginatedItems = allItems.slice(startIndex, endIndex);

  // Reset to page 1 when filters change
  const handleFilterChange = (newType: MediaType) => {
    setMediaType(newType);
    setCurrentPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    setCurrentPage(1);
  };

  const handleSortChange = (field: SortField) => {
    if (sortField === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortOrder('asc');
    }
    setCurrentPage(1);
  };

  const formatDate = (dateStr?: string, context: 'watched' | 'deletion' = 'watched') => {
    if (!dateStr) return context === 'deletion' ? 'N/A' : 'Unknown';
    const date = new Date(dateStr);
    // Check for zero time values (Jan 1, 0001 or Jan 1, 1970)
    if (date.getFullYear() <= 1970 && date.getMonth() === 0 && date.getDate() === 1) {
      return context === 'deletion' ? 'N/A' : 'Unknown';
    }
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes || bytes === 0) return 'Unknown';
    return (bytes / (1024 ** 3)).toFixed(2) + ' GB';
  };

  const getTotalFileSize = () => {
    const total = allItems.reduce((sum, item) => sum + (item.file_size || 0), 0);
    return (total / (1024 ** 3)).toFixed(2) + ' GB';
  };

  const getSortIcon = (field: SortField) => {
    if (sortField !== field) return '↕';
    return sortOrder === 'asc' ? '↑' : '↓';
  };

  const getRuleType = (reason?: string): { type: string; label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } | null => {
    if (!reason) return null;
    
    if (reason.startsWith('tag rule')) {
      return { type: 'tag', label: 'Tag Rule', variant: 'default' };
    } else if (reason.startsWith('user rule')) {
      return { type: 'user', label: 'User Rule', variant: 'secondary' };
    } else if (reason.startsWith('retention period expired') || reason.startsWith('within retention')) {
      return { type: 'standard', label: 'Standard Rule', variant: 'outline' };
    } else if (reason === 'excluded') {
      return { type: 'excluded', label: 'Excluded', variant: 'outline' };
    } else if (reason === 'requested') {
      return { type: 'requested', label: 'Requested', variant: 'outline' };
    }
    
    return null;
  };

  return (
    <div className="min-h-screen bg-background">
      <AppHeader />

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        {/* Page Header */}
        <div className="mb-6">
          <h2 className="text-3xl font-bold mb-2 flex items-center gap-2">
            <AlertTriangle className="h-8 w-8 text-yellow-600" />
            Scheduled Deletions
          </h2>
          <p className="text-muted-foreground">
            Items that would be deleted when dry-run mode is disabled
          </p>
        </div>

        {isLoading ? (
          <Card>
            <CardContent className="p-8 text-center">
              <Clock className="h-12 w-12 mx-auto mb-4 text-muted-foreground animate-spin" />
              <p className="text-muted-foreground">Loading scheduled deletions...</p>
            </CardContent>
          </Card>
        ) : scheduledDeletions.length === 0 ? (
          <Card>
            <CardContent className="p-8 text-center">
              <Info className="h-12 w-12 mx-auto mb-4 text-muted-foreground" />
              <p className="text-lg font-medium mb-2">No scheduled deletions</p>
              <p className="text-muted-foreground">
                No items are currently scheduled for deletion
              </p>
            </CardContent>
          </Card>
        ) : (
          <>
            {/* Stats Summary */}
            <div className="mb-6 p-4 bg-yellow-500/10 border border-yellow-500/50 rounded-lg">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-6">
                  <div>
                    <p className="text-sm text-muted-foreground">Total Items</p>
                    <p className="text-2xl font-bold">{scheduledDeletions.length}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Total Space</p>
                    <p className="text-2xl font-bold">{getTotalFileSize()}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Movies</p>
                    <p className="text-2xl font-bold">{movieCount}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">TV Shows</p>
                    <p className="text-2xl font-bold">{showCount}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  {isDryRunMode ? (
                    <>
                      <Badge variant="outline" className="bg-yellow-500/10 text-yellow-700 border-yellow-500/50">
                        Dry Run Active
                      </Badge>
                      <div className="flex flex-col items-end">
                        <Button
                          variant="destructive"
                          onClick={() => setShowDeleteDialog(true)}
                          disabled={true}
                          title="Deletions are disabled in dry-run mode. Change app.dry_run to false in config to enable."
                        >
                          <Trash2 className="h-4 w-4 mr-2" />
                          Execute Deletions
                        </Button>
                        <p className="text-xs text-muted-foreground mt-1">
                          Set <code className="bg-muted px-1 py-0.5 rounded">app.dry_run: false</code> in config to enable
                        </p>
                      </div>
                    </>
                  ) : (
                    <>
                      <Badge variant="outline" className="bg-green-500/10 text-green-700 border-green-500/50">
                        Deletions Enabled
                      </Badge>
                      <Button
                        variant="destructive"
                        onClick={() => setShowDeleteDialog(true)}
                        disabled={executeDeletionsMutation.isPending}
                      >
                        <Trash2 className="h-4 w-4 mr-2" />
                        Execute Deletions
                      </Button>
                    </>
                  )}
                </div>
              </div>
            </div>

            {/* Filters and Search */}
            <div className="mb-6 space-y-4">
              <div className="flex gap-4 items-center flex-wrap">
                {/* Media Type Filter */}
                <div className="flex gap-2">
                  <Button
                    variant={mediaType === 'all' ? 'default' : 'outline'}
                    onClick={() => handleFilterChange('all')}
                  >
                    All ({scheduledDeletions.length})
                  </Button>
                  <Button
                    variant={mediaType === 'movies' ? 'default' : 'outline'}
                    onClick={() => handleFilterChange('movies')}
                  >
                    Movies ({movieCount})
                  </Button>
                  <Button
                    variant={mediaType === 'shows' ? 'default' : 'outline'}
                    onClick={() => handleFilterChange('shows')}
                  >
                    TV Shows ({showCount})
                  </Button>
                </div>

                {/* Search */}
                <div className="flex-1 min-w-[300px]">
                  <Input
                    placeholder="Search by title or year..."
                    value={searchQuery}
                    onChange={(e) => handleSearchChange(e.target.value)}
                  />
                </div>
              </div>

              {/* Sort Controls */}
              <div className="flex gap-2 items-center text-sm">
                <span className="text-gray-600">Sort by:</span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleSortChange('title')}
                >
                  Title {getSortIcon('title')}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleSortChange('year')}
                >
                  Year {getSortIcon('year')}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleSortChange('days_overdue')}
                >
                  Days Overdue {getSortIcon('days_overdue')}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleSortChange('file_size')}
                >
                  File Size {getSortIcon('file_size')}
                </Button>
              </div>

              {/* Results count */}
              <div className="text-sm text-gray-600">
                Showing {startIndex + 1}-{Math.min(endIndex, allItems.length)} of {allItems.length} items
              </div>
            </div>

            {/* Deletion Candidates */}
            <div className="space-y-3 mb-6">
              {paginatedItems.map((item) => (
                <Card key={item.id} className="p-4 hover:shadow-md transition-shadow border-yellow-500/30">
                  <div className="flex justify-between items-start gap-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        {item.type === 'movie' ? (
                          <Film className="h-5 w-5 text-muted-foreground" />
                        ) : (
                          <Tv className="h-5 w-5 text-muted-foreground" />
                        )}
                        <h3 className="text-lg font-semibold text-gray-900">
                          {item.title}
                          {item.year && (
                            <span className="text-gray-500 font-normal ml-2">
                              ({item.year})
                            </span>
                          )}
                        </h3>
                        <Badge variant={item.type === 'movie' ? 'movie' : 'show'}>
                          {item.type === 'movie' ? 'Movie' : 'TV Show'}
                        </Badge>
                        {getRuleType(item.reason) && (
                          <Badge variant={getRuleType(item.reason)!.variant} className="text-xs">
                            {getRuleType(item.reason)!.label}
                          </Badge>
                        )}
                      </div>

                      {/* Tags */}
                      {item.tags && item.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1 mb-2">
                          {item.tags.map((tag) => (
                            <Badge key={tag} variant="secondary" className="text-xs">
                              {tag}
                            </Badge>
                          ))}
                        </div>
                      )}

                      {/* Jellyfin Match Warning */}
                      {item.jellyfin_match_status && item.jellyfin_match_status !== 'matched' && (
                        <div className="mb-2">
                          <Badge 
                            variant={item.jellyfin_match_status === 'metadata_mismatch' ? 'destructive' : 'outline'}
                            className="text-xs"
                          >
                            ⚠️ {item.jellyfin_match_status === 'metadata_mismatch' ? 'Jellyfin Metadata Mismatch' : 'Not in Jellyfin'}
                          </Badge>
                          {item.jellyfin_mismatch_info && (
                            <span className="ml-2 text-xs text-gray-600">
                              {item.jellyfin_mismatch_info}
                            </span>
                          )}
                        </div>
                      )}

                      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                        <div>
                          <span className="text-gray-500">File Size:</span>
                          <div className="font-medium flex items-center gap-1">
                            <HardDrive className="h-3 w-3" />
                            {formatFileSize(item.file_size)}
                          </div>
                        </div>
                        <div>
                          <span className="text-gray-500">Days Overdue:</span>
                          <div className="font-medium text-red-600">
                            {item.days_overdue} days
                          </div>
                        </div>
                        <div>
                          <span className="text-gray-500">Delete After:</span>
                          <div className="font-medium">{formatDate(item.delete_after, 'deletion')}</div>
                        </div>
                        <div>
                          <span className="text-gray-500">Last Watched:</span>
                          <div className="font-medium">{formatDate(item.last_watched, 'watched')}</div>
                        </div>
                      </div>

                      {item.reason && (
                        <div className="mt-2 text-sm text-gray-600">
                          <span className="font-medium">Reason:</span> {item.reason}
                        </div>
                      )}

                      {item.is_requested && item.requested_by_username && (
                        <div className="mt-2 text-sm text-gray-600">
                          <span className="font-medium">Requested by:</span> {item.requested_by_username}
                          {item.requested_by_email && ` (${item.requested_by_email})`}
                        </div>
                      )}
                    </div>

                    <div className="flex-shrink-0">
                      <Badge variant="destructive" className="whitespace-nowrap hover:bg-destructive">
                        Scheduled for Deletion
                      </Badge>
                    </div>
                  </div>
                </Card>
              ))}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex justify-center items-center gap-2">
                <Button
                  variant="outline"
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                >
                  Previous
                </Button>
                
                {Array.from({ length: totalPages }, (_, i) => i + 1)
                  .filter(page => {
                    // Show first, last, current, and 2 pages around current
                    return (
                      page === 1 ||
                      page === totalPages ||
                      Math.abs(page - currentPage) <= 2
                    );
                  })
                  .map((page, idx, arr) => (
                    <span key={page}>
                      {idx > 0 && arr[idx - 1] !== page - 1 && (
                        <span className="px-2 text-gray-400">...</span>
                      )}
                      <Button
                        variant={currentPage === page ? 'default' : 'outline'}
                        onClick={() => setCurrentPage(page)}
                      >
                        {page}
                      </Button>
                    </span>
                  ))}

                <Button
                  variant="outline"
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                >
                  Next
                </Button>
              </div>
            )}

            {/* Empty state */}
            {paginatedItems.length === 0 && allItems.length === 0 && (
              <div className="text-center py-12">
                <p className="text-gray-500 text-lg">
                  {searchQuery
                    ? 'No scheduled deletions match your search.'
                    : 'No scheduled deletions found.'}
                </p>
              </div>
            )}
          </>
        )}
      </main>

      {/* Confirmation Dialog */}
      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Execute Deletions</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete {scheduledDeletions.length} items 
              ({getTotalFileSize()} total)? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteDialog(false)}>
              Cancel
            </Button>
            <Button 
              variant="destructive" 
              onClick={handleExecuteDeletions}
              disabled={executeDeletionsMutation.isPending}
            >
              {executeDeletionsMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

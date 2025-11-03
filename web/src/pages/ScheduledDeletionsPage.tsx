import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import type { DeletionCandidate } from '@/lib/types';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Clock, LogOut, Film, Tv, HardDrive, AlertTriangle, Info } from 'lucide-react';

type MediaType = 'all' | 'movies' | 'shows';
type SortField = 'title' | 'year' | 'days_overdue' | 'file_size';
type SortOrder = 'asc' | 'desc';

const ITEMS_PER_PAGE = 50;

export default function ScheduledDeletionsPage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  
  const [mediaType, setMediaType] = useState<MediaType>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('days_overdue');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [currentPage, setCurrentPage] = useState(1);

  // Fetch latest job to get scheduled deletions
  const { data: jobsData, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: () => apiClient.listJobs(),
  });

  // Sync status
  const { data: syncStatus } = useQuery({
    queryKey: ['syncStatus'],
    queryFn: () => apiClient.getSyncStatus(),
    refetchInterval: 5000,
  });

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  // Get scheduled deletions from latest job
  const scheduledDeletions: DeletionCandidate[] = (() => {
    if (!jobsData?.jobs || jobsData.jobs.length === 0) return [];
    const latestJob = jobsData.jobs[0];
    return latestJob.summary?.would_delete || [];
  })();

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
              <Button variant="ghost" onClick={() => navigate('/timeline')}>
                Timeline
              </Button>
              <Button variant="ghost" onClick={() => navigate('/library')}>
                Library
              </Button>
              <Button variant="ghost" className="bg-accent">
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
                <Badge variant="outline" className="bg-yellow-500/10 text-yellow-700 border-yellow-500/50">
                  Dry Run Preview
                </Badge>
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
                      </div>

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
                      <Badge variant="destructive" className="whitespace-nowrap">
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
    </div>
  );
}

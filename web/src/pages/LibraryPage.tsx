import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { MediaItem } from '@/lib/types';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { useAuthStore } from '@/store/auth';
import { useNavigate } from 'react-router-dom';
import { Clock, LogOut, Shield, ShieldOff } from 'lucide-react';

type MediaType = 'all' | 'movies' | 'shows';
type SortField = 'title' | 'year' | 'last_watched' | 'deletion_date';
type SortOrder = 'asc' | 'desc';

const ITEMS_PER_PAGE = 50;

export default function LibraryPage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  
  const [mediaType, setMediaType] = useState<MediaType>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('title');
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
  const [currentPage, setCurrentPage] = useState(1);

  // Fetch movies
  const { data: moviesData } = useQuery({
    queryKey: ['movies'],
    queryFn: () => apiClient.listMovies(),
    enabled: mediaType === 'all' || mediaType === 'movies',
  });

  // Fetch shows
  const { data: showsData } = useQuery({
    queryKey: ['shows'],
    queryFn: () => apiClient.listShows(),
    enabled: mediaType === 'all' || mediaType === 'shows',
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

  // Combine and filter media items
  const allItems: MediaItem[] = (() => {
    let items: MediaItem[] = [];
    
    if (mediaType === 'all') {
      items = [
        ...(moviesData?.items || []),
        ...(showsData?.items || []),
      ];
    } else if (mediaType === 'movies') {
      items = moviesData?.items || [];
    } else if (mediaType === 'shows') {
      items = showsData?.items || [];
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
        case 'last_watched':
          aVal = a.last_watched ? new Date(a.last_watched).getTime() : 0;
          bVal = b.last_watched ? new Date(b.last_watched).getTime() : 0;
          break;
        case 'deletion_date':
          aVal = a.deletion_date ? new Date(a.deletion_date).getTime() : Infinity;
          bVal = b.deletion_date ? new Date(b.deletion_date).getTime() : Infinity;
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

  const handleExclude = async (id: string) => {
    try {
      await apiClient.addExclusion(id);
      // Refetch to update UI
      window.location.reload();
    } catch (error) {
      console.error('Failed to exclude media:', error);
    }
  };

  const handleUnexclude = async (id: string) => {
    try {
      await apiClient.removeExclusion(id);
      // Refetch to update UI
      window.location.reload();
    } catch (error) {
      console.error('Failed to unexclude media:', error);
    }
  };

  const formatDate = (dateStr?: string, context: 'watched' | 'deletion' = 'watched') => {
    if (!dateStr) return context === 'deletion' ? 'N/A' : 'Never';
    const date = new Date(dateStr);
    // Check for zero time values (Jan 1, 0001 or Jan 1, 1970)
    if (date.getFullYear() <= 1970 && date.getMonth() === 0 && date.getDate() === 1) {
      return context === 'deletion' ? 'N/A' : 'Never';
    }
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return 'Unknown';
    return (bytes / (1024 ** 3)).toFixed(2) + ' GB';
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
              <Button variant="ghost" className="bg-accent">
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
        {/* Filters and Search */}
        <div className="mb-6 space-y-4">
          <div className="flex gap-4 items-center flex-wrap">
            {/* Media Type Filter */}
            <div className="flex gap-2">
              <Button
                variant={mediaType === 'all' ? 'default' : 'outline'}
                onClick={() => handleFilterChange('all')}
              >
                All ({(moviesData?.total || 0) + (showsData?.total || 0)})
              </Button>
              <Button
                variant={mediaType === 'movies' ? 'default' : 'outline'}
                onClick={() => handleFilterChange('movies')}
              >
                Movies ({moviesData?.total || 0})
              </Button>
              <Button
                variant={mediaType === 'shows' ? 'default' : 'outline'}
                onClick={() => handleFilterChange('shows')}
              >
                TV Shows ({showsData?.total || 0})
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
              onClick={() => handleSortChange('last_watched')}
            >
              Last Watched {getSortIcon('last_watched')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleSortChange('deletion_date')}
            >
              Deletion Date {getSortIcon('deletion_date')}
            </Button>
          </div>

          {/* Results count */}
          <div className="text-sm text-gray-600">
            Showing {startIndex + 1}-{Math.min(endIndex, allItems.length)} of {allItems.length} items
          </div>
        </div>

        {/* Media Items */}
        <div className="space-y-3 mb-6">
          {paginatedItems.map((item) => (
            <Card key={item.id} className="p-4 hover:shadow-md transition-shadow">
              <div className="flex justify-between items-start gap-4">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-2">
                    <h3 className="text-lg font-semibold text-gray-900">
                      {item.title}
                      {item.year && (
                        <span className="text-gray-500 font-normal ml-2">
                          ({item.year})
                        </span>
                      )}
                    </h3>
                    {item.excluded && (
                      <Badge variant="outline">Excluded</Badge>
                    )}
                  </div>

                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                      <span className="text-gray-500">Last Watched:</span>
                      <div className="font-medium">{formatDate(item.last_watched)}</div>
                    </div>
                    <div>
                      <span className="text-gray-500">File Size:</span>
                      <div className="font-medium">{formatFileSize(item.file_size)}</div>
                    </div>
                    <div>
                      <span className="text-gray-500">Deletion Date:</span>
                      <div className="font-medium">
                        {item.deletion_date ? formatDate(item.deletion_date, 'deletion') : 'Not scheduled'}
                      </div>
                    </div>
                    <div>
                      <span className="text-gray-500">Days Until Deletion:</span>
                      <div className="font-medium">
                        {item.days_until_deletion !== undefined
                          ? `${item.days_until_deletion} days`
                          : 'N/A'}
                      </div>
                    </div>
                  </div>

                  {item.is_requested && item.requested_by_username && (
                    <div className="mt-2 text-sm text-gray-600">
                      <span className="font-medium">Requested by:</span> {item.requested_by_username}
                      {item.requested_by_email && ` (${item.requested_by_email})`}
                    </div>
                  )}

                  {item.deletion_reason && (
                    <div className="mt-2 text-sm text-gray-600">
                      <span className="font-medium">Reason:</span> {item.deletion_reason}
                    </div>
                  )}
                </div>

                <div className="flex items-center gap-3 flex-shrink-0">
                  <Badge variant={item.type === 'movie' ? 'movie' : 'show'}>
                    {item.type === 'movie' ? 'Movie' : 'TV Show'}
                  </Badge>
                  {item.excluded ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleUnexclude(item.id)}
                    >
                      <ShieldOff className="h-4 w-4 mr-2" />
                      Unexclude
                    </Button>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleExclude(item.id)}
                    >
                      <Shield className="h-4 w-4 mr-2" />
                      Exclude
                    </Button>
                  )}
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
        {paginatedItems.length === 0 && (
          <div className="text-center py-12">
            <p className="text-gray-500 text-lg">
              {searchQuery
                ? 'No media items match your search.'
                : 'No media items found.'}
            </p>
          </div>
        )}
      </main>
    </div>
  );
}

import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { MediaItem } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Shield, ShieldOff, Search, Monitor, Film, Filter, User } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import AppLayout from '@/components/AppLayout';

type MediaType = 'all' | 'movies' | 'shows';
type SortField = 'title' | 'year' | 'last_watched' | 'deletion_date';
type SortOrder = 'asc' | 'desc';

const ITEMS_PER_PAGE = 50;

export default function LibraryPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  
  const [mediaType, setMediaType] = useState<MediaType>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('title');
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
  const [currentPage, setCurrentPage] = useState(1);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [showUnmatchedOnly, setShowUnmatchedOnly] = useState(false);
  const [showFilters, setShowFilters] = useState(true);
  
  // Confirmation dialogs
  const [excludeConfirm, setExcludeConfirm] = useState<{ id: string; title: string } | null>(null);
  const [unexcludeConfirm, setUnexcludeConfirm] = useState<{ id: string; title: string } | null>(null);

  // Read URL parameters on mount
  useEffect(() => {
    const unmatchedParam = searchParams.get('unmatched');
    if (unmatchedParam === 'true') {
      setShowUnmatchedOnly(true);
    }
    
    const typeParam = searchParams.get('type');
    if (typeParam === 'movie') {
      setMediaType('movies');
    } else if (typeParam === 'show') {
      setMediaType('shows');
    }
  }, [searchParams]);

  // TODO: Implement server-side pagination, filtering, and sorting
  // Current implementation fetches all data and filters/sorts client-side.
  // For very large libraries (1000+ items), consider:
  // 1. Adding backend API support for filters (tags, search, type)
  // 2. Adding backend API support for sorting (by field + order)
  // 3. Fetching only one page at a time with limit/offset
  // 4. Using query keys that include filter/sort params for proper caching
  
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

  // Get all unique tags
  const allTags: string[] = (() => {
    const tagSet = new Set<string>();
    const allMedia = [
      ...(moviesData?.items || []),
      ...(showsData?.items || []),
    ];
    allMedia.forEach(item => {
      if (item.tags) {
        item.tags.forEach(tag => tagSet.add(tag));
      }
    });
    return Array.from(tagSet).sort();
  })();

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

    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      items = items.filter(item =>
        item.title.toLowerCase().includes(query) ||
        item.year?.toString().includes(query)
      );
    }

    if (selectedTags.length > 0) {
      items = items.filter(item => {
        return item.tags?.some(tag => selectedTags.includes(tag));
      });
    }

    if (showUnmatchedOnly) {
      items = items.filter(item => 
        item.jellyfin_match_status === 'metadata_mismatch' || 
        item.jellyfin_match_status === 'not_found'
      );
    }

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

  const totalPages = Math.ceil(allItems.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const endIndex = startIndex + ITEMS_PER_PAGE;
  const paginatedItems = allItems.slice(startIndex, endIndex);

  const handleFilterChange = (newType: MediaType) => {
    setMediaType(newType);
    setCurrentPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    setCurrentPage(1);
  };

  const handleTagToggle = (tag: string) => {
    setSelectedTags(prev => {
      if (prev.includes(tag)) {
        return prev.filter(t => t !== tag);
      } else {
        return [...prev, tag];
      }
    });
    setCurrentPage(1);
  };

  const handleClearTags = () => {
    setSelectedTags([]);
    setCurrentPage(1);
  };

  const handleToggleUnmatched = () => {
    const newValue = !showUnmatchedOnly;
    setShowUnmatchedOnly(newValue);
    setCurrentPage(1);
    
    if (newValue) {
      setSearchParams({ unmatched: 'true' });
    } else {
      setSearchParams({});
    }
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

  const excludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.addExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['excluded'] });
      toast({
        title: 'Excluded',
        description: 'Item has been added to the exclusion list',
      });
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to exclude media item',
        variant: 'destructive',
      });
    },
  });

  const unexcludeMutation = useMutation({
    mutationFn: (id: string) => apiClient.removeExclusion(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['excluded'] });
      toast({
        title: 'Unexcluded',
        description: 'Item has been removed from the exclusion list',
      });
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to unexclude media item',
        variant: 'destructive',
      });
    },
  });

  const handleExclude = (id: string, title: string) => {
    setExcludeConfirm({ id, title });
  };

  const handleUnexclude = (id: string, title: string) => {
    setUnexcludeConfirm({ id, title });
  };
  
  const confirmExclude = () => {
    if (excludeConfirm) {
      excludeMutation.mutate(excludeConfirm.id);
      setExcludeConfirm(null);
    }
  };
  
  const confirmUnexclude = () => {
    if (unexcludeConfirm) {
      unexcludeMutation.mutate(unexcludeConfirm.id);
      setUnexcludeConfirm(null);
    }
  };

  const formatDate = (dateStr?: string, context: 'watched' | 'deletion' = 'watched') => {
    if (!dateStr) return context === 'deletion' ? 'N/A' : 'Never';
    const date = new Date(dateStr);
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
    if (!bytes) return '0 B';
    return (bytes / (1024 ** 3)).toFixed(2) + ' GB';
  };

  const getSortIcon = (field: SortField) => {
    if (sortField !== field) return null;
    return sortOrder === 'asc' ? '↑' : '↓';
  };

  return (
    <AppLayout>
      <div className="container mx-auto max-w-[1600px] px-4 py-6">
        {/* Header Area */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
          <h1 className="text-3xl font-bold text-white tracking-tight">Library</h1>
          
          <div className="flex items-center gap-2">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-gray-500" />
              <Input
                placeholder="Search library..."
                value={searchQuery}
                onChange={(e) => handleSearchChange(e.target.value)}
                className="pl-9 w-[250px] bg-[#1a1a1a] border-[#333] focus:border-primary"
              />
            </div>
            
             <Button
                variant={showFilters ? 'secondary' : 'outline'}
                size="icon"
                onClick={() => setShowFilters(!showFilters)}
                className={showFilters ? 'bg-primary text-primary-foreground hover:bg-primary/90' : ''}
                title="Filter by Tags"
            >
                <Filter className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Filters Toolbar */}
        <div className="flex flex-col gap-4 mb-6">
             <div className="flex flex-wrap items-center gap-4 p-1 bg-[#1a1a1a] rounded-lg border border-[#333] w-fit">
              <Button
                variant={mediaType === 'all' ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => handleFilterChange('all')}
                className={mediaType === 'all' ? 'bg-[#333] text-white' : 'text-gray-400 hover:text-white'}
              >
                All Media
              </Button>
              <Button
                variant={mediaType === 'movies' ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => handleFilterChange('movies')}
                className={mediaType === 'movies' ? 'bg-[#333] text-white' : 'text-gray-400 hover:text-white'}
              >
                <Film className="h-4 w-4 mr-2" />
                Movies
              </Button>
              <Button
                variant={mediaType === 'shows' ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => handleFilterChange('shows')}
                className={mediaType === 'shows' ? 'bg-[#333] text-white' : 'text-gray-400 hover:text-white'}
              >
                <Monitor className="h-4 w-4 mr-2" />
                TV Shows
              </Button>
            </div>
            
            {/* Tag Filters (Conditional) */}
            {showFilters && allTags.length > 0 && (
                <div className="p-4 bg-[#1a1a1a] border border-[#333] rounded-lg animate-in slide-in-from-top-2">
                    <div className="flex items-center justify-between mb-2">
                         <h3 className="text-sm font-medium text-gray-400">Filter by Tags</h3>
                         {selectedTags.length > 0 && (
                            <Button variant="ghost" size="sm" onClick={handleClearTags} className="h-auto p-0 text-xs text-primary hover:text-primary/80">
                                Clear all
                            </Button>
                         )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                        {allTags.map(tag => (
                            <Badge
                                key={tag}
                                variant="outline"
                                className={`cursor-pointer transition-colors ${
                                    selectedTags.includes(tag) 
                                    ? 'bg-primary/20 text-primary border-primary' 
                                    : 'text-gray-400 border-[#444] hover:border-gray-300'
                                }`}
                                onClick={() => handleTagToggle(tag)}
                            >
                                {tag}
                            </Badge>
                        ))}
                    </div>
                     <div className="mt-4 pt-4 border-t border-[#333] flex items-center gap-2">
                        <Button
                          variant={showUnmatchedOnly ? 'destructive' : 'outline'}
                          size="sm"
                          onClick={handleToggleUnmatched}
                          className="text-xs h-8"
                        >
                          {showUnmatchedOnly ? '✓ ' : ''}Show Unmatched Only
                        </Button>
                     </div>
                </div>
            )}
        </div>

        {/* Data Table - Arr Style */}
        <div className="rounded-md border border-[#333] bg-[#1a1a1a] overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-gray-400 uppercase bg-[#262626] border-b border-[#333]">
                <tr>
                  <th className="px-6 py-3 font-medium cursor-pointer hover:text-white transition-colors" onClick={() => handleSortChange('title')}>
                    Title {getSortIcon('title')}
                  </th>
                  <th className="px-6 py-3 font-medium">Type</th>
                  <th className="px-6 py-3 font-medium cursor-pointer hover:text-white transition-colors" onClick={() => handleSortChange('year')}>
                    Year {getSortIcon('year')}
                  </th>
                  <th className="px-6 py-3 font-medium">Requested By</th>
                  <th className="px-6 py-3 font-medium">Quality/Size</th>
                  <th className="px-6 py-3 font-medium cursor-pointer hover:text-white transition-colors" onClick={() => handleSortChange('last_watched')}>
                    Last Watched {getSortIcon('last_watched')}
                  </th>
                  <th className="px-6 py-3 font-medium cursor-pointer hover:text-white transition-colors" onClick={() => handleSortChange('deletion_date')}>
                    Deletion {getSortIcon('deletion_date')}
                  </th>
                  <th className="px-6 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#333]">
                {paginatedItems.map((item) => (
                  <tr key={item.id} className="hover:bg-[#262626] transition-colors group">
                    <td className="px-6 py-4 font-medium text-white">
                      <div className="flex items-center gap-3">
                         {/* Placeholder Poster */}
                         <div className="w-8 h-12 bg-[#333] rounded flex items-center justify-center flex-shrink-0">
                            {item.type === 'movie' ? <Film className="h-4 w-4 text-gray-500" /> : <Monitor className="h-4 w-4 text-gray-500" />}
                         </div>
                         <div>
                            <div className="flex items-center gap-2">
                                <span className="line-clamp-1">{item.title}</span>
                                {item.jellyfin_match_status === 'metadata_mismatch' && (
                                   <Badge variant="destructive" className="text-[10px] h-5 px-1.5">Mismatch</Badge>
                                )}
                            </div>
                             {/* Tags displayed small under title */}
                             {item.tags && item.tags.length > 0 && (
                                <div className="flex gap-1 mt-1">
                                    {item.tags.slice(0, 3).map(tag => (
                                        <span key={tag} className="px-1.5 py-0.5 rounded text-[10px] bg-[#333] text-gray-400 border border-[#444]">
                                            {tag}
                                        </span>
                                    ))}
                                    {item.tags.length > 3 && (
                                        <span className="px-1.5 py-0.5 rounded text-[10px] bg-[#333] text-gray-400 border border-[#444]">
                                            +{item.tags.length - 3}
                                        </span>
                                    )}
                                </div>
                             )}
                         </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                        <Badge variant="secondary" className="text-[10px] h-5 px-1.5 bg-[#333] text-gray-400 border border-[#444]">
                            {item.type === 'movie' ? 'Movie' : 'TV'}
                        </Badge>
                    </td>
                    <td className="px-6 py-4 text-gray-400">{item.year || '-'}</td>
                    <td className="px-6 py-4">
                        {item.requested_by_username || item.requested_by_email ? (
                            <div className="flex items-center gap-2">
                                <User className="h-3 w-3 text-gray-500" />
                                <span className="text-gray-300">{item.requested_by_username || item.requested_by_email}</span>
                            </div>
                        ) : (
                            <span className="text-gray-600 text-xs italic">Unknown</span>
                        )}
                    </td>
                    <td className="px-6 py-4">
                        <div className="flex flex-col">
                            <span className="text-gray-300">{formatFileSize(item.file_size)}</span>
                            {/* Mock Quality Badge - In real app, this would come from media info */}
                            <span className="text-xs text-gray-500">WEBDL-1080p</span>
                        </div>
                    </td>
                    <td className="px-6 py-4 text-gray-400">{formatDate(item.last_watched)}</td>
                    <td className="px-6 py-4">
                      {item.excluded ? (
                        <div className="flex items-center gap-2">
                            <Badge variant="outline" className="bg-green-900/20 text-green-400 border-green-900/50 hover:bg-green-900/30">
                              Protected
                            </Badge>
                        </div>
                      ) : item.deletion_date && item.days_until_deletion !== undefined ? (
                         <div className="flex flex-col gap-1">
                             <div className="flex items-center gap-2">
                                 <span className="text-sm text-gray-300">{formatDate(item.deletion_date, 'deletion')}</span>
                             </div>
                             <Badge variant="outline" className={`w-fit
                                ${item.days_until_deletion <= 7 ? 'bg-red-900/20 text-red-400 border-red-900/50' : 'bg-blue-900/20 text-blue-400 border-blue-900/50'}
                             `}>
                                {item.days_until_deletion === 0 ? 'Today' : `${item.days_until_deletion} days left`}
                             </Badge>
                         </div>
                      ) : (
                        <span className="text-gray-500 text-sm">Not Scheduled</span>
                      )}
                    </td>
                     <td className="px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-2 opacity-100">
                        {item.excluded ? (
                            <Button 
                                variant="ghost" 
                                size="sm" 
                                className="h-8 w-8 p-0 text-gray-400 hover:text-white"
                                onClick={() => handleUnexclude(item.id, item.title)}
                                title="Remove protection"
                            >
                                <ShieldOff className="h-4 w-4" />
                            </Button>
                        ) : (
                            <Button 
                                variant="ghost" 
                                size="sm" 
                                className="h-8 w-8 p-0 text-gray-400 hover:text-white"
                                onClick={() => handleExclude(item.id, item.title)}
                                title="Protect from deletion"
                            >
                                <Shield className="h-4 w-4" />
                            </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          
          {/* Pagination Footer */}
          {totalPages > 1 && (
            <div className="px-6 py-4 border-t border-[#333] flex items-center justify-between bg-[#262626]">
                <div className="text-sm text-gray-500">
                    Showing {startIndex + 1} to {Math.min(endIndex, allItems.length)} of {allItems.length} entries
                </div>
                <div className="flex gap-2">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                        disabled={currentPage === 1}
                        className="h-8 text-xs bg-[#1a1a1a] border-[#333] hover:bg-[#333]"
                    >
                        Previous
                    </Button>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                        disabled={currentPage === totalPages}
                        className="h-8 text-xs bg-[#1a1a1a] border-[#333] hover:bg-[#333]"
                    >
                        Next
                    </Button>
                </div>
            </div>
          )}

           {/* Empty state */}
           {paginatedItems.length === 0 && (
            <div className="text-center py-16">
               <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-[#262626] mb-4">
                  <Film className="h-8 w-8 text-gray-600" />
               </div>
               <h3 className="text-lg font-medium text-white mb-1">No media found</h3>
               <p className="text-gray-500 max-w-sm mx-auto mb-4">
                 {searchQuery 
                   ? `No results for "${searchQuery}". Try adjusting your search or filters.`
                   : showUnmatchedOnly
                   ? "No unmatched items found. All media is properly matched with Jellyfin!"
                   : selectedTags.length > 0
                   ? "No media matches the selected tags. Try clearing some filters."
                   : "Your library is empty or hasn't been synced yet."}
               </p>
               {!searchQuery && !showUnmatchedOnly && selectedTags.length === 0 && (
                 <div className="flex flex-col items-center gap-2">
                   <p className="text-sm text-gray-400">Make sure OxiCleanarr is configured with:</p>
                   <ul className="text-sm text-gray-400 list-disc list-inside">
                     <li>Radarr for movies</li>
                     <li>Sonarr for TV shows</li>
                   </ul>
                   <Button 
                     variant="outline" 
                     size="sm" 
                     onClick={() => navigate('/settings')}
                     className="mt-4 bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
                   >
                     Go to Settings
                   </Button>
                 </div>
               )}
               {(searchQuery || showUnmatchedOnly || selectedTags.length > 0) && (
                 <Button 
                   variant="outline" 
                   size="sm" 
                   onClick={() => {
                     setSearchQuery('');
                     setShowUnmatchedOnly(false);
                     setSelectedTags([]);
                     setSearchParams({});
                   }}
                   className="mt-4 bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
                 >
                   Clear all filters
                 </Button>
               )}
            </div>
           )}
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
     </AppLayout>
  );
}

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { apiClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import type { Job, DeletionCandidate } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  CheckCircle,
  XCircle,
  Clock,
  Play,
  AlertTriangle,
  ChevronRight,
  Film,
  Tv,
  Calendar,
  HardDrive,
  Info,
} from 'lucide-react';
import AppLayout from '@/components/AppLayout';

export default function JobHistoryPage() {
  const navigate = useNavigate();
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);

  const { data: jobsData, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: () => apiClient.listJobs(),
    refetchInterval: 5000, // Refresh every 5 seconds
  });

  const jobs = jobsData?.jobs || [];

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="h-5 w-5 text-green-500" />;
      case 'failed':
        return <XCircle className="h-5 w-5 text-red-500" />;
      case 'running':
        return <Play className="h-5 w-5 text-blue-500 animate-pulse" />;
      default:
        return <Clock className="h-5 w-5 text-gray-500" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const variants: Record<string, any> = {
      completed: 'default',
      failed: 'destructive',
      running: 'secondary',
      pending: 'outline',
    };
    return (
      <Badge variant={variants[status] || 'outline'} className="capitalize">
        {status}
      </Badge>
    );
  };

  const getTypeBadge = (type: string) => {
    return (
      <Badge variant="outline" className="capitalize">
        {type.replace('_', ' ')}
      </Badge>
    );
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;

    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    const seconds = Math.floor(ms / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}m ${remainingSeconds}s`;
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '0 GB';
    const gb = bytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(2)} GB`;
  };

  return (
    <AppLayout>
      <div className="container mx-auto max-w-[1600px] px-4 py-6">
        {/* Page Header */}
        <div className="mb-6">
          <h2 className="text-3xl font-bold mb-2 text-white">Job History</h2>
          <p className="text-gray-400">
            View synchronization history and dry-run previews
          </p>
        </div>

        {/* Job List */}
        {isLoading ? (
          <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-8 text-center">
            <Clock className="h-12 w-12 mx-auto mb-4 text-gray-500 animate-spin" />
            <p className="text-gray-400">Loading job history...</p>
          </div>
        ) : jobs.length === 0 ? (
          <div className="bg-[#1a1a1a] border border-[#333] rounded-md p-8 text-center">
            <Info className="h-12 w-12 mx-auto mb-4 text-gray-500" />
            <p className="text-lg font-medium mb-2 text-white">No sync jobs yet</p>
            <p className="text-gray-400 mb-4">
              Job history will appear here after the first sync with Radarr/Sonarr
            </p>
            <div className="flex flex-col items-center gap-2 text-sm text-gray-400">
              <p>Sync jobs are created automatically based on your schedule, or you can:</p>
              <Button 
                variant="outline" 
                size="sm" 
                onClick={() => navigate('/settings')}
                className="mt-2 bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
              >
                Configure Sync Settings
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {jobs.map((job) => (
              <div
                key={job.id}
                className="bg-[#1a1a1a] border border-[#333] rounded-md p-6 hover:bg-[#262626] cursor-pointer transition-colors"
                onClick={() => setSelectedJob(job)}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-4 flex-1">
                    {/* Status Icon */}
                    <div className="mt-1">{getStatusIcon(job.status)}</div>

                    {/* Job Details */}
                    <div className="flex-1 space-y-2">
                      <div className="flex items-center gap-2">
                        <Badge variant="outline" className="bg-[#262626] text-gray-300 border-[#444] capitalize">
                          {job.type.replace('_', ' ')}
                        </Badge>
                        <Badge variant="outline" className={`capitalize ${
                          job.status === 'completed' ? 'bg-green-900/20 text-green-400 border-green-900/50' :
                          job.status === 'failed' ? 'bg-red-900/20 text-red-400 border-red-900/50' :
                          job.status === 'running' ? 'bg-blue-900/20 text-blue-400 border-blue-900/50' :
                          'bg-[#262626] text-gray-400 border-[#444]'
                        }`}>
                          {job.status}
                        </Badge>
                        {job.summary?.dry_run && (
                          <Badge variant="outline" className="bg-yellow-900/20 text-yellow-400 border-yellow-900/50">
                            Dry Run
                          </Badge>
                        )}
                      </div>

                      <div className="flex items-center gap-4 text-sm text-gray-400">
                        <div className="flex items-center gap-1">
                          <Calendar className="h-4 w-4" />
                          {formatDate(job.started_at)}
                        </div>
                        <div className="flex items-center gap-1">
                          <Clock className="h-4 w-4" />
                          {formatDuration(job.duration_ms)}
                        </div>
                      </div>

                      {/* Summary Stats */}
                      {job.summary && (
                        <div className="flex items-center gap-4 text-sm text-gray-300">
                          {job.summary.movies !== undefined && (
                            <div className="flex items-center gap-1">
                              <Film className="h-4 w-4 text-blue-500" />
                              {job.summary.movies} movies
                            </div>
                          )}
                          {job.summary.tv_shows !== undefined && (
                            <div className="flex items-center gap-1">
                              <Tv className="h-4 w-4 text-purple-500" />
                              {job.summary.tv_shows} shows
                            </div>
                          )}
                          {job.summary.leaving_soon_count !== undefined &&
                            job.summary.leaving_soon_count > 0 && (
                              <div className="flex items-center gap-1 text-orange-400">
                                <Clock className="h-4 w-4" />
                                {job.summary.leaving_soon_count} leaving soon
                              </div>
                            )}
                          {job.summary.scheduled_deletions !== undefined &&
                            job.summary.scheduled_deletions > 0 && (
                              <div className="flex items-center gap-1 text-red-400">
                                <AlertTriangle className="h-4 w-4" />
                                {job.summary.scheduled_deletions} scheduled
                              </div>
                            )}
                        </div>
                      )}

                      {job.error && (
                        <div className="text-sm text-red-400 flex items-start gap-1">
                          <XCircle className="h-4 w-4 mt-0.5" />
                          {job.error}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Arrow */}
                  <ChevronRight className="h-5 w-5 text-gray-500" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Job Details Dialog */}
        <Dialog open={selectedJob !== null} onOpenChange={() => setSelectedJob(null)}>
          <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
            {selectedJob && (
              <>
                <DialogHeader>
                  <DialogTitle className="flex items-center gap-2">
                    {getStatusIcon(selectedJob.status)}
                    Job Details
                  </DialogTitle>
                  <DialogDescription>
                    {getTypeBadge(selectedJob.type)} {getStatusBadge(selectedJob.status)}
                    {selectedJob.summary?.dry_run && (
                      <Badge
                        variant="outline"
                        className="ml-2 bg-yellow-500/10 text-yellow-700 border-yellow-500/50"
                      >
                        Dry Run Mode
                      </Badge>
                    )}
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-6">
                  {/* Metadata */}
                  <Card>
                    <CardHeader>
                      <CardTitle className="text-base">Metadata</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-2 text-sm">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Job ID:</span>
                        <span className="font-mono">{selectedJob.id}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Started:</span>
                        <span>{new Date(selectedJob.started_at).toLocaleString()}</span>
                      </div>
                      {selectedJob.completed_at && (
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">Completed:</span>
                          <span>
                            {new Date(selectedJob.completed_at).toLocaleString()}
                          </span>
                        </div>
                      )}
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Duration:</span>
                        <span>{formatDuration(selectedJob.duration_ms)}</span>
                      </div>
                    </CardContent>
                  </Card>

                  {/* Summary */}
                  {selectedJob.summary && (
                    <Card>
                      <CardHeader>
                        <CardTitle className="text-base">Summary</CardTitle>
                      </CardHeader>
                      <CardContent className="space-y-2 text-sm">
                        {selectedJob.summary.movies !== undefined && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Movies Synced:</span>
                            <span>{selectedJob.summary.movies}</span>
                          </div>
                        )}
                        {selectedJob.summary.tv_shows !== undefined && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">TV Shows Synced:</span>
                            <span>{selectedJob.summary.tv_shows}</span>
                          </div>
                        )}
                        {selectedJob.summary.total_media !== undefined && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Total Media:</span>
                            <span>{selectedJob.summary.total_media}</span>
                          </div>
                        )}
                        {selectedJob.summary.leaving_soon_count !== undefined && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">
                              Leaving Soon:
                            </span>
                            <span className="text-blue-600 font-medium">
                              {selectedJob.summary.leaving_soon_count}
                            </span>
                          </div>
                        )}
                        {selectedJob.summary.scheduled_deletions !== undefined && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">
                              Scheduled Deletions:
                            </span>
                            <span className="text-amber-600 font-medium">
                              {selectedJob.summary.scheduled_deletions}
                            </span>
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  )}

                  {/* Dry-Run Preview */}
                  {selectedJob.summary?.dry_run &&
                    selectedJob.summary?.would_delete &&
                    selectedJob.summary.would_delete.length > 0 && (
                      <Card className="border-yellow-500/50 bg-yellow-500/5">
                        <CardHeader>
                          <CardTitle className="text-base flex items-center gap-2">
                            <AlertTriangle className="h-5 w-5 text-yellow-600" />
                            Dry-Run Preview: Would Delete{' '}
                            {selectedJob.summary.would_delete.length} Items
                          </CardTitle>
                        </CardHeader>
                        <CardContent>
                          <p className="text-sm text-muted-foreground mb-4">
                            The following items would be deleted if dry-run mode was disabled:
                          </p>
                          <div className="space-y-2 max-h-96 overflow-y-auto">
                            {selectedJob.summary.would_delete.map(
                              (item: DeletionCandidate) => (
                                <div
                                  key={item.id}
                                  className="border rounded-lg p-3 bg-background space-y-2"
                                >
                                  <div className="flex items-start justify-between">
                                    <div className="space-y-1">
                                      <div className="flex items-center gap-2">
                                        <Badge
                                          variant={
                                            item.type === 'movie' ? 'default' : 'secondary'
                                          }
                                        >
                                          {item.type === 'movie' ? (
                                            <Film className="h-3 w-3 mr-1" />
                                          ) : (
                                            <Tv className="h-3 w-3 mr-1" />
                                          )}
                                          {item.type}
                                        </Badge>
                                        <span className="font-medium">
                                          {item.title}
                                          {item.year && (
                                            <span className="text-muted-foreground ml-1">
                                              ({item.year})
                                            </span>
                                          )}
                                        </span>
                                      </div>
                                      {item.reason && (
                                        <p className="text-sm text-muted-foreground">
                                          {item.reason}
                                        </p>
                                      )}
                                    </div>
                                    <div className="text-right text-sm space-y-1">
                                      {item.file_size && (
                                        <div className="flex items-center gap-1 text-muted-foreground">
                                          <HardDrive className="h-3 w-3" />
                                          {formatFileSize(item.file_size)}
                                        </div>
                                      )}
                                      <div className="text-red-600 font-medium">
                                        {item.days_overdue} days overdue
                                      </div>
                                    </div>
                                  </div>
                                </div>
                              )
                            )}
                          </div>
                        </CardContent>
                      </Card>
                    )}

                  {/* Actual Deletions */}
                  {selectedJob.summary?.deleted_count && 
                   selectedJob.summary.deleted_count > 0 &&
                   selectedJob.summary?.deleted_items &&
                   selectedJob.summary.deleted_items.length > 0 && (
                      <Card className="border-red-500/50 bg-red-500/5">
                        <CardHeader>
                          <CardTitle className="text-base flex items-center gap-2">
                            <XCircle className="h-5 w-5 text-red-600" />
                            Deleted {selectedJob.summary.deleted_count} Items
                          </CardTitle>
                        </CardHeader>
                        <CardContent>
                          <p className="text-sm text-muted-foreground mb-4">
                            The following items were permanently deleted:
                          </p>
                          <div className="space-y-2 max-h-96 overflow-y-auto">
                            {selectedJob.summary.deleted_items.map(
                              (item: DeletionCandidate) => (
                                <div
                                  key={item.id}
                                  className="border rounded-lg p-3 bg-background space-y-2"
                                >
                                  <div className="flex items-start justify-between">
                                    <div className="space-y-1">
                                      <div className="flex items-center gap-2">
                                        <Badge
                                          variant={
                                            item.type === 'movie' ? 'default' : 'secondary'
                                          }
                                        >
                                          {item.type === 'movie' ? (
                                            <Film className="h-3 w-3 mr-1" />
                                          ) : (
                                            <Tv className="h-3 w-3 mr-1" />
                                          )}
                                          {item.type}
                                        </Badge>
                                        <span className="font-medium">
                                          {item.title}
                                          {item.year && (
                                            <span className="text-muted-foreground ml-1">
                                              ({item.year})
                                            </span>
                                          )}
                                        </span>
                                      </div>
                                      {item.reason && (
                                        <p className="text-sm text-muted-foreground">
                                          {item.reason}
                                        </p>
                                      )}
                                    </div>
                                    <div className="text-right text-sm space-y-1">
                                      {item.file_size && (
                                        <div className="flex items-center gap-1 text-muted-foreground">
                                          <HardDrive className="h-3 w-3" />
                                          {formatFileSize(item.file_size)}
                                        </div>
                                      )}
                                      <div className="text-red-600 font-medium">
                                        {item.days_overdue} days overdue
                                      </div>
                                    </div>
                                  </div>
                                </div>
                              )
                            )}
                          </div>
                        </CardContent>
                      </Card>
                    )}

                  {/* Error */}
                  {selectedJob.error && (
                    <Card className="border-red-500/50 bg-red-500/5">
                      <CardHeader>
                        <CardTitle className="text-base flex items-center gap-2 text-red-600">
                          <XCircle className="h-5 w-5" />
                          Error
                        </CardTitle>
                      </CardHeader>
                      <CardContent>
                        <p className="text-sm text-red-600">{selectedJob.error}</p>
                      </CardContent>
                    </Card>
                  )}
                </div>
              </>
            )}
          </DialogContent>
        </Dialog>
      </div>
    </AppLayout>
  );
}

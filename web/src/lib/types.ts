export interface AuthResponse {
  token: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface MediaItem {
  id: string;
  title: string;
  year?: number;
  type: 'movie' | 'show';
  jellyfin_id?: string;
  radarr_id?: number;
  sonarr_id?: number;
  last_watched?: string;
  last_synced?: string;
  days_until_deletion?: number;
  deletion_date?: string;
  deletion_reason?: string;
  excluded: boolean;
  file_size?: number;
  file_path?: string;
}

export interface MediaListResponse {
  items: MediaItem[];
  total: number;
}

export interface SyncStatus {
  in_progress: boolean;
  last_sync?: string;
  status?: string;
}

export interface DeletionCandidate {
  id: string;
  title: string;
  year?: number;
  type: 'movie' | 'tv_show';
  file_size?: number;
  delete_after: string;
  days_overdue: number;
  reason?: string;
  last_watched?: string;
}

export interface JobSummary {
  movies?: number;
  tv_shows?: number;
  total_media?: number;
  scheduled_deletions?: number;
  dry_run?: boolean;
  would_delete?: DeletionCandidate[];
  [key: string]: any; // Allow other summary fields
}

export interface Job {
  id: string;
  type: 'full_sync' | 'incremental_sync';
  status: 'pending' | 'running' | 'completed' | 'failed';
  started_at: string;
  completed_at?: string;
  duration_ms: number;
  summary?: JobSummary;
  error?: string;
}

export interface JobListResponse {
  jobs: Job[];
  total: number;
}

export interface ApiError {
  error: string;
  message?: string;
}

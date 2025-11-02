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
  excluded: boolean;
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

export interface Job {
  id: string;
  type: string;
  status: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  error?: string;
  progress?: number;
}

export interface JobListResponse {
  jobs: Job[];
  total: number;
}

export interface ApiError {
  error: string;
  message?: string;
}

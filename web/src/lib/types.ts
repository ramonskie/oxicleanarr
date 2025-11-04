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
  is_requested?: boolean;
  requested_by_user_id?: number;
  requested_by_username?: string;
  requested_by_email?: string;
  tags?: string[];
  jellyfin_match_status?: string;
  jellyfin_mismatch_info?: string;
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
  is_requested?: boolean;
  requested_by_user_id?: number;
  requested_by_username?: string;
  requested_by_email?: string;
  tags?: string[];
  jellyfin_match_status?: string;
  jellyfin_mismatch_info?: string;
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

export interface DeletionExecutionResponse {
  success: boolean;
  scheduled_count: number;
  deleted_count?: number;
  failed_count?: number;
  dry_run?: boolean;
  message: string;
  candidates?: DeletionCandidate[];
  deleted_items?: DeletionCandidate[];
}

// Configuration types
export interface Config {
  admin: AdminConfig;
  app: AppConfig;
  sync: SyncConfig;
  rules: RulesConfig;
  server: ServerConfig;
  integrations: IntegrationsConfig;
  advanced_rules: AdvancedRule[];
}

export interface AdminConfig {
  username: string;
  disable_auth: boolean;
}

export interface AppConfig {
  dry_run: boolean;
  enable_deletion: boolean;
  leaving_soon_days: number;
}

export interface SyncConfig {
  full_interval: number;
  incremental_interval: number;
  auto_start: boolean;
}

export interface RulesConfig {
  movie_retention: string;
  tv_retention: string;
}

export interface ServerConfig {
  host: string;
  port: number;
}

export interface IntegrationsConfig {
  jellyfin: JellyfinIntegration;
  radarr: BaseIntegration;
  sonarr: BaseIntegration;
  jellyseerr: BaseIntegration;
  jellystat: BaseIntegration;
}

export interface BaseIntegration {
  enabled: boolean;
  url: string;
  has_api_key: boolean;
  timeout: string;
}

export interface JellyfinIntegration extends BaseIntegration {
  username: string;
  has_password: boolean;
  leaving_soon_type: string;
  collections: CollectionsConfig;
}

export interface CollectionsConfig {
  enabled: boolean;
  movies: CollectionItemConfig;
  tv_shows: CollectionItemConfig;
}

export interface CollectionItemConfig {
  name: string;
  hide_when_empty: boolean;
}

export interface AdvancedRule {
  name: string;
  type: 'tag' | 'episode' | 'user';
  enabled: boolean;
  tag?: string;
  retention?: string;
  max_episodes?: number;
  max_age?: string;
  require_watched?: boolean;
  users?: UserRule[];
}

export interface UserRule {
  user_id?: number;
  username?: string;
  email?: string;
  retention: string;
  require_watched?: boolean;
}

export interface UpdateConfigRequest {
  admin?: Partial<AdminConfig & { password?: string }>;
  app?: AppConfig;
  sync?: SyncConfig;
  rules?: RulesConfig;
  server?: ServerConfig;
  integrations?: Partial<{
    jellyfin?: Partial<JellyfinIntegration & { password?: string; api_key?: string }>;
    radarr?: Partial<BaseIntegration & { api_key?: string }>;
    sonarr?: Partial<BaseIntegration & { api_key?: string }>;
    jellyseerr?: Partial<BaseIntegration & { api_key?: string }>;
    jellystat?: Partial<BaseIntegration & { api_key?: string }>;
  }>;
  advanced_rules?: AdvancedRule[];
}

export interface RulesListResponse {
  rules: AdvancedRule[];
}

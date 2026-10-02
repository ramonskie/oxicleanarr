export interface AuthResponse {
  token?: string;
  username?: string;
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
  manual_leaving_soon?: boolean;
  file_size?: number;
  file_path?: string;
  is_requested?: boolean;
  requested_by_user_id?: number;
  requested_by_username?: string;
  requested_by_email?: string;
  tags?: string[];
  has_poster?: boolean;
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
  has_poster?: boolean;
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
  episode_files_deleted?: number;
  protected_count?: number;
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
  overlay: OverlayConfig;
  analytics: AnalyticsConfig;
  advanced_rules: AdvancedRule[];
}

export interface ValueThreshold {
  low: number;
  high: number;
}

export interface ValueThresholdsConfig {
  movie: ValueThreshold;
  episode: ValueThreshold;
  show: ValueThreshold;
}

// AnalyticsConfig holds stale-content and ROI (watch-hours-per-GB) settings.
export interface AnalyticsConfig {
  enabled: boolean;
  stale_days: number;
  roi_period_days: number;
  include_age_decay: boolean;
  suggest_deletion_days: number;
  value_thresholds: ValueThresholdsConfig;
}

export interface AdminConfig {
  username: string;
  disable_auth: boolean;
  api_key: string;
}

export interface DiskThresholdConfig {
  enabled: boolean;
  free_space_gb: number;
  check_source: 'radarr' | 'sonarr' | 'lowest';
}

export interface AppConfig {
  dry_run: boolean;
  enable_deletion: boolean;
  leaving_soon_days: number;
  disk_threshold: DiskThresholdConfig;
}

export interface DiskStatus {
  enabled: boolean;
  free_space_gb?: number;
  total_space_gb?: number;
  threshold_gb?: number;
  threshold_breached?: boolean;
  check_source?: string;
  message?: string;
}

export interface SyncConfig {
  full_interval: number;
  incremental_interval: number;
  auto_start: boolean;
}

// OverlayConfig holds the deletion-overlay (poster banner) settings.
export interface OverlayConfig {
  enabled: boolean;
  interval_hours: number;
  text_template: string;
  font_size_percent: number;
  font_color: string;
  background_color: string;
  padding_percent: number;
  corner_radius_percent: number;
  font_path?: string;
}

export interface RulesConfig {
  movie_retention: string;
  tv_retention: string;
  retention_base?: string;        // "last_watched_or_added" | "last_watched" | "added"
  unwatched_behavior?: string;    // "added" | "never"
  unwatched_retention?: string;   // e.g. "180d" — only used when retention_base=last_watched AND unwatched_behavior=added
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
  streamystats: StreamystatsIntegration;
  tracearr: TracearrIntegration;
}

export interface BaseIntegration {
  enabled: boolean;
  url: string;
  has_api_key: boolean;
  timeout: string;
}

export interface JellyfinIntegration extends BaseIntegration {}

export interface StreamystatsIntegration extends BaseIntegration {
  has_server_id: boolean;
  server_id: string;
}

export interface TracearrIntegration extends BaseIntegration {
  has_server_id: boolean;
  server_id: string;
}

export interface AdvancedRule {
  name: string;
  type: 'tag' | 'episode' | 'user' | 'stale' | 'roi';
  enabled: boolean;
  tag?: string;
  retention?: string;
  retention_base?: string;        // per-rule override: "last_watched_or_added" | "last_watched" | "added"
  unwatched_behavior?: string;    // per-rule override: "added" | "never"
  max_episodes?: number;
  max_age?: string;
  require_watched?: boolean;
  users?: UserRule[];
  stale_days?: number;              // stale-rule threshold override
  min_watch_hours_per_gb?: number;  // roi-rule low-value cutoff override
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
  app?: Partial<AppConfig>;
  sync?: SyncConfig;
  rules?: RulesConfig;
  server?: ServerConfig;
  integrations?: Partial<{
    jellyfin?: Partial<JellyfinIntegration & { api_key?: string }>;
    radarr?: Partial<BaseIntegration & { api_key?: string }>;
    sonarr?: Partial<BaseIntegration & { api_key?: string }>;
    jellyseerr?: Partial<BaseIntegration & { api_key?: string }>;
    jellystat?: Partial<BaseIntegration & { api_key?: string }>;
    streamystats?: Partial<StreamystatsIntegration & { api_key?: string }>;
    tracearr?: Partial<TracearrIntegration & { api_key?: string }>;
  }>;
  overlay?: Partial<OverlayConfig>;
  analytics?: Partial<AnalyticsConfig>;
  advanced_rules?: AdvancedRule[];
}

export interface RulesListResponse {
  rules: AdvancedRule[];
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'unknown';

export interface LogLine {
  raw: string;
  level?: LogLevel;
  time?: string;
  message?: string;
  component?: string;
}

export interface LogsResponse {
  file: string;
  lines: LogLine[];
  total: number;
}

// Analytics responses
export type StaleCategory = 'never_watched' | 'stale';
export type ValueCategory = 'low_value' | 'moderate_value' | 'high_value';

export interface StaleItem {
  id: string;
  title: string;
  type: string;
  year?: number;
  file_size: number;
  added_at: string;
  last_watched: string | null;
  watch_count: number;
  category: StaleCategory;
  days_stale: number;
  excluded: boolean;
  manual_leaving_soon: boolean;
}

export interface CategoryCount {
  count: number;
  size_bytes: number;
}

export interface StaleAnalyticsResponse {
  enabled: boolean;
  items: StaleItem[];
  summary: {
    never_watched: CategoryCount;
    stale: CategoryCount;
    total: CategoryCount;
    threshold_days: number;
  };
}

export interface ROIItem {
  id: string;
  title: string;
  type: string;
  year?: number;
  file_size_bytes: number;
  file_size_gb: number;
  watch_count: number;
  gated_play_count: number;
  total_watch_hours: number;
  last_watched: string | null;
  days_since_last_watch: number;
  watch_hours_per_gb: number;
  value_score: number;
  value_category: ValueCategory;
  suggest_deletion: boolean;
  excluded: boolean;
  manual_leaving_soon: boolean;
}

export interface ROIAnalyticsResponse {
  enabled: boolean;
  has_watch_data: boolean;
  items: ROIItem[];
  summary: {
    total_items: number;
    total_storage_gb: number;
    total_watch_hours: number;
    avg_watch_hours_per_gb: number;
    low_value_items: number;
    low_value_storage_gb: number;
    potential_savings_gb: number;
  };
  thresholds: ValueThresholdsConfig;
}

// All-time never-watched ("dead weight") titles.
export interface DeadWeightItem {
  id: string;
  title: string;
  type: string;
  year?: number;
  file_size: number;
  added_at: string;
  excluded: boolean;
  manual_leaving_soon: boolean;
}

export interface DeadWeightAnalyticsResponse {
  enabled: boolean;
  has_watch_data: boolean;
  items: DeadWeightItem[];
  summary: {
    count: number;
    total_size_bytes: number;
  };
}

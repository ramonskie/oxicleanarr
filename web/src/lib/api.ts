import type { 
  AuthResponse, 
  LoginRequest, 
  MediaListResponse, 
  MediaItem, 
  SyncStatus, 
  Job, 
  JobListResponse, 
  DeletionExecutionResponse,
  Config,
  UpdateConfigRequest,
  RulesListResponse,
  AdvancedRule,
  DiskStatus,
  LogsResponse,
  StaleAnalyticsResponse,
  ROIAnalyticsResponse,
  DeadWeightAnalyticsResponse,
} from './types';
import type { ServiceStatusResponse } from './types-services';

const API_BASE = '/api';

// Jellyfin match diagnosis. The backend serves one response shape for both the
// read-only analysis and the confirmed fix; `analysis` carries the adjudicator
// verdict, its confidence, and the human-readable evidence behind it.
export type MatchVerdict = 'jellyfin_wrong' | 'arr_wrong' | 'ambiguous';

export interface MatchAnalysis {
  verdict: MatchVerdict;
  confidence: number;
  evidence: string[];
}

export interface MatchResponse {
  analysis: MatchAnalysis;
  fixed: boolean;
  jellyfin_id: string;
  matched_title: string;
  provider_ids: Record<string, string> | null;
}

// Refusal body returned by the match endpoints on 409/422. `analysis` is present
// only when the refusal was driven by a fresh adjudication, so the UI can replace
// a stale verdict instead of leaving an actionable Fix button behind.
export interface MatchErrorBody {
  error: string;
  message?: string;
  analysis?: MatchAnalysis;
}

// Non-2xx wrapper that keeps the parsed body and status reachable — the match
// dialog needs a refusal's embedded analysis. It still extends Error, so every
// existing `.message` consumer keeps working unchanged.
export class ApiRequestError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(message: string, status: number, body: unknown) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
    this.body = body;
  }
}

// Extract a fresh adjudication from a match refusal, when the server embedded one.
export function matchAnalysisFromError(error: unknown): MatchAnalysis | undefined {
  if (!(error instanceof ApiRequestError)) return undefined;
  const body = error.body as MatchErrorBody | null | undefined;
  return body?.analysis;
}

class ApiClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    // Merge with provided headers
    if (options.headers) {
      Object.assign(headers, options.headers);
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
      // Same-origin requests send the httpOnly auth cookie automatically.
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const body = await response.json().catch(() => ({
        error: 'Unknown error',
        message: response.statusText,
      }));
      throw new ApiRequestError(
        body.message || body.error || 'Request failed',
        response.status,
        body,
      );
    }

    return response.json();
  }

  // Auth
  async login(credentials: LoginRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(credentials),
    });
  }

  async me(): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/me');
  }

  async logout(): Promise<{ message: string }> {
    return this.request<{ message: string }>('/auth/logout', {
      method: 'POST',
    });
  }

  // Media
  async listMovies(params?: { limit?: number; offset?: number }): Promise<MediaListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.offset) query.set('offset', params.offset.toString());
    const response = await this.request<MediaListResponse>(`/media/movies?${query}`);
    return {
      items: response.items || [],
      total: response.total || 0,
    };
  }

  async listShows(params?: { limit?: number; offset?: number }): Promise<MediaListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.offset) query.set('offset', params.offset.toString());
    const response = await this.request<MediaListResponse>(`/media/shows?${query}`);
    return {
      items: response.items || [],
      total: response.total || 0,
    };
  }

  async listLeavingSoon(params?: { limit?: number; offset?: number }): Promise<MediaListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.offset) query.set('offset', params.offset.toString());
    const response = await this.request<MediaListResponse>(`/media/leaving-soon/list?${query}`);
    return {
      items: response.items || [],
      total: response.total || 0,
    };
  }

  async listExcluded(params?: { limit?: number; offset?: number }): Promise<MediaListResponse> {
    const query = new URLSearchParams();
    query.set('status', 'excluded');
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.offset) query.set('offset', params.offset.toString());
    
    // Fetch both movies and shows with excluded status
    const [moviesResponse, showsResponse] = await Promise.all([
      this.request<MediaListResponse>(`/media/movies?${query}`),
      this.request<MediaListResponse>(`/media/shows?${query}`),
    ]);
    
    // Combine the results (handle null items arrays)
    const movieItems = moviesResponse.items || [];
    const showItems = showsResponse.items || [];
    
    return {
      items: [...movieItems, ...showItems],
      total: moviesResponse.total + showsResponse.total,
    };
  }

  async listUnmatched(params?: { limit?: number; offset?: number }): Promise<MediaListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.offset) query.set('offset', params.offset.toString());
    const response = await this.request<MediaListResponse>(`/media/unmatched?${query}`);
    return {
      items: response.items || [],
      total: response.total || 0,
    };
  }

  async getMediaItem(id: string): Promise<MediaItem> {
    return this.request<MediaItem>(`/media/${id}`);
  }

  // Adjudicate which side (Jellyfin vs Sonarr/Radarr) holds the wrong identity.
  // Read-only and only ever called on an explicit user request.
  async getMatchAnalysis(id: string): Promise<MatchResponse> {
    return this.request<MatchResponse>(`/media/${encodeURIComponent(id)}/match-analysis`);
  }

  // Re-identify the Jellyfin item when the adjudicator finds Jellyfin is the
  // outlier. Must be preceded by an explicit user confirmation; it never runs
  // automatically. `replaceImages` maps to the backend's optional
  // {"replace_images": bool} body (default true server-side) and controls
  // whether Jellyfin's poster/artwork is refreshed to the new identity.
  async fixMatch(id: string, replaceImages: boolean): Promise<MatchResponse> {
    return this.request<MatchResponse>(`/media/${encodeURIComponent(id)}/fix-match`, {
      method: 'POST',
      body: JSON.stringify({ replace_images: replaceImages }),
    });
  }

  async addExclusion(id: string): Promise<void> {
    await this.request(`/media/${id}/exclude`, {
      method: 'POST',
    });
  }

  async removeExclusion(id: string): Promise<void> {
    await this.request(`/media/${id}/exclude`, {
      method: 'DELETE',
    });
  }

  async addManualLeavingSoon(id: string): Promise<void> {
    await this.request(`/media/${id}/manual-leaving-soon`, {
      method: 'POST',
    });
  }

  async removeManualLeavingSoon(id: string): Promise<void> {
    await this.request(`/media/${id}/manual-leaving-soon`, {
      method: 'DELETE',
    });
  }

  async deleteMedia(id: string): Promise<void> {
    await this.request(`/media/${id}`, {
      method: 'DELETE',
    });
  }

  // Sync
  async triggerFullSync(): Promise<void> {
    await this.request('/sync/full', {
      method: 'POST',
    });
  }

  async triggerIncrementalSync(): Promise<void> {
    await this.request('/sync/incremental', {
      method: 'POST',
    });
  }

  async getSyncStatus(): Promise<SyncStatus> {
    return this.request<SyncStatus>('/sync/status');
  }

  // Jobs
  async listJobs(): Promise<JobListResponse> {
    return this.request<JobListResponse>('/jobs');
  }

  async getLatestJob(): Promise<Job> {
    return this.request<Job>('/jobs/latest');
  }

  async getJob(id: string): Promise<Job> {
    return this.request<Job>(`/jobs/${id}`);
  }

  // Deletions
  async executeDeletions(dryRun: boolean = false): Promise<DeletionExecutionResponse> {
    const query = dryRun ? '?dry_run=true' : '';
    return this.request<DeletionExecutionResponse>(`/deletions/execute${query}`, {
      method: 'POST',
    });
  }

  // Configuration
  async getConfig(): Promise<Config> {
    return this.request<Config>('/config');
  }

  async updateConfig(config: UpdateConfigRequest): Promise<{ message: string }> {
    return this.request<{ message: string }>('/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // Rules
  async listRules(): Promise<RulesListResponse> {
    return this.request<RulesListResponse>('/rules');
  }

  async createRule(rule: Omit<AdvancedRule, 'name'> & { name: string }): Promise<AdvancedRule> {
    return this.request<AdvancedRule>('/rules', {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  async updateRule(name: string, rule: Omit<AdvancedRule, 'name'> & { name: string }): Promise<AdvancedRule> {
    return this.request<AdvancedRule>(`/rules/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify(rule),
    });
  }

  async deleteRule(name: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/rules/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    });
  }

  async toggleRule(name: string, enabled: boolean): Promise<AdvancedRule> {
    return this.request<AdvancedRule>(`/rules/${encodeURIComponent(name)}/toggle`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });
  }

  // System
  async restartApplication(force: boolean = false): Promise<{ message: string; status: string }> {
    return this.request<{ message: string; status: string }>('/system/restart', {
      method: 'POST',
      body: JSON.stringify({ force }),
    });
  }

  async getSystemHealth(): Promise<{ status: string; sync_running: boolean; media_count: number; timestamp: string }> {
    return this.request<{ status: string; sync_running: boolean; media_count: number; timestamp: string }>('/system/health');
  }

  async getSystemInfo(): Promise<{ hostname: string; pid: number; go_version: string; restarting: boolean }> {
    return this.request<{ hostname: string; pid: number; go_version: string; restarting: boolean }>('/system/info');
  }

  async getServiceStatus(): Promise<ServiceStatusResponse> {
    return this.request<ServiceStatusResponse>('/system/services');
  }

  async getDiskStatus(): Promise<DiskStatus> {
    return this.request<DiskStatus>('/system/disk');
  }

  // Analytics
  async getStaleAnalytics(category: string = 'all'): Promise<StaleAnalyticsResponse> {
    const query = category && category !== 'all' ? `?category=${encodeURIComponent(category)}` : '';
    return this.request<StaleAnalyticsResponse>(`/analytics/stale${query}`);
  }

  async getROIAnalytics(valueCategory: string = 'all'): Promise<ROIAnalyticsResponse> {
    const query =
      valueCategory && valueCategory !== 'all'
        ? `?value_category=${encodeURIComponent(valueCategory)}`
        : '';
    return this.request<ROIAnalyticsResponse>(`/analytics/roi${query}`);
  }

  async getDeadWeightAnalytics(): Promise<DeadWeightAnalyticsResponse> {
    return this.request<DeadWeightAnalyticsResponse>('/analytics/dead-weight');
  }

  // Logs
  async getLogs(file: 'backend' | 'web' = 'backend', lines: number = 200): Promise<LogsResponse> {
    return this.request<LogsResponse>(`/logs?file=${file}&lines=${lines}`);
  }

  /**
   * Opens an SSE connection that streams live log lines.
   * Returns the EventSource so the caller can close it.
   * Auth is carried by the httpOnly cookie (EventSource sends cookies
   * automatically for same-origin connections), so no token is put in the URL.
   */
  streamLogs(file: 'backend' | 'web' = 'backend', lines: number = 200): EventSource {
    const url = `${API_BASE}/logs?file=${file}&lines=${lines}&stream=true`;
    return new EventSource(url);
  }
}

export const apiClient = new ApiClient();

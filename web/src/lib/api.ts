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
  AdvancedRule
} from './types';
import type { ServiceStatusResponse } from './types-services';

const API_BASE = '/api';

class ApiClient {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('auth_token', token);
    } else {
      localStorage.removeItem('auth_token');
    }
  }

  getToken() {
    if (!this.token) {
      this.token = localStorage.getItem('auth_token');
    }
    return this.token;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const token = this.getToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    // Merge with provided headers
    if (options.headers) {
      Object.assign(headers, options.headers);
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        error: 'Unknown error',
        message: response.statusText,
      }));
      throw new Error(error.message || error.error || 'Request failed');
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
    const response = await this.request<MediaListResponse>(`/media/leaving-soon?${query}`);
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
}

export const apiClient = new ApiClient();

export interface ServiceStatus {
  name: string;
  enabled: boolean;
  online: boolean;
  error?: string;
  latency?: string;
}

export interface ServiceStatusResponse {
  services: ServiceStatus[];
}

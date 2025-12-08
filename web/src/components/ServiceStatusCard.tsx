import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Activity, CheckCircle2, XCircle, AlertCircle, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

export function ServiceStatusCard() {
  const { data, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['service-status'],
    queryFn: () => apiClient.getServiceStatus(),
    refetchInterval: 60000, // Check every minute
  });

  if (isError) {
    return (
      <Card className="bg-[#1a1a1a] border-[#333]">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-gray-400 flex items-center gap-2">
            <Activity className="h-4 w-4" />
            Service Status
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-2 text-red-400 text-sm">
            <AlertCircle className="h-4 w-4" />
            <span>Failed to check services</span>
            <Button variant="ghost" size="icon" className="h-6 w-6 ml-auto" onClick={() => refetch()}>
              <RefreshCw className="h-3 w-3" />
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="bg-[#1a1a1a] border-[#333]">
      <CardHeader className="pb-3 flex flex-row items-center justify-between space-y-0">
        <CardTitle className="text-sm font-medium text-gray-400 flex items-center gap-2">
          <Activity className="h-4 w-4" />
          Service Status
        </CardTitle>
        <Button 
          variant="ghost" 
          size="icon" 
          className="h-6 w-6 text-gray-500 hover:text-white" 
          onClick={() => refetch()}
          disabled={isLoading || isRefetching}
        >
          <RefreshCw className={cn("h-3 w-3", (isLoading || isRefetching) && "animate-spin")} />
        </Button>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {isLoading ? (
            // Skeletons
            Array(5).fill(0).map((_, i) => (
              <div key={i} className="flex items-center justify-between">
                <div className="h-4 w-20 bg-[#333] rounded animate-pulse" />
                <div className="h-4 w-16 bg-[#333] rounded animate-pulse" />
              </div>
            ))
          ) : (
            data?.services.map((service) => (
              <div key={service.name} className="flex items-center justify-between">
                <span className="text-sm text-gray-300">{service.name}</span>
                <div className="flex items-center gap-2">
                  {!service.enabled ? (
                    <Badge variant="outline" className="bg-[#262626] text-gray-500 border-[#444] text-[10px] h-5">
                      Disabled
                    </Badge>
                  ) : service.online ? (
                    <div className="flex items-center gap-1.5">
                      <span className="text-[10px] text-gray-500 hidden group-hover:inline-block">{service.latency}</span>
                      <Badge variant="outline" className="bg-green-900/20 text-green-400 border-green-900/50 text-[10px] h-5 gap-1 pl-1">
                        <CheckCircle2 className="h-3 w-3" />
                        Online
                      </Badge>
                    </div>
                  ) : (
                    <Badge variant="outline" className="bg-red-900/20 text-red-400 border-red-900/50 text-[10px] h-5 gap-1 pl-1" title={service.error}>
                      <XCircle className="h-3 w-3" />
                      Offline
                    </Badge>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}

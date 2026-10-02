import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import AppLayout from '@/components/AppLayout';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useToast } from '@/hooks/use-toast';
import type { StaleItem, ROIItem, DeadWeightItem, ValueCategory } from '@/lib/types';

const CARD_CLASS = 'bg-[#262626] border-[#333]';
const SELECT_CLASS =
  'bg-[#1e1e1e] border border-[#333] text-gray-200 text-sm rounded-md px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-primary cursor-pointer';
const ACTION_BUTTON_CLASS =
  'h-7 px-2 text-xs border-[#444] bg-[#262626] text-gray-300 hover:bg-[#333]';

// Item shape shared by every analytics list for row actions.
interface ActionableItem {
  id: string;
  title: string;
  excluded: boolean;
  manual_leaving_soon: boolean;
}

type ActionKind = 'leaving-soon' | 'remove-leaving-soon' | 'protect' | 'unprotect';
type ActionHandler = (kind: ActionKind, item: ActionableItem) => void;

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDate(iso: string | null): string {
  if (!iso) return 'Never';
  try {
    return new Date(iso).toLocaleDateString();
  } catch {
    return iso;
  }
}

function formatDays(days: number): string {
  return days < 0 ? '—' : `${days}d`;
}

const STALE_BADGE: Record<string, string> = {
  never_watched: 'bg-red-900/60 text-red-300 border-red-800',
  stale: 'bg-yellow-900/60 text-yellow-300 border-yellow-800',
};

const VALUE_BADGE: Record<string, string> = {
  low_value: 'bg-red-900/60 text-red-300 border-red-800',
  moderate_value: 'bg-yellow-900/60 text-yellow-300 border-yellow-800',
  high_value: 'bg-green-900/60 text-green-300 border-green-800',
};

function Badge({ label, className }: { label: string; className?: string }) {
  return (
    <span
      className={`inline-flex px-2 py-0.5 text-xs font-medium rounded border ${
        className ?? 'bg-[#333] text-gray-300 border-[#444]'
      }`}
    >
      {label}
    </span>
  );
}

function SummaryCard({ title, value, sub }: { title: string; value: string; sub?: string }) {
  return (
    <Card className={CARD_CLASS}>
      <CardContent className="p-4">
        <p className="text-xs uppercase tracking-wide text-gray-500">{title}</p>
        <p className="text-2xl font-semibold text-white mt-1">{value}</p>
        {sub && <p className="text-xs text-gray-400 mt-1">{sub}</p>}
      </CardContent>
    </Card>
  );
}

function TableShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-[#333]">
      <table className="w-full text-sm text-left">{children}</table>
    </div>
  );
}

const TH = 'px-4 py-2 text-xs uppercase tracking-wide text-gray-500 bg-[#1e1e1e]';
const TD = 'px-4 py-2 text-gray-300 border-t border-[#2a2a2a]';

function RowActions({ item, onAction }: { item: ActionableItem; onAction: ActionHandler }) {
  return (
    <div className="flex items-center gap-2">
      <Button
        size="sm"
        variant="outline"
        className={ACTION_BUTTON_CLASS}
        disabled={item.excluded && !item.manual_leaving_soon}
        title={
          item.manual_leaving_soon
            ? 'Remove the leaving-soon flag'
            : item.excluded
              ? 'Remove protection first'
              : 'Flag as leaving soon'
        }
        onClick={() => onAction(item.manual_leaving_soon ? 'remove-leaving-soon' : 'leaving-soon', item)}
      >
        {item.manual_leaving_soon ? 'Unflag' : 'Leaving Soon'}
      </Button>
      <Button
        size="sm"
        variant="outline"
        className={ACTION_BUTTON_CLASS}
        onClick={() => onAction(item.excluded ? 'unprotect' : 'protect', item)}
      >
        {item.excluded ? 'Unprotect' : 'Protect'}
      </Button>
    </div>
  );
}

function StaleTab({ onAction }: { onAction: ActionHandler }) {
  const [category, setCategory] = useState('all');
  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['analytics-stale', category],
    queryFn: () => apiClient.getStaleAnalytics(category),
  });

  if (isLoading) return <p className="text-gray-400 py-8">Loading stale content…</p>;
  if (isError || !data) return <p className="text-red-400 py-8">Failed to load stale content.</p>;
  if (!data.enabled) {
    return <p className="text-gray-400 py-8">Analytics is disabled. Set <code>analytics.enabled: true</code> in your configuration.</p>;
  }

  return (
    <div className="space-y-4 mt-4">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <SummaryCard
          title="Never watched"
          value={String(data.summary.never_watched.count)}
          sub={formatBytes(data.summary.never_watched.size_bytes)}
        />
        <SummaryCard
          title={`Stale (> ${data.summary.threshold_days}d)`}
          value={String(data.summary.stale.count)}
          sub={formatBytes(data.summary.stale.size_bytes)}
        />
        <SummaryCard
          title="Total reclaimable"
          value={formatBytes(data.summary.total.size_bytes)}
          sub={`${data.summary.total.count} items`}
        />
      </div>

      <div className="flex items-center justify-between">
        <select className={SELECT_CLASS} value={category} onChange={(e) => setCategory(e.target.value)}>
          <option value="all">All categories</option>
          <option value="never_watched">Never watched</option>
          <option value="stale">Stale</option>
        </select>
        <button onClick={() => refetch()} className="text-sm text-gray-400 hover:text-white" disabled={isFetching}>
          Refresh
        </button>
      </div>

      <TableShell>
        <thead>
          <tr>
            <th className={TH}>Title</th>
            <th className={TH}>Type</th>
            <th className={TH}>Category</th>
            <th className={TH}>Days stale</th>
            <th className={TH}>Last watched</th>
            <th className={TH}>Size</th>
            <th className={TH}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {data.items.map((item: StaleItem) => (
            <tr key={item.id}>
              <td className={TD}>{item.title}{item.year ? ` (${item.year})` : ''}</td>
              <td className={TD}>{item.type}</td>
              <td className={TD}>
                <Badge
                  label={item.category === 'never_watched' ? 'Never watched' : 'Stale'}
                  className={STALE_BADGE[item.category]}
                />
              </td>
              <td className={TD}>{formatDays(item.days_stale)}</td>
              <td className={TD}>{formatDate(item.last_watched)}</td>
              <td className={TD}>{formatBytes(item.file_size)}</td>
              <td className={TD}><RowActions item={item} onAction={onAction} /></td>
            </tr>
          ))}
          {data.items.length === 0 && (
            <tr><td className={TD} colSpan={7}>No stale content.</td></tr>
          )}
        </tbody>
      </TableShell>
    </div>
  );
}

const VALUE_LABEL: Record<ValueCategory, string> = {
  low_value: 'Low',
  moderate_value: 'Moderate',
  high_value: 'High',
};

function ROITab({ onAction }: { onAction: ActionHandler }) {
  const [valueCategory, setValueCategory] = useState('all');
  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['analytics-roi', valueCategory],
    queryFn: () => apiClient.getROIAnalytics(valueCategory),
  });

  if (isLoading) return <p className="text-gray-400 py-8">Loading ROI…</p>;
  if (isError || !data) return <p className="text-red-400 py-8">Failed to load ROI.</p>;
  if (!data.enabled) {
    return <p className="text-gray-400 py-8">Analytics is disabled. Set <code>analytics.enabled: true</code> in your configuration.</p>;
  }
  if (!data.has_watch_data) {
    return <p className="text-gray-400 py-8">No watch-history provider is configured. Enable Jellystat, Streamystats, or Tracearr to compute storage ROI.</p>;
  }

  return (
    <div className="space-y-4 mt-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <SummaryCard title="Total storage" value={`${data.summary.total_storage_gb.toFixed(1)} GB`} sub={`${data.summary.total_items} items`} />
        <SummaryCard title="Total watch hours" value={data.summary.total_watch_hours.toFixed(1)} />
        <SummaryCard title="Avg hours / GB" value={data.summary.avg_watch_hours_per_gb.toFixed(2)} />
        <SummaryCard
          title="Potential savings"
          value={`${data.summary.potential_savings_gb.toFixed(1)} GB`}
          sub={`${data.summary.low_value_items} low-value items`}
        />
      </div>

      <div className="flex items-center justify-between">
        <select className={SELECT_CLASS} value={valueCategory} onChange={(e) => setValueCategory(e.target.value)}>
          <option value="all">All values</option>
          <option value="low_value">Low value</option>
          <option value="moderate_value">Moderate value</option>
          <option value="high_value">High value</option>
        </select>
        <button onClick={() => refetch()} className="text-sm text-gray-400 hover:text-white" disabled={isFetching}>
          Refresh
        </button>
      </div>

      <TableShell>
        <thead>
          <tr>
            <th className={TH}>Title</th>
            <th className={TH}>Value</th>
            <th className={TH}>Score</th>
            <th className={TH}>Hours / GB</th>
            <th className={TH}>Plays</th>
            <th className={TH}>Last watched</th>
            <th className={TH}>Size</th>
            <th className={TH}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {data.items.map((item: ROIItem) => (
            <tr key={item.id}>
              <td className={TD}>
                {item.title}
                {item.year ? ` (${item.year})` : ''}
                {item.suggest_deletion && <span className="ml-2 text-xs text-red-400">deletion candidate</span>}
              </td>
              <td className={TD}>
                <Badge label={VALUE_LABEL[item.value_category]} className={VALUE_BADGE[item.value_category]} />
              </td>
              <td className={TD}>{item.value_score.toFixed(0)}</td>
              <td className={TD}>{item.watch_hours_per_gb.toFixed(3)}</td>
              <td className={TD}>{item.gated_play_count}</td>
              <td className={TD}>{formatDate(item.last_watched)}</td>
              <td className={TD}>{item.file_size_gb.toFixed(2)} GB</td>
              <td className={TD}><RowActions item={item} onAction={onAction} /></td>
            </tr>
          ))}
          {data.items.length === 0 && (
            <tr><td className={TD} colSpan={8}>No items.</td></tr>
          )}
        </tbody>
      </TableShell>
    </div>
  );
}

function DeadWeightTab({ onAction }: { onAction: ActionHandler }) {
  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['analytics-deadweight'],
    queryFn: () => apiClient.getDeadWeightAnalytics(),
  });

  if (isLoading) return <p className="text-gray-400 py-8">Loading dead weight…</p>;
  if (isError || !data) return <p className="text-red-400 py-8">Failed to load dead weight.</p>;
  if (!data.enabled) {
    return <p className="text-gray-400 py-8">Analytics is disabled. Set <code>analytics.enabled: true</code> in your configuration.</p>;
  }
  if (!data.has_watch_data) {
    return <p className="text-gray-400 py-8">No watch data is available. Enable Jellyfin or a watch-history provider (Jellystat, Streamystats, Tracearr) to detect never-watched content.</p>;
  }

  return (
    <div className="space-y-4 mt-4">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <SummaryCard title="Never-watched titles" value={String(data.summary.count)} />
        <SummaryCard title="Storage reclaimable" value={formatBytes(data.summary.total_size_bytes)} />
        <SummaryCard title="Largest shown" value={String(data.items.length)} sub="top items by size" />
      </div>

      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-500">
          All-time never-watched titles. The summary covers every title; the table lists the largest titles by size.
        </p>
        <button onClick={() => refetch()} className="text-sm text-gray-400 hover:text-white" disabled={isFetching}>
          Refresh
        </button>
      </div>

      <TableShell>
        <thead>
          <tr>
            <th className={TH}>Title</th>
            <th className={TH}>Type</th>
            <th className={TH}>Added</th>
            <th className={TH}>Size</th>
            <th className={TH}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {data.items.map((item: DeadWeightItem) => (
            <tr key={item.id}>
              <td className={TD}>{item.title}{item.year ? ` (${item.year})` : ''}</td>
              <td className={TD}>{item.type}</td>
              <td className={TD}>{formatDate(item.added_at)}</td>
              <td className={TD}>{formatBytes(item.file_size)}</td>
              <td className={TD}><RowActions item={item} onAction={onAction} /></td>
            </tr>
          ))}
          {data.items.length === 0 && (
            <tr><td className={TD} colSpan={5}>No never-watched titles.</td></tr>
          )}
        </tbody>
      </TableShell>
    </div>
  );
}

const CONFIRM_COPY: Record<ActionKind, { title: string; body: (t: string) => string; confirm: string }> = {
  'leaving-soon': {
    title: 'Flag as Leaving Soon',
    body: (t) => `Flag "${t}" as leaving soon? It will appear in the leaving-soon list and be scheduled for deletion.`,
    confirm: 'Flag as Leaving Soon',
  },
  'remove-leaving-soon': {
    title: 'Remove Leaving Soon Flag',
    body: (t) => `Remove the leaving-soon flag from "${t}"? It returns to normal rule evaluation.`,
    confirm: 'Remove Flag',
  },
  protect: {
    title: 'Protect Item',
    body: (t) => `Protect "${t}" from deletion? It will be excluded from all cleanup rules.`,
    confirm: 'Protect',
  },
  unprotect: {
    title: 'Remove Protection',
    body: (t) => `Remove protection from "${t}"? It becomes eligible for cleanup again.`,
    confirm: 'Unprotect',
  },
};

export default function AnalyticsPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [pending, setPending] = useState<{ kind: ActionKind; id: string; title: string } | null>(null);

  const invalidate = () => {
    for (const key of ['analytics-stale', 'analytics-roi', 'analytics-deadweight']) {
      queryClient.invalidateQueries({ queryKey: [key] });
    }
    queryClient.invalidateQueries({ queryKey: ['movies'] });
    queryClient.invalidateQueries({ queryKey: ['shows'] });
    queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
    queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
    queryClient.invalidateQueries({ queryKey: ['excluded'] });
  };

  const onError = (error: Error, title: string) =>
    toast({ title, description: error.message, variant: 'destructive' });

  const settle = () => setPending(null);

  const protectMutation = useMutation({
    mutationFn: (id: string) => apiClient.addExclusion(id),
    onSuccess: () => { invalidate(); settle(); toast({ title: 'Protected', description: 'Item excluded from deletion.' }); },
    onError: (e: Error) => onError(e, 'Failed to protect item'),
  });
  const unprotectMutation = useMutation({
    mutationFn: (id: string) => apiClient.removeExclusion(id),
    onSuccess: () => { invalidate(); settle(); toast({ title: 'Protection removed' }); },
    onError: (e: Error) => onError(e, 'Failed to remove protection'),
  });
  const addLeavingSoonMutation = useMutation({
    mutationFn: (id: string) => apiClient.addManualLeavingSoon(id),
    onSuccess: () => { invalidate(); settle(); toast({ title: 'Flagged as leaving soon' }); },
    onError: (e: Error) => onError(e, 'Failed to flag item'),
  });
  const removeLeavingSoonMutation = useMutation({
    mutationFn: (id: string) => apiClient.removeManualLeavingSoon(id),
    onSuccess: () => { invalidate(); settle(); toast({ title: 'Leaving Soon flag removed' }); },
    onError: (e: Error) => onError(e, 'Failed to remove flag'),
  });

  const handleAction: ActionHandler = (kind, item) =>
    setPending({ kind, id: item.id, title: item.title });

  const confirming = pending !== null;
  const isPending =
    protectMutation.isPending ||
    unprotectMutation.isPending ||
    addLeavingSoonMutation.isPending ||
    removeLeavingSoonMutation.isPending;

  const confirmAction = () => {
    if (!pending) return;
    const { kind, id } = pending;
    switch (kind) {
      case 'leaving-soon': addLeavingSoonMutation.mutate(id); break;
      case 'remove-leaving-soon': removeLeavingSoonMutation.mutate(id); break;
      case 'protect': protectMutation.mutate(id); break;
      case 'unprotect': unprotectMutation.mutate(id); break;
    }
    // The dialog stays open until the mutation settles (onSuccess closes it),
    // so the pending state is visible and failures remain retryable.
  };

  const copy = pending ? CONFIRM_COPY[pending.kind] : null;

  return (
    <AppLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Analytics</h1>
          <p className="text-sm text-gray-400">
            Stale content, all-time dead weight, and storage ROI derived from the synced library and watch history.
          </p>
        </div>

        <Tabs defaultValue="stale">
          <TabsList className="bg-[#262626] border border-[#333]">
            <TabsTrigger value="stale">Stale Content</TabsTrigger>
            <TabsTrigger value="deadweight">Dead Weight</TabsTrigger>
            <TabsTrigger value="roi">Storage ROI</TabsTrigger>
          </TabsList>
          <TabsContent value="stale"><StaleTab onAction={handleAction} /></TabsContent>
          <TabsContent value="deadweight"><DeadWeightTab onAction={handleAction} /></TabsContent>
          <TabsContent value="roi"><ROITab onAction={handleAction} /></TabsContent>
        </Tabs>
      </div>

      <Dialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open && !isPending) setPending(null);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{copy?.title}</DialogTitle>
            <DialogDescription>{pending && copy ? copy.body(pending.title) : ''}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPending(null)} disabled={isPending}>
              Cancel
            </Button>
            <Button onClick={confirmAction} disabled={isPending}>
              {isPending ? 'Working…' : copy?.confirm}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout>
  );
}

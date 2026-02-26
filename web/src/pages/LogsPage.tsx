import { useState, useEffect, useRef, useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { LogLine, LogLevel } from '@/lib/types';
import AppLayout from '@/components/AppLayout';
import { Button } from '@/components/ui/button';
import { RefreshCw, Radio, Square, Download, ChevronDown } from 'lucide-react';

type LogFile = 'backend' | 'web';
type LineCount = 100 | 200 | 500 | 1000;

const LEVEL_COLORS: Record<string, string> = {
  error: 'text-red-400',
  warn:  'text-yellow-400',
  info:  'text-blue-300',
  debug: 'text-gray-500',
};

const LEVEL_BADGE: Record<string, string> = {
  error: 'bg-red-900/60 text-red-300 border-red-800',
  warn:  'bg-yellow-900/60 text-yellow-300 border-yellow-800',
  info:  'bg-blue-900/60 text-blue-300 border-blue-800',
  debug: 'bg-gray-800 text-gray-400 border-gray-700',
};

const ALL_LEVELS: LogLevel[] = ['error', 'warn', 'info', 'debug'];

function levelColor(level?: string): string {
  if (!level) return 'text-gray-400';
  return LEVEL_COLORS[level] ?? 'text-gray-400';
}

function levelBadge(level?: string): string {
  if (!level) return 'bg-gray-800 text-gray-400 border-gray-700';
  return LEVEL_BADGE[level] ?? 'bg-gray-800 text-gray-400 border-gray-700';
}

function formatTime(iso?: string): string {
  if (!iso) return '';
  try {
    return new Date(iso).toLocaleTimeString([], { hour12: false });
  } catch {
    return iso;
  }
}

const selectClass =
  'bg-[#1e1e1e] border border-[#333] text-gray-200 text-sm rounded-md px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-primary cursor-pointer';

interface LogRowProps {
  line: LogLine;
}

function LogRow({ line }: LogRowProps) {
  const isJson = !!line.level || !!line.message;

  if (!isJson) {
    return (
      <div className="font-mono text-xs text-gray-500 px-3 py-0.5 leading-5 break-all">
        {line.raw}
      </div>
    );
  }

  return (
    <div className={`flex items-start gap-2 px-3 py-0.5 leading-5 hover:bg-white/5 ${levelColor(line.level)}`}>
      {/* Timestamp */}
      <span className="text-[10px] text-gray-600 font-mono shrink-0 pt-px w-[72px]">
        {formatTime(line.time)}
      </span>

      {/* Level badge */}
      <span className={`text-[9px] font-bold uppercase tracking-wider border rounded px-1 shrink-0 mt-0.5 ${levelBadge(line.level)}`}>
        {line.level ?? '???'}
      </span>

      {/* Component */}
      {line.component && (
        <span className="text-[10px] text-gray-600 font-mono shrink-0 pt-px">
          [{line.component}]
        </span>
      )}

      {/* Message */}
      <span className="font-mono text-xs break-all flex-1">
        {line.message ?? line.raw}
      </span>
    </div>
  );
}

export default function LogsPage() {
  const queryClient = useQueryClient();
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const [logFile, setLogFile] = useState<LogFile>('backend');
  const [lineCount, setLineCount] = useState<LineCount>(200);
  const [levelFilter, setLevelFilter] = useState<Set<LogLevel>>(new Set(ALL_LEVELS));
  const [streaming, setStreaming] = useState(false);
  const [streamLines, setStreamLines] = useState<LogLine[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);

  const esRef = useRef<EventSource | null>(null);

  // Static snapshot query (only when not streaming)
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['logs', logFile, lineCount],
    queryFn: () => apiClient.getLogs(logFile, lineCount),
    enabled: !streaming,
    refetchOnWindowFocus: false,
    staleTime: Infinity,
  });

  // Auto-scroll whenever lines change and autoScroll is on
  useEffect(() => {
    if (autoScroll) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [data?.lines, streamLines, autoScroll]);

  // Detect manual scroll up to disable auto-scroll
  const handleScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    setAutoScroll(atBottom);
  }, []);

  // Start/stop SSE stream
  useEffect(() => {
    if (!streaming) {
      esRef.current?.close();
      esRef.current = null;
      return;
    }

    setStreamLines([]);
    const es = apiClient.streamLogs(logFile, lineCount);
    esRef.current = es;

    es.addEventListener('log', (e: MessageEvent) => {
      try {
        const ll: LogLine = JSON.parse(e.data);
        setStreamLines((prev) => {
          const next = [...prev, ll];
          return next.length > 2000 ? next.slice(next.length - 2000) : next;
        });
      } catch {
        // ignore parse errors
      }
    });

    es.addEventListener('error', () => {
      setStreaming(false);
    });

    return () => {
      es.close();
      esRef.current = null;
    };
  }, [streaming, logFile, lineCount]);

  // When switching file or line count, reset stream lines and invalidate cache
  useEffect(() => {
    setStreamLines([]);
    if (streaming) {
      setStreaming(false);
    }
    queryClient.invalidateQueries({ queryKey: ['logs'] });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logFile, lineCount]);

  const displayLines: LogLine[] = streaming ? streamLines : (data?.lines ?? []);

  const filteredLines = displayLines.filter((l) => {
    const lvl = (l.level as LogLevel | undefined);
    if (!lvl) return levelFilter.has('debug'); // unparsed lines shown when debug is on
    return levelFilter.has(lvl);
  });

  const toggleLevel = (level: LogLevel) => {
    setLevelFilter((prev) => {
      const next = new Set(prev);
      if (next.has(level)) {
        next.delete(level);
      } else {
        next.add(level);
      }
      return next;
    });
  };

  const handleDownload = () => {
    const text = displayLines.map((l) => l.raw).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `oxicleanarr-${logFile}.log`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <AppLayout>
      <div className="flex flex-col h-full gap-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-white">Logs</h1>
            <p className="text-sm text-gray-500 mt-0.5">Live application log viewer</p>
          </div>
        </div>

        {/* Toolbar */}
        <div className="flex flex-wrap items-center gap-3">
          {/* File selector */}
          <select
            value={logFile}
            onChange={(e) => setLogFile(e.target.value as LogFile)}
            className={selectClass}
          >
            <option value="backend">Backend</option>
            <option value="web">HTTP Access</option>
          </select>

          {/* Line count */}
          <select
            value={lineCount}
            onChange={(e) => setLineCount(Number(e.target.value) as LineCount)}
            className={selectClass}
          >
            <option value={100}>Last 100</option>
            <option value={200}>Last 200</option>
            <option value={500}>Last 500</option>
            <option value={1000}>Last 1000</option>
          </select>

          {/* Level filter chips */}
          <div className="flex items-center gap-1.5">
            {ALL_LEVELS.map((lvl) => (
              <button
                key={lvl}
                onClick={() => toggleLevel(lvl)}
                className={`text-[10px] font-bold uppercase tracking-wider border rounded px-2 py-0.5 transition-opacity
                  ${levelBadge(lvl)}
                  ${levelFilter.has(lvl) ? 'opacity-100' : 'opacity-30'}`}
              >
                {lvl}
              </button>
            ))}
          </div>

          {/* Spacer */}
          <div className="flex-1" />

          {/* Actions */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleDownload}
            className="text-gray-400 hover:text-white border border-[#333] hover:bg-[#333]"
            title="Download log file"
          >
            <Download className="h-4 w-4 mr-1.5" />
            Download
          </Button>

          {!streaming && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => refetch()}
              disabled={isFetching}
              className="text-gray-400 hover:text-white border border-[#333] hover:bg-[#333]"
            >
              <RefreshCw className={`h-4 w-4 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          )}

          <Button
            variant={streaming ? 'destructive' : 'default'}
            size="sm"
            onClick={() => setStreaming((s) => !s)}
            className={streaming ? '' : 'bg-primary hover:bg-primary/90'}
          >
            {streaming ? (
              <>
                <Square className="h-3.5 w-3.5 mr-1.5" />
                Stop
              </>
            ) : (
              <>
                <Radio className="h-3.5 w-3.5 mr-1.5" />
                Live Tail
              </>
            )}
          </Button>
        </div>

        {/* Status bar */}
        <div className="flex items-center gap-3 text-xs text-gray-600">
          {streaming ? (
            <span className="flex items-center gap-1.5 text-green-500">
              <span className="h-1.5 w-1.5 rounded-full bg-green-500 animate-pulse inline-block" />
              Streaming live — {filteredLines.length} lines
            </span>
          ) : (
            <span>
              {isLoading ? 'Loading…' : `${filteredLines.length} lines`}
              {data && ` · ${logFile}.log`}
            </span>
          )}

          {!autoScroll && (
            <button
              onClick={() => {
                setAutoScroll(true);
                bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
              }}
              className="flex items-center gap-1 text-primary hover:text-primary/80 transition-colors"
            >
              <ChevronDown className="h-3 w-3" />
              Scroll to bottom
            </button>
          )}
        </div>

        {/* Log pane */}
        <div
          ref={containerRef}
          onScroll={handleScroll}
          className="flex-1 min-h-0 overflow-y-auto rounded-md border border-[#2a2a2a] bg-[#0d0d0d] py-2"
          style={{ maxHeight: 'calc(100vh - 260px)' }}
        >
          {isLoading && !streaming && (
            <div className="flex items-center justify-center h-32 text-gray-600 text-sm">
              Loading logs…
            </div>
          )}

          {!isLoading && filteredLines.length === 0 && (
            <div className="flex items-center justify-center h-32 text-gray-600 text-sm">
              {streaming ? 'Waiting for log output…' : 'No log lines found'}
            </div>
          )}

          {filteredLines.map((line, i) => (
            <LogRow key={i} line={line} />
          ))}

          <div ref={bottomRef} />
        </div>

        {/* Legend */}
        <div className="flex items-center gap-4 text-[10px] text-gray-600">
          {ALL_LEVELS.map((lvl) => (
            <span key={lvl} className={`flex items-center gap-1 ${levelColor(lvl)}`}>
              <span className="h-1.5 w-1.5 rounded-full bg-current" />
              {lvl}
            </span>
          ))}
          <span className="ml-auto">
            Scroll up to pause auto-scroll · click level chips to filter
          </span>
        </div>
      </div>
    </AppLayout>
  );
}

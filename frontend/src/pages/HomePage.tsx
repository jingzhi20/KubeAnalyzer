import { useState, useMemo, useRef, useCallback } from 'react';
import {
  ChevronRight,
  ChevronLeft,
  MoreHorizontal,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Loader2,
} from 'lucide-react';

/* ── Types ── */
type InspectionStatus = 'completed' | 'failed' | 'running' | 'warning';

interface InspectionEvent {
  id: string;
  title: string;
  clusterName: string;
  status: InspectionStatus;
  triggerType: 'scheduled' | 'manual';
  day: number;        // 1-7 (Mon-Sun)
  startHour: number;
  startMin: number;
  endHour: number;
  endMin: number;
  anomalyCount: number;
  summary: string;
  details: InspectionDetail[];
}

interface InspectionDetail {
  resource: string;
  namespace: string;
  severity: 'critical' | 'warning' | 'info';
  message: string;
}

/* ── Static data ── */
const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const HOURS = Array.from({ length: 12 }, (_, i) => i + 6); // 06:00 – 17:00
const FILTERS = ['All', 'Completed', 'Warning', 'Failed', 'Running'];

const MOCK_EVENTS: InspectionEvent[] = [
  {
    id: '1',
    title: '节点健康检查',
    clusterName: 'prod-cluster',
    status: 'completed',
    triggerType: 'scheduled',
    day: 1, startHour: 7, startMin: 0, endHour: 7, endMin: 40,
    anomalyCount: 0,
    summary: '所有节点运行正常，CPU/内存使用率在合理范围内。',
    details: [
      { resource: 'node/worker-01', namespace: '-', severity: 'info', message: 'CPU 使用率 45%，内存 62%' },
      { resource: 'node/worker-02', namespace: '-', severity: 'info', message: 'CPU 使用率 38%，内存 55%' },
    ],
  },
  {
    id: '2',
    title: 'Pod 异常检测',
    clusterName: 'prod-cluster',
    status: 'warning',
    triggerType: 'scheduled',
    day: 2, startHour: 7, startMin: 0, endHour: 8, endMin: 30,
    anomalyCount: 3,
    summary: '发现 3 个 Pod 处于 CrashLoopBackOff 状态，建议检查日志。',
    details: [
      { resource: 'pod/api-gateway-7d8f6', namespace: 'default', severity: 'critical', message: 'CrashLoopBackOff: OOMKilled，内存限制 256Mi 不足' },
      { resource: 'pod/worker-5c9a2', namespace: 'jobs', severity: 'warning', message: '重启次数 12 次，最近一次 5 分钟前' },
      { resource: 'pod/logger-3b1e8', namespace: 'monitoring', severity: 'warning', message: 'ImagePullBackOff: 镜像拉取失败' },
    ],
  },
  {
    id: '3',
    title: '存储卷巡检',
    clusterName: 'staging-cluster',
    status: 'completed',
    triggerType: 'manual',
    day: 2, startHour: 10, startMin: 0, endHour: 10, endMin: 40,
    anomalyCount: 0,
    summary: 'PVC 绑定状态正常，存储使用率均低于 80%。',
    details: [
      { resource: 'pvc/data-mysql-0', namespace: 'database', severity: 'info', message: '使用率 65%，容量 50Gi' },
    ],
  },
  {
    id: '4',
    title: '安全策略审计',
    clusterName: 'prod-cluster',
    status: 'failed',
    triggerType: 'scheduled',
    day: 2, startHour: 11, startMin: 0, endHour: 12, endMin: 30,
    anomalyCount: 5,
    summary: '巡检执行失败：无法连接到集群 API Server，请检查网络和凭证。',
    details: [
      { resource: 'cluster/prod-cluster', namespace: '-', severity: 'critical', message: '连接超时: dial tcp 10.0.1.100:6443 i/o timeout' },
    ],
  },
  {
    id: '5',
    title: 'Ingress 配置检查',
    clusterName: 'dev-cluster',
    status: 'completed',
    triggerType: 'scheduled',
    day: 4, startHour: 7, startMin: 30, endHour: 9, endMin: 0,
    anomalyCount: 1,
    summary: '发现 1 条 Ingress 规则缺少 TLS 配置。',
    details: [
      { resource: 'ingress/app-frontend', namespace: 'web', severity: 'warning', message: '未配置 TLS，建议启用 HTTPS' },
      { resource: 'ingress/api-backend', namespace: 'web', severity: 'info', message: 'TLS 配置正常，证书有效期 89 天' },
    ],
  },
  {
    id: '6',
    title: 'HPA 弹性伸缩检查',
    clusterName: 'prod-cluster',
    status: 'completed',
    triggerType: 'manual',
    day: 4, startHour: 9, startMin: 0, endHour: 9, endMin: 40,
    anomalyCount: 0,
    summary: 'HPA 配置正常，当前副本数在目标范围内。',
    details: [
      { resource: 'hpa/api-gateway', namespace: 'default', severity: 'info', message: '当前 3/10 副本，CPU 目标 70%，实际 42%' },
    ],
  },
  {
    id: '7',
    title: '网络策略巡检',
    clusterName: 'prod-cluster',
    status: 'warning',
    triggerType: 'scheduled',
    day: 5, startHour: 8, startMin: 0, endHour: 9, endMin: 0,
    anomalyCount: 2,
    summary: '2 个命名空间缺少 NetworkPolicy，存在安全风险。',
    details: [
      { resource: 'namespace/default', namespace: 'default', severity: 'warning', message: '未配置任何 NetworkPolicy' },
      { resource: 'namespace/testing', namespace: 'testing', severity: 'warning', message: '未配置任何 NetworkPolicy' },
    ],
  },
  {
    id: '8',
    title: '全量集群巡检',
    clusterName: 'staging-cluster',
    status: 'running',
    triggerType: 'manual',
    day: 5, startHour: 10, startMin: 0, endHour: 11, endMin: 30,
    anomalyCount: 0,
    summary: '巡检正在执行中...',
    details: [],
  },
];

/* ── Color map for statuses ── */
const STATUS_STYLES: Record<InspectionStatus, { bg: string; border: string; icon: string }> = {
  completed: { bg: 'bg-gradient-to-br from-green-100/80 to-emerald-50/60', border: 'border-green-200/60', icon: 'text-green-600' },
  warning:   { bg: 'bg-gradient-to-br from-amber-100/80 to-yellow-50/60',  border: 'border-amber-200/60',  icon: 'text-amber-600' },
  failed:    { bg: 'bg-gradient-to-br from-red-100/80 to-rose-50/60',      border: 'border-red-200/60',    icon: 'text-red-600' },
  running:   { bg: 'bg-gradient-to-br from-blue-100/80 to-cyan-50/60',     border: 'border-blue-200/60',   icon: 'text-blue-600' },
};

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700',
  warning:  'bg-amber-100 text-amber-700',
  info:     'bg-blue-100 text-blue-700',
};

/* ── Helpers ── */
const fmt = (h: number, m: number) => {
  const suffix = h >= 12 ? 'pm' : 'am';
  const hh = h > 12 ? h - 12 : h;
  return `${hh}:${m.toString().padStart(2, '0')}${suffix}`;
};

const StatusIcon = ({ status, size = 14 }: { status: InspectionStatus; size?: number }) => {
  switch (status) {
    case 'completed': return <CheckCircle2 size={size} className="text-green-600" />;
    case 'warning':   return <AlertTriangle size={size} className="text-amber-600" />;
    case 'failed':    return <XCircle size={size} className="text-red-600" />;
    case 'running':   return <Loader2 size={size} className="text-blue-600 animate-spin" />;
  }
};

const statusLabel = (s: InspectionStatus) =>
  ({ completed: '完成', warning: '告警', failed: '失败', running: '执行中' }[s]);

const HOUR_HEIGHT = 80; // px per hour row

function HomePage() {
  const [activeFilter, setActiveFilter] = useState('All');
  const [currentDay] = useState(2); // Tuesday highlighted
  const [selectedEvent, setSelectedEvent] = useState<InspectionEvent | null>(null);
  const [timeLine, setTimeLine] = useState<{ hour: number; min: number } | null>({ hour: 8, min: 40 });
  const gridRef = useRef<HTMLDivElement>(null);

  const handleGridClick = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (!gridRef.current) return;
    const rect = gridRef.current.getBoundingClientRect();
    const y = e.clientY - rect.top;
    const totalMinutes = (y / HOUR_HEIGHT) * 60 + 6 * 60; // offset by start hour (6)
    const hour = Math.floor(totalMinutes / 60);
    const min = Math.floor(totalMinutes % 60);
    if (hour >= 6 && hour <= 17) {
      setTimeLine({ hour, min });
    }
  }, []);

  const timeLineOffset = timeLine
    ? (timeLine.hour - 6) * HOUR_HEIGHT + (timeLine.min / 60) * HOUR_HEIGHT
    : 0;

  const filteredEvents = useMemo(() => {
    if (activeFilter === 'All') return MOCK_EVENTS;
    const key = activeFilter.toLowerCase() as InspectionStatus;
    return MOCK_EVENTS.filter((e) => e.status === key);
  }, [activeFilter]);

  return (
    <div className="min-h-full">
      {/* Title + Filters */}
      <div className="flex items-center justify-between mb-6 flex-wrap gap-4">
        <div className="flex items-center gap-3">
          <button className="w-8 h-8 rounded-full border border-gray-200 flex items-center justify-center text-gray-400 hover:bg-gray-100 transition-colors">
            <ChevronLeft size={16} />
          </button>
          <h1 className="text-2xl font-semibold text-gray-800 tracking-tight">
            01-07 January 2025
          </h1>
          <button className="w-8 h-8 rounded-full border border-gray-200 flex items-center justify-center text-gray-400 hover:bg-gray-100 transition-colors">
            <ChevronRight size={16} />
          </button>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {FILTERS.map((f) => (
            <button
              key={f}
              onClick={() => setActiveFilter(f)}
              className={`px-4 py-1.5 rounded-full text-sm font-medium transition-all duration-200 ${
                activeFilter === f
                  ? 'bg-gray-900 text-white shadow-md'
                  : 'bg-white text-gray-500 hover:bg-gray-100 border border-gray-200'
              }`}
            >
              {activeFilter === f && f === 'All' && (
                <CheckCircle2 size={14} className="inline mr-1.5 -mt-0.5" />
              )}
              {f}
            </button>
          ))}
          <button className="px-4 py-1.5 rounded-full text-sm font-medium bg-white text-gray-500 hover:bg-gray-100 border border-gray-200 transition-all duration-200">
            This week
          </button>
        </div>
      </div>

      {/* Calendar Grid */}
      <div className="bg-white rounded-2xl shadow-sm shadow-gray-100 border border-gray-100 overflow-hidden">
        {/* Day headers */}
        <div className="grid grid-cols-[70px_repeat(7,1fr)] border-b border-gray-100">
          <div /> {/* spacer for time column */}
          {DAYS.map((day, i) => {
            const dayNum = i + 1;
            const isToday = dayNum === currentDay;
            return (
              <div
                key={day}
                className={`py-3 text-center text-sm font-medium transition-colors ${
                  isToday
                    ? 'bg-gray-900 text-white rounded-t-xl'
                    : 'text-gray-400'
                }`}
              >
                {dayNum} - {day}
              </div>
            );
          })}
        </div>

        {/* Time grid */}
        <div className="relative cursor-crosshair" ref={gridRef} onClick={handleGridClick}>
          {HOURS.map((hour) => (
            <div
              key={hour}
              className="grid grid-cols-[70px_repeat(7,1fr)] border-b border-gray-50"
              style={{ height: HOUR_HEIGHT }}
            >
              <div className="flex items-start justify-end pr-3 pt-1 text-xs text-gray-400 font-medium">
                {hour.toString().padStart(2, '0')}:00
              </div>
              {DAYS.map((_, di) => (
                <div key={di} className="border-l border-gray-50 relative" />
              ))}
            </div>
          ))}

          {/* Current time line */}
          {timeLine && (
          <div
            className="absolute left-0 right-0 z-20 pointer-events-none transition-all duration-150"
            style={{ top: timeLineOffset }}
          >
            <div className="grid grid-cols-[70px_repeat(7,1fr)]">
              <div className="flex items-center justify-end pr-1">
                <span className="bg-gray-900 text-white text-[10px] font-semibold px-2 py-0.5 rounded-full">
                  {timeLine.hour}:{timeLine.min.toString().padStart(2, '0')}
                </span>
              </div>
              <div className="col-span-7 flex items-center">
                <div className="w-full h-[2px] bg-gray-900" />
              </div>
            </div>
          </div>
          )}

        </div>

        {/* Overlay event cards */}
        <div className="relative" style={{ marginTop: `-${HOURS.length * HOUR_HEIGHT}px`, height: HOURS.length * HOUR_HEIGHT, pointerEvents: 'none' }}>
          <div className="grid grid-cols-[70px_repeat(7,1fr)] h-full">
            <div /> {/* time col spacer */}
            {DAYS.map((_, dayIdx) => {
              const dayNum = dayIdx + 1;
              const dayEvents = filteredEvents.filter((e) => e.day === dayNum);
              return (
                <div key={dayIdx} className="relative border-l border-transparent">
                  {dayEvents.map((event) => {
                    const topOffset =
                      (event.startHour - 6) * HOUR_HEIGHT +
                      (event.startMin / 60) * HOUR_HEIGHT;
                    const duration =
                      (event.endHour - event.startHour) * 60 +
                      (event.endMin - event.startMin);
                    const height = (duration / 60) * HOUR_HEIGHT;
                    const style = STATUS_STYLES[event.status];

                    return (
                      <div
                        key={event.id}
                        onClick={() => setSelectedEvent(event)}
                        className={`absolute left-1 right-1 rounded-xl border ${style.bg} ${style.border} p-2.5 overflow-hidden cursor-pointer hover:shadow-md transition-shadow duration-200`}
                        style={{
                          top: topOffset,
                          height: Math.max(height, 40),
                          pointerEvents: 'auto',
                        }}
                      >
                        <div className="flex items-start justify-between">
                          <div className="min-w-0 flex-1">
                            <div className="text-xs font-semibold text-gray-800 truncate flex items-center gap-1">
                              <StatusIcon status={event.status} size={12} />
                              {event.title}
                            </div>
                            <div className="text-[10px] text-gray-500 mt-0.5">
                              {fmt(event.startHour, event.startMin)} -{' '}
                              {fmt(event.endHour, event.endMin)}
                            </div>
                          </div>
                          <button className="text-gray-400 hover:text-gray-600 flex-shrink-0 pointer-events-auto">
                            <MoreHorizontal size={14} />
                          </button>
                        </div>
                        {event.anomalyCount > 0 && (
                          <div className="mt-auto pt-1 flex items-center gap-1">
                            <span className={`inline-flex items-center gap-1 bg-white/70 backdrop-blur-sm text-[10px] font-medium px-2 py-0.5 rounded-full ${
                              event.status === 'failed' ? 'text-red-700' : 'text-amber-700'
                            }`}>
                              <AlertTriangle size={10} />
                              {event.anomalyCount} 异常
                            </span>
                          </div>
                        )}
                        {event.anomalyCount === 0 && event.status === 'completed' && (
                          <div className="mt-auto pt-1 flex items-center gap-1">
                            <span className="inline-flex items-center gap-1 bg-white/70 backdrop-blur-sm text-[10px] font-medium text-emerald-700 px-2 py-0.5 rounded-full">
                              <CheckCircle2 size={10} className="text-emerald-500" />
                              正常
                            </span>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Detail Modal */}
      {selectedEvent && (
        <div className="fixed inset-0 bg-black/40 backdrop-blur-sm flex items-center justify-center z-50" onClick={() => setSelectedEvent(null)}>
          <div className="bg-white rounded-2xl shadow-2xl w-[560px] max-h-[80vh] overflow-hidden" onClick={(e) => e.stopPropagation()}>
            {/* Modal Header */}
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
              <div>
                <h3 className="text-base font-semibold text-gray-800">{selectedEvent.title}</h3>
                <div className="flex items-center gap-3 mt-1 text-xs text-gray-500">
                  <span>集群: {selectedEvent.clusterName}</span>
                  <span>·</span>
                  <span>{fmt(selectedEvent.startHour, selectedEvent.startMin)} - {fmt(selectedEvent.endHour, selectedEvent.endMin)}</span>
                  <span>·</span>
                  <span>{selectedEvent.triggerType === 'scheduled' ? '定时触发' : '手动触发'}</span>
                </div>
              </div>
              <button onClick={() => setSelectedEvent(null)} className="text-gray-400 hover:text-gray-600 text-sm transition-colors">
                ✕
              </button>
            </div>

            {/* Modal Body */}
            <div className="px-6 py-4 overflow-y-auto max-h-[calc(80vh-80px)]">
              {/* Status bar */}
              <div className="flex items-center gap-3 mb-4">
                <span className={`px-3 py-1 rounded-full text-xs font-medium ${
                  selectedEvent.status === 'completed' ? 'bg-green-100 text-green-700' :
                  selectedEvent.status === 'warning' ? 'bg-amber-100 text-amber-700' :
                  selectedEvent.status === 'failed' ? 'bg-red-100 text-red-700' :
                  'bg-blue-100 text-blue-700'
                }`}>
                  {statusLabel(selectedEvent.status)}
                </span>
                <span className="text-xs text-gray-400">异常数: {selectedEvent.anomalyCount}</span>
              </div>

              {/* Summary */}
              <div className="mb-5">
                <div className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-2">巡检摘要</div>
                <p className="text-sm text-gray-700 leading-relaxed bg-gray-50 rounded-xl p-4">{selectedEvent.summary}</p>
              </div>

              {/* Details */}
              {selectedEvent.details.length > 0 && (
                <div>
                  <div className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-2">详细信息</div>
                  <div className="space-y-2">
                    {selectedEvent.details.map((d, i) => (
                      <div key={i} className="bg-gray-50 rounded-xl p-3">
                        <div className="flex items-center gap-2 mb-1">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-semibold ${SEVERITY_COLORS[d.severity]}`}>
                            {d.severity.toUpperCase()}
                          </span>
                          <span className="text-xs font-medium text-gray-800">{d.resource}</span>
                          {d.namespace !== '-' && <span className="text-[10px] text-gray-400">ns: {d.namespace}</span>}
                        </div>
                        <div className="text-xs text-gray-600">{d.message}</div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default HomePage;

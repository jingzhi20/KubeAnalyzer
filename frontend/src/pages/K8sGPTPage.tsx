import { useState, useEffect } from 'react';
import { k8sgptApi, clusterApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { K8sGPTAnalyzeResult, K8sGPTAnalyzeStats, ClusterConfig } from '../types';
import CustomSelect from '../components/CustomSelect';

const traefikFilters = ['IngressRoute','IngressRouteTCP','IngressRouteUDP','Middleware','MiddlewareTCP','TraefikService','TLSOption','TLSStore'];
const hiddenFilters = ['StatefulSet', 'CronJob', 'Job', 'PodDisruptionBudget', 'IngressRouteTCP', 'IngressRouteUDP', 'MiddlewareTCP', 'PersistentVolumeClaim', 'HPA', 'PersistentVolume'];
const istioFilters = ['VirtualService','DestinationRule','IstioGateway','ServiceEntry','Sidecar','PeerAuthentication','AuthorizationPolicy'];
const gatewayAPIFilters = ['GatewayClass','Gateway','HTTPRoute'];
const olmFilters = ['ClusterCatalog','ClusterExtension','ClusterServiceVersion','Subscription','CatalogSource','OperatorGroup'];
const webhookFilters = ['MutatingWebhookConfiguration','ValidatingWebhookConfiguration'];
const networkDiagFilters = ['NetworkComponentPods', 'IngressAccessLog'];

// 诊断分析 - 核心诊断能力，独立展示
const diagnosticFilters = ['Log', 'WarningEvents', 'Secret', 'Security', 'Storage'];
const diagnosticLabels: Record<string, string> = {
  'Log': '日志分析',
  'WarningEvents': '事件诊断',
  'Secret': 'Secret 检查',
  'Security': '安全审计',
  'Storage': '存储健康',
};
const networkDiagLabels: Record<string, string> = {
  'NetworkComponentPods': '网络组件 Pod 诊断',
  'IngressAccessLog': '🌐 Ingress 访问日志分析',
};
const specialFilters = [...traefikFilters, ...istioFilters, ...gatewayAPIFilters, ...olmFilters, ...networkDiagFilters, ...diagnosticFilters, ...webhookFilters];

function K8sGPTPage() {
  const [clusters, setClusters] = useState<ClusterConfig[]>([]);
  const [selectedClusterId, setSelectedClusterId] = useState<number>(0);
  const [filters, setFilters] = useState<string[]>([]);
  const [selectedFilters, setSelectedFilters] = useState<string[]>([]);
  const [namespace, setNamespace] = useState('');
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [loadingNs, setLoadingNs] = useState(false);
  const [explain, setExplain] = useState(true);
  const [labelSelector, setLabelSelector] = useState('');
  const [withStats, setWithStats] = useState(false);
  const [useCache, setUseCache] = useState(false);
  const [results, setResults] = useState<K8sGPTAnalyzeResult[]>([]);
  const [stats, setStats] = useState<K8sGPTAnalyzeStats[]>([]);
  const [rawJSON, setRawJSON] = useState('');
  const [analyzing, setAnalyzing] = useState(false);
  const [problems, setProblems] = useState(0);

  useEffect(() => {
    loadClusters();
    loadFilters();
  }, []);

  // Reload namespaces when selected cluster changes
  useEffect(() => {
    loadNamespaces();
    setNamespace('');
  }, [selectedClusterId]);

  const loadClusters = async () => {
    try {
      const response = await clusterApi.list();
      setClusters(response.data);
      if (response.data.length === 1) {
        setSelectedClusterId(response.data[0].id);
      }
    } catch (err) {
      console.error('Failed to load clusters:', err);
    }
  };

  const loadFilters = async () => {
    try {
      const response = await k8sgptApi.listFilters();
      setFilters(response.data.filters);
    } catch (err) {
      console.error('Failed to load filters:', err);
      setFilters(['Pod', 'Service', 'Deployment', 'ReplicaSet', 'StatefulSet', 'Node', 'Ingress', 'PersistentVolumeClaim', 'CronJob', 'Job', 'DaemonSet', 'ConfigMap', 'Secret', 'HPA', 'PersistentVolume', 'NetworkPolicy', 'PodDisruptionBudget', 'Endpoints', 'MutatingWebhookConfiguration', 'ValidatingWebhookConfiguration', 'Log', 'WarningEvents', 'Security', 'Storage', 'NetworkComponentPods']);
    }
  };

  const loadNamespaces = async () => {
    setLoadingNs(true);
    try {
      const response = await k8sgptApi.listNamespaces(selectedClusterId || undefined);
      setNamespaces(response.data.namespaces || []);
    } catch (err) {
      console.error('Failed to load namespaces:', err);
    } finally {
      setLoadingNs(false);
    }
  };

  const handleAnalyze = async () => {
    setAnalyzing(true);
    setResults([]);
    setStats([]);
    setRawJSON('');
    try {
      const response = await k8sgptApi.analyze({
        filters: selectedFilters.length > 0 ? selectedFilters : undefined,
        namespace: namespace || undefined,
        label_selector: labelSelector || undefined,
        explain,
        with_stats: withStats,
        use_cache: useCache,
        cluster_id: selectedClusterId || undefined,
      });
      setResults(response.data.results || []);
      setProblems(response.data.problems || 0);
      setStats(response.data.stats || []);
      setRawJSON(response.data.raw_json || '');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '分析失败');
    } finally {
      setAnalyzing(false);
    }
  };

  const handleInvalidateCache = async () => {
    try {
      await k8sgptApi.invalidateCache();
      alert('缓存已清除');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '清除缓存失败');
    }
  };

  const toggleFilter = (f: string) => {
    setSelectedFilters(prev =>
      prev.includes(f) ? prev.filter(x => x !== f) : [...prev, f]
    );
  };

  return (
    <div style={styles.container}>
      {/* 核心配置区域 */}
      <div style={styles.topCard}>
        <div style={styles.cardHeader}>
          <h2 style={styles.cardTitle}>🔍 诊断范围选择</h2>
        </div>
        <div style={styles.toolbarRow}>
          <div style={styles.fieldGroup}>
            <span style={styles.fieldLabel}>目标集群 <span style={{color: '#f44336'}}>*</span></span>
            <CustomSelect
              error={selectedClusterId === 0}
              placeholder="请选择集群..."
              value={String(selectedClusterId)}
              onChange={(v) => setSelectedClusterId(Number(v))}
              options={[
                { value: '0', label: '请选择集群...' },
                ...clusters.map(c => ({
                  value: String(c.id),
                  label: `${c.name}${c.status === 'connected' ? '' : ' (已断开)'}`,
                })),
              ]}
            />
          </div>

          <div style={styles.fieldGroup}>
            <span style={styles.fieldLabel}>命名空间 (Namespace)</span>
            <div style={{ display: 'flex', gap: '8px' }}>
              <CustomSelect
                value={namespace}
                onChange={(v) => setNamespace(v)}
                options={[
                  { value: '', label: '全部命名空间' },
                  ...namespaces.map(ns => ({ value: ns, label: ns })),
                ]}
              />
              <button style={styles.iconBtn} onClick={loadNamespaces} disabled={loadingNs}
                title="刷新 Namespace">
                {loadingNs ? '⏳' : '🔄'}
              </button>
            </div>
          </div>

          <div style={{ ...styles.fieldGroup, flex: 1.5 }}>
            <span style={styles.fieldLabel}>标签选择 (Label Selector)</span>
            <input style={styles.textInput} value={labelSelector}
              onChange={(e) => setLabelSelector(e.target.value)}
              placeholder="例如: app=nginx, tier=frontend" />
          </div>

          <div style={styles.toggleGroup}>
            <button
              style={{ ...styles.toggleBtn, ...(explain ? styles.toggleActive : {}) }}
              onClick={() => setExplain(!explain)}>
              🤖  AI 解释 {explain && '✓'}
            </button>
            <button
              style={{ ...styles.toggleBtn, ...(withStats ? styles.toggleActive : {}) }}
              onClick={() => setWithStats(!withStats)}>
              📊  性能统计 {withStats && '✓'}
            </button>
            <button
              style={{ ...styles.toggleBtn, ...(useCache ? styles.toggleActive : {}) }}
              onClick={() => setUseCache(!useCache)}>
              💾  启用缓存 {useCache && '✓'}
            </button>
            {useCache && (
              <button style={{ ...styles.toggleBtn, color: '#f44336', borderColor: '#ffcdd2', background: '#ffebee' }}
                onClick={handleInvalidateCache}>
                🗑️ 清除缓存
              </button>
            )}
          </div>
        </div>

        <div style={{ height: '1px', background: '#f3f4f6', margin: '24px 0' }} />
        
        <div style={styles.filtersGrid}>
          {/* 网络诊断 */}
          {filters.some(f => networkDiagFilters.includes(f)) && (
            <div style={styles.filterSection}>
              <span style={styles.sectionTitle}>🌐 网络连接诊断</span>
              <div style={styles.filterChips}>
                {filters.filter(f => networkDiagFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      ...(selectedFilters.includes(f) ? styles.chipActive : styles.chipInactive)
                    }}
                    onClick={() => toggleFilter(f)}>
                    {networkDiagLabels[f] || f}
                  </button>
                ))}
              </div>
            </div>
          )}
          
          {/* 故障分析 */}
          {filters.some(f => diagnosticFilters.includes(f)) && (
            <div style={styles.filterSection}>
              <span style={styles.sectionTitle}>⚠️ 异常故障诊断</span>
              <div style={styles.filterChips}>
                {filters.filter(f => diagnosticFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      ...(selectedFilters.includes(f) ? styles.chipActive : styles.chipInactive)
                    }}
                    onClick={() => toggleFilter(f)}>
                    {diagnosticLabels[f] || f}
                  </button>
                ))}
              </div>
            </div>
          )}
          
          {/* Traefik 资源 */}
          {filters.some(f => traefikFilters.includes(f) && !hiddenFilters.includes(f)) && (
            <div style={styles.filterSection}>
              <span style={styles.sectionTitle}>🔀 Traefik 路由规则</span>
              <div style={styles.filterChips}>
                {filters.filter(f => traefikFilters.includes(f) && !hiddenFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      ...(selectedFilters.includes(f) ? styles.chipActive : styles.chipInactive)
                    }}
                    onClick={() => toggleFilter(f)}>
                    {f}
                  </button>
                ))}
              </div>
            </div>
          )}
          
          {/* K8s核心资源 */}
          <div style={styles.filterSection}>
            <span style={styles.sectionTitle}>☸ K8s 工作负载及资源</span>
            <div style={styles.filterChips}>
              {filters.filter(f => !specialFilters.includes(f) && !hiddenFilters.includes(f)).map(f => (
                <button key={f}
                  style={{
                     ...styles.chip,
                     ...(selectedFilters.includes(f) ? styles.chipActive : styles.chipInactive)
                  }}
                  onClick={() => toggleFilter(f)}>
                  {f}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div style={styles.actionRow}>
          <button
            style={{
              ...styles.analyzeBtn,
              opacity: selectedClusterId === 0 || selectedFilters.length === 0 || analyzing ? 0.6 : 1,
              cursor: selectedClusterId === 0 || selectedFilters.length === 0 || analyzing ? 'not-allowed' : 'pointer',
            }}
            onClick={handleAnalyze}
            disabled={analyzing || selectedClusterId === 0 || selectedFilters.length === 0}>
            {analyzing ? '⏳ 正在进行深度分析...' : '▶ 执行集群诊断分析'}
          </button>
        </div>
      </div>

      {/* 结果显示或空状态 */}
      {(results.length > 0 || rawJSON) ? (
        <div style={styles.resultCard}>
          <div style={styles.resultHeaderMain}>
            <h2 style={styles.resultTitle}>📋 诊断报告</h2>
            {problems > 0 && <span style={styles.problemBadge}>发现 {problems} 项异常及隐患</span>}
            {problems === 0 && <span style={styles.successBadge}>集群状态健康</span>}
          </div>
          
          {stats.length > 0 && (
            <div style={styles.statsBox}>
              <strong>分析耗时统计：</strong>
              <div style={styles.statsChips}>
                {stats.map((s, i) => (
                  <span key={i} style={styles.statChip}>
                    <span style={{color: '#666'}}>{s.analyzer}</span>
                    <span style={{fontWeight: 600, color: 'var(--k8s-blue)', marginLeft: '4px'}}>{s.duration}</span>
                  </span>
                ))}
              </div>
            </div>
          )}
          
          {results.length > 0 ? (
            <div style={styles.resultList}>
              {results.map((result, index) => (
                <div key={index} style={styles.resultItem}>
                  <div style={styles.resultHeader}>
                    <span style={styles.kindBadge}>{result.kind}</span>
                    <span style={styles.resultName}>{result.name}</span>
                    {result.parentObject && (
                       <span style={styles.parentObj}>所属：{result.parentObject}</span>
                    )}
                  </div>
                  
                  {result.error && result.error.length > 0 && (
                    <div style={styles.errorSection}>
                      <div style={styles.sectionHeader}>⚠️ 发现问题</div>
                      {result.error.map((e, i) => <div key={i} style={styles.errorItem}>• {e}</div>)}
                    </div>
                  )}
                  
                  {result.details && (
                    <div style={styles.detailsSection}>
                      <div style={styles.sectionHeader}>🤖 AI 修复建议</div>
                      <div style={styles.markdownContent}>
                        <ReactMarkdown>{result.details}</ReactMarkdown>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : rawJSON ? (
            <div style={styles.rawSection}>
              <div style={styles.sectionHeader}>原始分析输出</div>
              <pre style={styles.rawPre}>{rawJSON}</pre>
            </div>
          ) : null}
        </div>
      ) : (
        <div style={styles.emptyState}>
          <div style={styles.emptyIcon}>✨</div>
          <h3 style={styles.emptyTitle}>等待执行分析</h3>
          <p style={styles.emptyDesc}>请在上方选择目标集群和您希望诊断的资源类型，然后点击“执行集群诊断分析”获取健康报告。</p>
        </div>
      )}
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    maxWidth: '1200px',
    margin: '0 auto',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '20px',
    paddingBottom: '40px',
  },
  topCard: {
    background: '#ffffff',
    padding: '24px',
    borderRadius: '8px',
    boxShadow: '0 2px 12px rgba(0,0,0,0.04)',
    border: '1px solid #ebedf0',
  },
  middleCard: {
    background: '#ffffff',
    padding: '24px',
    borderRadius: '8px',
    boxShadow: '0 2px 12px rgba(0,0,0,0.04)',
    border: '1px solid #ebedf0',
  },
  cardHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '20px',
    flexWrap: 'wrap' as const,
    gap: '16px',
  },
  cardTitle: {
    margin: 0,
    fontSize: '18px',
    fontWeight: 600,
    color: '#1f2937',
  },
  toolbarRow: {
    display: 'flex',
    gap: '20px',
    flexWrap: 'nowrap' as const,
    alignItems: 'flex-end',
    marginTop: '16px',
    overflowX: 'auto' as const,
  },
  fieldGroup: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '8px',
    minWidth: '160px',
    flex: 1,
  },
  fieldLabel: {
    fontSize: '13px',
    fontWeight: 600,
    color: '#4b5563',
  },
  selectInput: {
    padding: '10px 14px',
    background: '#f9fafb',
    border: '1px solid #d1d5db',
    borderRadius: '6px',
    fontSize: '14px',
    color: '#111827',
    width: '100%',
    cursor: 'pointer',
    outline: 'none',
    transition: 'all 0.2s',
    WebkitAppearance: 'none' as any,
    MozAppearance: 'none' as any,
    appearance: 'none' as any,
    backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%236b7280' d='M6 8L1 3h10z'/%3E%3C/svg%3E")`,
    backgroundRepeat: 'no-repeat',
    backgroundPosition: 'right 12px center',
    paddingRight: '32px',
  },
  textInput: {
    padding: '10px 14px',
    background: '#f9fafb',
    border: '1px solid #d1d5db',
    borderRadius: '6px',
    fontSize: '14px',
    color: '#111827',
    width: '100%',
    outline: 'none',
    transition: 'all 0.2s',
  },
  iconBtn: {
    padding: '0 14px',
    background: '#f9fafb',
    border: '1.5px solid #d1d5db',
    borderRadius: '8px',
    cursor: 'pointer',
    fontSize: '14px',
    transition: 'all 0.2s',
  },
  filtersGrid: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '24px',
    paddingBottom: '24px',
    borderBottom: '1px solid #f3f4f6',
  },
  filterSection: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '12px',
  },
  sectionTitle: {
    fontSize: '14px',
    fontWeight: 600,
    color: '#374151',
    display: 'flex',
    alignItems: 'center',
    marginBottom: '4px',
  },
  filterChips: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: '10px',
  },
  chip: {
    padding: '6px 16px',
    borderRadius: '20px',
    fontSize: '13px',
    fontWeight: 500,
    cursor: 'pointer',
    transition: 'all 0.2s ease',
    userSelect: 'none' as const,
    border: '1px solid transparent',
  },
  chipInactive: {
    background: '#f3f4f6',
    color: '#4b5563',
    borderColor: '#e5e7eb',
  },
  chipActive: {
    background: '#2563eb',
    color: '#ffffff',
    borderColor: '#2563eb',
    boxShadow: '0 2px 8px rgba(37, 99, 235, 0.3)',
    fontWeight: 600,
  },
  actionRow: {
    display: 'flex',
    justifyContent: 'flex-end',
    marginTop: '24px',
  },
  toggleGroup: {
    display: 'flex',
    gap: '12px',
    flexWrap: 'nowrap' as const,
    alignItems: 'flex-end',
    marginLeft: 'auto', // Pushes it to the right if there is extra space
  },
  toggleBtn: {
    padding: '8px 16px',
    background: '#ffffff',
    border: '1px solid #d1d5db',
    borderRadius: '6px',
    color: '#4b5563',
    fontSize: '13px',
    fontWeight: 500,
    cursor: 'pointer',
    transition: 'all 0.2s',
  },
  toggleActive: {
    background: '#f0fdf4',
    borderColor: '#86efac',
    color: '#166534',
  },
  analyzeBtn: {
    padding: '12px 32px',
    background: 'linear-gradient(135deg, #2563eb, #1d4ed8)',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    fontSize: '15px',
    fontWeight: 600,
    cursor: 'pointer',
    boxShadow: '0 4px 12px rgba(37, 99, 235, 0.2)',
    transition: 'transform 0.1s, box-shadow 0.2s',
  },
  emptyState: {
    background: '#ffffff',
    borderRadius: '8px',
    padding: '80px 20px',
    border: '1px dashed #d1d5db',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    justifyContent: 'center',
    textAlign: 'center' as const,
  },
  emptyIcon: {
    fontSize: '48px',
    marginBottom: '16px',
    background: '#f3f4f6',
    width: '80px',
    height: '80px',
    borderRadius: '40px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyTitle: {
    fontSize: '20px',
    fontWeight: 600,
    color: '#111827',
    margin: '0 0 12px 0',
  },
  emptyDesc: {
    fontSize: '14px',
    color: '#6b7280',
    maxWidth: '400px',
    lineHeight: 1.6,
  },
  resultCard: {
    background: '#ffffff',
    padding: '24px',
    borderRadius: '8px',
    boxShadow: '0 2px 12px rgba(0,0,0,0.04)',
    border: '1px solid #ebedf0',
  },
  resultHeaderMain: {
    display: 'flex',
    alignItems: 'center',
    gap: '16px',
    marginBottom: '20px',
    paddingBottom: '16px',
    borderBottom: '1px solid #f3f4f6',
  },
  resultTitle: {
    margin: 0,
    fontSize: '20px',
    fontWeight: 600,
    color: '#1f2937',
  },
  problemBadge: {
    padding: '6px 16px',
    background: '#fef2f2',
    color: '#dc2626',
    borderRadius: '20px',
    fontSize: '14px',
    fontWeight: 600,
    border: '1px solid #fecaca',
  },
  successBadge: {
    padding: '6px 16px',
    background: '#f0fdf4',
    color: '#166534',
    borderRadius: '20px',
    fontSize: '14px',
    fontWeight: 600,
    border: '1px solid #bbf7d0',
  },
  statsBox: {
    background: '#f8fafc',
    padding: '16px',
    borderRadius: '8px',
    marginBottom: '24px',
    border: '1px solid #e2e8f0',
    fontSize: '14px',
  },
  statsChips: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: '12px',
    marginTop: '12px',
  },
  statChip: {
    background: '#ffffff',
    padding: '6px 12px',
    borderRadius: '6px',
    border: '1px solid #e2e8f0',
    fontSize: '13px',
    boxShadow: '0 1px 2px rgba(0,0,0,0.02)',
  },
  resultList: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '20px',
  },
  resultItem: {
    border: '1px solid #e5e7eb',
    borderRadius: '8px',
    padding: '20px',
    transition: 'box-shadow 0.2s',
  },
  resultHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    marginBottom: '16px',
    flexWrap: 'wrap' as const,
  },
  kindBadge: {
    padding: '4px 12px',
    background: '#e0e7ff',
    color: '#4338ca',
    borderRadius: '6px',
    fontSize: '13px',
    fontWeight: 600,
  },
  resultName: {
    fontSize: '16px',
    fontWeight: 600,
    color: '#111827',
  },
  parentObj: {
    fontSize: '13px',
    color: '#6b7280',
    borderLeft: '2px solid #e5e7eb',
    paddingLeft: '12px',
  },
  sectionHeader: {
    fontWeight: 600,
    marginBottom: '8px',
    fontSize: '14px',
  },
  errorSection: {
    background: '#fef2f2',
    padding: '16px',
    borderRadius: '8px',
    marginBottom: '16px',
    border: '1px solid #fecaca',
  },
  errorItem: {
    color: '#991b1b',
    fontSize: '14px',
    lineHeight: 1.6,
    marginTop: '4px',
  },
  detailsSection: {
    background: '#f8fafc',
    padding: '16px',
    borderRadius: '8px',
    border: '1px solid #e2e8f0',
  },
  markdownContent: {
    fontSize: '14px',
    color: '#334155',
    lineHeight: 1.7,
  },
  rawSection: {
    background: '#f3f4f6',
    padding: '16px',
    borderRadius: '8px',
  },
  rawPre: {
    margin: 0,
    fontSize: '13px',
    color: '#1f2937',
    whiteSpace: 'pre-wrap' as const,
    wordBreak: 'break-all' as const,
  },
};

export default K8sGPTPage;

import { useState, useEffect } from 'react';
import { k8sgptApi, clusterApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { K8sGPTAnalyzeResult, K8sGPTAnalyzeStats, ClusterConfig } from '../types';

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
    <div>
      <div style={styles.analyzeSection}>
        <div style={styles.filtersWrap}>
          {/* 网络诊断 - 快速排查，置顶 */}
          {filters.some(f => networkDiagFilters.includes(f)) && (
            <div style={styles.filterGroup}>
              <span style={styles.sectionLabel}>🌐 网络诊断</span>
              <div style={styles.filterChips}>
                {filters.filter(f => networkDiagFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.diagChip,
                      background: selectedFilters.includes(f) ? '#1976d2' : '#e3f2fd',
                      color: selectedFilters.includes(f) ? 'white' : '#1565c0',
                      border: selectedFilters.includes(f) ? 'none' : '1px solid #90caf9',
                    }}
                    onClick={() => toggleFilter(f)}>
                    {networkDiagLabels[f] || f}
                  </button>
                ))}
              </div>
            </div>
          )}
          {/* 诊断分析 - 核心诊断能力 */}
          {filters.some(f => diagnosticFilters.includes(f)) && (
            <div style={styles.filterGroup}>
              <span style={styles.sectionLabel}>🔍 诊断分析</span>
              <div style={styles.filterChips}>
                {filters.filter(f => diagnosticFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.diagChip,
                      background: selectedFilters.includes(f) ? '#1976d2' : '#e3f2fd',
                      color: selectedFilters.includes(f) ? 'white' : '#1565c0',
                      border: selectedFilters.includes(f) ? 'none' : '1px solid #90caf9',
                    }}
                    onClick={() => toggleFilter(f)}>
                    {diagnosticLabels[f] || f}
                  </button>
                ))}
              </div>
            </div>
          )}
          {/* Traefik CRD 资源 */}
          {filters.some(f => traefikFilters.includes(f) && !hiddenFilters.includes(f)) && (
            <div style={styles.filterGroup}>
              <span style={styles.sectionLabel}>🔀 Traefik CRD</span>
              <div style={styles.filterChips}>
                {filters.filter(f => traefikFilters.includes(f) && !hiddenFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.diagChip,
                      background: selectedFilters.includes(f) ? '#1976d2' : '#e3f2fd',
                      color: selectedFilters.includes(f) ? 'white' : '#1565c0',
                      border: selectedFilters.includes(f) ? 'none' : '1px solid #90caf9',
                    }}
                    onClick={() => toggleFilter(f)}>
                    {f}
                  </button>
                ))}
              </div>
            </div>
          )}
          {/* Kubernetes 核心资源 */}
          <div style={styles.filterGroup}>
            <span style={styles.sectionLabel}>☸ K8s 核心资源</span>
            <div style={styles.filterChips}>
              {filters.filter(f => !specialFilters.includes(f) && !hiddenFilters.includes(f)).map(f => (
                <button key={f}
                  style={{
                    ...styles.diagChip,
                    background: selectedFilters.includes(f) ? '#1976d2' : '#e3f2fd',
                    color: selectedFilters.includes(f) ? 'white' : '#1565c0',
                    border: selectedFilters.includes(f) ? 'none' : '1px solid #90caf9',
                  }}
                  onClick={() => toggleFilter(f)}>
                  {f}
                </button>
              ))}
            </div>
          </div>
        </div>
        {/* 操作栏 */}
        <div style={styles.toolbar}>
          {/* 第一行：集群 → NS → 标签 */}
          <div style={styles.toolbarRow}>
            <div style={styles.fieldGroup}>
              <span style={styles.fieldLabel}>☸ 集群</span>
              <select
                style={{
                  ...styles.toolbarSelect,
                  borderColor: selectedClusterId === 0 ? '#f44336' : '#30363d',
                }}
                value={selectedClusterId}
                onChange={(e) => setSelectedClusterId(Number(e.target.value))}
              >
                <option value={0}>请选择集群 *</option>
                {clusters.map(c => (
                  <option key={c.id} value={c.id}>
                    {c.name}{c.status === 'connected' ? '' : ' (断开)'}
                  </option>
                ))}
              </select>
            </div>

            <span style={styles.separator}>›</span>

            <div style={styles.fieldGroup}>
              <span style={styles.fieldLabel}>📦 Namespace</span>
              <div style={{ display: 'flex', gap: '4px' }}>
                <select style={styles.toolbarSelect} value={namespace}
                  onChange={(e) => setNamespace(e.target.value)}>
                  <option value="">全部</option>
                  {namespaces.map(ns => (
                    <option key={ns} value={ns}>{ns}</option>
                  ))}
                </select>
                <button style={styles.iconBtn} onClick={loadNamespaces} disabled={loadingNs}
                  title="刷新 Namespace">
                  {loadingNs ? '⏳' : '🔄'}
                </button>
              </div>
            </div>

            <span style={styles.separator}>›</span>

            <div style={{ ...styles.fieldGroup, flex: 1 }}>
              <span style={styles.fieldLabel}>🏷️ Label Selector</span>
              <input style={styles.toolbarInput} value={labelSelector}
                onChange={(e) => setLabelSelector(e.target.value)}
                placeholder="app=nginx, tier=frontend" />
            </div>
          </div>

          {/* 第二行：选项 + 开始分析 */}
          <div style={styles.toolbarRow2}>
            <div style={styles.toggleGroup}>
              <button
                style={{ ...styles.toggleBtn, ...(explain ? styles.toggleActive : {}) }}
                onClick={() => setExplain(!explain)}>
                🤖 AI 解释
              </button>
              <button
                style={{ ...styles.toggleBtn, ...(withStats ? styles.toggleActive : {}) }}
                onClick={() => setWithStats(!withStats)}>
                📊 统计
              </button>
              <button
                style={{ ...styles.toggleBtn, ...(useCache ? styles.toggleActive : {}) }}
                onClick={() => setUseCache(!useCache)}>
                💾 缓存
              </button>
              {useCache && (
                <button style={{ ...styles.toggleBtn, color: '#f44336', borderColor: '#f44336' }}
                  onClick={handleInvalidateCache}>
                  🗑️ 清除缓存
                </button>
              )}
            </div>
            <button
              style={{
                ...styles.analyzeBtn2,
                opacity: selectedClusterId === 0 ? 0.5 : 1,
                cursor: selectedClusterId === 0 || analyzing ? 'not-allowed' : 'pointer',
              }}
              onClick={handleAnalyze}
              disabled={analyzing || selectedClusterId === 0}>
              {analyzing ? '⏳ 分析中...' : '▶ 开始分析'}
            </button>
          </div>
        </div>
      </div>

      {(results.length > 0 || rawJSON) && (
        <div style={styles.resultSection}>
          <h3 style={{ marginTop: 0 }}>分析结果 {problems > 0 && <span style={styles.problemBadge}>发现 {problems} 个问题</span>}</h3>
          {stats.length > 0 && (
            <div style={{ marginBottom: '16px', padding: '12px', background: '#f5f5f5', borderRadius: '6px', fontSize: '13px' }}>
              <strong>分析器耗时统计：</strong>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '6px' }}>
                {stats.map((s, i) => (
                  <span key={i} style={{ padding: '2px 8px', background: '#e3f2fd', borderRadius: '4px' }}>
                    {s.analyzer}: {s.duration}
                  </span>
                ))}
              </div>
            </div>
          )}
          {results.length > 0 ? (
            <div style={styles.resultList}>
              {results.map((result, index) => (
                <div key={index} style={styles.resultCard}>
                  <div style={styles.resultHeader}>
                    <span style={styles.kindBadge}>{result.kind}</span>
                    <span style={styles.resultName}>{result.name}</span>
                  </div>
                  {result.error && result.error.length > 0 && (
                    <div style={styles.errorSection}>
                      <strong>错误：</strong>
                      {result.error.map((e, i) => <div key={i} style={styles.errorItem}>{e}</div>)}
                    </div>
                  )}
                  {result.details && (
                    <div style={styles.detailsSection}>
                      <strong>AI 分析：</strong>
                      <ReactMarkdown>{result.details}</ReactMarkdown>
                    </div>
                  )}
                  {result.parentObject && (
                    <div style={styles.parentObj}>
                      <strong>父对象：</strong> {result.parentObject}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : rawJSON ? (
            <div style={styles.rawSection}>
              <strong>原始输出：</strong>
              <pre style={styles.rawPre}>{rawJSON}</pre>
            </div>
          ) : null}
        </div>
      )}

      {analyzing && (
        <div style={{ textAlign: 'center', padding: '24px', color: 'var(--k8s-text-muted)', fontSize: '13px' }}>
          ⏳ 分析中，请稍候...
        </div>
      )}
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  analyzeSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: 'var(--k8s-card-radius)', border: '1px solid var(--k8s-border)' },
  filtersWrap: { display: 'flex', flexDirection: 'column' as const, gap: '14px' },
  filterGroup: { display: 'flex', flexDirection: 'column' as const, gap: '6px' },
  filterChips: { display: 'flex', flexWrap: 'wrap' as const, gap: '6px' },
  sectionLabel: { fontSize: '12px', color: 'var(--k8s-blue)', fontWeight: 600 },
  diagChip: { padding: '5px 12px', borderRadius: '3px', cursor: 'pointer', fontSize: '12px', transition: 'all 0.15s', fontWeight: 500 },
  toolbar: { marginTop: '14px', background: '#0d1117', borderRadius: '6px', padding: '14px', display: 'flex', flexDirection: 'column' as const, gap: '10px' },
  toolbarRow: { display: 'flex', alignItems: 'flex-end', gap: '10px', flexWrap: 'wrap' as const },
  toolbarRow2: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', flexWrap: 'wrap' as const, borderTop: '1px solid #21262d', paddingTop: '10px' },
  fieldGroup: { display: 'flex', flexDirection: 'column' as const, gap: '3px' },
  fieldLabel: { fontSize: '10px', color: '#8b949e', fontWeight: 500, letterSpacing: '0.5px', textTransform: 'uppercase' as const },
  separator: { color: '#30363d', fontSize: '18px', marginBottom: '4px', userSelect: 'none' as const },
  toolbarSelect: { padding: '6px 10px', background: '#161b22', border: '1px solid #30363d', borderRadius: '4px', color: '#c9d1d9', fontSize: '12px', cursor: 'pointer', outline: 'none', minWidth: '130px' },
  toolbarInput: { padding: '6px 10px', background: '#161b22', border: '1px solid #30363d', borderRadius: '4px', color: '#c9d1d9', fontSize: '12px', outline: 'none', width: '100%', boxSizing: 'border-box' as const },
  iconBtn: { padding: '5px 7px', background: '#161b22', border: '1px solid #30363d', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', lineHeight: 1 },
  toggleGroup: { display: 'flex', gap: '4px', flexWrap: 'wrap' as const },
  toggleBtn: { padding: '5px 10px', background: '#161b22', border: '1px solid #30363d', borderRadius: '4px', color: '#8b949e', fontSize: '11px', cursor: 'pointer', transition: 'all 0.15s', fontWeight: 500 },
  toggleActive: { background: '#1f6feb', borderColor: '#1f6feb', color: '#fff' },
  analyzeBtn2: { padding: '8px 20px', background: 'linear-gradient(135deg, #238636, #2ea043)', color: 'white', border: 'none', borderRadius: '4px', fontSize: '13px', fontWeight: 600, letterSpacing: '0.3px', whiteSpace: 'nowrap' as const, cursor: 'pointer' },
  resultSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: 'var(--k8s-card-radius)', border: '1px solid var(--k8s-border)', marginTop: '16px' },
  problemBadge: { fontSize: '13px', color: 'var(--k8s-danger)', fontWeight: 'normal' },
  resultList: { display: 'grid', gap: '12px' },
  resultCard: { border: '1px solid var(--k8s-border-light)', borderRadius: '4px', padding: '14px' },
  resultHeader: { display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '10px' },
  kindBadge: { padding: '2px 10px', background: 'var(--k8s-blue-light)', color: 'var(--k8s-blue)', borderRadius: '3px', fontSize: '11px', fontWeight: 600 },
  resultName: { fontSize: '14px', fontWeight: 500, color: 'var(--k8s-text-primary)' },
  errorSection: { background: '#ffebee', padding: '10px 14px', borderRadius: '4px', marginBottom: '10px', fontSize: '13px' },
  errorItem: { color: 'var(--k8s-danger)', marginTop: '3px' },
  detailsSection: { background: '#f8f9fa', padding: '10px 14px', borderRadius: '4px', marginBottom: '10px', fontSize: '13px', lineHeight: '1.7' },
  parentObj: { fontSize: '12px', color: 'var(--k8s-text-muted)' },
  rawSection: { fontSize: '13px' },
  rawPre: { background: '#f5f5f5', padding: '14px', borderRadius: '4px', overflow: 'auto', fontSize: '12px', maxHeight: '400px' },
};

export default K8sGPTPage;

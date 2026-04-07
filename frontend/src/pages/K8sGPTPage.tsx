import { useState, useEffect } from 'react';
import { k8sgptApi, clusterApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { K8sGPTAnalyzeResult, K8sGPTAnalyzeStats, ClusterConfig } from '../types';

const traefikFilters = ['IngressRoute','IngressRouteTCP','IngressRouteUDP','Middleware','MiddlewareTCP','TraefikService','TLSOption','TLSStore'];
const istioFilters = ['VirtualService','DestinationRule','IstioGateway','ServiceEntry','Sidecar','PeerAuthentication','AuthorizationPolicy'];
const gatewayAPIFilters = ['GatewayClass','Gateway','HTTPRoute'];
const olmFilters = ['ClusterCatalog','ClusterExtension','ClusterServiceVersion','Subscription','CatalogSource','OperatorGroup'];
const webhookFilters = ['MutatingWebhookConfiguration','ValidatingWebhookConfiguration'];
const networkDiagFilters = ['NetworkComponentPods', 'IngressAccessLog'];

// 诊断分析 - 核心诊断能力，独立展示
const diagnosticFilters = ['Log', 'WarningEvents', 'Secret', 'Security', 'Storage'];
const diagnosticLabels: Record<string, string> = {
  'Log': '📋 日志分析',
  'WarningEvents': '⚠️ 事件诊断',
  'Secret': '🔐 Secret 检查',
  'Security': '🛡️ 安全审计',
  'Storage': '💾 存储健康',
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
      <div style={styles.header}>
        <h1 style={styles.title}>K8sGPT 集群分析</h1>
        <select
          style={styles.clusterSelect}
          value={selectedClusterId}
          onChange={(e) => setSelectedClusterId(Number(e.target.value))}
        >
          <option value={0}>请选择集群</option>
          {clusters.map(c => (
            <option key={c.id} value={c.id}>
              {c.name}{c.status === 'connected' ? '' : ' (未连接)'}
            </option>
          ))}
        </select>
      </div>

      <div style={styles.analyzeSection}>
        <h3 style={{ marginTop: 0 }}>分析参数</h3>
        <div style={styles.filtersWrap}>
          <label style={styles.label}>资源过滤器（可多选，不选则分析默认资源）</label>
          <div style={{ marginBottom: '6px', fontSize: '12px', color: '#999' }}>Kubernetes 核心资源</div>
          <div style={styles.filterChips}>
            {filters.filter(f => !specialFilters.includes(f)).map(f => (
              <button key={f}
                style={{
                  ...styles.chip,
                  background: selectedFilters.includes(f) ? '#667eea' : '#f5f5f5',
                  color: selectedFilters.includes(f) ? 'white' : '#333',
                }}
                onClick={() => toggleFilter(f)}>
                {f}
              </button>
            ))}
          </div>
          {filters.some(f => traefikFilters.includes(f)) && (
            <>
              <div style={{ marginTop: '12px', marginBottom: '6px', fontSize: '12px', color: '#999' }}>Traefik CRD 资源</div>
              <div style={styles.filterChips}>
                {filters.filter(f => traefikFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      background: selectedFilters.includes(f) ? '#ff9800' : '#f5f5f5',
                      color: selectedFilters.includes(f) ? 'white' : '#333',
                    }}
                    onClick={() => toggleFilter(f)}>
                    {f}
                  </button>
                ))}
              </div>
            </>
          )}

          {filters.some(f => networkDiagFilters.includes(f)) && (
            <>
              <div style={{ marginTop: '12px', marginBottom: '6px', fontSize: '12px', color: '#f44336', fontWeight: 500 }}>网络诊断（快速排查）</div>
              <div style={styles.filterChips}>
                {filters.filter(f => networkDiagFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      background: selectedFilters.includes(f) ? '#f44336' : '#f5f5f5',
                      color: selectedFilters.includes(f) ? 'white' : '#333',
                    }}
                    onClick={() => toggleFilter(f)}>
                    {networkDiagLabels[f] || f}
                  </button>
                ))}
              </div>
            </>
          )}
          {/* 诊断分析 - 核心诊断能力 */}
          {filters.some(f => diagnosticFilters.includes(f)) && (
            <>
              <div style={{ marginTop: '16px', marginBottom: '6px', fontSize: '13px', color: '#1976d2', fontWeight: 600 }}>🔍 诊断分析（核心能力）</div>
              <div style={styles.filterChips}>
                {filters.filter(f => diagnosticFilters.includes(f)).map(f => (
                  <button key={f}
                    style={{
                      ...styles.chip,
                      background: selectedFilters.includes(f) ? '#1976d2' : '#e3f2fd',
                      color: selectedFilters.includes(f) ? 'white' : '#1565c0',
                      border: selectedFilters.includes(f) ? 'none' : '1px solid #90caf9',
                      fontWeight: 500,
                    }}
                    onClick={() => toggleFilter(f)}>
                    {diagnosticLabels[f] || f}
                  </button>
                ))}
              </div>
            </>
          )}



        </div>
        <div style={{ display: 'flex', gap: '12px', alignItems: 'center', marginTop: '16px', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: '150px' }}>
            <select style={{ ...styles.input, width: '100%' }} value={namespace}
              onChange={(e) => setNamespace(e.target.value)}>
              <option value="">全部 Namespace</option>
              {namespaces.map(ns => (
                <option key={ns} value={ns}>{ns}</option>
              ))}
            </select>
          </div>
          <button style={{ ...styles.configBtn, fontSize: '12px', padding: '6px 10px', whiteSpace: 'nowrap' as const }}
            onClick={loadNamespaces} disabled={loadingNs}>
            {loadingNs ? '加载中...' : '刷新NS'}
          </button>
          <div style={{ flex: 1, minWidth: '180px' }}>
            <input style={{ ...styles.input, width: '100%' }} value={labelSelector}
              onChange={(e) => setLabelSelector(e.target.value)}
              placeholder="Label Selector (如: app=nginx)" />
          </div>
          <label style={{ display: 'flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}>
            <input type="checkbox" checked={explain} onChange={(e) => setExplain(e.target.checked)} />
            AI 解释
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}>
            <input type="checkbox" checked={withStats} onChange={(e) => setWithStats(e.target.checked)} />
            统计
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}>
            <input type="checkbox" checked={useCache} onChange={(e) => setUseCache(e.target.checked)} />
            缓存
          </label>
          {useCache && (
            <button style={{ ...styles.configBtn, fontSize: '12px', padding: '6px 10px', whiteSpace: 'nowrap' as const, color: '#f44336', borderColor: '#f44336' }}
              onClick={handleInvalidateCache}>
              清除缓存
            </button>
          )}
          <button style={styles.analyzeBtn} onClick={handleAnalyze} disabled={analyzing}>
            {analyzing ? '分析中...' : '开始分析'}
          </button>
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

      {!analyzing && results.length === 0 && !rawJSON && (
        <div style={styles.emptyState}>
          <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔍</div>
          <p>选择过滤器和参数，点击 "开始分析" 对集群进行智能诊断</p>
        </div>
      )}
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' },
  title: { fontSize: '28px', color: '#333', margin: 0 },
  configBtn: { padding: '8px 16px', background: 'white', border: '1px solid #ddd', borderRadius: '6px', cursor: 'pointer', fontSize: '14px' },
  clusterSelect: { padding: '8px 12px', border: '1px solid #d0d5dd', borderRadius: '8px', fontSize: '14px', color: '#333', background: '#fff', cursor: 'pointer', minWidth: '200px', outline: 'none' },
  label: { display: 'block', fontSize: '13px', color: '#666', marginBottom: '6px' },
  input: { width: '100%', padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px', boxSizing: 'border-box' as const },
  analyzeSection: { background: 'white', padding: '24px', borderRadius: '10px', marginBottom: '24px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  filtersWrap: { marginBottom: '8px' },
  filterChips: { display: 'flex', flexWrap: 'wrap' as const, gap: '8px', marginTop: '8px' },
  chip: { padding: '6px 14px', borderRadius: '16px', border: 'none', cursor: 'pointer', fontSize: '13px', transition: 'all 0.2s' },
  analyzeBtn: { padding: '10px 28px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '14px', whiteSpace: 'nowrap' as const },
  resultSection: { background: 'white', padding: '24px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  problemBadge: { fontSize: '14px', color: '#f44336', fontWeight: 'normal' },
  resultList: { display: 'grid', gap: '16px' },
  resultCard: { border: '1px solid #eee', borderRadius: '8px', padding: '16px' },
  resultHeader: { display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '12px' },
  kindBadge: { padding: '4px 12px', background: '#e3f2fd', color: '#1976d2', borderRadius: '12px', fontSize: '12px', fontWeight: '600' },
  resultName: { fontSize: '15px', fontWeight: '500', color: '#333' },
  errorSection: { background: '#fff3f3', padding: '12px', borderRadius: '6px', marginBottom: '12px', fontSize: '14px' },
  errorItem: { color: '#d32f2f', marginTop: '4px' },
  detailsSection: { background: '#f8f9fa', padding: '12px', borderRadius: '6px', marginBottom: '12px', fontSize: '14px', lineHeight: '1.6' },
  parentObj: { fontSize: '13px', color: '#888' },
  rawSection: { fontSize: '14px' },
  rawPre: { background: '#f5f5f5', padding: '16px', borderRadius: '6px', overflow: 'auto', fontSize: '13px', maxHeight: '400px' },
  emptyState: { textAlign: 'center' as const, padding: '60px', color: '#999', background: 'white', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
};

export default K8sGPTPage;

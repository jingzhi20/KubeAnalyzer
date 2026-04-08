import { useState, useEffect } from 'react';
import { kubectlAiApi } from '../api';
import type { KubectlAIHistory } from '../types';
import { Settings } from 'lucide-react';

const DEFAULT_EXAMPLES = [
  '列出所有 namespace 中状态异常的 Pod',
  '查看 default namespace 中所有 deployment 的副本数',
  '获取所有节点的资源 requests 分配情况',
  '查找所有 OOMKilled 的容器',
];

function KubectlAIPage() {
  const [prompt, setPrompt] = useState('');
  const [command, setCommand] = useState('');
  const [output, setOutput] = useState('');
  const [warning, setWarning] = useState('');  // Agent 安全策略拦截提示
  const [errorInfo, setErrorInfo] = useState<{ code: string; message: string } | null>(null);
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<'generate' | 'execute'>('generate');
  const [history, setHistory] = useState<KubectlAIHistory[]>([]);
  const [showConfig, setShowConfig] = useState(false);
  const [configForm, setConfigForm] = useState({
    temperature: 0.7,
    system_prompt: '',
  });

  useEffect(() => {
    loadHistory();
    loadConfig();
  }, []);

  const loadHistory = async () => {
    try {
      const response = await kubectlAiApi.getHistory();
      setHistory(response.data);
    } catch (err) {
      console.error('Failed to load history:', err);
    }
  };

  const loadConfig = async () => {
    try {
      const response = await kubectlAiApi.getConfig();
      setConfigForm({
        temperature: response.data.temperature ?? 0.7,
        system_prompt: response.data.system_prompt ?? '',
      });
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  };

  const handleSubmit = async () => {
    if (!prompt.trim()) return;
    setLoading(true);
    setCommand('');
    setOutput('');
    setWarning('');
    setErrorInfo(null);
    try {
      if (mode === 'generate') {
        const response = await kubectlAiApi.generate({ prompt });
        setCommand(response.data.command || '');
        setOutput(response.data.output || '');
      } else {
        const response = await kubectlAiApi.execute({ prompt });
        const data = response.data as any;
        if (data.result) {
          setCommand(data.result.command || '');
          setOutput(data.result.output || '');
        } else {
          setCommand(data.command || '');
          setOutput(data.output || '');
        }
        // Show warning from Agent security policy (high-risk block / read-only mode)
        if (data.warning) {
          setWarning(data.warning);
        }
      }
      loadHistory();
    } catch (err: any) {
      const errData = err.response?.data?.error;
      if (errData?.code === 'LLM_NOT_CONFIGURED') {
        setErrorInfo({ code: 'LLM_NOT_CONFIGURED', message: errData.message });
      } else {
        setErrorInfo({ code: 'ERROR', message: errData?.message || '操作失败' });
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSaveConfig = async () => {
    try {
      const response = await kubectlAiApi.updateConfig(configForm);
      // Sync local state with server response to ensure consistency
      const saved = response.data;
      setConfigForm({
        temperature: saved.temperature ?? 0.7,
        system_prompt: saved.system_prompt ?? '',
      });
      alert('配置已保存');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '保存失败');
    }
  };

  const handleHistoryClick = (item: KubectlAIHistory) => {
    setPrompt(item.prompt);
    setCommand(item.command);
    setOutput(item.output);
  };

  // Use hardcoded default examples
  const getExamples = (): string[] => DEFAULT_EXAMPLES;

  return (
    <div style={styles.container}>
      <div style={styles.leftCol}>
        <div style={styles.headerRow}>
          <h1 style={styles.title}>智能命令助手</h1>
          <button
            style={{
              width: '36px',
              height: '36px',
              borderRadius: '10px',
              background: 'white',
              border: '1px solid #e5e7eb',
              boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              transition: 'all 0.2s',
            }}
            onClick={() => { if (!showConfig) loadConfig(); setShowConfig(!showConfig); }}
            title={showConfig ? '关闭配置' : '配置'}
          >
            <Settings size={16} style={{ color: showConfig ? 'var(--k8s-blue)' : '#6b7280' }} />
          </button>
        </div>
        <div style={styles.main}>

        {showConfig && (
          <div style={styles.configSection}>
            <h3 style={{ marginTop: 0 }}>智能命令配置</h3>
            <p style={{ fontSize: '13px', color: '#999', margin: '0 0 16px' }}>LLM 模型统一使用 “LLM 配置” 页面的默认配置</p>

            {/* 基础配置 */}
            <div style={styles.configGrid}>
              <div>
                <label style={styles.label}>Temperature</label>
                <input style={styles.input} type="number" step="0.1" min="0" max="2"
                  value={configForm.temperature}
                  onChange={(e) => setConfigForm({ ...configForm, temperature: parseFloat(e.target.value) })} />
                <span style={{ fontSize: '12px', color: '#999', marginTop: '4px', display: 'block' }}>
                  控制 AI 生成的随机性，值越低越精确（0~2，推荐 0.7）
                </span>
              </div>
            </div>
            <div style={{ marginTop: '12px' }}>
              <label style={styles.label}>自定义 System Prompt（留空使用默认）</label>
              <textarea style={{ ...styles.input, minHeight: '80px', resize: 'vertical' as const }}
                value={configForm.system_prompt}
                onChange={(e) => setConfigForm({ ...configForm, system_prompt: e.target.value })}
                placeholder="留空则使用内置的 Kubernetes 专家 prompt…" />
            </div>

            <button style={{ ...styles.saveBtn, marginTop: '16px' }} onClick={handleSaveConfig}>保存配置</button>
          </div>
        )}

        {!showConfig && (
        <div style={styles.inputSection}>
          <div style={styles.modeSwitch}>
            <button
              style={{ ...styles.modeBtn, borderRadius: '12px 0 0 12px', ...(mode === 'generate' ? styles.modeBtnActive : {}) }}
              onClick={() => setMode('generate')}>
              仅生成命令
            </button>
            <button
              style={{ ...styles.modeBtn, borderRadius: '0 12px 12px 0', ...(mode === 'execute' ? styles.modeBtnActive : {}) }}
              onClick={() => setMode('execute')}>
              生成并执行
            </button>
          </div>
          <div style={styles.inputRow}>
            <input
              style={styles.promptInput}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
              placeholder="用自然语言描述你想执行的 K8s 操作，如: 列出所有 CrashLoopBackOff 的 Pod"
              disabled={loading}
            />
            <button style={styles.submitBtn} onClick={handleSubmit} disabled={loading}>
              {loading ? '处理中...' : mode === 'generate' ? '生成' : '执行'}
            </button>
          </div>
        </div>
        )}

        {errorInfo && (
          <div style={{
            padding: '16px 20px',
            borderRadius: '10px',
            border: errorInfo.code === 'LLM_NOT_CONFIGURED' ? '2px solid #ff9800' : '2px solid #f44336',
            background: errorInfo.code === 'LLM_NOT_CONFIGURED' ? '#fff8e1' : '#ffebee',
          }}>
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              marginBottom: '8px',
              fontWeight: 600,
              fontSize: '15px',
              color: errorInfo.code === 'LLM_NOT_CONFIGURED' ? '#e65100' : '#c62828',
            }}>
              <span style={{ fontSize: '20px' }}>
                {errorInfo.code === 'LLM_NOT_CONFIGURED' ? '⚙️' : '❌'}
              </span>
              {errorInfo.code === 'LLM_NOT_CONFIGURED' ? 'LLM 未配置' : '操作失败'}
            </div>
            <div style={{
              fontSize: '14px',
              lineHeight: '1.6',
              color: '#333',
            }}>
              {errorInfo.message}
            </div>
            {errorInfo.code === 'LLM_NOT_CONFIGURED' && (
              <button
                style={{ marginTop: '12px', padding: '8px 20px', background: 'var(--k8s-blue)', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px' }}
                onClick={() => setShowConfig(true)}>
                打开配置
              </button>
            )}
          </div>
        )}

        {warning && (
          <div style={{
            padding: '16px 20px',
            borderRadius: '10px',
            border: warning.includes('高危操作已拦截') ? '2px solid #f44336' : '2px solid #ff9800',
            background: warning.includes('高危操作已拦截') ? '#ffebee' : '#fff8e1',
          }}>
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              marginBottom: '8px',
              fontWeight: 600,
              fontSize: '15px',
              color: warning.includes('高危操作已拦截') ? '#c62828' : '#e65100',
            }}>
              <span style={{ fontSize: '20px' }}>
                {warning.includes('高危操作已拦截') ? '🛑' : '🔒'}
              </span>
              {warning.includes('高危操作已拦截') ? 'Agent 安全策略 — 高危操作拦截' : 'Agent 安全策略 — 写权限未授权'}
            </div>
            <div style={{
              fontSize: '14px',
              lineHeight: '1.6',
              color: '#333',
              padding: '10px 12px',
              background: 'rgba(255,255,255,0.7)',
              borderRadius: '6px',
              fontFamily: 'monospace',
            }}>
              {warning}
            </div>
          </div>
        )}

        {command && (
          <div style={styles.resultSection}>
            <h3 style={{ marginTop: 0, marginBottom: '12px' }}>生成的命令</h3>
            <div style={styles.commandBlock}>
              <code style={styles.commandCode}>{command}</code>
              <button style={styles.copyBtn}
                onClick={() => { navigator.clipboard.writeText(command); }}>
                复制
              </button>
            </div>
          </div>
        )}

        {output && mode === 'execute' && (
          <div style={styles.outputSection}>
            <h3 style={{ marginTop: 0, marginBottom: '12px' }}>输出结果</h3>
            <pre style={styles.outputPre}>{output}</pre>
          </div>
        )}

        {!showConfig && !command && !output && !loading && !errorInfo && (
          <div style={styles.emptyState}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>🤖</div>
            <p>输入自然语言描述，AI 将为你生成对应的 kubectl 命令</p>
            <div style={styles.examples}>
              <p style={styles.exampleTitle}>示例：</p>
              <div style={styles.exampleList}>
                {getExamples().map((ex, i) => (
                  <button key={i} style={styles.exampleBtn} onClick={() => setPrompt(ex)}>
                    {ex}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
      </div>

      {/* 右侧历史记录 */}
      <div style={styles.rightCol}>
        <h3 style={styles.sideTitle}>历史记录</h3>
        <div style={styles.sidebar}>
          <div style={styles.historyList}>
            {history.length === 0 && <div style={styles.noHistory}>暂无历史记录</div>}
            {history.map(item => (
              <div key={item.id} style={styles.historyItem} onClick={() => handleHistoryClick(item)}>
                <div style={styles.historyPrompt}>{item.prompt}</div>
                <div style={styles.historyCommand}>{item.command}</div>
                <div style={styles.historyTime}>{new Date(item.created_at).toLocaleString()}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: { display: 'flex', gap: '20px', height: 'calc(100vh - 100px)' },
  leftCol: { flex: 6, display: 'flex', flexDirection: 'column' as const },
  headerRow: { display: 'flex', alignItems: 'center', marginBottom: '16px', gap: '12px' },
  rightCol: { flex: 4, display: 'flex', flexDirection: 'column' as const },
  sideTitle: { margin: '0 0 16px 0', fontSize: '20px', fontWeight: 600, color: 'var(--k8s-text-primary)' },
  main: { flex: 1, display: 'flex', flexDirection: 'column' as const, gap: '16px', overflow: 'auto' },
  sidebar: { flex: 1, background: 'var(--k8s-card-bg)', borderRadius: '16px', border: '1px solid var(--k8s-border)', display: 'flex', flexDirection: 'column' as const, overflow: 'hidden' },
  historyList: { flex: 1, overflow: 'auto', padding: '10px' },
  noHistory: { textAlign: 'center' as const, padding: '24px', color: 'var(--k8s-text-muted)', fontSize: '13px' },
  historyItem: { padding: '10px 12px', borderRadius: '12px', cursor: 'pointer', marginBottom: '4px', border: '1px solid var(--k8s-border-light)', transition: 'border-color 0.15s' },
  historyPrompt: { fontSize: '13px', color: 'var(--k8s-text-primary)', marginBottom: '3px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  historyCommand: { fontSize: '11px', color: 'var(--k8s-blue)', fontFamily: 'var(--k8s-mono)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  historyTime: { fontSize: '10px', color: 'var(--k8s-text-muted)', marginTop: '3px' },
  header: { display: 'none' },
  title: { fontSize: '20px', fontWeight: 600, color: 'var(--k8s-text-primary)', margin: 0 },
  configBtn: { padding: '6px 14px', background: 'var(--k8s-card-bg)', border: '1px solid var(--k8s-border)', borderRadius: '4px', cursor: 'pointer', fontSize: '12px' },
  configSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: '16px', border: '1px solid var(--k8s-border)' },
  configGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '12px', marginBottom: '14px' },
  label: { display: 'block', fontSize: '12px', color: 'var(--k8s-text-secondary)', marginBottom: '4px' },
  input: { width: '100%', padding: '8px 10px', border: '1px solid var(--k8s-border)', borderRadius: '4px', fontSize: '13px', boxSizing: 'border-box' as const, outline: 'none' },
  saveBtn: { padding: '8px 20px', background: 'var(--k8s-blue)', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', fontWeight: 500 },
  inputSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: '16px', border: '1px solid var(--k8s-border)' },
  modeSwitch: { display: 'flex', gap: '0', marginBottom: '14px' },
  modeBtn: { padding: '7px 16px', background: '#f5f5f5', border: '1px solid var(--k8s-border)', cursor: 'pointer', fontSize: '13px', color: 'var(--k8s-text-secondary)', borderRadius: '0' },
  modeBtnActive: { background: 'var(--k8s-blue)', color: 'white', borderColor: 'var(--k8s-blue)' },
  inputRow: { display: 'flex', gap: '10px' },
  promptInput: { flex: 1, padding: '10px 14px', border: '1px solid var(--k8s-border)', borderRadius: '12px', fontSize: '13px', outline: 'none' },
  submitBtn: { padding: '10px 24px', background: 'var(--k8s-blue)', color: 'white', border: 'none', borderRadius: '12px', cursor: 'pointer', fontSize: '13px', whiteSpace: 'nowrap' as const, fontWeight: 500 },
  resultSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: '16px', border: '1px solid var(--k8s-border)' },
  commandBlock: { display: 'flex', alignItems: 'center', background: '#1e1e1e', padding: '12px 14px', borderRadius: '10px', gap: '10px' },
  commandCode: { flex: 1, color: '#4ec9b0', fontSize: '13px', fontFamily: 'var(--k8s-mono)', wordBreak: 'break-all' as const },
  copyBtn: { padding: '4px 12px', background: '#333', color: '#fff', border: 'none', borderRadius: '3px', cursor: 'pointer', fontSize: '11px' },
  outputSection: { background: 'var(--k8s-card-bg)', padding: '20px', borderRadius: '16px', border: '1px solid var(--k8s-border)' },
  outputPre: { background: '#f5f5f5', padding: '14px', borderRadius: '10px', fontSize: '12px', fontFamily: 'var(--k8s-mono)', overflow: 'auto', maxHeight: '400px', lineHeight: '1.5' },
  emptyState: { textAlign: 'center' as const, padding: '36px', color: 'var(--k8s-text-muted)', background: 'var(--k8s-card-bg)', borderRadius: '16px', border: '1px solid var(--k8s-border)', flex: 1, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center' },
  examples: { marginTop: '20px', textAlign: 'left' as const, maxWidth: '480px' },
  exampleTitle: { fontSize: '13px', color: 'var(--k8s-text-secondary)', marginBottom: '6px' },
  exampleList: { display: 'flex', flexDirection: 'column', gap: '6px' },
  exampleBtn: { padding: '8px 14px', background: '#f8f9fa', border: '1px solid var(--k8s-border-light)', borderRadius: '10px', cursor: 'pointer', textAlign: 'left' as const, fontSize: '13px', color: 'var(--k8s-text-secondary)', transition: 'border-color 0.15s' },
};

export default KubectlAIPage;

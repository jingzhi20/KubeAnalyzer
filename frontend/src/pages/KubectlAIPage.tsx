import { useState, useEffect } from 'react';
import { kubectlAiApi } from '../api';
import type { KubectlAIHistory } from '../types';

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
      <div style={styles.sidebar}>
        <div style={styles.sidebarHeader}>
          <h3 style={{ margin: 0, fontSize: '16px' }}>历史记录</h3>
        </div>
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

      <div style={styles.main}>
        <div style={styles.header}>
          <h1 style={styles.title}>智能命令助手</h1>
          <button style={styles.configBtn} onClick={() => { if (!showConfig) loadConfig(); setShowConfig(!showConfig); }}>
            {showConfig ? '关闭配置' : '配置'}
          </button>
        </div>

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
              style={{ ...styles.modeBtn, ...(mode === 'generate' ? styles.modeBtnActive : {}) }}
              onClick={() => setMode('generate')}>
              仅生成命令
            </button>
            <button
              style={{ ...styles.modeBtn, ...(mode === 'execute' ? styles.modeBtnActive : {}) }}
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
                style={{ marginTop: '12px', padding: '8px 20px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '13px' }}
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

        {output && (
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
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: { display: 'flex', gap: '20px', height: 'calc(100vh - 100px)' },
  sidebar: { width: '280px', background: 'white', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)', display: 'flex', flexDirection: 'column', overflow: 'hidden' },
  sidebarHeader: { padding: '16px', borderBottom: '1px solid #eee' },
  historyList: { flex: 1, overflow: 'auto', padding: '8px' },
  noHistory: { textAlign: 'center' as const, padding: '30px', color: '#999', fontSize: '14px' },
  historyItem: { padding: '12px', borderRadius: '6px', cursor: 'pointer', marginBottom: '6px', border: '1px solid #f0f0f0', transition: 'all 0.2s' },
  historyPrompt: { fontSize: '14px', color: '#333', marginBottom: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  historyCommand: { fontSize: '12px', color: '#667eea', fontFamily: 'monospace', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  historyTime: { fontSize: '11px', color: '#999', marginTop: '4px' },
  main: { flex: 1, display: 'flex', flexDirection: 'column', gap: '20px', overflow: 'auto' },
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' },
  title: { fontSize: '28px', color: '#333', margin: 0 },
  configBtn: { padding: '8px 16px', background: 'white', border: '1px solid #ddd', borderRadius: '6px', cursor: 'pointer', fontSize: '14px' },
  configSection: { background: 'white', padding: '24px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  configGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px', marginBottom: '16px' },
  label: { display: 'block', fontSize: '13px', color: '#666', marginBottom: '6px' },
  input: { width: '100%', padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px', boxSizing: 'border-box' as const },
  saveBtn: { padding: '10px 24px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '14px' },
  inputSection: { background: 'white', padding: '24px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  modeSwitch: { display: 'flex', gap: '0', marginBottom: '16px' },
  modeBtn: { padding: '8px 20px', background: '#f5f5f5', border: '1px solid #ddd', cursor: 'pointer', fontSize: '14px', color: '#666' },
  modeBtnActive: { background: '#667eea', color: 'white', borderColor: '#667eea' },
  inputRow: { display: 'flex', gap: '12px' },
  promptInput: { flex: 1, padding: '12px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px' },
  submitBtn: { padding: '12px 28px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '14px', whiteSpace: 'nowrap' as const },
  resultSection: { background: 'white', padding: '24px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  commandBlock: { display: 'flex', alignItems: 'center', background: '#1e1e1e', padding: '16px', borderRadius: '8px', gap: '12px' },
  commandCode: { flex: 1, color: '#4ec9b0', fontSize: '14px', fontFamily: 'monospace', wordBreak: 'break-all' as const },
  copyBtn: { padding: '6px 14px', background: '#333', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '12px' },
  outputSection: { background: 'white', padding: '24px', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' },
  outputPre: { background: '#f5f5f5', padding: '16px', borderRadius: '8px', fontSize: '13px', fontFamily: 'monospace', overflow: 'auto', maxHeight: '400px', lineHeight: '1.5' },
  emptyState: { textAlign: 'center' as const, padding: '40px', color: '#999', background: 'white', borderRadius: '10px', boxShadow: '0 2px 8px rgba(0,0,0,0.1)', flex: 1, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center' },
  examples: { marginTop: '24px', textAlign: 'left' as const, maxWidth: '500px' },
  exampleTitle: { fontSize: '14px', color: '#666', marginBottom: '8px' },
  exampleList: { display: 'flex', flexDirection: 'column', gap: '8px' },
  exampleBtn: { padding: '10px 16px', background: '#f8f9fa', border: '1px solid #eee', borderRadius: '8px', cursor: 'pointer', textAlign: 'left' as const, fontSize: '14px', color: '#555', transition: 'all 0.2s' },
};

export default KubectlAIPage;

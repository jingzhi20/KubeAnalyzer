import { useState, useEffect, useRef } from 'react';
import { diagnosisApi } from '../api';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { DiagnosisSession, DiagnosisRecord } from '../types';
import { Trash2, Pencil, Check, X, SquarePen, Search, Send, Sparkles } from 'lucide-react';
import './DiagnosisPage.css';

function DiagnosisPage() {
  const [sessions, setSessions] = useState<DiagnosisSession[]>([]);
  const [currentSession, setCurrentSession] = useState<DiagnosisSession | null>(null);
  const [question, setQuestion] = useState('');
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState<DiagnosisRecord[]>([]);
  const [errorInfo, setErrorInfo] = useState<{ code: string; message: string } | null>(null);
  const [renamingId, setRenamingId] = useState<number | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [hoveredId, setHoveredId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const pendingQuestion = useRef('');
  const searchInputRef = useRef<HTMLInputElement>(null);
  const isComposingRef = useRef(false);

  useEffect(() => {
    loadSessions();
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [records]);

  useEffect(() => {
    if (showSearch) searchInputRef.current?.focus();
  }, [showSearch]);

  const loadSessions = async () => {
    try {
      const response = await diagnosisApi.listSessions();
      setSessions(response.data);
    } catch (err) {
      console.error('Failed to load sessions:', err);
    }
  };

  const filteredSessions = searchQuery.trim()
    ? sessions.filter(s => s.title.toLowerCase().includes(searchQuery.toLowerCase()))
    : sessions;

  const createSession = async () => {
    try {
      const response = await diagnosisApi.createSession({ title: '新对话' });
      const newSession = response.data;
      setCurrentSession(newSession);
      setRecords([]);
      setErrorInfo(null);
      setShowSearch(false);
      setSearchQuery('');
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  };

  const selectSession = async (session: DiagnosisSession) => {
    setCurrentSession(session);
    setErrorInfo(null);
    try {
      const response = await diagnosisApi.getSession(session.id);
      setRecords(response.data.records || []);
    } catch (err) {
      setRecords([]);
    }
  };

  const deleteSession = async (e: React.MouseEvent, id: number) => {
    e.stopPropagation();
    try {
      await diagnosisApi.deleteSession(id);
      setSessions(prev => prev.filter(s => s.id !== id));
      if (currentSession?.id === id) {
        setCurrentSession(null);
        setRecords([]);
        setErrorInfo(null);
      }
    } catch (err) {
      console.error('Failed to delete session:', err);
    }
  };

  const startRename = (e: React.MouseEvent, session: DiagnosisSession) => {
    e.stopPropagation();
    setRenamingId(session.id);
    setRenameValue(session.title);
  };

  const confirmRename = async (id: number) => {
    if (!renameValue.trim()) { setRenamingId(null); return; }
    try {
      await diagnosisApi.renameSession(id, renameValue.trim());
      setSessions(prev => prev.map(s => s.id === id ? { ...s, title: renameValue.trim() } : s));
      if (currentSession?.id === id) {
        setCurrentSession(prev => prev ? { ...prev, title: renameValue.trim() } : prev);
      }
    } catch (err) {
      console.error('Failed to rename session:', err);
    }
    setRenamingId(null);
  };

  const submitQuery = async () => {
    if (!currentSession || !question.trim()) return;
    const currentQuestion = question.trim();
    setQuestion('');
    pendingQuestion.current = currentQuestion;
    setLoading(true);
    setErrorInfo(null);
    try {
      const response = await diagnosisApi.submitQuery(currentSession.id, { question: currentQuestion });
      setRecords(prev => [...prev, response.data]);
      if (records.length === 0) {
        const title = currentQuestion.slice(0, 20) + (currentQuestion.length > 20 ? '...' : '');
        diagnosisApi.renameSession(currentSession.id, title).catch(() => {});
        const namedSession = { ...currentSession, title };
        setSessions(prev => [namedSession, ...prev]);
        setCurrentSession(namedSession);
      }
    } catch (err: any) {
      const errData = err.response?.data?.error;
      setErrorInfo({
        code: errData?.code || 'ERROR',
        message: errData?.message || '提交失败，请稍后重试',
      });
    } finally {
      setLoading(false);
    }
  };

  // 从欢迎页直接提问：先创建会话，再提交问题
  const submitFromWelcome = async () => {
    const q = question.trim();
    if (!q) return;
    setQuestion('');
    try {
      const response = await diagnosisApi.createSession({ title: '新对话' });
      const newSession = response.data;
      setCurrentSession(newSession);
      setRecords([]);
      setErrorInfo(null);
      setShowSearch(false);
      setSearchQuery('');

      // 提交问题
      pendingQuestion.current = q;
      setLoading(true);
      try {
        const res = await diagnosisApi.submitQuery(newSession.id, { question: q });
        setRecords([res.data]);
        const title = q.slice(0, 20) + (q.length > 20 ? '...' : '');
        diagnosisApi.renameSession(newSession.id, title).catch(() => {});
        const namedSession = { ...newSession, title };
        setSessions(prev => [namedSession, ...prev]);
        setCurrentSession(namedSession);
      } catch (err: any) {
        const errData = err.response?.data?.error;
        setErrorInfo({
          code: errData?.code || 'ERROR',
          message: errData?.message || '提交失败，请稍后重试',
        });
      } finally {
        setLoading(false);
      }
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  };

  const suggestedQuestions = [
    { icon: '🔍', text: '集群中有哪些异常的 Pod？' },
    { icon: '📊', text: '各节点的资源使用情况如何？' },
    { icon: '🌐', text: '集群中有多少个 namespace？' },
    { icon: '⚡', text: '最近有哪些告警事件？' },
  ];

  return (
    <div style={styles.container}>
      {/* 左侧会话列表 */}
      <div style={styles.sidebar}>
        <div style={styles.sidebarHeader}>
          <button style={styles.headerBtn} onClick={createSession} title="新聊天">
            <SquarePen size={16} />
            <span>新聊天</span>
          </button>
        </div>
        <button
          style={{ ...styles.headerBtn, marginBottom: '10px' , color: showSearch ? 'var(--k8s-blue)' : undefined }}
          onClick={() => { setShowSearch(!showSearch); if (showSearch) setSearchQuery(''); }}
          title="搜索聊天"
        >
          <Search size={16} />
          <span>搜索聊天</span>
        </button>
        {showSearch && (
          <input
            ref={searchInputRef}
            style={styles.searchInput}
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder="搜索聊天记录..."
            onKeyDown={e => { if (e.key === 'Escape') { setShowSearch(false); setSearchQuery(''); } }}
          />
        )}
        <div style={styles.sessionList}>
          {filteredSessions.map(session => {
            const isActive = currentSession?.id === session.id;
            const isHovered = hoveredId === session.id;
            return (
              <div
                key={session.id}
                style={{
                  ...styles.sessionItem,
                  background: isActive ? 'var(--k8s-blue)' : 'transparent',
                  color: isActive ? 'white' : 'var(--k8s-text-primary)',
                }}
                onClick={() => renamingId !== session.id && selectSession(session)}
                onMouseEnter={() => setHoveredId(session.id)}
                onMouseLeave={() => setHoveredId(null)}
              >
                {renamingId === session.id ? (
                  <div style={styles.renameRow} onClick={e => e.stopPropagation()}>
                    <input
                      style={styles.renameInput}
                      value={renameValue}
                      autoFocus
                      onChange={e => setRenameValue(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter') confirmRename(session.id);
                        if (e.key === 'Escape') setRenamingId(null);
                      }}
                    />
                    <button style={styles.iconBtn} onClick={() => confirmRename(session.id)}><Check size={13} /></button>
                    <button style={styles.iconBtn} onClick={() => setRenamingId(null)}><X size={13} /></button>
                  </div>
                ) : (
                  <>
                    <span style={styles.sessionTitle}>{session.title}</span>
                    {(isActive || isHovered) && (
                      <div style={styles.sessionActions}>
                        <button
                          style={{ ...styles.iconBtn, color: isActive ? 'rgba(255,255,255,0.8)' : 'var(--k8s-text-muted)' }}
                          onClick={e => startRename(e, session)}
                          title="重命名"
                        >
                          <Pencil size={13} />
                        </button>
                        <button
                          style={{ ...styles.iconBtn, color: isActive ? 'rgba(255,255,255,0.8)' : 'var(--k8s-text-muted)' }}
                          onClick={e => deleteSession(e, session.id)}
                          title="删除"
                        >
                          <Trash2 size={13} />
                        </button>
                      </div>
                    )}
                  </>
                )}
              </div>
            );
          })}
          {searchQuery && filteredSessions.length === 0 && (
            <div style={{ padding: '12px', color: 'var(--k8s-text-muted)', fontSize: 12, textAlign: 'center' }}>
              未找到匹配的会话
            </div>
          )}
        </div>
      </div>

      {/* 右侧对话区 */}
      <div style={styles.main}>
        {currentSession ? (
          <>
            <div style={styles.messages}>
              {records.length === 0 && !loading && (
                <div style={styles.empty}>开始提问，AI 将结合集群诊断数据为你解答</div>
              )}
              {records.map((record, index) => (
                <div key={index} style={styles.messageGroup}>
                  <div style={styles.question}>{record.question}</div>
                  <div style={styles.answer}>
                    {record.llm_available ? (
                      <div className="markdown-body">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{record.llm_response}</ReactMarkdown>
                      </div>
                    ) : (
                      <div style={styles.warning}>
                        LLM 分析不可用，显示原始数据：
                        <pre style={{ marginTop: 8, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{record.grpc_response}</pre>
                      </div>
                    )}
                  </div>
                </div>
              ))}
              {loading && (
                <div style={styles.messageGroup}>
                  <div style={styles.question}>{pendingQuestion.current}</div>
                  <div style={{ ...styles.answer, color: 'var(--k8s-text-muted)', fontStyle: 'italic' }}>
                    正在分析集群数据，请稍候...
                  </div>
                </div>
              )}
              {errorInfo && (
                <div style={{
                  padding: '14px 18px',
                  borderRadius: '8px',
                  border: '2px solid #f44336',
                  background: '#ffebee',
                  marginTop: 8,
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600, fontSize: 14, color: '#c62828', marginBottom: 6 }}>
                    <span>❌</span> 提问失败
                  </div>
                  <div style={{ fontSize: 13, color: '#333', lineHeight: 1.6 }}>{errorInfo.message}</div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>
            <div style={styles.inputArea}>
              <input
                style={styles.input}
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                onCompositionStart={() => { isComposingRef.current = true; }}
                onCompositionEnd={() => { isComposingRef.current = false; }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    if (!isComposingRef.current) submitQuery();
                  }
                }}
                placeholder="输入您的诊断问题..."
                disabled={loading}
              />
              <button style={styles.sendBtn} onClick={submitQuery} disabled={loading || !question.trim()}>
                {loading ? '诊断中...' : '发送'}
              </button>
            </div>
          </>
        ) : (
          <div style={styles.welcomeContainer}>
            <div style={styles.welcomeContent}>
              <div style={styles.welcomeIcon}>
                <Sparkles size={36} color="var(--k8s-blue)" />
              </div>
              <h1 style={styles.welcomeTitle}>有什么可以帮你的？</h1>
              <p style={styles.welcomeSubtitle}>AI 智能诊断助手，帮你快速分析 Kubernetes 集群问题</p>
              <div style={styles.welcomeInputWrapper}>
                <input
                  style={styles.welcomeInput}
                  value={question}
                  onChange={(e) => setQuestion(e.target.value)}
                  onCompositionStart={() => { isComposingRef.current = true; }}
                  onCompositionEnd={() => { isComposingRef.current = false; }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      if (!isComposingRef.current) submitFromWelcome();
                    }
                  }}
                  placeholder="输入你的问题，开始诊断..."
                />
                <button
                  style={{
                    ...styles.welcomeSendBtn,
                    opacity: question.trim() ? 1 : 0.4,
                    cursor: question.trim() ? 'pointer' : 'default',
                  }}
                  onClick={submitFromWelcome}
                  disabled={!question.trim()}
                >
                  <Send size={18} />
                </button>
              </div>
              <div style={styles.suggestionsGrid}>
                {suggestedQuestions.map((item, i) => (
                  <button
                    key={i}
                    style={styles.suggestionCard}
                    onClick={() => { setQuestion(item.text); }}
                    onMouseEnter={e => {
                      (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--k8s-blue)';
                      (e.currentTarget as HTMLButtonElement).style.background = 'var(--k8s-blue-light, #eff6ff)';
                    }}
                    onMouseLeave={e => {
                      (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--k8s-border)';
                      (e.currentTarget as HTMLButtonElement).style.background = 'var(--k8s-card-bg, #fff)';
                    }}
                  >
                    <span style={{ fontSize: '18px' }}>{item.icon}</span>
                    <span style={{ fontSize: '13px', color: 'var(--k8s-text-primary)' }}>{item.text}</span>
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
  container: {
    display: 'flex',
    gap: '16px',
    height: 'calc(100vh - 100px)',
  },
  sidebar: {
    width: '240px',
    background: 'var(--k8s-card-bg)',
    borderRadius: 'var(--k8s-card-radius)',
    padding: '14px',
    border: '1px solid var(--k8s-border)',
    display: 'flex',
    flexDirection: 'column',
  },
  sidebarHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '12px',
    padding: '0 2px',
  },
  headerBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: '6px 8px',
    display: 'flex',
    alignItems: 'center',
    gap: '5px',
    borderRadius: '6px',
    color: 'var(--k8s-text-primary)',
    fontSize: '13px',
    transition: 'background 0.15s',
  },
  searchInput: {
    width: '100%',
    padding: '7px 10px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '12px',
    outline: 'none',
    marginBottom: '10px',
    boxSizing: 'border-box',
  },
  sessionList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '2px',
    overflow: 'auto',
    flex: 1,
  },
  sessionItem: {
    padding: '9px 10px',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    transition: 'background 0.15s',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: '4px',
    minHeight: '36px',
  },
  sessionTitle: {
    flex: 1,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  sessionActions: {
    display: 'flex',
    gap: '2px',
    flexShrink: 0,
  },
  renameRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '4px',
    width: '100%',
  },
  renameInput: {
    flex: 1,
    padding: '3px 6px',
    border: '1px solid var(--k8s-blue)',
    borderRadius: '3px',
    fontSize: '12px',
    outline: 'none',
    minWidth: 0,
  },
  iconBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    padding: '2px',
    display: 'flex',
    alignItems: 'center',
    borderRadius: '3px',
    color: 'inherit',
  },
  main: {
    flex: 1,
    background: 'var(--k8s-card-bg)',
    borderRadius: 'var(--k8s-card-radius)',
    padding: '20px',
    border: '1px solid var(--k8s-border)',
    display: 'flex',
    flexDirection: 'column',
  },
  messages: {
    flex: 1,
    overflow: 'auto',
    marginBottom: '14px',
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
  },
  messageGroup: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  },
  question: {
    background: 'var(--k8s-blue-light)',
    padding: '10px 14px',
    borderRadius: '4px',
    fontSize: '13px',
    color: 'var(--k8s-text-primary)',
    alignSelf: 'flex-end',
    maxWidth: '80%',
  },
  answer: {
    background: '#f8f9fa',
    padding: '14px',
    borderRadius: '4px',
    fontSize: '13px',
    lineHeight: '1.7',
    borderLeft: '3px solid var(--k8s-blue)',
    minWidth: 0,
    overflow: 'hidden',
  },
  warning: {
    background: '#fff8e1',
    padding: '10px 14px',
    borderRadius: '4px',
    color: 'var(--k8s-warning)',
    fontSize: '13px',
  },
  inputArea: {
    display: 'flex',
    gap: '10px',
  },
  input: {
    flex: 1,
    padding: '10px 12px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '13px',
    outline: 'none',
  },
  sendBtn: {
    padding: '10px 20px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  empty: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100%',
    color: 'var(--k8s-text-muted)',
    fontSize: '14px',
  },
  welcomeContainer: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100%',
    width: '100%',
  },
  welcomeContent: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    maxWidth: '600px',
    width: '100%',
    padding: '0 20px',
  },
  welcomeIcon: {
    width: '64px',
    height: '64px',
    borderRadius: '50%',
    background: 'var(--k8s-blue-light, #eff6ff)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: '20px',
  },
  welcomeTitle: {
    fontSize: '24px',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    margin: '0 0 8px 0',
  },
  welcomeSubtitle: {
    fontSize: '14px',
    color: 'var(--k8s-text-muted)',
    margin: '0 0 32px 0',
  },
  welcomeInputWrapper: {
    display: 'flex',
    alignItems: 'center',
    width: '100%',
    border: '1px solid var(--k8s-border)',
    borderRadius: '12px',
    padding: '4px 4px 4px 16px',
    background: 'var(--k8s-card-bg, #fff)',
    boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
    transition: 'border-color 0.2s, box-shadow 0.2s',
  },
  welcomeInput: {
    flex: 1,
    border: 'none',
    outline: 'none',
    fontSize: '14px',
    padding: '10px 0',
    background: 'transparent',
    color: 'var(--k8s-text-primary)',
  },
  welcomeSendBtn: {
    width: '36px',
    height: '36px',
    borderRadius: '8px',
    border: 'none',
    background: 'var(--k8s-blue)',
    color: 'white',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    transition: 'opacity 0.2s',
  },
  suggestionsGrid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '10px',
    width: '100%',
    marginTop: '28px',
  },
  suggestionCard: {
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    padding: '12px 14px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '10px',
    background: 'var(--k8s-card-bg, #fff)',
    cursor: 'pointer',
    transition: 'border-color 0.2s, background 0.2s',
    textAlign: 'left',
  },
};

export default DiagnosisPage;

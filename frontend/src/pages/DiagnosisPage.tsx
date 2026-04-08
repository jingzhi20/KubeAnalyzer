import { useState, useEffect, useRef } from 'react';
import { diagnosisApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { DiagnosisSession, DiagnosisRecord } from '../types';
import { Trash2, Pencil, Check, X } from 'lucide-react';

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
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadSessions();
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [records]);

  const loadSessions = async () => {
    try {
      const response = await diagnosisApi.listSessions();
      setSessions(response.data);
    } catch (err) {
      console.error('Failed to load sessions:', err);
    }
  };

  const createSession = async () => {
    try {
      const response = await diagnosisApi.createSession({ title: '新诊断会话' });
      const newSession = response.data;
      setSessions(prev => [newSession, ...prev]);
      setCurrentSession(newSession);
      setRecords([]);
      setErrorInfo(null);
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
    setLoading(true);
    setErrorInfo(null);
    try {
      const response = await diagnosisApi.submitQuery(currentSession.id, { question });
      setRecords(prev => [...prev, response.data]);
      setQuestion('');
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

  return (
    <div style={styles.container}>
      {/* 左侧会话列表 */}
      <div style={styles.sidebar}>
        <button style={styles.newBtn} onClick={createSession}>+ 新建会话</button>
        <div style={styles.sessionList}>
          {sessions.map(session => {
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
                      <ReactMarkdown>{record.llm_response}</ReactMarkdown>
                    ) : (
                      <div style={styles.warning}>
                        LLM 分析不可用，显示原始数据：
                        <pre style={{ marginTop: 8, fontSize: 12 }}>{record.grpc_response}</pre>
                      </div>
                    )}
                  </div>
                </div>
              ))}
              {loading && (
                <div style={styles.messageGroup}>
                  <div style={styles.question}>{question}</div>
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
                onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && submitQuery()}
                placeholder="输入您的诊断问题..."
                disabled={loading}
              />
              <button style={styles.sendBtn} onClick={submitQuery} disabled={loading || !question.trim()}>
                {loading ? '诊断中...' : '发送'}
              </button>
            </div>
          </>
        ) : (
          <div style={styles.empty}>请选择或创建一个诊断会话</div>
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
  newBtn: {
    width: '100%',
    padding: '9px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    marginBottom: '14px',
    fontSize: '13px',
    fontWeight: 500,
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
};

export default DiagnosisPage;

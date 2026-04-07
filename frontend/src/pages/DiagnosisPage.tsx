import { useState, useEffect } from 'react';
import { diagnosisApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { DiagnosisSession, DiagnosisRecord } from '../types';

function DiagnosisPage() {
  const [sessions, setSessions] = useState<DiagnosisSession[]>([]);
  const [currentSession, setCurrentSession] = useState<DiagnosisSession | null>(null);
  const [question, setQuestion] = useState('');
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState<DiagnosisRecord[]>([]);

  useEffect(() => {
    loadSessions();
  }, []);

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
      setCurrentSession(response.data);
      setRecords([]);
      loadSessions();
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  };

  const submitQuery = async () => {
    if (!currentSession || !question.trim()) return;
    
    setLoading(true);
    try {
      const response = await diagnosisApi.submitQuery(currentSession.id, { question });
      setRecords([...records, response.data]);
      setQuestion('');
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '提交失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.sidebar}>
        <button style={styles.newBtn} onClick={createSession}>+ 新建会话</button>
        <div style={styles.sessionList}>
          {sessions.map(session => (
            <div
              key={session.id}
              style={{
                ...styles.sessionItem,
                background: currentSession?.id === session.id ? 'var(--k8s-blue)' : 'var(--k8s-card-bg)',
                color: currentSession?.id === session.id ? 'white' : 'var(--k8s-text-primary)',
              }}
              onClick={() => setCurrentSession(session)}
            >
              {session.title}
            </div>
          ))}
        </div>
      </div>

      <div style={styles.main}>
        {currentSession ? (
          <>
            <div style={styles.messages}>
              {records.map((record, index) => (
                <div key={index} style={styles.messageGroup}>
                  <div style={styles.question}>{record.question}</div>
                  <div style={styles.answer}>
                    {record.llm_available ? (
                      <ReactMarkdown>{record.llm_response}</ReactMarkdown>
                    ) : (
                      <div style={styles.warning}>
                        LLM分析不可用，显示原始数据：
                        <pre>{record.grpc_response}</pre>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
            <div style={styles.inputArea}>
              <input
                style={styles.input}
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && submitQuery()}
                placeholder="输入您的诊断问题..."
                disabled={loading}
              />
              <button style={styles.sendBtn} onClick={submitQuery} disabled={loading}>
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
    gap: '4px',
  },
  sessionItem: {
    padding: '10px 12px',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    transition: 'all 0.15s',
    border: '1px solid transparent',
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
  },
  messageGroup: {
    marginBottom: '16px',
  },
  question: {
    background: 'var(--k8s-blue-light)',
    padding: '10px 14px',
    borderRadius: '4px',
    marginBottom: '8px',
    fontSize: '13px',
    color: 'var(--k8s-text-primary)',
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

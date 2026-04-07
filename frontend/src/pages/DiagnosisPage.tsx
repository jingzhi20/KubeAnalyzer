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
                background: currentSession?.id === session.id ? '#667eea' : 'white',
                color: currentSession?.id === session.id ? 'white' : '#333',
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
    gap: '20px',
    height: 'calc(100vh - 100px)',
  },
  sidebar: {
    width: '250px',
    background: 'white',
    borderRadius: '10px',
    padding: '16px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
  },
  newBtn: {
    width: '100%',
    padding: '10px',
    background: '#667eea',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    marginBottom: '16px',
  },
  sessionList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  },
  sessionItem: {
    padding: '12px',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
    transition: 'all 0.2s',
  },
  main: {
    flex: 1,
    background: 'white',
    borderRadius: '10px',
    padding: '24px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
    display: 'flex',
    flexDirection: 'column',
  },
  messages: {
    flex: 1,
    overflow: 'auto',
    marginBottom: '16px',
  },
  messageGroup: {
    marginBottom: '20px',
  },
  question: {
    background: '#f0f0f0',
    padding: '12px',
    borderRadius: '8px',
    marginBottom: '8px',
    fontSize: '14px',
  },
  answer: {
    background: '#f8f9fa',
    padding: '16px',
    borderRadius: '8px',
    fontSize: '14px',
    lineHeight: '1.6',
  },
  warning: {
    background: '#fff3cd',
    padding: '12px',
    borderRadius: '6px',
    color: '#856404',
  },
  inputArea: {
    display: 'flex',
    gap: '12px',
  },
  input: {
    flex: 1,
    padding: '12px',
    border: '1px solid #ddd',
    borderRadius: '6px',
    fontSize: '14px',
  },
  sendBtn: {
    padding: '12px 24px',
    background: '#667eea',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
  empty: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100%',
    color: '#999',
    fontSize: '16px',
  },
};

export default DiagnosisPage;

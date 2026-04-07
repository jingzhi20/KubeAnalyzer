import { useState, useEffect } from 'react';
import { inspectionApi, clusterApi } from '../api';
import ReactMarkdown from 'react-markdown';
import type { InspectionTask, InspectionRule, ScriptResult, ClusterConfig } from '../types';

const RULE_TYPE_LABELS: Record<string, string> = { builtin: '内置', command: '命令行', script: '脚本' };
const SCRIPT_TYPE_OPTIONS = ['bash', 'python', 'perl'];
const emptyRule: Partial<InspectionRule> = {
  name: '', rule_type: 'command', resource_type: '', check_items: '',
  command: '', script: '', script_type: 'bash', timeout: 60,
  target_nodes: '', namespaces: '', cluster_id: 0, cron_expression: '', enabled: true,
};

function InspectionPage() {
  const [activeTab, setActiveTab] = useState<'default' | 'custom' | 'history'>('default');
  const [tasks, setTasks] = useState<InspectionTask[]>([]);
  const [selectedTask, setSelectedTask] = useState<InspectionTask | null>(null);
  const [rules, setRules] = useState<InspectionRule[]>([]);
  const [clusters, setClusters] = useState<ClusterConfig[]>([]);
  const [showRuleForm, setShowRuleForm] = useState(false);
  const [editingRule, setEditingRule] = useState<Partial<InspectionRule>>(emptyRule);
  const [editingRuleId, setEditingRuleId] = useState<number | null>(null);
  const [triggering, setTriggering] = useState<number | null>(null); // rule ID being triggered

  const defaultRules = rules.filter(r => r.is_default);
  const customRules = rules.filter(r => !r.is_default);

  useEffect(() => { loadAll(); }, []);

  const loadAll = () => { loadTasks(); loadRules(); loadClusters(); };
  const loadTasks = async () => {
    try { setTasks((await inspectionApi.listTasks()).data); } catch (e) { console.error(e); }
  };
  const loadRules = async () => {
    try { setRules((await inspectionApi.listRules()).data); } catch (e) { console.error(e); }
  };
  const loadClusters = async () => {
    try { setClusters((await clusterApi.list()).data); } catch (e) { console.error(e); }
  };

  const saveRuleConfig = async (rule: InspectionRule) => {
    try {
      await inspectionApi.updateRule(rule.id, rule);
      loadRules();
    } catch (err: any) { alert(err.response?.data?.error?.message || '保存失败'); }
  };

  const toggleRule = async (rule: InspectionRule) => {
    await saveRuleConfig({ ...rule, enabled: !rule.enabled });
  };

  const triggerRule = async (rule: InspectionRule) => {
    setTriggering(rule.id);
    try {
      await inspectionApi.trigger(rule.cluster_id || undefined);
      loadTasks();
      alert(`巡检已触发：${rule.name}`);
    } catch (err: any) { alert(err.response?.data?.error?.message || '触发失败'); }
    finally { setTriggering(null); }
  };

  const openCreateRule = () => {
    setEditingRule({ ...emptyRule }); setEditingRuleId(null); setShowRuleForm(true);
  };
  const openEditRule = (rule: InspectionRule) => {
    setEditingRule({ ...rule }); setEditingRuleId(rule.id); setShowRuleForm(true);
  };
  const saveRule = async () => {
    try {
      if (editingRuleId) await inspectionApi.updateRule(editingRuleId, editingRule);
      else await inspectionApi.createRule(editingRule);
      setShowRuleForm(false); loadRules();
    } catch (err: any) { alert(err.response?.data?.error?.message || '保存失败'); }
  };
  const deleteRule = async (id: number) => {
    if (!confirm('确定删除该规则？')) return;
    try { await inspectionApi.deleteRule(id); loadRules(); } catch { alert('删除失败'); }
  };

  const parseScriptResults = (task: InspectionTask): ScriptResult[] => {
    if (!task.script_results) return [];
    try { return JSON.parse(task.script_results); } catch { return []; }
  };

  const getClusterName = (id: number) => {
    if (!id) return '活跃集群';
    return clusters.find(c => c.id === id)?.name || `集群#${id}`;
  };

  // Inline editable config for a default rule
  const RuleConfigCard = ({ rule }: { rule: InspectionRule }) => {
    const [localRule, setLocalRule] = useState(rule);
    const [dirty, setDirty] = useState(false);

    useEffect(() => { setLocalRule(rule); setDirty(false); }, [rule]);

    const update = (patch: Partial<InspectionRule>) => {
      setLocalRule(prev => ({ ...prev, ...patch }));
      setDirty(true);
    };

    return (
      <div style={{ ...S.ruleCard, borderLeft: `4px solid ${localRule.enabled ? '#4caf50' : '#ddd'}` }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <span style={{ fontWeight: 600, fontSize: '15px' }}>{localRule.name}</span>
            <span style={S.badge}>{RULE_TYPE_LABELS[localRule.rule_type]}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <label style={S.switchLabel}>
              <input type="checkbox" checked={localRule.enabled} onChange={() => { update({ enabled: !localRule.enabled }); }} />
              {localRule.enabled ? '已开启' : '已关闭'}
            </label>
          </div>
        </div>

        <div style={S.configGrid}>
          <div style={S.configItem}>
            <label style={S.configLabel}>定时执行</label>
            <input style={S.configInput} value={localRule.cron_expression || ''} onChange={e => update({ cron_expression: e.target.value })} placeholder="Cron（如 0 */2 * * *）留空=仅手动" />
          </div>
          <div style={S.configItem}>
            <label style={S.configLabel}>集群范围</label>
            <select style={S.configInput} value={localRule.cluster_id || 0} onChange={e => update({ cluster_id: Number(e.target.value) })}>
              <option value={0}>当前活跃集群</option>
              {clusters.map(c => <option key={c.id} value={c.id}>{c.name}{c.is_active ? ' (活跃)' : ''}</option>)}
            </select>
          </div>
          <div style={S.configItem}>
            <label style={S.configLabel}>目标节点</label>
            <input style={S.configInput} value={localRule.target_nodes || ''} onChange={e => update({ target_nodes: e.target.value })} placeholder="留空=全部节点" />
          </div>
          <div style={S.configItem}>
            <label style={S.configLabel}>命名空间</label>
            <input style={S.configInput} value={localRule.namespaces || ''} onChange={e => update({ namespaces: e.target.value })} placeholder="留空=脚本默认" />
          </div>
        </div>

        <div style={{ display: 'flex', gap: '8px', marginTop: '12px', justifyContent: 'flex-end' }}>
          {dirty && <button style={S.primaryBtn} onClick={() => { saveRuleConfig(localRule as InspectionRule); setDirty(false); }}>保存配置</button>}
          <button style={S.successBtn} onClick={() => triggerRule(localRule as InspectionRule)} disabled={triggering === rule.id}>
            {triggering === rule.id ? '执行中...' : '立即执行'}
          </button>
        </div>
      </div>
    );
  };

  return (
    <div>
      <h1 style={S.title}>集群巡检管理</h1>

      {/* Tabs */}
      <div style={S.tabBar}>
        {(['default', 'custom', 'history'] as const).map(tab => (
          <button key={tab} style={activeTab === tab ? S.tabActive : S.tab} onClick={() => setActiveTab(tab)}>
            {{ default: '默认巡检', custom: '自定义巡检', history: '巡检记录' }[tab]}
          </button>
        ))}
      </div>

      {/* Tab: 默认巡检 */}
      {activeTab === 'default' && (
        <div>
          {defaultRules.length === 0 && <div style={S.card}><div style={S.empty}>暂无默认巡检规则（数据库初始化后自动生成）</div></div>}
          {defaultRules.map(rule => <RuleConfigCard key={rule.id} rule={rule} />)}
        </div>
      )}

      {/* Tab: 自定义巡检 */}
      {activeTab === 'custom' && (
        <div style={S.card}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <span style={{ fontSize: '14px', color: '#666' }}>共 {customRules.length} 条自定义规则</span>
            <button style={S.primaryBtn} onClick={openCreateRule}>+ 新建规则</button>
          </div>
          {customRules.length === 0 && <div style={S.empty}>暂无自定义规则，点击上方按钮创建</div>}
          {customRules.map(rule => (
            <div key={rule.id} style={{ ...S.ruleRow, opacity: rule.enabled ? 1 : 0.5 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flex: 1, minWidth: 0 }}>
                <input type="checkbox" checked={rule.enabled} onChange={() => toggleRule(rule)} />
                <span style={{ fontWeight: 500, fontSize: '14px' }}>{rule.name}</span>
                <span style={S.badge}>{RULE_TYPE_LABELS[rule.rule_type]}</span>
                {rule.cron_expression && <span style={S.meta}>定时: {rule.cron_expression}</span>}
                {rule.cluster_id > 0 && <span style={S.meta}>集群: {getClusterName(rule.cluster_id)}</span>}
              </div>
              <div style={{ display: 'flex', gap: '4px' }}>
                <button style={S.linkBtn} onClick={() => triggerRule(rule)}>执行</button>
                <button style={S.linkBtn} onClick={() => openEditRule(rule)}>编辑</button>
                <button style={{ ...S.linkBtn, color: '#f44336' }} onClick={() => deleteRule(rule.id)}>删除</button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Tab: 巡检记录 */}
      {activeTab === 'history' && (
        <div style={S.card}>
          {tasks.length === 0 && <div style={S.empty}>暂无巡检记录</div>}
          {tasks.map(task => (
            <div key={task.id} style={S.taskCard} onClick={() => setSelectedTask(selectedTask?.id === task.id ? null : task)}>
              <div style={S.taskHeader}>
                <span style={{ fontSize: '14px', color: '#666' }}>{new Date(task.started_at).toLocaleString()}</span>
                {task.cluster_name && <span style={S.badge}>{task.cluster_name}</span>}
                <span style={{
                  ...S.badge,
                  background: task.status === 'completed' ? '#e8f5e9' : task.status === 'failed' ? '#fee' : '#fff3cd',
                  color: task.status === 'completed' ? '#4caf50' : task.status === 'failed' ? '#f44336' : '#ff9800',
                }}>{task.status === 'completed' ? '完成' : task.status === 'failed' ? '失败' : '运行中'}</span>
                <span style={S.badge}>{task.trigger_type === 'manual' ? '手动' : '定时'}</span>
                <span style={S.meta}>异常: {task.anomaly_count}</span>
              </div>
              {selectedTask?.id === task.id && (
                <div style={{ marginTop: '16px', paddingTop: '16px', borderTop: '1px solid #eee' }}>
                  {task.status === 'failed' ? (
                    <div style={S.errorBox}>{task.error_message}</div>
                  ) : (
                    <>
                      <div style={{ marginBottom: '16px' }}>
                        <div style={{ fontSize: '13px', color: '#888', marginBottom: '6px' }}>LLM 巡检摘要</div>
                        <div style={S.markdownBox}><ReactMarkdown>{task.llm_summary || '暂无摘要'}</ReactMarkdown></div>
                      </div>
                      {parseScriptResults(task).length > 0 && (
                        <div>
                          <div style={{ fontSize: '13px', color: '#888', marginBottom: '6px' }}>脚本执行结果</div>
                          {parseScriptResults(task).map((sr, i) => (
                            <div key={i} style={S.scriptCard}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                                <span style={{ fontWeight: 500 }}>{sr.rule_name}</span>
                                <span style={{ ...S.badge, background: sr.exit_code === 0 ? '#e8f5e9' : '#fee', color: sr.exit_code === 0 ? '#4caf50' : '#f44336' }}>退出码: {sr.exit_code}</span>
                                <span style={S.meta}>{sr.duration}</span>
                              </div>
                              {sr.stdout && <pre style={S.codeBlock}>{sr.stdout}</pre>}
                              {sr.stderr && <pre style={{ ...S.codeBlock, borderColor: '#f44336' }}>{sr.stderr}</pre>}
                              {sr.error && <div style={S.errorBox}>{sr.error}</div>}
                            </div>
                          ))}
                        </div>
                      )}
                    </>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Rule Form Modal */}
      {showRuleForm && (
        <div style={S.overlay}>
          <div style={S.modal}>
            <h3 style={{ marginTop: 0 }}>{editingRuleId ? '编辑规则' : '新建自定义规则'}</h3>
            <div style={S.fg}><label style={S.fl}>规则名称</label><input style={S.fi} value={editingRule.name || ''} onChange={e => setEditingRule({ ...editingRule, name: e.target.value })} /></div>
            <div style={S.fg}><label style={S.fl}>规则类型</label>
              <select style={S.fi} value={editingRule.rule_type || 'command'} onChange={e => setEditingRule({ ...editingRule, rule_type: e.target.value as any })}>
                <option value="builtin">内置</option><option value="command">命令行</option><option value="script">脚本</option>
              </select>
            </div>
            {editingRule.rule_type === 'command' && (
              <div style={S.fg}><label style={S.fl}>Shell 命令</label><textarea style={S.ft} value={editingRule.command || ''} onChange={e => setEditingRule({ ...editingRule, command: e.target.value })} rows={3} /></div>
            )}
            {editingRule.rule_type === 'script' && (
              <>
                <div style={S.fg}><label style={S.fl}>脚本类型</label>
                  <select style={S.fi} value={editingRule.script_type || 'bash'} onChange={e => setEditingRule({ ...editingRule, script_type: e.target.value as any })}>
                    {SCRIPT_TYPE_OPTIONS.map(t => <option key={t} value={t}>{t}</option>)}
                  </select>
                </div>
                <div style={S.fg}><label style={S.fl}>脚本内容</label><textarea style={{ ...S.ft, fontFamily: 'monospace' }} value={editingRule.script || ''} onChange={e => setEditingRule({ ...editingRule, script: e.target.value })} rows={8} /></div>
              </>
            )}
            {editingRule.rule_type === 'builtin' && (
              <>
                <div style={S.fg}><label style={S.fl}>资源类型</label><input style={S.fi} value={editingRule.resource_type || ''} onChange={e => setEditingRule({ ...editingRule, resource_type: e.target.value })} /></div>
                <div style={S.fg}><label style={S.fl}>检查项</label><textarea style={S.ft} value={editingRule.check_items || ''} onChange={e => setEditingRule({ ...editingRule, check_items: e.target.value })} rows={3} /></div>
              </>
            )}
            <div style={S.fg}><label style={S.fl}>集群</label>
              <select style={S.fi} value={editingRule.cluster_id || 0} onChange={e => setEditingRule({ ...editingRule, cluster_id: Number(e.target.value) })}>
                <option value={0}>当前活跃集群</option>
                {clusters.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
            </div>
            <div style={S.fg}><label style={S.fl}>定时执行（Cron）</label><input style={S.fi} value={editingRule.cron_expression || ''} onChange={e => setEditingRule({ ...editingRule, cron_expression: e.target.value })} placeholder="留空=仅手动触发" /></div>
            <div style={S.fg}><label style={S.fl}>目标节点</label><input style={S.fi} value={editingRule.target_nodes || ''} onChange={e => setEditingRule({ ...editingRule, target_nodes: e.target.value })} placeholder="逗号分隔，留空=全部" /></div>
            <div style={S.fg}><label style={S.fl}>命名空间</label><input style={S.fi} value={editingRule.namespaces || ''} onChange={e => setEditingRule({ ...editingRule, namespaces: e.target.value })} placeholder="逗号分隔，留空=默认" /></div>
            <div style={S.fg}><label style={S.fl}>超时（秒）</label><input style={S.fi} type="number" value={editingRule.timeout || 60} onChange={e => setEditingRule({ ...editingRule, timeout: parseInt(e.target.value) || 60 })} /></div>
            <div style={S.fg}><label style={S.switchLabel}><input type="checkbox" checked={editingRule.enabled ?? true} onChange={e => setEditingRule({ ...editingRule, enabled: e.target.checked })} />启用</label></div>
            <div style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end', marginTop: '20px' }}>
              <button style={S.cancelBtn} onClick={() => setShowRuleForm(false)}>取消</button>
              <button style={S.primaryBtn} onClick={saveRule}>保存</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

const S: { [k: string]: React.CSSProperties } = {
  title: { fontSize: '28px', color: '#333', marginBottom: '24px' },
  tabBar: { display: 'flex', gap: '4px', marginBottom: '20px' },
  tab: { padding: '10px 28px', background: '#f5f5f5', border: 'none', borderRadius: '8px 8px 0 0', cursor: 'pointer', fontSize: '14px', color: '#666' },
  tabActive: { padding: '10px 28px', background: '#667eea', border: 'none', borderRadius: '8px 8px 0 0', cursor: 'pointer', fontSize: '14px', color: 'white', fontWeight: 500 },
  card: { background: 'white', padding: '24px', borderRadius: '10px', marginBottom: '16px', boxShadow: '0 2px 8px rgba(0,0,0,0.08)' },
  ruleCard: { background: 'white', padding: '20px', borderRadius: '10px', marginBottom: '16px', boxShadow: '0 2px 8px rgba(0,0,0,0.08)', borderLeft: '4px solid #4caf50' },
  configGrid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' },
  configItem: {},
  configLabel: { display: 'block', fontSize: '12px', color: '#888', marginBottom: '4px' },
  configInput: { width: '100%', padding: '8px 10px', border: '1px solid #e0e0e0', borderRadius: '6px', fontSize: '13px', boxSizing: 'border-box' as const },
  ruleRow: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 16px', border: '1px solid #f0f0f0', borderRadius: '8px', marginBottom: '8px' },
  badge: { padding: '2px 10px', borderRadius: '12px', fontSize: '12px', fontWeight: 500, background: '#f0f0f0', color: '#555', whiteSpace: 'nowrap' as const },
  meta: { fontSize: '12px', color: '#999', whiteSpace: 'nowrap' as const },
  primaryBtn: { padding: '8px 18px', background: '#667eea', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '13px', whiteSpace: 'nowrap' as const },
  successBtn: { padding: '8px 18px', background: '#4caf50', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '13px', whiteSpace: 'nowrap' as const },
  cancelBtn: { padding: '8px 18px', background: '#f5f5f5', color: '#666', border: '1px solid #ddd', borderRadius: '6px', cursor: 'pointer', fontSize: '13px' },
  linkBtn: { background: 'none', border: 'none', color: '#667eea', cursor: 'pointer', fontSize: '13px', padding: '4px 8px' },
  switchLabel: { display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', cursor: 'pointer', whiteSpace: 'nowrap' as const },
  taskCard: { padding: '16px', border: '1px solid #eee', borderRadius: '8px', marginBottom: '12px', cursor: 'pointer' },
  taskHeader: { display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' as const },
  markdownBox: { background: '#f8f9fa', padding: '16px', borderRadius: '8px', fontSize: '14px', lineHeight: '1.6' },
  scriptCard: { background: '#f8f9fa', padding: '16px', borderRadius: '8px', marginBottom: '12px' },
  codeBlock: { background: '#1e1e1e', color: '#d4d4d4', padding: '12px', borderRadius: '6px', fontSize: '13px', fontFamily: 'monospace', overflow: 'auto', maxHeight: '300px', whiteSpace: 'pre-wrap' as const, border: '1px solid #333' },
  errorBox: { background: '#fee', color: '#c33', padding: '12px', borderRadius: '6px' },
  empty: { textAlign: 'center' as const, color: '#999', padding: '32px', fontSize: '14px' },
  overlay: { position: 'fixed' as const, top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 },
  modal: { background: 'white', padding: '32px', borderRadius: '12px', width: '560px', maxHeight: '80vh', overflow: 'auto', boxShadow: '0 8px 32px rgba(0,0,0,0.2)' },
  fg: { marginBottom: '14px' },
  fl: { display: 'block', fontSize: '13px', color: '#555', marginBottom: '4px' },
  fi: { width: '100%', padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px', boxSizing: 'border-box' as const },
  ft: { width: '100%', padding: '10px', border: '1px solid #ddd', borderRadius: '6px', fontSize: '14px', boxSizing: 'border-box' as const, resize: 'vertical' as const },
};

export default InspectionPage;

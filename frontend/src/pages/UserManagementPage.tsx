import { useState, useEffect } from 'react';
import { userApi } from '../api';
import type { UserInfo, CreateUserRequest, UpdateRoleRequest } from '../types';

function UserManagementPage() {
  const [users, setUsers] = useState<UserInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingUserId, setEditingUserId] = useState<number | null>(null);
  const [newUser, setNewUser] = useState<CreateUserRequest>({
    username: '',
    password: '',
    display_name: '',
    role: 'user',
  });

  useEffect(() => {
    loadUsers();
  }, []);

  const loadUsers = async () => {
    setLoading(true);
    try {
      const response = await userApi.list();
      setUsers(response.data);
    } catch (err) {
      console.error('Failed to load users:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await userApi.create(newUser);
      setShowCreateModal(false);
      setNewUser({ username: '', password: '', display_name: '', role: 'user' });
      loadUsers();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '创建用户失败');
    }
  };

  const handleUpdateRole = async (userId: number, currentRole: string) => {
    const newRole: UpdateRoleRequest = {
      role: currentRole === 'admin' ? 'user' : 'admin',
    };
    try {
      setEditingUserId(userId);
      await userApi.updateRole(userId, newRole);
      loadUsers();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '更新角色失败');
    } finally {
      setEditingUserId(null);
    }
  };

  const handleDeleteUser = async (userId: number, username: string) => {
    if (!confirm(`确定要删除用户 "${username}" 吗？`)) {
      return;
    }
    try {
      await userApi.delete(userId);
      loadUsers();
    } catch (err: any) {
      alert(err.response?.data?.error?.message || '删除用户失败');
    }
  };

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h1 style={styles.title}>用户管理</h1>
        <button style={styles.createBtn} onClick={() => setShowCreateModal(true)}>
          + 新增用户
        </button>
      </div>

      {loading ? (
        <div style={styles.loading}>加载中...</div>
      ) : (
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>用户名</th>
              <th style={styles.th}>显示名称</th>
              <th style={styles.th}>角色</th>
              <th style={styles.th}>登录方式</th>
              <th style={styles.th}>创建时间</th>
              <th style={styles.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {users.map(user => (
              <tr key={user.id} style={styles.tr}>
                <td style={styles.td}>{user.username}</td>
                <td style={styles.td}>{user.display_name || '-'}</td>
                <td style={styles.td}>
                  <span style={{
                    ...styles.badge,
                    background: user.role === 'admin' ? '#fee' : '#e8f5e9',
                    color: user.role === 'admin' ? '#c33' : '#2e7d32',
                  }}>
                    {user.role === 'admin' ? '管理员' : '普通用户'}
                  </span>
                </td>
                <td style={styles.td}>
                  <span style={styles.loginMethod}>{user.login_method}</span>
                </td>
                <td style={styles.td}>{user.created_at}</td>
                <td style={styles.td}>
                  <button
                    style={styles.actionBtn}
                    onClick={() => handleUpdateRole(user.id, user.role)}
                    disabled={editingUserId === user.id}
                  >
                    {editingUserId === user.id ? '更新中...' : '编辑角色'}
                  </button>
                  <button
                    style={{ ...styles.actionBtn, ...styles.deleteBtn }}
                    onClick={() => handleDeleteUser(user.id, user.username)}
                  >
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {/* Create User Modal */}
      {showCreateModal && (
        <div style={styles.modalOverlay} onClick={() => setShowCreateModal(false)}>
          <div style={styles.modal} onClick={e => e.stopPropagation()}>
            <h2 style={styles.modalTitle}>新增用户</h2>
            <form onSubmit={handleCreateUser}>
              <div style={styles.formGroup}>
                <label style={styles.label}>用户名</label>
                <input
                  type="text"
                  style={styles.input}
                  value={newUser.username}
                  onChange={e => setNewUser({ ...newUser, username: e.target.value })}
                  required
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>密码</label>
                <input
                  type="password"
                  style={styles.input}
                  value={newUser.password}
                  onChange={e => setNewUser({ ...newUser, password: e.target.value })}
                  required
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>显示名称</label>
                <input
                  type="text"
                  style={styles.input}
                  value={newUser.display_name}
                  onChange={e => setNewUser({ ...newUser, display_name: e.target.value })}
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>角色</label>
                <select
                  style={styles.input}
                  value={newUser.role}
                  onChange={e => setNewUser({ ...newUser, role: e.target.value as 'admin' | 'user' })}
                >
                  <option value="user">普通用户</option>
                  <option value="admin">管理员</option>
                </select>
              </div>
              <div style={styles.modalActions}>
                <button
                  type="button"
                  style={styles.cancelBtn}
                  onClick={() => setShowCreateModal(false)}
                >
                  取消
                </button>
                <button type="submit" style={styles.submitBtn}>
                  创建
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    background: 'var(--k8s-card-bg)',
    borderRadius: 'var(--k8s-card-radius)',
    padding: '20px',
    border: '1px solid var(--k8s-border)',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '20px',
  },
  title: {
    fontSize: '20px',
    fontWeight: 600,
    margin: 0,
    color: 'var(--k8s-text-primary)',
  },
  createBtn: {
    padding: '8px 16px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
  loading: {
    textAlign: 'center',
    padding: '36px',
    color: 'var(--k8s-text-muted)',
    fontSize: '13px',
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse' as const,
  },
  th: {
    textAlign: 'left' as const,
    padding: '10px 12px',
    borderBottom: '2px solid var(--k8s-border)',
    fontWeight: 600,
    color: 'var(--k8s-text-primary)',
    fontSize: '13px',
  },
  tr: {
    borderBottom: '1px solid var(--k8s-border-light)',
  },
  td: {
    padding: '10px 12px',
    color: 'var(--k8s-text-secondary)',
    fontSize: '13px',
  },
  badge: {
    display: 'inline-block',
    padding: '2px 10px',
    borderRadius: '3px',
    fontSize: '11px',
    fontWeight: 600,
  },
  loginMethod: {
    fontSize: '12px',
    color: 'var(--k8s-text-secondary)',
  },
  actionBtn: {
    padding: '4px 10px',
    marginRight: '6px',
    background: '#f5f5f5',
    border: 'none',
    borderRadius: '3px',
    cursor: 'pointer',
    fontSize: '12px',
  },
  deleteBtn: {
    background: '#ffebee',
    color: 'var(--k8s-danger)',
  },
  modalOverlay: {
    position: 'fixed' as const,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0,0,0,0.4)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  modal: {
    background: 'var(--k8s-card-bg)',
    padding: '24px',
    borderRadius: 'var(--k8s-card-radius)',
    maxWidth: '460px',
    width: '90%',
    maxHeight: '90vh',
    overflow: 'auto',
    border: '1px solid var(--k8s-border)',
  },
  modalTitle: {
    fontSize: '18px',
    fontWeight: 600,
    marginBottom: '20px',
    marginTop: 0,
    color: 'var(--k8s-text-primary)',
  },
  formGroup: {
    marginBottom: '14px',
  },
  label: {
    display: 'block',
    marginBottom: '4px',
    fontWeight: 500,
    color: 'var(--k8s-text-secondary)',
    fontSize: '13px',
  },
  input: {
    width: '100%',
    padding: '8px 10px',
    border: '1px solid var(--k8s-border)',
    borderRadius: '4px',
    fontSize: '13px',
    boxSizing: 'border-box' as const,
    outline: 'none',
  },
  modalActions: {
    display: 'flex',
    justifyContent: 'flex-end',
    gap: '10px',
    marginTop: '20px',
  },
  cancelBtn: {
    padding: '8px 16px',
    background: '#f5f5f5',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
  },
  submitBtn: {
    padding: '8px 16px',
    background: 'var(--k8s-blue)',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: 500,
  },
};

export default UserManagementPage;

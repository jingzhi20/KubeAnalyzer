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
    background: 'white',
    borderRadius: '8px',
    padding: '24px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '24px',
  },
  title: {
    fontSize: '24px',
    fontWeight: 'bold',
    margin: 0,
  },
  createBtn: {
    padding: '10px 20px',
    background: '#667eea',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
  loading: {
    textAlign: 'center',
    padding: '40px',
    color: '#999',
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse' as const,
  },
  th: {
    textAlign: 'left' as const,
    padding: '12px',
    borderBottom: '2px solid #e8e8e8',
    fontWeight: 'bold',
    color: '#333',
  },
  tr: {
    borderBottom: '1px solid #f0f0f0',
  },
  td: {
    padding: '12px',
    color: '#666',
  },
  badge: {
    display: 'inline-block',
    padding: '4px 12px',
    borderRadius: '12px',
    fontSize: '12px',
    fontWeight: 'bold',
  },
  loginMethod: {
    fontSize: '13px',
    color: '#666',
  },
  actionBtn: {
    padding: '6px 12px',
    marginRight: '8px',
    background: '#f0f0f0',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '13px',
  },
  deleteBtn: {
    background: '#fee',
    color: '#c33',
  },
  modalOverlay: {
    position: 'fixed' as const,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  modal: {
    background: 'white',
    padding: '32px',
    borderRadius: '12px',
    maxWidth: '500px',
    width: '90%',
    maxHeight: '90vh',
    overflow: 'auto',
  },
  modalTitle: {
    fontSize: '20px',
    fontWeight: 'bold',
    marginBottom: '24px',
    marginTop: 0,
  },
  formGroup: {
    marginBottom: '16px',
  },
  label: {
    display: 'block',
    marginBottom: '8px',
    fontWeight: 'bold',
    color: '#333',
    fontSize: '14px',
  },
  input: {
    width: '100%',
    padding: '10px',
    border: '1px solid #ddd',
    borderRadius: '6px',
    fontSize: '14px',
    boxSizing: 'border-box' as const,
  },
  modalActions: {
    display: 'flex',
    justifyContent: 'flex-end',
    gap: '12px',
    marginTop: '24px',
  },
  cancelBtn: {
    padding: '10px 20px',
    background: '#f0f0f0',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
  submitBtn: {
    padding: '10px 20px',
    background: '#667eea',
    color: 'white',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
  },
};

export default UserManagementPage;

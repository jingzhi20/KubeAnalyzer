import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import ProtectedRoute from './components/ProtectedRoute';
import LoginPage from './pages/LoginPage';
import FeishuCallbackPage from './pages/FeishuCallbackPage';
import HomePage from './pages/HomePage';
import DiagnosisPage from './pages/DiagnosisPage';
import LLMConfigPage from './pages/LLMConfigPage';
import InspectionPage from './pages/InspectionPage';
import NotificationPage from './pages/NotificationPage';
import ClusterPage from './pages/ClusterPage';
import K8sGPTPage from './pages/K8sGPTPage';
import KubectlAIPage from './pages/KubectlAIPage';
import UserManagementPage from './pages/UserManagementPage';
import FeishuSSOConfigPage from './pages/FeishuSSOConfigPage';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/auth/feishu/callback" element={<FeishuCallbackPage />} />
        <Route
          path="/app"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<HomePage />} />
          <Route path="diagnosis" element={<DiagnosisPage />} />
          <Route path="llm-configs" element={<LLMConfigPage />} />
          <Route path="inspections" element={<InspectionPage />} />
          <Route path="notifications" element={<NotificationPage />} />
          <Route path="clusters" element={<ClusterPage />} />
          <Route path="k8sgpt" element={<K8sGPTPage />} />
          <Route path="kubectl-ai" element={<KubectlAIPage />} />
          <Route path="users" element={<UserManagementPage />} />
          <Route path="feishu-sso" element={<FeishuSSOConfigPage />} />
        </Route>
        <Route path="/" element={<Navigate to="/app" replace />} />
        <Route path="*" element={<Navigate to="/app" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;

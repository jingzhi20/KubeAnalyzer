import { useState, useRef, useCallback, useEffect } from 'react';
import type { ReactNode } from 'react';
import {
  K8sWheelIcon, PodIcon, ServiceIcon, StorageIcon, ShieldIcon, NodeIcon,
  DeploymentIcon, ControlPlaneIcon, ConfigIcon, AppLayerIcon,
  CrashLoopIcon, HPAIcon, OOMIcon, CPUThrottleIcon, ScheduleFailIcon,
  ServiceDownIcon, IngressIcon, DNSFailIcon, PVCIcon, VolumeIcon, ReadOnlyIcon,
  RBACIcon, SecretIcon, NetworkPolicyIcon, NodeNotReadyIcon, DiskPressureIcon,
  EvictionIcon, JobTimeoutIcon, RollingUpdateIcon, ImagePullIcon, PodPendingIcon,
  InitContainerIcon, EtcdIcon, CertIcon, APIServerIcon, ConfigMapIcon, QuotaIcon,
  FinalizerIcon, ZombieProcessIcon, ConnectionPoolIcon,
} from '../components/K8sIcons';
import './HomePage.css';

interface FaultCase {
  icon: ReactNode;
  title: string;
  phenomenon: string;
  causes: string[];
  commands: string[];
  solutions: string[];
  priority: '高' | '中' | '低';
}

interface Category {
  icon: ReactNode;
  name: string;
  color: string;
  stars: number;
  cases: FaultCase[];
}

const categories: Category[] = [
  {
    icon: <DeploymentIcon size={30} color="#ff7043" />, name: '工作负载', color: '#ff7043', stars: 5,
    cases: [
      {
        icon: <CrashLoopIcon size={22} />, title: 'CrashLoopBackOff', phenomenon: 'Pod 反复重启，状态显示 CrashLoopBackOff',
        causes: ['应用启动报错、缺少配置', '探针配置不当导致误杀', '依赖服务未就绪'],
        commands: ['kubectl logs <pod> --previous', 'kubectl describe pod <pod>', 'kubectl get events --field-selector involvedObject.name=<pod>'],
        solutions: ['检查应用日志定位启动错误', '验证环境变量和 ConfigMap 挂载', '调整 livenessProbe 的 initialDelaySeconds'],
        priority: '高',
      },
      {
        icon: <ImagePullIcon size={22} />, title: 'ImagePullBackOff', phenomenon: '镜像拉取失败，Pod 无法启动',
        causes: ['镜像名称或 tag 错误', '私有仓库认证失败', '网络隔离无法访问仓库'],
        commands: ['kubectl describe pod <pod> | grep -A5 Events', 'kubectl get secret <pull-secret> -o yaml'],
        solutions: ['验证 imagePullSecrets 配置', '检查镜像仓库可达性', '确认镜像名称和 tag 正确'],
        priority: '高',
      },
      {
        icon: <RollingUpdateIcon size={22} />, title: 'Rollout 卡住', phenomenon: '滚动更新停滞，新旧版本共存',
        causes: ['新副本探针失败', '资源不足无法调度', 'PVC 冲突'],
        commands: ['kubectl rollout status deployment/<name>', 'kubectl describe deployment <name>'],
        solutions: ['kubectl rollout undo 回滚', '分步排查探针、资源、依赖', '检查 PDB 是否阻塞'],
        priority: '高',
      },
      {
        icon: <JobTimeoutIcon size={22} />, title: 'Job/CronJob 超时', phenomenon: '任务长时间运行不结束',
        causes: ['任务逻辑卡死', '资源限制导致 OOM', '依赖服务不可用'],
        commands: ['kubectl logs job/<name>', 'kubectl describe job <name>'],
        solutions: ['设置 activeDeadlineSeconds', '检查依赖服务健康状态', '增加资源 limits'],
        priority: '中',
      },
      {
        icon: <PodPendingIcon size={22} />, title: 'Pod Pending', phenomenon: 'Pod 一直处于 Pending 状态',
        causes: ['节点资源不足', 'PVC 未绑定', '配额耗尽', '污点未容忍'],
        commands: ['kubectl describe pod <pod>', 'kubectl get events -n <ns>', 'kubectl describe nodes'],
        solutions: ['检查资源 requests 是否合理', '验证 StorageClass 和 PVC', '检查 tolerations 配置'],
        priority: '高',
      },
      {
        icon: <InitContainerIcon size={22} />, title: 'Init Container 失败', phenomenon: '初始化容器执行失败，主容器无法启动',
        causes: ['初始化脚本错误', '依赖服务未就绪', '权限不足'],
        commands: ['kubectl logs <pod> -c <init-container>', 'kubectl describe pod <pod>'],
        solutions: ['确保初始化逻辑幂等', '添加重试和超时机制', '检查 ServiceAccount 权限'],
        priority: '中',
      },
    ],
  },
  {
    icon: <PodIcon size={30} color="#ef5350" />, name: '计算资源', color: '#ef5350', stars: 5,
    cases: [
      {
        icon: <OOMIcon size={22} />, title: 'OOMKilled', phenomenon: 'Pod 因内存超限被杀',
        causes: ['memory limits 设置过低', '应用内存泄漏', '突发流量导致内存飙升'],
        commands: ['kubectl get pod -o yaml | grep -A5 resources', 'kubectl describe pod <pod> | grep OOM', 'kubectl top pod'],
        solutions: ['调整 resources.limits.memory', '应用层排查内存泄漏', '配合 HPA 弹性伸缩'],
        priority: '高',
      },
      {
        icon: <CPUThrottleIcon size={22} />, title: 'CPU Throttling', phenomenon: '应用响应变慢，CPU 被限流',
        causes: ['CPU limits 设置过低', '突发计算密集型任务', 'requests/limits 比例不合理'],
        commands: ['kubectl top pod', 'kubectl describe pod <pod>', '监控 container_cpu_cfs_throttled_periods_total'],
        solutions: ['适当提高 CPU limits', '考虑移除 CPU limits（谨慎）', '优化应用 CPU 使用'],
        priority: '中',
      },
      {
        icon: <HPAIcon size={22} />, title: 'HPA 不生效', phenomenon: 'HPA 无法自动扩缩容',
        causes: ['未设置 resources.requests', 'metrics-server 未部署或异常', '指标采集延迟'],
        commands: ['kubectl describe hpa <name>', 'kubectl get --raw /apis/metrics.k8s.io/v1beta1/pods', 'kubectl top pod'],
        solutions: ['确保 Pod 设置了 resources.requests', '检查 metrics-server 运行状态', '设置合理的扩缩阈值'],
        priority: '中',
      },
      {
        icon: <ScheduleFailIcon size={22} />, title: '调度失败', phenomenon: 'Pod 无法被调度到任何节点',
        causes: ['节点资源不足', '亲和性/反亲和性冲突', '污点未容忍'],
        commands: ['kubectl describe pod <pod>', 'kubectl get nodes -o wide', 'kubectl describe nodes | grep -A5 Allocated'],
        solutions: ['扩容节点或调整资源请求', '检查 nodeAffinity 和 podAntiAffinity', '添加对应的 tolerations'],
        priority: '高',
      },
    ],
  },
  {
    icon: <ServiceIcon size={30} color="#5c6bc0" />, name: '网络通信', color: '#5c6bc0', stars: 5,
    cases: [
      {
        icon: <ServiceDownIcon size={22} />, title: 'Service 无 Endpoint', phenomenon: 'Service 存在但无法访问后端 Pod',
        causes: ['Pod 标签与 Service selector 不匹配', 'Pod 未通过 readinessProbe', 'Pod 不在 Running 状态'],
        commands: ['kubectl get endpoints <svc>', 'kubectl describe svc <svc>', 'kubectl get pods -l <selector>'],
        solutions: ['检查 Service selector 与 Pod labels 一致', '修复 readinessProbe 配置', '确保后端 Pod 正常运行'],
        priority: '高',
      },
      {
        icon: <IngressIcon size={22} />, title: 'Ingress 502/504', phenomenon: 'Ingress 返回 502 Bad Gateway 或 504 超时',
        causes: ['后端 Service 未就绪', '超时配置过短', 'TLS 证书问题'],
        commands: ['kubectl describe ingress <name>', 'curl -v https://<host>', 'kubectl logs <ingress-controller-pod>'],
        solutions: ['调整 proxy-read-timeout 等注解', '检查后端 Service 健康状态', '验证 TLS Secret 配置'],
        priority: '高',
      },
      {
        icon: <DNSFailIcon size={22} />, title: 'DNS 解析失败', phenomenon: 'Pod 内无法解析 Service 域名',
        causes: ['CoreDNS 崩溃或异常', '上游 DNS 不可达', 'resolv.conf 配置错误'],
        commands: ['nslookup <svc> <coredns-ip>', 'kubectl logs -n kube-system -l k8s-app=kube-dns', 'kubectl exec <pod> -- cat /etc/resolv.conf'],
        solutions: ['重启 CoreDNS Pod', '检查上游 DNS 服务器可达性', '验证 dnsPolicy 和 dnsConfig'],
        priority: '高',
      },
      {
        icon: <NetworkPolicyIcon size={22} />, title: '跨节点 Pod 不通', phenomenon: '不同节点上的 Pod 无法互相通信',
        causes: ['CNI 插件异常', '节点防火墙规则', 'MTU 不匹配'],
        commands: ['kubectl get pods -n kube-system -l k8s-app=<cni>', 'ping/traceroute 测试', 'ip link show'],
        solutions: ['检查 CNI 插件状态和日志', '验证节点间网络策略', '统一 MTU 配置'],
        priority: '中',
      },
    ],
  },
  {
    icon: <StorageIcon size={30} color="#ab47bc" />, name: '存储管理', color: '#ab47bc', stars: 4,
    cases: [
      {
        icon: <PVCIcon size={22} />, title: 'PVC Pending', phenomenon: 'PVC 一直处于 Pending 状态',
        causes: ['StorageClass 不存在', '存储配额不足', '云盘资源售罄'],
        commands: ['kubectl describe pvc <name>', 'kubectl get sc', 'kubectl get events -n <ns>'],
        solutions: ['创建或指定正确的 StorageClass', '调整存储资源配额', '检查云平台存储可用性'],
        priority: '高',
      },
      {
        icon: <VolumeIcon size={22} />, title: 'Volume 挂载失败', phenomenon: 'Pod 启动时 Volume 挂载报错',
        causes: ['存储后端不可达', '挂载点冲突', 'CSI 驱动未安装'],
        commands: ['kubectl describe pod <pod>', 'kubectl get csidriver', 'kubectl logs -n kube-system <csi-pod>'],
        solutions: ['检查存储驱动安装状态', '验证节点挂载能力', '确认 PV 访问模式正确'],
        priority: '高',
      },
      {
        icon: <ReadOnlyIcon size={22} />, title: '只读/写入失败', phenomenon: '容器内写入文件报 Read-only filesystem',
        causes: ['PV 访问模式限制', '文件系统权限不足', 'SELinux 策略拦截'],
        commands: ['kubectl exec <pod> -- touch /data/test', 'kubectl get pv <name> -o yaml | grep accessModes'],
        solutions: ['调整 securityContext 的 fsGroup', '检查 PV accessModes 配置', '配置 SELinux 标签'],
        priority: '中',
      },
    ],
  },
  {
    icon: <NodeIcon size={30} color="#8d6e63" />, name: '节点健康', color: '#8d6e63', stars: 4,
    cases: [
      {
        icon: <NodeNotReadyIcon size={22} />, title: 'Node NotReady', phenomenon: '节点状态变为 NotReady',
        causes: ['Kubelet 崩溃', '容器运行时异常', '网络断开'],
        commands: ['kubectl describe node <name>', 'systemctl status kubelet', 'journalctl -u kubelet --since "10m ago"'],
        solutions: ['重启 kubelet 服务', '检查容器运行时状态', '验证节点网络连通性'],
        priority: '高',
      },
      {
        icon: <DiskPressureIcon size={22} />, title: '磁盘压力', phenomenon: '节点出现 DiskPressure 状态',
        causes: ['日志/镜像堆积', 'emptyDir 滥用', '未配置日志轮转'],
        commands: ['df -h', 'kubectl describe node <name>', 'crictl images | wc -l'],
        solutions: ['配置 logrotate 日志轮转', '设置 imageGC 策略', '清理无用镜像和容器'],
        priority: '中',
      },
      {
        icon: <EvictionIcon size={22} />, title: '驱逐风暴', phenomenon: '大量 Pod 被同时驱逐',
        causes: ['多资源同时压力', '驱逐阈值设置过高', '节点资源碎片化'],
        commands: ['kubectl get events --field-selector reason=Evicted', 'kubectl describe node <name> | grep -A10 Conditions'],
        solutions: ['预留缓冲资源', '调整 eviction 阈值', '设置 PodDisruptionBudget'],
        priority: '高',
      },
    ],
  },
  {
    icon: <ShieldIcon size={30} color="#26a69a" />, name: '安全权限', color: '#26a69a', stars: 4,
    cases: [
      {
        icon: <RBACIcon size={22} />, title: 'RBAC 403', phenomenon: 'API 调用返回 403 Forbidden',
        causes: ['ServiceAccount 权限不足', 'RoleBinding 缺失', '命名空间作用域错误'],
        commands: ['kubectl auth can-i <verb> <resource> --as=system:serviceaccount:<ns>:<sa>', 'kubectl get rolebinding,clusterrolebinding -A | grep <sa>'],
        solutions: ['遵循最小权限原则创建 Role', '检查 RoleBinding 的 namespace 作用域', '审计日志分析权限缺口'],
        priority: '高',
      },
      {
        icon: <SecretIcon size={22} />, title: 'Secret 挂载失败', phenomenon: 'Pod 因 Secret 不存在无法启动',
        causes: ['Secret 未创建', '命名空间不匹配', '权限限制'],
        commands: ['kubectl get secret -n <ns>', 'kubectl describe pod <pod> | grep -A5 Volumes'],
        solutions: ['确保 Secret 先于 Pod 创建', '检查 Secret 所在命名空间', '验证 ServiceAccount 权限'],
        priority: '中',
      },
      {
        icon: <NetworkPolicyIcon size={22} />, title: 'NetworkPolicy 拦截', phenomenon: '业务流量被网络策略意外阻断',
        causes: ['默认拒绝策略未放行业务端口', '标签选择器不匹配', '策略优先级冲突'],
        commands: ['kubectl get networkpolicy -n <ns>', 'kubectl describe networkpolicy <name>', '抓包验证流量路径'],
        solutions: ['先放行再逐步收敛策略', '测试环境验证后再上线', '使用 NetworkPolicy 可视化工具'],
        priority: '中',
      },
    ],
  },
  {
    icon: <ControlPlaneIcon size={30} color="#1e88e5" />, name: '控制平面', color: '#1e88e5', stars: 3,
    cases: [
      {
        icon: <APIServerIcon size={22} />, title: 'kubectl 超时', phenomenon: 'kubectl 命令无响应或超时',
        causes: ['API Server 过载', 'etcd 延迟高', '网络抖动'],
        commands: ['kubectl get --raw /healthz', 'kubectl get --raw /metrics | grep apiserver_request_duration', 'curl -k https://<apiserver>:6443/healthz'],
        solutions: ['API Server 限流配置', '扩容 API Server 副本', '优化 etcd 性能'],
        priority: '高',
      },
      {
        icon: <EtcdIcon size={22} />, title: 'Etcd 性能瓶颈', phenomenon: 'etcd 响应延迟高，集群操作变慢',
        causes: ['磁盘 I/O 慢', 'compaction 未配置', '数据膨胀'],
        commands: ['etcdctl endpoint status --write-out=table', 'etcdctl endpoint health', 'journalctl -u etcd | grep "slow"'],
        solutions: ['使用 SSD 存储', '定期执行 compaction 和 defrag', '监控 etcd latency 指标'],
        priority: '高',
      },
      {
        icon: <CertIcon size={22} />, title: '证书过期', phenomenon: '组件间 TLS 通信失败',
        causes: ['kubelet/etcd/apiserver 证书到期', '证书轮换未配置', '手动签发证书遗忘续期'],
        commands: ['openssl x509 -in <cert> -text -noout | grep "Not After"', 'kubeadm certs check-expiration'],
        solutions: ['配置自动化证书轮换', '提前 30 天设置告警', '使用 cert-manager 管理证书'],
        priority: '高',
      },
    ],
  },
  {
    icon: <ConfigIcon size={30} color="#ffa726" />, name: '配置元数据', color: '#ffa726', stars: 3,
    cases: [
      {
        icon: <ConfigMapIcon size={22} />, title: 'ConfigMap 未热更新', phenomenon: '修改 ConfigMap 后应用未生效',
        causes: ['应用未监听文件变化', 'ConfigMap 以环境变量方式挂载', '缓存未刷新'],
        commands: ['kubectl exec <pod> -- cat /config/<file>', 'kubectl describe pod <pod> | grep -A10 Mounts'],
        solutions: ['使用 Volume 挂载而非环境变量', '应用支持热加载或使用 sidecar 重载', '滚动重启 Pod 使配置生效'],
        priority: '低',
      },
      {
        icon: <QuotaIcon size={22} />, title: 'ResourceQuota 耗尽', phenomenon: '新 Pod 无法创建，提示配额不足',
        causes: ['Namespace 配额用尽', '僵尸资源未清理', '配额设置过低'],
        commands: ['kubectl describe resourcequota -n <ns>', 'kubectl get all -n <ns>'],
        solutions: ['清理无用资源释放配额', '调整 ResourceQuota 上限', '监控配额使用率并告警'],
        priority: '中',
      },
      {
        icon: <FinalizerIcon size={22} />, title: 'Finalizers 卡住删除', phenomenon: '资源删除后一直处于 Terminating',
        causes: ['外部资源未释放', '控制器未清理 Finalizer', 'Operator 崩溃'],
        commands: ['kubectl get <resource> -o yaml | grep finalizers', 'kubectl get <resource> -o json | jq .metadata.finalizers'],
        solutions: ['修复对应的控制器/Operator', '手动移除 finalizers（谨慎操作）', '检查 Operator 日志定位原因'],
        priority: '中',
      },
    ],
  },
  {
    icon: <AppLayerIcon size={30} color="#78909c" />, name: '应用层故障', color: '#78909c', stars: 3,
    cases: [
      {
        icon: <ZombieProcessIcon size={22} />, title: '僵尸进程', phenomenon: 'Pod Running 但应用无响应',
        causes: ['PID 1 未转发信号', '未使用 tini/init', '应用死锁'],
        commands: ['kubectl exec <pod> -- ps aux', 'kubectl exec <pod> -- kill -0 1'],
        solutions: ['使用 tini 作为 entrypoint', '正确处理 SIGTERM 信号', '增加 liveness 探针覆盖业务逻辑'],
        priority: '中',
      },
      {
        icon: <ConnectionPoolIcon size={22} />, title: '连接池耗尽', phenomenon: '应用报连接超时但 Pod 资源正常',
        causes: ['数据库连接泄漏', 'HTTP 连接未释放', 'conntrack 表满'],
        commands: ['kubectl exec <pod> -- ss -s', 'kubectl exec <pod> -- ulimit -n', 'sysctl net.netfilter.nf_conntrack_count'],
        solutions: ['优化应用连接池配置', '设置连接超时和回收策略', '调整内核 conntrack 参数'],
        priority: '中',
      },
    ],
  },
];

const CENTER = { x: 50, y: 50 };
const CAT_RADIUS = 28;
const CASE_RADIUS = 18;
const START_ANGLE = -90;

function getPosition(cx: number, cy: number, radius: number, angleDeg: number) {
  const rad = (angleDeg * Math.PI) / 180;
  return { x: cx + radius * Math.cos(rad), y: cy + radius * 0.88 * Math.sin(rad) };
}

type BubbleId = string;
interface DragOffset { dx: number; dy: number }

function getFloatParams(id: string) {
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = ((hash << 5) - hash + id.charCodeAt(i)) | 0;
  const seed = Math.abs(hash);
  const isCenter = id === 'center';
  const isCat = id.startsWith('cat-');
  const baseAmp = isCenter ? 4 : isCat ? 7 : 10;
  return {
    a1x: baseAmp + (seed % 3), a1y: baseAmp + ((seed >> 2) % 3),
    s1: 0.0004 + ((seed >> 4) % 3) * 0.00006, p1: ((seed >> 6) % 100) / 16,
    a2x: baseAmp * 0.4 + ((seed >> 8) % 2), a2y: baseAmp * 0.4 + ((seed >> 10) % 2),
    s2: 0.001 + ((seed >> 12) % 4) * 0.00008, p2: ((seed >> 14) % 100) / 16,
  };
}

const priorityColors: Record<string, string> = { '高': '#ef5350', '中': '#ffa726', '低': '#66bb6a' };

function HomePage() {
  const [activeCase, setActiveCase] = useState<{ catIdx: number; caseIdx: number } | null>(null);
  const [expandedCats, setExpandedCats] = useState<Set<number>>(new Set());
  const [offsets, setOffsets] = useState<Record<BubbleId, DragOffset>>({});
  const [returning, setReturning] = useState<Record<BubbleId, boolean>>({});
  const dragRef = useRef<{ id: BubbleId; startX: number; startY: number; hasMoved: boolean } | null>(null);
  const [floatOffsets, setFloatOffsets] = useState<Record<BubbleId, { fx: number; fy: number }>>({});
  const allBubbleIds = useRef<BubbleId[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);

  const toggleCat = useCallback((ci: number) => {
    setExpandedCats(prev => {
      const next = new Set(prev);
      if (next.has(ci)) {
        next.delete(ci);
        // Close any active case in this category
        setActiveCase(ac => ac && ac.catIdx === ci ? null : ac);
      } else {
        next.add(ci);
      }
      return next;
    });
  }, []);

  if (allBubbleIds.current.length === 0) {
    allBubbleIds.current = ['center'];
    categories.forEach((cat, ci) => {
      allBubbleIds.current.push(`cat-${ci}`);
      cat.cases.forEach((_, ki) => allBubbleIds.current.push(`case-${ci}-${ki}`));
    });
  }

  useEffect(() => {
    let raf: number;
    let lastUpdate = 0;
    const animate = (time: number) => {
      if (time - lastUpdate > 33) {
        lastUpdate = time;
        const nf: Record<BubbleId, { fx: number; fy: number }> = {};
        for (const id of allBubbleIds.current) {
          const p = getFloatParams(id);
          nf[id] = {
            fx: Math.sin(time * p.s1 + p.p1) * p.a1x + Math.sin(time * p.s2 + p.p2) * p.a2x,
            fy: Math.cos(time * p.s1 * 0.7 + p.p1 + 2) * p.a1y + Math.cos(time * p.s2 * 1.3 + p.p2 + 1) * p.a2y,
          };
        }
        setFloatOffsets(nf);
      }
      raf = requestAnimationFrame(animate);
    };
    raf = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(raf);
  }, []);

  const getOffset = (id: BubbleId): DragOffset => offsets[id] || { dx: 0, dy: 0 };

  const getChildren = useCallback((id: BubbleId): BubbleId[] => {
    if (id === 'center') {
      const ch: BubbleId[] = [];
      categories.forEach((cat, ci) => { ch.push(`cat-${ci}`); cat.cases.forEach((_, ki) => ch.push(`case-${ci}-${ki}`)); });
      return ch;
    }
    const m = id.match(/^cat-(\d+)$/);
    if (m) return categories[parseInt(m[1])].cases.map((_, ki) => `case-${m[1]}-${ki}`);
    return [];
  }, []);

  const handleMouseDown = useCallback((e: React.MouseEvent, id: BubbleId) => {
    e.preventDefault();
    dragRef.current = { id, startX: e.clientX, startY: e.clientY, hasMoved: false };
    const children = getChildren(id);
    setReturning(prev => { const n = { ...prev, [id]: false }; children.forEach(c => { n[c] = false; }); return n; });
  }, [getChildren]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!dragRef.current) return;
    const { id, startX, startY } = dragRef.current;
    const dx = e.clientX - startX, dy = e.clientY - startY;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) dragRef.current.hasMoved = true;
    dragRef.current.startX = e.clientX;
    dragRef.current.startY = e.clientY;
    const children = getChildren(id);
    setOffsets(prev => {
      const n = { ...prev };
      const c = prev[id] || { dx: 0, dy: 0 };
      n[id] = { dx: c.dx + dx, dy: c.dy + dy };
      children.forEach(cid => { const cc = prev[cid] || { dx: 0, dy: 0 }; n[cid] = { dx: cc.dx + dx, dy: cc.dy + dy }; });
      return n;
    });
  }, [getChildren]);

  const handleMouseUp = useCallback(() => {
    if (!dragRef.current) return;
    const { id, hasMoved } = dragRef.current;
    dragRef.current = null;
    if (!hasMoved) return;
    const all = [id, ...getChildren(id)];
    setReturning(p => { const n = { ...p }; all.forEach(b => { n[b] = true; }); return n; });
    setOffsets(p => { const n = { ...p }; all.forEach(b => { n[b] = { dx: 0, dy: 0 }; }); return n; });
    setTimeout(() => { setReturning(p => { const n = { ...p }; all.forEach(b => { n[b] = false; }); return n; }); }, 650);
  }, [getChildren]);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    return () => { window.removeEventListener('mousemove', handleMouseMove); window.removeEventListener('mouseup', handleMouseUp); };
  }, [handleMouseMove, handleMouseUp]);

  const bubbleTransform = (id: BubbleId) => {
    const o = getOffset(id);
    const f = floatOffsets[id] || { fx: 0, fy: 0 };
    const isRet = returning[id];
    const isDrag = dragRef.current?.id === id;
    const fx = isDrag ? 0 : f.fx, fy = isDrag ? 0 : f.fy;
    return {
      transform: `translate(calc(-50% + ${o.dx + fx}px), calc(-50% + ${o.dy + fy}px))`,
      transition: isRet ? 'transform 0.6s cubic-bezier(0.25, 1.5, 0.5, 1)' : undefined,
      cursor: isDrag ? 'grabbing' : 'grab',
    } as React.CSSProperties;
  };

  const catPositions = categories.map((_, i) => {
    const angle = START_ANGLE + (360 / categories.length) * i;
    return getPosition(CENTER.x, CENTER.y, CAT_RADIUS, angle);
  });

  const casePositions = categories.map((cat, ci) => {
    const catPos = catPositions[ci];
    const catAngle = START_ANGLE + (360 / categories.length) * ci;
    return cat.cases.map((_, ki) => {
      const n = cat.cases.length;
      const totalArc = Math.min(160, n * 28);
      const step = n === 1 ? 0 : totalArc / (n - 1);
      const offset = n === 1 ? 0 : (ki - (n - 1) / 2) * step;
      return getPosition(catPos.x, catPos.y, CASE_RADIUS, catAngle + offset);
    });
  });

  const getLineOffset = (id: BubbleId) => {
    const o = getOffset(id);
    const f = floatOffsets[id] || { fx: 0, fy: 0 };
    const isDrag = dragRef.current?.id === id;
    const fx = isDrag ? 0 : f.fx, fy = isDrag ? 0 : f.fy;
    const el = containerRef.current;
    if (!el) return { dx: 0, dy: 0 };
    return { dx: ((o.dx + fx) / el.clientWidth) * 100, dy: ((o.dy + fy) / el.clientHeight) * 100 };
  };

  const ac = activeCase ? categories[activeCase.catIdx].cases[activeCase.caseIdx] : null;
  const acCat = activeCase ? categories[activeCase.catIdx] : null;

  return (
    <div className="home-container">
      <div className={`topo-area ${activeCase ? 'with-detail' : ''}`} ref={containerRef}>
        <svg className="topo-lines">
          {catPositions.map((cp, ci) => {
            const centerOff = getLineOffset('center');
            const catOff = getLineOffset(`cat-${ci}`);
            return (
              <g key={`lines-${ci}`}>
                <line
                  x1={`${CENTER.x + centerOff.dx}%`} y1={`${CENTER.y + centerOff.dy}%`}
                  x2={`${cp.x + catOff.dx}%`} y2={`${cp.y + catOff.dy}%`}
                  className="topo-line-main"
                />
                {expandedCats.has(ci) && casePositions[ci].map((kp, ki) => {
                  const caseOff = getLineOffset(`case-${ci}-${ki}`);
                  return (
                    <line key={ki}
                      x1={`${cp.x + catOff.dx}%`} y1={`${cp.y + catOff.dy}%`}
                      x2={`${kp.x + caseOff.dx}%`} y2={`${kp.y + caseOff.dy}%`}
                      className="topo-line-sub"
                    />
                  );
                })}
              </g>
            );
          })}
        </svg>

        <div className="center-bubble" style={bubbleTransform('center')} onMouseDown={e => handleMouseDown(e, 'center')}>
          <span className="center-icon"><K8sWheelIcon size={44} color="#fff" /></span>
          <span className="center-label">故障全景</span>
        </div>

        {categories.map((cat, ci) => (
          <div key={ci}>
            <div
              className="cat-bubble"
              style={{
                left: `${catPositions[ci].x}%`, top: `${catPositions[ci].y}%`,
                borderColor: cat.color, boxShadow: `0 0 20px ${cat.color}33`,
                ...bubbleTransform(`cat-${ci}`),
              }}
              onMouseDown={e => handleMouseDown(e, `cat-${ci}`)}
              onClick={() => { if (!dragRef.current?.hasMoved) toggleCat(ci); }}
            >
              <span className="cat-icon">{cat.icon}</span>
              <span className="cat-name" style={{ color: cat.color }}>{cat.name}</span>
              <span className="cat-expand-hint">{expandedCats.has(ci) ? '−' : `+${cat.cases.length}`}</span>
            </div>
            {expandedCats.has(ci) && cat.cases.map((c, ki) => {
              const pos = casePositions[ci][ki];
              const isActive = activeCase?.catIdx === ci && activeCase?.caseIdx === ki;
              const bubbleId = `case-${ci}-${ki}`;
              return (
                <div key={ki}
                  className={`case-bubble ${isActive ? 'active' : ''}`}
                  style={{ left: `${pos.x}%`, top: `${pos.y}%`, background: cat.color, ...bubbleTransform(bubbleId) }}
                  onMouseDown={e => handleMouseDown(e, bubbleId)}
                  onClick={() => { if (!dragRef.current?.hasMoved) setActiveCase(isActive ? null : { catIdx: ci, caseIdx: ki }); }}
                >
                  <span className="case-icon">{c.icon}</span>
                  <span className="case-title">{c.title}</span>
                </div>
              );
            })}
          </div>
        ))}
      </div>

      {ac && acCat && (
        <div className="detail-panel">
          <button className="detail-close" onClick={() => setActiveCase(null)}>✕</button>
          <div className="detail-header" style={{ borderLeftColor: acCat.color }}>
            <div className="detail-title-row">
              <span className="detail-title">{ac.title}</span>
              <span className="detail-priority" style={{ background: priorityColors[ac.priority] }}>
                {ac.priority === '高' ? '🔴' : ac.priority === '中' ? '🟡' : '🟢'} {ac.priority}优先级
              </span>
            </div>
            <div className="detail-category" style={{ color: acCat.color }}>
              {acCat.name} · {'★'.repeat(acCat.stars)}
            </div>
          </div>

          <div className="detail-section">
            <div className="detail-section-title">📋 故障现象</div>
            <div className="detail-phenomenon">{ac.phenomenon}</div>
          </div>

          <div className="detail-section">
            <div className="detail-section-title">🔍 常见原因</div>
            <ul className="detail-list">
              {ac.causes.map((c, i) => <li key={i}>{c}</li>)}
            </ul>
          </div>

          <div className="detail-section">
            <div className="detail-section-title">💻 排查命令</div>
            <div className="detail-commands">
              {ac.commands.map((cmd, i) => <code key={i} className="detail-cmd">{cmd}</code>)}
            </div>
          </div>

          <div className="detail-section">
            <div className="detail-section-title">✅ 解决方案</div>
            <ol className="detail-solutions">
              {ac.solutions.map((s, i) => <li key={i}>{s}</li>)}
            </ol>
          </div>
        </div>
      )}
    </div>
  );
}

export default HomePage;

import { useState, useRef, useCallback, useEffect } from 'react';
import type { ReactNode } from 'react';
import {
  K8sWheelIcon,
  PodIcon,
  ServiceIcon,
  StorageIcon,
  ShieldIcon,
  NodeIcon,
  DeploymentIcon,
  CrashLoopIcon,
  HPAIcon,
  ServiceDownIcon,
  IngressIcon,
  PVCIcon,
  VolumeIcon,
  RBACIcon,
  SecretIcon,
  NodeNotReadyIcon,
  EvictionIcon,
  JobTimeoutIcon,
  RollingUpdateIcon,
} from '../components/K8sIcons';
import './HomePage.css';

interface CaseItem {
  icon: ReactNode;
  title: string;
  detail: string;
}

interface Category {
  icon: ReactNode;
  name: string;
  color: string;
  cases: CaseItem[];
}

const categories: Category[] = [
  {
    icon: <PodIcon size={30} color="#ef5350" />, name: '计算资源', color: '#ef5350',
    cases: [
      { icon: <CrashLoopIcon size={22} />, title: 'Pod CrashLoop', detail: 'Pod 因 OOMKilled 反复重启，需调整 resources.limits.memory' },
      { icon: <HPAIcon size={22} />, title: 'HPA 失效', detail: 'metrics-server 未部署，HPA 无法获取 CPU/Memory 指标' },
    ],
  },
  {
    icon: <ServiceIcon size={30} color="#5c6bc0" />, name: '网络通信', color: '#5c6bc0',
    cases: [
      { icon: <ServiceDownIcon size={22} />, title: 'Service 不通', detail: 'NetworkPolicy 阻断跨 namespace 访问，需添加 ingress 规则' },
      { icon: <IngressIcon size={22} />, title: 'Ingress 502', detail: 'Ingress 后端 Service 端口与 Pod containerPort 不匹配' },
    ],
  },
  {
    icon: <StorageIcon size={30} color="#ab47bc" />, name: '存储管理', color: '#ab47bc',
    cases: [
      { icon: <PVCIcon size={22} />, title: 'PVC Pending', detail: 'StorageClass 未配置 provisioner，导致 PVC 无法绑定' },
      { icon: <VolumeIcon size={22} />, title: 'Volume 挂载失败', detail: 'NFS/CSI 驱动未安装或权限不足导致 Pod 启动失败' },
    ],
  },
  {
    icon: <ShieldIcon size={30} color="#26a69a" />, name: '安全权限', color: '#26a69a',
    cases: [
      { icon: <RBACIcon size={22} />, title: 'RBAC 403', detail: 'ServiceAccount 缺少 ClusterRoleBinding，API 调用 403' },
      { icon: <SecretIcon size={22} />, title: 'Secret 泄露', detail: '敏感配置未加密存储，需启用 EncryptionConfiguration' },
    ],
  },
  {
    icon: <NodeIcon size={30} color="#8d6e63" />, name: '节点健康', color: '#8d6e63',
    cases: [
      { icon: <NodeNotReadyIcon size={22} />, title: 'Node NotReady', detail: '节点 DiskPressure / MemoryPressure，需清理资源' },
      { icon: <EvictionIcon size={22} />, title: '驱逐风暴', detail: '节点资源不足触发大规模 Pod Eviction，需扩容或限流' },
    ],
  },
  {
    icon: <DeploymentIcon size={30} color="#ff7043" />, name: '工作负载', color: '#ff7043',
    cases: [
      { icon: <JobTimeoutIcon size={22} />, title: 'Job 超时', detail: 'CronJob 未设置 activeDeadlineSeconds，任务长时间挂起' },
      { icon: <RollingUpdateIcon size={22} />, title: '滚动更新卡住', detail: 'Readiness Probe 配置错误导致新 Pod 永远不 Ready' },
    ],
  },
];

const CENTER = { x: 50, y: 50 };
const CAT_RADIUS = 26;
const CASE_RADIUS = 12;
const START_ANGLE = -90;

function getPosition(cx: number, cy: number, radius: number, angleDeg: number) {
  const rad = (angleDeg * Math.PI) / 180;
  return { x: cx + radius * Math.cos(rad), y: cy + radius * 0.9 * Math.sin(rad) };
}

// Unique key for each draggable bubble: "center", "cat-0", "case-0-1", etc.
type BubbleId = string;
interface DragOffset { dx: number; dy: number }

// Generate stable float parameters per bubble — multi-layer for organic motion
function getFloatParams(id: string) {
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = ((hash << 5) - hash + id.charCodeAt(i)) | 0;
  const seed = Math.abs(hash);
  // Determine bubble type for amplitude scaling
  const isCenter = id === 'center';
  const isCat = id.startsWith('cat-');
  const baseAmp = isCenter ? 6 : isCat ? 10 : 14;
  return {
    // Layer 1: slow, wide orbit
    a1x: baseAmp + (seed % 4),
    a1y: baseAmp + ((seed >> 2) % 4),
    s1: 0.0005 + ((seed >> 4) % 4) * 0.00008,
    p1: ((seed >> 6) % 100) / 16,
    // Layer 2: faster, tighter wobble
    a2x: baseAmp * 0.5 + ((seed >> 8) % 3),
    a2y: baseAmp * 0.5 + ((seed >> 10) % 3),
    s2: 0.0012 + ((seed >> 12) % 5) * 0.0001,
    p2: ((seed >> 14) % 100) / 16,
    // Layer 3: subtle high-freq jitter
    a3x: baseAmp * 0.2,
    a3y: baseAmp * 0.2,
    s3: 0.002 + ((seed >> 16) % 3) * 0.0003,
    p3: ((seed >> 18) % 100) / 16,
  };
}

function HomePage() {
  const [activeCase, setActiveCase] = useState<{ catIdx: number; caseIdx: number } | null>(null);

  // Drag state
  const [offsets, setOffsets] = useState<Record<BubbleId, DragOffset>>({});
  const [returning, setReturning] = useState<Record<BubbleId, boolean>>({});
  const dragRef = useRef<{
    id: BubbleId;
    startX: number;
    startY: number;
    hasMoved: boolean;
  } | null>(null);

  // Floating animation via requestAnimationFrame
  const [floatOffsets, setFloatOffsets] = useState<Record<BubbleId, { fx: number; fy: number }>>({});
  const allBubbleIds = useRef<BubbleId[]>([]);

  // Build bubble ID list once
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
      // Throttle to ~30fps for performance
      if (time - lastUpdate > 33) {
        lastUpdate = time;
        const newFloats: Record<BubbleId, { fx: number; fy: number }> = {};
        for (const id of allBubbleIds.current) {
          const p = getFloatParams(id);
          // Layer 1: slow orbit
          const fx1 = Math.sin(time * p.s1 + p.p1) * p.a1x;
          const fy1 = Math.cos(time * p.s1 * 0.7 + p.p1 + 2) * p.a1y;
          // Layer 2: medium wobble
          const fx2 = Math.sin(time * p.s2 + p.p2) * p.a2x;
          const fy2 = Math.cos(time * p.s2 * 1.3 + p.p2 + 1) * p.a2y;
          // Layer 3: subtle jitter
          const fx3 = Math.sin(time * p.s3 + p.p3) * p.a3x;
          const fy3 = Math.cos(time * p.s3 * 0.9 + p.p3 + 3) * p.a3y;
          newFloats[id] = {
            fx: fx1 + fx2 + fx3,
            fy: fy1 + fy2 + fy3,
          };
        }
        setFloatOffsets(newFloats);
      }
      raf = requestAnimationFrame(animate);
    };
    raf = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(raf);
  }, []);

  const getOffset = (id: BubbleId): DragOffset => offsets[id] || { dx: 0, dy: 0 };

  // Get child bubble IDs that should follow when a bubble is dragged
  const getChildren = useCallback((id: BubbleId): BubbleId[] => {
    if (id === 'center') {
      // All cat and case bubbles
      const children: BubbleId[] = [];
      categories.forEach((cat, ci) => {
        children.push(`cat-${ci}`);
        cat.cases.forEach((_, ki) => children.push(`case-${ci}-${ki}`));
      });
      return children;
    }
    const catMatch = id.match(/^cat-(\d+)$/);
    if (catMatch) {
      const ci = parseInt(catMatch[1]);
      return categories[ci].cases.map((_, ki) => `case-${ci}-${ki}`);
    }
    return [];
  }, []);

  const handleMouseDown = useCallback((e: React.MouseEvent, id: BubbleId) => {
    e.preventDefault();
    dragRef.current = { id, startX: e.clientX, startY: e.clientY, hasMoved: false };
    // Clear returning for this bubble and all children
    const children = getChildren(id);
    setReturning(prev => {
      const next = { ...prev, [id]: false };
      children.forEach(c => { next[c] = false; });
      return next;
    });
  }, [getChildren]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!dragRef.current) return;
    const { id, startX, startY } = dragRef.current;
    const deltaX = e.clientX - startX;
    const deltaY = e.clientY - startY;
    if (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3) {
      dragRef.current.hasMoved = true;
    }
    dragRef.current.startX = e.clientX;
    dragRef.current.startY = e.clientY;

    const children = getChildren(id);
    setOffsets(prev => {
      const next = { ...prev };
      // Move the dragged bubble
      const cur = prev[id] || { dx: 0, dy: 0 };
      next[id] = { dx: cur.dx + deltaX, dy: cur.dy + deltaY };
      // Move all children by the same delta
      children.forEach(cid => {
        const cc = prev[cid] || { dx: 0, dy: 0 };
        next[cid] = { dx: cc.dx + deltaX, dy: cc.dy + deltaY };
      });
      return next;
    });
  }, [getChildren]);

  const handleMouseUp = useCallback(() => {
    if (!dragRef.current) return;
    const { id, hasMoved } = dragRef.current;
    dragRef.current = null;
    if (!hasMoved) return;

    const children = getChildren(id);
    const allIds = [id, ...children];

    // Animate all back
    setReturning(prev => {
      const next = { ...prev };
      allIds.forEach(bid => { next[bid] = true; });
      return next;
    });
    setOffsets(prev => {
      const next = { ...prev };
      allIds.forEach(bid => { next[bid] = { dx: 0, dy: 0 }; });
      return next;
    });
    setTimeout(() => {
      setReturning(prev => {
        const next = { ...prev };
        allIds.forEach(bid => { next[bid] = false; });
        return next;
      });
    }, 650);
  }, [getChildren]);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };
  }, [handleMouseMove, handleMouseUp]);

  const bubbleTransform = (id: BubbleId) => {
    const o = getOffset(id);
    const f = floatOffsets[id] || { fx: 0, fy: 0 };
    const isReturning = returning[id];
    const isDragTarget = dragRef.current?.id === id;
    // Only pause float on the exact bubble being grabbed, children keep floating
    const fx = isDragTarget ? 0 : f.fx;
    const fy = isDragTarget ? 0 : f.fy;
    return {
      transform: `translate(calc(-50% + ${o.dx + fx}px), calc(-50% + ${o.dy + fy}px))`,
      transition: isReturning ? 'transform 0.6s cubic-bezier(0.25, 1.5, 0.5, 1)' : undefined,
      cursor: isDragTarget ? 'grabbing' : 'grab',
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
      const spread = cat.cases.length === 1 ? 0 : 40;
      const offset = cat.cases.length === 1 ? 0 : (ki - (cat.cases.length - 1) / 2) * spread;
      return getPosition(catPos.x, catPos.y, CASE_RADIUS, catAngle + offset);
    });
  });

  // SVG line endpoints need to account for drag offsets (in px → need container ref)
  const containerRef = useRef<HTMLDivElement>(null);

  const getLineOffset = (id: BubbleId) => {
    const o = getOffset(id);
    const f = floatOffsets[id] || { fx: 0, fy: 0 };
    const isDragTarget = dragRef.current?.id === id;
    const fx = isDragTarget ? 0 : f.fx;
    const fy = isDragTarget ? 0 : f.fy;
    const el = containerRef.current;
    if (!el) return { dx: 0, dy: 0 };
    return {
      dx: ((o.dx + fx) / el.clientWidth) * 100,
      dy: ((o.dy + fy) / el.clientHeight) * 100,
    };
  };

  return (
    <div className="home-container">
      <div className="topo-area" ref={containerRef}>
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
                {casePositions[ci].map((kp, ki) => {
                  const caseOff = getLineOffset(`case-${ci}-${ki}`);
                  return (
                    <line
                      key={ki}
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

        <div
          className="center-bubble"
          style={bubbleTransform('center')}
          onMouseDown={e => handleMouseDown(e, 'center')}
        >
          <span className="center-icon"><K8sWheelIcon size={44} color="#fff" /></span>
          <span className="center-label">智能诊断</span>
        </div>

        {categories.map((cat, ci) => (
          <div key={ci}>
            <div
              className="cat-bubble"
              style={{
                left: `${catPositions[ci].x}%`,
                top: `${catPositions[ci].y}%`,
                borderColor: cat.color,
                boxShadow: `0 0 20px ${cat.color}33`,
                ...bubbleTransform(`cat-${ci}`),
              }}
              onMouseDown={e => handleMouseDown(e, `cat-${ci}`)}
            >
              <span className="cat-icon">{cat.icon}</span>
              <span className="cat-name" style={{ color: cat.color }}>{cat.name}</span>
            </div>

            {cat.cases.map((c, ki) => {
              const pos = casePositions[ci][ki];
              const isActive = activeCase?.catIdx === ci && activeCase?.caseIdx === ki;
              const bubbleId = `case-${ci}-${ki}`;
              return (
                <div
                  key={ki}
                  className={`case-bubble ${isActive ? 'active' : ''}`}
                  style={{
                    left: `${pos.x}%`,
                    top: `${pos.y}%`,
                    background: cat.color,
                    ...bubbleTransform(bubbleId),
                  }}
                  onMouseDown={e => handleMouseDown(e, bubbleId)}
                  onClick={() => {
                    if (!dragRef.current?.hasMoved) {
                      setActiveCase(isActive ? null : { catIdx: ci, caseIdx: ki });
                    }
                  }}
                >
                  <span className="case-icon">{c.icon}</span>
                  <span className="case-title">{c.title}</span>
                </div>
              );
            })}
          </div>
        ))}

        {activeCase && (
          <div className="topo-detail">
            <span style={{ fontWeight: 600 }}>
              {categories[activeCase.catIdx].cases[activeCase.caseIdx].title}：
            </span>
            {categories[activeCase.catIdx].cases[activeCase.caseIdx].detail}
          </div>
        )}
      </div>
    </div>
  );
}

export default HomePage;

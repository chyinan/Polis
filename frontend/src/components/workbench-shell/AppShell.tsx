// pattern: Imperative Shell

import {useEffect, useState, type ReactNode} from 'react';
import {NavLink, useLocation, useNavigate} from 'react-router-dom';
import {Activity, BadgeCheck, Building2, ChevronDown, CircleHelp, FileCheck2, Layers3, MessageSquare, Moon, Radio, Settings, ShieldCheck, Sun, Target, UsersRound, type LucideIcon} from 'lucide-react';
import type {ActivityStreamStatus} from '../../data/workbench-api';
import type {DataMode, Freshness} from '../../domain/workbench';
import {labelDataMode, labelRecoveryState} from '../../domain/display-labels';
import {managementAccessForOrigins} from '../../domain/management-access';
import styles from '../../styles/workbench.module.css';

type ThemeMode = 'dark' | 'light';

type AppShellProps = Readonly<{
  apiBaseUrl: string;
  children: ReactNode;
  companyId: string;
  companyName: string;
  dataMode: DataMode;
  freshness: Freshness;
  observedAt: string | null;
  recoveryState: 'operational' | 'recovery_required' | 'unknown';
  streamStatus: ActivityStreamStatus;
}>;

type NavigationItem = Readonly<{
  label: string;
  path: string;
  icon: LucideIcon;
}>;

const companyNavigation: ReadonlyArray<NavigationItem> = [
  {label: '公司总览', path: 'overview', icon: Building2},
  {label: '使命', path: 'mission', icon: Target},
  {label: '任务', path: 'tasks', icon: FileCheck2},
  {label: '员工', path: 'employees', icon: UsersRound},
  {label: '组织关系', path: 'org-chart', icon: ShieldCheck},
  {label: '活动', path: 'activity', icon: Activity},
  {label: '协作', path: 'collaboration', icon: MessageSquare},
  {label: '运行与资源', path: 'operations', icon: Radio},
  {label: '资源', path: 'resources', icon: Layers3},
  {label: '证据', path: 'evidence', icon: BadgeCheck},
  {label: '待决事项', path: 'decisions', icon: CircleHelp},
  {label: '反馈', path: 'feedback', icon: MessageSquare},
  {label: 'Bot提醒', path: 'notifications', icon: Radio},
  {label: '设置', path: 'settings', icon: Settings},
];

const groupNavigation: ReadonlyArray<NavigationItem> = [
  {label: '公司总览', path: 'overview', icon: Building2},
  {label: '新建公司', path: 'new-company', icon: Layers3},
  {label: '集团资源', path: 'resources', icon: Layers3},
  {label: '集团设置', path: 'settings', icon: Settings},
];

function readThemePreference(): ThemeMode {
  if (typeof window === 'undefined') return 'dark';
  return window.localStorage.getItem('polis-theme') === 'light' ? 'light' : 'dark';
}

function formatObservedAt(iso: string | null): string {
  if (iso === null) return '未观测';
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? '不可得' : new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Shanghai',
    timeZoneName: 'short',
  }).format(date);
}

export function AppShell({apiBaseUrl, children, companyId, companyName, dataMode, observedAt, recoveryState}: AppShellProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const [theme, setTheme] = useState<ThemeMode>(readThemePreference);
  const [scopeOpen, setScopeOpen] = useState(false);
  const isGroupScope = location.pathname.startsWith('/group/');
  const navItems = isGroupScope ? groupNavigation : companyNavigation;
  const groupViewLabel = location.pathname.includes('/new-company') ? '新建公司' : location.pathname.includes('/resources') ? '集团资源' : location.pathname.includes('/settings') ? '集团设置' : '总览';

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    window.localStorage.setItem('polis-theme', theme);
  }, [theme]);

  useEffect(() => {
    if (!scopeOpen) return;
    function closeOnEscape(event: KeyboardEvent): void {
      if (event.key === 'Escape') setScopeOpen(false);
    }
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [scopeOpen]);

  function toggleTheme(): void {
    setTheme(current => current === 'dark' ? 'light' : 'dark');
  }

  function switchScope(nextScope: 'company' | 'group'): void {
    setScopeOpen(false);
    navigate(nextScope === 'company' ? '/companies/' + companyId + '/overview' : '/group/overview');
  }

  const ThemeIcon = theme === 'dark' ? Sun : Moon;
  const modeLabel = labelDataMode(dataMode);
  const managementAccess = managementAccessForOrigins(window.location.origin, apiBaseUrl);

  return (
    <div className={styles.appShell} data-theme={theme}>
      <a className={styles.skipLink} href="#main-content">跳到主要内容</a>
      <aside className={styles.sidebar} aria-label={isGroupScope ? '集团导航' : '公司工作台导航'}>
        <div className={styles.brandBlock}>
          <div className={styles.brandMark}>P</div>
          <div className={styles.brandText}><strong>polis</strong><span>{managementAccess.label}</span></div>
        </div>

        <div className={styles.scopeBlock}>
          <button aria-expanded={scopeOpen} aria-haspopup="menu" aria-label={isGroupScope ? '当前集团作用域' : '当前作用域公司'} className={styles.workspaceSelector} data-od-id="workspace-selector" onClick={() => setScopeOpen(open => !open)} type="button">
            <span className={styles.workspaceDot} />
            <span className={styles.workspaceCopy}><strong>{isGroupScope ? '集团' : companyName}</strong><small>{isGroupScope ? '集团作用域' : '作用域公司'}</small></span>
            <ChevronDown className={styles.icon} aria-hidden="true" size={14} />
          </button>
          {scopeOpen ? <div className={styles.scopeMenu} id="scope-menu" role="menu">
            <button className={!isGroupScope ? styles.scopeOptionActive : styles.scopeOption} onClick={() => switchScope('company')} role="menuitem" type="button"><Building2 aria-hidden="true" size={15} /><span>公司工作台<small>{companyName}</small></span></button>
            <button className={isGroupScope ? styles.scopeOptionActive : styles.scopeOption} onClick={() => switchScope('group')} role="menuitem" type="button"><Layers3 aria-hidden="true" size={15} /><span>集团总览<small>跨公司入口</small></span></button>
          </div> : null}
        </div>

        <nav className={styles.navSection} aria-label={isGroupScope ? '集团导航' : '公司工作台'}>
          <span className={styles.navLabel}>{isGroupScope ? '集团' : '公司'}</span>
          <div className={styles.navList}>
            {navItems.map(item => {
              const Icon = item.icon;
              const path = isGroupScope ? '/group/' + item.path : '/companies/' + companyId + '/' + item.path;
              return <NavLink aria-label={item.label} className={({isActive}) => styles.navButton + ' ' + (isActive ? styles.navButtonActive : '')} end key={path} to={path}><span className={styles.navIcon}><Icon aria-hidden="true" size={16} /></span><span>{item.label}</span></NavLink>;
            })}
          </div>
        </nav>

        <div className={styles.sidebarFooter}>
          <button aria-label={theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'} className={styles.themeToggle} onClick={toggleTheme} type="button">
            <ThemeIcon className={styles.icon} aria-hidden="true" size={16} /><span>{theme === 'dark' ? '浅色主题' : '深色主题'}</span>
          </button>
        </div>
      </aside>

      <div className={styles.appFrame}>
        <header className={styles.topbar}>
          <div className={styles.breadcrumbs}><span>{isGroupScope ? '集团' : 'Polis'}</span><span>/</span><strong>{isGroupScope ? groupViewLabel : companyName}</strong></div>
          <div className={styles.topbarActions}>
            <span aria-label={`管理入口：${managementAccess.label}`} className={styles.managementAccessPill} data-mode={managementAccess.mode} role="status">{managementAccess.label}</span>
            <span className={styles.readOnlyPill}><span className={styles.readOnlyDot} />只读 · {modeLabel} · {formatObservedAt(observedAt)} · {labelRecoveryState(recoveryState)}</span>
          </div>
        </header>
        <main className={styles.content} id="main-content">{children}</main>
      </div>
    </div>
  );
}

// pattern: Imperative Shell

import {CircleAlert, Filter, Radio} from 'lucide-react';
import {useState} from 'react';
import type {ActivityStreamStatus, WorkbenchApi} from '../data/workbench-api';
import {useActivityEvents} from '../data/workbench-query';
import {filterActivityEvents} from '../domain/activity-presentation';
import {labelErrorMessage} from '../domain/display-labels';
import {ActivityTimeline} from '../components/activity-timeline/ActivityTimeline';
import styles from '../styles/workbench.module.css';
import pageStyles from './ActivityPage.module.css';

type ActivityPageProps = Readonly<{
  api: WorkbenchApi;
  companyId: string;
  snapshotCursor: string | null;
  streamStatus: ActivityStreamStatus;
}>;

type ActivityFilter = 'all' | 'collaboration' | 'lifecycle' | 'evidence';

const FILTERS: ReadonlyArray<Readonly<{key: ActivityFilter; label: string}>> = [
  {key: 'all', label: '全部活动'},
  {key: 'collaboration', label: '协作与责任'},
  {key: 'lifecycle', label: '生命周期'},
  {key: 'evidence', label: '合同与证据'},
];

function ViewHeader() {
  return (
    <div className={styles.viewHeader}>
      <div>
        <p className={styles.eyebrow}>公司 / 活动</p>
        <h1 className={styles.pageTitle}>活动时间线</h1>
      </div>
    </div>
  );
}

function LoadingState() {
  return <div className={styles.emptyState} role="status"><Radio aria-hidden="true" className={pageStyles.loadingIcon} size={18} /><span>正在读取事件快照</span></div>;
}

function ErrorState({message}: Readonly<{message: string}>) {
  return <div className={styles.errorState} role="alert"><CircleAlert aria-hidden="true" size={18} /><div><strong>活动读取失败</strong><p>{labelErrorMessage(message)}</p></div></div>;
}

export function ActivityPage({api, companyId, snapshotCursor}: ActivityPageProps) {
  const [filter, setFilter] = useState<ActivityFilter>('all');
  const [cursor, setCursor] = useState<string | null>(null);
  const query = useActivityEvents(api, companyId, cursor, 50, snapshotCursor);

  if (query.isPending) return <div className={styles.viewStack}><ViewHeader /><LoadingState /></div>;
  if (query.isError) return <div className={styles.viewStack}><ViewHeader /><ErrorState message={query.error.message} /></div>;

  const events = filterActivityEvents(query.data.items, filter);
  return (
    <div className={styles.viewStack} data-od-id="activity-timeline-view">
      <ViewHeader />
      <section className={styles.activityToolbar} data-od-id="activity-filter-toolbar">
        <div className={styles.toolbarLabel}><Filter aria-hidden="true" className={styles.icon} size={15} />筛选时间线</div>
        <div className={styles.filterGroup} role="group" aria-label="活动类型">
          {FILTERS.map(item => <button aria-pressed={filter === item.key} className={`${styles.filterButton} ${filter === item.key ? styles.filterButtonActive : ''}`} key={item.key} onClick={() => setFilter(item.key)} type="button">{item.label}</button>)}
        </div>
      </section>
      <section className={styles.activityLayout} data-od-id="activity-timeline-content">
        {events.length > 0 ? <ActivityTimeline events={events} /> : <div className={styles.emptyState} role="status"><span>当前筛选暂无事件。</span></div>}
        <div className={pageStyles.paginationBar}>
          {query.data.nextCursor !== null ? <><span>当前快照仍有后续活动</span><button className={styles.commandButton} onClick={() => setCursor(query.data.nextCursor)} type="button">读取下一页</button></> : <span>已到当前快照末尾</span>}
        </div>
      </section>
    </div>
  );
}

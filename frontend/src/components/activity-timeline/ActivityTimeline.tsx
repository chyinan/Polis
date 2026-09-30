// pattern: Imperative Shell

import {useState} from 'react';
import {ChevronDown} from 'lucide-react';
import type {ActivityEvent} from '../../domain/workbench';
import {formatEventTime, getActivityPresentation} from '../../domain/activity-presentation';
import {labelActivityKind, labelActivityText} from '../../domain/display-labels';
import {StatusBadge} from '../status-badge/StatusBadge';
import surfaceStyles from '../../styles/workbench.module.css';
import styles from './ActivityTimeline.module.css';

type ActivityTimelineProps = Readonly<{
  events: ReadonlyArray<ActivityEvent>;
  compact?: boolean;
}>;

export function ActivityTimeline({events, compact = false}: ActivityTimelineProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [copiedReference, setCopiedReference] = useState<string | null>(null);
  const [copyFailure, setCopyFailure] = useState(false);

  async function copyEvidenceReference(reference: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(reference);
      setCopiedReference(reference);
      setCopyFailure(false);
    } catch {
      setCopiedReference(null);
      setCopyFailure(true);
    }
  }

  return (
    <ol className={surfaceStyles.activityFeed} aria-label="活动时间线">
      {events.map(event => {
        const presentation = getActivityPresentation(event.kind);
        const isExpanded = expandedId === event.id;
        return (
          <li className={surfaceStyles.activityItem} data-event-kind={event.kind} key={event.id}>
            <time className={surfaceStyles.activityTime} dateTime={event.occurredAt}>{formatEventTime(event.occurredAt)}</time>
            <div className={surfaceStyles.activityBody}>
              <div className={surfaceStyles.activityTitleRow}>
                <StatusBadge compact label={presentation.label} tone={presentation.tone} />
                <h3>{labelActivityText(event.summary)}</h3>
              </div>
              <p>{labelActivityText(event.detail)}</p>
              <div className={surfaceStyles.activityMeta}>
                <span>{labelActivityText(event.actor.label)}</span>
                <span className={surfaceStyles.activityMetaDivider}>·</span>
                <span>{labelActivityText(event.subject.label)}</span>
                <span className={surfaceStyles.activityMetaDivider}>·</span>
                <code>序列 {event.companySeq}</code>
              </div>
              {!compact ? (
                <>
                  <button aria-controls={`event-details-${event.id}`} aria-expanded={isExpanded} className={styles.detailButton} onClick={() => setExpandedId(current => current === event.id ? null : event.id)} type="button">
                    <ChevronDown aria-hidden="true" className={isExpanded ? styles.chevronOpen : ''} size={14} />
                    {isExpanded ? '收起证据与调试信息' : '展开证据与调试信息'}
                  </button>
                  {isExpanded ? (
                    <div className={styles.evidencePanel} id={`event-details-${event.id}`}>
                      <div className={styles.evidenceColumn}>
                        <div className={styles.evidenceLabel}>证据引用</div>
                        {event.evidenceRefs.length > 0 ? event.evidenceRefs.map(reference => <button aria-label={`复制证据引用 ${reference}`} className={styles.evidenceReference} key={reference} onClick={() => { void copyEvidenceReference(reference); }} title={reference} type="button"><code>{copiedReference === reference ? '已复制 · ' : ''}{reference}</code></button>) : <span className={styles.muted}>无显式引用</span>}
                        {copyFailure ? <span className={styles.copyFailure} role="status">复制失败，请手动复制引用。</span> : null}
                      </div>
                      <div className={styles.evidenceColumn}>
                        <div className={styles.evidenceLabel}>调试元数据</div>
                        {Object.entries(event.metadata).map(([key, value]) => <div className={styles.metadataRow} key={key}><span>{labelActivityKind(key)}</span><code>{labelActivityText(value)}</code></div>)}
                      </div>
                    </div>
                  ) : null}
                </>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

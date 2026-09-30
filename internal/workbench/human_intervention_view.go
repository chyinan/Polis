// pattern: Functional Core
package workbench

import "strings"

type humanInterventionRow struct {
	ID                string
	Severity          string
	ReasonCode        string
	ProtectionAction  string
	RequiredAction    string
	WorkflowState     string
	NotificationState string
	EvidenceRefs      []string
}

func humanInterventionAttention(row humanInterventionRow) AttentionItem {
	reason := humanInterventionReasonLabels[row.ReasonCode]
	if reason == "" {
		reason = "存在需要管理员处理的问题"
	}
	protection := humanInterventionProtectionLabels[row.ProtectionAction]
	if protection == "" {
		protection = "保护状态不可得"
	}
	action := humanInterventionActionLabels[row.RequiredAction]
	if action == "" {
		action = "请在工作台核对当前状态"
	}
	incidentID := row.ID
	if len(incidentID) > 8 {
		incidentID = strings.ToUpper(incidentID[:8])
	}
	return AttentionItem{
		ID:                "intervention-" + row.ID,
		Tone:              map[bool]string{true: "danger", false: "warning"}[row.Severity == "critical"],
		Title:             "需要管理员接管",
		Description:       reason + " " + protection + "；" + action,
		Subject:           EntityRef{Kind: "human_intervention", ID: row.ID, Label: "INC-" + incidentID},
		EvidenceRefs:      append([]string{}, row.EvidenceRefs...),
		WorkflowState:     row.WorkflowState,
		NotificationState: row.NotificationState,
	}
}

var humanInterventionReasonLabels = map[string]string{
	"credential_unavailable":   "执行凭据不可用",
	"provider_quota_exhausted": "执行预算已到边界",
	"external_outcome_unknown": "外部操作结果无法确认",
	"capability_revoked":       "能力授权已撤销",
	"handover_required":        "任务需要人工接管",
	"environment_not_ready":    "受控环境尚未就绪",
}

var humanInterventionProtectionLabels = map[string]string{
	"execution_paused":  "相关执行已暂停",
	"old_writer_fenced": "旧写者已隔离",
	"readonly_mode":     "已保持只读",
	"no_mutation":       "未执行外部写入",
}

var humanInterventionActionLabels = map[string]string{
	"reauthorize_in_workbench": "请在工作台检查并重新授权",
	"review_unknown_outcome":   "请在工作台核对结果后决定后续操作",
	"resume_or_cancel":         "请在工作台恢复或结束使命",
	"supply_missing_input":     "请在工作台补充资料",
}

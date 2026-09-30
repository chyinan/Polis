// pattern: Functional Core
package qqnotify

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	AdapterName             = "qq_official"
	MaxInterventionRunes    = 700
	MaxTargetOpenIDRunes    = 128
	MaxRetryAfterSeconds    = 900
	MaxNotificationAttempts = 3
)

type Outcome string

const (
	OutcomeProviderAccepted Outcome = "provider_accepted"
	OutcomeRetryWait        Outcome = "retry_wait"
	OutcomeRejected         Outcome = "rejected"
	OutcomeUnknown          Outcome = "outcome_unknown"
)

type Route struct {
	Adapter        string
	Enabled        bool
	Status         string
	TargetOpenID   string
	CredentialRef  string
	Revision       int64
	QualifiedUntil time.Time
	MaxAttempts    int
}

type Intervention struct {
	CompanyAlias     string
	IncidentID       string
	Severity         string
	ReasonCode       string
	ProtectionAction string
	RequiredAction   string
	ObservedAt       time.Time
}

func ValidateRoute(route Route, now time.Time) error {
	if route.Adapter != AdapterName || !route.Enabled || route.Status != "ready" || route.Revision <= 0 || route.QualifiedUntil.IsZero() || !now.Before(route.QualifiedUntil) || route.MaxAttempts < 1 || route.MaxAttempts > MaxNotificationAttempts {
		return errors.New("QQ route is disabled, unqualified, expired, or outside the bounded retry policy")
	}
	if !validTarget(route.TargetOpenID) || !ValidCredentialRef(route.CredentialRef) {
		return errors.New("QQ C2C route requires one explicit target and protected credential reference")
	}
	return nil
}

func ValidTargetOpenID(value string) bool {
	return validTarget(value)
}

func ValidSafetyAlias(value string) bool {
	return validAlias(value)
}

func BuildInterventionMessage(intervention Intervention) (string, error) {
	if !validAlias(intervention.CompanyAlias) || !validIncidentID(intervention.IncidentID) || (intervention.Severity != "high" && intervention.Severity != "critical") || intervention.ObservedAt.IsZero() {
		return "", errors.New("intervention message fields are invalid")
	}
	reason, ok := reasonText[intervention.ReasonCode]
	if !ok {
		return "", errors.New("intervention reason is not allowlisted")
	}
	protection, ok := protectionText[intervention.ProtectionAction]
	if !ok {
		return "", errors.New("intervention protection action is not allowlisted")
	}
	action, ok := requiredActionText[intervention.RequiredAction]
	if !ok {
		return "", errors.New("intervention action is not allowlisted")
	}
	message := fmt.Sprintf("[需要接管] %s · %s\n严重性：%s\n原因：%s\n已采取保护：%s\n需要你：%s\n时间：%s\n请在 Polis 工作台查看；此 QQ 通道不处理回复指令。",
		intervention.CompanyAlias, intervention.IncidentID, intervention.Severity, reason, protection, action,
		intervention.ObservedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	if len([]rune(message)) > MaxInterventionRunes {
		return "", errors.New("intervention message exceeds the bounded text limit")
	}
	return message, nil
}

func validTarget(value string) bool {
	if strings.TrimSpace(value) != value || value == "" || len([]rune(value)) > MaxTargetOpenIDRunes || strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "://") {
		return false
	}
	return strings.IndexFunc(value, func(character rune) bool { return unicode.IsControl(character) || unicode.IsSpace(character) }) < 0
}

func validAlias(value string) bool {
	if strings.TrimSpace(value) != value || value == "" || len([]rune(value)) > 40 {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune(" _-.·", character) {
			continue
		}
		return false
	}
	return true
}

func validIncidentID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return true
}

var reasonText = map[string]string{
	"credential_unavailable":   "执行凭据不可用，相关执行已暂停。",
	"provider_quota_exhausted": "执行预算已到边界，相关任务未继续启动。",
	"external_outcome_unknown": "外部操作回执无法确认，相关任务已保持暂停。",
	"capability_revoked":       "能力授权已撤销，新调用已被阻止。",
	"handover_required":        "当前工作需要管理员接管。",
	"environment_not_ready":    "受控执行环境尚未就绪。",
}

var protectionText = map[string]string{
	"execution_paused":  "已暂停相关执行",
	"old_writer_fenced": "旧写者已隔离",
	"readonly_mode":     "已保持只读",
	"no_mutation":       "未执行外部写入",
}

var requiredActionText = map[string]string{
	"reauthorize_in_workbench": "请在 Polis 工作台检查并重新授权。",
	"review_unknown_outcome":   "请在 Polis 工作台核对外部结果，再选择后续操作。",
	"resume_or_cancel":         "请在 Polis 工作台恢复或结束本次使命。",
	"supply_missing_input":     "请在 Polis 工作台补充缺少的资料。",
}

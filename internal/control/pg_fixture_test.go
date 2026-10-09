// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"fmt"
	"polis/internal/kernel"
	"polis/internal/organization"
	"polis/internal/taskvalidation"
	"polis/spec"
)

func pgFixtureKey(prefix, id string) string {
	return fmt.Sprintf("%s-%x", prefix, sha256.Sum256([]byte(id)))[:64]
}

func pgTestAcceptance() *taskvalidation.AcceptanceContract {
	return &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task summary:"}}
}
func createPGTestCompany(k *kernel.Kernel, ctx context.Context, id string) (kernel.Scope, error) {
	s, err := k.TXCreateCompany(ctx, id)
	if err != nil {
		return s, err
	}
	details, err := k.CompanyDetails(ctx, id)
	if err != nil {
		return s, err
	}
	_, err = k.TXUpdateCompanyWithOrganizationAndCoverageConfirmation(ctx, organization.CompanyDraft{ID: id, Name: details.Name, WorkspaceRoot: details.WorkspaceRoot, Roster: details.Roster}, pgFixtureKey("test-owner", id), spec.FixedTeamCoverageSHA256())
	if err != nil {
		return s, err
	}
	slots, err := k.GetInstallationWorkerSlotPolicy(ctx, k.LocalInstallationOwnerScope())
	if err != nil {
		return s, err
	}
	if !slots.Configured {
		_, err = k.TXSetInstallationWorkerSlotPolicy(ctx, k.LocalInstallationOwnerScope(), kernel.InstallationWorkerSlotPolicyInput{RequestID: pgFixtureKey("test-slots", id), ExpectedRevision: slots.Revision, MaxActiveSlots: 4, ProtectedSlots: 1})
		if err != nil {
			return s, err
		}
	}
	budget, err := k.GetCompanyToolCallBudget(ctx, s)
	if err != nil {
		return s, err
	}
	if budget.ToolCallLimit == nil {
		_, err = k.TXChangeCompanyToolCallBudget(ctx, s, kernel.CompanyToolCallBudgetChangeInput{RequestID: pgFixtureKey("test-company-budget", id), ExpectedRevision: budget.Revision, ResultingLimit: 2000, Reason: "explicit bounded offline integration fixture"})
	}
	return s, err
}
func configurePGTestMission(k *kernel.Kernel, ctx context.Context, s kernel.Scope, id string) error {
	budgets, err := k.ListMissionToolCallBudgets(ctx, s, 100)
	if err != nil {
		return err
	}
	for _, b := range budgets.Items {
		if b.MissionID == id {
			if b.ToolCallLimit != nil {
				return nil
			}
			_, err = k.TXChangeMissionToolCallBudget(ctx, s, id, kernel.MissionToolCallBudgetChangeInput{RequestID: pgFixtureKey("test-mission-budget", id), ExpectedRevision: b.Revision, ResultingLimit: 2000, Reason: "explicit bounded offline integration fixture"})
			return err
		}
	}
	return fmt.Errorf("fixture mission not found")
}
func createPGTestMission(k *kernel.Kernel, ctx context.Context, s kernel.Scope, id string) error {
	if err := k.TXCreateMission(ctx, s, id); err != nil {
		return err
	}
	return configurePGTestMission(k, ctx, s, id)
}
func createPGTestMissionGoal(k *kernel.Kernel, ctx context.Context, s kernel.Scope, title, goal, key string) (kernel.Receipt, error) {
	r, e := k.TXCreateMissionGoal(ctx, s, title, goal, key)
	if e == nil {
		e = configurePGTestMission(k, ctx, s, r.ID)
	}
	return r, e
}
func createPGTestMissionGoalWithAcceptance(k *kernel.Kernel, ctx context.Context, s kernel.Scope, title, goal string, a *taskvalidation.AcceptanceContract, key string) (kernel.Receipt, error) {
	r, e := k.TXCreateMissionGoalWithAcceptance(ctx, s, title, goal, a, key)
	if e == nil {
		e = configurePGTestMission(k, ctx, s, r.ID)
	}
	return r, e
}
func createPGTestProbe(k *kernel.Kernel, ctx context.Context, s kernel.Scope, id string) (kernel.Task, error) {
	r, e := k.TXCreateProbe(ctx, s, id)
	if e == nil {
		e = configurePGTestMission(k, ctx, s, r.Mission)
	}
	return r, e
}
func createPGTestPeerFixture(k *kernel.Kernel, ctx context.Context, s kernel.Scope, id string) (kernel.PeerFixture, error) {
	r, e := k.TXCreatePeerFixture(ctx, s, id)
	if e == nil {
		e = configurePGTestMission(k, ctx, s, r.Mission)
	}
	return r, e
}

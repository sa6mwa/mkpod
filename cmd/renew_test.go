package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/spf13/cobra"
)

func TestBuildRenewPlansSelectsExistingEpisode(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)

	plans, err := buildRenewPlans(context.Background(), specFile, "1")
	if err != nil {
		t.Fatalf("buildRenewPlans() error = %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want 1", len(plans))
	}
	if plans[0].SpecFile != specFile || plans[0].Episode.UID != 1 {
		t.Fatalf("plan = %+v, want spec %q episode 1", plans[0], specFile)
	}
}

func TestBuildRenewPlansAllSelectsEveryEpisode(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)

	plans, err := buildRenewPlans(context.Background(), specFile, "all")
	if err != nil {
		t.Fatalf("buildRenewPlans() error = %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want all fixture episodes", len(plans))
	}
}

func TestWriteRenewPlansWritesRenewWorkflow(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	out := filepath.Join(t.TempDir(), "renew-episode.plan.json")
	cmd := renewCmdForTest(t)
	mustSetFlag(t, cmd, "spec", specFile)
	mustSetFlag(t, cmd, "out", out)

	paths, err := writeRenewPlans(context.Background(), cmd, "1")
	if err != nil {
		t.Fatalf("writeRenewPlans() error = %v", err)
	}
	if len(paths) != 1 || paths[0] != out {
		t.Fatalf("paths = %v, want [%s]", paths, out)
	}
	saved, err := loadSavedPlan(out)
	if err != nil {
		t.Fatalf("loadSavedPlan() error = %v", err)
	}
	if saved.Workflow != "renew" {
		t.Fatalf("workflow = %q, want renew", saved.Workflow)
	}
}

func TestInspectSavedRenewPlanUsesApplyDecision(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "renew.plan.json")
	writeSavedPlanFixture(t, planPath, "renew", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureFromSpec(t, specFile, 1),
	})

	if err := inspectSavedPlan(planPath); err != nil {
		t.Fatalf("inspectSavedPlan() error = %v", err)
	}
}

func TestApplyRenewPlanRejectsMissingExistingEpisode(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "renew.plan.json")
	writeSavedPlanFixture(t, planPath, "renew", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})

	err := applySavedPlanWithOptions(context.Background(), planPath, applySavedPlanOptions{JustMaster: true, Yes: true, Storage: &fakeApplyStorage{}})
	if err == nil {
		t.Fatal("applySavedPlanWithOptions() error = nil, want missing existing episode error")
	}
	if !strings.Contains(err.Error(), "current metadata conflicts with plan") {
		t.Fatalf("applySavedPlanWithOptions() error = %v, want metadata conflict", err)
	}
}

func episodeFixtureFromSpec(t *testing.T, specFile string, uid int64) model.Episode {
	t.Helper()
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	index := atom.ContainsEpisode(uid)
	if index < 0 {
		t.Fatalf("episode %d missing in fixture", uid)
	}
	return atom.Episodes[index]
}

func renewCmdForTest(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	addPlanOutputFlag(cmd)
	return cmd
}

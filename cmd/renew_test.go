package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/spec"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
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

func TestRenewApplyDecisionPreservesInheritedMetadata(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	episode := episodeFixtureFromSpec(t, specFile, 1)
	episode.Author = ""
	episode.Image = ""
	if err := savePodcastSpec(specFile, atom); err != nil {
		t.Fatalf("save fixture spec: %v", err)
	}
	atom.Episodes[0] = episode
	if err := savePodcastSpec(specFile, atom); err != nil {
		t.Fatalf("save inherited fixture spec: %v", err)
	}

	decision, err := buildRenewEpisodeApplyDecision(context.Background(), &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episode,
	}, applyPreflightOptions{JustMaster: true}, &applyPreflightRemote{})
	if err != nil {
		t.Fatalf("buildRenewEpisodeApplyDecision() error = %v", err)
	}
	for _, check := range decision.Checks {
		if check.Label == "podspec metadata" && !check.Passed {
			t.Fatalf("metadata check = %+v, want inherited raw metadata to match", check)
		}
	}
	if strings.Contains(strings.Join(decisionReasons(decision), "\n"), "current metadata conflicts with plan") {
		t.Fatalf("decision = %+v, want no metadata conflict", decision)
	}
}

func TestApplyRenewEpisodePlanPreservesInheritedMetadata(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	episode := atom.Episodes[0]
	episode.Link = "https://example.com/episode"
	episode.Subtitle = "Episode subtitle"
	episode.Description = "Episode description"
	episode.Author = ""
	episode.Image = ""
	atom.Episodes[0] = episode
	if err := savePodcastSpec(specFile, atom); err != nil {
		t.Fatalf("save inherited fixture spec: %v", err)
	}

	if err := applyRenewEpisodePlan(context.Background(), &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episode,
	}); err != nil {
		t.Fatalf("applyRenewEpisodePlan() error = %v, want inherited metadata match", err)
	}
}

func TestRenewPlansRejectStaleGeneratedProductionMetadata(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.Episode)
	}{
		{
			name: "output",
			mutate: func(episode *model.Episode) {
				episode.Output = "episode-reencoded.m4a"
			},
		},
		{
			name: "type",
			mutate: func(episode *model.Episode) {
				episode.Type = "audio/mp4"
			},
		},
		{
			name: "length",
			mutate: func(episode *model.Episode) {
				episode.Length = 999
			},
		},
		{
			name: "duration",
			mutate: func(episode *model.Episode) {
				episode.Duration = model.ItunesDuration{Duration: 2 * time.Minute}
			},
		},
		{
			name: "pubdate",
			mutate: func(episode *model.Episode) {
				episode.PubDate = model.ItunesTime{Time: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			specFile := writeWorkflowSpecFixture(t)
			atom, err := specStoreLoadForTest(t, specFile)
			if err != nil {
				t.Fatalf("load spec: %v", err)
			}
			episode := atom.Episodes[0]
			episode.Link = "https://example.com/episode"
			episode.Subtitle = "Episode subtitle"
			episode.Description = "Episode description"
			episode.Type = "audio/mpeg"
			episode.Length = 123
			episode.Duration = model.ItunesDuration{Duration: time.Minute}
			episode.PubDate = model.ItunesTime{Time: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)}
			atom.Episodes[0] = episode
			if err := savePodcastSpec(specFile, atom); err != nil {
				t.Fatalf("save initial fixture spec: %v", err)
			}

			stalePlanEpisode := episode
			tt.mutate(&atom.Episodes[0])
			if err := savePodcastSpec(specFile, atom); err != nil {
				t.Fatalf("save mutated fixture spec: %v", err)
			}

			decision, err := buildRenewEpisodeApplyDecision(context.Background(), &newEpisodePlan{
				SpecFile: specFile,
				Episode:  stalePlanEpisode,
			}, applyPreflightOptions{JustMaster: true}, &applyPreflightRemote{})
			if err != nil {
				t.Fatalf("buildRenewEpisodeApplyDecision() error = %v", err)
			}
			if decision.State != workflow.StateBlocked {
				t.Fatalf("State = %q, want blocked for stale %s metadata", decision.State, tt.name)
			}
			if !strings.Contains(strings.Join(decisionReasons(decision), "\n"), "current metadata conflicts with plan") {
				t.Fatalf("decision = %+v, want metadata conflict for stale %s metadata", decision, tt.name)
			}

			err = applyRenewEpisodePlan(context.Background(), &newEpisodePlan{
				SpecFile: specFile,
				Episode:  stalePlanEpisode,
			})
			if err == nil {
				t.Fatalf("applyRenewEpisodePlan() error = nil, want stale %s metadata rejection", tt.name)
			}
			if !strings.Contains(err.Error(), "already exists with different metadata") {
				t.Fatalf("applyRenewEpisodePlan() error = %v, want different metadata", err)
			}
		})
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

func decisionReasons(decision workflow.Decision) []string {
	reasons := make([]string, 0, len(decision.Checks)+len(decision.Operations))
	for _, check := range decision.Checks {
		reasons = append(reasons, check.Reason)
	}
	for _, operation := range decision.Operations {
		reasons = append(reasons, operation.Reason)
	}
	return reasons
}

func renewCmdForTest(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	addPlanOutputFlag(cmd)
	return cmd
}

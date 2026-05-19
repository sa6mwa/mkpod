package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/spf13/cobra"
)

var renewCmd = &cobra.Command{
	Use:   "renew <uid|all>",
	Short: "Create apply plans for existing podcast episodes",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("provide exactly one episode UID or all")
		}
		if strings.EqualFold(args[0], "all") {
			return nil
		}
		uid, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || uid <= 0 {
			return fmt.Errorf("episode selector must be a positive UID or all")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		paths, err := writeRenewPlans(context.Background(), cmd, args[0])
		if err != nil {
			l.Error("Unable to create renew plan", "error", err)
			os.Exit(1)
		}
		for _, path := range paths {
			fmt.Printf("Apply with: mkpod apply %s\n", shellescape.Quote(path))
		}
	},
}

func writeRenewPlans(ctx context.Context, cmd *cobra.Command, selector string) ([]string, error) {
	specFile, err := cmd.Flags().GetString("spec")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Flags().GetString("out")
	if err != nil {
		return nil, err
	}
	plans, err := buildRenewPlans(ctx, specFile, selector)
	if err != nil {
		return nil, err
	}
	if len(plans) == 1 {
		path := strings.TrimSpace(out)
		if path == "" {
			path = defaultRenewPlanPath(specFile, plans[0].Episode.UID)
		}
		if err := writeSavedWorkflowPlan(path, "renew", plans[0]); err != nil {
			return nil, err
		}
		printEpisodeMutationPlan("renew", plans[0])
		fmt.Printf("Wrote plan: %s\n", path)
		return []string{path}, nil
	}

	dir := strings.TrimSpace(out)
	if dir == "" {
		dir = filepath.Dir(specFile)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(plans))
	for _, plan := range plans {
		path := filepath.Join(dir, renewPlanFilename(plan.Episode.UID))
		if err := writeSavedWorkflowPlan(path, "renew", plan); err != nil {
			return nil, err
		}
		printEpisodeMutationPlan("renew", plan)
		fmt.Printf("Wrote plan: %s\n", path)
		paths = append(paths, path)
	}
	return paths, nil
}

func buildRenewPlans(ctx context.Context, specFile, selector string) ([]*newEpisodePlan, error) {
	atom, err := spec.New(specFile).Load(ctx)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(selector, "all") {
		plans := make([]*newEpisodePlan, 0, len(atom.Episodes))
		for i := range atom.Episodes {
			episode := atom.Episodes[i]
			plans = append(plans, &newEpisodePlan{SpecFile: specFile, Episode: episode})
		}
		return plans, nil
	}
	uid, err := strconv.ParseInt(selector, 10, 64)
	if err != nil || uid <= 0 {
		return nil, fmt.Errorf("episode selector must be a positive UID or all")
	}
	index := atom.ContainsEpisode(uid)
	if index < 0 {
		return nil, fmt.Errorf("episode UID %d does not exist in podcast specification", uid)
	}
	episode := atom.Episodes[index]
	return []*newEpisodePlan{{SpecFile: specFile, Episode: episode}}, nil
}

func writeSavedWorkflowPlan(path, workflow string, plan any) error {
	planContent, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(savedPlan{Workflow: workflow, Plan: planContent}, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}

func defaultRenewPlanPath(specFile string, uid int64) string {
	return filepath.Join(filepath.Dir(specFile), renewPlanFilename(uid))
}

func renewPlanFilename(uid int64) string {
	return fmt.Sprintf("renew-%d.plan.json", uid)
}

func init() {
	rootCmd.AddCommand(renewCmd)
	renewCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	addPlanOutputFlag(renewCmd)
}

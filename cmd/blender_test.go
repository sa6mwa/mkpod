package cmd

import "testing"

func TestBlenderCommandIsDirectInstaller(t *testing.T) {
	if blenderCmd.Use != "blender" {
		t.Fatalf("blender Use = %q, want blender", blenderCmd.Use)
	}
	for _, flag := range []string{"blender", "repo"} {
		if blenderCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("blender flag %q is missing", flag)
		}
	}
}

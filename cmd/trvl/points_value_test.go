package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/points"
)

func TestPointsValueCmd_RequiresProgram(t *testing.T) {
	cmd := pointsValueCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"--cash", "450", "--points", "20000"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing program error")
	}
	if !strings.Contains(err.Error(), "--program is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPointsValueCmd_ListJSON(t *testing.T) {
	stdout, stderr, err := captureTripCostOutput(t, func() error {
		cmd := pointsValueCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"--list", "--format", "json"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("pointsValueCmd list failed: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var programs []points.Program
	if err := json.Unmarshal([]byte(stdout), &programs); err != nil {
		t.Fatalf("unmarshal programs: %v\nstdout=%s", err, stdout)
	}
	if len(programs) != len(points.Programs) {
		t.Fatalf("program count = %d, want %d", len(programs), len(points.Programs))
	}

	found := map[string]bool{
		"finnair-plus":   false,
		"world-of-hyatt": false,
		"amex-mr":        false,
	}
	for _, program := range programs {
		if _, ok := found[program.Slug]; ok {
			found[program.Slug] = true
		}
	}
	for slug, ok := range found {
		if !ok {
			t.Fatalf("expected program slug %q in list output", slug)
		}
	}
}

func TestPointsValueCmd_SuccessJSON(t *testing.T) {
	stdout, stderr, err := captureTripCostOutput(t, func() error {
		cmd := pointsValueCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{
			"--cash", "450",
			"--points", "20000",
			"--program", "finnair-plus",
			"--format", "json",
		})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("pointsValueCmd failed: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var rec points.Recommendation
	if err := json.Unmarshal([]byte(stdout), &rec); err != nil {
		t.Fatalf("unmarshal recommendation: %v\nstdout=%s", err, stdout)
	}
	if rec.ProgramSlug != "finnair-plus" {
		t.Fatalf("program_slug = %q, want finnair-plus", rec.ProgramSlug)
	}
	if rec.ProgramName != "Finnair Plus" {
		t.Fatalf("program_name = %q, want Finnair Plus", rec.ProgramName)
	}
	if rec.Verdict != "use points" {
		t.Fatalf("verdict = %q, want use points", rec.Verdict)
	}
}

func TestPrintRecommendation_RendersTable(t *testing.T) {
	oldUseColor := models.UseColor
	models.UseColor = false
	defer func() { models.UseColor = oldUseColor }()

	stdout, stderr, err := captureTripCostOutput(t, func() error {
		printRecommendation(&points.Recommendation{
			ProgramSlug:    "finnair-plus",
			ProgramName:    "Finnair Plus",
			CashPrice:      450,
			PointsRequired: 20000,
			CPP:            2.25,
			FloorCPP:       1.0,
			CeilingCPP:     2.2,
			Verdict:        "use points",
			Explanation:    "Excellent redemption.",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("printRecommendation failed: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	for _, needle := range []string{
		"Points vs Cash",
		"Program: Finnair Plus",
		"20,000",
		"2.25¢/pt",
		"use points",
		"Excellent redemption.",
	} {
		if !strings.Contains(stdout, needle) {
			t.Fatalf("stdout missing %q:\n%s", needle, stdout)
		}
	}
}

func TestFormatPoints(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{999, "999"},
		{1000, "1,000"},
		{20000, "20,000"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		if got := formatPoints(tt.input); got != tt.want {
			t.Fatalf("formatPoints(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

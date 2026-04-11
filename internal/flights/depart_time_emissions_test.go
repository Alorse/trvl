package flights

import (
	"testing"
)

// --- parseHour ---

func TestParseHour(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"06:00", 6},
		{"00:00", 0},
		{"23:59", 23},
		{"12:30", 12},
		// Invalid inputs return -1.
		{"", -1},
		{"6:00", -1}, // single digit hour — no leading zero
		{"24:00", -1},
		{"ab:cd", -1},
		{"06", -1},
		{"06:00:00", -1},
	}

	for _, tt := range tests {
		got := parseHour(tt.input)
		if got != tt.want {
			t.Errorf("parseHour(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- departTimeWindow ---

func TestDepartTimeWindow_BothSet(t *testing.T) {
	got := departTimeWindow("06:00", "22:00")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected len 2, got %d", len(arr))
	}
	if arr[0] != 6 {
		t.Errorf("startHour = %v, want 6", arr[0])
	}
	if arr[1] != 22 {
		t.Errorf("endHour = %v, want 22", arr[1])
	}
}

func TestDepartTimeWindow_OnlyAfter(t *testing.T) {
	got := departTimeWindow("08:00", "")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if arr[0] != 8 {
		t.Errorf("startHour = %v, want 8", arr[0])
	}
	if arr[1] != 24 {
		t.Errorf("endHour = %v, want 24 (sentinel for unset)", arr[1])
	}
}

func TestDepartTimeWindow_OnlyBefore(t *testing.T) {
	got := departTimeWindow("", "20:00")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if arr[0] != 0 {
		t.Errorf("startHour = %v, want 0 (sentinel for unset)", arr[0])
	}
	if arr[1] != 20 {
		t.Errorf("endHour = %v, want 20", arr[1])
	}
}

func TestDepartTimeWindow_NeitherSet(t *testing.T) {
	got := departTimeWindow("", "")
	if got != nil {
		t.Errorf("expected nil when neither bound set, got %v", got)
	}
}

func TestDepartTimeWindow_InvalidInputs(t *testing.T) {
	// Malformed values should be treated as unset.
	got := departTimeWindow("bad", "also-bad")
	if got != nil {
		t.Errorf("expected nil for both-invalid inputs, got %v", got)
	}

	// One valid, one invalid: only the valid side should be applied.
	got = departTimeWindow("06:00", "bad")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if arr[0] != 6 {
		t.Errorf("startHour = %v, want 6", arr[0])
	}
	if arr[1] != 24 {
		t.Errorf("endHour = %v, want 24 (sentinel)", arr[1])
	}
}

// --- emissionsFilter ---

func TestEmissionsFilter_True(t *testing.T) {
	got := emissionsFilter(true)
	if got != 1 {
		t.Errorf("emissionsFilter(true) = %v, want 1", got)
	}
}

func TestEmissionsFilter_False(t *testing.T) {
	got := emissionsFilter(false)
	if got != nil {
		t.Errorf("emissionsFilter(false) = %v, want nil", got)
	}
}

// --- buildSegment integration: depart time and emissions wired correctly ---

func TestBuildSegment_DepartWindow_WiredToPosition2(t *testing.T) {
	opts := SearchOptions{
		DepartAfter:  "06:00",
		DepartBefore: "22:00",
	}
	seg := buildSegment("HEL", "NRT", "2026-06-01", opts)
	arr := seg.([]any)

	window, ok := arr[2].([]any)
	if !ok {
		t.Fatalf("segment[2] expected []any, got %T", arr[2])
	}
	if window[0] != 6 || window[1] != 22 {
		t.Errorf("window = %v, want [6 22]", window)
	}
}

func TestBuildSegment_NoWindow_Position2IsNil(t *testing.T) {
	opts := SearchOptions{}
	seg := buildSegment("HEL", "NRT", "2026-06-01", opts)
	arr := seg.([]any)

	if arr[2] != nil {
		t.Errorf("segment[2] expected nil when no window, got %v", arr[2])
	}
}

func TestBuildSegment_Emissions_WiredToPosition13(t *testing.T) {
	opts := SearchOptions{LessEmissions: true}
	seg := buildSegment("HEL", "NRT", "2026-06-01", opts)
	arr := seg.([]any)

	if arr[13] != 1 {
		t.Errorf("segment[13] expected 1 (emissions), got %v", arr[13])
	}
}

func TestBuildSegment_NoEmissions_Position13IsNil(t *testing.T) {
	opts := SearchOptions{}
	seg := buildSegment("HEL", "NRT", "2026-06-01", opts)
	arr := seg.([]any)

	if arr[13] != nil {
		t.Errorf("segment[13] expected nil when emissions not set, got %v", arr[13])
	}
}

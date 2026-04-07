package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/trip"
)

func captureTripCostOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	runErr := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	stdout, readErr := io.ReadAll(stdoutReader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	stderr, readErr := io.ReadAll(stderrReader)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}

	return string(stdout), string(stderr), runErr
}

func TestPrintTripCostTable_PartialFailureWarning(t *testing.T) {
	result := &trip.TripCostResult{
		Success:   true,
		Flights:   trip.FlightCost{Outbound: 100, Return: 120, Currency: "EUR"},
		Hotels:    trip.HotelCost{Currency: "EUR"},
		Total:     220,
		Currency:  "EUR",
		PerPerson: 220,
		PerDay:    110,
		Nights:    2,
		Error:     "partial failure: hotels: hotel error",
	}

	stdout, stderr, err := captureTripCostOutput(t, func() error {
		return printTripCostTable(result, "HEL", "BCN", 1)
	})
	if err != nil {
		t.Fatalf("printTripCostTable returned error: %v", err)
	}
	if !strings.Contains(stderr, "Warning: partial failure: hotels: hotel error") {
		t.Fatalf("stderr = %q, want partial warning", stderr)
	}
	if !strings.Contains(stdout, "Total") {
		t.Fatalf("stdout = %q, want rendered table", stdout)
	}
}

func TestPrintTripCostTable_UnavailableComponents(t *testing.T) {
	result := &trip.TripCostResult{
		Success:   true,
		Flights:   trip.FlightCost{Outbound: 100, Currency: "EUR"},
		Hotels:    trip.HotelCost{Currency: "EUR"},
		Total:     100,
		Currency:  "EUR",
		PerPerson: 100,
		PerDay:    50,
		Nights:    2,
	}

	stdout, stderr, err := captureTripCostOutput(t, func() error {
		return printTripCostTable(result, "HEL", "BCN", 1)
	})
	if err != nil {
		t.Fatalf("printTripCostTable returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Unavailable") {
		t.Fatalf("stdout = %q, want Unavailable markers", stdout)
	}
	if strings.Contains(stdout, "0/night") {
		t.Fatalf("stdout = %q, want no 0/night detail", stdout)
	}
}

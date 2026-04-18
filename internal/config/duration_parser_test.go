package config

import (
	"testing"
	"time"
)

func TestParseDurationWithDays(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"5h", 5 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"7d", 168 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"2h30m", 2*time.Hour + 30*time.Minute, false},
		{"0.5d", 12 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDurationWithDays(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFloatOptional(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"3.5", 3.5, false},
		{"0", 0, false},
		{"", 0, false},
		{"  ", 0, false},
		{"10", 10.0, false},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFloatOptional(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseModelLimitWindows_EmptyPriceFields(t *testing.T) {
	limits := []ModelLimitWindow{
		{
			Window:       "1h",
			Model:        "gpt-4",
			InputTokens:  "1k",
			OutputTokens: "1k",
			// Price fields left empty — must not cause the window to be skipped
		},
	}
	result := parseModelLimitWindows(limits, "test")
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].InputTokens != 1000 {
		t.Fatalf("input_tokens = %d, want 1000", result[0].InputTokens)
	}
	if result[0].OutputTokens != 1000 {
		t.Fatalf("output_tokens = %d, want 1000", result[0].OutputTokens)
	}
	if result[0].InputPriceM != 0 {
		t.Fatalf("input_price_m = %v, want 0", result[0].InputPriceM)
	}
	if result[0].OutputPriceM != 0 {
		t.Fatalf("output_price_m = %v, want 0", result[0].OutputPriceM)
	}
	if result[0].Price != 0 {
		t.Fatalf("price = %v, want 0", result[0].Price)
	}
}

func TestParseModelLimitWindows_WithPriceFields(t *testing.T) {
	limits := []ModelLimitWindow{
		{
			Window:       "1d",
			Model:        "claude-3",
			InputTokens:  "5m",
			OutputTokens: "2m",
			InputPriceM:  "3.0",
			OutputPriceM: "15.0",
			Price:        "50",
		},
	}
	result := parseModelLimitWindows(limits, "test")
	if len(result) != 1 {
		t.Fatalf("expected 1 window, got %d", len(result))
	}
	if result[0].InputPriceM != 3.0 {
		t.Fatalf("input_price_m = %v, want 3.0", result[0].InputPriceM)
	}
	if result[0].OutputPriceM != 15.0 {
		t.Fatalf("output_price_m = %v, want 15.0", result[0].OutputPriceM)
	}
	if result[0].Price != 50.0 {
		t.Fatalf("price = %v, want 50.0", result[0].Price)
	}
}

func TestParseModelLimitWindows_AllZeroLimitsSkipped(t *testing.T) {
	limits := []ModelLimitWindow{
		{Window: "1h", Model: "gpt-4"}, // all token/price fields empty => 0
	}
	result := parseModelLimitWindows(limits, "test")
	if len(result) != 0 {
		t.Fatalf("expected 0 windows (all-zero should be skipped), got %d", len(result))
	}
}

func TestParseTokenAmount(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"5m", 5_000_000, false},
		{"1m", 1_000_000, false},
		{"0.5m", 500_000, false},
		{"500k", 500_000, false},
		{"1k", 1_000, false},
		{"1000", 1_000, false},
		{"0", 0, false},
		{"", 0, false},
		{"abc", 0, true},
		{"5x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTokenAmount(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

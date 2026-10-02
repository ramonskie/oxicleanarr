package config

import "testing"

func TestValidate_AnalyticsThresholds(t *testing.T) {
	valid := AnalyticsConfig{
		Enabled:             true,
		StaleDays:           90,
		ROIPeriodDays:       90,
		SuggestDeletionDays: 180,
		ValueThresholds: ValueThresholdsConfig{
			Movie:   ValueThreshold{Low: 0.1, High: 0.5},
			Episode: ValueThreshold{Low: 0.5, High: 2.0},
			Show:    ValueThreshold{Low: 0.3, High: 1.0},
		},
	}

	tests := []struct {
		name      string
		mutate    func(*AnalyticsConfig)
		wantField string
	}{
		{
			name:   "valid defaults have no error",
			mutate: func(*AnalyticsConfig) {},
		},
		{
			name:      "zero stale days rejected",
			mutate:    func(a *AnalyticsConfig) { a.StaleDays = 0 },
			wantField: "analytics.stale_days",
		},
		{
			name:      "negative roi period rejected",
			mutate:    func(a *AnalyticsConfig) { a.ROIPeriodDays = -1 },
			wantField: "analytics.roi_period_days",
		},
		{
			name:      "negative suggest days rejected",
			mutate:    func(a *AnalyticsConfig) { a.SuggestDeletionDays = -5 },
			wantField: "analytics.suggest_deletion_days",
		},
		{
			name: "inverted value thresholds rejected",
			mutate: func(a *AnalyticsConfig) {
				a.ValueThresholds.Movie = ValueThreshold{Low: 0.6, High: 0.5}
			},
			wantField: "analytics.value_thresholds.movie",
		},
		{
			name: "negative value threshold rejected",
			mutate: func(a *AnalyticsConfig) {
				a.ValueThresholds.Show = ValueThreshold{Low: -0.1, High: 1.0}
			},
			wantField: "analytics.value_thresholds.show",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validStatsBaseConfig()
			analytics := valid
			tt.mutate(&analytics)
			cfg.Analytics = analytics

			err := Validate(cfg)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected valid config, got %v", err)
				}
				return
			}
			if !hasValidationField(err, tt.wantField) {
				t.Fatalf("expected validation field %q, got %v", tt.wantField, err)
			}
		})
	}
}

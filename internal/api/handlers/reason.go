package handlers

import (
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/ramonskie/oxicleanarr/internal/services/rules"
)

// FormatDeletionReason converts a structured RuleVerdict into a
// human-readable explanation for the UI.
// This is a thin wrapper around services.FormatDeletionReason for use in the handlers layer.
func FormatDeletionReason(v rules.RuleVerdict, media *models.Media) string {
	return services.FormatDeletionReason(v, media)
}

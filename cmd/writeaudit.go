package cmd

import (
	"github.com/nic0der-im/routeros-cli/internal/audit"
	"github.com/nic0der-im/routeros-cli/internal/config"
)

// appendWriteAudit best-effort logs a mutation to the NDJSON write-audit trail.
// Failures are ignored except at -v (never fail the user command).
func (a *App) appendWriteAudit(deviceName string, ev audit.Event) {
	if a == nil || !audit.Enabled() {
		return
	}
	if ev.RequestID == "" {
		ev.RequestID = a.RequestID
	}
	if ev.Profile == "" {
		ev.Profile = a.Profile
	}
	if ev.Device == "" {
		ev.Device = deviceName
	}
	if ev.EnvClass == "" {
		dev := a.deviceConfig(deviceName)
		ev.EnvClass = config.EffectiveEnvClass(dev, a.effectiveStrict())
	}
	dir := a.AuditDir
	if dir == "" {
		dir = audit.DefaultDir()
	}
	if err := audit.Append(dir, ev); err != nil {
		a.verbosef("write-audit: %v", err)
	}
}

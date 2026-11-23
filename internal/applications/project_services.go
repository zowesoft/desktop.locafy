package applications

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"locafy.dev/desktop/internal/services"
)

// ServiceRuntime is the service supervisor as project orchestration needs it.
// The interface keeps the orchestration testable without a real engine.
type ServiceRuntime interface {
	Start(id string) (services.Instance, error)
	Stop(id string) error
}

// serviceHealthWait bounds how long a started service may take to answer before
// the run reports it as failed. Initialisation happens inside Start, so this
// only covers the server binding its port.
var serviceHealthWait = 60 * time.Second

// startServices brings the project's services up, in name order, and waits for
// each one to answer before returning. Applications start only after this
// returns, so an application never races its database.
func (s *Supervisor) startServices(projectID string, report *ProjectReport) {
	if s.services == nil {
		return
	}
	instances, err := services.List(s.database, projectID)
	if err != nil {
		report.Failed = append(report.Failed, fmt.Sprintf("services: %s", err))
		return
	}
	sort.SliceStable(instances, func(left, right int) bool {
		return strings.ToLower(instances[left].Name) < strings.ToLower(instances[right].Name)
	})

	for _, instance := range instances {
		current, err := services.Get(s.database, instance.ID)
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", instance.Name, err))
			continue
		}
		switch current.Status {
		case services.StatusRunning:
			report.AlreadyRunning = append(report.AlreadyRunning, current.Name)
			continue
		case services.StatusStarting, services.StatusStopping:
			report.Skipped = append(report.Skipped,
				fmt.Sprintf("%s: already %s", current.Name, current.Status))
			continue
		}

		started, err := s.services.Start(current.ID)
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, serviceFailure(started, err)))
			continue
		}
		if reason, ready := s.waitForService(current.ID); !ready {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, reason))
			continue
		}
		report.Started = append(report.Started, started.Name)
	}
}

// waitForService blocks until the instance answers on its port, fails, or the
// ceiling passes. A started instance that never answers is left running for the
// operator to inspect rather than killed behind their back.
func (s *Supervisor) waitForService(id string) (string, bool) {
	deadline := time.Now().Add(serviceHealthWait)
	for {
		instance, err := services.Get(s.database, id)
		if err != nil {
			return "the service could not be read", false
		}
		switch instance.Status {
		case services.StatusError:
			return serviceFailure(instance, nil), false
		case services.StatusStopped:
			return "the service stopped during startup", false
		case services.StatusRunning:
			if instance.Health == services.HealthHealthy {
				return "", true
			}
			if time.Now().After(deadline) {
				return fmt.Sprintf("%s does not answer on port %d", instance.Name, instance.Port), false
			}
		}
		if time.Now().After(deadline) {
			return "the service did not reach running in time", false
		}
		time.Sleep(readyPoll)
	}
}

// stopServices stops the project's running services after its applications, in
// reverse of their start order.
func (s *Supervisor) stopServices(projectID string, report *ProjectReport) {
	if s.services == nil {
		return
	}
	instances, err := services.List(s.database, projectID)
	if err != nil {
		report.Failed = append(report.Failed, fmt.Sprintf("services: %s", err))
		return
	}
	sort.SliceStable(instances, func(left, right int) bool {
		return strings.ToLower(instances[left].Name) > strings.ToLower(instances[right].Name)
	})

	for _, instance := range instances {
		current, err := services.Get(s.database, instance.ID)
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", instance.Name, err))
			continue
		}
		if current.Status != services.StatusRunning && current.Status != services.StatusStarting {
			continue
		}
		if err := s.services.Stop(current.ID); err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, err))
			continue
		}
		report.Stopped = append(report.Stopped, current.Name)
	}
}

// serviceFailure words a service that did not start, preferring what the
// engine itself reported.
func serviceFailure(instance services.Instance, err error) string {
	if instance.Error != "" {
		return instance.Error
	}
	if err != nil {
		return err.Error()
	}
	return "the service did not start"
}

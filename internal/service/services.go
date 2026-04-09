package service

import (
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/websocket"
	"github.com/prasanth-33460/Project-Management-Platform/internal/config"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

// Services is the top-level container for all domain services.
type Services struct {
	Auth          *AuthService
	Project       *ProjectService
	Issue         *IssueService
	Workflow      *WorkflowEngine
	Sprint        *SprintService
	Collaboration *CollaborationService
	Notification  *NotificationService
	CustomField   *CustomFieldService
}

// NewServices wires all services against the provided repositories, WebSocket hub, and config.
func NewServices(repos *repository.Repositories, hub *websocket.Hub, cfg *config.Config) *Services {
	auth := NewAuthService(repos.User, cfg.JWTSecret)
	project := NewProjectService(repos.Project, repos.Workflow, repos.User)
	issue := NewIssueService(repos.Issue, repos.Project, repos.Workflow, repos.Notification, hub)
	workflow := NewWorkflowEngine(repos.Issue, repos.Workflow, repos.Notification, repos.Tx, hub)
	sprint := NewSprintService(repos.Sprint, repos.Issue, repos.Project, repos.Tx, hub)
	collab := NewCollaborationService(repos.Comment, repos.Issue, repos.User, repos.Notification, hub)
	notif := NewNotificationService(repos.Notification)
	customField := NewCustomFieldService(repos.CustomField, repos.Issue)

	return &Services{
		Auth:          auth,
		Project:       project,
		Issue:         issue,
		Workflow:      workflow,
		Sprint:        sprint,
		Collaboration: collab,
		Notification:  notif,
		CustomField:   customField,
	}
}

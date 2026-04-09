package handlers

import (
	"github.com/gofiber/fiber/v2"
	gows "github.com/gofiber/websocket/v2"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	wshandler "github.com/prasanth-33460/Project-Management-Platform/internal/api/websocket"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

// RegisterRoutes wires all API routes onto the given Fiber router group.
func RegisterRoutes(api fiber.Router, svcs *service.Services, repos *repository.Repositories, hub *wshandler.Hub) {
	// --- Auth (public) ---
	authH := NewAuthHandler(svcs.Auth)
	authGrp := api.Group("/auth")
	authGrp.Post("/register", authH.Register)
	authGrp.Post("/login", authH.Login)

	// Authenticated middleware
	authMW := middleware.Auth(svcs.Auth)

	// --- Projects ---
	projectH := NewProjectHandler(svcs.Project, svcs.Issue)
	sprintH := NewSprintHandler(svcs.Sprint)
	wfH := NewWorkflowHandler(repos.Workflow)

	projects := api.Group("/projects", authMW)
	projects.Get("", projectH.List)
	projects.Post("", projectH.Create)
	projects.Get("/:id", projectH.Get)
	projects.Patch("/:id", projectH.Update)
	projects.Delete("/:id", projectH.Delete)
	projects.Get("/:id/board", projectH.GetBoard)
	projects.Get("/:id/backlog", projectH.GetBacklog)
	projects.Post("/:id/issues", projectH.CreateIssue)
	projects.Get("/:id/sprints", sprintH.ListByProject)
	projects.Post("/:id/sprints", sprintH.Create)
	projects.Get("/:id/activity", projectH.GetActivity)
	projects.Get("/:id/workflow/statuses", wfH.ListStatuses)
	projects.Post("/:id/workflow/statuses", wfH.CreateStatus)
	projects.Post("/:id/workflow/transitions", wfH.CreateTransition)
	projects.Delete("/:id/workflow/transitions/:transitionId", wfH.DeleteTransition)

	// --- Issues ---
	issueH := NewIssueHandler(svcs.Issue, svcs.Workflow)
	commentH := NewCommentHandler(svcs.Collaboration)

	issues := api.Group("/issues", authMW)
	issues.Get("/:id", issueH.Get)
	issues.Patch("/:id", issueH.Update)
	issues.Delete("/:id", issueH.Delete)
	issues.Post("/:id/transitions", issueH.Transition)
	issues.Post("/:id/watch", issueH.Watch)
	issues.Delete("/:id/watch", issueH.Unwatch)
	issues.Get("/:id/comments", commentH.List)
	issues.Post("/:id/comments", commentH.Create)

	// --- Comments ---
	comments := api.Group("/comments", authMW)
	comments.Patch("/:id", commentH.Update)
	comments.Delete("/:id", commentH.Delete)

	// --- Sprints ---
	sprints := api.Group("/sprints", authMW)
	sprints.Get("/:id", sprintH.GetByID)
	sprints.Patch("/:id", sprintH.Update)
	sprints.Delete("/:id", sprintH.Delete)
	sprints.Post("/:id/start", sprintH.Start)
	sprints.Post("/:id/complete", sprintH.Complete)
	sprints.Post("/:id/move-issue", sprintH.MoveIssue)
	sprints.Post("/move-to-backlog", sprintH.MoveToBacklog)

	// --- Search ---
	searchH := NewSearchHandler(svcs.Issue)
	api.Get("/search", authMW, searchH.Search)

	// --- Notifications ---
	notifH := NewNotificationHandler(svcs.Notification)
	notifs := api.Group("/notifications", authMW)
	notifs.Get("", notifH.List)
	notifs.Post("/read-all", notifH.MarkAllRead)
	notifs.Post("/:id/read", notifH.MarkRead)

	// --- WebSocket  (ws://<host>/api/ws?user_id=...&project_id=...) ---
	wsH := wshandler.NewHandler(hub)
	api.Get("/ws", wshandler.Upgrade, gows.New(wsH.Handle))
}

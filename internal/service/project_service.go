package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

// ProjectService handles project lifecycle and board assembly.
type ProjectService struct {
	projects  repository.ProjectStore
	workflows repository.WorkflowStore
	users     repository.UserStore
}

func NewProjectService(
	projects repository.ProjectStore,
	workflows repository.WorkflowStore,
	users repository.UserStore,
) *ProjectService {
	return &ProjectService{projects: projects, workflows: workflows, users: users}
}

func (s *ProjectService) Create(ctx context.Context, req *models.CreateProjectRequest, creatorID uuid.UUID) (*models.Project, error) {
	req.Key = strings.ToUpper(req.Key)

	project, err := s.projects.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.projects.AddMember(ctx, project.ID, creatorID, "admin"); err != nil {
		return nil, err
	}

	// seed default workflow: To Do → In Progress → In Review → Done
	defaultStatuses := []models.CreateStatusRequest{
		{Name: "To Do", Color: "#6B7280", Position: 0, IsDefault: true, IsDone: false},
		{Name: "In Progress", Color: "#3B82F6", Position: 1, IsDefault: false, IsDone: false},
		{Name: "In Review", Color: "#F59E0B", Position: 2, IsDefault: false, IsDone: false},
		{Name: "Done", Color: "#10B981", Position: 3, IsDefault: false, IsDone: true},
	}

	statusIDs := make([]uuid.UUID, len(defaultStatuses))
	for i, sr := range defaultStatuses {
		st, err := s.workflows.CreateStatus(ctx, project.ID, &sr)
		if err != nil {
			return nil, err
		}
		statusIDs[i] = st.ID
	}

	// seed transitions, including backward paths for revert
	transitions := [][2]int{
		{0, 1}, // To Do       → In Progress
		{1, 2}, // In Progress → In Review
		{2, 3}, // In Review   → Done
		{2, 1}, // In Review   → In Progress (revert)
		{1, 0}, // In Progress → To Do       (revert)
	}
	for _, tr := range transitions {
		if _, err := s.workflows.CreateTransition(ctx, project.ID, &models.CreateTransitionRequest{
			FromStatusID: statusIDs[tr[0]],
			ToStatusID:   statusIDs[tr[1]],
		}); err != nil {
			return nil, err
		}
	}

	return project, nil
}

func (s *ProjectService) Get(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	p, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.ErrNotFound
	}
	return p, nil
}

func (s *ProjectService) List(ctx context.Context, userID uuid.UUID) ([]*models.Project, error) {
	return s.projects.List(ctx, userID)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, req *models.UpdateProjectRequest) (*models.Project, error) {
	p, err := s.projects.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.ErrNotFound
	}
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.projects.Delete(ctx, id)
}

func (s *ProjectService) AddMember(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	return s.projects.AddMember(ctx, projectID, userID, role)
}

func (s *ProjectService) IsMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	return s.projects.IsMember(ctx, projectID, userID)
}

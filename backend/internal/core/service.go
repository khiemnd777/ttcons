package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSecret string
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{repo: repo, jwtSecret: jwtSecret}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (Record, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprint(user["password_hash"])
	ok := false
	if strings.HasPrefix(hash, "demo:") {
		ok = strings.TrimPrefix(hash, "demo:") == req.Password
	} else {
		ok = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
	}
	if !ok {
		return nil, sql.ErrNoRows
	}
	roles, err := s.repo.UserRoles(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	token, err := s.signToken(fmt.Sprint(user["id"]), fmt.Sprint(user["email"]), roles)
	if err != nil {
		return nil, err
	}
	delete(user, "password_hash")
	user["roles"] = roles
	return Record{"token": token, "user": user}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (Record, error) {
	user, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	roles, _ := s.repo.UserRoles(ctx, userID)
	delete(user, "password_hash")
	user["roles"] = roles
	return user, nil
}

func (s *Service) signToken(userID, email string, roles []string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"roles":   roles,
		"exp":     time.Now().Add(12 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Service) ListProjects(ctx context.Context) ([]Record, error) {
	return s.repo.ListProjects(ctx)
}

func (s *Service) CreateProject(ctx context.Context, body Record) (Record, error) {
	if body["code"] == nil || body["code"] == "" {
		body["code"] = fmt.Sprintf("PRJ-%d", time.Now().Unix())
	}
	if body["status"] == nil {
		body["status"] = "draft"
	}
	return s.repo.Insert(ctx, "projects", projectColumns(), body)
}

func (s *Service) UpdateProject(ctx context.Context, id string, body Record) (Record, error) {
	return s.repo.Update(ctx, "projects", projectColumns(), id, body)
}

func (s *Service) ProjectDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetProjectDetail(ctx, id)
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	return s.repo.Exec(ctx, `UPDATE projects SET status='cancelled', updated_at=now() WHERE id=$1`, id)
}

func (s *Service) ListTemplates(ctx context.Context) ([]Record, error) {
	return s.repo.Query(ctx, `
		SELECT t.*, COUNT(n.id) AS node_count
		FROM pipeline_templates t
		LEFT JOIN pipeline_template_nodes n ON n.template_id=t.id
		GROUP BY t.id
		ORDER BY t.updated_at DESC, t.created_at DESC
	`)
}

func (s *Service) TemplateDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CreateTemplate(ctx context.Context, body Record, actorID string) (Record, error) {
	if body["status"] == nil {
		body["status"] = "draft"
	}
	return s.repo.CreateTemplateWithNodes(ctx, body, actorID)
}

func (s *Service) UpdateTemplate(ctx context.Context, id string, body Record) (Record, error) {
	return s.repo.Update(ctx, "pipeline_templates", templateColumns(), id, body)
}

func (s *Service) PublishTemplate(ctx context.Context, id, actorID, actorName string) (Record, error) {
	template, err := s.repo.QueryOne(ctx, `UPDATE pipeline_templates SET status='published', updated_at=now() WHERE id=$1 RETURNING *`, id)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "pipeline_template_published", "pipeline_template", id, fmt.Sprintf("Publish template %s", template["name"]), Record{})
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CloneTemplate(ctx context.Context, id, actorID string) (Record, error) {
	return s.repo.CloneTemplate(ctx, id, actorID)
}

func (s *Service) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	return s.repo.StartPipeline(ctx, projectID, templateID, actorID, actorName)
}

func (s *Service) PipelineRun(ctx context.Context, projectID string) (Record, error) {
	return s.repo.GetPipelineRun(ctx, projectID)
}

func (s *Service) SubmitRunStep(ctx context.Context, stepID, actorID, actorName string) (Record, error) {
	return s.repo.SubmitRunStep(ctx, stepID, actorID, actorName)
}

func (s *Service) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	return s.repo.AdvanceRunNode(ctx, nodeID, actorID, actorName)
}

func (s *Service) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	return s.repo.RequestNodeApproval(ctx, nodeID, actorID, actorName, reason)
}

func (s *Service) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	return s.repo.BlockRunNode(ctx, nodeID, reason, actorID, actorName)
}

func (s *Service) ApprovalAction(ctx context.Context, id, action, comment, actorID, actorName string) (Record, error) {
	return s.repo.ApprovalAction(ctx, id, action, comment, actorID, actorName)
}

func (s *Service) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	return s.repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, actorName)
}

func (s *Service) ProcurementAction(ctx context.Context, id, action, actorID, actorName string) (Record, error) {
	return s.repo.ProcurementAction(ctx, id, action, actorID, actorName)
}

func (s *Service) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockMovement(ctx, req, actorID, actorName)
}

func (s *Service) AcceptanceAction(ctx context.Context, id, result, actorID, actorName string) (Record, error) {
	return s.repo.AcceptanceAction(ctx, id, result, actorID, actorName)
}

func (s *Service) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	return s.repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, actorName)
}

func (s *Service) HandoverCheck(ctx context.Context, id string) (Record, error) {
	return s.repo.HandoverCheck(ctx, id)
}

func (s *Service) CompleteHandover(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return s.repo.CompleteHandover(ctx, id, actorID, actorName)
}

func (s *Service) Dashboard(ctx context.Context) (Record, error) {
	return s.repo.DashboardReport(ctx)
}

func (s *Service) Report(ctx context.Context, kind string) ([]Record, error) {
	return s.repo.Report(ctx, kind)
}

func (s *Service) AuditLogs(ctx context.Context, projectID string) ([]Record, error) {
	if projectID != "" {
		return s.repo.Query(ctx, `
			SELECT * FROM audit_logs
			WHERE project_id=$1
			ORDER BY created_at DESC
			LIMIT 200
		`, projectID)
	}
	return s.repo.Query(ctx, `SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 300`)
}

package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo                  *Repository
	jwtSecret             string
	streamHub             *ActivityStreamHub
	uploadDir             string
	maxUploadBytes        int64
	accessTokenTTL        time.Duration
	refreshTokenTTL       time.Duration
	refreshCookieSecure   bool
	refreshCookieSameSite string
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:                  repo,
		jwtSecret:             jwtSecret,
		streamHub:             NewActivityStreamHub(),
		uploadDir:             defaultUploadDir,
		maxUploadBytes:        defaultMaxUploadBytes,
		accessTokenTTL:        defaultAccessTokenTTL,
		refreshTokenTTL:       defaultRefreshTokenTTL,
		refreshCookieSameSite: defaultRefreshCookieSameSite,
	}
}

const (
	defaultUploadDir             = "uploads"
	defaultMaxUploadBytes        = int64(10 * 1024 * 1024)
	defaultAccessTokenTTL        = 15 * time.Minute
	defaultRefreshTokenTTL       = 7 * 24 * time.Hour
	defaultRefreshCookieSameSite = "Lax"
	activityMessageEditWindow    = 15 * time.Minute
)

type evidenceUploadMimeSpec struct {
	Extension    string
	EvidenceType string
}

var allowedEvidenceUploadMIMEs = map[string]evidenceUploadMimeSpec{
	"image/jpeg":      {Extension: ".jpg", EvidenceType: "photo"},
	"image/png":       {Extension: ".png", EvidenceType: "photo"},
	"image/webp":      {Extension: ".webp", EvidenceType: "photo"},
	"application/pdf": {Extension: ".pdf", EvidenceType: "document"},
}

func (s *Service) ConfigureUploads(uploadDir string, maxUploadBytes int64) {
	uploadDir = strings.TrimSpace(uploadDir)
	if uploadDir == "" {
		uploadDir = defaultUploadDir
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	s.uploadDir = uploadDir
	s.maxUploadBytes = maxUploadBytes
}

func (s *Service) ConfigureAuth(accessTokenMinutes int64, refreshTokenHours int64, refreshCookieSecure bool, refreshCookieSameSite string) {
	if accessTokenMinutes <= 0 {
		accessTokenMinutes = int64(defaultAccessTokenTTL / time.Minute)
	}
	if refreshTokenHours <= 0 {
		refreshTokenHours = int64(defaultRefreshTokenTTL / time.Hour)
	}
	refreshCookieSameSite = strings.TrimSpace(refreshCookieSameSite)
	if refreshCookieSameSite == "" {
		refreshCookieSameSite = defaultRefreshCookieSameSite
	}
	s.accessTokenTTL = time.Duration(accessTokenMinutes) * time.Minute
	s.refreshTokenTTL = time.Duration(refreshTokenHours) * time.Hour
	s.refreshCookieSecure = refreshCookieSecure
	s.refreshCookieSameSite = refreshCookieSameSite
}

func (s *Service) Login(ctx context.Context, req LoginRequest, userAgent string, ipAddress string) (Record, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("Email và mật khẩu là bắt buộc")
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
	permissions, err := s.repo.UserPermissions(ctx, fmt.Sprint(user["id"]))
	if err != nil {
		return nil, err
	}
	refreshToken, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	accessTokenID := uuid.NewString()
	refreshExpiresAt := time.Now().Add(s.refreshTokenTTL)
	session, err := s.repo.CreateAuthSession(ctx, fmt.Sprint(user["id"]), hashSessionToken(refreshToken), accessTokenID, userAgent, ipAddress, refreshExpiresAt)
	if err != nil {
		return nil, err
	}
	token, accessExpiresAt, err := s.signToken(fmt.Sprint(user["id"]), fmt.Sprint(user["email"]), roles, fmt.Sprint(session["id"]), accessTokenID)
	if err != nil {
		return nil, err
	}
	delete(user, "password_hash")
	user["roles"] = roles
	user["permissions"] = permissions
	return Record{
		"token":              token,
		"access_expires_at":  accessExpiresAt.Format(time.RFC3339),
		"refresh_token":      refreshToken,
		"refresh_expires_at": refreshExpiresAt.Format(time.RFC3339),
		"user":               user,
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (Record, error) {
	user, err := s.repo.GetByID(ctx, "users", userID)
	if err != nil {
		return nil, err
	}
	roles, _ := s.repo.UserRoles(ctx, userID)
	permissions, _ := s.repo.UserPermissions(ctx, userID)
	delete(user, "password_hash")
	user["roles"] = roles
	user["permissions"] = permissions
	return user, nil
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string, userAgent string, ipAddress string) (Record, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, sql.ErrNoRows
	}
	nextRefreshToken, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	accessTokenID := uuid.NewString()
	refreshExpiresAt := time.Now().Add(s.refreshTokenTTL)
	session, err := s.repo.RotateAuthSession(ctx, hashSessionToken(refreshToken), hashSessionToken(nextRefreshToken), accessTokenID, userAgent, ipAddress, refreshExpiresAt)
	if err != nil {
		return nil, err
	}
	userID := fmt.Sprint(session["user_id"])
	user, err := s.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, _ := user["roles"].([]string)
	token, accessExpiresAt, err := s.signToken(userID, fmt.Sprint(user["email"]), roles, fmt.Sprint(session["id"]), accessTokenID)
	if err != nil {
		return nil, err
	}
	return Record{
		"token":              token,
		"access_expires_at":  accessExpiresAt.Format(time.RFC3339),
		"refresh_token":      nextRefreshToken,
		"refresh_expires_at": refreshExpiresAt.Format(time.RFC3339),
		"user":               user,
	}, nil
}

func (s *Service) LogoutByRefreshToken(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	return s.repo.RevokeAuthSessionByRefreshHash(ctx, hashSessionToken(refreshToken))
}

func (s *Service) signToken(userID, email string, roles []string, sessionID string, accessTokenID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)
	claims := jwt.MapClaims{
		"user_id":    userID,
		"email":      email,
		"roles":      roles,
		"session_id": sessionID,
		"jti":        accessTokenID,
		"exp":        expiresAt.Unix(),
		"iat":        time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	return signed, expiresAt, err
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashSessionToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Service) ListProjects(ctx context.Context, actorID string) ([]Record, error) {
	return s.repo.ListProjects(ctx, actorID)
}

func (s *Service) ListPartners(ctx context.Context, filter PartnerFilter) ([]Record, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Type != "" && !validPartnerType(filter.Type) {
		return nil, validationError("Loại đối tác không hợp lệ", Record{"partner_type": filter.Type})
	}
	if filter.Status != "" && !statusIn(filter.Status, "active", "inactive", "all") {
		return nil, validationError("Trạng thái đối tác không hợp lệ", Record{"status": filter.Status})
	}
	return s.repo.ListPartners(ctx, filter)
}

func (s *Service) CreatePartner(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizePartnerBody(body, true)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Insert(ctx, "partners", partnerColumns(), normalized)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "partner_created", "partner", cleanID(row["id"]), fmt.Sprintf("Create partner %s", row["name"]), Record{"partner_code": row["partner_code"], "partner_types": row["partner_types"]})
	return row, nil
}

func (s *Service) UpdatePartner(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizePartnerBody(body, false)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Update(ctx, "partners", partnerColumns(), id, normalized)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "partner_updated", "partner", cleanID(row["id"]), fmt.Sprintf("Update partner %s", row["name"]), Record{"partner_code": row["partner_code"], "partner_types": row["partner_types"]})
	return row, nil
}

func (s *Service) CreateProject(ctx context.Context, body Record) (Record, error) {
	if body["code"] == nil || body["code"] == "" {
		body["code"] = fmt.Sprintf("PRJ-%d", time.Now().Unix())
	}
	if body["status"] == nil {
		body["status"] = "draft"
	}
	if err := s.applyPartnerSnapshot(ctx, body, "partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	return s.repo.Insert(ctx, "projects", projectColumns(), body)
}

func (s *Service) UpdateProject(ctx context.Context, id string, body Record) (Record, error) {
	if err := s.applyPartnerSnapshot(ctx, body, "partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, "projects", projectColumns(), id, body)
}

func (s *Service) ProjectDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetProjectDetail(ctx, id)
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	return s.repo.Exec(ctx, `UPDATE projects SET status='cancelled', updated_at=now() WHERE id=$1`, id)
}

func (s *Service) ProjectMembers(ctx context.Context, projectID, actorID string, includeInactive bool) (Record, error) {
	canManage, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.member.manage"})
	if err != nil {
		return nil, err
	}
	includeInactive = includeInactive && canManage
	items, err := s.repo.ListProjectMembers(ctx, projectID, includeInactive)
	if err != nil {
		return nil, err
	}
	options := Record{"staff": []Record{}, "access_roles": []Record{}, "role_labels": []string{}}
	if canManage {
		options, err = s.repo.ProjectMemberOptions(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}
	return Record{
		"items":   items,
		"summary": projectMemberSummary(items),
		"options": options,
		"capabilities": Record{
			"can_manage":        canManage,
			"can_view_inactive": canManage,
		},
	}, nil
}

func (s *Service) SaveProjectMember(ctx context.Context, projectID, memberID string, req ProjectMemberMutationRequest, actorID, actorName string) (Record, error) {
	normalized, err := normalizeProjectMemberMutationRequest(projectID, memberID, req)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveProjectMember(ctx, projectID, memberID, normalized, actorID, actorName)
}

func (s *Service) DeactivateProjectMember(ctx context.Context, projectID, memberID, actorID, actorName string) (Record, error) {
	if _, err := uuid.Parse(strings.TrimSpace(projectID)); err != nil {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	if _, err := uuid.Parse(strings.TrimSpace(memberID)); err != nil {
		return nil, validationError("member_id không hợp lệ", Record{"member_id": memberID})
	}
	return s.repo.DeactivateProjectMember(ctx, strings.TrimSpace(projectID), strings.TrimSpace(memberID), actorID, actorName)
}

func normalizeProjectMemberMutationRequest(projectID, memberID string, req ProjectMemberMutationRequest) (ProjectMemberMutationRequest, error) {
	projectID = strings.TrimSpace(projectID)
	memberID = strings.TrimSpace(memberID)
	req.StaffID = strings.TrimSpace(req.StaffID)
	req.Notes = strings.TrimSpace(req.Notes)
	if _, err := uuid.Parse(projectID); err != nil {
		return req, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	if memberID != "" {
		if _, err := uuid.Parse(memberID); err != nil {
			return req, validationError("member_id không hợp lệ", Record{"member_id": memberID})
		}
	}
	if _, err := uuid.Parse(req.StaffID); err != nil {
		return req, validationError("Phải chọn nhân sự hợp lệ", Record{"staff_id": req.StaffID})
	}
	req.RoleIDs = normalizedRoleList(req.RoleIDs)
	if len(req.RoleIDs) == 0 {
		return req, validationError("Chọn ít nhất một vai trò vận hành", Record{"field": "role_ids"})
	}
	if len(req.RoleIDs) > 12 {
		return req, validationError("Một thành viên không thể có quá 12 vai trò vận hành")
	}
	for _, role := range req.RoleIDs {
		if len([]rune(role)) > 100 {
			return req, validationError("Tên vai trò vận hành không được vượt quá 100 ký tự", Record{"role": role})
		}
	}
	if len(req.Notes) > 1000 {
		return req, validationError("Ghi chú phân công không được vượt quá 1000 ký tự")
	}
	seen := map[string]bool{}
	projectRoleIDs := make([]string, 0, len(req.ProjectRoleIDs))
	for _, roleID := range req.ProjectRoleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" || seen[roleID] {
			continue
		}
		if _, err := uuid.Parse(roleID); err != nil {
			return req, validationError("Role phân quyền không hợp lệ", Record{"role_id": roleID})
		}
		seen[roleID] = true
		projectRoleIDs = append(projectRoleIDs, roleID)
	}
	req.ProjectRoleIDs = projectRoleIDs
	return req, nil
}

func projectMemberSummary(items []Record) Record {
	departments := map[string]bool{}
	roles := map[string]bool{}
	active := 0
	inactive := 0
	for _, item := range items {
		if item["active"] == true {
			active++
			department := strings.TrimSpace(fmt.Sprint(item["department"]))
			if department != "" && department != "<nil>" {
				departments[department] = true
			}
			for _, role := range stringSlice(item["role_ids"]) {
				roles[role] = true
			}
		} else {
			inactive++
		}
	}
	return Record{
		"active":      active,
		"inactive":    inactive,
		"departments": len(departments),
		"roles":       len(roles),
	}
}

func (s *Service) CreateMaterial(ctx context.Context, body Record) (Record, error) {
	if err := s.applyPartnerSnapshot(ctx, body, "default_supplier_partner_id", "default_supplier", "", []string{"supplier", "service_provider", "other"}); err != nil {
		return nil, err
	}
	return s.repo.Insert(ctx, "materials", materialColumns(), body)
}

func (s *Service) CreateWarehouse(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	row, err := s.repo.CreateWarehouse(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "warehouse_created", "warehouse", cleanID(row["id"]), fmt.Sprintf("Tạo kho %s", row["name"]), Record{"type": row["type"]})
	return row, nil
}

func (s *Service) ProjectDailyReports(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectDailyReports(ctx, cleanID(projectID))
}

func (s *Service) CreateDailyReport(ctx context.Context, projectID string, req ConstructionDailyReportCreateRequest, actorID, actorName string) (Record, error) {
	body, err := s.normalizeConstructionDailyReport(ctx, projectID, req, actorID, nil)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.CreateDailyReport(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "daily_report_created", "daily_report", cleanID(row["id"]), fmt.Sprintf("Tạo nhật ký ngày %s", row["report_date"]), Record{
		"progress_percent": row["progress_percent"],
		"work_item_id":     body["work_item_id"],
		"evidence_count":   len(normalizeConstructionReportEvidenceIDs(req.EvidenceIDs, req.PrimaryEvidenceID)),
	})
	return row, nil
}

func (s *Service) normalizeConstructionDailyReport(ctx context.Context, projectID string, req ConstructionDailyReportCreateRequest, actorID string, existingEvidenceIDs map[string]bool) (Record, error) {
	projectID = cleanID(projectID)
	reportDate := strings.TrimSpace(req.ReportDate)
	if _, err := time.Parse("2006-01-02", reportDate); err != nil {
		return nil, validationError("Ngày báo cáo không hợp lệ", Record{"report_date": req.ReportDate})
	}
	weather := strings.TrimSpace(req.Weather)
	if weather == "" {
		return nil, validationError("Thời tiết là bắt buộc", Record{"weather": "required"})
	}
	if req.ProgressPercent < 0 || req.ProgressPercent > 100 {
		return nil, validationError("Tiến độ báo cáo phải từ 0 đến 100", Record{"progress_percent": req.ProgressPercent})
	}

	doneItems, doneTitles, err := normalizeConstructionReportWorkItems(req.DoneItems, "done_items", "done", 250)
	if err != nil {
		return nil, err
	}
	if len(doneItems) == 0 {
		return nil, validationError("Báo cáo cần ít nhất một hạng mục đã làm", Record{"done_items": "required"})
	}
	currentItems, currentTitles, err := normalizeConstructionReportWorkItems(req.CurrentItems, "current_items", "partial", 250)
	if err != nil {
		return nil, err
	}
	nextItems, nextTitles, err := normalizeConstructionReportNextItems(req.NextItems)
	if err != nil {
		return nil, err
	}
	blockers, err := normalizeConstructionReportBlockers(req.Blockers)
	if err != nil {
		return nil, err
	}
	crewItems, manpowerSummary, err := normalizeConstructionReportCrew(req.CrewItems)
	if err != nil {
		return nil, err
	}
	if manpowerSummary == "" {
		manpowerSummary, err = normalizeConstructionLegacySummary(req.ManpowerSummary, "manpower_summary")
		if err != nil {
			return nil, err
		}
	}
	materialItems, materialSummary, err := normalizeConstructionReportMaterials(req.MaterialItems)
	if err != nil {
		return nil, err
	}
	if materialSummary == "" {
		materialSummary, err = normalizeConstructionLegacySummary(req.MaterialSummary, "material_usage_summary")
		if err != nil {
			return nil, err
		}
	}

	workItemID := cleanID(req.WorkItemID)
	if workItemID != "" {
		if !validActivityUUID(workItemID) {
			return nil, validationError("Công việc liên quan không hợp lệ", Record{"work_item_id": req.WorkItemID})
		}
		item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, validationError("Không tìm thấy công việc liên quan", Record{"work_item_id": workItemID})
		}
		if err != nil {
			return nil, err
		}
		if cleanID(item["project_id"]) != projectID || fmt.Sprint(item["status"]) == "cancelled" {
			return nil, validationError("Công việc liên quan không thuộc công trình hoặc đã bị hủy", Record{"work_item_id": workItemID})
		}
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền sử dụng công việc liên quan này")
		}
	}

	evidenceIDs := normalizeConstructionReportEvidenceIDs(req.EvidenceIDs, req.PrimaryEvidenceID)
	if len(evidenceIDs) > 50 {
		return nil, validationError("Báo cáo chỉ được liên kết tối đa 50 minh chứng", Record{"evidence_ids": len(evidenceIDs), "max": 50})
	}
	if len(evidenceIDs) > 0 && workItemID == "" {
		return nil, validationError("Chọn công việc liên quan trước khi liên kết minh chứng", Record{"work_item_id": "required"})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	for _, evidenceID := range evidenceIDs {
		if !validActivityUUID(evidenceID) {
			return nil, validationError("Minh chứng không hợp lệ", Record{"evidence_id": evidenceID})
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, validationError("Không tìm thấy minh chứng", Record{"evidence_id": evidenceID})
		}
		if err != nil {
			return nil, err
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng không thuộc công việc liên quan của báo cáo", Record{"evidence_id": evidenceID, "work_item_id": workItemID})
		}
		if fmt.Sprint(evidence["status"]) == "rejected" && !existingEvidenceIDs[evidenceID] {
			return nil, validationError("Không thể liên kết minh chứng đã bị từ chối", Record{"evidence_id": evidenceID})
		}
		if profileIsClient(profile) && !profile.IsManager && fmt.Sprint(evidence["visibility"]) != "client" {
			return nil, forbiddenError("Bạn không có quyền liên kết minh chứng này")
		}
	}
	primaryEvidenceID := cleanID(req.PrimaryEvidenceID)
	if primaryEvidenceID == "" && len(evidenceIDs) > 0 {
		primaryEvidenceID = evidenceIDs[0]
	}

	completedItems := append(doneItems, currentItems...)
	structured := Record{
		"completed_items":      completedItems,
		"next_plan_items":      nextItems,
		"blockers":             blockers,
		"crew_items":           crewItems,
		"material_usage_items": materialItems,
	}
	if primaryEvidenceID != "" {
		structured["site_photo_evidence_id"] = primaryEvidenceID
	}
	body := Record{
		"project_id":             projectID,
		"report_date":            reportDate,
		"reported_by":            actorID,
		"weather":                weather,
		"work_summary":           constructionReportWorkSummary(doneTitles, currentTitles, nextTitles),
		"manpower_summary":       manpowerSummary,
		"material_usage_summary": materialSummary,
		"issues_summary":         strings.TrimSpace(req.FieldNotes),
		"progress_percent":       req.ProgressPercent,
		"photos":                 evidenceIDs,
		"work_item_id":           workItemID,
		"structured_json":        structured,
		"primary_evidence_id":    primaryEvidenceID,
	}
	return body, nil
}

func (s *Service) UpdateDailyReport(ctx context.Context, reportID string, req ConstructionDailyReportCreateRequest, actorID, actorName string) (Record, error) {
	reportID = cleanID(reportID)
	if !validActivityUUID(reportID) {
		return nil, validationError("Báo cáo thi công không hợp lệ", Record{"report_id": reportID})
	}
	current, err := s.repo.GetByID(ctx, "operation_daily_reports", reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo thi công", Record{"report_id": reportID})
	}
	if err != nil {
		return nil, err
	}
	if cleanID(current["workflow_form_instance_id"]) != "" {
		return nil, validationError("Báo cáo từ quy trình phải được cập nhật tại công việc liên quan", Record{"workflow_form_instance_id": current["workflow_form_instance_id"]})
	}
	existingEvidenceIDs := map[string]bool{}
	for _, evidenceID := range constructionReportStringList(current["photos"]) {
		existingEvidenceIDs[evidenceID] = true
	}
	body, err := s.normalizeConstructionDailyReport(ctx, cleanID(current["project_id"]), req, actorID, existingEvidenceIDs)
	if err != nil {
		return nil, err
	}
	delete(body, "project_id")
	delete(body, "reported_by")
	row, err := s.repo.UpdateDailyReport(ctx, reportID, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(current["project_id"]), actorID, actorName, "daily_report_updated", "daily_report", reportID, fmt.Sprintf("Cập nhật nhật ký ngày %s", row["report_date"]), Record{
		"progress_percent": row["progress_percent"],
		"work_item_id":     row["work_item_id"],
		"evidence_count":   len(constructionReportStringList(row["photos"])),
	})
	return row, nil
}

func (s *Service) AddDailyReportEvidenceUpload(ctx context.Context, reportID, requestedWorkItemID string, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	reportID = cleanID(reportID)
	if !validActivityUUID(reportID) {
		return nil, validationError("Báo cáo thi công không hợp lệ", Record{"report_id": reportID})
	}
	report, err := s.repo.GetByID(ctx, "operation_daily_reports", reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy báo cáo thi công", Record{"report_id": reportID})
	}
	if err != nil {
		return nil, err
	}
	if cleanID(report["workflow_form_instance_id"]) != "" {
		return nil, validationError("Báo cáo từ quy trình phải bổ sung minh chứng tại công việc liên quan", Record{"workflow_form_instance_id": report["workflow_form_instance_id"]})
	}

	workItemID := cleanID(report["work_item_id"])
	requestedWorkItemID = cleanID(requestedWorkItemID)
	if workItemID != "" && requestedWorkItemID != "" && requestedWorkItemID != workItemID {
		return nil, validationError("Công việc liên quan không khớp với báo cáo", Record{"work_item_id": requestedWorkItemID, "expected_work_item_id": workItemID})
	}
	if workItemID == "" {
		workItemID = requestedWorkItemID
	}
	if workItemID == "" || !validActivityUUID(workItemID) {
		return nil, validationError("Chọn công việc liên quan trước khi tải minh chứng", Record{"work_item_id": "required"})
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Không tìm thấy công việc liên quan", Record{"work_item_id": workItemID})
	}
	if err != nil {
		return nil, err
	}
	projectID := cleanID(report["project_id"])
	if cleanID(item["project_id"]) != projectID || fmt.Sprint(item["status"]) == "cancelled" {
		return nil, validationError("Công việc liên quan không thuộc công trình hoặc đã bị hủy", Record{"work_item_id": workItemID})
	}
	photos := constructionReportStringList(report["photos"])
	if len(photos) >= 50 {
		return nil, validationError("Báo cáo chỉ được liên kết tối đa 50 minh chứng", Record{"evidence_ids": len(photos), "max": 50})
	}
	if strings.TrimSpace(req.Visibility) == "" {
		req.Visibility = "project"
	}
	evidence, err := s.AddWorkItemEvidenceUpload(ctx, workItemID, req, fileHeader, actorID, actorName)
	if err != nil {
		return nil, err
	}
	evidenceID := cleanID(evidence["id"])
	photos = normalizeConstructionReportEvidenceIDs(append(photos, evidenceID), "")
	primaryEvidenceID := cleanID(report["primary_evidence_id"])
	if primaryEvidenceID == "" {
		primaryEvidenceID = evidenceID
	}
	structured := constructionReportRecord(report["structured_json"])
	if cleanID(structured["site_photo_evidence_id"]) == "" {
		structured["site_photo_evidence_id"] = primaryEvidenceID
	}
	updated, err := s.repo.UpdateDailyReport(ctx, reportID, Record{
		"photos":              photos,
		"work_item_id":        workItemID,
		"structured_json":     structured,
		"primary_evidence_id": primaryEvidenceID,
	})
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, projectID, actorID, actorName, "daily_report_evidence_added", "daily_report", reportID, fmt.Sprintf("Thêm minh chứng %s vào nhật ký ngày %s", evidence["name"], report["report_date"]), Record{
		"work_item_id": workItemID,
		"evidence_id":  evidenceID,
	})
	return updated, nil
}

func normalizeConstructionLegacySummary(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 4000 {
		return "", validationError("Nội dung tóm tắt tối đa 4000 ký tự", Record{field: "too_long"})
	}
	return value, nil
}

func constructionReportRecord(value any) Record {
	switch typed := value.(type) {
	case Record:
		return typed
	case map[string]any:
		return Record(typed)
	case string:
		row := Record{}
		if json.Unmarshal([]byte(typed), &row) == nil {
			return row
		}
	}
	return Record{}
}

func constructionReportStringList(value any) []string {
	values := []string{}
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
	case string:
		_ = json.Unmarshal([]byte(typed), &values)
	}
	return normalizeConstructionReportEvidenceIDs(values, "")
}

func normalizeConstructionReportWorkItems(items []ConstructionDailyReportWorkItemRequest, field, result string, maxItems int) ([]Record, []string, error) {
	rows := make([]Record, 0, len(items))
	titles := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		if len([]rune(title)) > 180 {
			return nil, nil, validationError("Tên hạng mục tối đa 180 ký tự", Record{field: title})
		}
		rows = append(rows, Record{"title": title, "result": result})
		titles = append(titles, title)
	}
	if len(rows) > maxItems {
		return nil, nil, validationError("Báo cáo có quá nhiều hạng mục", Record{field: len(rows), "max": maxItems})
	}
	return rows, titles, nil
}

func normalizeConstructionReportNextItems(items []ConstructionDailyReportWorkItemRequest) ([]Record, []string, error) {
	rows, titles, err := normalizeConstructionReportWorkItems(items, "next_items", "", 120)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		delete(row, "result")
	}
	return rows, titles, nil
}

func normalizeConstructionReportBlockers(items []ConstructionDailyReportBlockerRequest) ([]Record, error) {
	rows := make([]Record, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if len([]rune(content)) > 1000 {
			return nil, validationError("Nội dung blocker tối đa 1000 ký tự", Record{"blockers": content})
		}
		rows = append(rows, Record{"content": content})
	}
	if len(rows) > 120 {
		return nil, validationError("Báo cáo có quá nhiều blocker", Record{"blockers": len(rows), "max": 120})
	}
	return rows, nil
}

func normalizeConstructionReportCrew(items []ConstructionDailyReportCrewRequest) ([]Record, string, error) {
	rows := make([]Record, 0, len(items))
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.CrewName)
		if name == "" && item.WorkerCount == 0 && item.WorkHours == 0 && strings.TrimSpace(item.Note) == "" {
			continue
		}
		if name == "" || item.WorkerCount < 1 || item.WorkHours < 0 {
			return nil, "", validationError("Nhân công cần tên tổ đội, số người lớn hơn 0 và giờ công không âm", Record{"crew_name": name, "worker_count": item.WorkerCount, "work_hours": item.WorkHours})
		}
		row := Record{"crew_name": name, "worker_count": item.WorkerCount, "work_hours": item.WorkHours, "note": strings.TrimSpace(item.Note)}
		rows = append(rows, row)
		summary := fmt.Sprintf("%s · %d người", name, item.WorkerCount)
		if item.WorkHours > 0 {
			summary += fmt.Sprintf(" · %s giờ", strconv.FormatFloat(item.WorkHours, 'f', -1, 64))
		}
		summaries = append(summaries, summary)
	}
	if len(rows) > 120 {
		return nil, "", validationError("Báo cáo có quá nhiều tổ đội", Record{"crew_items": len(rows), "max": 120})
	}
	return rows, strings.Join(summaries, "; "), nil
}

func normalizeConstructionReportMaterials(items []ConstructionDailyReportMaterialRequest) ([]Record, string, error) {
	rows := make([]Record, 0, len(items))
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.MaterialName)
		unit := strings.TrimSpace(item.Unit)
		if name == "" && item.Quantity == 0 && unit == "" && strings.TrimSpace(item.Note) == "" {
			continue
		}
		if name == "" || item.Quantity < 0 || unit == "" {
			return nil, "", validationError("Vật tư cần tên, số lượng không âm và đơn vị tính", Record{"material_name": name, "quantity": item.Quantity, "unit": unit})
		}
		row := Record{"material_name": name, "quantity": item.Quantity, "unit": unit, "source": "other", "note": strings.TrimSpace(item.Note)}
		rows = append(rows, row)
		summaries = append(summaries, fmt.Sprintf("%s · %s %s", name, strconv.FormatFloat(item.Quantity, 'f', -1, 64), unit))
	}
	if len(rows) > 200 {
		return nil, "", validationError("Báo cáo có quá nhiều dòng vật tư", Record{"material_items": len(rows), "max": 200})
	}
	return rows, strings.Join(summaries, "; "), nil
}

func normalizeConstructionReportEvidenceIDs(ids []string, primaryID string) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(ids)+1)
	add := func(value string) {
		value = cleanID(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		values = append(values, value)
	}
	add(primaryID)
	for _, id := range ids {
		add(id)
	}
	return values
}

func normalizeConstructionCheckCriteria(items []ConstructionCheckCriterion) ([]ConstructionCheckCriterion, error) {
	criteria := make([]ConstructionCheckCriterion, 0, len(items))
	for _, item := range items {
		item.Title = strings.TrimSpace(item.Title)
		item.Criteria = strings.TrimSpace(item.Criteria)
		if item.Title == "" && item.Criteria == "" {
			continue
		}
		if item.Title == "" {
			item.Title = item.Criteria
		}
		if item.Criteria == "" {
			item.Criteria = item.Title
		}
		criteria = append(criteria, item)
	}
	if len(criteria) > 30 {
		return nil, validationError("Mỗi kiểm tra có tối đa 30 tiêu chí")
	}
	return criteria, nil
}

func normalizeConstructionCheckEvidenceIDs(ids []string, fallbackID string, required bool) ([]string, error) {
	values := normalizeConstructionReportEvidenceIDs(ids, fallbackID)
	if len(values) > 10 {
		return nil, validationError("Mỗi nhóm minh chứng có tối đa 10 ảnh hoặc tệp")
	}
	for _, id := range values {
		if !validActivityUUID(id) {
			return nil, validationError("Minh chứng kiểm tra không hợp lệ")
		}
	}
	if required && len(values) == 0 {
		return nil, validationError("Kết quả kiểm tra cần ít nhất một minh chứng")
	}
	return values, nil
}

func constructionReportWorkSummary(done, current, next []string) string {
	return strings.Join([]string{
		"Đã làm:\n" + strings.Join(done, "\n"),
		"Đang làm:\n" + strings.Join(current, "\n"),
		"Sẽ làm:\n" + strings.Join(next, "\n"),
	}, "\n\n")
}

func (s *Service) ProjectConstructionWorkspace(ctx context.Context, projectID string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	return s.repo.ProjectConstructionWorkspace(ctx, projectID)
}

func (s *Service) CreateConstructionCheckTemplate(ctx context.Context, projectID string, req ConstructionCheckTemplateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	name := strings.TrimSpace(req.Name)
	if !validActivityUUID(projectID) || name == "" {
		return nil, validationError("Mẫu kiểm tra cần tên")
	}
	criteria := make([]ConstructionCheckCriterion, 0, len(req.Criteria))
	for _, item := range req.Criteria {
		item.Title = strings.TrimSpace(item.Title)
		item.Criteria = strings.TrimSpace(item.Criteria)
		if item.Title == "" && item.Criteria == "" {
			continue
		}
		if item.Title == "" {
			item.Title = item.Criteria
		}
		criteria = append(criteria, item)
	}
	if len(criteria) == 0 || len(criteria) > 30 {
		return nil, validationError("Mẫu kiểm tra cần từ 1 đến 30 tiêu chí")
	}
	return s.repo.CreateConstructionCheckTemplate(ctx, projectID, name, strings.TrimSpace(req.Description), criteria, actorID, actorName)
}

func (s *Service) CreateConstructionCheck(ctx context.Context, projectID string, req ConstructionCheckRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	sourceDailyReportID := cleanID(req.SourceDailyReportID)
	templateID := cleanID(req.TemplateID)
	title := strings.TrimSpace(req.Title)
	if !validActivityUUID(projectID) || !validActivityUUID(workItemID) || title == "" {
		return nil, validationError("Kiểm tra cần công việc liên quan và nội dung ngắn gọn")
	}
	if templateID != "" && !validActivityUUID(templateID) {
		return nil, validationError("Mẫu kiểm tra không hợp lệ")
	}
	if sourceDailyReportID != "" && !validActivityUUID(sourceDailyReportID) {
		return nil, validationError("Báo cáo nguồn không hợp lệ")
	}
	scheduledAt, err := constructionOptionalTime(req.ScheduledAt, "Thời điểm kiểm tra")
	if err != nil {
		return nil, err
	}
	criteria, err := normalizeConstructionCheckCriteria(req.Criteria)
	if err != nil {
		return nil, err
	}
	contextEvidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.ContextEvidenceIDs, "", false)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateConstructionCheck(ctx, projectID, workItemID, sourceDailyReportID, templateID, title, strings.TrimSpace(req.Location), scheduledAt, criteria, req.Criteria != nil, contextEvidenceIDs, actorID, actorName)
}

func (s *Service) UpdateConstructionCheck(ctx context.Context, checkID string, req ConstructionCheckUpdateRequest, actorID, actorName string) (Record, error) {
	checkID = cleanID(checkID)
	workItemID := cleanID(req.WorkItemID)
	templateID := cleanID(req.TemplateID)
	title := strings.TrimSpace(req.Title)
	if !validActivityUUID(checkID) || !validActivityUUID(workItemID) || title == "" {
		return nil, validationError("Kiểm tra cần công việc liên quan và nội dung ngắn gọn")
	}
	if templateID != "" && !validActivityUUID(templateID) {
		return nil, validationError("Mẫu kiểm tra không hợp lệ")
	}
	scheduledAt, err := constructionOptionalTime(req.ScheduledAt, "Thời điểm kiểm tra")
	if err != nil {
		return nil, err
	}
	criteria, err := normalizeConstructionCheckCriteria(req.Criteria)
	if err != nil {
		return nil, err
	}
	contextEvidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.ContextEvidenceIDs, "", false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateConstructionCheck(ctx, checkID, workItemID, templateID, title, strings.TrimSpace(req.Location), scheduledAt, criteria, req.Criteria != nil, contextEvidenceIDs, req.ContextEvidenceIDs != nil, actorID, actorName)
}

func (s *Service) CompleteConstructionCheck(ctx context.Context, checkID string, req ConstructionCheckCompleteRequest, actorID, actorName string) (Record, error) {
	checkID = cleanID(checkID)
	result := strings.TrimSpace(req.Result)
	if !validActivityUUID(checkID) || !statusIn(result, "pass", "fail") {
		return nil, validationError("Kết quả kiểm tra cần kết luận Đạt hoặc Không đạt")
	}
	evidenceIDs, err := normalizeConstructionCheckEvidenceIDs(req.EvidenceIDs, req.EvidenceID, true)
	if err != nil {
		return nil, err
	}
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	impactScope := strings.TrimSpace(req.ImpactScope)
	if impactScope == "" {
		impactScope = "none"
	}
	dueAt := ""
	if result == "fail" {
		if !validActivityUUID(cleanID(req.AssignedUserID)) || !statusIn(priority, "normal", "urgent") || !statusIn(impactScope, "none", "timeline", "budget", "design_scope") {
			return nil, validationError("Việc không đạt cần người phụ trách, mức ưu tiên và ảnh hưởng hợp lệ")
		}
		var err error
		dueAt, err = constructionRequiredTime(req.DueAt, "Hạn xử lý")
		if err != nil {
			return nil, err
		}
	}
	return s.repo.CompleteConstructionCheck(ctx, checkID, result, strings.TrimSpace(req.Note), evidenceIDs, strings.TrimSpace(req.ActionTitle), cleanID(req.AssignedUserID), dueAt, impactScope, priority, actorID, actorName)
}

func (s *Service) CreateConstructionAction(ctx context.Context, projectID string, req ConstructionActionRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	sourceDailyReportID := cleanID(req.SourceDailyReportID)
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	impactScope := strings.TrimSpace(req.ImpactScope)
	if impactScope == "" {
		impactScope = "none"
	}
	if !validActivityUUID(projectID) || !validActivityUUID(cleanID(req.WorkItemID)) || strings.TrimSpace(req.Title) == "" ||
		!validActivityUUID(cleanID(req.AssignedUserID)) || !validActivityUUID(cleanID(req.EvidenceID)) ||
		!statusIn(priority, "normal", "urgent") || !statusIn(impactScope, "none", "timeline", "budget", "design_scope") {
		return nil, validationError("Việc cần xử lý cần nội dung, công việc, người phụ trách, ảnh và ảnh hưởng hợp lệ")
	}
	if sourceDailyReportID != "" && !validActivityUUID(sourceDailyReportID) {
		return nil, validationError("Báo cáo nguồn không hợp lệ")
	}
	dueAt, err := constructionRequiredTime(req.DueAt, "Hạn xử lý")
	if err != nil {
		return nil, err
	}
	return s.repo.CreateConstructionAction(ctx, projectID, cleanID(req.WorkItemID), sourceDailyReportID, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), priority, cleanID(req.AssignedUserID), dueAt, impactScope, cleanID(req.EvidenceID), actorID, actorName)
}

func (s *Service) UpdateConstructionAction(ctx context.Context, actionID string, req ConstructionActionUpdateRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	if !validActivityUUID(actionID) || strings.TrimSpace(req.Title) == "" || !validActivityUUID(cleanID(req.AssignedUserID)) || !statusIn(priority, "normal", "urgent") {
		return nil, validationError("Việc cần xử lý cần nội dung, người phụ trách và mức ưu tiên hợp lệ")
	}
	dueAt, err := constructionRequiredTime(req.DueAt, "Hạn xử lý")
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateConstructionAction(ctx, actionID, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), priority, cleanID(req.AssignedUserID), dueAt, actorID, actorName)
}

func (s *Service) SubmitConstructionActionResolution(ctx context.Context, actionID string, req ConstructionActionResolutionRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	evidenceID := cleanID(req.EvidenceID)
	if !validActivityUUID(actionID) || !validActivityUUID(evidenceID) {
		return nil, validationError("Kết quả khắc phục cần một minh chứng")
	}
	return s.repo.SubmitConstructionActionResolution(ctx, actionID, evidenceID, strings.TrimSpace(req.Note), actorID, actorName)
}

func (s *Service) VerifyConstructionAction(ctx context.Context, actionID string, req ConstructionActionVerifyRequest, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	result := strings.TrimSpace(req.Result)
	comment := strings.TrimSpace(req.Comment)
	if !validActivityUUID(actionID) || !statusIn(result, "pass", "fail") {
		return nil, validationError("Kết quả xác nhận phải là Đạt hoặc Chưa đạt")
	}
	if result == "fail" && comment == "" {
		return nil, validationError("Trả lại xử lý cần ghi rõ lý do")
	}
	return s.repo.VerifyConstructionAction(ctx, actionID, result, comment, actorID, actorName)
}

func (s *Service) ApproveConstructionActionImpact(ctx context.Context, actionID, actorID, actorName string) (Record, error) {
	actionID = cleanID(actionID)
	if !validActivityUUID(actionID) {
		return nil, validationError("Việc cần xử lý không hợp lệ")
	}
	return s.repo.ApproveConstructionActionImpact(ctx, actionID, actorID, actorName)
}

func constructionOptionalTime(raw, label string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return "", validationError(label + " không hợp lệ")
	}
	return value, nil
}

func constructionRequiredTime(raw, label string) (string, error) {
	value, err := constructionOptionalTime(raw, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", validationError(label + " là bắt buộc")
	}
	return value, nil
}

func (s *Service) ProjectIssues(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectIssues(ctx, cleanID(projectID))
}

func (s *Service) CreateIssue(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["reported_by"]) == "" {
		body["reported_by"] = actorID
	}
	row, err := s.repo.CreateIssue(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "issue_created", "issue", cleanID(row["id"]), fmt.Sprintf("Tạo vấn đề %s", row["title"]), Record{"severity": row["severity"], "status": row["status"]})
	return row, nil
}

func (s *Service) UpdateIssue(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	delete(body, "project_id")
	row, err := s.repo.UpdateIssue(ctx, cleanID(id), body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "issue_updated", "issue", cleanID(row["id"]), fmt.Sprintf("Cập nhật vấn đề %s", row["title"]), Record{"status": row["status"], "severity": row["severity"]})
	return row, nil
}

func (s *Service) ProjectReworks(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectReworks(ctx, cleanID(projectID))
}

func (s *Service) CreateRework(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["created_by"]) == "" {
		body["created_by"] = actorID
	}
	row, err := s.repo.CreateRework(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "rework_created", "rework", cleanID(row["id"]), fmt.Sprintf("Tạo yêu cầu sửa lỗi %s", row["title"]), Record{"status": row["status"], "issue_id": cleanID(row["issue_id"]), "acceptance_id": cleanID(row["acceptance_id"])})
	return row, nil
}

func (s *Service) UpdateReworkStatus(ctx context.Context, id, status string, actorID, actorName string) (Record, error) {
	status = strings.TrimSpace(status)
	if !statusIn(status, "approved", "done") {
		return nil, validationError("Trạng thái sửa lỗi không hợp lệ", Record{"status": status})
	}
	row, err := s.repo.UpdateReworkStatus(ctx, cleanID(id), status, actorID)
	if err != nil {
		return nil, err
	}
	event := "rework_approved"
	if status == "done" {
		event = "rework_completed"
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, event, "rework", cleanID(row["id"]), fmt.Sprintf("Cập nhật sửa lỗi sang %s", status), Record{"status": status})
	return row, nil
}

func (s *Service) ProjectReplannings(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectReplannings(ctx, cleanID(projectID))
}

func (s *Service) CreateReplanning(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	body["project_id"] = cleanID(projectID)
	if cleanID(body["created_by"]) == "" {
		body["created_by"] = actorID
	}
	row, err := s.repo.CreateReplanning(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "replanning_created", "replanning", cleanID(row["id"]), "Tạo yêu cầu lập kế hoạch lại", Record{"status": row["status"], "issue_id": cleanID(row["issue_id"]), "node_id": cleanID(row["node_id"])})
	return row, nil
}

func (s *Service) ApproveReplanning(ctx context.Context, id string, actorID, actorName string) (Record, error) {
	row, err := s.repo.ApproveReplanning(ctx, cleanID(id), actorID)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "replanning_approved", "replanning", cleanID(row["id"]), "Duyệt yêu cầu lập kế hoạch lại", Record{"status": row["status"]})
	return row, nil
}

func (s *Service) ProjectHandover(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectHandover(ctx, cleanID(projectID))
}

func (s *Service) HandoverOptions(ctx context.Context, projectID string) (Record, error) {
	return s.repo.HandoverOptions(ctx, cleanID(projectID))
}

func (s *Service) CreateHandover(ctx context.Context, projectID string, req HandoverCreateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	clientRepresentative := strings.TrimSpace(req.ClientRepresentative)
	companyRepresentative := strings.TrimSpace(req.CompanyRepresentative)
	if clientRepresentative == "" || companyRepresentative == "" {
		return nil, validationError("Hồ sơ bàn giao cần đại diện hai bên", Record{"client_representative": clientRepresentative, "company_representative": companyRepresentative})
	}
	handoverCode := strings.TrimSpace(req.HandoverCode)
	if handoverCode == "" {
		randomCode := strings.ToUpper(strings.ReplaceAll(newID(), "-", ""))
		handoverCode = "HO-" + randomCode[:10]
	}
	if len([]rune(handoverCode)) > 80 {
		return nil, validationError("Mã bàn giao không được vượt quá 80 ký tự")
	}
	handoverDate, err := normalizeOptionalDate(req.HandoverDate, "handover_date")
	if err != nil {
		return nil, err
	}
	finalAcceptanceID := cleanID(req.FinalAcceptanceID)
	if finalAcceptanceID != "" {
		acceptance, err := s.repo.GetByID(ctx, "acceptance_records", finalAcceptanceID)
		if err != nil || cleanID(acceptance["project_id"]) != projectID || fmt.Sprint(acceptance["type"]) != "final" {
			return nil, validationError("Nghiệm thu cuối không thuộc công trình", Record{"final_acceptance_id": finalAcceptanceID})
		}
	}
	formInstanceID := cleanID(req.WorkflowFormInstanceID)
	if formInstanceID != "" {
		instance, err := s.repo.GetByID(ctx, "workflow_form_instances", formInstanceID)
		if err != nil || cleanID(instance["project_id"]) != projectID {
			return nil, validationError("Biểu mẫu bàn giao không thuộc công trình", Record{"workflow_form_instance_id": formInstanceID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 4000 {
		return nil, validationError("Ghi chú bàn giao không được vượt quá 4000 ký tự")
	}
	return s.repo.CreateHandoverPackage(ctx, Record{
		"project_id": projectID, "handover_code": handoverCode, "status": "draft",
		"final_acceptance_id": finalAcceptanceID, "workflow_form_instance_id": formInstanceID,
		"handover_date": handoverDate, "client_representative": clientRepresentative,
		"company_representative": companyRepresentative, "documents_ready": false,
		"warehouse_closed": false, "payables_recorded": false, "receivables_recorded": false,
		"notes": notes, "created_by": actorID,
	}, actorID, actorName)
}

func (s *Service) ProjectBudget(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectBudget(ctx, cleanID(projectID))
}

func (s *Service) CreateBudget(ctx context.Context, projectID string, req BudgetCreateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	budgetCode := strings.TrimSpace(req.BudgetCode)
	name := strings.TrimSpace(req.Name)
	if !validActivityUUID(projectID) || budgetCode == "" || name == "" || req.PlannedAmount <= 0 {
		return nil, validationError("Ngân sách cần mã, tên và giá trị kế hoạch lớn hơn 0", Record{"project_id": projectID, "budget_code": budgetCode, "planned_amount": req.PlannedAmount})
	}
	if len([]rune(budgetCode)) > 100 || len([]rune(name)) > 255 {
		return nil, validationError("Mã hoặc tên ngân sách vượt quá độ dài cho phép")
	}
	body := Record{
		"project_id": projectID, "budget_code": budgetCode, "name": name,
		"planned_amount": req.PlannedAmount, "approved_amount": 0, "used_amount": 0,
		"remaining_amount": 0, "status": "draft",
	}
	row, err := s.repo.CreateBudget(ctx, body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "budget_created", "budget", cleanID(row["id"]), fmt.Sprintf("Tạo ngân sách %s", row["budget_code"]), Record{"status": row["status"], "planned_amount": row["planned_amount"]})
	return row, nil
}

func (s *Service) ApproveBudget(ctx context.Context, id string, actorID, actorName string) (Record, error) {
	row, err := s.repo.ApproveBudget(ctx, cleanID(id), actorID)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "budget_approved", "budget", cleanID(row["id"]), fmt.Sprintf("Duyệt ngân sách %s", row["budget_code"]), Record{"status": row["status"], "approved_amount": row["approved_amount"]})
	return row, nil
}

func (s *Service) CreatePayable(ctx context.Context, req PayableCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	code := strings.TrimSpace(req.PayableCode)
	if !validActivityUUID(projectID) || code == "" || req.GrossAmount <= 0 {
		return nil, validationError("Phải trả cần công trình, mã và giá trị lớn hơn 0", Record{"project_id": projectID, "payable_code": code, "gross_amount": req.GrossAmount})
	}
	if req.AdvanceAmount < 0 || req.RetentionAmount < 0 || req.DeductionAmount < 0 || req.AdvanceAmount+req.DeductionAmount > req.GrossAmount {
		return nil, validationError("Giá trị tạm ứng/giữ lại/khấu trừ không hợp lệ")
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	vendorType := strings.TrimSpace(req.VendorType)
	if vendorType == "" {
		vendorType = "supplier"
	}
	if !statusIn(vendorType, vendorPartnerTypes()...) {
		return nil, validationError("Loại vendor không hợp lệ", Record{"vendor_type": vendorType})
	}
	body := Record{
		"project_id": projectID, "payable_code": code, "vendor_type": vendorType,
		"vendor_partner_id": cleanID(req.VendorPartnerID), "vendor_name": strings.TrimSpace(req.VendorName),
		"source_type": "manual", "gross_amount": req.GrossAmount, "advance_amount": req.AdvanceAmount,
		"retention_amount": req.RetentionAmount, "deduction_amount": req.DeductionAmount,
		"paid_amount": 0, "remaining_amount": req.GrossAmount - req.AdvanceAmount - req.DeductionAmount,
		"due_date": dueDate, "status": "draft", "created_by": actorID,
	}
	if err := s.applyPartnerSnapshot(ctx, body, "vendor_partner_id", "vendor_name", "", vendorPartnerTypes()); err != nil {
		return nil, err
	}
	row, err := s.repo.Insert(ctx, "payables", payableColumns(), body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, projectID, actorID, actorName, "payable_created", "payable", cleanID(row["id"]), "Tạo phải trả ở trạng thái nháp", Record{"payable_code": code, "gross_amount": req.GrossAmount, "remaining_amount": row["remaining_amount"]})
	return row, nil
}

func (s *Service) CreateReceivable(ctx context.Context, req ReceivableCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	code := strings.TrimSpace(req.ReceivableCode)
	if !validActivityUUID(projectID) || code == "" || req.GrossAmount <= 0 {
		return nil, validationError("Phải thu cần công trình, mã và giá trị lớn hơn 0", Record{"project_id": projectID, "receivable_code": code, "gross_amount": req.GrossAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "receivable_code": code,
		"client_partner_id": cleanID(req.ClientPartnerID), "client_name": strings.TrimSpace(req.ClientName),
		"source_type": "manual", "contract_amount": req.ContractAmount,
		"milestone_name": strings.TrimSpace(req.MilestoneName), "gross_amount": req.GrossAmount,
		"received_amount": 0, "remaining_amount": req.GrossAmount, "due_date": dueDate,
		"status": "draft", "created_by": actorID,
	}
	if err := s.applyPartnerSnapshot(ctx, body, "client_partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	row, err := s.repo.Insert(ctx, "receivables", receivableColumns(), body)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, projectID, actorID, actorName, "receivable_created", "receivable", cleanID(row["id"]), "Tạo phải thu ở trạng thái nháp", Record{"receivable_code": code, "gross_amount": req.GrossAmount, "remaining_amount": row["remaining_amount"]})
	return row, nil
}

func (s *Service) ListTemplates(ctx context.Context) ([]Record, error) {
	return s.repo.Query(ctx, `
		SELECT t.*, COUNT(n.id) AS node_count
		FROM pipeline_templates t
		LEFT JOIN pipeline_template_nodes n ON n.template_id=t.id
		WHERE t.status NOT IN ('deleted','archived')
		GROUP BY t.id
		ORDER BY t.updated_at DESC, t.created_at DESC
	`)
}

func (s *Service) TemplateDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CreateTemplate(ctx context.Context, body Record, actorID string) (Record, error) {
	body["status"] = "draft"
	for _, key := range []string{"id", "template_family_id", "revision_no", "source_template_id", "definition_hash", "published_at", "published_by"} {
		delete(body, key)
	}
	return s.repo.CreateTemplateWithNodes(ctx, body, actorID)
}

func (s *Service) UpdateTemplate(ctx context.Context, id string, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "template", id); err != nil {
		return nil, err
	}
	for _, key := range []string{"id", "status", "version", "created_by", "template_family_id", "revision_no", "source_template_id", "engine_version", "definition_hash", "published_at", "published_by"} {
		delete(body, key)
	}
	return s.repo.Update(ctx, "pipeline_templates", templateColumns(), id, body)
}

func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	template, err := s.repo.GetByID(ctx, "pipeline_templates", id)
	if err != nil {
		return err
	}
	targetStatus := "deleted"
	if strings.TrimSpace(fmt.Sprint(template["status"])) == "published" {
		targetStatus = "archived"
	}
	return s.repo.Exec(ctx, `UPDATE pipeline_templates SET status=$2, updated_at=now() WHERE id=$1`, id, targetStatus)
}

func (s *Service) PublishTemplate(ctx context.Context, id, actorID, actorName string) (Record, error) {
	template, err := s.repo.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fmt.Sprint(template["status"])) == "published" {
		return template, nil
	}
	if strings.TrimSpace(fmt.Sprint(template["status"])) != "draft" {
		return nil, validationError("Chỉ revision nháp mới có thể được phát hành", Record{"template_id": id, "status": template["status"]})
	}
	issues, err := s.repo.TemplatePublishIssues(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, validationError("Mẫu quy trình chưa đạt điều kiện phát hành", Record{"issues": issues})
	}
	definitionHash, err := templateDefinitionHash(template)
	if err != nil {
		return nil, err
	}
	template, err = s.repo.QueryOne(ctx, `
		UPDATE pipeline_templates
		SET status='published', definition_hash=$2, published_at=now(), published_by=NULLIF($3,'')::uuid, updated_at=now()
		WHERE id=$1 AND status='draft'
		RETURNING *
	`, id, definitionHash, actorID)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, "", actorID, actorName, "pipeline_template_published", "pipeline_template", id, fmt.Sprintf("Phát hành mẫu %s", template["name"]), Record{})
	return s.repo.GetTemplateDetail(ctx, id)
}

func (s *Service) CloneTemplate(ctx context.Context, id, actorID string) (Record, error) {
	return s.repo.CloneTemplate(ctx, id, actorID)
}

func (s *Service) WorkflowFormCatalog(ctx context.Context) ([]Record, error) {
	return s.repo.WorkflowFormCatalog(ctx)
}

func (s *Service) WorkflowFormTemplateLibrary(ctx context.Context) ([]Record, error) {
	return s.repo.WorkflowFormTemplateLibrary(ctx)
}

func (s *Service) WorkflowFormTemplateDetail(ctx context.Context, templateID string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	return s.repo.WorkflowFormTemplateDetail(ctx, templateID)
}

func (s *Service) CreateWorkflowFormTemplate(ctx context.Context, req WorkflowFormTemplateRequest, actorID, actorName string) (Record, error) {
	normalized, err := normalizeWorkflowFormTemplateRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateWorkflowFormTemplate(ctx, normalized, actorID, actorName)
}

func (s *Service) UpdateWorkflowFormTemplate(ctx context.Context, templateID string, req WorkflowFormTemplateRequest, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	normalized, err := normalizeWorkflowFormTemplateRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateWorkflowFormTemplate(ctx, templateID, normalized, actorID, actorName)
}

func (s *Service) ArchiveWorkflowFormTemplate(ctx context.Context, templateID, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	return s.repo.ArchiveWorkflowFormTemplate(ctx, templateID, actorID, actorName)
}

func (s *Service) CreateWorkflowFormVersion(ctx context.Context, templateID string, req WorkflowFormVersionRequest, actorID, actorName string) (Record, error) {
	templateID = cleanID(templateID)
	if templateID == "" {
		return nil, validationError("ID mẫu biểu mẫu quy trình không hợp lệ", Record{"template_id": templateID})
	}
	normalized, err := normalizeWorkflowFormVersionRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateWorkflowFormVersion(ctx, templateID, normalized, actorID, actorName)
}

func (s *Service) UpdateWorkflowFormVersion(ctx context.Context, versionID string, req WorkflowFormVersionRequest, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	normalized, err := normalizeWorkflowFormVersionRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateWorkflowFormVersion(ctx, versionID, normalized, actorID, actorName)
}

func (s *Service) PublishWorkflowFormVersion(ctx context.Context, versionID, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	return s.repo.PublishWorkflowFormVersion(ctx, versionID, actorID, actorName)
}

func (s *Service) ArchiveWorkflowFormVersion(ctx context.Context, versionID, actorID, actorName string) (Record, error) {
	versionID = cleanID(versionID)
	if versionID == "" {
		return nil, validationError("ID phiên bản biểu mẫu quy trình không hợp lệ", Record{"version_id": versionID})
	}
	return s.repo.ArchiveWorkflowFormVersion(ctx, versionID, actorID, actorName)
}

func (s *Service) CreateTemplateStep(ctx context.Context, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "node", cleanID(body["node_id"])); err != nil {
		return nil, err
	}
	if err := s.applyTemplateStepFormDefaults(ctx, body, nil); err != nil {
		return nil, err
	}
	return s.repo.Insert(ctx, "pipeline_template_steps", templateStepColumns(), body)
}

func (s *Service) UpdateTemplateStep(ctx context.Context, id string, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "step", id); err != nil {
		return nil, err
	}
	current, err := s.repo.GetByID(ctx, "pipeline_template_steps", id)
	if err != nil {
		return nil, err
	}
	if err := s.applyTemplateStepFormDefaults(ctx, body, current); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, "pipeline_template_steps", templateStepColumns(), id, body)
}

func (s *Service) DeleteTemplateStep(ctx context.Context, id string) error {
	if _, err := s.ensureDraftTemplate(ctx, "step", id); err != nil {
		return err
	}
	return s.repo.Exec(ctx, `DELETE FROM pipeline_template_steps WHERE id=$1`, id)
}

func (s *Service) applyTemplateStepFormDefaults(ctx context.Context, body Record, current Record) error {
	if body == nil {
		return nil
	}
	stepType := "task"
	if current != nil {
		stepType = strings.TrimSpace(fmt.Sprint(current["step_type"]))
	}
	if raw, ok := body["step_type"]; ok {
		stepType = strings.TrimSpace(fmt.Sprint(raw))
	}
	if stepType == "" || stepType == "<nil>" {
		stepType = "task"
	}
	stepName := ""
	if current != nil {
		stepName = strings.TrimSpace(fmt.Sprint(current["name"]))
	}
	if raw, ok := body["name"]; ok {
		stepName = strings.TrimSpace(fmt.Sprint(raw))
	}
	checklistRequired := true
	if current != nil {
		checklistRequired = boolValue(current["checklist_required"])
	}
	if raw, ok := body["checklist_required"]; ok {
		checklistRequired = boolValue(raw)
	}
	formRequired := workflowFormRequiredForStep(stepType, checklistRequired)
	if raw, ok := body["form_required"]; ok {
		formRequired = boolValue(raw)
	} else {
		body["form_required"] = formRequired
	}
	if _, provided := body["surface_kind"]; !provided {
		if current != nil && strings.TrimSpace(fmt.Sprint(current["surface_kind"])) != "" && fmt.Sprint(current["surface_kind"]) != "<nil>" {
			body["surface_kind"] = current["surface_kind"]
		} else {
			body["surface_kind"] = defaultStepSurfaceKind(stepType, formRequired)
		}
	}
	_, formVersionProvided := body["form_version_id"]
	if !formVersionProvided && cleanID(current["form_version_id"]) != "" {
		return nil
	}
	if formVersionProvided && cleanID(body["form_version_id"]) != "" {
		return nil
	}
	if !formRequired {
		return nil
	}
	formVersionID, err := s.repo.DefaultWorkflowFormVersionID(ctx, stepType, checklistRequired, stepName)
	if err != nil {
		return err
	}
	if formVersionID != "" {
		body["form_version_id"] = formVersionID
	}
	return nil
}

func workflowFormRequiredForStep(stepType string, checklistRequired bool) bool {
	return checklistRequired || statusIn(stepType, "daily_report", "meeting_minutes", "checklist", "acceptance_check", "handover_check")
}

func defaultStepSurfaceKind(stepType string, formRequired bool) string {
	switch {
	case statusIn(stepType, "photo_upload", "evidence_upload"):
		return "evidence"
	case stepType == "approval":
		return "approval"
	case formRequired:
		return "workflow_form"
	default:
		return "generic"
	}
}

func normalizeWorkflowFormTemplateRequest(req WorkflowFormTemplateRequest, create bool) (Record, error) {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	formType := strings.TrimSpace(req.FormType)
	status := strings.TrimSpace(req.Status)
	if create {
		if code == "" || !validWorkflowFormCode(code) {
			return nil, validationError("Mã biểu mẫu quy trình không hợp lệ", Record{"code": code})
		}
		if formType == "" {
			formType = "generic_submission"
		}
		if !validWorkflowFormType(formType) {
			return nil, validationError("Loại biểu mẫu quy trình không hợp lệ", Record{"form_type": formType})
		}
	}
	if name == "" {
		return nil, validationError("Tên biểu mẫu quy trình là bắt buộc")
	}
	if status == "" {
		status = "active"
	}
	if !statusIn(status, "active", "archived") {
		return nil, validationError("Trạng thái mẫu biểu mẫu quy trình không hợp lệ", Record{"status": status})
	}
	out := Record{"name": name, "description": description, "status": status}
	if create {
		out["code"] = code
		out["form_type"] = formType
	}
	return out, nil
}

func normalizeWorkflowFormVersionRequest(req WorkflowFormVersionRequest, requireFields bool) (Record, error) {
	fields, err := normalizeWorkflowFormFieldRequests(req.Fields, requireFields)
	if err != nil {
		return nil, err
	}
	schema := req.Schema
	if schema == nil {
		schema = Record{}
	}
	validation := req.Validation
	if validation == nil {
		validation = Record{}
	}
	return Record{
		"source_version_id": cleanID(req.SourceVersionID),
		"schema_json":       schema,
		"validation_json":   validation,
		"fields":            fields,
	}, nil
}

func normalizeWorkflowFormFieldRequests(fields []WorkflowFormFieldRequest, requireFields bool) ([]Record, error) {
	if requireFields && len(fields) == 0 {
		return nil, validationError("Phiên bản biểu mẫu quy trình cần ít nhất một trường")
	}
	seen := map[string]bool{}
	out := make([]Record, 0, len(fields))
	for index, field := range fields {
		fieldKey := strings.TrimSpace(field.FieldKey)
		if !validWorkflowFieldKey(fieldKey) {
			return nil, validationError("Mã trường biểu mẫu quy trình không hợp lệ", Record{"field_key": fieldKey})
		}
		if seen[fieldKey] {
			return nil, validationError("Mã trường biểu mẫu quy trình bị trùng", Record{"field_key": fieldKey})
		}
		seen[fieldKey] = true
		label := strings.TrimSpace(field.Label)
		if label == "" {
			return nil, validationError("Nhãn trường biểu mẫu quy trình là bắt buộc", Record{"field_key": fieldKey})
		}
		fieldType := strings.TrimSpace(field.FieldType)
		if !validWorkflowFieldType(fieldType) {
			return nil, validationError("Loại trường biểu mẫu quy trình không hợp lệ", Record{"field_key": fieldKey, "field_type": fieldType})
		}
		orderIndex := field.OrderIndex
		if orderIndex <= 0 {
			orderIndex = (index + 1) * 10
		}
		options := field.Options
		if options == nil {
			options = []any{}
		}
		validation := field.Validation
		if validation == nil {
			validation = Record{}
		}
		visibility := field.VisibilityCondition
		if visibility == nil {
			visibility = Record{}
		}
		out = append(out, Record{
			"field_key":                 fieldKey,
			"label":                     label,
			"field_type":                fieldType,
			"required":                  field.Required,
			"options_json":              options,
			"validation_json":           validation,
			"help_text":                 strings.TrimSpace(field.HelpText),
			"order_index":               orderIndex,
			"visibility_condition_json": visibility,
		})
	}
	return out, nil
}

func validWorkflowFormCode(value string) bool {
	if value == "" {
		return false
	}
	return validWorkflowIdentifier(value)
}

func validWorkflowFieldKey(value string) bool {
	return validWorkflowIdentifier(value)
}

func validWorkflowIdentifier(value string) bool {
	for index, char := range value {
		if index == 0 {
			if char < 'a' || char > 'z' {
				return false
			}
			continue
		}
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return value != ""
}

func validWorkflowFormType(value string) bool {
	return statusIn(value, "checklist", "daily_report", "meeting_minutes", "generic_submission")
}

func validWorkflowFieldType(value string) bool {
	return statusIn(value, "text", "number", "date", "textarea", "select", "multi_select", "checkbox", "evidence_reference", "structured_list")
}

func (s *Service) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền chạy quy trình của công trình này")
	}
	canRun, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{"pipeline.run"})
	if err != nil {
		return nil, err
	}
	if !canRun {
		return nil, forbiddenError("Bạn không có quyền chạy quy trình", Record{"permissions": []string{"pipeline.run"}})
	}
	return s.repo.StartPipeline(ctx, projectID, templateID, actorID, actorName)
}

func (s *Service) ProjectWorkItems(ctx context.Context, projectID, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem công việc của công trình này")
	}
	return s.repo.ListProjectWorkItems(ctx, projectID, actorID)
}

func (s *Service) MyWorkItems(ctx context.Context, filter MyWorkItemsFilter, actorID string) (Record, error) {
	filter, offset, err := normalizeMyWorkItemsFilter(filter)
	if err != nil {
		return nil, err
	}
	page, err := s.repo.MyWorkItemsPage(ctx, actorID, filter, offset)
	if err != nil {
		return nil, err
	}
	for _, item := range page.Items {
		canAct := boolValue(item["can_act"])
		canDecide := boolValue(item["can_decide"])
		item["capabilities"] = Record{"can_act": canAct, "can_decide": canDecide}
		item["queue_scopes"] = myWorkItemQueueScopes(item, canDecide)
		item["available_actions"] = myWorkItemAvailableActions(item)
		for _, internalKey := range []string{
			"action_scope", "decision_required_roles", "decision_requested_by",
			"pipeline_current_node_id", "run_node_status", "is_mine", "is_department", "is_watching",
		} {
			delete(item, internalKey)
		}
	}
	return Record{
		"items":       page.Items,
		"summary":     page.Summary,
		"facets":      page.Facets,
		"total":       page.Total,
		"has_more":    page.HasMore,
		"next_cursor": page.NextCursor,
	}, nil
}

func (s *Service) MyActivityAttention(ctx context.Context, filter ActivityAttentionFilter, actorID string) ([]Record, error) {
	filter.Queue = strings.TrimSpace(filter.Queue)
	if filter.Queue == "" {
		filter.Queue = "all"
	}
	if !activityAttentionQueues[filter.Queue] {
		return nil, validationError("Hàng đợi hoạt động không hợp lệ", Record{"queue": filter.Queue})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" {
		if !validActivityUUID(filter.ProjectID) {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
		}
		allowed, err := s.repo.CanViewProjectOperations(ctx, filter.ProjectID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
		}
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 80
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMyActivityAttention(ctx, actorID, profile, filter)
}

const (
	projectOperationsViewFull            = ""
	projectOperationsViewWorkflowSidebar = "workflow-sidebar"
)

func (s *Service) ProjectOperations(ctx context.Context, projectID, actorID string, filter ProjectOperationsFilter) (Record, error) {
	filter.View = strings.ToLower(strings.TrimSpace(filter.View))
	if filter.View == "full" {
		filter.View = projectOperationsViewFull
	}
	if filter.View != projectOperationsViewFull && filter.View != projectOperationsViewWorkflowSidebar {
		return nil, validationError("Kiểu dữ liệu điều hành công trình không hợp lệ", Record{"view": filter.View})
	}
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem điều hành công trình này")
	}
	return s.repo.ProjectOperations(ctx, projectID, actorID, filter)
}

func (s *Service) ProjectEvidencePage(ctx context.Context, projectID string, filter ProjectEvidenceFilter, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hồ sơ của công trình này")
	}
	filter, err = normalizeProjectEvidenceFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	queryFilter := filter
	queryFilter.Limit = filter.Limit + 1
	rows, err := s.repo.ListProjectEvidence(ctx, projectID, profile, queryFilter)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = encodeEvidenceCursor(rows[len(rows)-1])
	}
	for _, row := range rows {
		row["can_review"] = false
		if statusIn(fmt.Sprint(row["status"]), "draft", "submitted") {
			canReview, err := s.repo.CanDecideWorkItem(ctx, Record{
				"id":                  row["work_item_id"],
				"project_id":          projectID,
				"status":              row["work_item_status"],
				"role_ids":            row["work_item_role_ids"],
				"approval_request_id": row["work_item_approval_request_id"],
			}, actorID)
			if err != nil {
				return nil, err
			}
			row["can_review"] = canReview
		}
		delete(row, "review_priority")
		delete(row, "work_item_role_ids")
		delete(row, "work_item_approval_request_id")
	}
	filteredTotal, err := s.repo.ProjectEvidenceCount(ctx, projectID, profile, filter)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.ProjectEvidenceSummary(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	options, err := s.repo.ProjectEvidenceOptions(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	return Record{
		"items":          rows,
		"summary":        summary,
		"options":        options,
		"filtered_total": filteredTotal,
		"has_more":       hasMore,
		"next_cursor":    nextCursor,
	}, nil
}

func (s *Service) ProjectActivity(ctx context.Context, projectID string, filter ProjectActivityFilter, actorID string) ([]Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	filter, err = normalizeProjectActivityFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ProjectActivityThread(ctx, projectID, profile, filter)
}

func (s *Service) ProjectActivityPage(ctx context.Context, projectID string, filter ProjectActivityFilter, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	filter, err = normalizeProjectActivityFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	queryFilter := filter
	queryFilter.Limit = filter.Limit + 1
	rows, err := s.repo.ProjectActivityThread(ctx, projectID, profile, queryFilter)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = encodeActivityCursor(rows[len(rows)-1])
	}
	summary, err := s.repo.ProjectActivitySummary(ctx, projectID, profile)
	if err != nil {
		return nil, err
	}
	return Record{
		"items":       rows,
		"summary":     summary,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	}, nil
}

func (s *Service) ProjectActivityOptions(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem tùy chọn trao đổi của công trình này")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.ProjectActivityOptions(ctx, projectID, profile)
}

func normalizeProjectEvidenceFilter(filter ProjectEvidenceFilter) (ProjectEvidenceFilter, error) {
	filter.EvidenceType = strings.ToLower(strings.TrimSpace(filter.EvidenceType))
	if filter.EvidenceType == "all" {
		filter.EvidenceType = ""
	}
	if filter.EvidenceType != "" && !statusIn(filter.EvidenceType, "document", "photo", "checklist", "link", "note", "other") {
		return filter, validationError("Loại minh chứng lọc không hợp lệ", Record{"evidence_type": filter.EvidenceType})
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "all" {
		filter.Status = ""
	}
	if filter.Status == "pending" || filter.Status == "submitted" || filter.Status == "draft" {
		filter.Status = "waiting"
	}
	if filter.Status != "" && !statusIn(filter.Status, "waiting", "accepted", "rejected") {
		return filter, validationError("Trạng thái hồ sơ lọc không hợp lệ", Record{"status": filter.Status})
	}
	filter.WorkItemID = cleanID(filter.WorkItemID)
	if filter.WorkItemID != "" && !validActivityUUID(filter.WorkItemID) {
		return filter, validationError("Đầu việc lọc không hợp lệ", Record{"work_item_id": filter.WorkItemID})
	}
	filter.SubmittedBy = cleanID(filter.SubmittedBy)
	if filter.SubmittedBy != "" && !validActivityUUID(filter.SubmittedBy) {
		return filter, validationError("Người gửi lọc không hợp lệ", Record{"submitted_by": filter.SubmittedBy})
	}
	filter.Visibility = strings.ToLower(strings.TrimSpace(filter.Visibility))
	if filter.Visibility != "" && !statusIn(filter.Visibility, "internal", "project", "client") {
		return filter, validationError("Phạm vi hiển thị lọc không hợp lệ", Record{"visibility": filter.Visibility})
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	var dateFrom, dateTo time.Time
	var err error
	if filter.DateFrom != "" {
		dateFrom, err = time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return filter, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": filter.DateFrom})
		}
	}
	if filter.DateTo != "" {
		dateTo, err = time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return filter, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": filter.DateTo})
		}
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		return filter, validationError("Khoảng thời gian hồ sơ không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa tìm kiếm quá dài", Record{"max_length": 200})
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Cursor != "" {
		priority, createdAt, evidenceID, err := parseEvidenceCursor(filter.Cursor)
		if err != nil {
			return filter, validationError("Con trỏ hồ sơ không hợp lệ")
		}
		filter.CursorPriority = priority
		filter.CursorCreatedAt = createdAt
		filter.CursorID = evidenceID
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 25
	}
	return filter, nil
}

func encodeEvidenceCursor(evidence Record) string {
	evidenceID := cleanID(evidence["id"])
	priority := int(number(evidence["review_priority"]))
	createdAt := ""
	switch value := evidence["created_at"].(type) {
	case time.Time:
		createdAt = value.UTC().Format(time.RFC3339Nano)
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			createdAt = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if createdAt == "" || evidenceID == "" || (priority != 0 && priority != 1) {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%s|%s", priority, createdAt, evidenceID)))
}

func parseEvidenceCursor(cursor string) (int, string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", "", err
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 || !validActivityUUID(parts[2]) {
		return 0, "", "", errors.New("invalid evidence cursor")
	}
	priority, err := strconv.Atoi(parts[0])
	if err != nil || (priority != 0 && priority != 1) {
		return 0, "", "", errors.New("invalid evidence cursor priority")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return 0, "", "", err
	}
	return priority, parsed.UTC().Format(time.RFC3339Nano), parts[2], nil
}

func normalizeProjectActivityFilter(filter ProjectActivityFilter) (ProjectActivityFilter, error) {
	filter.ScopeType = strings.ToLower(strings.TrimSpace(filter.ScopeType))
	filter.ScopeID = cleanID(filter.ScopeID)
	if filter.ScopeType != "" && !activityTargetScopes[filter.ScopeType] {
		return filter, validationError("Phạm vi hoạt động không hợp lệ", Record{"scope_type": filter.ScopeType})
	}
	if filter.ScopeID != "" && filter.ScopeType == "" {
		return filter, validationError("Cần scope_type khi lọc theo scope_id")
	}
	if filter.ScopeID != "" && !validActivityUUID(filter.ScopeID) {
		return filter, validationError("scope_id không hợp lệ", Record{"scope_id": filter.ScopeID})
	}
	filter.ActivityType = strings.ToLower(strings.TrimSpace(filter.ActivityType))
	if filter.ActivityType != "" && !activityFilterTypes[filter.ActivityType] {
		return filter, validationError("Loại hoạt động lọc không hợp lệ", Record{"activity_type": filter.ActivityType})
	}
	filter.ActorID = cleanID(filter.ActorID)
	if filter.ActorID != "" && !validActivityUUID(filter.ActorID) {
		return filter, validationError("Người gửi lọc không hợp lệ", Record{"actor_id": filter.ActorID})
	}
	filter.Visibility = strings.ToLower(strings.TrimSpace(filter.Visibility))
	if filter.Visibility != "" && !activityVisibilities[filter.Visibility] {
		return filter, validationError("Phạm vi hiển thị lọc không hợp lệ", Record{"visibility": filter.Visibility})
	}
	filter.Kind = strings.ToLower(strings.TrimSpace(filter.Kind))
	if filter.Kind == "all" {
		filter.Kind = ""
	}
	if filter.Kind != "" && filter.Kind != "discussion" && filter.Kind != "system" && filter.Kind != "decision" {
		return filter, validationError("Nhóm hoạt động lọc không hợp lệ", Record{"kind": filter.Kind})
	}
	filter.Attention = strings.ToLower(strings.TrimSpace(filter.Attention))
	if filter.Attention == "all" {
		filter.Attention = ""
	}
	if filter.Attention != "" && !activityAttentionQueues[filter.Attention] {
		return filter, validationError("Hàng đợi chú ý không hợp lệ", Record{"attention": filter.Attention})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa tìm kiếm quá dài", Record{"max_length": 200})
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Cursor != "" {
		createdAt, activityID, err := parseActivityCursor(filter.Cursor)
		if err != nil {
			return filter, validationError("Con trỏ hoạt động không hợp lệ")
		}
		filter.CursorCreatedAt = createdAt
		filter.CursorID = activityID
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 30
	}
	return filter, nil
}

func encodeActivityCursor(activity Record) string {
	activityID := cleanID(activity["id"])
	createdAt := ""
	if value := strings.TrimSpace(fmt.Sprint(activity["cursor_created_at"])); value != "" && value != "<nil>" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			createdAt = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	if createdAt == "" {
		switch value := activity["created_at"].(type) {
		case time.Time:
			createdAt = value.UTC().Format(time.RFC3339Nano)
		case string:
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				createdAt = parsed.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	if createdAt == "" || activityID == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt + "|" + activityID))
}

func parseActivityCursor(cursor string) (string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || !validActivityUUID(parts[1]) {
		return "", "", errors.New("invalid activity cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return "", "", err
	}
	return parsed.UTC().Format(time.RFC3339Nano), parts[1], nil
}

func (s *Service) WorkItemDetail(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID, profileIsClient(profile) && !profile.IsManager)
	if err != nil {
		return nil, err
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	canAct, err = s.myWorkItemCanActNow(ctx, item, actorID, canAct)
	if err != nil {
		return nil, err
	}
	canDecide := false
	if statusIn(fmt.Sprint(item["status"]), "waiting_review", "waiting_approval") {
		canDecide, err = s.repo.CanDecideWorkItem(ctx, item, actorID)
		if err != nil {
			return nil, err
		}
	}
	item["capabilities"] = Record{"can_act": canAct, "can_decide": canDecide}
	item["can_act"] = canAct
	item["can_decide"] = canDecide
	item["available_actions"] = myWorkItemAvailableActions(item)
	delete(item, "pipeline_current_node_id")
	delete(item, "run_node_status")
	return item, nil
}

func (s *Service) WorkItemForm(ctx context.Context, workItemID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem biểu mẫu quy trình này")
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false}, nil
	}
	if err != nil {
		return nil, err
	}
	form["available"] = true
	return form, nil
}

func (s *Service) SaveWorkflowFormDraft(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, false, actorID, actorName)
}

func (s *Service) SubmitWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, actorID, actorName string) (Record, error) {
	return s.saveWorkflowForm(ctx, instanceID, req, true, actorID, actorName)
}

func (s *Service) SubcontractorReconciliation(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Đầu việc chưa có biểu mẫu quy trình"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return Record{"available": false, "reason": "Biểu mẫu quy trình không phải checklist thầu phụ", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return Record{"available": false, "reason": "Gửi checklist thầu phụ trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.view", "boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát thầu phụ", Record{"permissions": []string{"boq.view", "operation.report"}})
	}
	return s.subcontractorReconciliationResponse(ctx, item, form)
}

func (s *Service) SaveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist thầu phụ", Record{"permissions": []string{"boq.update", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return nil, validationError("Biểu mẫu quy trình không phải checklist thầu phụ")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Gửi checklist thầu phụ trước khi đối soát", Record{"status": instance["status"]})
	}
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeSubcontractorReconciliationRequest(ctx, item, form, boqItems, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSubcontractorReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.subcontractorReconciliationResponse(ctx, item, form)
}

func (s *Service) subcontractorReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reconciliation, err := s.repo.GetSubcontractorReconciliation(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		sourceItems, err := subcontractorChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultSubcontractorReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                 "",
			"project_id":         projectID,
			"work_item_id":       item["id"],
			"form_instance_id":   instance["id"],
			"form_revision_no":   instance["current_revision"],
			"status":             "draft",
			"subcontractor_name": answers["subcontractor_name"],
			"work_package":       answers["work_package"],
			"construction_area":  answers["construction_area"],
			"summary_json":       subcontractorReconciliationSummary(items),
			"comment":            "",
			"items":              items,
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"boq_items":      boqItems,
	}, nil
}

func (s *Service) normalizeSubcontractorReconciliationRequest(ctx context.Context, item Record, form Record, boqItems []Record, req SubcontractorReconciliationRequest) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := subcontractorChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]SubcontractorReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	boqByID := map[string]Record{}
	for _, row := range boqItems {
		boqByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = SubcontractorReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeSubcontractorReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, boqByID)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, subcontractorReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeSubcontractorReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req SubcontractorReconciliationItemRequest, boqByID map[string]Record) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Hạng mục checklist thiếu tên", Record{"item_index": itemIndex})
	}
	claimed, err := optionalWorkflowNumber(source["claimed_quantity"])
	if err != nil {
		return nil, validationError("KL claim không hợp lệ", Record{"item_index": itemIndex})
	}
	confirmed, err := optionalWorkflowNumber(source["confirmed_quantity"])
	if err != nil {
		return nil, validationError("KL xác nhận không hợp lệ", Record{"item_index": itemIndex})
	}
	if req.ConfirmedQuantity != nil {
		if *req.ConfirmedQuantity < 0 {
			return nil, validationError("KL xác nhận không được âm", Record{"item_index": itemIndex})
		}
		value := *req.ConfirmedQuantity
		confirmed = &value
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultSubcontractorDecision(source)
	}
	if !statusIn(decision, "accepted", "adjust_quantity", "rework_required", "waiting_evidence", "rejected") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	if decision == "adjust_quantity" && confirmed == nil {
		return nil, validationError("Điều chỉnh KL cần KL xác nhận", Record{"item_index": itemIndex})
	}
	boqItemID := cleanID(req.BOQItemID)
	var boqItem Record
	if boqItemID != "" {
		var ok bool
		boqItem, ok = boqByID[boqItemID]
		if !ok {
			return nil, validationError("Dòng BOQ không thuộc công trình", Record{"item_index": itemIndex, "boq_item_id": boqItemID})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng đối soát không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	row := Record{
		"item_index":         itemIndex,
		"checklist_title":    title,
		"boq_item_id":        boqItemID,
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        evidenceID,
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            comment,
		"payload_json":       source,
	}
	if boqItem != nil {
		row["boq_work_item_name"] = boqItem["work_item_name"]
		row["boq_quantity"] = boqItem["quantity"]
		row["boq_unit"] = boqItem["unit"]
	}
	return row, nil
}

func isSubcontractorChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_subcontractor_checklist"
}

func subcontractorChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist thầu phụ chưa có hạng mục")
	}
	return workflowRecordList(rawItems)
}

func defaultSubcontractorReconciliationItem(index int, source Record) Record {
	decision := defaultSubcontractorDecision(source)
	claimed, _ := optionalWorkflowNumber(source["claimed_quantity"])
	confirmed, _ := optionalWorkflowNumber(source["confirmed_quantity"])
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	return Record{
		"item_index":         index,
		"checklist_title":    cleanOptionalText(source["title"]),
		"boq_item_id":        "",
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        cleanID(source["evidence_id"]),
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            cleanOptionalText(source["comment"]),
		"payload_json":       source,
	}
}

func defaultSubcontractorDecision(source Record) string {
	ownerResponse := cleanOptionalText(source["owner_response"])
	if statusIn(ownerResponse, "accepted", "adjust_quantity", "rework_required", "waiting_evidence") {
		return ownerResponse
	}
	status := cleanOptionalText(source["status"])
	if statusIn(status, "done", "not_applicable") {
		return "accepted"
	}
	return "waiting_evidence"
}

func subcontractorReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":            len(items),
		"accepted":         0,
		"adjust_quantity":  0,
		"rework_required":  0,
		"waiting_evidence": 0,
		"rejected":         0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}

func optionalWorkflowNumber(value any) (*float64, error) {
	if workflowFormValueEmpty(Record{"field_type": "number"}, value) {
		return nil, nil
	}
	numeric, err := workflowNumberValue(value)
	if err != nil {
		return nil, err
	}
	if numeric < 0 {
		return nil, errors.New("Số lượng không được âm")
	}
	return &numeric, nil
}

func numberOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func cleanOptionalText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func normalizePartnerBody(body Record, create bool) (Record, error) {
	if body == nil {
		body = Record{}
	}
	normalized := Record{}
	for key, value := range body {
		normalized[key] = value
	}
	if create && cleanOptionalText(normalized["partner_code"]) == "" {
		normalized["partner_code"] = fmt.Sprintf("PT-%d", time.Now().Unix())
	}
	for _, field := range []string{"partner_code", "name", "legal_name", "tax_code", "phone", "email", "address", "contact_name", "notes"} {
		if _, ok := normalized[field]; ok {
			normalized[field] = strings.TrimSpace(fmt.Sprint(normalized[field]))
		}
	}
	if create || normalized["name"] != nil {
		if cleanOptionalText(normalized["name"]) == "" {
			return nil, validationError("Tên đối tác là bắt buộc")
		}
	}
	for _, field := range []string{"partner_code", "name", "legal_name", "tax_code", "phone", "email", "contact_name"} {
		if len([]rune(cleanOptionalText(normalized[field]))) > 240 {
			return nil, validationError("Thông tin đối tác không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	if len([]rune(cleanOptionalText(normalized["address"]))) > 1000 {
		return nil, validationError("Địa chỉ đối tác không được vượt quá 1000 ký tự")
	}
	if len([]rune(cleanOptionalText(normalized["notes"]))) > 2000 {
		return nil, validationError("Ghi chú đối tác không được vượt quá 2000 ký tự")
	}
	if create || normalized["partner_types"] != nil {
		types, err := normalizePartnerTypes(normalized["partner_types"])
		if err != nil {
			return nil, err
		}
		if len(types) == 0 {
			return nil, validationError("Cần chọn ít nhất một loại đối tác")
		}
		normalized["partner_types"] = types
	}
	if create || normalized["payment_terms_days"] != nil {
		days := int(number(normalized["payment_terms_days"]))
		if days < 0 {
			return nil, validationError("Điều khoản thanh toán không được âm")
		}
		normalized["payment_terms_days"] = days
	}
	if create && normalized["active"] == nil {
		normalized["active"] = true
	}
	return normalized, nil
}

func normalizePartnerTypes(value any) ([]string, error) {
	out := make([]string, 0)
	seen := map[string]bool{}
	add := func(raw any) error {
		for _, part := range strings.Split(fmt.Sprint(raw), ",") {
			value := strings.TrimSpace(part)
			if value == "" || value == "<nil>" {
				continue
			}
			if !validPartnerType(value) {
				return validationError("Loại đối tác không hợp lệ", Record{"partner_type": value})
			}
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
		return nil
	}
	switch typed := value.(type) {
	case nil:
		return out, nil
	case []string:
		for _, item := range typed {
			if err := add(item); err != nil {
				return nil, err
			}
		}
	case []any:
		for _, item := range typed {
			if err := add(item); err != nil {
				return nil, err
			}
		}
	case string:
		text := strings.TrimSpace(typed)
		if strings.HasPrefix(text, "[") {
			var items []string
			if err := json.Unmarshal([]byte(text), &items); err == nil {
				for _, item := range items {
					if err := add(item); err != nil {
						return nil, err
					}
				}
				return out, nil
			}
		}
		if err := add(typed); err != nil {
			return nil, err
		}
	default:
		if err := add(typed); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validPartnerType(value string) bool {
	return statusIn(value, "client", "supplier", "subcontractor", "labor_team", "transport", "equipment_rental", "service_provider", "other")
}

func vendorPartnerTypes() []string {
	return []string{"supplier", "subcontractor", "labor_team", "transport", "equipment_rental", "service_provider", "other"}
}

func (s *Service) applyPartnerSnapshot(ctx context.Context, body Record, idField, nameField, taxCodeField string, allowedTypes []string) error {
	if body == nil {
		return nil
	}
	rawID, exists := body[idField]
	if !exists {
		return nil
	}
	partnerID := cleanID(rawID)
	if partnerID == "" {
		body[idField] = nil
		return nil
	}
	partner, err := s.requireActivePartner(ctx, partnerID, allowedTypes)
	if err != nil {
		return err
	}
	body[idField] = partnerID
	body[nameField] = cleanOptionalText(partner["name"])
	if taxCodeField != "" {
		body[taxCodeField] = cleanOptionalText(partner["tax_code"])
	}
	return nil
}

func (s *Service) requireActivePartner(ctx context.Context, partnerID string, allowedTypes []string) (Record, error) {
	if !validActivityUUID(partnerID) {
		return nil, validationError("partner_id không hợp lệ", Record{"partner_id": partnerID})
	}
	partner, err := s.repo.GetByID(ctx, "partners", partnerID)
	if err != nil {
		return nil, validationError("Đối tác không tồn tại", Record{"partner_id": partnerID})
	}
	if active, ok := partner["active"].(bool); ok && !active {
		return nil, validationError("Đối tác đang không hoạt động", Record{"partner_id": partnerID})
	}
	if len(allowedTypes) > 0 && !partnerHasAnyType(partner, allowedTypes) {
		return nil, validationError("Loại đối tác không phù hợp nghiệp vụ", Record{"partner_id": partnerID, "allowed_types": allowedTypes, "partner_types": partner["partner_types"]})
	}
	return partner, nil
}

func partnerHasAnyType(partner Record, allowedTypes []string) bool {
	types := map[string]bool{}
	switch raw := partner["partner_types"].(type) {
	case []any:
		for _, item := range raw {
			types[fmt.Sprint(item)] = true
		}
	case []string:
		for _, item := range raw {
			types[item] = true
		}
	case string:
		parsed, _ := normalizePartnerTypes(raw)
		for _, item := range parsed {
			types[item] = true
		}
	}
	for _, allowed := range allowedTypes {
		if types[allowed] {
			return true
		}
	}
	return false
}

func (s *Service) OwnerClientReconciliation(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Đầu việc chưa có biểu mẫu quy trình"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return Record{"available": false, "reason": "Biểu mẫu quy trình không phải checklist chủ đầu tư", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return Record{"available": false, "reason": "Gửi checklist chủ đầu tư trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát chủ đầu tư", Record{"permissions": []string{"acceptance.view", "operation.report"}})
	}
	return s.ownerClientReconciliationResponse(ctx, item, form)
}

func (s *Service) SaveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist chủ đầu tư", Record{"permissions": []string{"acceptance.approve", "acceptance.reject", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return nil, validationError("Biểu mẫu quy trình không phải checklist chủ đầu tư")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Gửi checklist chủ đầu tư trước khi đối soát", Record{"status": instance["status"]})
	}
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeOwnerClientReconciliationRequest(ctx, item, form, acceptances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveOwnerClientReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.ownerClientReconciliationResponse(ctx, item, form)
}

func (s *Service) ownerClientReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reconciliation, err := s.repo.GetOwnerClientReconciliation(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		sourceItems, err := ownerClientChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultOwnerClientReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                     "",
			"project_id":             projectID,
			"work_item_id":           item["id"],
			"form_instance_id":       instance["id"],
			"form_revision_no":       instance["current_revision"],
			"status":                 "draft",
			"client_representative":  answers["client_representative"],
			"company_representative": answers["company_representative"],
			"handover_scope":         answers["handover_scope"],
			"scope_detail":           answers["scope_detail"],
			"summary_json":           ownerClientReconciliationSummary(items),
			"comment":                "",
			"items":                  items,
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"acceptances":    acceptances,
	}, nil
}

func (s *Service) normalizeOwnerClientReconciliationRequest(ctx context.Context, item Record, form Record, acceptances []Record, req OwnerClientReconciliationRequest, submit bool) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := ownerClientChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]OwnerClientReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	acceptanceByID := map[string]Record{}
	for _, row := range acceptances {
		acceptanceByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = OwnerClientReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeOwnerClientReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, acceptanceByID, submit)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, ownerClientReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeOwnerClientReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req OwnerClientReconciliationItemRequest, acceptanceByID map[string]Record, submit bool) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Hạng mục checklist thiếu tên", Record{"item_index": itemIndex})
	}
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision != "" && !statusIn(clientDecision, "pass", "conditional_pass", "fail", "not_applicable") {
		return nil, validationError("Quyết định chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "client_decision": clientDecision})
	}
	severity := cleanOptionalText(source["severity"])
	if severity != "" && !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ checklist chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultOwnerClientDecision(source)
	}
	if !statusIn(decision, "accepted", "conditional_acceptance", "rework_required", "waiting_internal_acceptance", "rejected", "not_applicable") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if clientDecision == "fail" && decision == "accepted" {
		return nil, validationError("Không thể chấp nhận khi chủ đầu tư đánh không đạt", Record{"item_index": itemIndex})
	}
	acceptanceID := cleanID(req.AcceptanceID)
	var acceptance Record
	if acceptanceID != "" {
		var ok bool
		acceptance, ok = acceptanceByID[acceptanceID]
		if !ok {
			return nil, validationError("Nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
	}
	if submit && statusIn(decision, "accepted", "conditional_acceptance") {
		if acceptance == nil {
			return nil, validationError("Cần gắn nghiệm thu nội bộ cho dòng đã chấp nhận", Record{"item_index": itemIndex})
		}
		if fmt.Sprint(acceptance["status"]) != "completed" {
			return nil, validationError("Nghiệm thu nội bộ phải ở trạng thái hoàn tất", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
		result := cleanOptionalText(acceptance["result"])
		if decision == "accepted" && result != "pass" {
			return nil, validationError("Dòng đã chấp nhận cần nghiệm thu nội bộ đạt", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
		if decision == "conditional_acceptance" && !statusIn(result, "pass", "conditional_pass") {
			return nil, validationError("Dòng chấp nhận có điều kiện cần nghiệm thu nội bộ đạt hoặc đạt có điều kiện", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng đối soát không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	row := Record{
		"item_index":      itemIndex,
		"checklist_title": title,
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": clientDecision,
		"severity":        severity,
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     evidenceID,
		"acceptance_id":   acceptanceID,
		"decision":        decision,
		"comment":         comment,
		"payload_json":    source,
	}
	if acceptance != nil {
		row["acceptance_title"] = acceptance["title"]
		row["acceptance_type"] = acceptance["type"]
		row["acceptance_status"] = acceptance["status"]
		row["acceptance_result"] = acceptance["result"]
	}
	return row, nil
}

func isOwnerClientChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_owner_client_checklist"
}

func ownerClientChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist chủ đầu tư chưa có hạng mục")
	}
	return workflowRecordList(rawItems)
}

func defaultOwnerClientReconciliationItem(index int, source Record) Record {
	return Record{
		"item_index":      index,
		"checklist_title": cleanOptionalText(source["title"]),
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": cleanOptionalText(source["client_decision"]),
		"severity":        cleanOptionalText(source["severity"]),
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     cleanID(source["evidence_id"]),
		"acceptance_id":   "",
		"decision":        defaultOwnerClientDecision(source),
		"comment":         cleanOptionalText(source["comment"]),
		"payload_json":    source,
	}
}

func defaultOwnerClientDecision(source Record) string {
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision == "not_applicable" || cleanOptionalText(source["status"]) == "not_applicable" {
		return "not_applicable"
	}
	switch clientDecision {
	case "pass":
		return "accepted"
	case "conditional_pass":
		return "conditional_acceptance"
	case "fail":
		return "rework_required"
	default:
		return "waiting_internal_acceptance"
	}
}

func ownerClientReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":                       len(items),
		"accepted":                    0,
		"conditional_acceptance":      0,
		"rework_required":             0,
		"waiting_internal_acceptance": 0,
		"rejected":                    0,
		"not_applicable":              0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}

func (s *Service) MaterialSnapshot(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem ghi nhận vật tư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.materialSnapshotResponse(ctx, item)
}

func (s *Service) SaveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError("Bạn không có quyền ghi nhận vật tư", Record{"permissions": []string{"operation.report", "warehouse.issue"}})
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, header, err := s.normalizeMaterialSnapshotRequest(ctx, item, materials, balances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveMaterialSnapshot(ctx, item, header, normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	return s.materialSnapshotResponse(ctx, item)
}

func (s *Service) materialSnapshotResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		snapshot = Record{
			"id":                      "",
			"project_id":              projectID,
			"work_item_id":            item["id"],
			"status":                  "draft",
			"snapshot_date":           time.Now().Format("2006-01-02"),
			"construction_area":       "",
			"summary_json":            materialSnapshotSummary(nil),
			"comment":                 "",
			"applied_to_site_balance": false,
			"items":                   []Record{},
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"snapshot":          snapshot,
		"materials":         materials,
		"material_balances": balances,
	}, nil
}

func (s *Service) normalizeMaterialSnapshotRequest(ctx context.Context, item Record, materials []Record, balances []Record, req MaterialSnapshotRequest, submit bool) ([]Record, Record, Record, error) {
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, nil, validationError("Ghi chú ghi nhận vật tư không được vượt quá 2000 ký tự")
	}
	constructionArea := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(constructionArea)) > 240 {
		return nil, nil, nil, validationError("Khu vực thi công không được vượt quá 240 ký tự")
	}
	snapshotDate := strings.TrimSpace(req.SnapshotDate)
	if snapshotDate != "" {
		if _, err := time.Parse("2006-01-02", snapshotDate); err != nil {
			return nil, nil, nil, validationError("Ngày ghi nhận vật tư không hợp lệ", Record{"snapshot_date": snapshotDate})
		}
	} else if submit {
		return nil, nil, nil, validationError("Ngày ghi nhận vật tư là bắt buộc khi gửi")
	}
	if submit && len(req.Items) == 0 {
		return nil, nil, nil, validationError("Cần ít nhất một dòng vật tư để gửi ghi nhận")
	}

	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	balanceByMaterialID := map[string]Record{}
	for _, balance := range balances {
		balanceByMaterialID[cleanID(balance["material_id"])] = balance
	}
	seen := map[int]bool{}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, nil, validationError("Dòng ghi nhận vật tư không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, nil, validationError("Dòng ghi nhận vật tư bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeMaterialSnapshotItem(ctx, projectID, workItemID, itemIndex, materialByID, balanceByMaterialID, itemReq, submit)
		if err != nil {
			return nil, nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := materialSnapshotSummary(normalized)
	header := Record{
		"snapshot_date":     snapshotDate,
		"construction_area": constructionArea,
		"comment":           comment,
	}
	return normalized, summary, header, nil
}

func (s *Service) normalizeMaterialSnapshotItem(ctx context.Context, projectID, workItemID string, itemIndex int, materialByID map[string]Record, balanceByMaterialID map[string]Record, req MaterialSnapshotItemRequest, submit bool) (Record, error) {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "site_balance"
	}
	if !statusIn(source, "site_balance", "warehouse_issue", "subcontractor", "other") {
		return nil, validationError("Nguồn vật tư không hợp lệ", Record{"item_index": itemIndex, "source": source})
	}
	affectsSiteBalance := source == "site_balance" || source == "warehouse_issue"
	if req.AffectsSiteBalance != nil {
		affectsSiteBalance = *req.AffectsSiteBalance
	}

	materialID := cleanID(req.MaterialID)
	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	if affectsSiteBalance && materialID == "" {
		return nil, validationError("Dòng ảnh hưởng số dư vật tư công trình cần chọn vật tư", Record{"item_index": itemIndex})
	}

	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = strings.TrimSpace(fmt.Sprint(material["name"]))
		materialCode = strings.TrimSpace(fmt.Sprint(material["code"]))
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = strings.TrimSpace(fmt.Sprint(material["unit"]))
	}
	if unit == "" {
		return nil, validationError("Đơn vị vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị vật tư không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	beforeQuantity, err := materialSnapshotQuantity(req.BeforeQuantity, "Số lượng trước không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	addedQuantity, err := materialSnapshotQuantity(req.AddedQuantity, "Số lượng bổ sung không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	afterQuantity, err := materialSnapshotQuantity(req.AfterQuantity, "Số lượng sau không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	wasteQuantity, err := materialSnapshotQuantity(req.WasteQuantity, "Số lượng hao hụt/hư hỏng không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	usedQuantity := materialSnapshotRound(beforeQuantity + addedQuantity - afterQuantity - wasteQuantity)
	if usedQuantity < -0.000001 {
		return nil, validationError("Số lượng sử dụng tính ra không được âm", Record{
			"item_index":      itemIndex,
			"before_quantity": beforeQuantity,
			"added_quantity":  addedQuantity,
			"after_quantity":  afterQuantity,
			"waste_quantity":  wasteQuantity,
		})
	}
	if usedQuantity < 0 {
		usedQuantity = 0
	}

	var balance Record
	siteBalanceID := ""
	if affectsSiteBalance {
		balance = balanceByMaterialID[materialID]
		if balance == nil {
			return nil, validationError("Chưa có số dư vật tư tại công trình cho dòng snapshot", Record{"item_index": itemIndex, "material_id": materialID})
		}
		siteBalanceID = cleanID(balance["id"])
		if submit && materialSnapshotQuantityExceeds(usedQuantity, materialBalanceAvailable(balance)) {
			return nil, validationError("Số lượng sử dụng vượt vật tư còn tại công trình", Record{
				"item_index":  itemIndex,
				"material_id": materialID,
				"available":   materialBalanceAvailable(balance),
				"quantity":    usedQuantity,
			})
		}
	}

	beforeEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.BeforeEvidenceID, "before_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	afterEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.AfterEvidenceID, "after_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	area := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(area)) > 240 {
		return nil, validationError("Khu vực dòng snapshot không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng snapshot không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}

	return Record{
		"item_index":           itemIndex,
		"material_id":          materialID,
		"material_code":        materialCode,
		"material_name":        materialName,
		"unit":                 unit,
		"source":               source,
		"before_quantity":      beforeQuantity,
		"added_quantity":       addedQuantity,
		"after_quantity":       afterQuantity,
		"waste_quantity":       wasteQuantity,
		"used_quantity":        usedQuantity,
		"affects_site_balance": affectsSiteBalance,
		"site_balance_id":      siteBalanceID,
		"before_evidence_id":   beforeEvidenceID,
		"after_evidence_id":    afterEvidenceID,
		"construction_area":    area,
		"condition":            condition,
		"comment":              comment,
		"payload_json": Record{
			"source":                source,
			"affects_site_balance":  affectsSiteBalance,
			"computed_used_formula": "before_quantity + added_quantity - after_quantity - waste_quantity",
		},
	}, nil
}

func (s *Service) validateMaterialSnapshotEvidence(ctx context.Context, projectID, workItemID, evidenceID, field string, itemIndex int) (string, error) {
	evidenceID = cleanID(evidenceID)
	if evidenceID == "" {
		return "", nil
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return "", validationError("Minh chứng ghi nhận vật tư không hợp lệ", Record{"item_index": itemIndex, field: evidenceID})
	}
	if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
		return "", validationError("Minh chứng ghi nhận vật tư không thuộc đầu việc", Record{"item_index": itemIndex, field: evidenceID})
	}
	return evidenceID, nil
}

func materialSnapshotQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func materialSnapshotRound(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func materialSnapshotQuantityExceeds(quantity, available float64) bool {
	return quantity > available+0.000001
}

func materialBalanceAvailable(balance Record) float64 {
	issuedMinusUsed := number(balance["issued_quantity"]) - number(balance["used_quantity"])
	surplus := number(balance["surplus_quantity"])
	available := math.Max(issuedMinusUsed, surplus)
	if available < 0 {
		return 0
	}
	return available
}

func materialSnapshotSummary(items []Record) Record {
	summary := Record{
		"total":                   len(items),
		"affects_site_balance":    0,
		"complete_evidence":       0,
		"with_waste_or_damage":    0,
		"site_balance_used_total": 0.0,
		"non_balance_item_count":  0,
		"source_site_balance":     0,
		"source_warehouse_issue":  0,
		"source_subcontractor":    0,
		"source_other":            0,
	}
	for _, item := range items {
		source := cleanOptionalText(item["source"])
		sourceKey := "source_" + source
		if _, ok := summary[sourceKey]; ok {
			summary[sourceKey] = int(number(summary[sourceKey])) + 1
		}
		if boolValue(item["affects_site_balance"]) {
			summary["affects_site_balance"] = int(number(summary["affects_site_balance"])) + 1
			summary["site_balance_used_total"] = materialSnapshotRound(number(summary["site_balance_used_total"]) + number(item["used_quantity"]))
		} else {
			summary["non_balance_item_count"] = int(number(summary["non_balance_item_count"])) + 1
		}
		if cleanID(item["before_evidence_id"]) != "" && cleanID(item["after_evidence_id"]) != "" {
			summary["complete_evidence"] = int(number(summary["complete_evidence"])) + 1
		}
		if number(item["waste_quantity"]) > 0 || cleanOptionalText(item["condition"]) == "damaged" {
			summary["with_waste_or_damage"] = int(number(summary["with_waste_or_damage"])) + 1
		}
	}
	return summary
}

func (s *Service) SurplusHandlings(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.surplus_handle"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem xử lý vật tư dư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.surplusHandlingResponse(ctx, item)
}

func (s *Service) CreateSurplusHandling(ctx context.Context, workItemID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, workItemID, actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, "", normalized, actorID, actorName)
}

func (s *Service) UpdateSurplusHandling(ctx context.Context, handlingID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi, không thể sửa", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, cleanID(handling["id"]), normalized, actorID, actorName)
}

func (s *Service) SubmitSurplusHandling(ctx context.Context, handlingID string, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"warehouse.surplus_handle"}, "Bạn không có quyền gửi hồ sơ xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	req := SurplusHandlingRequest{
		SiteBalanceID:          cleanID(handling["site_balance_id"]),
		MaterialSnapshotID:     cleanID(handling["material_snapshot_id"]),
		MaterialSnapshotItemID: cleanID(handling["material_snapshot_item_id"]),
		HandlingType:           cleanOptionalText(handling["handling_type"]),
		Unit:                   cleanOptionalText(handling["unit"]),
		WarehouseID:            cleanID(handling["warehouse_id"]),
		CustodyLocation:        cleanOptionalText(handling["custody_location"]),
		Condition:              cleanOptionalText(handling["condition"]),
		EvidenceID:             cleanID(handling["evidence_id"]),
		Comment:                cleanOptionalText(handling["comment"]),
	}
	quantity := number(handling["quantity"])
	req.Quantity = &quantity
	if _, err := s.normalizeSurplusHandlingRequest(ctx, item, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitSurplusHandling(ctx, cleanID(handling["id"]), actorID, actorName)
}

func (s *Service) requireSurplusHandlingWorkItemAccess(ctx context.Context, workItemID, actorID string, permissions []string, message string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return item, nil
}

func (s *Service) surplusHandlingResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	handlings, err := s.repo.ListSurplusHandlingsForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"material_balances": balances,
		"warehouses":        warehouses,
		"handlings":         handlings,
	}, nil
}

func (s *Service) normalizeSurplusHandlingRequest(ctx context.Context, item Record, req SurplusHandlingRequest, submit bool) (Record, error) {
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	balanceByID := map[string]Record{}
	for _, balance := range balances {
		balanceByID[cleanID(balance["id"])] = balance
	}
	siteBalanceID := cleanID(req.SiteBalanceID)
	if siteBalanceID == "" {
		return nil, validationError("Số dư vật tư công trình là bắt buộc")
	}
	balance := balanceByID[siteBalanceID]
	if balance == nil {
		return nil, validationError("Số dư vật tư không thuộc công trình", Record{"site_balance_id": siteBalanceID})
	}
	surplusQuantity := number(balance["surplus_quantity"])
	if surplusQuantity <= 0 {
		return nil, validationError("Vật tư này không còn dư để xử lý", Record{"site_balance_id": siteBalanceID})
	}
	quantity, err := surplusHandlingQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	if quantityExceedsAvailable(quantity, surplusQuantity) {
		return nil, validationError("Số lượng xử lý vượt vật tư dư", Record{"available": surplusQuantity, "quantity": quantity})
	}
	handlingType := strings.TrimSpace(req.HandlingType)
	if handlingType == "" {
		handlingType = "return_to_warehouse"
	}
	if !statusIn(handlingType, "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại xử lý vật tư dư không hợp lệ", Record{"handling_type": handlingType})
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = cleanOptionalText(balance["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị xử lý vật tư dư là bắt buộc")
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị xử lý vật tư dư không được vượt quá 40 ký tự")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID == "" {
		return nil, validationError("Kho/kho tạm là bắt buộc")
	}
	if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
		return nil, err
	}
	custodyLocation := strings.TrimSpace(req.CustodyLocation)
	if len([]rune(custodyLocation)) > 240 {
		return nil, validationError("Vị trí lưu/trả vật tư không được vượt quá 240 ký tự")
	}
	if submit && custodyLocation == "" {
		return nil, validationError("Vị trí lưu/trả vật tư là bắt buộc khi gửi")
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư dư không hợp lệ", Record{"condition": condition})
	}
	evidenceID := cleanID(req.EvidenceID)
	if submit && evidenceID == "" {
		return nil, validationError("Minh chứng/ảnh vật tư dư là bắt buộc khi gửi")
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng xử lý vật tư dư không hợp lệ", Record{"evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng xử lý vật tư dư không thuộc đầu việc", Record{"evidence_id": evidenceID})
		}
	}
	materialSnapshotID, materialSnapshotItemID, err := s.validateSurplusHandlingSnapshotLink(ctx, workItemID, req.MaterialSnapshotID, req.MaterialSnapshotItemID)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú xử lý vật tư dư không được vượt quá 1000 ký tự")
	}
	return Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"site_balance_id":           siteBalanceID,
		"material_id":               cleanID(balance["material_id"]),
		"material_snapshot_id":      materialSnapshotID,
		"material_snapshot_item_id": materialSnapshotItemID,
		"handling_type":             handlingType,
		"quantity":                  materialSnapshotRound(quantity),
		"unit":                      unit,
		"warehouse_id":              warehouseID,
		"custody_location":          custodyLocation,
		"condition":                 condition,
		"evidence_id":               evidenceID,
		"comment":                   comment,
		"material_name":             cleanOptionalText(balance["material_name"]),
		"material_code":             cleanOptionalText(balance["code"]),
		"surplus_before_handling":   surplusQuantity,
	}, nil
}

func surplusHandlingQuantity(value *float64) (float64, error) {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 {
		return 0, validationError("Số lượng xử lý vật tư dư phải lớn hơn 0")
	}
	return materialSnapshotRound(*value), nil
}

func (s *Service) requireActiveWarehouse(ctx context.Context, warehouseID string) error {
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return err
	}
	for _, warehouse := range warehouses {
		if cleanID(warehouse["id"]) == warehouseID {
			return nil
		}
	}
	return validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
}

func (s *Service) validateSurplusHandlingSnapshotLink(ctx context.Context, workItemID, snapshotID, snapshotItemID string) (string, string, error) {
	snapshotID = cleanID(snapshotID)
	snapshotItemID = cleanID(snapshotItemID)
	if snapshotID == "" && snapshotItemID == "" {
		return "", "", nil
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, workItemID)
	if err != nil {
		return "", "", validationError("Ghi nhận vật tư liên kết không hợp lệ", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID != "" && cleanID(snapshot["id"]) != snapshotID {
		return "", "", validationError("Ghi nhận vật tư không thuộc đầu việc", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID == "" {
		snapshotID = cleanID(snapshot["id"])
	}
	if snapshotItemID != "" {
		items, _ := snapshot["items"].([]Record)
		found := false
		for _, item := range items {
			if cleanID(item["id"]) == snapshotItemID {
				found = true
				break
			}
		}
		if !found {
			return "", "", validationError("Dòng ghi nhận vật tư không thuộc đầu việc", Record{"material_snapshot_item_id": snapshotItemID})
		}
	}
	return snapshotID, snapshotItemID, nil
}

func (s *Service) ProjectIncomingAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.repo.ListIncomingAcceptancesForProject(ctx, projectID)
}

func (s *Service) WorkItemIncomingAcceptances(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view", "operation.report", "acceptance.create"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.incomingAcceptanceResponse(ctx, item)
}

func (s *Service) GetIncomingAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
		}
	}
	return record, nil
}

func (s *Service) CreateIncomingAcceptance(ctx context.Context, projectID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateIncomingAcceptance(ctx context.Context, acceptanceID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitIncomingAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := cleanID(existing["work_item_id"])
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.approve", "acceptance.create", "operation.report"}, "Bạn không có quyền gửi nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	req := incomingAcceptanceRequestFromRecord(existing)
	header, _, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.SubmitIncomingAcceptance(ctx, cleanID(existing["id"]), cleanOptionalText(header["result"]), actorID, actorName)
}

func (s *Service) requireIncomingAcceptanceAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	if workItemID == "" {
		return nil, nil
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Đầu việc không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) incomingAcceptanceResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	partners, err := s.repo.ListPartners(ctx, PartnerFilter{Status: "active"})
	if err != nil {
		return nil, err
	}
	acceptances, err := s.repo.ListIncomingAcceptancesForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, err
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return Record{
		"available":                 true,
		"work_item_id":              item["id"],
		"project_id":                projectID,
		"records":                   acceptances,
		"materials":                 materials,
		"warehouses":                warehouses,
		"partners":                  partners,
		"procurement_requests":      procurementRequests,
		"procurement_request_items": procurementItems,
	}, nil
}

func (s *Service) normalizeIncomingAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req IncomingAcceptanceRequest, submit bool) (Record, []Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" && item != nil {
		workItemID = cleanID(item["id"])
	}
	if workItemID == "" && existing != nil {
		workItemID = cleanID(existing["work_item_id"])
	}
	if workItemID != "" {
		if item == nil || cleanID(item["id"]) != workItemID {
			workItem, err := s.repo.GetWorkItemDetail(ctx, workItemID)
			if err != nil {
				return nil, nil, err
			}
			item = workItem
		}
		if cleanID(item["project_id"]) != projectID {
			return nil, nil, validationError("Đầu việc không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	sourceType := strings.TrimSpace(req.SourceType)
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if sourceType == "" && procurementRequestID != "" {
		sourceType = "procurement"
	}
	if sourceType == "" {
		sourceType = "supplier"
	}
	if !statusIn(sourceType, "supplier", "subcontractor", "procurement", "other") {
		return nil, nil, validationError("Nguồn nghiệm thu đầu vào không hợp lệ", Record{"source_type": sourceType})
	}
	supplierPartnerID := cleanID(req.SupplierPartnerID)
	subcontractorPartnerID := cleanID(req.SubcontractorPartnerID)
	supplierName := strings.TrimSpace(req.SupplierName)
	subcontractorName := strings.TrimSpace(req.SubcontractorName)
	if supplierPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, supplierPartnerID, []string{"supplier", "service_provider", "other"})
		if err != nil {
			return nil, nil, err
		}
		supplierName = cleanOptionalText(partner["name"])
	}
	if subcontractorPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, subcontractorPartnerID, []string{"subcontractor", "labor_team"})
		if err != nil {
			return nil, nil, err
		}
		subcontractorName = cleanOptionalText(partner["name"])
	}
	deliveryReference := strings.TrimSpace(req.DeliveryReference)
	for field, value := range map[string]string{
		"supplier_name":      supplierName,
		"subcontractor_name": subcontractorName,
		"delivery_reference": deliveryReference,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu đầu vào không được vượt quá 2000 ký tự")
	}
	receivedAt := strings.TrimSpace(req.ReceivedAt)
	if receivedAt != "" {
		if _, err := time.Parse("2006-01-02", receivedAt); err != nil {
			return nil, nil, validationError("Ngày nhận vật tư không hợp lệ", Record{"received_at": receivedAt})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nhận vật tư là bắt buộc khi gửi")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID != "" {
		if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
			return nil, nil, err
		}
	}

	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	if procurementRequestID != "" {
		found := false
		for _, request := range procurementRequests {
			if cleanID(request["id"]) == procurementRequestID {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, validationError("Yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"procurement_request_id": procurementRequestID})
		}
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, nil, err
	}
	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	procurementItemByID := map[string]Record{}
	for _, item := range procurementItems {
		procurementItemByID[cleanID(item["id"])] = item
	}

	if submit && len(req.Items) == 0 {
		return nil, nil, validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
	}
	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeIncomingAcceptanceItem(ctx, projectID, workItemID, procurementRequestID, itemIndex, materialByID, procurementItemByID, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := incomingAcceptanceSummary(normalized)
	result := strings.TrimSpace(req.Result)
	if result == "" && submit {
		result = incomingAcceptanceResultFromSummary(summary)
	}
	if result != "" && !statusIn(result, "pass", "conditional_pass", "fail") {
		return nil, nil, validationError("Kết quả nghiệm thu đầu vào không hợp lệ", Record{"result": result})
	}
	if submit {
		if result == "" {
			return nil, nil, validationError("Kết quả nghiệm thu đầu vào là bắt buộc khi gửi")
		}
		if err := validateIncomingAcceptanceResult(result, summary); err != nil {
			return nil, nil, err
		}
	}
	header := Record{
		"project_id":               projectID,
		"work_item_id":             workItemID,
		"procurement_request_id":   procurementRequestID,
		"warehouse_id":             warehouseID,
		"source_type":              sourceType,
		"supplier_partner_id":      supplierPartnerID,
		"supplier_name":            supplierName,
		"subcontractor_partner_id": subcontractorPartnerID,
		"subcontractor_name":       subcontractorName,
		"delivery_reference":       deliveryReference,
		"received_at":              receivedAt,
		"result":                   result,
		"comment":                  comment,
		"summary_json":             summary,
		"incoming_material_count":  summary["total"],
	}
	return header, normalized, nil
}

func (s *Service) normalizeIncomingAcceptanceItem(ctx context.Context, projectID, workItemID, procurementRequestID string, itemIndex int, materialByID, procurementItemByID map[string]Record, req IncomingAcceptanceItemRequest, submit bool) (Record, error) {
	materialID := cleanID(req.MaterialID)
	procurementRequestItemID := cleanID(req.ProcurementRequestItemID)
	var procurementItem Record
	if procurementRequestItemID != "" {
		var ok bool
		procurementItem, ok = procurementItemByID[procurementRequestItemID]
		if !ok {
			return nil, validationError("Dòng yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		itemRequestID := cleanID(procurementItem["request_id"])
		if procurementRequestID != "" && itemRequestID != procurementRequestID {
			return nil, validationError("Dòng yêu cầu mua không thuộc yêu cầu mua đã chọn", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		procurementMaterialID := cleanID(procurementItem["material_id"])
		if materialID == "" {
			materialID = procurementMaterialID
		}
		if materialID != "" && procurementMaterialID != "" && materialID != procurementMaterialID {
			return nil, validationError("Vật tư không khớp dòng yêu cầu mua", Record{"item_index": itemIndex, "material_id": materialID, "procurement_request_item_id": procurementRequestItemID})
		}
	}

	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = cleanOptionalText(material["name"])
		materialCode = cleanOptionalText(material["code"])
	} else if procurementItem != nil {
		materialName = cleanOptionalText(procurementItem["material_name"])
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = cleanOptionalText(material["unit"])
	}
	if unit == "" && procurementItem != nil {
		unit = cleanOptionalText(procurementItem["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị nghiệm thu đầu vào không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	deliveredQuantity, err := incomingAcceptanceQuantity(req.DeliveredQuantity, "Số lượng giao không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	acceptedQuantity, err := incomingAcceptanceQuantity(req.AcceptedQuantity, "Số lượng nhận không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	rejectedQuantity, err := incomingAcceptanceQuantity(req.RejectedQuantity, "Số lượng từ chối không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	if submit && deliveredQuantity <= 0 {
		return nil, validationError("Số lượng giao phải lớn hơn 0 khi gửi", Record{"item_index": itemIndex})
	}
	if materialSnapshotQuantityExceeds(acceptedQuantity+rejectedQuantity, deliveredQuantity) {
		return nil, validationError("Số lượng nhận và từ chối vượt số lượng giao", Record{"item_index": itemIndex, "delivered_quantity": deliveredQuantity})
	}
	if submit && procurementItem != nil {
		alreadyAccepted, err := s.repo.QueryOne(ctx, `
			SELECT COALESCE(SUM(iai.accepted_quantity),0) AS accepted_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id
			WHERE iai.procurement_request_item_id=$1 AND iar.status='submitted'
		`, procurementRequestItemID)
		if err != nil {
			return nil, err
		}
		acceptedBefore := number(alreadyAccepted["accepted_quantity"])
		requested := number(procurementItem["quantity_requested"])
		if materialSnapshotQuantityExceeds(acceptedBefore+acceptedQuantity, requested) {
			return nil, validationError("Khối lượng nghiệm thu lũy kế vượt khối lượng yêu cầu mua", Record{
				"item_index": itemIndex, "quantity_requested": requested, "accepted_before": acceptedBefore, "accepted_quantity": acceptedQuantity,
			})
		}
	}

	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = incomingAcceptanceDefaultDecision(deliveredQuantity, acceptedQuantity, rejectedQuantity)
	}
	if !statusIn(decision, "pending", "accepted", "conditional_acceptance", "rejected") {
		return nil, validationError("Quyết định dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit {
		if decision == "pending" {
			return nil, validationError("Dòng nghiệm thu đầu vào chưa có quyết định", Record{"item_index": itemIndex})
		}
		if err := validateIncomingAcceptanceDecision(decision, deliveredQuantity, acceptedQuantity, rejectedQuantity); err != nil {
			return nil, validationError(err.Error(), Record{"item_index": itemIndex, "decision": decision})
		}
	}

	defectNote := strings.TrimSpace(req.DefectNote)
	if len([]rune(defectNote)) > 1000 {
		return nil, validationError("Mô tả lỗi nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceID := cleanID(req.EvidenceID)
	evidenceRequired := decision != "accepted" || condition != "usable" || rejectedQuantity > 0
	if submit && evidenceRequired && evidenceID == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/có điều kiện cần minh chứng", Record{"item_index": itemIndex})
	}
	if submit && evidenceRequired && defectNote == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/có điều kiện cần mô tả lỗi", Record{"item_index": itemIndex})
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}

	return Record{
		"item_index":                  itemIndex,
		"material_id":                 materialID,
		"material_code":               materialCode,
		"material_name":               materialName,
		"procurement_request_item_id": procurementRequestItemID,
		"delivered_quantity":          deliveredQuantity,
		"accepted_quantity":           acceptedQuantity,
		"rejected_quantity":           rejectedQuantity,
		"unit":                        unit,
		"condition":                   condition,
		"defect_note":                 defectNote,
		"evidence_id":                 evidenceID,
		"decision":                    decision,
		"comment":                     comment,
		"payload_json": Record{
			"procurement_request_item_id": procurementRequestItemID,
			"accepted_plus_rejected":      materialSnapshotRound(acceptedQuantity + rejectedQuantity),
		},
	}, nil
}

func incomingAcceptanceRequestFromRecord(record Record) IncomingAcceptanceRequest {
	req := IncomingAcceptanceRequest{
		WorkItemID:             cleanID(record["work_item_id"]),
		ProcurementRequestID:   cleanID(record["procurement_request_id"]),
		WarehouseID:            cleanID(record["warehouse_id"]),
		SourceType:             cleanOptionalText(record["source_type"]),
		SupplierPartnerID:      cleanID(record["supplier_partner_id"]),
		SupplierName:           cleanOptionalText(record["supplier_name"]),
		SubcontractorPartnerID: cleanID(record["subcontractor_partner_id"]),
		SubcontractorName:      cleanOptionalText(record["subcontractor_name"]),
		DeliveryReference:      cleanOptionalText(record["delivery_reference"]),
		ReceivedAt:             dateOnly(record["received_at"]),
		Result:                 cleanOptionalText(record["result"]),
		Comment:                cleanOptionalText(record["comment"]),
		Items:                  []IncomingAcceptanceItemRequest{},
	}
	if items, ok := record["items"].([]Record); ok {
		req.Items = make([]IncomingAcceptanceItemRequest, 0, len(items))
		for _, item := range items {
			delivered := number(item["delivered_quantity"])
			accepted := number(item["accepted_quantity"])
			rejected := number(item["rejected_quantity"])
			req.Items = append(req.Items, IncomingAcceptanceItemRequest{
				ItemIndex:                int(number(item["item_index"])),
				MaterialID:               cleanID(item["material_id"]),
				MaterialName:             cleanOptionalText(item["material_name"]),
				ProcurementRequestItemID: cleanID(item["procurement_request_item_id"]),
				DeliveredQuantity:        &delivered,
				AcceptedQuantity:         &accepted,
				RejectedQuantity:         &rejected,
				Unit:                     cleanOptionalText(item["unit"]),
				Condition:                cleanOptionalText(item["condition"]),
				DefectNote:               cleanOptionalText(item["defect_note"]),
				EvidenceID:               cleanID(item["evidence_id"]),
				Decision:                 cleanOptionalText(item["decision"]),
				Comment:                  cleanOptionalText(item["comment"]),
			})
		}
	}
	return req
}

func incomingAcceptanceQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func incomingAcceptanceDefaultDecision(delivered, accepted, rejected float64) string {
	if delivered <= 0 && accepted <= 0 && rejected <= 0 {
		return "pending"
	}
	if rejected > 0 && accepted > 0 {
		return "conditional_acceptance"
	}
	if rejected > 0 {
		return "rejected"
	}
	if accepted > 0 && !materialSnapshotQuantityExceeds(delivered, accepted) && !materialSnapshotQuantityExceeds(accepted, delivered) {
		return "accepted"
	}
	return "pending"
}

func validateIncomingAcceptanceDecision(decision string, delivered, accepted, rejected float64) error {
	switch decision {
	case "accepted":
		if rejected > 0 || accepted <= 0 || materialSnapshotQuantityExceeds(accepted, delivered) || materialSnapshotQuantityExceeds(delivered, accepted) {
			return errors.New("Dòng được chấp nhận phải nhận đủ và không có số lượng từ chối")
		}
	case "conditional_acceptance":
		if accepted <= 0 || rejected <= 0 {
			return errors.New("Dòng có điều kiện cần có cả số lượng nhận và từ chối")
		}
	case "rejected":
		if rejected <= 0 || accepted > 0 {
			return errors.New("Dòng bị từ chối phải có số lượng từ chối và không có số lượng nhận")
		}
	}
	return nil
}

func incomingAcceptanceSummary(items []Record) Record {
	summary := Record{
		"total":                len(items),
		"accepted":             0,
		"conditional":          0,
		"rejected":             0,
		"pending":              0,
		"defect_count":         0,
		"evidence_count":       0,
		"delivered_quantity":   0.0,
		"accepted_quantity":    0.0,
		"rejected_quantity":    0.0,
		"with_procurement_ref": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "accepted":
			summary["accepted"] = int(number(summary["accepted"])) + 1
		case "conditional_acceptance":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "rejected":
			summary["rejected"] = int(number(summary["rejected"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanOptionalText(item["defect_note"]) != "" || number(item["rejected_quantity"]) > 0 || cleanOptionalText(item["condition"]) != "usable" {
			summary["defect_count"] = int(number(summary["defect_count"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanID(item["procurement_request_item_id"]) != "" {
			summary["with_procurement_ref"] = int(number(summary["with_procurement_ref"])) + 1
		}
		summary["delivered_quantity"] = materialSnapshotRound(number(summary["delivered_quantity"]) + number(item["delivered_quantity"]))
		summary["accepted_quantity"] = materialSnapshotRound(number(summary["accepted_quantity"]) + number(item["accepted_quantity"]))
		summary["rejected_quantity"] = materialSnapshotRound(number(summary["rejected_quantity"]) + number(item["rejected_quantity"]))
	}
	return summary
}

func incomingAcceptanceResultFromSummary(summary Record) string {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	if total == 0 {
		return ""
	}
	if accepted == total {
		return "pass"
	}
	if rejected == total {
		return "fail"
	}
	if conditional > 0 || rejected > 0 {
		return "conditional_pass"
	}
	return ""
}

func validateIncomingAcceptanceResult(result string, summary Record) error {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
	}
	if pending > 0 {
		return validationError("Không thể gửi nghiệm thu đầu vào khi còn dòng chờ xử lý", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if accepted != total {
			return validationError("Kết quả đạt chỉ hợp lệ khi mọi dòng đều được chấp nhận")
		}
	case "conditional_pass":
		if conditional+rejected == 0 {
			return validationError("Kết quả đạt có điều kiện cần có dòng có điều kiện hoặc bị từ chối")
		}
	case "fail":
		if rejected == 0 || accepted > 0 || conditional > 0 {
			return validationError("Kết quả không đạt chỉ hợp lệ khi có dòng bị từ chối và không có dòng được chấp nhận/có điều kiện")
		}
	}
	return nil
}

func dateOnly(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}

func (s *Service) SignWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSignRequest, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = "internal_ack"
	}
	if !validWorkflowSignaturePurpose(purpose) {
		return nil, validationError("Mục đích ký biểu mẫu quy trình không hợp lệ", Record{"purpose": purpose})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú chữ ký không được vượt quá 1000 ký tự")
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi ký nội bộ", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền ký biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	imageBytes, imageMIME, err := parseSignatureImageDataURL(req.SignatureImageDataURL)
	if err != nil {
		return nil, err
	}
	strokes, err := validateSignatureStrokes(req.SignatureStrokes)
	if err != nil {
		return nil, err
	}
	storageKey := path.Join("workflow-signatures", cleanID(instance["project_id"]), instanceID, uuid.NewString()+".png")
	visualFile, storedPath, err := s.storeGeneratedFile(storageKey, imageBytes, imageMIME)
	if err != nil {
		return nil, err
	}
	signedForm, err := s.repo.SignWorkflowForm(ctx, instanceID, purpose, comment, visualFile, strokes, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return signedForm, nil
}

func validWorkflowSignaturePurpose(purpose string) bool {
	return statusIn(purpose, "internal_ack", "prepared_by", "reviewed_by", "approved_by", "received_by")
}

func (s *Service) GenerateWorkflowFormPDFExport(ctx context.Context, instanceID, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Biểu mẫu quy trình phải được gửi trước khi xuất PDF", Record{"status": instance["status"]})
	}
	workItemID := cleanID(instance["work_item_id"])
	projectID := cleanID(instance["project_id"])
	if workItemID == "" || projectID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xuất PDF biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDF(form)
	if err != nil {
		return nil, err
	}
	revisionNo := int(number(instance["current_revision"]))
	fileName := workflowFormPDFFileName(form, revisionNo)
	storageKey := path.Join("workflow-form-pdfs", projectID, instanceID, uuid.NewString()+".pdf")
	pdfFile, storedPath, err := s.storeGeneratedFile(storageKey, pdfBytes, "application/pdf")
	if err != nil {
		return nil, err
	}
	exportedForm, err := s.repo.CreateWorkflowFormPDFExport(ctx, instanceID, fileName, fmt.Sprint(instance["title"]), pdfFile, payloadHash, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return exportedForm, nil
}

func (s *Service) WorkflowFormSignatureVisual(ctx context.Context, signatureID, actorID string) (EvidenceFileResult, error) {
	signatureID = cleanID(signatureID)
	if !validActivityUUID(signatureID) {
		return EvidenceFileResult{}, validationError("signature_id không hợp lệ", Record{"signature_id": signatureID})
	}
	signature, err := s.repo.GetWorkflowFormSignatureDetail(ctx, signatureID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(signature["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem chữ ký này")
	}
	return s.storedFileResult(signature, "visual_storage_key", "visual_mime_type", signatureVisualFilename(signature))
}

func (s *Service) WorkflowFormPDFExportFile(ctx context.Context, exportID, actorID string) (EvidenceFileResult, error) {
	exportID = cleanID(exportID)
	if !validActivityUUID(exportID) {
		return EvidenceFileResult{}, validationError("pdf_export_id không hợp lệ", Record{"pdf_export_id": exportID})
	}
	export, err := s.repo.GetWorkflowFormPDFExportDetail(ctx, exportID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(export["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem PDF biểu mẫu quy trình này")
	}
	return s.storedFileResult(export, "storage_key", "mime_type", fmt.Sprint(export["file_name"]))
}

func (s *Service) saveWorkflowForm(ctx context.Context, instanceID string, req WorkflowFormSaveRequest, submit bool, actorID, actorName string) (Record, error) {
	if !validActivityUUID(instanceID) {
		return nil, validationError("workflow_form_instance_id không hợp lệ", Record{"workflow_form_instance_id": instanceID})
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	workItemID := cleanID(instance["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Biểu mẫu quy trình chưa liên kết đầu việc", Record{"workflow_form_instance_id": instanceID})
	}
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác biểu mẫu quy trình này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	if submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái biểu mẫu quy trình không cho phép gửi", Record{"status": instance["status"]})
	}
	if !submit && statusIn(fmt.Sprint(instance["status"]), "submitted", "cancelled") {
		return nil, validationError("Trạng thái biểu mẫu quy trình không cho phép lưu nháp", Record{"status": instance["status"]})
	}
	fields, _ := form["fields"].([]Record)
	normalized, err := s.validateWorkflowFormAnswers(ctx, instance, fields, req.Answers, submit)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveWorkflowForm(ctx, instanceID, normalized, strings.TrimSpace(req.Comment), submit, actorID, actorName)
}

func (s *Service) WorkItemAction(ctx context.Context, workItemID, action string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	plan, err := workItemTransitionForAction(action, item)
	if err != nil {
		return nil, err
	}
	if plan.RequiresReason && workItemActionComment(req) == "" {
		return nil, validationError("Lý do thao tác là bắt buộc", Record{"action": action})
	}
	if plan.Decision {
		allowed, err := s.repo.CanDecideWorkItem(ctx, item, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền duyệt đầu việc này")
		}
	} else {
		allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
		}
		if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
			return nil, err
		}
	}
	if plan.RequiresEvidence {
		missing, err := s.missingRequiredEvidence(ctx, item, req)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return nil, validationError("Đầu việc cần đủ minh chứng trước khi gửi rà soát, gửi duyệt hoặc hoàn tất", Record{"missing_evidence": missing})
		}
	}
	if plan.ToStatus == "completed" {
		if err := s.repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName); err != nil {
			return nil, err
		}
	}
	if plan.Decision {
		updated, err := s.repo.ApplyWorkItemDecision(ctx, item, action, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
		if err != nil {
			return nil, err
		}
		s.publishWorkItemChanged(updated, actorID)
		return updated, nil
	}
	if plan.CreatesApproval {
		updated, err := s.repo.RequestWorkItemApproval(ctx, item, req, actorID, actorName)
		if err != nil {
			return nil, err
		}
		s.publishWorkItemChanged(updated, actorID)
		return updated, nil
	}
	updated, err := s.repo.ApplyWorkItemTransition(ctx, item, plan.ToStatus, plan.ActivityType, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if plan.ToStatus == "completed" {
		_ = s.repo.RunClientWorkflowAutomationForWorkItem(ctx, cleanID(updated["id"]), actorID, actorName)
	}
	s.publishWorkItemChanged(updated, actorID)
	return updated, nil
}

func (s *Service) publishWorkItemChanged(item Record, actorID string) {
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "work_item_changed",
		ProjectID:  cleanID(item["project_id"]),
		WorkItemID: cleanID(item["id"]),
		ActorID:    actorID,
	})
}

func (s *Service) requireActivePipelineWorkItem(ctx context.Context, item Record) error {
	if int(number(item["pipeline_engine_version"])) < 2 || cleanID(item["pipeline_run_id"]) == "" {
		return nil
	}
	rows, err := s.repo.Query(ctx, `
		SELECT pr.current_node_id, rn.status AS node_status
		FROM pipeline_runs pr
		LEFT JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id
		WHERE pr.id=$1
	`, item["pipeline_run_id"])
	if err != nil {
		return err
	}
	currentNodeID := ""
	if len(rows) > 0 {
		currentNodeID = cleanID(rows[0]["current_node_id"])
	}
	if currentNodeID == "" || currentNodeID != cleanID(item["run_node_id"]) {
		return validationError("Đầu việc chưa thuộc node đang thực hiện của quy trình", Record{
			"work_item_id":    item["id"],
			"run_node_id":     item["run_node_id"],
			"current_node_id": currentNodeID,
		})
	}
	if !statusIn(fmt.Sprint(rows[0]["node_status"]), "in_progress", "waiting_approval", "blocked") {
		return validationError("Node của đầu việc chưa sẵn sàng thao tác", Record{"node_status": rows[0]["node_status"]})
	}
	return nil
}

type workItemTransitionPlan struct {
	ToStatus         string
	ActivityType     string
	RequiresEvidence bool
	RequiresReason   bool
	CreatesApproval  bool
	Decision         bool
}

func workItemTransitionForAction(action string, item Record) (workItemTransitionPlan, error) {
	fromStatus := fmt.Sprint(item["status"])
	switch action {
	case "start":
		if !statusIn(fromStatus, "pending", "ready", "needs_changes") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép bắt đầu", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "in_progress", ActivityType: "started"}, nil
	case "complete":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép hoàn tất", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "completed", RequiresEvidence: true}, nil
	case "block":
		if !statusIn(fromStatus, "pending", "ready", "in_progress", "waiting_evidence", "waiting_review", "waiting_approval", "needs_changes") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép báo điểm chặn", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "blocked", ActivityType: "blocked", RequiresReason: true}, nil
	case "resolve-blocker":
		if fromStatus != "blocked" {
			return workItemTransitionPlan{}, validationError("Chỉ đầu việc đang bị chặn mới có thể gỡ điểm chặn", Record{"status": fromStatus})
		}
		toStatus := "ready"
		if cleanID(item["started_at"]) != "" {
			toStatus = "in_progress"
		}
		return workItemTransitionPlan{ToStatus: toStatus, ActivityType: "blocker_resolved"}, nil
	case "request-review":
		if !statusIn(fromStatus, "ready", "in_progress", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép gửi rà soát", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_review", ActivityType: "review_requested", RequiresEvidence: true}, nil
	case "request-approval":
		if !statusIn(fromStatus, "ready", "in_progress", "waiting_review", "needs_changes", "waiting_evidence") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép gửi duyệt", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "waiting_approval", ActivityType: "approval_requested", RequiresEvidence: true, CreatesApproval: true}, nil
	case "approve":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép duyệt", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "completed", ActivityType: "approved", RequiresEvidence: true, Decision: true}, nil
	case "reject":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép từ chối", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "rejected", RequiresReason: true, Decision: true}, nil
	case "request-change":
		if !statusIn(fromStatus, "waiting_review", "waiting_approval") {
			return workItemTransitionPlan{}, validationError("Trạng thái đầu việc không cho phép yêu cầu chỉnh sửa", Record{"status": fromStatus})
		}
		return workItemTransitionPlan{ToStatus: "needs_changes", ActivityType: "changes_requested", RequiresReason: true, Decision: true}, nil
	default:
		return workItemTransitionPlan{}, validationError("Hành động đầu việc không hợp lệ", Record{"action": action})
	}
}

func (s *Service) AddWorkItemEvidence(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm minh chứng cho đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	normalized, err := validateWorkItemEvidenceRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.StorageKey != "" {
		return nil, validationError("Storage minh chứng nội bộ chỉ được tạo qua endpoint tải tệp")
	}
	return s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
}

func (s *Service) AddWorkItemEvidenceUpload(ctx context.Context, workItemID string, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm minh chứng cho đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	storedPath, normalized, err := s.storeEvidenceUpload(item, req, fileHeader)
	if err != nil {
		return nil, err
	}
	evidence, err := s.repo.AddWorkItemEvidence(ctx, item, normalized, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return evidence, nil
}

type EvidenceFileResult struct {
	Path     string
	MIMEType string
	Filename string
}

func (s *Service) EvidenceFile(ctx context.Context, evidenceID, actorID string) (EvidenceFileResult, error) {
	evidenceID = cleanID(evidenceID)
	if !validActivityUUID(evidenceID) {
		return EvidenceFileResult{}, validationError("Mã minh chứng không hợp lệ", Record{"evidence_id": evidenceID})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	allowed, err := s.repo.CanViewWorkItem(ctx, cleanID(evidence["work_item_id"]), actorID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem tệp minh chứng này")
	}
	if fmt.Sprint(evidence["storage_provider"]) != "local" || cleanID(evidence["storage_key"]) == "" {
		return EvidenceFileResult{}, validationError("Minh chứng này chưa có tệp nội bộ được bảo vệ", Record{"evidence_id": evidenceID})
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	filePath, err := safeStoragePath(root, fmt.Sprint(evidence["storage_key"]))
	if err != nil {
		return EvidenceFileResult{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EvidenceFileResult{}, validationError("Tệp minh chứng không còn tồn tại trong kho lưu trữ", Record{"evidence_id": evidenceID})
		}
		return EvidenceFileResult{}, err
	}
	if info.IsDir() {
		return EvidenceFileResult{}, validationError("Đường dẫn minh chứng không hợp lệ", Record{"evidence_id": evidenceID})
	}
	mimeType := strings.TrimSpace(fmt.Sprint(evidence["mime_type"]))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{
		Path:     filePath,
		MIMEType: mimeType,
		Filename: evidenceDownloadName(evidence),
	}, nil
}

func (s *Service) storeEvidenceUpload(item Record, req WorkItemEvidenceRequest, fileHeader *multipart.FileHeader) (string, WorkItemEvidenceRequest, error) {
	if fileHeader == nil {
		return "", req, validationError("Tệp minh chứng là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return "", req, validationError("Tệp minh chứng không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return "", req, validationError("Tệp minh chứng vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return "", req, err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return "", req, readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	spec, ok := allowedEvidenceUploadMIMEs[detectedMIME]
	if !ok {
		return "", req, validationError("Định dạng minh chứng không được hỗ trợ", Record{"mime_type": detectedMIME})
	}
	requestedType := strings.TrimSpace(req.EvidenceType)
	if requestedType != "" && requestedType != spec.EvidenceType {
		return "", req, validationError("Loại minh chứng không khớp với định dạng tệp", Record{"evidence_type": requestedType, "mime_type": detectedMIME})
	}
	if requestedType == "" {
		req.EvidenceType = spec.EvidenceType
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = originalName
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = req.Name
	}
	req.MIMEType = detectedMIME
	req.FileSize = fileHeader.Size
	req.StorageProvider = "local"
	req.StorageKey = path.Join("evidence", cleanID(item["project_id"]), cleanID(item["id"]), uuid.NewString()+spec.Extension)
	if req.Payload == nil {
		req.Payload = Record{}
	}
	req.Payload["original_filename"] = originalName
	req.Payload["detected_mime_type"] = detectedMIME

	normalized, err := validateWorkItemEvidenceRequest(req)
	if err != nil {
		return "", req, err
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return "", req, err
	}
	fullPath, err := safeStoragePath(root, normalized.StorageKey)
	if err != nil {
		return "", normalized, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", normalized, err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return "", normalized, err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return "", normalized, err
	}
	if written != fileHeader.Size {
		return "", normalized, validationError("Kích thước tệp minh chứng không khớp dữ liệu tải lên", Record{"expected": fileHeader.Size, "written": written})
	}
	normalized.ChecksumSHA256 = hex.EncodeToString(hasher.Sum(nil))
	cleanup = false
	return fullPath, normalized, nil
}

func (s *Service) storeInputInvoicePDF(projectID, payableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, string, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return StoredFileMetadata{}, "", "", readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, "", "", validationError("Hoá đơn đầu vào chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "minh-chung" || originalName == "evidence-file" {
		originalName = "input-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("input-invoices", cleanID(projectID), cleanID(payableID), uuid.NewString()+".pdf")
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if written != fileHeader.Size {
		return StoredFileMetadata{}, "", "", validationError("Kích thước file hoá đơn không khớp dữ liệu upload", Record{"expected": fileHeader.Size, "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        "application/pdf",
		FileSize:        fileHeader.Size,
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, originalName, nil
}

func (s *Service) storeOutputInvoicePDF(projectID, receivableID string, fileHeader *multipart.FileHeader) (StoredFileMetadata, string, string, error) {
	if fileHeader == nil {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn là bắt buộc")
	}
	if fileHeader.Size <= 0 {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn không được rỗng")
	}
	if fileHeader.Size > s.maxUploadBytes {
		return StoredFileMetadata{}, "", "", validationError("File PDF hoá đơn vượt quá dung lượng cho phép", Record{"max_bytes": s.maxUploadBytes})
	}
	source, err := fileHeader.Open()
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	defer source.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return StoredFileMetadata{}, "", "", readErr
	}
	detectedMIME := http.DetectContentType(header[:n])
	if detectedMIME != "application/pdf" {
		return StoredFileMetadata{}, "", "", validationError("Hoá đơn đầu ra chỉ hỗ trợ PDF", Record{"mime_type": detectedMIME})
	}
	originalName := cleanUploadFilename(fileHeader.Filename)
	if originalName == "minh-chung" || originalName == "evidence-file" {
		originalName = "output-invoice.pdf"
	}
	if filepath.Ext(originalName) == "" {
		originalName += ".pdf"
	}
	storageKey := path.Join("output-invoices", cleanID(projectID), cleanID(receivableID), uuid.NewString()+".pdf")
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()

	hasher := sha256.New()
	reader := io.MultiReader(bytes.NewReader(header[:n]), source)
	written, err := io.Copy(io.MultiWriter(destination, hasher), reader)
	if err != nil {
		return StoredFileMetadata{}, "", "", err
	}
	if written != fileHeader.Size {
		return StoredFileMetadata{}, "", "", validationError("Kích thước file hoá đơn không khớp dữ liệu upload", Record{"expected": fileHeader.Size, "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        "application/pdf",
		FileSize:        fileHeader.Size,
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, originalName, nil
}

func (s *Service) storeGeneratedFile(storageKey string, content []byte, mimeType string) (StoredFileMetadata, string, error) {
	if len(content) == 0 {
		return StoredFileMetadata{}, "", validationError("File generated không được rỗng")
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	fullPath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return StoredFileMetadata{}, "", err
	}
	destination, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = os.Remove(fullPath)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(destination, hasher), bytes.NewReader(content))
	if err != nil {
		return StoredFileMetadata{}, "", err
	}
	if written != int64(len(content)) {
		return StoredFileMetadata{}, "", validationError("Kích thước generated file không khớp dữ liệu ghi", Record{"expected": len(content), "written": written})
	}
	cleanup = false
	return StoredFileMetadata{
		StorageProvider: "local",
		StorageKey:      storageKey,
		MIMEType:        mimeType,
		FileSize:        int64(len(content)),
		ChecksumSHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}, fullPath, nil
}

func (s *Service) storedFileResult(record Record, storageKeyField, mimeField, filename string) (EvidenceFileResult, error) {
	storageKey := strings.TrimSpace(fmt.Sprint(record[storageKeyField]))
	if storageKey == "" || storageKey == "<nil>" {
		return EvidenceFileResult{}, validationError("Tệp được bảo vệ chưa có khóa lưu trữ")
	}
	root, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	filePath, err := safeStoragePath(root, storageKey)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EvidenceFileResult{}, validationError("Tệp được bảo vệ không còn tồn tại trong kho lưu trữ")
		}
		return EvidenceFileResult{}, err
	}
	if info.IsDir() {
		return EvidenceFileResult{}, validationError("Đường dẫn tệp được bảo vệ không hợp lệ")
	}
	mimeType := strings.TrimSpace(fmt.Sprint(record[mimeField]))
	if mimeType == "" || mimeType == "<nil>" {
		mimeType = "application/octet-stream"
	}
	return EvidenceFileResult{Path: filePath, MIMEType: mimeType, Filename: cleanUploadFilename(filename)}, nil
}

func parseSignatureImageDataURL(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) {
		return nil, "", validationError("Chữ ký tay cần là PNG data URL")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, "", validationError("Dữ liệu ảnh chữ ký không hợp lệ")
	}
	if len(decoded) < 16 {
		return nil, "", validationError("Ảnh chữ ký quá nhỏ hoặc rỗng")
	}
	if len(decoded) > 1024*1024 {
		return nil, "", validationError("Ảnh chữ ký vượt quá dung lượng cho phép", Record{"max_bytes": 1024 * 1024})
	}
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if !bytes.HasPrefix(decoded, pngHeader) {
		return nil, "", validationError("Ảnh chữ ký phải là PNG hợp lệ")
	}
	return decoded, "image/png", nil
}

func validateSignatureStrokes(strokes []WorkflowFormSignatureStroke) ([]WorkflowFormSignatureStroke, error) {
	if len(strokes) == 0 {
		return nil, validationError("Cần nét ký tay trước khi ký biểu mẫu quy trình")
	}
	if len(strokes) > 80 {
		return nil, validationError("Chữ ký có quá nhiều nét", Record{"max_strokes": 80})
	}
	totalPoints := 0
	out := make([]WorkflowFormSignatureStroke, 0, len(strokes))
	for _, stroke := range strokes {
		points := make([]WorkflowFormSignaturePoint, 0, len(stroke.Points))
		for _, point := range stroke.Points {
			if math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0) {
				return nil, validationError("Tọa độ chữ ký không hợp lệ")
			}
			if point.X < 0 || point.X > 1 || point.Y < 0 || point.Y > 1 {
				return nil, validationError("Tọa độ chữ ký nằm ngoài canvas")
			}
			points = append(points, WorkflowFormSignaturePoint{X: point.X, Y: point.Y})
		}
		if len(points) >= 2 {
			totalPoints += len(points)
			out = append(out, WorkflowFormSignatureStroke{Points: points})
		}
	}
	if totalPoints < 2 {
		return nil, validationError("Cần nét ký tay trước khi ký biểu mẫu quy trình")
	}
	if totalPoints > 2000 {
		return nil, validationError("Chữ ký có quá nhiều điểm", Record{"max_points": 2000})
	}
	return out, nil
}

func workflowFormPDFFileName(form Record, revisionNo int) string {
	instance, _ := form["instance"].(Record)
	title := cleanUploadFilename(fmt.Sprintf("%s-rev-%d.pdf", slugText(fmt.Sprint(instance["title"])), revisionNo))
	if title == "minh-chung" || title == "evidence-file" || title == ".pdf" {
		return fmt.Sprintf("workflow-form-rev-%d.pdf", revisionNo)
	}
	return title
}

func signatureVisualFilename(signature Record) string {
	name := slugText(fmt.Sprintf("%s-%s", signature["signer_name"], signature["purpose"]))
	if name == "" {
		name = "workflow-signature"
	}
	return name + ".png"
}

func slugText(value string) string {
	value = strings.ToLower(asciiFold(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		allowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if allowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func (s *Service) AddWorkItemActivity(ctx context.Context, workItemID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm hoạt động cho đầu việc này")
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ParentActivityID != "" {
		return nil, validationError("Phản hồi trực tiếp chỉ được tạo từ thảo luận công trình")
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	normalized, err = s.validateActivityAttachments(ctx, cleanID(item["project_id"]), workItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddWorkItemActivity(ctx, item, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

func (s *Service) MarkActivityRead(ctx context.Context, activityID, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	readState, err := s.repo.MarkActivityRead(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "activity_read",
		ProjectID:  cleanID(readState["project_id"]),
		ActivityID: activityID,
		ActorID:    actorID,
	})
	return readState, nil
}

func (s *Service) MarkProjectActivityRead(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoạt động của công trình này")
	}
	result, err := s.repo.MarkProjectActivityRead(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{Type: "activity_read", ProjectID: projectID, ActorID: actorID})
	return result, nil
}

func (s *Service) ResolveActivity(ctx context.Context, activityID string, req ActivityResolutionRequest, actorID, actorName string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(activity["project_id"])
	workItemID := cleanID(activity["work_item_id"])
	if workItemID != "" {
		allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xử lý hoạt động của đầu việc này")
		}
	} else {
		allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xử lý hoạt động của công trình này")
		}
		if !profile.IsManager && cleanID(activity["actor_id"]) != actorID {
			mentioned, err := s.repo.ActivityMentionedUser(ctx, activityID, actorID)
			if err != nil {
				return nil, err
			}
			if !mentioned {
				return nil, forbiddenError("Chỉ người tạo, người được nhắc hoặc quản lý công trình có thể xử lý hoạt động này")
			}
		}
	}
	activityType := strings.TrimSpace(fmt.Sprint(activity["activity_type"]))
	if activityType != "question" && activityType != "blocker" && activityType != "reminder" {
		return nil, validationError("Loại hoạt động này không có hành động hoàn tất", Record{"activity_type": activityType})
	}
	comment := strings.TrimSpace(req.Comment)
	resolved, err := s.repo.ResolveActivity(ctx, activity, comment, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", resolved, actorID))
	return resolved, nil
}

func (s *Service) SetActivityReaction(ctx context.Context, activityID string, req ActivityReactionRequest, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	reaction := strings.TrimSpace(req.Reaction)
	if !activityReactions[reaction] {
		return nil, validationError("Phản ứng không hợp lệ", Record{"reaction": reaction})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if !userActivityTypes[strings.TrimSpace(fmt.Sprint(activity["activity_type"]))] {
		return nil, validationError("Không thể phản ứng với sự kiện hệ thống")
	}
	if cleanID(activity["withdrawn_at"]) != "" {
		return nil, validationError("Không thể phản ứng với tin nhắn đã được thu hồi")
	}
	result, err := s.repo.SetActivityReaction(ctx, activity, reaction, req.Active, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "activity_changed",
		ProjectID:  cleanID(activity["project_id"]),
		ActivityID: activityID,
		ActorID:    actorID,
	})
	return result, nil
}

func (s *Service) UpdateActivityMessage(ctx context.Context, activityID string, req ActivityMessageUpdateRequest, actorID, actorName string) (Record, error) {
	activity, err := s.activityMessageForMutation(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return nil, validationError("Nội dung tin nhắn là bắt buộc")
	}
	if comment == strings.TrimSpace(fmt.Sprint(activity["comment"])) {
		return nil, validationError("Nội dung tin nhắn chưa thay đổi")
	}
	updated, err := s.repo.UpdateActivityMessage(ctx, activity, comment, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", updated, actorID))
	return updated, nil
}

func (s *Service) WithdrawActivityMessage(ctx context.Context, activityID, actorID, actorName string) (Record, error) {
	activity, err := s.activityMessageForMutation(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	withdrawn, err := s.repo.WithdrawActivityMessage(ctx, activity, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", withdrawn, actorID))
	return withdrawn, nil
}

func (s *Service) activityMessageForMutation(ctx context.Context, activityID, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if cleanID(activity["actor_id"]) != cleanID(actorID) {
		return nil, forbiddenError("Chỉ tác giả được chỉnh sửa hoặc thu hồi tin nhắn")
	}
	if strings.TrimSpace(fmt.Sprint(activity["activity_type"])) != "comment" {
		return nil, validationError("Chỉ tin nhắn ghi chú mới có thể chỉnh sửa hoặc thu hồi")
	}
	if cleanID(activity["withdrawn_at"]) != "" {
		return nil, validationError("Tin nhắn đã được thu hồi")
	}
	createdAt, err := time.Parse(time.RFC3339, cleanID(activity["created_at"]))
	if err != nil {
		return nil, err
	}
	if time.Since(createdAt) > activityMessageEditWindow {
		return nil, validationError("Thời hạn chỉnh sửa hoặc thu hồi tin nhắn đã kết thúc")
	}
	return activity, nil
}

func (s *Service) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm hoạt động cho công trình này")
	}
	if strings.TrimSpace(req.ParentActivityID) != "" {
		req.ActivityType = "comment"
		req.Visibility = "project"
		req.Attachments = nil
		req.Metadata = Record{}
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	normalized, err = s.normalizeActivityReply(ctx, projectID, normalized, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	normalized, err = s.normalizeProjectActivityTarget(ctx, projectID, normalized, actorID)
	if err != nil {
		return nil, err
	}
	attachmentWorkItemID := ""
	if normalized.ScopeType == "work_item" {
		attachmentWorkItemID = normalized.ScopeID
	} else if normalized.WorkItemID != "" {
		attachmentWorkItemID = normalized.WorkItemID
	}
	normalized, err = s.validateActivityAttachments(ctx, projectID, attachmentWorkItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddProjectActivity(ctx, projectID, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

func (s *Service) normalizeActivityReply(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID string) (WorkItemActivityRequest, error) {
	parentActivityID := cleanID(req.ParentActivityID)
	if parentActivityID == "" {
		req.ParentActivityID = ""
		return req, nil
	}
	if !validActivityUUID(parentActivityID) {
		return req, validationError("Mã trao đổi được trả lời không hợp lệ", Record{"parent_activity_id": parentActivityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return req, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, parentActivityID, profile)
	if err != nil {
		return req, err
	}
	if !visible {
		return req, forbiddenError("Bạn không có quyền trả lời trao đổi này")
	}
	parent, err := s.repo.ActivityRecord(ctx, parentActivityID)
	if err != nil {
		return req, err
	}
	if cleanID(parent["project_id"]) != cleanID(projectID) {
		return req, validationError("Trao đổi được trả lời không thuộc công trình này")
	}
	if !userActivityTypes[strings.TrimSpace(fmt.Sprint(parent["activity_type"]))] {
		return req, validationError("Không thể trả lời sự kiện hệ thống")
	}
	if cleanID(parent["withdrawn_at"]) != "" {
		return req, validationError("Không thể trả lời tin nhắn đã được thu hồi")
	}
	req.ParentActivityID = parentActivityID
	req.ActivityType = "comment"
	req.Visibility = strings.TrimSpace(fmt.Sprint(parent["visibility"]))
	req.ScopeType = strings.TrimSpace(fmt.Sprint(parent["scope_type"]))
	req.ScopeID = cleanID(parent["scope_id"])
	req.WorkItemID = cleanID(parent["work_item_id"])
	req.Attachments = nil
	req.Metadata = Record{}
	parentActorID := cleanID(parent["actor_id"])
	if parentActorID != "" && parentActorID != actorID {
		req.Mentions = append(req.Mentions, ActivityMentionRequest{Type: "user", ID: parentActorID})
	}
	mentions, err := normalizeActivityMentions(req.Mentions)
	if err != nil {
		return req, err
	}
	req.Mentions = mentions
	if len(mentions) > 0 {
		req.Metadata["mentions"] = mentions
	}
	return req, nil
}

var userActivityTypes = map[string]bool{
	"comment":   true,
	"status":    true,
	"blocker":   true,
	"question":  true,
	"decision":  true,
	"reminder":  true,
	"file_note": true,
}

var activityFilterTypes = map[string]bool{
	"created":            true,
	"started":            true,
	"completed":          true,
	"blocked":            true,
	"blocker_resolved":   true,
	"status_changed":     true,
	"comment":            true,
	"status":             true,
	"blocker":            true,
	"question":           true,
	"decision":           true,
	"reminder":           true,
	"file_note":          true,
	"system_event":       true,
	"evidence_added":     true,
	"evidence_accepted":  true,
	"evidence_rejected":  true,
	"review_requested":   true,
	"approval_requested": true,
	"approved":           true,
	"rejected":           true,
	"changes_requested":  true,
}

var activityVisibilities = map[string]bool{
	"internal": true,
	"project":  true,
	"client":   true,
}

var activityTargetScopes = map[string]bool{
	"project":             true,
	"work_item":           true,
	"boq_item":            true,
	"procurement_request": true,
	"stock_movement":      true,
	"acceptance":          true,
	"rework":              true,
	"handover":            true,
	"payable":             true,
	"receivable":          true,
	"payment":             true,
}

var activityAttentionQueues = map[string]bool{
	"all":       true,
	"mentions":  true,
	"questions": true,
	"blockers":  true,
	"reminders": true,
	"unread":    true,
}

var activityAttachmentTypes = map[string]bool{
	"evidence": true,
	"document": true,
}

var activityReactions = map[string]bool{
	"👍": true,
	"👀": true,
	"🙏": true,
	"🎉": true,
}

func validateActivityRequest(req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	activityType := strings.TrimSpace(req.ActivityType)
	if activityType == "" {
		activityType = "comment"
	}
	if activityType == "system_event" {
		return req, validationError("Sự kiện hệ thống chỉ được tạo bởi quy trình")
	}
	if !userActivityTypes[activityType] {
		return req, validationError("Loại hoạt động không hợp lệ", Record{"activity_type": activityType})
	}

	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return req, validationError("Nội dung hoạt động là bắt buộc")
	}

	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "project"
	}
	if !activityVisibilities[visibility] {
		return req, validationError("Phạm vi hiển thị hoạt động không hợp lệ", Record{"visibility": visibility})
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = Record{}
	}

	if activityType == "reminder" {
		dueAt := activityMetadataText(metadata, "due_at")
		if dueAt == "" {
			return req, validationError("Nhắc việc cần ngày nhắc lại", Record{"field": "metadata.due_at"})
		}
		if !validActivityDueDate(dueAt) {
			return req, validationError("Ngày nhắc lại không hợp lệ", Record{"field": "metadata.due_at"})
		}
		metadata["due_at"] = dueAt
	}
	if activityType == "blocker" {
		if dueAt := activityMetadataText(metadata, "due_at"); dueAt != "" && !validActivityDueDate(dueAt) {
			return req, validationError("Hạn xử lý điểm chặn không hợp lệ", Record{"field": "metadata.due_at"})
		}
	}
	req.ActivityType = activityType
	req.Comment = comment
	req.Visibility = visibility
	req.ScopeType = strings.TrimSpace(req.ScopeType)
	req.ScopeID = cleanID(req.ScopeID)
	req.WorkItemID = cleanID(req.WorkItemID)
	req.ParentActivityID = cleanID(req.ParentActivityID)
	attachments, err := normalizeActivityAttachments(req.Attachments, metadata)
	if err != nil {
		return req, err
	}
	req.Attachments = attachments
	if len(attachments) > 0 {
		metadata["attachments"] = attachments
		for _, attachment := range attachments {
			if attachment.Type == "evidence" && activityMetadataText(metadata, "evidence_id") == "" {
				metadata["evidence_id"] = attachment.ID
			}
			if attachment.Type == "document" && activityMetadataText(metadata, "document_id") == "" {
				metadata["document_id"] = attachment.ID
			}
		}
	}
	if activityType == "file_note" && len(attachments) == 0 {
		return req, validationError("Ghi chú tệp cần minh chứng hoặc tài liệu", Record{"fields": []string{"attachments", "metadata.evidence_id", "metadata.document_id"}})
	}
	mentions, err := normalizeActivityMentions(req.Mentions)
	if err != nil {
		return req, err
	}
	req.Mentions = mentions
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}
	req.Metadata = metadata
	return req, nil
}

func normalizeActivityAttachments(input []ActivityAttachmentRequest, metadata Record) ([]ActivityAttachmentRequest, error) {
	out := make([]ActivityAttachmentRequest, 0, len(input))
	seen := map[string]bool{}
	add := func(attachmentType, attachmentID, label string) error {
		attachmentType = strings.TrimSpace(strings.ToLower(attachmentType))
		attachmentID = cleanID(attachmentID)
		label = strings.TrimSpace(label)
		if attachmentType == "" && attachmentID == "" {
			return nil
		}
		if !activityAttachmentTypes[attachmentType] {
			return validationError("Loại tệp đính kèm không hợp lệ", Record{"attachment_type": attachmentType})
		}
		if attachmentID == "" || !validActivityUUID(attachmentID) {
			return validationError("ID tệp đính kèm không hợp lệ", Record{"attachment_type": attachmentType, "id": attachmentID})
		}
		key := attachmentType + ":" + strings.ToLower(attachmentID)
		if seen[key] {
			return nil
		}
		seen[key] = true
		out = append(out, ActivityAttachmentRequest{Type: attachmentType, ID: attachmentID, Label: label})
		return nil
	}
	for _, attachment := range input {
		if err := add(attachment.Type, attachment.ID, attachment.Label); err != nil {
			return nil, err
		}
	}
	if rawAttachments, ok := metadata["attachments"]; ok {
		for _, rawAttachment := range anySlice(rawAttachments) {
			if record, ok := rawAttachment.(map[string]any); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
				continue
			}
			if record, ok := rawAttachment.(Record); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
			}
		}
	}
	if evidenceID := activityMetadataText(metadata, "evidence_id"); evidenceID != "" {
		if err := add("evidence", evidenceID, ""); err != nil {
			return nil, err
		}
	}
	if documentID := activityMetadataText(metadata, "document_id"); documentID != "" {
		if err := add("document", documentID, ""); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []Record:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case []ActivityAttachmentRequest:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, Record{"type": item.Type, "id": item.ID, "label": item.Label})
		}
		return out
	default:
		return nil
	}
}

func (s *Service) validateActivityAttachments(ctx context.Context, projectID, workItemID string, req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	if len(req.Attachments) == 0 {
		return req, nil
	}
	for _, attachment := range req.Attachments {
		switch attachment.Type {
		case "evidence":
			evidence, err := s.repo.GetEvidenceDetail(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Minh chứng đính kèm không tồn tại", Record{"evidence_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(evidence["project_id"]) != projectID {
				return req, validationError("Minh chứng đính kèm không thuộc công trình này", Record{"evidence_id": attachment.ID})
			}
			if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
				return req, validationError("Minh chứng đính kèm không thuộc đầu việc này", Record{"evidence_id": attachment.ID})
			}
			if req.Visibility == "client" && fmt.Sprint(evidence["visibility"]) != "client" {
				return req, validationError("Hoạt động hiển thị cho chủ đầu tư chỉ được gắn minh chứng cùng phạm vi hiển thị", Record{"evidence_id": attachment.ID})
			}
		case "document":
			document, err := s.repo.GetDocumentAttachment(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Tài liệu đính kèm không tồn tại", Record{"document_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(document["project_id"]) != projectID {
				return req, validationError("Tài liệu đính kèm không thuộc công trình này", Record{"document_id": attachment.ID})
			}
			if req.Visibility == "client" {
				return req, validationError("Tài liệu đính kèm chưa có phạm vi hiển thị; hãy gắn minh chứng hiển thị cho chủ đầu tư thay thế", Record{"document_id": attachment.ID})
			}
		}
	}
	return req, nil
}

func normalizeActivityMentions(input []ActivityMentionRequest) ([]ActivityMentionRequest, error) {
	out := make([]ActivityMentionRequest, 0, len(input))
	seen := map[string]bool{}
	for _, mention := range input {
		mentionType := strings.TrimSpace(strings.ToLower(mention.Type))
		mentionID := cleanID(mention.ID)
		label := strings.TrimSpace(mention.Label)
		if mentionType == "" && mentionID == "" {
			continue
		}
		if mentionType != "user" && mentionType != "staff" && mentionType != "role" {
			return nil, validationError("Loại nhắc đến không hợp lệ", Record{"mention_type": mention.Type})
		}
		if mentionID == "" {
			return nil, validationError("Nhắc đến cần ID hoặc nhãn vai trò", Record{"mention_type": mentionType})
		}
		if mentionType != "role" && !validActivityUUID(mentionID) {
			return nil, validationError("Người dùng/nhân sự được nhắc đến không hợp lệ", Record{"mention_type": mentionType, "id": mentionID})
		}
		key := mentionType + ":" + strings.ToLower(mentionID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ActivityMentionRequest{Type: mentionType, ID: mentionID, Label: label})
	}
	return out, nil
}

func (s *Service) normalizeProjectActivityTarget(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID string) (WorkItemActivityRequest, error) {
	scopeType := strings.TrimSpace(req.ScopeType)
	if scopeType == "" {
		scopeType = "project"
	}
	if !activityTargetScopes[scopeType] {
		return req, validationError("Phạm vi hoạt động không hợp lệ", Record{"scope_type": scopeType})
	}
	if scopeType == "project" {
		req.ScopeType = "project"
		req.ScopeID = projectID
		req.WorkItemID = ""
		return req, nil
	}

	scopeID := cleanID(req.ScopeID)
	if scopeID == "" {
		return req, validationError("Phạm vi hoạt động cần bản ghi đích", Record{"scope_type": scopeType})
	}
	if !validActivityUUID(scopeID) {
		return req, validationError("scope_id không hợp lệ", Record{"scope_id": scopeID})
	}
	if scopeType == "work_item" {
		allowed, err := s.repo.CanActOnWorkItem(ctx, scopeID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền thêm hoạt động cho đầu việc này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, scopeID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Đầu việc không thuộc công trình này", Record{"work_item_id": scopeID})
		}
		req.ScopeType = "work_item"
		req.ScopeID = scopeID
		req.WorkItemID = scopeID
		return req, nil
	}

	exists, err := s.repo.ProjectActivityTargetExists(ctx, projectID, scopeType, scopeID)
	if err != nil {
		return req, err
	}
	if !exists {
		return req, validationError("Bản ghi hoạt động không thuộc công trình này", Record{"scope_type": scopeType, "scope_id": scopeID})
	}
	if req.WorkItemID != "" {
		if !validActivityUUID(req.WorkItemID) {
			return req, validationError("Mã đầu việc không hợp lệ", Record{"work_item_id": req.WorkItemID})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, req.WorkItemID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền liên kết hoạt động với đầu việc này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, req.WorkItemID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Đầu việc liên kết không thuộc công trình này", Record{"work_item_id": req.WorkItemID})
		}
	}
	req.ScopeType = scopeType
	req.ScopeID = scopeID
	return req, nil
}

func validActivityUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func activityMetadataText(metadata Record, keys ...string) string {
	for _, key := range keys {
		text := strings.TrimSpace(fmt.Sprint(metadata[key]))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func validActivityDueDate(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) requireActivityVisibility(ctx context.Context, req WorkItemActivityRequest, actorID string) error {
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return err
	}
	if profileIsClient(profile) && !profile.IsManager && req.Visibility != "client" {
		return validationError("Chủ đầu tư chỉ được tạo hoạt động hiển thị cho chủ đầu tư", Record{"visibility": req.Visibility})
	}
	return nil
}

func (s *Service) EvidenceAction(ctx context.Context, evidenceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "accept" && action != "reject" {
		return nil, validationError("Hành động minh chứng không hợp lệ", Record{"action": action})
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, fmt.Sprint(evidence["work_item_id"]))
	if err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanDecideWorkItem(ctx, item, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền rà soát minh chứng này")
	}
	return s.repo.EvidenceAction(ctx, evidence, item, action, req, actorID, actorName)
}

func validateWorkItemEvidenceRequest(req WorkItemEvidenceRequest) (WorkItemEvidenceRequest, error) {
	req.EvidenceType = strings.TrimSpace(req.EvidenceType)
	req.DocumentType = strings.TrimSpace(req.DocumentType)
	req.Name = strings.TrimSpace(req.Name)
	req.DocumentID = cleanID(req.DocumentID)
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.FileURL = strings.TrimSpace(req.FileURL)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.StorageProvider = strings.TrimSpace(req.StorageProvider)
	req.StorageKey = strings.TrimSpace(req.StorageKey)
	req.ChecksumSHA256 = strings.ToLower(strings.TrimSpace(req.ChecksumSHA256))
	req.Caption = strings.TrimSpace(req.Caption)
	req.RevisionNo = strings.TrimSpace(req.RevisionNo)
	req.Visibility = strings.TrimSpace(req.Visibility)
	if req.Visibility == "" {
		req.Visibility = "internal"
	}
	if req.StorageProvider == "" {
		req.StorageProvider = "external"
	}
	if req.Payload == nil {
		req.Payload = Record{}
	}
	validType := map[string]bool{"document": true, "photo": true, "checklist": true, "link": true, "note": true, "other": true}
	if !validType[req.EvidenceType] {
		return req, validationError("Loại minh chứng không hợp lệ", Record{"evidence_type": req.EvidenceType})
	}
	validVisibility := map[string]bool{"internal": true, "project": true, "client": true}
	if !validVisibility[req.Visibility] {
		return req, validationError("Phạm vi hiển thị minh chứng không hợp lệ", Record{"visibility": req.Visibility})
	}
	validStorageProvider := map[string]bool{"external": true, "local": true}
	if !validStorageProvider[req.StorageProvider] {
		return req, validationError("Nơi lưu minh chứng không hợp lệ", Record{"storage_provider": req.StorageProvider})
	}
	if req.StorageKey != "" && req.StorageProvider != "local" {
		return req, validationError("Khóa lưu trữ chỉ hợp lệ với kho lưu trữ nội bộ", Record{"storage_provider": req.StorageProvider})
	}
	if req.ChecksumSHA256 != "" && !validSHA256Hex(req.ChecksumSHA256) {
		return req, validationError("Checksum minh chứng không hợp lệ", Record{"checksum_sha256": req.ChecksumSHA256})
	}
	if req.Name == "" {
		req.Name = req.Title
	}
	if req.Name == "" && req.Caption != "" {
		req.Name = req.Caption
	}
	if req.Name == "" {
		return req, validationError("Tên minh chứng là bắt buộc")
	}
	if req.FileSize < 0 {
		return req, validationError("Kích thước file không hợp lệ")
	}
	if statusIn(req.EvidenceType, "document", "photo", "link") && req.FileURL == "" && req.DocumentID == "" && req.StorageKey == "" {
		return req, validationError("Minh chứng dạng tài liệu/ảnh/liên kết cần URL tệp, tài liệu liên kết hoặc tệp tải lên", Record{"evidence_type": req.EvidenceType})
	}
	return req, nil
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func safeStoragePath(root, storageKey string) (string, error) {
	if strings.TrimSpace(storageKey) == "" {
		return "", validationError("Khóa lưu trữ minh chứng là bắt buộc")
	}
	if filepath.IsAbs(storageKey) {
		return "", validationError("Khóa lưu trữ minh chứng không hợp lệ")
	}
	cleanKey := filepath.Clean(filepath.FromSlash(storageKey))
	if cleanKey == "." || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) {
		return "", validationError("Khóa lưu trữ minh chứng không hợp lệ")
	}
	fullPath := filepath.Join(root, cleanKey)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absPath != root && !strings.HasPrefix(absPath, root+string(os.PathSeparator)) {
		return "", validationError("Khóa lưu trữ minh chứng vượt khỏi thư mục lưu trữ")
	}
	return absPath, nil
}

func cleanUploadFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == "<nil>" || name == string(filepath.Separator) {
		return "minh-chung"
	}
	return strings.Trim(name, " \t\r\n")
}

func evidenceDownloadName(evidence Record) string {
	name := cleanUploadFilename(fmt.Sprint(evidence["name"]))
	storageKey := fmt.Sprint(evidence["storage_key"])
	extension := filepath.Ext(storageKey)
	if filepath.Ext(name) == "" && extension != "" {
		name += extension
	}
	return name
}

func (s *Service) validateWorkflowFormAnswers(ctx context.Context, instance Record, fields []Record, answers Record, submit bool) (Record, error) {
	if answers == nil {
		answers = Record{}
	}
	fieldByKey := map[string]Record{}
	for _, field := range fields {
		fieldByKey[fmt.Sprint(field["field_key"])] = field
	}
	for key := range answers {
		if _, ok := fieldByKey[key]; !ok {
			return nil, validationError("Biểu mẫu quy trình có trường không thuộc schema", Record{"field": key})
		}
	}
	normalized := Record{}
	missing := []string{}
	fieldErrors := Record{}
	for _, field := range fields {
		key := fmt.Sprint(field["field_key"])
		value, hasValue := answers[key]
		required := boolValue(field["required"])
		if submit && required && workflowFormValueEmpty(field, value) {
			missing = append(missing, key)
			fieldErrors[key] = fmt.Sprintf("%s là bắt buộc", field["label"])
			continue
		}
		if !hasValue || workflowFormValueEmpty(field, value) {
			continue
		}
		normalizedValue, err := s.normalizeWorkflowFormValue(ctx, instance, field, value)
		if err != nil {
			fieldErrors[key] = err.Error()
			continue
		}
		normalized[key] = normalizedValue
	}
	if len(fieldErrors) > 0 {
		return nil, validationError("Biểu mẫu quy trình có dữ liệu không hợp lệ", Record{"fields": fieldErrors, "missing": missing})
	}
	if submit && fmt.Sprint(instance["form_type"]) == "checklist" {
		if err := validateChecklistSubmitReady(normalized); err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

func workflowFormValueEmpty(field Record, value any) bool {
	fieldType := fmt.Sprint(field["field_type"])
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case []Record:
		return len(v) == 0
	case bool:
		return fieldType == "checkbox" && !v
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text == "" || text == "<nil>"
	}
}

func (s *Service) normalizeWorkflowFormValue(ctx context.Context, instance Record, field Record, value any) (any, error) {
	fieldType := fmt.Sprint(field["field_type"])
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if err := validateWorkflowTextLength(field, text); err != nil {
			return nil, err
		}
		return text, nil
	case "number":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, err
		}
		return numeric, validateWorkflowNumberRange(field, numeric)
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, errors.New("Ngày không hợp lệ")
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, errors.New("Lựa chọn không hợp lệ")
		}
		if !workflowOptionAllowed(field, text) {
			return nil, errors.New("Lựa chọn không nằm trong schema")
		}
		return text, nil
	case "multi_select":
		values, err := workflowStringList(value)
		if err != nil {
			return nil, err
		}
		for _, selected := range values {
			if !workflowOptionAllowed(field, selected) {
				return nil, errors.New("Lựa chọn không nằm trong schema")
			}
		}
		return values, nil
	case "checkbox":
		checked, ok := value.(bool)
		if !ok {
			return nil, errors.New("Giá trị checkbox phải là boolean")
		}
		return checked, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, errors.New("Minh chứng không hợp lệ")
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("Minh chứng không tồn tại")
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, errors.New("Minh chứng không thuộc công trình này")
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, errors.New("Minh chứng không thuộc đầu việc này")
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, errors.New("Minh chứng đã bị từ chối")
		}
		return evidenceID, nil
	case "structured_list":
		return s.normalizeWorkflowStructuredList(ctx, instance, field, value)
	default:
		return nil, errors.New("Loại trường không được hỗ trợ")
	}
}

func validateChecklistSubmitReady(answers Record) error {
	rawItems, ok := answers["checklist_items"]
	if !ok {
		return nil
	}
	items, err := workflowRecordList(rawItems)
	if err != nil {
		return err
	}
	blocking := []Record{}
	for index, item := range items {
		status := strings.TrimSpace(fmt.Sprint(item["status"]))
		if !statusIn(status, "done", "not_applicable") {
			blocking = append(blocking, Record{
				"row":    index + 1,
				"title":  item["title"],
				"status": status,
			})
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return validationError("Checklist còn hạng mục chưa đạt, chỉ được gửi khi tất cả dòng là Hoàn tất hoặc Không áp dụng", Record{
		"fields":         Record{"checklist_items": "Tất cả dòng checklist phải là Hoàn tất hoặc Không áp dụng trước khi gửi"},
		"blocking_items": blocking,
	})
}

func (s *Service) normalizeWorkflowStructuredList(ctx context.Context, instance Record, field Record, value any) ([]Record, error) {
	items, err := workflowRecordList(value)
	if err != nil {
		return nil, err
	}
	validation, _ := field["validation_json"].(map[string]any)
	minItems := int(number(validation["min_items"]))
	maxItems := int(number(validation["max_items"]))
	if minItems > 0 && len(items) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng", field["label"], minItems)
	}
	if maxItems > 0 && len(items) > maxItems {
		return nil, fmt.Errorf("%s không được vượt quá %d dòng", field["label"], maxItems)
	}
	rawFields, _ := validation["item_fields"].([]any)
	if len(rawFields) == 0 {
		return items, nil
	}
	itemFields := make([]Record, 0, len(rawFields))
	fieldByKey := map[string]Record{}
	for _, rawField := range rawFields {
		spec, ok := rawField.(map[string]any)
		if !ok {
			continue
		}
		key := strings.TrimSpace(fmt.Sprint(spec["key"]))
		if key == "" || key == "<nil>" {
			continue
		}
		record := Record(spec)
		itemFields = append(itemFields, record)
		fieldByKey[key] = record
	}
	out := make([]Record, 0, len(items))
	for index, item := range items {
		for key := range item {
			if _, ok := fieldByKey[key]; !ok {
				return nil, fmt.Errorf("%s dòng %d có field không thuộc schema: %s", field["label"], index+1, key)
			}
		}
		normalized := Record{}
		for _, spec := range itemFields {
			key := fmt.Sprint(spec["key"])
			label := fmt.Sprint(spec["label"])
			if label == "" || label == "<nil>" {
				label = key
			}
			value, hasValue := item[key]
			required := boolValue(spec["required"])
			if required && structuredValueEmpty(spec, value) {
				return nil, fmt.Errorf("%s dòng %d: %s là bắt buộc", field["label"], index+1, label)
			}
			if !hasValue || structuredValueEmpty(spec, value) {
				continue
			}
			normalizedValue, err := s.normalizeStructuredFieldValue(ctx, instance, spec, value)
			if err != nil {
				return nil, fmt.Errorf("%s dòng %d: %s", field["label"], index+1, err.Error())
			}
			normalized[key] = normalizedValue
		}
		if len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	if minItems > 0 && len(out) < minItems {
		return nil, fmt.Errorf("%s cần ít nhất %d dòng hợp lệ", field["label"], minItems)
	}
	return out, nil
}

func workflowRecordList(value any) ([]Record, error) {
	rawItems, ok := value.([]any)
	if !ok {
		if records, ok := value.([]Record); ok {
			return records, nil
		}
		return nil, errors.New("Giá trị danh sách phải là array")
	}
	items := make([]Record, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("Mỗi dòng trong danh sách phải là object")
		}
		items = append(items, Record(item))
	}
	return items, nil
}

func structuredValueEmpty(field Record, value any) bool {
	if value == nil {
		return true
	}
	fieldType := fmt.Sprint(field["type"])
	if fieldType == "checkbox" {
		return value != true
	}
	if values, ok := value.([]any); ok {
		return len(values) == 0
	}
	return strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>"
}

func (s *Service) normalizeStructuredFieldValue(ctx context.Context, instance Record, field Record, value any) (any, error) {
	fieldType := fmt.Sprint(field["type"])
	label := fmt.Sprint(field["label"])
	if label == "" || label == "<nil>" {
		label = fmt.Sprint(field["key"])
	}
	switch fieldType {
	case "text", "textarea":
		text := strings.TrimSpace(fmt.Sprint(value))
		if min := int(number(field["min_length"])); min > 0 && len([]rune(text)) < min {
			return nil, fmt.Errorf("%s phải có ít nhất %d ký tự", label, min)
		}
		if max := int(number(field["max_length"])); max > 0 && len([]rune(text)) > max {
			return nil, fmt.Errorf("%s không được vượt quá %d ký tự", label, max)
		}
		return text, nil
	case "number":
		numeric, err := workflowNumberValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s phải là số", label)
		}
		if minValue, ok := field["min"]; ok && numeric < number(minValue) {
			return nil, fmt.Errorf("%s phải lớn hơn hoặc bằng %v", label, minValue)
		}
		if maxValue, ok := field["max"]; ok && numeric > number(maxValue) {
			return nil, fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", label, maxValue)
		}
		return numeric, nil
	case "date":
		text := strings.TrimSpace(fmt.Sprint(value))
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		return text, nil
	case "select":
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		if !structuredOptionAllowed(field, text) {
			return nil, fmt.Errorf("%s không nằm trong schema", label)
		}
		return text, nil
	case "evidence_reference":
		evidenceID := cleanID(value)
		if evidenceID == "" {
			return nil, fmt.Errorf("%s không hợp lệ", label)
		}
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("%s không tồn tại", label)
			}
			return nil, err
		}
		if cleanID(evidence["project_id"]) != cleanID(instance["project_id"]) {
			return nil, fmt.Errorf("%s không thuộc công trình này", label)
		}
		if workItemID := cleanID(instance["work_item_id"]); workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, fmt.Errorf("%s không thuộc đầu việc này", label)
		}
		if fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, fmt.Errorf("%s đã bị từ chối", label)
		}
		return evidenceID, nil
	default:
		return nil, fmt.Errorf("Loại trường %s không được hỗ trợ", fieldType)
	}
}

func structuredOptionAllowed(field Record, selected string) bool {
	rawOptions, _ := field["options"].([]any)
	if len(rawOptions) == 0 {
		return true
	}
	for _, raw := range rawOptions {
		switch option := raw.(type) {
		case string:
			if option == selected {
				return true
			}
		case map[string]any:
			if strings.TrimSpace(fmt.Sprint(option["value"])) == selected {
				return true
			}
		}
	}
	return false
}

func validateWorkflowTextLength(field Record, text string) error {
	validation, _ := field["validation_json"].(map[string]any)
	if min := int(number(validation["min_length"])); min > 0 && len([]rune(text)) < min {
		return fmt.Errorf("%s phải có ít nhất %d ký tự", field["label"], min)
	}
	if max := int(number(validation["max_length"])); max > 0 && len([]rune(text)) > max {
		return fmt.Errorf("%s không được vượt quá %d ký tự", field["label"], max)
	}
	return nil
}

func workflowNumberValue(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, errors.New("Giá trị phải là số")
		}
		return out, nil
	default:
		return 0, errors.New("Giá trị phải là số")
	}
}

func validateWorkflowNumberRange(field Record, numeric float64) error {
	validation, _ := field["validation_json"].(map[string]any)
	if minValue, ok := validation["min"]; ok && numeric < number(minValue) {
		return fmt.Errorf("%s phải lớn hơn hoặc bằng %v", field["label"], minValue)
	}
	if maxValue, ok := validation["max"]; ok && numeric > number(maxValue) {
		return fmt.Errorf("%s phải nhỏ hơn hoặc bằng %v", field["label"], maxValue)
	}
	return nil
}

func workflowStringList(value any) ([]string, error) {
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(item)
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		seen := map[string]bool{}
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text == "" || text == "<nil>" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, errors.New("Giá trị phải là danh sách")
	}
}

func workflowOptionAllowed(field Record, value string) bool {
	options, _ := field["options_json"].([]any)
	if len(options) == 0 {
		return true
	}
	for _, rawOption := range options {
		option, _ := rawOption.(map[string]any)
		if strings.TrimSpace(fmt.Sprint(option["value"])) == value {
			return true
		}
		if option == nil && strings.TrimSpace(fmt.Sprint(rawOption)) == value {
			return true
		}
	}
	return false
}

func workItemActionComment(req WorkItemActionRequest) string {
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	return comment
}

func (s *Service) requireWorkItemPermission(ctx context.Context, item Record, actorID string) error {
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	requiredPermission := strings.TrimSpace(fmt.Sprint(requirement["required_permission"]))
	if requiredPermission == "" || requiredPermission == "<nil>" {
		return nil
	}
	allowed, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(item["project_id"]), []string{requiredPermission})
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Bạn không có quyền bắt buộc để thao tác đầu việc này", Record{"permission": requiredPermission})
	}
	return nil
}

func (s *Service) missingRequiredEvidence(ctx context.Context, item Record, req WorkItemActionRequest) ([]string, error) {
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	if len(requirement) == 0 {
		return nil, nil
	}
	counts, err := s.repo.WorkItemEvidenceCounts(ctx, fmt.Sprint(item["id"]))
	if err != nil {
		return nil, err
	}
	missing := []string{}
	engineVersion := int(number(item["pipeline_engine_version"]))
	formRequired := boolValue(item["workflow_form_required"]) || boolValue(requirement["form_required"])
	requiredFormVersionID := cleanID(item["required_workflow_form_version_id"])
	if requiredFormVersionID == "" {
		requiredFormVersionID = cleanID(requirement["form_version_id"])
	}
	exactFormSubmitted := fmt.Sprint(item["workflow_form_status"]) == "submitted" &&
		(requiredFormVersionID == "" || cleanID(item["workflow_form_version_id"]) == requiredFormVersionID)
	if engineVersion >= 2 && formRequired && !exactFormSubmitted {
		missing = append(missing, "workflow_form")
	}
	if boolValue(requirement["document_required"]) && counts["document"] == 0 {
		missing = append(missing, "document")
	}
	if boolValue(requirement["photo_required"]) && counts["photo"] == 0 {
		missing = append(missing, "photo")
	}
	if boolValue(requirement["checklist_required"]) && counts["checklist"] == 0 && !hasChecklistPayload(req.CompletionPayload) {
		if engineVersion >= 2 && formRequired {
			if !exactFormSubmitted && !stringSliceContains(missing, "workflow_form") {
				missing = append(missing, "workflow_form")
			}
			return missing, nil
		}
		submittedForm, err := s.repo.HasSubmittedWorkflowFormForWorkItem(ctx, fmt.Sprint(item["id"]))
		if err != nil {
			return nil, err
		}
		if !submittedForm {
			missing = append(missing, "checklist")
		}
	}
	return missing, nil
}

func withLegacyChecklistCompletion(item Record, req WorkItemActionRequest) WorkItemActionRequest {
	if int(number(item["pipeline_engine_version"])) >= 2 {
		return req
	}
	requirement, _ := item["evidence_requirement_json"].(map[string]any)
	if !boolValue(requirement["checklist_required"]) || hasChecklistPayload(req.CompletionPayload) {
		return req
	}
	payload := Record{}
	for key, value := range req.CompletionPayload {
		payload[key] = value
	}
	payload["checklist"] = []any{
		Record{
			"label":   "Đã gửi từ bước quy trình",
			"checked": true,
			"source":  "pipeline_run_step_submit",
		},
	}
	req.CompletionPayload = payload
	return req
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func hasChecklistPayload(payload Record) bool {
	if len(payload) == 0 {
		return false
	}
	for _, key := range []string{"checklist", "checklist_items"} {
		if hasNonEmptyPayloadValue(payload[key]) {
			return true
		}
	}
	return false
}

func hasNonEmptyPayloadValue(value any) bool {
	switch v := value.(type) {
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	case Record:
		return len(v) > 0
	case bool:
		return v
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text != "" && text != "<nil>"
	}
}

func statusIn(status string, allowed ...string) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func (s *Service) PipelineRun(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem quy trình của công trình này")
	}
	return s.repo.ProjectPipelineRun(ctx, projectID, actorID)
}

func (s *Service) PlanningDailyReports(ctx context.Context, filter PlanningDailyReportFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningDailyReportFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem nhật ký kế hoạch của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningDailyReports(ctx, normalized, actorID)
}

func normalizePlanningDailyReportFilter(filter PlanningDailyReportFilter) (PlanningDailyReportFilter, error) {
	normalized := PlanningDailyReportFilter{
		ProjectID:  cleanID(filter.ProjectID),
		DateFrom:   strings.TrimSpace(filter.DateFrom),
		DateTo:     strings.TrimSpace(filter.DateTo),
		ReportedBy: strings.TrimSpace(filter.ReportedBy),
		Owner:      strings.TrimSpace(filter.Owner),
		Status:     strings.TrimSpace(filter.Status),
		Search:     strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày nhật ký không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái nhật ký không hợp lệ", Record{"status": normalized.Status})
	}
	return normalized, nil
}

func (s *Service) PlanningMeetingMinutes(ctx context.Context, filter PlanningMeetingMinutesFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningMeetingMinutesFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem biên bản họp của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningMeetingMinutes(ctx, normalized, actorID)
}

func normalizePlanningMeetingMinutesFilter(filter PlanningMeetingMinutesFilter) (PlanningMeetingMinutesFilter, error) {
	normalized := PlanningMeetingMinutesFilter{
		ProjectID: cleanID(filter.ProjectID),
		DateFrom:  strings.TrimSpace(filter.DateFrom),
		DateTo:    strings.TrimSpace(filter.DateTo),
		Chair:     strings.TrimSpace(filter.Chair),
		Owner:     strings.TrimSpace(filter.Owner),
		Status:    strings.TrimSpace(filter.Status),
		Search:    strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày biên bản họp không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái biên bản họp không hợp lệ", Record{"status": normalized.Status})
	}
	return normalized, nil
}

func (s *Service) PlanningChecklists(ctx context.Context, filter PlanningChecklistFilter, actorID string) ([]Record, error) {
	normalized, err := normalizePlanningChecklistFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"planning.view", "pipeline.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem checklist kế hoạch của công trình này", Record{"permissions": []string{"planning.view", "pipeline.view"}})
		}
	}
	return s.repo.PlanningChecklists(ctx, normalized, actorID)
}

func normalizePlanningChecklistFilter(filter PlanningChecklistFilter) (PlanningChecklistFilter, error) {
	normalized := PlanningChecklistFilter{
		ProjectID:  cleanID(filter.ProjectID),
		DateFrom:   strings.TrimSpace(filter.DateFrom),
		DateTo:     strings.TrimSpace(filter.DateTo),
		CheckedBy:  strings.TrimSpace(filter.CheckedBy),
		Assignee:   strings.TrimSpace(filter.Assignee),
		ItemStatus: strings.TrimSpace(filter.ItemStatus),
		Evidence:   strings.TrimSpace(filter.Evidence),
		Status:     strings.TrimSpace(filter.Status),
		Search:     strings.TrimSpace(filter.Search),
	}
	if normalized.ProjectID != "" && !validActivityUUID(normalized.ProjectID) {
		return normalized, validationError("project_id không hợp lệ", Record{"project_id": normalized.ProjectID})
	}
	if normalized.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateFrom); err != nil {
			return normalized, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": normalized.DateFrom})
		}
	}
	if normalized.DateTo != "" {
		if _, err := time.Parse("2006-01-02", normalized.DateTo); err != nil {
			return normalized, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": normalized.DateTo})
		}
	}
	if normalized.DateFrom != "" && normalized.DateTo != "" && normalized.DateFrom > normalized.DateTo {
		return normalized, validationError("Khoảng ngày checklist kế hoạch không hợp lệ", Record{"date_from": normalized.DateFrom, "date_to": normalized.DateTo})
	}
	if normalized.Status == "all" {
		normalized.Status = ""
	}
	if normalized.Status != "" && !statusIn(normalized.Status, "draft", "submitted", "reopened") {
		return normalized, validationError("Trạng thái checklist kế hoạch không hợp lệ", Record{"status": normalized.Status})
	}
	if normalized.ItemStatus == "all" {
		normalized.ItemStatus = ""
	}
	if normalized.ItemStatus != "" && !statusIn(normalized.ItemStatus, "done", "not_applicable", "needs_follow_up", "blocked") {
		return normalized, validationError("Trạng thái dòng checklist không hợp lệ", Record{"item_status": normalized.ItemStatus})
	}
	if normalized.Evidence == "all" {
		normalized.Evidence = ""
	}
	if normalized.Evidence != "" && !statusIn(normalized.Evidence, "linked", "missing") {
		return normalized, validationError("Bộ lọc minh chứng checklist không hợp lệ", Record{"evidence": normalized.Evidence})
	}
	return normalized, nil
}

func (s *Service) SubmitRunStep(ctx context.Context, stepID string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	workItemID := ""
	item, err := s.repo.GetWorkItemByRunStep(ctx, stepID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		workItemID = cleanID(item["id"])
		if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
			return nil, err
		}
		fromStatus := fmt.Sprint(item["status"])
		if !canCompleteWorkItemStatus(fromStatus) {
			return nil, validationError("Trạng thái đầu việc không cho phép hoàn tất", Record{"status": fromStatus})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, fmt.Sprint(item["id"]), actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
		}
		if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
			return nil, err
		}
		req = withLegacyChecklistCompletion(item, req)
		missing, err := s.missingRequiredEvidence(ctx, item, req)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return nil, validationError("Đầu việc cần đủ minh chứng trước khi hoàn tất", Record{"missing_evidence": missing})
		}
		if err := s.repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName); err != nil {
			return nil, err
		}
	}
	step, err := s.repo.SubmitRunStep(ctx, stepID, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if workItemID != "" {
		_ = s.repo.RunClientWorkflowAutomationForWorkItem(ctx, workItemID, actorID, actorName)
	}
	return step, nil
}

func canCompleteWorkItemStatus(status string) bool {
	return statusIn(status, "ready", "in_progress", "needs_changes", "waiting_evidence")
}

func (s *Service) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	return s.repo.AdvanceRunNode(ctx, nodeID, actorID, actorName)
}

func (s *Service) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := s.repo.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	issues, err := s.repo.NodeCompletionIssues(ctx, node)
	if err != nil {
		return nil, err
	}
	blocking := []Record{}
	for _, issue := range issues {
		if fmt.Sprint(issue["code"]) != "NODE_APPROVAL_REQUIRED" {
			blocking = append(blocking, issue)
		}
	}
	if len(blocking) > 0 {
		return nil, validationError("Node chưa đủ điều kiện gửi phê duyệt", Record{"issues": blocking})
	}
	return s.repo.RequestNodeApproval(ctx, nodeID, actorID, actorName, reason)
}

func (s *Service) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	return s.repo.BlockRunNode(ctx, nodeID, reason, actorID, actorName)
}

func (s *Service) ApprovalAction(ctx context.Context, id, action, comment, actorID, actorName string) (Record, error) {
	action = normalizeApprovalAction(action)
	if _, ok := approvalStatusForAction(action); !ok {
		return nil, validationError("Hành động phê duyệt không hợp lệ", Record{"action": action})
	}
	request, err := s.repo.GetByID(ctx, "approval_requests", id)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(request["target_type"]) == "work_item" {
		workItemID := cleanID(request["target_id"])
		if workItemID == "" {
			return nil, validationError("Yêu cầu phê duyệt chưa liên kết đầu việc", Record{"approval_request_id": id})
		}
		_, err := s.WorkItemAction(ctx, workItemID, action, WorkItemActionRequest{
			Comment: comment,
			Reason:  comment,
		}, actorID, actorName)
		if err != nil {
			return nil, err
		}
		return s.repo.GetByID(ctx, "approval_requests", id)
	}
	allowed, err := s.repo.CanDecideApprovalRequest(ctx, request, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền duyệt yêu cầu này")
	}
	return s.repo.ApplyApprovalPolicyAction(ctx, request, action, comment, actorID, actorName)
}

func normalizeApprovalAction(action string) string {
	action = strings.TrimSpace(action)
	if action == "request_change" {
		return "request-change"
	}
	return action
}

func approvalStatusForAction(action string) (string, bool) {
	switch action {
	case "approve":
		return "approved", true
	case "reject":
		return "rejected", true
	case "request-change":
		return "changes_requested", true
	default:
		return "", false
	}
}

func approvalAuditActionLabel(action string) string {
	switch action {
	case "approve":
		return "Duyệt"
	case "reject":
		return "Từ chối"
	case "request-change":
		return "Yêu cầu chỉnh sửa"
	default:
		return action
	}
}

func (s *Service) ProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	if cleanID(projectID) == "" {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	return s.repo.ListProjectBOQ(ctx, projectID)
}

func (s *Service) GetBOQ(ctx context.Context, itemID string) (Record, error) {
	itemID = cleanID(itemID)
	if itemID == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	return s.repo.GetBOQItemDetail(ctx, itemID)
}

func (s *Service) BOQProjectAction(ctx context.Context, projectID, action, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	return s.repo.BOQProjectAction(ctx, projectID, action, actorID, actorName)
}

func (s *Service) CreateBOQ(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeBOQItemRequest(projectID, body, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateBOQItem(ctx, normalized, actorID, actorName)
}

func (s *Service) UpdateBOQ(ctx context.Context, itemID string, body Record, actorID, actorName string) (Record, error) {
	if cleanID(itemID) == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	normalized, err := normalizeBOQItemRequest("", body, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQItem(ctx, itemID, normalized, actorID, actorName)
}

func (s *Service) ReviseBOQ(ctx context.Context, itemID string, req BOQReviseRequest, actorID, actorName string) (Record, error) {
	itemID = cleanID(itemID)
	if itemID == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Yêu cầu chỉnh sửa BOQ cần nêu lý do")
	}
	if len([]rune(reason)) > 2000 {
		return nil, validationError("Lý do chỉnh sửa BOQ không được vượt quá 2000 ký tự")
	}
	return s.repo.ReviseBOQItem(ctx, itemID, reason, actorID, actorName)
}

func (s *Service) UpdateBOQExpectedUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQExpectedUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQNormUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.LockBOQNormUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQFinalUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	if comment == "" {
		return nil, validationError("Khóa giá cuối thủ công cần nêu lý do miễn so sánh báo giá")
	}
	return s.repo.LockBOQFinalUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) ProjectSupplierQuotations(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectSupplierQuotations(ctx, cleanID(projectID))
}

func (s *Service) SupplierQuotationDetail(ctx context.Context, quotationID string) (Record, error) {
	return s.repo.GetSupplierQuotationDetail(ctx, cleanID(quotationID))
}

func (s *Service) SaveSupplierQuotation(ctx context.Context, projectID, quotationID string, req SupplierQuotationRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	partner, err := s.requireActivePartner(ctx, cleanID(req.SupplierPartnerID), []string{"supplier", "service_provider", "other"})
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeSupplierQuotation(ctx, projectID, req)
	if err != nil {
		return nil, err
	}
	header["supplier_partner_id"] = cleanID(partner["id"])
	header["supplier_name"] = cleanOptionalText(partner["name"])
	if cleanOptionalText(header["quotation_code"]) == "" {
		header["quotation_code"] = fmt.Sprintf("BG-%s-%d", projectID[:8], timeNowUnix())
	}
	return s.repo.SaveSupplierQuotation(ctx, projectID, cleanID(quotationID), header, items, actorID, actorName)
}

func (s *Service) SubmitSupplierQuotation(ctx context.Context, quotationID, actorID, actorName string) (Record, error) {
	return s.repo.SupplierQuotationAction(ctx, cleanID(quotationID), "submit", "", "", "", actorID, actorName)
}

func (s *Service) QualifySupplierQuotation(ctx context.Context, quotationID string, req SupplierQuotationQualificationRequest, actorID, actorName string) (Record, error) {
	technicalStatus := strings.TrimSpace(req.TechnicalStatus)
	commercialStatus := strings.TrimSpace(req.CommercialStatus)
	comment := strings.TrimSpace(req.Comment)
	if !statusIn(technicalStatus, "compliant", "conditional", "non_compliant") {
		return nil, validationError("Kết quả đánh giá kỹ thuật không hợp lệ", Record{"technical_status": technicalStatus})
	}
	if !statusIn(commercialStatus, "compliant", "non_compliant") {
		return nil, validationError("Kết quả đánh giá thương mại không hợp lệ", Record{"commercial_status": commercialStatus})
	}
	if comment == "" {
		return nil, validationError("Đánh giá báo giá cần có nhận xét")
	}
	return s.repo.SupplierQuotationAction(ctx, cleanID(quotationID), "qualify", technicalStatus, commercialStatus, comment, actorID, actorName)
}

func (s *Service) SelectSupplierQuote(ctx context.Context, quotationItemID string, req SupplierQuoteSelectionRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.DecisionReason)
	if reason == "" {
		return nil, validationError("Quyết định chọn báo giá cần nêu lý do")
	}
	if len([]rune(reason)) > 2000 {
		return nil, validationError("Lý do chọn báo giá không được vượt quá 2000 ký tự")
	}
	return s.repo.SelectSupplierQuote(ctx, cleanID(quotationItemID), reason, actorID, actorName)
}

func (s *Service) SupplierQuoteComparison(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.SupplierQuoteComparison(ctx, cleanID(projectID))
}

func (s *Service) normalizeSupplierQuotation(ctx context.Context, projectID string, req SupplierQuotationRequest) (Record, []Record, error) {
	quotationDate := strings.TrimSpace(req.QuotationDate)
	if quotationDate == "" {
		quotationDate = time.Now().Format("2006-01-02")
	}
	quotedAt, err := time.Parse("2006-01-02", quotationDate)
	if err != nil {
		return nil, nil, validationError("Ngày báo giá không hợp lệ")
	}
	validUntil := strings.TrimSpace(req.ValidUntil)
	if validUntil != "" {
		validAt, err := time.Parse("2006-01-02", validUntil)
		if err != nil || validAt.Before(quotedAt) {
			return nil, nil, validationError("Hạn hiệu lực báo giá không hợp lệ")
		}
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "VND"
	}
	if len(currency) != 3 {
		return nil, nil, validationError("Mã tiền tệ phải gồm 3 ký tự")
	}
	if req.PaymentTermsDays < 0 || req.LeadTimeDays < 0 {
		return nil, nil, validationError("Số ngày thanh toán/giao hàng không được âm")
	}
	if len(req.Items) == 0 {
		return nil, nil, validationError("Báo giá cần ít nhất một dòng BOQ")
	}
	items := make([]Record, 0, len(req.Items))
	seen := map[string]bool{}
	for index, itemReq := range req.Items {
		boqID := cleanID(itemReq.BOQItemID)
		if boqID == "" || seen[boqID] {
			return nil, nil, validationError("Dòng BOQ trong báo giá không hợp lệ hoặc bị trùng", Record{"item_index": index})
		}
		seen[boqID] = true
		boq, err := s.repo.GetBOQItemDetail(ctx, boqID)
		if err != nil || cleanID(boq["project_id"]) != projectID {
			return nil, nil, validationError("Dòng BOQ không thuộc công trình", Record{"item_index": index, "boq_item_id": boqID})
		}
		if fmt.Sprint(boq["status"]) == "deleted" || boolValue(boq["final_price_locked"]) {
			return nil, nil, validationError("Dòng BOQ đã khóa giá hoặc không còn hiệu lực", Record{"item_index": index, "boq_item_id": boqID})
		}
		quantity := itemReq.Quantity
		if quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
			quantity = number(boq["quantity"])
		}
		if quantity <= 0 || itemReq.UnitPrice <= 0 || math.IsNaN(itemReq.UnitPrice) || math.IsInf(itemReq.UnitPrice, 0) {
			return nil, nil, validationError("Khối lượng và đơn giá báo giá phải lớn hơn 0", Record{"item_index": index})
		}
		if itemReq.DiscountAmount < 0 || itemReq.DiscountAmount > quantity*itemReq.UnitPrice || itemReq.TaxRate < 0 || itemReq.TaxRate > 100 || itemReq.LeadTimeDays < 0 {
			return nil, nil, validationError("Chiết khấu, thuế hoặc thời gian giao hàng không hợp lệ", Record{"item_index": index})
		}
		compliance := strings.TrimSpace(itemReq.ComplianceStatus)
		if compliance == "" {
			compliance = "pending"
		}
		if !statusIn(compliance, "pending", "compliant", "conditional", "non_compliant") {
			return nil, nil, validationError("Mức đáp ứng của dòng báo giá không hợp lệ", Record{"item_index": index})
		}
		description := strings.TrimSpace(itemReq.Description)
		if description == "" {
			description = cleanOptionalText(boq["work_item_name"])
		}
		unit := strings.TrimSpace(itemReq.Unit)
		if unit == "" {
			unit = cleanOptionalText(boq["unit"])
		}
		items = append(items, Record{
			"boq_item_id": boqID, "material_id": cleanID(boq["material_id"]), "description": description,
			"unit": unit, "quantity": quantity, "unit_price": itemReq.UnitPrice, "discount_amount": itemReq.DiscountAmount,
			"tax_rate": itemReq.TaxRate, "lead_time_days": itemReq.LeadTimeDays, "compliance_status": compliance,
			"deviation_note": strings.TrimSpace(itemReq.DeviationNote),
		})
	}
	header := Record{
		"quotation_code": strings.TrimSpace(req.QuotationCode), "quotation_date": quotationDate, "valid_until": validUntil,
		"currency": currency, "payment_terms_days": req.PaymentTermsDays, "lead_time_days": req.LeadTimeDays,
		"delivery_terms": strings.TrimSpace(req.DeliveryTerms), "warranty_terms": strings.TrimSpace(req.WarrantyTerms), "notes": strings.TrimSpace(req.Notes),
	}
	return header, items, nil
}

func normalizeBOQUnitPriceRequest(req BOQUnitPriceRequest, lock bool) (float64, string, error) {
	if math.IsNaN(req.UnitPrice) || math.IsInf(req.UnitPrice, 0) {
		return 0, "", validationError("Đơn giá không hợp lệ")
	}
	if lock && req.UnitPrice <= 0 {
		return 0, "", validationError("Đơn giá lock phải lớn hơn 0")
	}
	if !lock && req.UnitPrice < 0 {
		return 0, "", validationError("Expected unit price không được âm")
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return 0, "", validationError("Ghi chú đơn giá không được vượt quá 1000 ký tự")
	}
	return req.UnitPrice, comment, nil
}

func normalizeBOQItemRequest(projectID string, body Record, create bool) (Record, error) {
	out := Record{}
	if projectID != "" {
		if cleanID(projectID) == "" {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		out["project_id"] = projectID
	}
	textFields := map[string]int{
		"category":       120,
		"work_item_name": 180,
		"description":    2000,
		"unit":           40,
	}
	for field, maxLen := range textFields {
		value, ok := body[field]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if len([]rune(text)) > maxLen {
			return nil, validationError("Dữ liệu BOQ vượt quá độ dài cho phép", Record{"field": field, "max_length": maxLen})
		}
		out[field] = text
	}
	if create {
		for _, field := range []string{"category", "work_item_name", "unit"} {
			if strings.TrimSpace(fmt.Sprint(out[field])) == "" {
				return nil, validationError("Dòng BOQ thiếu dữ liệu bắt buộc", Record{"field": field})
			}
		}
	}
	if value, ok := body["quantity"]; ok {
		quantity, err := workflowNumberValue(value)
		if err != nil {
			return nil, validationError("Khối lượng BOQ phải là số")
		}
		if quantity < 0 {
			return nil, validationError("Khối lượng BOQ không được âm")
		}
		out["quantity"] = quantity
	} else if create {
		out["quantity"] = float64(0)
	}
	priceValue, hasExpected := body["expected_unit_price"]
	if !hasExpected {
		priceValue, hasExpected = body["unit_price"]
	}
	if hasExpected {
		price, err := workflowNumberValue(priceValue)
		if err != nil {
			return nil, validationError("Expected unit price phải là số")
		}
		if price < 0 {
			return nil, validationError("Expected unit price không được âm")
		}
		out["expected_unit_price"] = price
	} else if create {
		out["expected_unit_price"] = float64(0)
	}
	for _, field := range []string{"material_id", "node_id"} {
		if value, ok := body[field]; ok {
			out[field] = cleanID(value)
		}
	}
	if create {
		out["status"] = "draft"
	}
	return out, nil
}

func (s *Service) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	canGenerate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"procurement.create", "procurement.approve", "boq.approve"})
	if err != nil {
		return nil, err
	}
	if !canGenerate {
		return nil, forbiddenError("Bạn không có quyền sinh yêu cầu mua từ BOQ", Record{"permissions": []string{"procurement.create", "procurement.approve", "boq.approve"}})
	}
	return s.repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, actorName)
}

func (s *Service) CreateProcurement(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	normalized, err := s.normalizeProcurementHeader(ctx, body, true)
	if err != nil {
		return nil, err
	}
	normalized["requested_by"] = actorID
	normalized["status"] = "draft"
	row, err := s.repo.Insert(ctx, "procurement_requests", procurementColumns(), normalized)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "procurement_created", "procurement_request", cleanID(row["id"]), "Tạo yêu cầu mua", Record{"request_code": row["request_code"]})
	return row, nil
}

func (s *Service) UpdateProcurement(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	current, err := s.repo.GetByID(ctx, "procurement_requests", cleanID(id))
	if err != nil {
		return nil, err
	}
	if statusIn(fmt.Sprint(current["status"]), "ordered", "partially_received", "received", "cancelled") {
		return nil, validationError("Yêu cầu mua đã đặt/nhận hàng, không thể sửa thông tin thương mại", Record{"status": current["status"]})
	}
	body["project_id"] = current["project_id"]
	body["request_code"] = current["request_code"]
	body["status"] = current["status"]
	body["requested_by"] = current["requested_by"]
	normalized, err := s.normalizeProcurementHeader(ctx, body, false)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Update(ctx, "procurement_requests", procurementColumns(), id, normalized)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "procurement_updated", "procurement_request", cleanID(row["id"]), "Cập nhật yêu cầu mua", Record{"request_code": row["request_code"]})
	return row, nil
}

func (s *Service) normalizeProcurementHeader(ctx context.Context, body Record, create bool) (Record, error) {
	out := Record{}
	projectID := cleanID(body["project_id"])
	if projectID == "" {
		return nil, validationError("Công trình là bắt buộc")
	}
	out["project_id"] = projectID
	requestCode := strings.TrimSpace(fmt.Sprint(body["request_code"]))
	if requestCode == "" && create {
		requestCode = fmt.Sprintf("PR-%d", timeNowUnix())
	}
	if requestCode == "" {
		return nil, validationError("Mã yêu cầu mua là bắt buộc")
	}
	title := strings.TrimSpace(fmt.Sprint(body["title"]))
	if title == "" {
		return nil, validationError("Tiêu đề yêu cầu mua là bắt buộc")
	}
	out["request_code"] = requestCode
	out["title"] = title
	out["requested_by"] = cleanID(body["requested_by"])
	out["status"] = strings.TrimSpace(fmt.Sprint(body["status"]))
	if out["status"] == "" {
		out["status"] = "draft"
	}
	total := number(body["total_estimated_amount"])
	if total < 0 {
		return nil, validationError("Giá trị dự kiến không được âm")
	}
	out["total_estimated_amount"] = total
	budgetStatus := strings.TrimSpace(fmt.Sprint(body["budget_status"]))
	if budgetStatus == "" {
		budgetStatus = "within_budget"
	}
	if !statusIn(budgetStatus, "within_budget", "budget_exceeded") {
		return nil, validationError("Trạng thái ngân sách không hợp lệ")
	}
	out["budget_status"] = budgetStatus
	out["approval_request_id"] = cleanID(body["approval_request_id"])
	partnerID := cleanID(body["supplier_partner_id"])
	if partnerID != "" {
		partner, err := s.requireActivePartner(ctx, partnerID, []string{"supplier", "service_provider", "other"})
		if err != nil {
			return nil, err
		}
		out["supplier_partner_id"] = partnerID
		out["supplier_name"] = cleanOptionalText(partner["name"])
	} else {
		out["supplier_partner_id"] = ""
		out["supplier_name"] = strings.TrimSpace(fmt.Sprint(body["supplier_name"]))
	}
	out["order_reference"] = strings.TrimSpace(fmt.Sprint(body["order_reference"]))
	expectedDeliveryDate := dateOnly(body["expected_delivery_date"])
	if expectedDeliveryDate != "" {
		if _, err := time.Parse("2006-01-02", expectedDeliveryDate); err != nil {
			return nil, validationError("Ngày giao dự kiến không hợp lệ")
		}
	}
	out["expected_delivery_date"] = expectedDeliveryDate
	return out, nil
}

func (s *Service) ProcurementAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.ProcurementAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) ProjectContracts(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectContracts(ctx, cleanID(projectID))
}

func (s *Service) ContractDetail(ctx context.Context, contractID string) (Record, error) {
	return s.repo.GetContractDetail(ctx, cleanID(contractID))
}

func (s *Service) SaveContract(ctx context.Context, projectID, contractID string, req ContractRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	contractType := strings.TrimSpace(req.ContractType)
	direction := strings.TrimSpace(req.Direction)
	if !statusIn(contractType, "main_client", "subcontractor", "supplier", "service", "other") {
		return nil, validationError("Loại hợp đồng không hợp lệ")
	}
	if !statusIn(direction, "receivable", "payable") || (contractType == "main_client" && direction != "receivable") {
		return nil, validationError("Chiều tài chính của hợp đồng không hợp lệ")
	}
	partnerID := cleanID(req.CounterpartyPartnerID)
	partnerName := strings.TrimSpace(req.CounterpartyName)
	if partnerID != "" {
		allowedTypes := vendorPartnerTypes()
		if direction == "receivable" {
			allowedTypes = []string{"client"}
		}
		partner, err := s.requireActivePartner(ctx, partnerID, allowedTypes)
		if err != nil {
			return nil, err
		}
		partnerName = cleanOptionalText(partner["name"])
	}
	if partnerName == "" {
		return nil, validationError("Đối tác hợp đồng là bắt buộc")
	}
	code := strings.TrimSpace(req.ContractCode)
	title := strings.TrimSpace(req.Title)
	if code == "" || title == "" {
		return nil, validationError("Mã và tên hợp đồng là bắt buộc")
	}
	if req.OriginalAmount < 0 || req.AdvancePercent < 0 || req.AdvancePercent > 100 || req.RetentionPercent < 0 || req.RetentionPercent > 100 || req.WarrantyDays < 0 {
		return nil, validationError("Giá trị, tạm ứng, giữ lại hoặc bảo hành không hợp lệ")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "VND"
	}
	if len(currency) != 3 {
		return nil, validationError("Mã tiền tệ phải gồm 3 ký tự")
	}
	signedDate, effectiveDate, expiryDate, err := normalizeContractDates(req.SignedDate, req.EffectiveDate, req.ExpiryDate)
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "contract_code": code, "title": title, "contract_type": contractType, "direction": direction,
		"counterparty_partner_id": partnerID, "counterparty_name": partnerName, "currency": currency,
		"original_amount": req.OriginalAmount, "advance_percent": req.AdvancePercent, "retention_percent": req.RetentionPercent,
		"warranty_days": req.WarrantyDays, "signed_date": signedDate, "effective_date": effectiveDate, "expiry_date": expiryDate,
	}
	return s.repo.SaveContract(ctx, contractID, body, actorID, actorName)
}

func normalizeContractDates(signedDate, effectiveDate, expiryDate string) (string, string, string, error) {
	values := []string{strings.TrimSpace(signedDate), strings.TrimSpace(effectiveDate), strings.TrimSpace(expiryDate)}
	for _, value := range values {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return "", "", "", validationError("Ngày hợp đồng không hợp lệ")
			}
		}
	}
	if values[1] != "" && values[2] != "" && values[2] < values[1] {
		return "", "", "", validationError("Ngày hết hạn phải sau ngày hiệu lực")
	}
	return values[0], values[1], values[2], nil
}

func (s *Service) CreateContractVersion(ctx context.Context, contractID string, req ContractVersionRequest, actorID, actorName string) (Record, error) {
	if req.Amount < 0 || strings.TrimSpace(req.ScopeSummary) == "" {
		return nil, validationError("Giá trị và phạm vi phiên bản hợp đồng không hợp lệ")
	}
	startDate, endDate, _, err := normalizeContractDates(req.PlannedStartDate, req.PlannedEndDate, "")
	if err != nil {
		return nil, err
	}
	if startDate != "" && endDate != "" && endDate < startDate {
		return nil, validationError("Ngày kết thúc phiên bản phải sau ngày bắt đầu")
	}
	body := Record{
		"version_label": strings.TrimSpace(req.VersionLabel), "amount": req.Amount, "scope_summary": strings.TrimSpace(req.ScopeSummary),
		"commercial_terms_json": req.CommercialTerms, "payment_terms_json": req.PaymentTerms,
		"planned_start_date": startDate, "planned_end_date": endDate, "document_id": cleanID(req.DocumentID), "comment": strings.TrimSpace(req.Comment),
	}
	return s.repo.CreateContractVersion(ctx, cleanID(contractID), body, actorID, actorName)
}

func (s *Service) ContractVersionAction(ctx context.Context, versionID, action, comment, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "approve", "sign") && comment == "" {
		return nil, validationError("Duyệt/ký phiên bản hợp đồng cần có nhận xét")
	}
	return s.repo.ContractVersionAction(ctx, cleanID(versionID), action, comment, actorID, actorName)
}

func (s *Service) ActivateContract(ctx context.Context, contractID, comment, actorID, actorName string) (Record, error) {
	return s.repo.ActivateContract(ctx, cleanID(contractID), strings.TrimSpace(comment), actorID, actorName)
}

func (s *Service) CreateContractMilestone(ctx context.Context, contractID string, req ContractMilestoneRequest, actorID, actorName string) (Record, error) {
	if strings.TrimSpace(req.MilestoneCode) == "" || strings.TrimSpace(req.MilestoneName) == "" || req.SequenceNo <= 0 {
		return nil, validationError("Mã, tên và thứ tự milestone là bắt buộc")
	}
	if !statusIn(req.MilestoneType, "advance", "progress", "final", "retention_release", "other") || !statusIn(req.AmountBasis, "fixed", "percentage", "quantity_certificate") {
		return nil, validationError("Loại milestone hoặc cơ sở giá trị không hợp lệ")
	}
	if req.GrossAmount < 0 || req.Percentage < 0 || req.Percentage > 100 || req.RetentionPercent < 0 || req.RetentionPercent > 100 || req.DueDays < 0 {
		return nil, validationError("Giá trị milestone không hợp lệ")
	}
	plannedDate := strings.TrimSpace(req.PlannedDate)
	if plannedDate != "" {
		if _, err := time.Parse("2006-01-02", plannedDate); err != nil {
			return nil, validationError("Ngày milestone không hợp lệ")
		}
	}
	body := Record{
		"milestone_code": strings.TrimSpace(req.MilestoneCode), "milestone_name": strings.TrimSpace(req.MilestoneName), "sequence_no": req.SequenceNo,
		"milestone_type": req.MilestoneType, "amount_basis": req.AmountBasis, "percentage": req.Percentage, "gross_amount": req.GrossAmount,
		"retention_percent": req.RetentionPercent, "planned_date": plannedDate, "due_days": req.DueDays,
		"acceptance_required": req.AcceptanceRequired, "acceptance_id": cleanID(req.AcceptanceID),
	}
	return s.repo.CreateContractMilestone(ctx, cleanID(contractID), body, actorID, actorName)
}

func (s *Service) ContractMilestoneAction(ctx context.Context, milestoneID, action, comment, actorID, actorName string) (Record, error) {
	return s.repo.ContractMilestoneAction(ctx, cleanID(milestoneID), action, strings.TrimSpace(comment), actorID, actorName)
}

func (s *Service) CreateContractVariation(ctx context.Context, contractID string, req ContractVariationRequest, actorID, actorName string) (Record, error) {
	if strings.TrimSpace(req.VariationCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Reason) == "" {
		return nil, validationError("Mã, tên và lý do phụ lục/variation là bắt buộc")
	}
	if !statusIn(req.VariationType, "scope", "price", "time", "mixed") {
		return nil, validationError("Loại variation không hợp lệ")
	}
	body := Record{"variation_code": strings.TrimSpace(req.VariationCode), "title": strings.TrimSpace(req.Title), "variation_type": req.VariationType,
		"reason": strings.TrimSpace(req.Reason), "amount_delta": req.AmountDelta, "days_delta": req.DaysDelta,
		"scope_delta": strings.TrimSpace(req.ScopeDelta), "document_id": cleanID(req.DocumentID), "comment": strings.TrimSpace(req.Comment)}
	return s.repo.CreateContractVariation(ctx, cleanID(contractID), body, actorID, actorName)
}

func (s *Service) ContractVariationAction(ctx context.Context, variationID, action, comment, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "approve", "reject") && comment == "" {
		return nil, validationError("Duyệt/từ chối variation cần có nhận xét")
	}
	return s.repo.ContractVariationAction(ctx, cleanID(variationID), action, comment, actorID, actorName)
}

func (s *Service) ReleaseContractRetention(ctx context.Context, retentionID string, req ContractRetentionReleaseRequest, actorID, actorName string) (Record, error) {
	if req.Amount <= 0 || strings.TrimSpace(req.Comment) == "" {
		return nil, validationError("Số tiền và lý do giải phóng giữ lại là bắt buộc")
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Ngày đến hạn giải phóng giữ lại không hợp lệ")
		}
	}
	return s.repo.ReleaseContractRetention(ctx, cleanID(retentionID), req.Amount, dueDate, strings.TrimSpace(req.Comment), actorID, actorName)
}

func (s *Service) ProjectQualityWorkspace(ctx context.Context, projectID string) (Record, error) {
	return s.repo.ProjectQualityWorkspace(ctx, cleanID(projectID))
}

func (s *Service) CreateQualityITP(ctx context.Context, projectID string, req QualityITPRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.ITPCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Discipline) == "" || strings.TrimSpace(req.WorkPackage) == "" {
		return nil, validationError("Mã, tên, bộ môn và gói công việc ITP là bắt buộc")
	}
	if len(req.Items) == 0 {
		return nil, validationError("ITP cần ít nhất một inspection item")
	}
	items := make([]Record, 0, len(req.Items))
	seen := map[int]bool{}
	for index, item := range req.Items {
		if item.SequenceNo <= 0 || seen[item.SequenceNo] || strings.TrimSpace(item.ActivityName) == "" || strings.TrimSpace(item.AcceptanceCriteria) == "" || strings.TrimSpace(item.InspectionMethod) == "" || strings.TrimSpace(item.ResponsibleRole) == "" {
			return nil, validationError("Inspection item ITP không hợp lệ hoặc trùng thứ tự", Record{"item_index": index})
		}
		seen[item.SequenceNo] = true
		controlPoint := strings.TrimSpace(item.ControlPoint)
		if !statusIn(controlPoint, "review", "witness", "hold", "surveillance") {
			return nil, validationError("Control point ITP không hợp lệ", Record{"item_index": index})
		}
		frequencyType := strings.TrimSpace(item.FrequencyType)
		if !statusIn(frequencyType, "each_lot", "daily", "weekly", "quantity", "custom") || item.FrequencyValue <= 0 {
			return nil, validationError("Tần suất ITP không hợp lệ", Record{"item_index": index})
		}
		items = append(items, Record{"sequence_no": item.SequenceNo, "activity_name": strings.TrimSpace(item.ActivityName), "inspection_stage": strings.TrimSpace(item.InspectionStage), "acceptance_criteria": strings.TrimSpace(item.AcceptanceCriteria), "inspection_method": strings.TrimSpace(item.InspectionMethod), "control_point": controlPoint, "frequency_type": frequencyType, "frequency_value": item.FrequencyValue, "responsible_role": strings.TrimSpace(item.ResponsibleRole), "evidence_required": item.EvidenceRequired, "client_witness_required": item.ClientWitnessRequired, "notes": strings.TrimSpace(item.Notes)})
	}
	header := Record{"itp_code": strings.TrimSpace(req.ITPCode), "title": strings.TrimSpace(req.Title), "discipline": strings.TrimSpace(req.Discipline), "work_package": strings.TrimSpace(req.WorkPackage), "document_id": cleanID(req.DocumentID), "review_comment": strings.TrimSpace(req.ReviewComment)}
	return s.repo.CreateQualityITP(ctx, projectID, header, items, actorID, actorName)
}

func (s *Service) ReviseQualityITP(ctx context.Context, itpID, comment, actorID, actorName string) (Record, error) {
	return s.repo.ReviseQualityITP(ctx, cleanID(itpID), strings.TrimSpace(comment), actorID, actorName)
}

func (s *Service) QualityITPAction(ctx context.Context, itpID, action, comment, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "approve", "activate") && comment == "" {
		return nil, validationError("Duyệt/kích hoạt ITP cần có nhận xét")
	}
	return s.repo.QualityITPAction(ctx, cleanID(itpID), action, comment, actorID, actorName)
}

func (s *Service) CreateQualityInspectionRequest(ctx context.Context, projectID string, req QualityInspectionRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || cleanID(req.ITPItemID) == "" || strings.TrimSpace(req.RFICode) == "" || strings.TrimSpace(req.Location) == "" {
		return nil, validationError("Mã RFI, ITP item và vị trí kiểm tra là bắt buộc")
	}
	requestedAt := strings.TrimSpace(req.RequestedInspectionAt)
	if _, err := time.Parse(time.RFC3339, requestedAt); err != nil {
		return nil, validationError("Thời điểm yêu cầu kiểm tra phải theo RFC3339")
	}
	body := Record{"rfi_code": strings.TrimSpace(req.RFICode), "itp_item_id": cleanID(req.ITPItemID), "work_item_id": cleanID(req.WorkItemID), "location": strings.TrimSpace(req.Location), "lot_reference": strings.TrimSpace(req.LotReference), "requested_inspection_at": requestedAt, "request_comment": strings.TrimSpace(req.RequestComment), "request_evidence_id": cleanID(req.RequestEvidenceID)}
	return s.repo.CreateQualityInspectionRequest(ctx, projectID, body, actorID, actorName)
}

func (s *Service) QualityInspectionAction(ctx context.Context, rfiID, action string, req QualityInspectionDecisionRequest, actorID, actorName string) (Record, error) {
	if action == "inspect" {
		if !statusIn(req.Result, "pass", "conditional_pass", "fail") || strings.TrimSpace(req.Comment) == "" {
			return nil, validationError("Inspection cần kết quả và nhận xét")
		}
	}
	return s.repo.QualityInspectionAction(ctx, cleanID(rfiID), action, strings.TrimSpace(req.Result), strings.TrimSpace(req.Comment), cleanID(req.InspectionEvidenceID), actorID, actorName)
}

func (s *Service) CreateQualityNCR(ctx context.Context, projectID string, req QualityNCRRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.NCRCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Nonconformity) == "" || !statusIn(req.SourceType, "rfi", "issue", "work_item", "manual") || !statusIn(req.Severity, "low", "medium", "high", "critical") {
		return nil, validationError("Dữ liệu NCR không hợp lệ")
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Hạn NCR không hợp lệ")
		}
	}
	sourceID := cleanID(req.SourceID)
	if sourceID == "" {
		sourceID = map[string]string{"rfi": cleanID(req.RFIID), "issue": cleanID(req.IssueID), "work_item": cleanID(req.WorkItemID), "manual": projectID}[req.SourceType]
	}
	if sourceID == "" {
		return nil, validationError("NCR cần hồ sơ nguồn phù hợp")
	}
	body := Record{"ncr_code": strings.TrimSpace(req.NCRCode), "source_type": req.SourceType, "source_id": sourceID, "rfi_id": cleanID(req.RFIID), "work_item_id": cleanID(req.WorkItemID), "issue_id": cleanID(req.IssueID), "title": strings.TrimSpace(req.Title), "severity": req.Severity, "nonconformity": strings.TrimSpace(req.Nonconformity), "immediate_action": strings.TrimSpace(req.ImmediateAction), "due_date": dueDate, "evidence_id": cleanID(req.EvidenceID), "owner_id": cleanID(req.OwnerID)}
	return s.repo.CreateQualityNCR(ctx, projectID, body, actorID, actorName)
}

func (s *Service) QualityNCRAction(ctx context.Context, ncrID, action string, req QualityNCRActionRequest, actorID, actorName string) (Record, error) {
	if action == "root-cause" && strings.TrimSpace(req.RootCause) == "" {
		return nil, validationError("NCR cần phân tích nguyên nhân gốc")
	}
	if action == "corrective" && (strings.TrimSpace(req.CorrectiveAction) == "" || cleanID(req.CorrectiveEvidenceID) == "") {
		return nil, validationError("Corrective action cần nội dung và minh chứng")
	}
	if action == "reinspection" && cleanID(req.ReinspectionRFIID) == "" {
		return nil, validationError("Cần liên kết RFI tái kiểm tra")
	}
	if action == "close" && strings.TrimSpace(req.Comment) == "" {
		return nil, validationError("Đóng NCR cần nhận xét")
	}
	return s.repo.QualityNCRAction(ctx, cleanID(ncrID), action, req, actorID, actorName)
}

func (s *Service) CreateRecurringPlan(ctx context.Context, projectID string, req ConstructionRecurringPlanRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" || strings.TrimSpace(req.PlanCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.ResponsibleRole) == "" || !statusIn(req.RecurrenceType, "daily", "weekly", "monthly", "custom_days") || req.IntervalValue <= 0 {
		return nil, validationError("Dữ liệu lịch QA/QC lặp không hợp lệ")
	}
	startDate := strings.TrimSpace(req.StartDate)
	endDate := strings.TrimSpace(req.EndDate)
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		return nil, validationError("Ngày bắt đầu lịch lặp không hợp lệ")
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil || endDate < startDate {
			return nil, validationError("Ngày kết thúc lịch lặp không hợp lệ")
		}
	}
	body := Record{"plan_code": strings.TrimSpace(req.PlanCode), "title": strings.TrimSpace(req.Title), "description": strings.TrimSpace(req.Description), "recurrence_type": req.RecurrenceType, "interval_value": req.IntervalValue, "start_date": startDate, "end_date": endDate, "due_time": strings.TrimSpace(req.DueTime), "responsible_role": strings.TrimSpace(req.ResponsibleRole), "assigned_user_id": cleanID(req.AssignedUserID), "evidence_required": req.EvidenceRequired, "source_work_item_id": cleanID(req.SourceWorkItemID)}
	return s.repo.CreateRecurringPlan(ctx, projectID, body, actorID, actorName)
}

func (s *Service) GenerateRecurringOccurrences(ctx context.Context, planID string, throughDate string, actorID, actorName string) (Record, error) {
	throughDate = strings.TrimSpace(throughDate)
	if throughDate == "" {
		throughDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", throughDate); err != nil {
		return nil, validationError("Ngày sinh occurrence không hợp lệ")
	}
	return s.repo.GenerateRecurringOccurrences(ctx, cleanID(planID), throughDate, actorID, actorName)
}

func (s *Service) RecurringOccurrenceAction(ctx context.Context, occurrenceID, action string, req ConstructionRecurringOccurrenceRequest, actorID, actorName string) (Record, error) {
	if action == "complete" && strings.TrimSpace(req.Comment) == "" {
		return nil, validationError("Hoàn tất occurrence cần ghi chú")
	}
	if action == "skip" && strings.TrimSpace(req.Reason) == "" {
		return nil, validationError("Bỏ qua occurrence cần lý do")
	}
	return s.repo.RecurringOccurrenceAction(ctx, cleanID(occurrenceID), action, strings.TrimSpace(req.Comment), cleanID(req.EvidenceID), strings.TrimSpace(req.Reason), actorID, actorName)
}

func (s *Service) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockMovement(ctx, req, actorID, actorName)
}

func (s *Service) CreateStockDocument(ctx context.Context, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockDocument(ctx, req, actorID, actorName)
}

func (s *Service) ProjectAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	return s.repo.ListProjectAcceptanceRecords(ctx, projectID)
}

func (s *Service) WorkItemOutgoingAcceptance(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem công việc liên quan này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "acceptance.create", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu ra", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	records, err := s.repo.ListAcceptanceRecordsForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		form = Record{"available": false}
	} else if err != nil {
		return nil, err
	} else {
		form["available"] = true
	}
	return Record{
		"available":    true,
		"work_item_id": item["id"],
		"project_id":   projectID,
		"records":      records,
		"form":         form,
	}, nil
}

func (s *Service) GetAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem công việc liên quan này")
		}
	}
	return record, nil
}

func (s *Service) CreateAcceptance(ctx context.Context, projectID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateAcceptance(ctx context.Context, acceptanceID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Hồ sơ nghiệm thu đầu ra đã gửi/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được gửi", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, cleanID(existing["work_item_id"]), actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền gửi nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	req := acceptanceRequestFromRecord(existing)
	if _, _, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitAcceptance(ctx, cleanID(existing["id"]), actorID, actorName)
}

func (s *Service) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if cleanID(existing["work_item_id"]) != "" {
		if fmt.Sprint(existing["status"]) != "submitted" {
			return nil, validationError("Nghiệm thu đầu ra cần gửi trước khi chốt kết quả", Record{"acceptance_id": id, "status": existing["status"]})
		}
		if err := validateAcceptanceActionResult(result, recordValue(existing["summary_json"])); err != nil {
			return nil, err
		}
	}
	return s.repo.AcceptanceAction(ctx, id, result, req, actorID, actorName)
}

func (s *Service) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	return s.repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, actorName)
}

func (s *Service) requireAcceptanceWriteAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	if workItemID == "" {
		return nil, nil
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác công việc liên quan này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Công việc liên quan không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) normalizeAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req AcceptanceRequest, submit bool) (Record, []Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" && item != nil {
		workItemID = cleanID(item["id"])
	}
	if workItemID == "" && existing != nil {
		workItemID = cleanID(existing["work_item_id"])
	}
	if workItemID != "" {
		if item == nil || cleanID(item["id"]) != workItemID {
			workItem, err := s.repo.GetWorkItemDetail(ctx, workItemID)
			if err != nil {
				return nil, nil, err
			}
			item = workItem
		}
		if cleanID(item["project_id"]) != projectID {
			return nil, nil, validationError("Công việc liên quan không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	acceptanceType := strings.TrimSpace(req.Type)
	if acceptanceType == "" {
		acceptanceType = "partial"
	}
	if !statusIn(acceptanceType, "partial", "final") {
		return nil, nil, validationError("Loại nghiệm thu không hợp lệ", Record{"type": acceptanceType})
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Nghiệm thu đầu ra"
	}
	if len([]rune(title)) > 240 {
		return nil, nil, validationError("Tiêu đề nghiệm thu không được vượt quá 240 ký tự")
	}
	acceptanceDate := strings.TrimSpace(req.AcceptanceDate)
	if acceptanceDate != "" {
		if _, err := time.Parse("2006-01-02", acceptanceDate); err != nil {
			return nil, nil, validationError("Ngày nghiệm thu không hợp lệ", Record{"acceptance_date": acceptanceDate})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nghiệm thu là bắt buộc khi gửi")
	}
	location := strings.TrimSpace(req.Location)
	clientRepresentative := strings.TrimSpace(req.ClientRepresentative)
	contractorRepresentative := strings.TrimSpace(req.ContractorRepresentative)
	scopeSummary := strings.TrimSpace(req.ScopeSummary)
	for field, value := range map[string]string{
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	if len([]rune(scopeSummary)) > 1000 {
		return nil, nil, validationError("Phạm vi nghiệm thu không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu không được vượt quá 2000 ký tự")
	}

	formInstanceID := cleanID(req.FormInstanceID)
	formHasSignature := false
	if formInstanceID != "" {
		form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, formInstanceID)
		if err != nil {
			return nil, nil, validationError("Biểu mẫu quy trình liên kết nghiệm thu không hợp lệ", Record{"form_instance_id": formInstanceID})
		}
		instance, _ := form["instance"].(Record)
		if cleanID(instance["project_id"]) != projectID {
			return nil, nil, validationError("Biểu mẫu quy trình không thuộc công trình nghiệm thu", Record{"form_instance_id": formInstanceID})
		}
		if workItemID != "" && cleanID(instance["work_item_id"]) != workItemID {
			return nil, nil, validationError("Biểu mẫu quy trình không thuộc công việc liên quan", Record{"form_instance_id": formInstanceID})
		}
		if submit && fmt.Sprint(instance["status"]) != "submitted" {
			return nil, nil, validationError("Biểu mẫu quy trình cần gửi trước nghiệm thu đầu ra", Record{"form_instance_id": formInstanceID})
		}
		if signatures, ok := form["signatures"].([]Record); ok && len(signatures) > 0 {
			formHasSignature = true
		}
	}
	pdfExportID := cleanID(req.PDFExportID)
	if pdfExportID != "" {
		export, err := s.repo.GetWorkflowFormPDFExportDetail(ctx, pdfExportID)
		if err != nil {
			return nil, nil, validationError("PDF biểu mẫu quy trình liên kết nghiệm thu không hợp lệ", Record{"pdf_export_id": pdfExportID})
		}
		if cleanID(export["project_id"]) != projectID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc công trình nghiệm thu", Record{"pdf_export_id": pdfExportID})
		}
		if workItemID != "" && cleanID(export["work_item_id"]) != workItemID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc công việc liên quan", Record{"pdf_export_id": pdfExportID})
		}
		if formInstanceID != "" && cleanID(export["form_instance_id"]) != formInstanceID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc biểu mẫu quy trình đã chọn", Record{"pdf_export_id": pdfExportID, "form_instance_id": formInstanceID})
		}
		if formInstanceID == "" {
			formInstanceID = cleanID(export["form_instance_id"])
		}
	}
	signatureStatus := strings.TrimSpace(req.SignatureStatus)
	if signatureStatus == "" {
		if formHasSignature {
			signatureStatus = "signed"
		} else {
			signatureStatus = "pending"
		}
	}
	if !statusIn(signatureStatus, "pending", "signed", "waived") {
		return nil, nil, validationError("Trạng thái chữ ký nghiệm thu không hợp lệ", Record{"signature_status": signatureStatus})
	}
	documentStatus := strings.TrimSpace(req.DocumentStatus)
	if documentStatus == "" {
		if pdfExportID != "" {
			documentStatus = "linked"
		} else {
			documentStatus = "pending"
		}
	}
	if !statusIn(documentStatus, "pending", "linked", "waived") {
		return nil, nil, validationError("Trạng thái tài liệu nghiệm thu không hợp lệ", Record{"document_status": documentStatus})
	}
	if submit && signatureStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần chữ ký hoặc miễn chữ ký trước khi gửi")
	}
	if submit && documentStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần tài liệu/PDF hoặc miễn tài liệu trước khi gửi")
	}
	if submit && len(req.Checklist) == 0 {
		return nil, nil, validationError("Cần ít nhất một dòng checklist để gửi nghiệm thu đầu ra")
	}

	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Checklist))
	for index, itemReq := range req.Checklist {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng checklist nghiệm thu không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng checklist nghiệm thu bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeAcceptanceChecklistItem(ctx, projectID, workItemID, itemIndex, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := acceptanceSummary(normalized)
	if submit {
		if pending := int(number(summary["pending"])); pending > 0 {
			return nil, nil, validationError("Không thể gửi nghiệm thu đầu ra khi còn checklist chờ xử lý", Record{"pending": pending})
		}
	}
	header := Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"form_instance_id":          formInstanceID,
		"pdf_export_id":             pdfExportID,
		"type":                      acceptanceType,
		"title":                     title,
		"acceptance_date":           acceptanceDate,
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
		"scope_summary":             scopeSummary,
		"signature_status":          signatureStatus,
		"document_status":           documentStatus,
		"notes":                     notes,
		"summary_json":              summary,
	}
	return header, normalized, nil
}

func (s *Service) normalizeAcceptanceChecklistItem(ctx context.Context, projectID, workItemID string, itemIndex int, req AcceptanceChecklistItemRequest, submit bool) (Record, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, validationError("Tên checklist nghiệm thu là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(title)) > 240 {
		return nil, validationError("Tên checklist nghiệm thu không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 1000 {
		return nil, validationError("Mô tả checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = "pending"
	}
	if !statusIn(decision, "pending", "pass", "conditional_pass", "fail") {
		return nil, validationError("Quyết định checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit && decision == "pending" {
		return nil, validationError("Checklist nghiệm thu chưa có quyết định", Record{"item_index": itemIndex})
	}
	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "medium"
	}
	if !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	requiredAction := strings.TrimSpace(req.RequiredAction)
	if len([]rune(requiredAction)) > 1000 {
		return nil, validationError("Yêu cầu xử lý checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Ngày hạn xử lý checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "due_date": dueDate})
		}
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 1000 {
		return nil, validationError("Ghi chú checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	photoURL := strings.TrimSpace(req.PhotoURL)
	if len([]rune(photoURL)) > 1000 {
		return nil, validationError("URL ảnh checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceIDs := normalizeAcceptanceEvidenceIDs(req.EvidenceIDs, req.EvidenceID)
	if err := validateAcceptanceEvidenceLimit(evidenceIDs, itemIndex); err != nil {
		return nil, err
	}
	for _, evidenceID := range evidenceIDs {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng checklist nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng checklist nghiệm thu không thuộc công việc liên quan", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	evidenceRequired := req.PhotoRequired || decision == "conditional_pass" || decision == "fail"
	if submit && evidenceRequired && len(evidenceIDs) == 0 && photoURL == "" {
		return nil, validationError("Checklist nghiệm thu cần minh chứng/ảnh", Record{"item_index": itemIndex})
	}
	if submit && statusIn(decision, "conditional_pass", "fail") && requiredAction == "" {
		return nil, validationError("Checklist có điều kiện/không đạt cần yêu cầu xử lý", Record{"item_index": itemIndex})
	}
	return Record{
		"item_index":      itemIndex,
		"title":           title,
		"description":     description,
		"decision":        decision,
		"severity":        severity,
		"required_action": requiredAction,
		"due_date":        dueDate,
		"evidence_id":     firstString(evidenceIDs),
		"evidence_ids":    evidenceIDs,
		"note":            note,
		"photo_required":  req.PhotoRequired,
		"photo_url":       photoURL,
		"payload_json": Record{
			"evidence_required": evidenceRequired,
		},
	}, nil
}

func acceptanceRequestFromRecord(record Record) AcceptanceRequest {
	req := AcceptanceRequest{
		WorkItemID:               cleanID(record["work_item_id"]),
		FormInstanceID:           cleanID(record["form_instance_id"]),
		PDFExportID:              cleanID(record["pdf_export_id"]),
		Type:                     cleanOptionalText(record["type"]),
		Title:                    cleanOptionalText(record["title"]),
		AcceptanceDate:           dateOnly(record["acceptance_date"]),
		Location:                 cleanOptionalText(record["location"]),
		ClientRepresentative:     cleanOptionalText(record["client_representative"]),
		ContractorRepresentative: cleanOptionalText(record["contractor_representative"]),
		ScopeSummary:             cleanOptionalText(record["scope_summary"]),
		SignatureStatus:          cleanOptionalText(record["signature_status"]),
		DocumentStatus:           cleanOptionalText(record["document_status"]),
		Notes:                    cleanOptionalText(record["notes"]),
		Checklist:                []AcceptanceChecklistItemRequest{},
	}
	if items, ok := record["checklist"].([]Record); ok {
		req.Checklist = make([]AcceptanceChecklistItemRequest, 0, len(items))
		for _, item := range items {
			req.Checklist = append(req.Checklist, AcceptanceChecklistItemRequest{
				ItemIndex:      int(number(item["item_index"])),
				Title:          cleanOptionalText(item["title"]),
				Description:    cleanOptionalText(item["description"]),
				Decision:       cleanOptionalText(item["decision"]),
				Severity:       cleanOptionalText(item["severity"]),
				RequiredAction: cleanOptionalText(item["required_action"]),
				DueDate:        dateOnly(item["due_date"]),
				EvidenceID:     cleanID(item["evidence_id"]),
				EvidenceIDs:    normalizeAcceptanceEvidenceIDs(stringSlice(item["evidence_ids"]), cleanID(item["evidence_id"])),
				Note:           cleanOptionalText(item["note"]),
				PhotoRequired:  boolValue(item["photo_required"]),
				PhotoURL:       cleanOptionalText(item["photo_url"]),
			})
		}
	}
	return req
}

func normalizeAcceptanceEvidenceIDs(ids []string, primaryID string) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(ids)+1)
	add := func(value string) {
		value = cleanID(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		values = append(values, value)
	}
	add(primaryID)
	for _, id := range ids {
		add(id)
	}
	return values
}

func validateAcceptanceEvidenceLimit(ids []string, itemIndex int) error {
	if len(ids) <= 10 {
		return nil
	}
	return validationError("Mỗi tiêu chí nghiệm thu được chọn tối đa 10 minh chứng", Record{"item_index": itemIndex, "evidence_count": len(ids), "max": 10})
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func acceptanceSummary(items []Record) Record {
	summary := Record{
		"total":          len(items),
		"pass":           0,
		"conditional":    0,
		"fail":           0,
		"pending":        0,
		"evidence_count": 0,
		"critical_count": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "pass":
			summary["pass"] = int(number(summary["pass"])) + 1
		case "conditional_pass":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "fail":
			summary["fail"] = int(number(summary["fail"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" || cleanOptionalText(item["photo_url"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanOptionalText(item["severity"]) == "critical" {
			summary["critical_count"] = int(number(summary["critical_count"])) + 1
		}
	}
	return summary
}

func validateAcceptanceActionResult(result string, summary Record) error {
	result = strings.TrimSpace(result)
	total := int(number(summary["total"]))
	passed := int(number(summary["pass"]))
	conditional := int(number(summary["conditional"]))
	failed := int(number(summary["fail"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần hạng mục checklist trước khi chốt nghiệm thu đầu ra")
	}
	if pending > 0 {
		return validationError("Không thể chốt nghiệm thu khi còn checklist pending", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if passed != total {
			return validationError("Kết quả đạt chỉ hợp lệ khi mọi checklist đều đạt")
		}
	case "conditional_pass":
		if conditional+failed == 0 {
			return validationError("Kết quả đạt có điều kiện cần checklist có điều kiện hoặc không đạt")
		}
	case "fail":
		if failed == 0 {
			return validationError("Kết quả không đạt cần ít nhất một checklist không đạt")
		}
	default:
		return validationError("Kết quả nghiệm thu không hợp lệ", Record{"result": result})
	}
	return nil
}

func recordValue(value any) Record {
	switch v := value.(type) {
	case Record:
		return v
	case map[string]any:
		return Record(v)
	default:
		return Record{}
	}
}

func (s *Service) HandoverDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetHandoverDetail(ctx, cleanID(id))
}

func (s *Service) SubmitHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.SubmitHandover(ctx, cleanID(id), req, actorID, actorName)
}

func (s *Service) HandoverCheck(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return s.repo.HandoverCheck(ctx, cleanID(id), actorID, actorName)
}

func (s *Service) UpdateHandoverChecklistItem(ctx context.Context, id string, req HandoverChecklistItemRequest, actorID, actorName string) (Record, error) {
	status := strings.TrimSpace(req.Status)
	if !statusIn(status, "pending", "verified", "waived") {
		return nil, validationError("Trạng thái checklist bàn giao không hợp lệ", Record{"status": status})
	}
	evidenceID := cleanID(req.EvidenceID)
	documentID := cleanID(req.DocumentID)
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú checklist không được vượt quá 2000 ký tự")
	}
	return s.repo.UpdateHandoverChecklistItem(ctx, cleanID(id), Record{
		"status": status, "evidence_id": evidenceID, "document_id": documentID, "notes": notes,
	}, actorID, actorName)
}

func (s *Service) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CompleteHandover(ctx, id, req, actorID, actorName)
}

func (s *Service) PayablePaymentSchedules(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem lịch thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayablePaymentSchedules(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayablePaymentSchedule(ctx context.Context, payableID string, req PayablePaymentScheduleRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create", "accounting.payable_approve"}, "Bạn không có quyền tạo kỳ thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayablePaymentScheduleRequest(payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayablePaymentSchedule(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	scheduleID = cleanID(scheduleID)
	if !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	return s.repo.PayablePaymentScheduleAction(ctx, scheduleID, action, req, actorID, actorName)
}

func (s *Service) PayableAdvanceRequests(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem ứng trước đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayableAdvanceRequests(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayableAdvanceRequest(ctx context.Context, payableID string, req AdvanceRequestCreateRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create", "accounting.payable_approve"}, "Bạn không có quyền tạo ứng trước đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayableAdvanceRequest(payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayableAdvanceRequest(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayableAdvanceRequestAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	return s.repo.PayableAdvanceRequestAction(ctx, advanceID, action, req, actorID, actorName)
}

func (s *Service) SettlePayableAdvanceRequest(ctx context.Context, advanceID string, req AdvanceSettlementRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	advance, err := s.repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeAdvanceSettlementRequest(ctx, advance, req)
	if err != nil {
		return nil, err
	}
	return s.repo.SettlePayableAdvanceRequest(ctx, advanceID, body, actorID, actorName)
}

func (s *Service) PayablePayments(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view", "accounting.payment_create", "accounting.payment_approve"}, "Bạn không có quyền xem thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayablePayments(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayablePayment(ctx context.Context, payableID string, req PayablePaymentCreateRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create"}, "Bạn không có quyền tạo thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayablePaymentRequest(ctx, payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayablePayment(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayableInputInvoices(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListInputInvoicesForPayable(ctx, cleanID(payable["id"]))
}

func (s *Service) PayableReconciliation(ctx context.Context, payableID, actorID string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem đối soát công nợ phải trả")
	if err != nil {
		return nil, err
	}
	return s.repo.PayableReconciliation(ctx, cleanID(payable["id"]))
}

func (s *Service) ProjectInputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu vào", Record{"permissions": []string{"accounting.view", "accounting.payable_view"}})
	}
	return s.repo.ListInputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateInputInvoice(ctx context.Context, payableID string, req InputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payable_approve"}, "Bạn không có quyền upload hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeInputInvoiceRequest(ctx, payable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedPath, originalName, err := s.storeInputInvoicePDF(cleanID(payable["project_id"]), cleanID(payable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateInputInvoice(ctx, payable, body, fileMeta, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return record, nil
}

func (s *Service) InputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.payable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu vào", Record{"permissions": []string{"accounting.payable_approve"}})
	}
	return s.repo.InputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) InputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu vào")
	}
	if fmt.Sprint(invoice["status"]) == "deleted" {
		return EvidenceFileResult{}, validationError("Hoá đơn đầu vào đã bị xoá", Record{"invoice_id": invoiceID})
	}
	return s.storedFileResult(invoice, "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requirePayableAccess(ctx context.Context, payableID, actorID string, permissions []string, message string) (Record, error) {
	payableID = cleanID(payableID)
	if !validActivityUUID(payableID) {
		return nil, validationError("payable_id không hợp lệ", Record{"payable_id": payableID})
	}
	payable, err := s.repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(payable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return payable, nil
}

func (s *Service) normalizePayablePaymentScheduleRequest(payable Record, req PayablePaymentScheduleRequest) (Record, error) {
	scheduleCode := strings.TrimSpace(req.ScheduleCode)
	if scheduleCode == "" {
		return nil, validationError("Mã kỳ thanh toán là bắt buộc")
	}
	if len([]rune(scheduleCode)) > 80 {
		return nil, validationError("Mã kỳ thanh toán không được vượt quá 80 ký tự")
	}
	milestoneName := strings.TrimSpace(req.MilestoneName)
	if milestoneName == "" {
		return nil, validationError("Tên kỳ thanh toán là bắt buộc")
	}
	if len([]rune(milestoneName)) > 180 {
		return nil, validationError("Tên kỳ thanh toán không được vượt quá 180 ký tự")
	}
	scheduleType := strings.TrimSpace(req.ScheduleType)
	if scheduleType == "" {
		scheduleType = "progress"
	}
	if !statusIn(scheduleType, "advance", "progress", "final", "retention", "other") {
		return nil, validationError("Loại kỳ thanh toán không hợp lệ", Record{"schedule_type": scheduleType})
	}
	if req.PlannedAmount <= 0 {
		return nil, validationError("Số tiền kế hoạch phải lớn hơn 0")
	}
	if number(payable["gross_amount"]) > 0 && req.PlannedAmount > number(payable["gross_amount"]) {
		return nil, validationError("Số tiền kỳ thanh toán vượt giá trị phải trả", Record{"planned_amount": req.PlannedAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú kỳ thanh toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":        cleanID(payable["project_id"]),
		"payable_id":        cleanID(payable["id"]),
		"schedule_code":     scheduleCode,
		"milestone_name":    milestoneName,
		"schedule_type":     scheduleType,
		"planned_amount":    req.PlannedAmount,
		"paid_amount":       0,
		"remaining_amount":  req.PlannedAmount,
		"due_date":          dueDate,
		"status":            "draft",
		"notes":             notes,
		"payable_remaining": number(payable["remaining_amount"]),
	}, nil
}

func (s *Service) normalizePayableAdvanceRequest(payable Record, req AdvanceRequestCreateRequest) (Record, error) {
	requestCode := strings.TrimSpace(req.RequestCode)
	if requestCode == "" {
		return nil, validationError("Mã ứng trước là bắt buộc")
	}
	if len([]rune(requestCode)) > 80 {
		return nil, validationError("Mã ứng trước không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền ứng trước phải lớn hơn 0")
	}
	remaining := number(payable["remaining_amount"])
	if remaining > 0 && req.Amount > remaining {
		return nil, validationError("Số tiền ứng trước vượt số còn phải trả", Record{"amount": req.Amount, "remaining_amount": remaining})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 1000 {
		return nil, validationError("Lý do ứng trước không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú ứng trước không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":   cleanID(payable["project_id"]),
		"payable_id":   cleanID(payable["id"]),
		"request_code": requestCode,
		"vendor_name":  cleanOptionalText(payable["vendor_name"]),
		"amount":       req.Amount,
		"due_date":     dueDate,
		"reason":       reason,
		"notes":        notes,
		"status":       "draft",
	}, nil
}

func (s *Service) normalizePayablePaymentRequest(ctx context.Context, payable Record, req PayablePaymentCreateRequest) (Record, error) {
	if !statusIn(fmt.Sprint(payable["status"]), "approved", "partially_paid", "overdue") {
		return nil, validationError("Phải trả cần được duyệt trước khi tạo thanh toán", Record{"status": payable["status"]})
	}
	paymentCode := strings.TrimSpace(req.PaymentCode)
	if paymentCode == "" {
		return nil, validationError("Mã thanh toán là bắt buộc")
	}
	if len([]rune(paymentCode)) > 80 {
		return nil, validationError("Mã thanh toán không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền thanh toán phải lớn hơn 0")
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "bank_transfer"
	}
	if !statusIn(paymentMethod, "bank_transfer", "cash") {
		return nil, validationError("Phương thức thanh toán không hợp lệ", Record{"payment_method": paymentMethod})
	}
	advanceID := cleanID(req.AdvanceRequestID)
	scheduleID := cleanID(req.ScheduleID)
	if advanceID != "" && scheduleID != "" {
		return nil, validationError("Chỉ được chọn một kỳ thanh toán hoặc một ứng trước cho phiếu thanh toán")
	}
	paymentType := strings.TrimSpace(req.PaymentType)
	if paymentType == "" {
		if advanceID != "" {
			paymentType = "advance"
		} else if statusIn(fmt.Sprint(payable["vendor_type"]), "subcontractor", "labor_team") {
			paymentType = "subcontractor_payment"
		} else {
			paymentType = "supplier_payment"
		}
	}
	if !statusIn(paymentType, "supplier_payment", "subcontractor_payment", "advance", "advance_settlement", "other") {
		return nil, validationError("Loại thanh toán đầu vào không hợp lệ", Record{"payment_type": paymentType})
	}
	if advanceID != "" && !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	if scheduleID != "" && !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	inputInvoiceID := cleanID(req.InputInvoiceID)
	if inputInvoiceID != "" {
		if !validActivityUUID(inputInvoiceID) {
			return nil, validationError("input_invoice_id không hợp lệ", Record{"input_invoice_id": inputInvoiceID})
		}
		invoice, err := s.repo.GetInputInvoiceDetail(ctx, inputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn đầu vào liên kết không tồn tại", Record{"input_invoice_id": inputInvoiceID})
		}
		if cleanID(invoice["payable_id"]) != cleanID(payable["id"]) || fmt.Sprint(invoice["status"]) != "verified" {
			return nil, validationError("Hoá đơn đầu vào phải thuộc phải trả và đã xác thực", Record{"input_invoice_id": inputInvoiceID})
		}
	}
	if scheduleID != "" {
		schedule, err := s.repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thanh toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["payable_id"]) != cleanID(payable["id"]) || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_paid", "overdue") {
			return nil, validationError("Kỳ thanh toán không thuộc phải trả hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if req.Amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": req.Amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advance, err := s.repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
		if err != nil {
			return nil, validationError("Ứng trước không tồn tại", Record{"advance_request_id": advanceID})
		}
		if cleanID(advance["payable_id"]) != cleanID(payable["id"]) || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải trả hoặc chưa được duyệt", Record{"advance_request_id": advanceID})
		}
		advanceRemaining := number(advance["amount"]) - number(advance["paid_amount"])
		if req.Amount > advanceRemaining {
			return nil, validationError("Số tiền thanh toán vượt số ứng trước chưa chi", Record{"amount": req.Amount, "advance_remaining": advanceRemaining})
		}
	}
	if scheduleID == "" && advanceID == "" && req.Amount > number(payable["remaining_amount"]) {
		return nil, validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": req.Amount, "remaining_amount": payable["remaining_amount"]})
	}
	paidAt, err := normalizeOptionalDateTime(req.PaidAt, "paid_at")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú thanh toán không được vượt quá 2000 ký tự")
	}
	allocationType := "payable_payment"
	if advanceID != "" {
		allocationType = "advance_payment"
	}
	return Record{
		"project_id":          cleanID(payable["project_id"]),
		"payable_id":          cleanID(payable["id"]),
		"payment_code":        paymentCode,
		"payment_type":        paymentType,
		"payment_method":      paymentMethod,
		"amount":              req.Amount,
		"schedule_id":         scheduleID,
		"advance_request_id":  advanceID,
		"input_invoice_id":    inputInvoiceID,
		"paid_at":             paidAt,
		"notes":               notes,
		"allocation_type":     allocationType,
		"target_type":         "payable",
		"target_id":           cleanID(payable["id"]),
		"status":              "draft",
		"payable_vendor_type": fmt.Sprint(payable["vendor_type"]),
	}, nil
}

func (s *Service) normalizeAdvanceSettlementRequest(ctx context.Context, advance Record, req AdvanceSettlementRequest) (Record, error) {
	if !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
		return nil, validationError("Ứng trước chưa sẵn sàng quyết toán", Record{"status": advance["status"]})
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền quyết toán phải lớn hơn 0")
	}
	remaining := number(advance["amount"]) - number(advance["settled_amount"])
	if req.Amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": req.Amount, "remaining": remaining})
	}
	inputInvoiceID := cleanID(req.InputInvoiceID)
	if inputInvoiceID != "" {
		if !validActivityUUID(inputInvoiceID) {
			return nil, validationError("input_invoice_id không hợp lệ", Record{"input_invoice_id": inputInvoiceID})
		}
		invoice, err := s.repo.GetInputInvoiceDetail(ctx, inputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn quyết toán không tồn tại", Record{"input_invoice_id": inputInvoiceID})
		}
		if cleanID(invoice["payable_id"]) != cleanID(advance["payable_id"]) || fmt.Sprint(invoice["status"]) != "verified" {
			return nil, validationError("Hoá đơn quyết toán phải thuộc phải trả và đã xác thực", Record{"input_invoice_id": inputInvoiceID})
		}
	}
	scheduleID := cleanID(req.ScheduleID)
	if scheduleID != "" {
		if !validActivityUUID(scheduleID) {
			return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
		}
		schedule, err := s.repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thanh toán quyết toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["payable_id"]) != cleanID(advance["payable_id"]) {
			return nil, validationError("Kỳ thanh toán quyết toán không thuộc phải trả", Record{"schedule_id": scheduleID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, validationError("Ghi chú quyết toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"amount":           req.Amount,
		"input_invoice_id": inputInvoiceID,
		"schedule_id":      scheduleID,
		"comment":          comment,
	}, nil
}

func (s *Service) ReceivablePaymentSchedules(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem lịch thu đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivablePaymentSchedules(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivablePaymentSchedule(ctx context.Context, receivableID string, req PayablePaymentScheduleRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create", "accounting.receivable_approve"}, "Bạn không có quyền tạo kỳ thu đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivablePaymentScheduleRequest(receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivablePaymentSchedule(ctx, receivable, body, actorID, actorName)
}

func (s *Service) ReceivablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	scheduleID = cleanID(scheduleID)
	if !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	return s.repo.ReceivablePaymentScheduleAction(ctx, scheduleID, action, req, actorID, actorName)
}

func (s *Service) ReceivableAdvanceReceipts(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem ứng trước đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivableAdvanceReceipts(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivableAdvanceReceipt(ctx context.Context, receivableID string, req ReceivableAdvanceReceiptCreateRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create", "accounting.receivable_approve"}, "Bạn không có quyền tạo ứng trước đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivableAdvanceReceiptRequest(receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivableAdvanceReceipt(ctx, receivable, body, actorID, actorName)
}

func (s *Service) ReceivableAdvanceReceiptAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	return s.repo.ReceivableAdvanceReceiptAction(ctx, advanceID, action, req, actorID, actorName)
}

func (s *Service) SettleReceivableAdvanceReceipt(ctx context.Context, advanceID string, req ReceivableAdvanceSettlementRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	advance, err := s.repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivableAdvanceSettlementRequest(ctx, advance, req)
	if err != nil {
		return nil, err
	}
	return s.repo.SettleReceivableAdvanceReceipt(ctx, advanceID, body, actorID, actorName)
}

func (s *Service) ReceivablePayments(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view", "accounting.payment_create", "accounting.payment_approve"}, "Bạn không có quyền xem phiếu thu đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivablePayments(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivablePayment(ctx context.Context, receivableID string, req ReceivableReceiptCreateRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create"}, "Bạn không có quyền tạo phiếu thu đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivablePaymentRequest(ctx, receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivablePayment(ctx, receivable, body, actorID, actorName)
}

func (s *Service) normalizeReceivablePaymentScheduleRequest(receivable Record, req PayablePaymentScheduleRequest) (Record, error) {
	scheduleCode := strings.TrimSpace(req.ScheduleCode)
	if scheduleCode == "" {
		return nil, validationError("Mã kỳ thu là bắt buộc")
	}
	if len([]rune(scheduleCode)) > 80 {
		return nil, validationError("Mã kỳ thu không được vượt quá 80 ký tự")
	}
	milestoneName := strings.TrimSpace(req.MilestoneName)
	if milestoneName == "" {
		return nil, validationError("Tên kỳ thu là bắt buộc")
	}
	if len([]rune(milestoneName)) > 180 {
		return nil, validationError("Tên kỳ thu không được vượt quá 180 ký tự")
	}
	scheduleType := strings.TrimSpace(req.ScheduleType)
	if scheduleType == "" {
		scheduleType = "progress"
	}
	if !statusIn(scheduleType, "advance", "progress", "final", "retention", "other") {
		return nil, validationError("Loại kỳ thu không hợp lệ", Record{"schedule_type": scheduleType})
	}
	if req.PlannedAmount <= 0 {
		return nil, validationError("Số tiền kế hoạch phải lớn hơn 0")
	}
	if number(receivable["gross_amount"]) > 0 && req.PlannedAmount > number(receivable["gross_amount"]) {
		return nil, validationError("Số tiền kỳ thu vượt giá trị phải thu", Record{"planned_amount": req.PlannedAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú kỳ thu không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":              cleanID(receivable["project_id"]),
		"receivable_id":           cleanID(receivable["id"]),
		"schedule_code":           scheduleCode,
		"milestone_name":          milestoneName,
		"schedule_type":           scheduleType,
		"planned_amount":          req.PlannedAmount,
		"received_amount":         0,
		"remaining_amount":        req.PlannedAmount,
		"due_date":                dueDate,
		"status":                  "draft",
		"notes":                   notes,
		"receivable_remaining":    number(receivable["remaining_amount"]),
		"receivable_gross_amount": number(receivable["gross_amount"]),
	}, nil
}

func (s *Service) normalizeReceivableAdvanceReceiptRequest(receivable Record, req ReceivableAdvanceReceiptCreateRequest) (Record, error) {
	receiptCode := strings.TrimSpace(req.ReceiptCode)
	if receiptCode == "" {
		return nil, validationError("Mã ứng trước là bắt buộc")
	}
	if len([]rune(receiptCode)) > 80 {
		return nil, validationError("Mã ứng trước không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền ứng trước phải lớn hơn 0")
	}
	remaining := number(receivable["remaining_amount"])
	if remaining > 0 && req.Amount > remaining {
		return nil, validationError("Số tiền ứng trước vượt số còn phải thu", Record{"amount": req.Amount, "remaining_amount": remaining})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 1000 {
		return nil, validationError("Lý do ứng trước không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú ứng trước không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":    cleanID(receivable["project_id"]),
		"receivable_id": cleanID(receivable["id"]),
		"receipt_code":  receiptCode,
		"client_name":   cleanOptionalText(receivable["client_name"]),
		"amount":        req.Amount,
		"due_date":      dueDate,
		"reason":        reason,
		"notes":         notes,
		"status":        "draft",
	}, nil
}

func (s *Service) normalizeReceivablePaymentRequest(ctx context.Context, receivable Record, req ReceivableReceiptCreateRequest) (Record, error) {
	if !statusIn(fmt.Sprint(receivable["status"]), "approved", "partially_received", "overdue") {
		return nil, validationError("Phải thu cần được duyệt trước khi tạo phiếu thu", Record{"status": receivable["status"]})
	}
	paymentCode := strings.TrimSpace(req.PaymentCode)
	if paymentCode == "" {
		return nil, validationError("Mã phiếu thu là bắt buộc")
	}
	if len([]rune(paymentCode)) > 80 {
		return nil, validationError("Mã phiếu thu không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền thu phải lớn hơn 0")
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "bank_transfer"
	}
	if !statusIn(paymentMethod, "bank_transfer", "cash") {
		return nil, validationError("Phương thức thu không hợp lệ", Record{"payment_method": paymentMethod})
	}
	advanceID := cleanID(req.AdvanceReceiptID)
	scheduleID := cleanID(req.ScheduleID)
	if advanceID != "" && scheduleID != "" {
		return nil, validationError("Chỉ được chọn một kỳ thu hoặc một ứng trước cho phiếu thu")
	}
	paymentType := strings.TrimSpace(req.PaymentType)
	if paymentType == "" {
		paymentType = "client_receipt"
	}
	if !statusIn(paymentType, "client_receipt", "advance", "advance_settlement", "inbound", "other") {
		return nil, validationError("Loại phiếu thu đầu ra không hợp lệ", Record{"payment_type": paymentType})
	}
	if advanceID != "" && !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	if scheduleID != "" && !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	outputInvoiceID := cleanID(req.OutputInvoiceID)
	if outputInvoiceID != "" {
		if !validActivityUUID(outputInvoiceID) {
			return nil, validationError("output_invoice_id không hợp lệ", Record{"output_invoice_id": outputInvoiceID})
		}
		invoice, err := s.repo.GetOutputInvoiceDetail(ctx, outputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn đầu ra liên kết không tồn tại", Record{"output_invoice_id": outputInvoiceID})
		}
		if cleanID(invoice["receivable_id"]) != cleanID(receivable["id"]) || fmt.Sprint(invoice["status"]) != "issued" {
			return nil, validationError("Hoá đơn đầu ra phải thuộc phải thu và đã phát hành", Record{"output_invoice_id": outputInvoiceID})
		}
	}
	if scheduleID != "" {
		schedule, err := s.repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thu không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["receivable_id"]) != cleanID(receivable["id"]) || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_received", "overdue") {
			return nil, validationError("Kỳ thu không thuộc phải thu hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if req.Amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": req.Amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advance, err := s.repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
		if err != nil {
			return nil, validationError("Ứng trước đầu ra không tồn tại", Record{"advance_receipt_id": advanceID})
		}
		if cleanID(advance["receivable_id"]) != cleanID(receivable["id"]) || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải thu hoặc chưa được duyệt", Record{"advance_receipt_id": advanceID})
		}
		advanceRemaining := number(advance["amount"]) - number(advance["received_amount"])
		if req.Amount > advanceRemaining {
			return nil, validationError("Số tiền thu vượt số ứng trước chưa ghi nhận tiền", Record{"amount": req.Amount, "advance_remaining": advanceRemaining})
		}
	}
	if scheduleID == "" && advanceID == "" && req.Amount > number(receivable["remaining_amount"]) {
		return nil, validationError("Số tiền thu vượt số còn phải thu", Record{"amount": req.Amount, "remaining_amount": receivable["remaining_amount"]})
	}
	paidAt, err := normalizeOptionalDateTime(req.PaidAt, "paid_at")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú phiếu thu không được vượt quá 2000 ký tự")
	}
	allocationType := "receivable_receipt"
	if advanceID != "" {
		allocationType = "advance_receipt"
	}
	return Record{
		"project_id":         cleanID(receivable["project_id"]),
		"receivable_id":      cleanID(receivable["id"]),
		"payment_code":       paymentCode,
		"payment_type":       paymentType,
		"payment_method":     paymentMethod,
		"amount":             req.Amount,
		"schedule_id":        scheduleID,
		"advance_receipt_id": advanceID,
		"output_invoice_id":  outputInvoiceID,
		"paid_at":            paidAt,
		"notes":              notes,
		"allocation_type":    allocationType,
		"target_type":        "receivable",
		"target_id":          cleanID(receivable["id"]),
		"status":             "draft",
	}, nil
}

func (s *Service) normalizeReceivableAdvanceSettlementRequest(ctx context.Context, advance Record, req ReceivableAdvanceSettlementRequest) (Record, error) {
	if !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
		return nil, validationError("Ứng trước đầu ra chưa sẵn sàng quyết toán", Record{"status": advance["status"]})
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền quyết toán phải lớn hơn 0")
	}
	remaining := number(advance["amount"]) - number(advance["settled_amount"])
	if req.Amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": req.Amount, "remaining": remaining})
	}
	outputInvoiceID := cleanID(req.OutputInvoiceID)
	if outputInvoiceID != "" {
		if !validActivityUUID(outputInvoiceID) {
			return nil, validationError("output_invoice_id không hợp lệ", Record{"output_invoice_id": outputInvoiceID})
		}
		invoice, err := s.repo.GetOutputInvoiceDetail(ctx, outputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn quyết toán không tồn tại", Record{"output_invoice_id": outputInvoiceID})
		}
		if cleanID(invoice["receivable_id"]) != cleanID(advance["receivable_id"]) || fmt.Sprint(invoice["status"]) != "issued" {
			return nil, validationError("Hoá đơn quyết toán phải thuộc phải thu và đã phát hành", Record{"output_invoice_id": outputInvoiceID})
		}
	}
	scheduleID := cleanID(req.ScheduleID)
	if scheduleID != "" {
		if !validActivityUUID(scheduleID) {
			return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
		}
		schedule, err := s.repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thu quyết toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["receivable_id"]) != cleanID(advance["receivable_id"]) {
			return nil, validationError("Kỳ thu quyết toán không thuộc phải thu", Record{"schedule_id": scheduleID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, validationError("Ghi chú quyết toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"amount":            req.Amount,
		"output_invoice_id": outputInvoiceID,
		"schedule_id":       scheduleID,
		"comment":           comment,
	}, nil
}

func normalizeOptionalDate(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", validationError("Ngày không hợp lệ", Record{field: value})
	}
	return value, nil
}

func normalizeOptionalDateTime(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return value, nil
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return value, nil
	}
	return "", validationError("Thời điểm không hợp lệ", Record{field: value})
}

func (s *Service) normalizeInputInvoiceRequest(ctx context.Context, payable Record, req InputInvoiceRequest) (Record, error) {
	projectID := cleanID(payable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	vendorPartnerID := cleanID(req.VendorPartnerID)
	if vendorPartnerID == "" {
		vendorPartnerID = cleanID(payable["vendor_partner_id"])
	}
	vendorName := strings.TrimSpace(req.VendorName)
	vendorTaxCode := strings.TrimSpace(req.VendorTaxCode)
	if vendorPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, vendorPartnerID, vendorPartnerTypes())
		if err != nil {
			return nil, err
		}
		vendorName = cleanOptionalText(partner["name"])
		vendorTaxCode = cleanOptionalText(partner["tax_code"])
	}
	if vendorName == "" {
		vendorName = cleanOptionalText(payable["vendor_name"])
	}
	if vendorName == "" {
		return nil, validationError("Tên nhà cung cấp trên hoá đơn là bắt buộc")
	}
	if len([]rune(vendorName)) > 240 {
		return nil, validationError("Tên nhà cung cấp trên hoá đơn không được vượt quá 240 ký tự")
	}
	if len([]rune(vendorTaxCode)) > 80 {
		return nil, validationError("Mã số thuế nhà cung cấp không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if procurementRequestID != "" {
		if !validActivityUUID(procurementRequestID) {
			return nil, validationError("procurement_request_id không hợp lệ", Record{"procurement_request_id": procurementRequestID})
		}
		procurement, err := s.repo.GetByID(ctx, "procurement_requests", procurementRequestID)
		if err != nil {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không tồn tại", Record{"procurement_request_id": procurementRequestID})
		}
		if cleanID(procurement["project_id"]) != projectID || fmt.Sprint(procurement["status"]) == "cancelled" {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không thuộc công trình", Record{"procurement_request_id": procurementRequestID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Tài liệu liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Tài liệu liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":             projectID,
		"payable_id":             cleanID(payable["id"]),
		"procurement_request_id": procurementRequestID,
		"document_id":            documentID,
		"invoice_number":         invoiceNumber,
		"invoice_symbol":         invoiceSymbol,
		"invoice_date":           invoiceDate,
		"vendor_partner_id":      vendorPartnerID,
		"vendor_name":            vendorName,
		"vendor_tax_code":        vendorTaxCode,
		"amount_before_vat":      req.AmountBeforeVAT,
		"vat_amount":             req.VATAmount,
		"total_amount":           totalAmount,
		"notes":                  notes,
	}, nil
}

func (s *Service) ReceivableOutputInvoices(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListOutputInvoicesForReceivable(ctx, cleanID(receivable["id"]))
}

func (s *Service) ReceivableReconciliation(ctx context.Context, receivableID, actorID string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem đối soát công nợ phải thu")
	if err != nil {
		return nil, err
	}
	return s.repo.ReceivableReconciliation(ctx, cleanID(receivable["id"]))
}

func (s *Service) ProjectOutputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu ra", Record{"permissions": []string{"accounting.view", "accounting.receivable_view"}})
	}
	return s.repo.ListOutputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateOutputInvoice(ctx context.Context, receivableID string, req OutputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.receivable_approve"}, "Bạn không có quyền upload hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeOutputInvoiceRequest(ctx, receivable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedPath, originalName, err := s.storeOutputInvoicePDF(cleanID(receivable["project_id"]), cleanID(receivable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateOutputInvoice(ctx, receivable, body, fileMeta, actorID, actorName)
	if err != nil {
		_ = os.Remove(storedPath)
		return nil, err
	}
	return record, nil
}

func (s *Service) OutputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.receivable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu ra", Record{"permissions": []string{"accounting.receivable_approve"}})
	}
	return s.repo.OutputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) OutputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu ra")
	}
	return s.storedFileResult(invoice, "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requireReceivableAccess(ctx context.Context, receivableID, actorID string, permissions []string, message string) (Record, error) {
	receivableID = cleanID(receivableID)
	if !validActivityUUID(receivableID) {
		return nil, validationError("receivable_id không hợp lệ", Record{"receivable_id": receivableID})
	}
	receivable, err := s.repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(receivable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return receivable, nil
}

func (s *Service) normalizeOutputInvoiceRequest(ctx context.Context, receivable Record, req OutputInvoiceRequest) (Record, error) {
	projectID := cleanID(receivable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	clientPartnerID := cleanID(req.ClientPartnerID)
	if clientPartnerID == "" {
		clientPartnerID = cleanID(receivable["client_partner_id"])
	}
	clientName := strings.TrimSpace(req.ClientName)
	clientTaxCode := strings.TrimSpace(req.ClientTaxCode)
	if clientPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, clientPartnerID, []string{"client"})
		if err != nil {
			return nil, err
		}
		clientName = cleanOptionalText(partner["name"])
		clientTaxCode = cleanOptionalText(partner["tax_code"])
	}
	if clientName == "" {
		clientName = cleanOptionalText(receivable["client_name"])
	}
	if clientName == "" {
		return nil, validationError("Tên khách hàng trên hoá đơn là bắt buộc")
	}
	if len([]rune(clientName)) > 240 {
		return nil, validationError("Tên khách hàng trên hoá đơn không được vượt quá 240 ký tự")
	}
	if len([]rune(clientTaxCode)) > 80 {
		return nil, validationError("Mã số thuế khách hàng không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	paymentID := cleanID(req.PaymentID)
	if paymentID != "" {
		if !validActivityUUID(paymentID) {
			return nil, validationError("payment_id không hợp lệ", Record{"payment_id": paymentID})
		}
		payment, err := s.repo.GetByID(ctx, "payments", paymentID)
		if err != nil {
			return nil, validationError("Phiếu thu liên kết hoá đơn không tồn tại", Record{"payment_id": paymentID})
		}
		if cleanID(payment["project_id"]) != projectID || fmt.Sprint(payment["status"]) == "cancelled" || fmt.Sprint(payment["payment_type"]) != "client_receipt" {
			return nil, validationError("Phiếu thu liên kết hoá đơn không thuộc công trình hoặc không phải phiếu thu khách hàng", Record{"payment_id": paymentID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Tài liệu liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Tài liệu liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":        projectID,
		"receivable_id":     cleanID(receivable["id"]),
		"payment_id":        paymentID,
		"document_id":       documentID,
		"invoice_number":    invoiceNumber,
		"invoice_symbol":    invoiceSymbol,
		"invoice_date":      invoiceDate,
		"client_partner_id": clientPartnerID,
		"client_name":       clientName,
		"client_tax_code":   clientTaxCode,
		"amount_before_vat": req.AmountBeforeVAT,
		"vat_amount":        req.VATAmount,
		"total_amount":      totalAmount,
		"notes":             notes,
	}, nil
}

func (s *Service) PayableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PayableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) ReceivableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.ReceivableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) CreatePayment(ctx context.Context, req PaymentCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	paymentCode := strings.TrimSpace(req.PaymentCode)
	paymentType := strings.TrimSpace(req.PaymentType)
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	amount := req.Amount
	if !validActivityUUID(projectID) || paymentCode == "" || amount <= 0 {
		return nil, validationError("Phiếu thu/chi cần công trình, mã và số tiền lớn hơn 0", Record{"project_id": projectID, "payment_code": paymentCode, "amount": amount})
	}
	if !statusIn(paymentType, "outbound", "inbound", "other") {
		return nil, validationError("Phiếu gắn công nợ phải được tạo tại Phải trả/Phải thu để giữ đúng phân bổ", Record{"payment_type": paymentType, "allowed_types": []string{"outbound", "inbound", "other"}})
	}
	if paymentMethod == "" {
		paymentMethod = "bank_transfer"
	}
	if !statusIn(paymentMethod, "bank_transfer", "cash") {
		return nil, validationError("Phương thức thanh toán không hợp lệ", Record{"payment_method": paymentMethod})
	}
	targetType := strings.TrimSpace(req.TargetType)
	targetID := cleanID(req.TargetID)
	if (targetType == "") != (targetID == "") || (targetID != "" && !validActivityUUID(targetID)) {
		return nil, validationError("Nguồn liên kết phiếu thu/chi không hợp lệ", Record{"target_type": targetType, "target_id": targetID})
	}
	normalized := Record{
		"project_id": projectID, "payment_code": paymentCode, "payment_type": paymentType,
		"target_type": targetType, "target_id": targetID,
		"amount": amount, "payment_method": paymentMethod, "status": "draft", "created_by": actorID,
	}
	row, err := s.repo.CreatePayment(ctx, normalized)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(ctx, cleanID(row["project_id"]), actorID, actorName, "payment_created", "payment", cleanID(row["id"]), fmt.Sprintf("Tạo phiếu thanh toán %s", row["payment_code"]), Record{"status": row["status"], "payment_type": row["payment_type"], "amount": row["amount"]})
	return row, nil
}

func (s *Service) PaymentAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PaymentAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) Dashboard(ctx context.Context, actorID string) (Record, error) {
	return s.repo.DashboardReport(ctx, actorID)
}

func (s *Service) ReportOptions(ctx context.Context, actorID string) (Record, error) {
	projects, err := s.repo.ReportProjectOptions(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return Record{"projects": projects}, nil
}

func (s *Service) Report(ctx context.Context, kind string, req OperationalReportRequest, actorID string) ([]Record, error) {
	req.ProjectID = cleanID(req.ProjectID)
	if req.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, req.ProjectID, operationalReportPermissions(kind))
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo của công trình này", Record{"permissions": operationalReportPermissions(kind)})
		}
	}
	return s.repo.Report(ctx, kind, actorID, req.ProjectID)
}

func operationalReportPermissions(kind string) []string {
	if kind == "accounting" {
		return []string{"report.finance.view", "accounting.view"}
	}
	return []string{"report.view"}
}

func (s *Service) CashflowReport(ctx context.Context, req CashflowReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeCashflowReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem dòng tiền của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.CashflowReport(ctx, normalized, actorID)
}

func (s *Service) TaxReport(ctx context.Context, req TaxReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeTaxReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo thuế của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.TaxReport(ctx, normalized, actorID)
}

func (s *Service) ProfitReport(ctx context.Context, req ProfitReportRequest, actorID string) (Record, error) {
	normalized, err := normalizeProfitReportRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ProjectID != "" {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, normalized.ProjectID, []string{"report.finance.view", "accounting.view"})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem báo cáo lợi nhuận của công trình này", Record{"permissions": []string{"report.finance.view", "accounting.view"}})
		}
	}
	return s.repo.ProfitReport(ctx, normalized, actorID)
}

func normalizeCashflowReportRequest(req CashflowReportRequest) (CashflowReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return CashflowReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return CashflowReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return CashflowReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "day"
	}
	if !statusIn(groupBy, "day", "week", "month") {
		return CashflowReportRequest{}, validationError("Nhóm thời gian dòng tiền không hợp lệ", Record{"group_by": groupBy})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "approved"
	}
	if !statusIn(status, "approved", "all") {
		return CashflowReportRequest{}, validationError("Bộ lọc trạng thái dòng tiền không hợp lệ", Record{"status": status})
	}
	return CashflowReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Status:    status,
	}, nil
}

func normalizeProfitReportRequest(req ProfitReportRequest) (ProfitReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return ProfitReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return ProfitReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return ProfitReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "month"
	}
	if !statusIn(groupBy, "month", "quarter") {
		return ProfitReportRequest{}, validationError("Nhóm kỳ báo cáo lợi nhuận không hợp lệ", Record{"group_by": groupBy})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "posted"
	}
	if !statusIn(status, "posted", "all") {
		return ProfitReportRequest{}, validationError("Bộ lọc trạng thái báo cáo lợi nhuận không hợp lệ", Record{"status": status})
	}
	return ProfitReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Status:    status,
	}, nil
}

func normalizeTaxReportRequest(req TaxReportRequest) (TaxReportRequest, error) {
	now := time.Now()
	from := strings.TrimSpace(req.FromDate)
	to := strings.TrimSpace(req.ToDate)
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return TaxReportRequest{}, validationError("Ngày bắt đầu không hợp lệ", Record{"from": from})
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return TaxReportRequest{}, validationError("Ngày kết thúc không hợp lệ", Record{"to": to})
	}
	if toDate.Before(fromDate) {
		return TaxReportRequest{}, validationError("Ngày kết thúc phải sau ngày bắt đầu", Record{"from": from, "to": to})
	}
	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "month"
	}
	if !statusIn(groupBy, "month", "quarter") {
		return TaxReportRequest{}, validationError("Nhóm kỳ báo cáo thuế không hợp lệ", Record{"group_by": groupBy})
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "all"
	}
	if !statusIn(source, "all", "input", "output") {
		return TaxReportRequest{}, validationError("Nguồn hoá đơn báo cáo thuế không hợp lệ", Record{"source": source})
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "posted"
	}
	if !statusIn(status, "posted", "all") {
		return TaxReportRequest{}, validationError("Bộ lọc trạng thái báo cáo thuế không hợp lệ", Record{"status": status})
	}
	return TaxReportRequest{
		ProjectID: cleanID(req.ProjectID),
		FromDate:  from,
		ToDate:    to,
		GroupBy:   groupBy,
		Source:    source,
		Status:    status,
	}, nil
}

func (s *Service) AuditLogs(ctx context.Context, projectID, actorID string, filter AuditLogFilter) (Record, error) {
	filter, err := normalizeAuditLogFilter(filter)
	if err != nil {
		return nil, err
	}

	fromSQL := "FROM audit_logs al JOIN projects p ON p.id=al.project_id"
	scopeConditions := make([]string, 0, 3)
	scopeArgs := make([]any, 0, 3)
	if projectID != "" {
		scopeArgs = append(scopeArgs, projectID)
		scopeConditions = append(scopeConditions, "p.id=$1")
	} else {
		scopeSQL, accessArgs, accessErr := s.repo.ProjectAccessFilter(ctx, actorID, "p", 1)
		if accessErr != nil {
			return nil, accessErr
		}
		scopeArgs = append(scopeArgs, accessArgs...)
		scopeConditions = append(scopeConditions, "p.status <> 'cancelled'", scopeSQL)
	}

	conditions := append([]string(nil), scopeConditions...)
	args := append([]any(nil), scopeArgs...)
	addCondition := func(format string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}
	if projectID == "" && filter.ProjectID != "" {
		addCondition("p.id=$%d", filter.ProjectID)
	}
	if filter.EventType != "" {
		addCondition("al.event_type=$%d", filter.EventType)
	}
	if filter.TargetType != "" {
		addCondition("al.target_type=$%d", filter.TargetType)
	}
	if filter.ActorName != "" {
		addCondition("LOWER(COALESCE(al.actor_name,''))=LOWER($%d)", filter.ActorName)
	}
	if filter.DateFrom != "" {
		addCondition("al.created_at >= $%d::date", filter.DateFrom)
	}
	if filter.DateTo != "" {
		addCondition("al.created_at < ($%d::date + INTERVAL '1 day')", filter.DateTo)
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			al.description ILIKE $%[1]d OR al.event_type ILIKE $%[1]d OR
			COALESCE(al.actor_name,'') ILIKE $%[1]d OR COALESCE(al.target_type,'') ILIKE $%[1]d OR
			COALESCE(al.target_id::text,'') ILIKE $%[1]d OR al.id::text ILIKE $%[1]d OR
			p.code ILIKE $%[1]d OR p.name ILIKE $%[1]d
		)`, placeholder))
	}

	whereSQL := strings.Join(conditions, " AND ")
	countRow, err := s.repo.QueryOne(ctx, fmt.Sprintf("SELECT COUNT(*) AS total %s WHERE %s", fromSQL, whereSQL), args...)
	if err != nil {
		return nil, err
	}
	total := int(number(countRow["total"]))
	offset := (filter.Page - 1) * filter.PageSize
	itemArgs := append([]any(nil), args...)
	itemArgs = append(itemArgs, filter.PageSize, offset)
	items, err := s.repo.Query(ctx, fmt.Sprintf(`
		SELECT al.*, p.code AS project_code, p.name AS project_name
		%s
		WHERE %s
		ORDER BY al.created_at DESC, al.id DESC
		LIMIT $%d OFFSET $%d
	`, fromSQL, whereSQL, len(itemArgs)-1, len(itemArgs)), itemArgs...)
	if err != nil {
		return nil, err
	}

	options, err := s.auditLogOptions(ctx, fromSQL, scopeConditions, scopeArgs, projectID == "")
	if err != nil {
		return nil, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.PageSize)))
	}
	return Record{
		"items":       items,
		"total":       total,
		"page":        filter.Page,
		"page_size":   filter.PageSize,
		"total_pages": totalPages,
		"options":     options,
	}, nil
}

func (s *Service) auditLogOptions(ctx context.Context, fromSQL string, conditions []string, args []any, includeProjects bool) (Record, error) {
	row, err := s.repo.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(jsonb_agg(DISTINCT al.event_type) FILTER (WHERE al.event_type <> ''), '[]'::jsonb) AS event_types,
			COALESCE(jsonb_agg(DISTINCT al.target_type) FILTER (WHERE COALESCE(al.target_type,'') <> ''), '[]'::jsonb) AS target_types,
			COALESCE(jsonb_agg(DISTINCT al.actor_name) FILTER (WHERE COALESCE(al.actor_name,'') <> ''), '[]'::jsonb) AS actors
		%s
		WHERE %s
	`, fromSQL, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	options := Record{
		"event_types":  row["event_types"],
		"target_types": row["target_types"],
		"actors":       row["actors"],
		"projects":     []Record{},
	}
	if !includeProjects {
		return options, nil
	}
	projects, err := s.repo.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT p.id, p.code, p.name
		%s
		WHERE %s
		ORDER BY p.code, p.name
	`, fromSQL, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	options["projects"] = projects
	return options, nil
}

func normalizeAuditLogFilter(filter AuditLogFilter) (AuditLogFilter, error) {
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("Công trình lọc không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.EventType = strings.ToLower(strings.TrimSpace(filter.EventType))
	filter.TargetType = strings.ToLower(strings.TrimSpace(filter.TargetType))
	filter.ActorName = strings.TrimSpace(filter.ActorName)
	filter.Search = strings.TrimSpace(filter.Search)
	for field, value := range map[string]string{
		"event_type":  filter.EventType,
		"target_type": filter.TargetType,
		"actor_name":  filter.ActorName,
		"search":      filter.Search,
	} {
		if len([]rune(value)) > 200 {
			return filter, validationError("Bộ lọc audit quá dài", Record{"field": field, "max_length": 200})
		}
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	var dateFrom, dateTo time.Time
	if filter.DateFrom != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return filter, validationError("Ngày bắt đầu audit không hợp lệ", Record{"date_from": filter.DateFrom})
		}
		dateFrom = parsed
	}
	if filter.DateTo != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return filter, validationError("Ngày kết thúc audit không hợp lệ", Record{"date_to": filter.DateTo})
		}
		dateTo = parsed
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		return filter, validationError("Khoảng thời gian audit không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 25
	}
	return filter, nil
}

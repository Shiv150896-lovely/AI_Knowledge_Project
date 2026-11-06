package handler

import (
	"net/http"
	"os"

	"ai-knowledge-assistant/Repository"
	"ai-knowledge-assistant/Utils"
	"ai-knowledge-assistant/config"
	"ai-knowledge-assistant/middleware"
	"ai-knowledge-assistant/models"
	"ai-knowledge-assistant/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Repo            repository.Repository
	DocumentService service.DocumentService
	Ask             service.AskService
	Config          config.Config
}

func ok(c *gin.Context, status int, data any) { c.JSON(status, gin.H{"data": data}) }
func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
func (h Handler) Register(c *gin.Context) {
	var r models.RegisterRequest
	if c.ShouldBindJSON(&r) != nil {
		fail(c, 400, "validation_error", "organization name, valid email, and password of at least 8 characters are required")
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if e != nil {
		fail(c, 500, "internal_error", "could not create account")
		return
	}
	token, e := utils.NewToken()
	if e != nil {
		fail(c, 500, "internal_error", "could not create assistant")
		return
	}
	org, user, e := h.Repo.Register(c, r.OrganizationName, r.Email, string(hash), token, utils.HashToken(token), token[:8])
	if e != nil {
		fail(c, 409, "account_exists", "organization account could not be created with this email")
		return
	}
	jwt, e := utils.SignJWT(user, org, h.Config.JWTSecret)
	if e != nil {
		fail(c, 500, "internal_error", "could not sign access token")
		return
	}
	ok(c, 201, gin.H{"organization": gin.H{"id": org, "name": r.OrganizationName}, "access_token": jwt, "assistant": assistantData(h.Config, token)})
}
func (h Handler) Login(c *gin.Context) {
	var r models.LoginRequest
	if c.ShouldBindJSON(&r) != nil {
		fail(c, 400, "validation_error", "valid email and password are required")
		return
	}
	user, org, hash, name, e := h.Repo.Login(c, r.Email)
	if e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.Password)) != nil {
		fail(c, 401, "invalid_credentials", "invalid email or password")
		return
	}
	jwt, e := utils.SignJWT(user, org, h.Config.JWTSecret)
	if e != nil {
		fail(c, 500, "internal_error", "could not sign access token")
		return
	}
	ok(c, 200, gin.H{"access_token": jwt, "organization": gin.H{"id": org, "name": name}})
}
func (h Handler) Logout(c *gin.Context) {
	ok(c, 200, gin.H{"logged_out": true, "message": "remove the bearer token from the client"})
}
func (h Handler) Organization(c *gin.Context) {
	x, e := h.Repo.Organization(c, middleware.OrganizationID(c))
	if e != nil {
		fail(c, 500, "internal_error", "could not load organization")
		return
	}
	ok(c, 200, x)
}
func (h Handler) Documents(c *gin.Context) {
	org := middleware.OrganizationID(c)
	xs, e := h.Repo.Documents(c, org)
	if e != nil {
		fail(c, 500, "internal_error", "could not list documents")
		return
	}
	ok(c, 200, gin.H{"limit": 10, "count": len(xs), "documents": xs})
}
func (h Handler) Upload(c *gin.Context) {
	f, e := c.FormFile("file")
	if e != nil {
		fail(c, 400, "invalid_pdf", "PDF file is required")
		return
	}
	x, e := h.DocumentService.Upload(c, middleware.OrganizationID(c), f)
	if e != nil {
		msg := e.Error()
		status, code := 400, "invalid_pdf"
		if msg == "document limit reached" {
			status, code = 409, "document_limit"
		}
		if len(msg) >= 11 && msg[:11] == "PDF exceeds" {
			if msg == "PDF exceeds 10 MB" {
				status, code = 413, "file_too_large"
			} else {
				status, code = 422, "page_limit"
			}
		}
		fail(c, status, code, msg)
		return
	}
	ok(c, 201, x)
}
func (h Handler) DeleteDocument(c *gin.Context) {
	e := h.DocumentService.Delete(c, middleware.OrganizationID(c), c.Param("documentId"))
	if e != nil {
		if repository.NotFound(e) {
			fail(c, 404, "not_found", "document not found")
			return
		}
		if !os.IsNotExist(e) {
			fail(c, 500, "delete_failed", "could not delete document")
			return
		}
	}
	ok(c, 200, gin.H{"id": c.Param("documentId"), "deleted": true})
}
func assistantData(cfg config.Config, token string) gin.H {
	u := cfg.AppBaseURL + "/assistant/" + token
	return gin.H{"url": u, "embed_code": "<iframe src=\"" + u + "\" width=\"400\" height=\"600\" title=\"AI Assistant\"></iframe>"}
}
func (h Handler) Assistant(c *gin.Context) {
	_, token, e := h.Repo.Assistant(c, middleware.OrganizationID(c))
	if e != nil {
		fail(c, 500, "internal_error", "could not load assistant")
		return
	}
	ok(c, 200, assistantData(h.Config, token))
}
func (h Handler) Regenerate(c *gin.Context) {
	token, e := utils.NewToken()
	if e != nil {
		fail(c, 500, "internal_error", "could not regenerate assistant")
		return
	}
	if e = h.Repo.RotateAssistant(c, middleware.OrganizationID(c), token, utils.HashToken(token), token[:8]); e != nil {
		fail(c, 500, "internal_error", "could not regenerate assistant")
		return
	}
	ok(c, 200, assistantData(h.Config, token))
}
func (h Handler) Usage(c *gin.Context) {
	x, e := h.Repo.Usage(c, middleware.OrganizationID(c))
	if e != nil {
		fail(c, 500, "internal_error", "could not load usage")
		return
	}
	ok(c, 200, x)
}
func (h Handler) AskQuestion(c *gin.Context) {
	var r models.AskRequest
	if c.ShouldBindJSON(&r) != nil {
		fail(c, 400, "empty_question", "question is required")
		return
	}
	data, status, e := h.Ask.Ask(c, c.GetHeader("X-Assistant-Token"), r.Question)
	if e != nil {
		code := "ask_failed"
		if status == 401 {
			code = "invalid_assistant_token"
		}
		if status == 400 {
			code = "empty_question"
		}
		if status == 502 {
			code = "processing_failure"
		}
		fail(c, status, code, e.Error())
		return
	}
	ok(c, status, data)
}
func (h Handler) AssistantPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, assistantHTML(c.Param("assistantToken")))
}

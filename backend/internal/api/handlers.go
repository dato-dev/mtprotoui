package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"

	"github.com/timurabdullin/mtprotoui/internal/auth"
	"github.com/timurabdullin/mtprotoui/internal/crypto"
	"github.com/timurabdullin/mtprotoui/internal/deploy"
	"github.com/timurabdullin/mtprotoui/internal/health"
	"github.com/timurabdullin/mtprotoui/internal/operation"
	"github.com/timurabdullin/mtprotoui/internal/sshclient"
	"github.com/timurabdullin/mtprotoui/internal/store"
	"github.com/timurabdullin/mtprotoui/internal/whitelist"
)

const (
	// deployAttempts bounds how many times a deploy is retried on transient
	// failures; deployAttemptTimeout caps a single attempt so a hung SSH
	// session can't leave a server stuck in "deploying".
	deployAttempts       = 3
	deployAttemptTimeout = 6 * time.Minute
)

type Handler struct {
	store      *store.Store
	auth       *auth.Service
	encKey     []byte
	whitelist  *whitelist.Service
	deployLock sync.Map
	deleteLock sync.Map
}

func NewHandler(s *store.Store, a *auth.Service, encKey []byte, wl *whitelist.Service) *Handler {
	return &Handler{
		store:     s,
		auth:      a,
		encKey:    encKey,
		whitelist: wl,
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("GET /api/auth/me", h.me)
	mux.HandleFunc("POST /api/auth/change-password", h.changePassword)

	mux.HandleFunc("GET /api/servers", h.listServers)
	mux.HandleFunc("POST /api/servers", h.createServer)
	mux.HandleFunc("GET /api/servers/{id}", h.getServer)
	mux.HandleFunc("PATCH /api/servers/{id}", h.editServer)
	mux.HandleFunc("DELETE /api/servers/{id}", h.deleteServer)
	mux.HandleFunc("POST /api/servers/{id}/recreate", h.recreateServer)
	mux.HandleFunc("POST /api/servers/{id}/health", h.checkHealth)
	mux.HandleFunc("GET /api/servers/{id}/qr", h.serverQR)
	mux.HandleFunc("GET /api/servers/{id}/logs", h.serverLogs)
	mux.HandleFunc("GET /api/servers/{id}/stats", h.serverStats)
	mux.HandleFunc("POST /api/servers/test-ssh", h.testSSH)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type sshRequest struct {
	Name        string   `json:"name"`
	Host        string   `json:"host"`
	SSHPort     int      `json:"ssh_port"`
	SSHUser     string   `json:"ssh_user"`
	SSHAuthType string   `json:"ssh_auth_type"`
	Password    string   `json:"password"`
	PrivateKey  string   `json:"private_key"`
	Passphrase  string   `json:"passphrase"`
	ProxyType   string   `json:"proxy_type"`
	MTProtoPort int      `json:"mtproto_port"`
	FakeTLS     bool     `json:"fake_tls"`
	Tags        []string `json:"tags"`
}

type editServerRequest struct {
	Name        string   `json:"name"`
	Host        string   `json:"host"`
	SSHPort     int      `json:"ssh_port"`
	SSHUser     string   `json:"ssh_user"`
	SSHAuthType string   `json:"ssh_auth_type"`
	Password    string   `json:"password"`
	PrivateKey  string   `json:"private_key"`
	Passphrase  string   `json:"passphrase"`
	MTProtoPort int      `json:"mtproto_port"`
	Tags        []string `json:"tags"`
}

type serverResponse struct {
	store.Server
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, token, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	h.auth.SetSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"username":             result.Username,
		"must_change_password": result.MustChangePassword,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, err := h.auth.ParseRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":             claims.Username,
		"must_change_password": claims.MustChangePassword,
	})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, err := h.auth.ParseRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	token, err := h.auth.ChangePassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.auth.SetSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"username":             claims.Username,
		"must_change_password": false,
	})
}

func (h *Handler) listServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}
	if servers == nil {
		servers = []store.Server{}
	}
	writeJSON(w, http.StatusOK, servers)
}

func (h *Handler) getServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get server")
		return
	}
	if srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, srv)
}

func (h *Handler) createServer(w http.ResponseWriter, r *http.Request) {
	var req sshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateSSHRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	creds := buildCredentials(req)
	encBlob, nonce, err := crypto.Encrypt(h.encKey, creds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.Name == "" {
		req.Name = req.Host
	}

	proxyType := deploy.NormalizeProxyType(req.ProxyType)
	mtprotoPort := req.MTProtoPort
	if mtprotoPort <= 0 || mtprotoPort > 65535 {
		mtprotoPort = deploy.DefaultPort(proxyType)
	}
	fakeTLS := req.FakeTLS && proxyType == deploy.ProxyTypeTGWS

	srv := store.Server{
		ID:              uuid.NewString(),
		Name:            req.Name,
		Host:            req.Host,
		SSHPort:         req.SSHPort,
		SSHUser:         req.SSHUser,
		SSHAuthType:     req.SSHAuthType,
		EncryptedBlob:   encBlob,
		EncryptionNonce: nonce,
		ProxyType:       proxyType,
		FakeTLS:         fakeTLS,
		MTProtoPort:     mtprotoPort,
		Tags:            req.Tags,
		ContainerName:   deploy.DefaultContainerName(proxyType),
		Status:          "unknown",
		DeployStatus:    "deploying",
		CreatedAt:       time.Now().UTC(),
	}

	if err := h.store.CreateServer(r.Context(), srv); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create server")
		return
	}

	h.setDeployStep(r.Context(), srv.ID, deployStepByKey(operation.DeployPreparing))
	go h.runDeploy(srv.ID, true)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":            srv.ID,
		"deploy_status": "deploying",
	})
}

func (h *Handler) editServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if srv.DeployStatus == "deploying" || srv.DeployStatus == "deleting" {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	var req editServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Merge with current values; empty fields keep existing data.
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = srv.Name
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = srv.Host
	}
	sshPort := req.SSHPort
	if sshPort <= 0 {
		sshPort = srv.SSHPort
	}
	sshUser := strings.TrimSpace(req.SSHUser)
	if sshUser == "" {
		sshUser = srv.SSHUser
	}
	mtprotoPort := req.MTProtoPort
	if mtprotoPort <= 0 || mtprotoPort > 65535 {
		mtprotoPort = srv.MTProtoPort
	}

	// Credentials change is optional. When auth type or secrets are provided,
	// validate and re-encrypt; otherwise keep the stored blob.
	credsChanged := req.Password != "" || req.PrivateKey != "" || (req.SSHAuthType != "" && req.SSHAuthType != srv.SSHAuthType)
	sshAuthType := srv.SSHAuthType
	if credsChanged {
		authReq := sshRequest{
			Host:        host,
			SSHUser:     sshUser,
			SSHAuthType: req.SSHAuthType,
			Password:    req.Password,
			PrivateKey:  req.PrivateKey,
			Passphrase:  req.Passphrase,
		}
		if err := validateSSHRequest(authReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		encBlob, nonce, encErr := crypto.Encrypt(h.encKey, buildCredentials(authReq))
		if encErr != nil {
			writeError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		if err := h.store.UpdateServerCredentials(r.Context(), id, encBlob, nonce); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update credentials")
			return
		}
		sshAuthType = req.SSHAuthType
	}

	if err := h.store.UpdateServerMeta(r.Context(), id, name, host, sshPort, sshUser, sshAuthType, mtprotoPort, req.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update server")
		return
	}

	// A port or host change only takes effect once the container is recreated.
	needsRedeploy := mtprotoPort != srv.MTProtoPort || host != srv.Host
	if needsRedeploy && srv.DeployStatus == "ready" {
		_ = h.store.UpdateServerDeploy(r.Context(), id, srv.SNIDomain, srv.Secret, srv.ProxyLink, mtprotoPort, "deploying", "")
		h.setDeployStep(r.Context(), id, deployStepByKey(operation.DeployPreparing))
		go h.runDeploy(id, false)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          id,
		"redeploying": needsRedeploy && srv.DeployStatus == "ready",
	})
}

func (h *Handler) deleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if srv.DeployStatus == "deploying" || srv.DeployStatus == "deleting" {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	h.setDeleteStep(r.Context(), id, deleteStepByKey(operation.DeletePreparing))
	go h.runDelete(id)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":            id,
		"deploy_status": "deleting",
	})
}

func (h *Handler) recreateServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	_ = h.store.UpdateServerDeploy(r.Context(), id, srv.SNIDomain, srv.Secret, srv.ProxyLink, srv.MTProtoPort, "deploying", "")
	h.setDeployStep(r.Context(), id, deployStepByKey(operation.DeployPreparing))
	go h.runDeploy(id, true)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":            id,
		"deploy_status": "deploying",
	})
}

func (h *Handler) checkHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	result := health.Check(srv.Host, srv.MTProtoPort)
	now := time.Now().UTC()
	_ = h.store.UpdateServerHealth(r.Context(), id, result.Status, result.PingStatus, result.TCPStatus, result.PingRTTMs, now)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        result.Status,
		"ping_status":   result.PingStatus,
		"tcp_status":    result.TCPStatus,
		"ping_rtt_ms":   result.PingRTTMs,
		"last_check_at": now,
	})
}

func (h *Handler) serverQR(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if srv.ProxyLink == "" {
		writeError(w, http.StatusNotFound, "proxy link not ready")
		return
	}

	png, err := qrcode.Encode(srv.ProxyLink, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate qr")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *Handler) serverLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			tail = n
		}
	}

	cfg, err := h.sshConfigFromServer(*srv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ssh config error")
		return
	}

	logs, err := deploy.ContainerLogs(cfg, srv.ContainerName, tail)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"container": srv.ContainerName,
		"logs":      logs,
	})
}

func (h *Handler) serverStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	cfg, err := h.sshConfigFromServer(*srv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ssh config error")
		return
	}

	stats, err := deploy.ContainerStats(cfg, srv.ContainerName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) testSSH(w http.ResponseWriter, r *http.Request) {
	var req sshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateSSHRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}

	cfg := sshclient.Config{
		Host:        req.Host,
		Port:        req.SSHPort,
		User:        req.SSHUser,
		AuthType:    req.SSHAuthType,
		Credentials: buildCredentials(req),
		HostKeys:    h.store,
	}

	if err := sshclient.TestConnection(cfg); err != nil {
		writeError(w, http.StatusBadGateway, "ssh connection failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) runDeploy(serverID string, pickNewSNI bool) {
	lockKey := serverID
	if _, loaded := h.deployLock.LoadOrStore(lockKey, struct{}{}); loaded {
		return
	}
	defer h.deployLock.Delete(lockKey)

	ctx := context.Background()
	srv, err := h.store.GetServer(ctx, serverID)
	if err != nil || srv == nil {
		return
	}

	h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeployPreparing))

	sni := srv.SNIDomain
	if deploy.UsesSNI(srv.ProxyType, srv.FakeTLS) && (pickNewSNI || sni == "") {
		h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeploySNI))
		sni, err = h.whitelist.RandomDomain()
		if err != nil {
			h.markDeployFailed(ctx, serverID, sni, err.Error())
			return
		}
	}

	h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeploySSH))
	cfg, err := h.sshConfigFromServer(*srv)
	if err != nil {
		h.markDeployFailed(ctx, serverID, sni, err.Error())
		return
	}

	h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeployRun))
	result, err := deploy.DeployWithRetry(cfg, deploy.Options{
		ProxyType:     srv.ProxyType,
		SNIDomain:     sni,
		Port:          srv.MTProtoPort,
		FakeTLS:       srv.FakeTLS,
		ContainerName: srv.ContainerName,
	}, deployAttempts, deployAttemptTimeout)
	if err != nil {
		h.markDeployFailed(ctx, serverID, sni, err.Error())
		return
	}

	h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeployFinalize))
	proxyLink := result.ProxyLink
	if !strings.Contains(proxyLink, srv.Host) {
		proxyLink = buildProxyLink(srv.Host, result.Port, result.Secret)
	}

	_ = h.store.UpdateServerDeploy(ctx, serverID, result.SNI, result.Secret, proxyLink, result.Port, "deploying", "")

	h.setDeployStep(ctx, serverID, deployStepByKey(operation.DeployHealth))
	check := health.Check(srv.Host, result.Port)
	_ = h.store.UpdateServerHealth(ctx, serverID, check.Status, check.PingStatus, check.TCPStatus, check.PingRTTMs, time.Now().UTC())

	h.clearOperation(ctx, serverID, "ready")
}

func (h *Handler) runDelete(serverID string) {
	lockKey := serverID
	if _, loaded := h.deleteLock.LoadOrStore(lockKey, struct{}{}); loaded {
		return
	}
	defer h.deleteLock.Delete(lockKey)

	ctx := context.Background()
	srv, err := h.store.GetServer(ctx, serverID)
	if err != nil || srv == nil {
		return
	}

	h.setDeleteStep(ctx, serverID, deleteStepByKey(operation.DeletePreparing))

	h.setDeleteStep(ctx, serverID, deleteStepByKey(operation.DeleteSSH))
	cfg, err := h.sshConfigFromServer(*srv)
	if err != nil {
		_ = h.store.SetServerOperationFailed(ctx, serverID, "ready", err.Error())
		return
	}

	h.setDeleteStep(ctx, serverID, deleteStepByKey(operation.DeleteContainer))
	if err := deploy.CleanupContainer(cfg, srv.ContainerName); err != nil {
		log.Printf("cleanup container %s: %v", serverID, err)
		_ = h.store.SetServerOperationFailed(ctx, serverID, "ready", err.Error())
		return
	}

	h.setDeleteStep(ctx, serverID, deleteStepByKey(operation.DeleteImage))
	if err := deploy.CleanupImage(cfg, deploy.ImageFor(srv.ProxyType)); err != nil {
		log.Printf("cleanup image %s: %v", serverID, err)
		_ = h.store.SetServerOperationFailed(ctx, serverID, "ready", err.Error())
		return
	}

	h.setDeleteStep(ctx, serverID, deleteStepByKey(operation.DeleteFinalize))
	if err := h.store.DeleteServer(ctx, serverID); err != nil {
		log.Printf("delete server %s: %v", serverID, err)
		_ = h.store.SetServerOperationFailed(ctx, serverID, "ready", err.Error())
		return
	}
	// Drop the pinned SSH host key so a re-imaged host can be re-trusted later.
	_ = h.store.DeleteHostKey(srv.Host + ":" + strconv.Itoa(srv.SSHPort))
}

func (h *Handler) sshConfigFromServer(srv store.Server) (sshclient.Config, error) {
	creds, err := crypto.Decrypt(h.encKey, srv.EncryptedBlob, srv.EncryptionNonce)
	if err != nil {
		return sshclient.Config{}, err
	}
	return sshclient.Config{
		Host:        srv.Host,
		Port:        srv.SSHPort,
		User:        srv.SSHUser,
		AuthType:    srv.SSHAuthType,
		Credentials: creds,
		HostKeys:    h.store,
	}, nil
}

func validateSSHRequest(req sshRequest) error {
	if req.Host == "" || req.SSHUser == "" {
		return errors.New("host and ssh_user are required")
	}
	switch req.SSHAuthType {
	case "password":
		if req.Password == "" {
			return errors.New("password is required")
		}
	case "key":
		if req.PrivateKey == "" {
			return errors.New("private_key is required")
		}
	default:
		return errors.New("ssh_auth_type must be password or key")
	}
	return nil
}

func buildCredentials(req sshRequest) crypto.SSHCredentials {
	return crypto.SSHCredentials{
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		Passphrase: req.Passphrase,
	}
}

func buildProxyLink(host string, port int, secret string) string {
	return "tg://proxy?server=" + host + "&port=" + strconv.Itoa(port) + "&secret=" + secret
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

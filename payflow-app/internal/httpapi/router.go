package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/payflow/payflow-app/internal/auth"
)

func NewRouter(s *Server) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(RequestID)
	r.Use(RequestLogger)

	r.Post("/v1/tenants", s.postTenants)
	r.Post("/v1/auth/login", s.postAuthLogin)

	r.Route("/v1", func(r chi.Router) {
		r.With(MeAuth(s.Pool, s.JWTSecret)).Get("/tenants/me", s.getTenantsMe)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Get("/tenants/me/api-keys", s.getTenantAPIKeys)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Delete("/tenants/me/api-keys/{keyID}", s.deleteTenantAPIKey)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Patch("/tenants/me/webhook", s.patchTenantWebhook)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Get("/webhook-deliveries", s.listWebhookDeliveries)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Get("/webhook-deliveries/{deliveryID}", s.getWebhookDelivery)
		r.With(MeAuth(s.Pool, s.JWTSecret)).Post("/webhook-deliveries/{deliveryID}/retry", s.postWebhookDeliveryRetry)
		r.With(auth.MiddlewareAPIKey(s.Pool)).Post("/tenants/{tenantID}/dashboard-users", s.postDashboardUser)
		r.With(auth.MiddlewareAPIKey(s.Pool)).Post("/payments", s.postPayment)
		r.With(auth.MiddlewareAPIKey(s.Pool)).Get("/payments/{paymentID}", s.getPayment)
		r.With(auth.MiddlewareAPIKey(s.Pool)).Post("/payments/{paymentID}/refunds", s.postRefund)
		r.With(auth.MiddlewareAPIKey(s.Pool)).Get("/refunds/{refundID}", s.getRefund)
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", promhttp.Handler())
	return r
}

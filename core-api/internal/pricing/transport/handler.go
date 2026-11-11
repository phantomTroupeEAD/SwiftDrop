package transport

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/events"
	identitydomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
	"github.com/swiftdrop/core-api/internal/pricing/application"
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

const maxQuoteBodyBytes int64 = 64 * 1024

// Handler exposes HTTP REST handlers for the pricing module.
type Handler struct {
	quoteUseCase   *application.GenerateQuoteUseCase
	merchantReader application.MerchantReader
}

// NewHandler constructs a pricing HTTP Handler with required use case and merchant reader ports.
func NewHandler(quoteUseCase *application.GenerateQuoteUseCase, merchantReader application.MerchantReader) (*Handler, error) {
	if quoteUseCase == nil {
		return nil, errors.New("pricing transport: nil generate quote use case")
	}
	if merchantReader == nil {
		return nil, errors.New("pricing transport: nil merchant reader")
	}

	return &Handler{
		quoteUseCase:   quoteUseCase,
		merchantReader: merchantReader,
	}, nil
}

type ownershipContext struct {
	requesterID *string
	merchantID  *string
}

type httpErrorSpec struct {
	status  int
	code    string
	message string
	details []httpx.ErrorDetail
}

// CreateQuote godoc
// @Summary      Calculate and persist a delivery pricing quote
// @Description  Evaluates geographic tariffs, volumetric weight, and service level multipliers to generate a 15-minute price-frozen quote scoped to the authenticated customer, merchant, or admin.
// @Tags         pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateQuoteRequest true "Pricing quote request payload"
// @Success      201 {object} httpx.DataEnvelope "Generated pricing quote wrapped in standard data envelope"
// @Failure      400 {object} httpx.ErrorEnvelope "Malformed JSON body or disallowed fields"
// @Failure      401 {object} httpx.ErrorEnvelope "Missing or invalid Bearer token"
// @Failure      403 {object} httpx.ErrorEnvelope "Forbidden role, missing/invalid tenant UUID, owner mismatch, or unapproved merchant"
// @Failure      413 {object} httpx.ErrorEnvelope "Request body exceeds size limit"
// @Failure      415 {object} httpx.ErrorEnvelope "Unsupported Content-Type"
// @Failure      422 {object} httpx.ErrorEnvelope "Semantic validation failure or no applicable tariff available"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal quote generation or verification failure"
// @Router       /api/v1/pricing/quotes [post]
func (h *Handler) CreateQuote(w http.ResponseWriter, r *http.Request) {
	owners, errSpec := h.resolveQuoteOwnership(r.Context())
	if errSpec != nil {
		httpx.WriteError(w, r, errSpec.status, errSpec.code, errSpec.message, errSpec.details...)
		return
	}

	req, errSpec := decodeQuoteRequest(w, r)
	if errSpec != nil {
		httpx.WriteError(w, r, errSpec.status, errSpec.code, errSpec.message, errSpec.details...)
		return
	}

	quote, err := h.quoteUseCase.Execute(r.Context(), application.GenerateQuoteCommand{
		OriginZoneID:      req.OriginZoneID,
		DestinationZoneID: req.DestinationZoneID,
		ActualWeightKg:    req.ActualWeightKg,
		LengthCm:          req.Dimensions.LengthCm,
		WidthCm:           req.Dimensions.WidthCm,
		HeightCm:          req.Dimensions.HeightCm,
		ServiceLevel:      domain.ServiceLevel(req.ServiceLevel),
		RequesterID:       owners.requesterID,
		MerchantID:        owners.merchantID,
	})
	if err != nil {
		writePricingError(w, r, err)
		return
	}

	_ = httpx.WriteData(w, http.StatusCreated, NewQuoteResponse(quote))
}

func (h *Handler) resolveQuoteOwnership(ctx context.Context) (ownershipContext, *httpErrorSpec) {
	principal, ok := identitytransport.GetPrincipal(ctx)
	if !ok || principal == nil {
		return ownershipContext{}, &httpErrorSpec{
			status:  http.StatusUnauthorized,
			code:    "AUTH_UNAUTHORIZED",
			message: "Authentication required",
		}
	}

	userID := strings.TrimSpace(principal.UserID)
	if userID == "" {
		return ownershipContext{}, &httpErrorSpec{
			status:  http.StatusUnauthorized,
			code:    "AUTH_UNAUTHORIZED",
			message: "Authenticated user identity is required",
		}
	}
	if !events.IsValidUUID(userID) {
		return ownershipContext{}, &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "INVALID_CALLER_ID",
			message: "Authenticated user ID must be a valid UUID",
		}
	}

	switch principal.Role {
	case identitydomain.RoleCustomer, identitydomain.RoleAdmin:
		return ownershipContext{
			requesterID: &userID,
		}, nil

	case identitydomain.RoleMerchant:
		merchantID, errSpec := h.resolveVerifiedMerchantID(ctx, userID, principal.TenantID)
		if errSpec != nil {
			return ownershipContext{}, errSpec
		}
		return ownershipContext{
			requesterID: &userID,
			merchantID:  &merchantID,
		}, nil

	default:
		return ownershipContext{}, &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "AUTH_FORBIDDEN",
			message: "Insufficient role permissions for this resource",
		}
	}
}

func (h *Handler) resolveVerifiedMerchantID(
	ctx context.Context,
	userID string,
	rawTenantID *string,
) (string, *httpErrorSpec) {
	if rawTenantID == nil || strings.TrimSpace(*rawTenantID) == "" {
		return "", &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MISSING_TENANT_ID",
			message: "tenant id required for merchant endpoints",
		}
	}

	tenantID := strings.TrimSpace(*rawTenantID)
	if !events.IsValidUUID(tenantID) {
		return "", &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "TENANT_NOT_MERCHANT_UUID",
			message: "tenant ID in JWT claim must be a valid merchant UUID",
		}
	}

	profile, err := h.merchantReader.FindByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, application.ErrMerchantNotFound) {
			return "", &httpErrorSpec{
				status:  http.StatusForbidden,
				code:    "MERCHANT_NOT_FOUND",
				message: "Merchant tenant account was not found",
			}
		}
		return "", &httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "MERCHANT_LOOKUP_FAILED",
			message: "Failed to verify merchant tenant ownership",
		}
	}

	if strings.TrimSpace(profile.OwnerUserID) != userID {
		return "", &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MERCHANT_OWNER_MISMATCH",
			message: "Authenticated user does not own the merchant tenant",
		}
	}

	if strings.TrimSpace(profile.Status) != application.MerchantStatusApproved {
		return "", &httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MERCHANT_NOT_APPROVED",
			message: "Merchant account is not approved for this action",
		}
	}

	merchantID := strings.TrimSpace(profile.ID)
	if !events.IsValidUUID(merchantID) {
		return "", &httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "MERCHANT_LOOKUP_FAILED",
			message: "Resolved merchant record contains an invalid identifier",
		}
	}

	return merchantID, nil
}

func decodeQuoteRequest(w http.ResponseWriter, r *http.Request) (CreateQuoteRequest, *httpErrorSpec) {
	if ct := strings.TrimSpace(r.Header.Get("Content-Type")); ct != "" && !isJSONMediaType(ct) {
		return CreateQuoteRequest{}, &httpErrorSpec{
			status:  http.StatusUnsupportedMediaType,
			code:    "UNSUPPORTED_MEDIA_TYPE",
			message: "Content-Type must be application/json",
		}
	}

	var req CreateQuoteRequest
	if err := httpx.DecodeJSON(w, r, &req, maxQuoteBodyBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return CreateQuoteRequest{}, &httpErrorSpec{
				status:  http.StatusRequestEntityTooLarge,
				code:    "PAYLOAD_TOO_LARGE",
				message: "Request body exceeds maximum allowed size",
			}
		}
		return CreateQuoteRequest{}, &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Request body must be valid JSON containing only documented fields",
		}
	}

	if req.Dimensions == nil {
		return CreateQuoteRequest{}, &httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_DIMENSIONS",
			message: "Parcel dimensions (length_cm, width_cm, height_cm) are required",
			details: []httpx.ErrorDetail{{Field: "dimensions", Issue: "dimensions object is required"}},
		}
	}

	return req, nil
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/json")
}

func writePricingError(w http.ResponseWriter, r *http.Request, err error) {
	spec := mapPricingError(err)
	httpx.WriteError(w, r, spec.status, spec.code, spec.message, spec.details...)
}

func mapPricingError(err error) httpErrorSpec {
	switch {
	// Check ErrInvalidZoneID before ErrInvalidTariff because GenerateQuoteUseCase wraps both together.
	case errors.Is(err, domain.ErrInvalidZoneID):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_ZONE_ID",
			message: "Origin and destination zones must follow the canonical ZONE_* format",
			details: []httpx.ErrorDetail{{Field: "zone_id", Issue: "must match ^ZONE_[A-Z0-9_]{2,27}$"}},
		}
	case errors.Is(err, domain.ErrInvalidServiceLevel):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_SERVICE_LEVEL",
			message: "Service level must be one of STANDARD, EXPRESS, or SAME_DAY",
			details: []httpx.ErrorDetail{{Field: "service_level", Issue: "unsupported service level"}},
		}
	case errors.Is(err, domain.ErrInvalidWeight):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_WEIGHT",
			message: "Actual parcel weight must be a positive finite number in kilograms",
			details: []httpx.ErrorDetail{{Field: "actual_weight_kg", Issue: "must be > 0"}},
		}
	case errors.Is(err, domain.ErrInvalidDimensions):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_DIMENSIONS",
			message: "Package length, width, and height must be positive finite numbers in centimeters",
			details: []httpx.ErrorDetail{{Field: "dimensions", Issue: "length_cm, width_cm, and height_cm must be > 0"}},
		}
	case errors.Is(err, domain.ErrChargeableWeightExceeded):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "PARCEL_LIMITS_EXCEEDED",
			message: "Parcel chargeable weight exceeds maximum allowed limit of 30.00 kg",
			details: []httpx.ErrorDetail{{Field: "actual_weight_kg", Issue: "chargeable weight cannot exceed 30.00 kg"}},
		}
	case errors.Is(err, domain.ErrTariffNotFound):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "PRICING_UNAVAILABLE",
			message: "No active pricing tariff covers the requested origin and destination zones",
		}
	case errors.Is(err, domain.ErrInvalidQuote):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_QUOTE",
			message: "Unable to construct a valid pricing quote from the supplied inputs",
		}
	case errors.Is(err, domain.ErrInvalidMoney), errors.Is(err, domain.ErrInvalidCurrency), errors.Is(err, domain.ErrCurrencyMismatch):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_MONEY",
			message: "Pricing monetary configuration is invalid",
		}
	case errors.Is(err, domain.ErrInvalidTariff):
		return httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "PRICING_CONFIGURATION_ERROR",
			message: "Active pricing tariff configuration is invalid",
		}
	case errors.Is(err, domain.ErrMoneyOverflow):
		return httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "PRICING_CALCULATION_ERROR",
			message: "Pricing calculation exceeded supported monetary bounds",
		}
	default:
		return httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "INTERNAL_ERROR",
			message: "Failed to generate pricing quote",
		}
	}
}

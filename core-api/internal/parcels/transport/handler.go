package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/swiftdrop/core-api/internal/events"
	identityapp "github.com/swiftdrop/core-api/internal/identity/application"
	identitytransport "github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/parcels/application"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
	"github.com/swiftdrop/core-api/internal/platform/httpx"
	"github.com/swiftdrop/core-api/internal/platform/middleware"
)

const maxParcelBodyBytes int64 = 64 * 1024

// Handler exposes HTTP REST handlers for the parcels module.
type Handler struct {
	bookUseCase       *application.BookParcelUseCase
	getParcelUseCase  *application.GetParcelUseCase
	getHistoryUseCase *application.GetParcelHistoryUseCase
	merchantReader    application.MerchantReader
}

// NewHandler constructs a parcels HTTP Handler with required use cases and merchant reader ports.
func NewHandler(
	bookUseCase *application.BookParcelUseCase,
	getParcelUseCase *application.GetParcelUseCase,
	getHistoryUseCase *application.GetParcelHistoryUseCase,
	merchantReader application.MerchantReader,
) (*Handler, error) {
	if bookUseCase == nil {
		return nil, errors.New("parcels transport: nil book parcel use case")
	}
	if getParcelUseCase == nil {
		return nil, errors.New("parcels transport: nil get parcel use case")
	}
	if getHistoryUseCase == nil {
		return nil, errors.New("parcels transport: nil get parcel history use case")
	}
	if merchantReader == nil {
		return nil, errors.New("parcels transport: nil merchant reader")
	}

	return &Handler{
		bookUseCase:       bookUseCase,
		getParcelUseCase:  getParcelUseCase,
		getHistoryUseCase: getHistoryUseCase,
		merchantReader:    merchantReader,
	}, nil
}

type httpErrorSpec struct {
	status  int
	code    string
	message string
	details []httpx.ErrorDetail
}

// BookParcel godoc
// @Summary      Book a new parcel using an accepted pricing quote
// @Description  Validates caller identity and merchant approval, verifies UUID Idempotency-Key and raw SHA-256 request hash, atomically consumes an unexpired quote owned by the caller, allocates a Crockford Base32 tracking number, and persists the parcel, audit history, and outbox event in a single transaction.
// @Tags         parcels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string true "UUID Idempotency Key"
// @Param        request body BookParcelRequest true "Parcel booking payload"
// @Success      201 {object} httpx.DataEnvelope "Booked parcel aggregate wrapped in standard data envelope"
// @Failure      400 {object} httpx.ErrorEnvelope "Missing/invalid Idempotency-Key, malformed JSON, or disallowed fields"
// @Failure      401 {object} httpx.ErrorEnvelope "Missing or invalid Bearer token"
// @Failure      403 {object} httpx.ErrorEnvelope "Forbidden role, invalid/missing merchant tenant, unapproved merchant, or quote ownership mismatch"
// @Failure      404 {object} httpx.ErrorEnvelope "Referenced pricing quote not found"
// @Failure      409 {object} httpx.ErrorEnvelope "Idempotency key payload mismatch/conflict or quote already used"
// @Failure      413 {object} httpx.ErrorEnvelope "Request body exceeds 64 KB size limit"
// @Failure      415 {object} httpx.ErrorEnvelope "Unsupported Content-Type"
// @Failure      422 {object} httpx.ErrorEnvelope "Semantic validation failure, expired quote, or quote zone/weight mismatch"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal parcel booking failure"
// @Failure      503 {object} httpx.ErrorEnvelope "Tracking number allocation temporarily unavailable"
// @Router       /api/v1/parcels [post]
func (h *Handler) BookParcel(w http.ResponseWriter, r *http.Request) {
	principal, errSpec := requireAuthenticatedPrincipal(r.Context())
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	caller := callerContextFromPrincipal(principal)
	resolvedCaller, err := application.ResolveParcelCallerForBooking(r.Context(), caller, h.merchantReader)
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	idemKey, errSpec := parseIdempotencyKeyHeader(r.Header.Get("Idempotency-Key"))
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	callerScope, err := application.CallerScopeForBooking(resolvedCaller.SenderID, resolvedCaller.MerchantID)
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	req, requestHash, errSpec := decodeBookingRequest(w, r)
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	if errSpec := validateBookingRequestStructure(req); errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	result, err := h.bookUseCase.Execute(r.Context(), application.BookParcelCommand{
		QuoteID:        strings.TrimSpace(req.QuoteID),
		SenderID:       resolvedCaller.SenderID,
		MerchantID:     resolvedCaller.MerchantID,
		ActorType:      resolvedCaller.ActorType,
		RecipientName:  req.RecipientName,
		RecipientPhone: req.RecipientPhone,
		WeightKg:       req.WeightKg,
		Dimensions: application.DimensionsDTO{
			LengthCm: req.Dimensions.LengthCm,
			WidthCm:  req.Dimensions.WidthCm,
			HeightCm: req.Dimensions.HeightCm,
		},
		PickupAddress:       toAddressDTO(*req.Pickup),
		DropoffAddress:      toAddressDTO(*req.Dropoff),
		ItemDescription:     req.ItemDescription,
		SpecialInstructions: req.SpecialInstructions,
		CorrelationID:       middleware.CorrelationID(r.Context()),
		Idempotency: &application.IdempotencyInput{
			Key:         idemKey,
			CallerScope: callerScope,
			Operation:   application.DefaultBookingOperation,
			RequestHash: requestHash,
		},
	})
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	if result.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}

	_ = httpx.WriteData(w, http.StatusCreated, NewParcelResponse(result.Parcel))
}

// GetParcel godoc
// @Summary      Retrieve authorized parcel details
// @Description  Returns the Parcel aggregate root for the owning customer, approved owning merchant tenant, or admin. Disallows couriers, dispatchers, and non-approved merchants.
// @Tags         parcels
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Parcel UUID"
// @Success      200 {object} httpx.DataEnvelope "Parcel details wrapped in standard data envelope"
// @Failure      400 {object} httpx.ErrorEnvelope "Invalid parcel UUID path parameter"
// @Failure      401 {object} httpx.ErrorEnvelope "Missing or invalid Bearer token"
// @Failure      403 {object} httpx.ErrorEnvelope "Forbidden role, unapproved merchant, or caller does not own parcel"
// @Failure      404 {object} httpx.ErrorEnvelope "Parcel not found"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal parcel lookup failure"
// @Router       /api/v1/parcels/{id} [get]
func (h *Handler) GetParcel(w http.ResponseWriter, r *http.Request) {
	principal, errSpec := requireAuthenticatedPrincipal(r.Context())
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	parcel, err := h.getParcelUseCase.Execute(r.Context(), application.GetParcelQuery{
		ParcelID: strings.TrimSpace(r.PathValue("id")),
		Caller:   callerContextFromPrincipal(principal),
	})
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, NewParcelResponse(parcel))
}

// GetParcelReceipt returns the accepted booking price to authorized callers.
func (h *Handler) GetParcelReceipt(w http.ResponseWriter, r *http.Request) {
	principal, errSpec := requireAuthenticatedPrincipal(r.Context())
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	parcel, err := h.getParcelUseCase.Execute(r.Context(), application.GetParcelQuery{
		ParcelID: strings.TrimSpace(r.PathValue("id")),
		Caller:   callerContextFromPrincipal(principal),
	})
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteData(w, http.StatusOK, newReceiptResponse(parcel))
}

// GetParcelHistory godoc
// @Summary      Retrieve authorized chronological parcel status history
// @Description  Returns parcel metadata and ordered lifecycle status history entries for the owning customer, approved owning merchant tenant, or admin.
// @Tags         parcels
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Parcel UUID"
// @Success      200 {object} httpx.DataEnvelope "Parcel status history wrapped in standard data envelope"
// @Failure      400 {object} httpx.ErrorEnvelope "Invalid parcel UUID path parameter"
// @Failure      401 {object} httpx.ErrorEnvelope "Missing or invalid Bearer token"
// @Failure      403 {object} httpx.ErrorEnvelope "Forbidden role, unapproved merchant, or caller does not own parcel"
// @Failure      404 {object} httpx.ErrorEnvelope "Parcel not found"
// @Failure      500 {object} httpx.ErrorEnvelope "Internal parcel history lookup failure"
// @Router       /api/v1/parcels/{id}/history [get]
func (h *Handler) GetParcelHistory(w http.ResponseWriter, r *http.Request) {
	principal, errSpec := requireAuthenticatedPrincipal(r.Context())
	if errSpec != nil {
		writeErrorSpec(w, r, *errSpec)
		return
	}

	history, err := h.getHistoryUseCase.Execute(r.Context(), application.GetParcelHistoryQuery{
		ParcelID: strings.TrimSpace(r.PathValue("id")),
		Caller:   callerContextFromPrincipal(principal),
	})
	if err != nil {
		writeParcelError(w, r, err)
		return
	}

	_ = httpx.WriteData(w, http.StatusOK, NewParcelHistoryResponse(history.Parcel, history.Entries))
}

func requireAuthenticatedPrincipal(ctx context.Context) (*identityapp.Principal, *httpErrorSpec) {
	principal, ok := identitytransport.GetPrincipal(ctx)
	if !ok || principal == nil {
		return nil, &httpErrorSpec{
			status:  http.StatusUnauthorized,
			code:    "AUTH_UNAUTHORIZED",
			message: "Authentication required",
		}
	}
	if strings.TrimSpace(principal.UserID) == "" {
		return nil, &httpErrorSpec{
			status:  http.StatusUnauthorized,
			code:    "AUTH_UNAUTHORIZED",
			message: "Authenticated user identity is required",
		}
	}
	return principal, nil
}

func callerContextFromPrincipal(p *identityapp.Principal) application.CallerContext {
	return application.CallerContext{
		UserID:   strings.TrimSpace(p.UserID),
		Role:     strings.ToUpper(strings.TrimSpace(string(p.Role))),
		TenantID: p.TenantID,
	}
}

func parseIdempotencyKeyHeader(rawHeader string) (string, *httpErrorSpec) {
	key := strings.TrimSpace(rawHeader)
	if key == "" {
		return "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "MISSING_IDEMPOTENCY_KEY",
			message: "Idempotency-Key header is required",
		}
	}

	normalized := strings.ToLower(key)
	if !events.IsValidUUID(normalized) {
		return "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_IDEMPOTENCY_KEY",
			message: "Idempotency-Key header must be a valid UUID",
		}
	}

	return normalized, nil
}

func decodeBookingRequest(w http.ResponseWriter, r *http.Request) (BookParcelRequest, string, *httpErrorSpec) {
	if ct := strings.TrimSpace(r.Header.Get("Content-Type")); ct != "" && !isJSONMediaType(ct) {
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusUnsupportedMediaType,
			code:    "UNSUPPORTED_MEDIA_TYPE",
			message: "Content-Type must be application/json",
		}
	}

	if r.Body == nil {
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Request body must be valid JSON containing only documented fields",
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxParcelBodyBytes)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return BookParcelRequest{}, "", &httpErrorSpec{
				status:  http.StatusRequestEntityTooLarge,
				code:    "PAYLOAD_TOO_LARGE",
				message: "Request body exceeds maximum allowed size",
			}
		}
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Failed to read JSON request body",
		}
	}

	if len(bytes.TrimSpace(rawBody)) == 0 {
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Request body must be valid JSON containing only documented fields",
		}
	}

	var req BookParcelRequest
	dec := json.NewDecoder(bytes.NewReader(rawBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Request body must be valid JSON containing only documented fields",
		}
	}

	// Verify no trailing tokens or extra JSON documents exist after the primary payload.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BookParcelRequest{}, "", &httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Request body must contain only a single JSON document",
		}
	}

	sum := sha256.Sum256(rawBody)
	return req, hex.EncodeToString(sum[:]), nil
}

func validateBookingRequestStructure(req BookParcelRequest) *httpErrorSpec {
	quoteID := strings.TrimSpace(req.QuoteID)
	if quoteID == "" || !events.IsValidUUID(quoteID) {
		return &httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_QUOTE_ID",
			message: "A valid quote_id UUID is required to book a parcel",
			details: []httpx.ErrorDetail{{Field: "quote_id", Issue: "must be a valid UUID"}},
		}
	}

	if req.Dimensions == nil {
		return &httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_DIMENSIONS",
			message: "Parcel dimensions (length_cm, width_cm, height_cm) are required",
			details: []httpx.ErrorDetail{{Field: "dimensions", Issue: "dimensions object is required"}},
		}
	}

	if req.Pickup == nil {
		return &httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_ADDRESS",
			message: "Pickup address object is required",
			details: []httpx.ErrorDetail{{Field: "pickup", Issue: "pickup address object is required"}},
		}
	}

	if req.Dropoff == nil {
		return &httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_ADDRESS",
			message: "Dropoff address object is required",
			details: []httpx.ErrorDetail{{Field: "dropoff", Issue: "dropoff address object is required"}},
		}
	}

	return nil
}

func toAddressDTO(a AddressRequest) application.AddressDTO {
	return application.AddressDTO{
		ContactName:  a.ContactName,
		ContactPhone: a.ContactPhone,
		StreetLine1:  a.StreetLine1,
		StreetLine2:  a.StreetLine2,
		City:         a.City,
		PostalCode:   a.PostalCode,
		ZoneID:       a.ZoneID,
		Instructions: a.Instructions,
	}
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/json")
}

func writeErrorSpec(w http.ResponseWriter, r *http.Request, spec httpErrorSpec) {
	httpx.WriteError(w, r, spec.status, spec.code, spec.message, spec.details...)
}

func writeParcelError(w http.ResponseWriter, r *http.Request, err error) {
	writeErrorSpec(w, r, mapParcelError(err))
}

func mapParcelError(err error) httpErrorSpec {
	switch {
	// Check wrapped merchant/caller sub-sentinels before fallback ErrForbiddenParcelAccess
	// because application queries multi-wrap them with ErrForbiddenParcelAccess.
	case errors.Is(err, application.ErrInvalidCallerID):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "INVALID_CALLER_ID",
			message: "Authenticated user ID must be a valid UUID",
		}
	case errors.Is(err, application.ErrMissingTenantID):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MISSING_TENANT_ID",
			message: "tenant id required for merchant endpoints",
		}
	case errors.Is(err, application.ErrInvalidTenantID):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "TENANT_NOT_MERCHANT_UUID",
			message: "tenant ID in JWT claim must be a valid merchant UUID",
		}
	case errors.Is(err, application.ErrMerchantNotFound):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MERCHANT_NOT_FOUND",
			message: "Merchant tenant account was not found",
		}
	case errors.Is(err, application.ErrMerchantOwnerMismatch):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MERCHANT_OWNER_MISMATCH",
			message: "Authenticated user does not own the merchant tenant",
		}
	case errors.Is(err, application.ErrMerchantNotApproved):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "MERCHANT_NOT_APPROVED",
			message: "Merchant account is not approved for this action",
		}
	case errors.Is(err, application.ErrForbiddenParcelAccess):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "PARCEL_FORBIDDEN",
			message: "Caller is not permitted to access this parcel",
		}
	case errors.Is(err, application.ErrQuoteOwnershipMismatch):
		return httpErrorSpec{
			status:  http.StatusForbidden,
			code:    "QUOTE_FORBIDDEN",
			message: "Pricing quote does not belong to the requesting customer or merchant",
		}

	case errors.Is(err, application.ErrParcelNotFound):
		return httpErrorSpec{
			status:  http.StatusNotFound,
			code:    "PARCEL_NOT_FOUND",
			message: "Parcel was not found",
		}
	case errors.Is(err, application.ErrQuoteNotFound):
		return httpErrorSpec{
			status:  http.StatusNotFound,
			code:    "QUOTE_NOT_FOUND",
			message: "Referenced pricing quote was not found",
		}

	case errors.Is(err, domain.ErrInvalidParcelID):
		return httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_PARCEL_ID",
			message: "Parcel identifier path parameter must be a valid UUID",
			details: []httpx.ErrorDetail{{Field: "id", Issue: "must be a valid UUID"}},
		}
	case errors.Is(err, application.ErrInvalidIdempotencyKey):
		return httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_IDEMPOTENCY_KEY",
			message: "Idempotency-Key header is missing or invalid",
		}
	case errors.Is(err, application.ErrInvalidCallerScope),
		errors.Is(err, application.ErrInvalidIdempotencyOperation),
		errors.Is(err, application.ErrInvalidRequestHash),
		errors.Is(err, application.ErrInvalidIdempotencyRecord):
		return httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_IDEMPOTENCY_METADATA",
			message: "Idempotency metadata failed validation",
		}
	case errors.Is(err, application.ErrInvalidBookingCommand):
		return httpErrorSpec{
			status:  http.StatusBadRequest,
			code:    "INVALID_REQUEST_PAYLOAD",
			message: "Invalid parcel booking request payload",
		}

	case errors.Is(err, application.ErrIdempotencyPayloadMismatch):
		return httpErrorSpec{
			status:  http.StatusConflict,
			code:    "IDEMPOTENCY_KEY_PAYLOAD_MISMATCH",
			message: "Idempotency-Key was reused with a different request payload or operation",
		}
	case errors.Is(err, application.ErrIdempotencyConflict):
		return httpErrorSpec{
			status:  http.StatusConflict,
			code:    "IDEMPOTENCY_CONFLICT",
			message: "Conflicting active idempotency key for this request",
		}
	case errors.Is(err, application.ErrQuoteAlreadyUsed):
		return httpErrorSpec{
			status:  http.StatusConflict,
			code:    "QUOTE_ALREADY_USED",
			message: "Pricing quote has already been consumed by an existing booking",
		}
	case errors.Is(err, application.ErrConcurrencyConflict):
		return httpErrorSpec{
			status:  http.StatusConflict,
			code:    "CONCURRENCY_CONFLICT",
			message: "Parcel was modified concurrently by another operation",
		}

	case errors.Is(err, domain.ErrInvalidQuoteID):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_QUOTE_ID",
			message: "A valid quote_id UUID is required to book a parcel",
			details: []httpx.ErrorDetail{{Field: "quote_id", Issue: "must be a valid UUID"}},
		}
	case errors.Is(err, application.ErrQuoteExpired):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "QUOTE_EXPIRED",
			message: "Referenced pricing quote has expired; generate a fresh quote before booking",
			details: []httpx.ErrorDetail{{Field: "quote_id", Issue: "quote expired"}},
		}
	case errors.Is(err, application.ErrQuoteWeightMismatch), errors.Is(err, application.ErrQuoteZoneMismatch):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "QUOTE_PARAM_MISMATCH",
			message: "Parcel weight, dimensions, or address zones do not match the referenced pricing quote",
		}
	// Check ErrInvalidZone before ErrInvalidAddress because Address.Validate wraps both together.
	case errors.Is(err, domain.ErrInvalidZone):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_ZONE_ID",
			message: "Pickup and dropoff zones must follow the canonical ZONE_* format",
			details: []httpx.ErrorDetail{{Field: "zone_id", Issue: "must match ^ZONE_[A-Z0-9_]{2,27}$"}},
		}
	case errors.Is(err, domain.ErrZoneMismatch):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "ZONE_MISMATCH",
			message: "Address zones do not match parcel origin and destination zones",
		}
	case errors.Is(err, domain.ErrInvalidDimensions):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_DIMENSIONS",
			message: "Package length, width, and height must be positive finite numbers in centimeters",
			details: []httpx.ErrorDetail{{Field: "dimensions", Issue: "length_cm, width_cm, and height_cm must be > 0"}},
		}
	case errors.Is(err, domain.ErrInvalidWeight):
		if strings.Contains(err.Error(), "exceeds maximum limit") {
			return httpErrorSpec{
				status:  http.StatusUnprocessableEntity,
				code:    "PARCEL_LIMITS_EXCEEDED",
				message: "Parcel chargeable weight exceeds maximum allowed limit of 30.00 kg",
				details: []httpx.ErrorDetail{{Field: "weight_kg", Issue: "chargeable weight cannot exceed 30.00 kg"}},
			}
		}
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_WEIGHT",
			message: "Parcel weight must be a positive finite number in kilograms",
			details: []httpx.ErrorDetail{{Field: "weight_kg", Issue: "must be > 0 and <= 30.00"}},
		}
	case errors.Is(err, domain.ErrInvalidRecipient):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_RECIPIENT",
			message: "Recipient name and contact phone are required",
			details: []httpx.ErrorDetail{{Field: "recipient", Issue: "recipient_name and recipient_phone are required"}},
		}
	case errors.Is(err, domain.ErrInvalidAddress):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_ADDRESS",
			message: "Pickup and dropoff addresses must contain all required contact, street, city, postal code, and zone fields",
		}
	case errors.Is(err, domain.ErrInvalidServiceLevel):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_SERVICE_LEVEL",
			message: "Quote service level must be one of STANDARD, EXPRESS, or SAME_DAY",
		}
	case errors.Is(err, domain.ErrInvalidOwner),
		errors.Is(err, domain.ErrInvalidSenderID),
		errors.Is(err, domain.ErrInvalidMerchantID),
		errors.Is(err, domain.ErrInvalidParcel),
		errors.Is(err, domain.ErrInvalidDeliveryFee),
		errors.Is(err, domain.ErrInvalidPricingRule):
		return httpErrorSpec{
			status:  http.StatusUnprocessableEntity,
			code:    "INVALID_PARCEL",
			message: "Parcel parameters violate domain invariants",
		}

	case errors.Is(err, application.ErrTrackingNumberCollision):
		return httpErrorSpec{
			status:  http.StatusServiceUnavailable,
			code:    "TRACKING_NUMBER_UNAVAILABLE",
			message: "Unable to allocate a unique tracking number; retry with the same Idempotency-Key",
		}

	default:
		return httpErrorSpec{
			status:  http.StatusInternalServerError,
			code:    "INTERNAL_ERROR",
			message: "Failed to process parcel request",
		}
	}
}

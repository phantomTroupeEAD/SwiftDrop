package events

// Canonical domain and integration event type constants.
// Governed by Decision D2 and team-wide event contracts.
const (
	// Parcel Lifecycle Events (Member 02 / 03)
	ParcelBookedV1         = "parcel.booked.v1"
	ParcelAssignedV1       = "parcel.assigned.v1"
	ParcelPickedUpV1       = "parcel.picked_up.v1"
	ParcelInTransitV1      = "parcel.in_transit.v1"
	ParcelOutForDeliveryV1 = "parcel.out_for_delivery.v1"
	ParcelDeliveredV1      = "parcel.delivered.v1"
	ParcelDeliveryFailedV1 = "parcel.delivery_failed.v1"
	ParcelReturnedV1       = "parcel.returned.v1"
	ParcelCancelledV1      = "parcel.cancelled.v1"

	// Courier & Dispatch Events (Member 03)
	CourierUnassignedV1 = "courier.unassigned.v1"

	// Merchant Integration Events (Member 04)
	MerchantStatusChangedV1 = "merchant.status_changed.v1"
	MerchantApprovedV1      = "merchant.approved.v1"
)

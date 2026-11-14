import React, { useEffect, useMemo, useState } from "react";
import { Button } from "@/shared/components/ui/button";
import { DialogFooter } from "@/shared/components/ui/dialog";
import { Input } from "@/shared/components/ui/input";
import { Label } from "@/shared/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/shared/components/ui/select";
import { ApiError } from "../api";
import { listUsers } from "../admin-console/consoleApi";
import {
  AdminDialog,
  Banner,
  Busy,
  FieldError,
  FieldHint,
} from "../admin-console/ui";
import {
  registerCourier,
  type CourierCreateInput,
  type VehicleType,
  type ZoneOption,
} from "./fleetApi";
import type { User } from "@/shared/types/auth";

// Adds a courier profile for an account that already has the COURIER role (dispatch API.md
// section 1). An admin picks the account from the courier accounts that have no profile yet.
// A dispatcher cannot read the user list, so they enter the account id they were given.

const VEHICLES: { value: VehicleType; label: string }[] = [
  { value: "MOTORCYCLE", label: "Motorcycle" },
  { value: "VAN", label: "Van" },
  { value: "TRUCK", label: "Truck" },
];
const CODE_MAX = 20; // employee_code column is VARCHAR(20)
const CAP_MIN = 1;
const CAP_MAX = 200;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

interface Errors {
  user?: string;
  code?: string;
  vehicle?: string;
  capacity?: string;
  zone?: string;
}

const FIELD_FOR: Record<string, keyof Errors> = {
  user_id: "user",
  employee_code: "code",
  vehicle_type: "vehicle",
  max_capacity: "capacity",
  current_zone_id: "zone",
};

interface AddCourierModalProps {
  token: string | null;
  /** ADMIN picks from a list; DISPATCHER enters an account id. */
  canListUsers: boolean;
  zones: ZoneOption[];
  /** Accounts that already have a fleet profile, so they are not offered again. */
  existingIds: Set<string>;
  onClose: () => void;
  onSaved: (message: string) => void;
}

export const AddCourierModal: React.FC<AddCourierModalProps> = ({
  token,
  canListUsers,
  zones,
  existingIds,
  onClose,
  onSaved,
}) => {
  const [accounts, setAccounts] = useState<User[] | null>(null);
  const [accountsError, setAccountsError] = useState<ApiError | null>(null);
  const [userId, setUserId] = useState("");
  const [code, setCode] = useState("");
  const [vehicle, setVehicle] = useState<VehicleType>("MOTORCYCLE");
  const [capacity, setCapacity] = useState("");
  const [zone, setZone] = useState("");
  const [errors, setErrors] = useState<Errors>({});
  const [banner, setBanner] = useState<ApiError | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!canListUsers) return;
    let cancelled = false;
    listUsers(token, "COURIER", 200)
      .then((res) => !cancelled && setAccounts(res.users))
      .catch(
        (err) =>
          !cancelled &&
          setAccountsError(
            err instanceof ApiError
              ? err
              : new ApiError(0, "UNKNOWN", "Couldn't load courier accounts."),
          ),
      );
    return () => {
      cancelled = true;
    };
  }, [canListUsers, token]);

  const available = useMemo(
    () => (accounts ?? []).filter((u) => !existingIds.has(u.id)),
    [accounts, existingIds],
  );
  const noAccounts =
    canListUsers && accounts !== null && available.length === 0;

  const validate = (): Errors => {
    const e: Errors = {};
    if (!userId.trim())
      e.user = canListUsers
        ? "Choose a courier account."
        : "Enter the courier account id.";
    else if (!UUID.test(userId.trim()))
      e.user =
        "The account id must look like 3fa85f64-5717-4562-b3fc-2c963f66afa6.";
    if (!code.trim()) e.code = "Enter an employee code.";
    else if (code.trim().length > CODE_MAX)
      e.code = `Use at most ${CODE_MAX} characters.`;
    if (capacity.trim() !== "") {
      const n = Number(capacity);
      if (!Number.isInteger(n) || n < CAP_MIN || n > CAP_MAX)
        e.capacity = `Enter a whole number from ${CAP_MIN} to ${CAP_MAX}, or leave it blank.`;
    }
    if (!zone) e.zone = "Choose the zone this courier starts in.";
    return e;
  };

  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    setBanner(null);
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    const input: CourierCreateInput = {
      user_id: userId.trim(),
      employee_code: code.trim(),
      vehicle_type: vehicle,
      max_capacity: capacity.trim() === "" ? 0 : Number(capacity),
      current_zone_id: zone,
    };
    setSaving(true);
    try {
      const saved = await registerCourier(token, input);
      onSaved(`Courier ${saved.employee_code} added. They start off shift.`);
    } catch (err) {
      const api =
        err instanceof ApiError
          ? err
          : new ApiError(0, "UNKNOWN", "Couldn't add the courier. Try again.");
      const next: Errors = {};
      if (api.code === "COURIER_PROFILE_EXISTS")
        next.user = "This account already has a courier profile.";
      else if (api.code === "USER_NOT_A_COURIER")
        next.user =
          "This account does not have the Courier role. An admin can change it under Users and roles.";
      else if (api.code === "USER_NOT_FOUND")
        next.user = "No account has this id.";
      else if (api.code === "VALIDATION_FAILED") {
        for (const d of api.details) {
          const key = d.field ? FIELD_FOR[d.field] : undefined;
          if (key) next[key] = d.issue;
        }
      }
      setErrors(next);
      // Anything that is not tied to a field goes in a banner, with the reference id.
      if (Object.keys(next).length === 0) setBanner(api);
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminDialog
      title="Add a courier"
      description="Create the fleet profile for an account that has the Courier role. The courier starts off shift."
      onClose={onClose}
      busy={saving}
      testId="fleet-add-dialog"
    >
      <form onSubmit={submit} noValidate>
        <div className="grid max-h-96 gap-4 overflow-y-auto px-5 py-5">
          {banner && (
            <Banner
              tone="err"
              reference={banner.correlationId}
              testId="fleet-add-error"
            >
              {banner.status === 403
                ? "Your account is not allowed to add couriers."
                : banner.message}
            </Banner>
          )}

          <div className="grid content-start gap-1.5">
            <Label htmlFor="fleet-user">
              {canListUsers ? "Courier account" : "Courier account id"}
            </Label>
            {canListUsers ? (
              <>
                {accountsError && (
                  <Banner tone="err" reference={accountsError.correlationId}>
                    {accountsError.message}
                  </Banner>
                )}
                {accounts === null && !accountsError && (
                  <FieldHint>Loading courier accounts…</FieldHint>
                )}
                {noAccounts && (
                  <Banner tone="info" testId="fleet-add-no-accounts">
                    Every account with the Courier role already has a profile.
                    To add someone new, first give their account the Courier
                    role under Users and roles.
                  </Banner>
                )}
                {available.length > 0 && (
                  <Select
                    value={userId}
                    onValueChange={(v) => {
                      setUserId(v);
                      setErrors((x) => ({ ...x, user: undefined }));
                    }}
                  >
                    <SelectTrigger
                      id="fleet-user"
                      data-testid="fleet-add-user"
                      aria-invalid={Boolean(errors.user)}
                      aria-describedby={
                        errors.user ? "fleet-user-error" : undefined
                      }
                    >
                      <SelectValue placeholder="Choose an account" />
                    </SelectTrigger>
                    <SelectContent>
                      {available.map((u) => (
                        <SelectItem key={u.id} value={u.id}>
                          {u.display_name ||
                            [u.first_name, u.last_name]
                              .filter(Boolean)
                              .join(" ") ||
                            u.email}{" "}
                          · {u.email}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </>
            ) : (
              <>
                <Input
                  id="fleet-user"
                  data-testid="fleet-add-user"
                  className="font-mono"
                  value={userId}
                  onChange={(e) => {
                    setUserId(e.target.value);
                    setErrors((x) => ({ ...x, user: undefined }));
                  }}
                  placeholder="3fa85f64-5717-4562-b3fc-2c963f66afa6"
                  autoComplete="off"
                  aria-invalid={Boolean(errors.user)}
                  aria-describedby={
                    errors.user ? "fleet-user-error" : "fleet-user-hint"
                  }
                />
                {!errors.user && (
                  <FieldHint id="fleet-user-hint">
                    Ask an admin for the account id from Users and roles. The
                    account must have the Courier role.
                  </FieldHint>
                )}
              </>
            )}
            {errors.user && (
              <FieldError id="fleet-user-error">{errors.user}</FieldError>
            )}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-code">Employee code</Label>
              <Input
                id="fleet-code"
                data-testid="fleet-add-code"
                className="font-mono"
                value={code}
                maxLength={CODE_MAX + 10}
                onChange={(e) => {
                  setCode(e.target.value);
                  setErrors((x) => ({ ...x, code: undefined }));
                }}
                placeholder="CR-7001"
                autoComplete="off"
                aria-invalid={Boolean(errors.code)}
                aria-describedby={errors.code ? "fleet-code-error" : undefined}
              />
              {errors.code && (
                <FieldError id="fleet-code-error">{errors.code}</FieldError>
              )}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-vehicle">Vehicle</Label>
              <Select
                value={vehicle}
                onValueChange={(v) => setVehicle(v as VehicleType)}
              >
                <SelectTrigger
                  id="fleet-vehicle"
                  data-testid="fleet-add-vehicle"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VEHICLES.map((v) => (
                    <SelectItem key={v.value} value={v.value}>
                      {v.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-capacity">Capacity (parcels at once)</Label>
              <Input
                id="fleet-capacity"
                data-testid="fleet-add-capacity"
                className="font-mono tabular-nums"
                inputMode="numeric"
                value={capacity}
                onChange={(e) => {
                  setCapacity(e.target.value);
                  setErrors((x) => ({ ...x, capacity: undefined }));
                }}
                placeholder="Vehicle default"
                autoComplete="off"
                aria-invalid={Boolean(errors.capacity)}
                aria-describedby={
                  errors.capacity
                    ? "fleet-capacity-error"
                    : "fleet-capacity-hint"
                }
              />
              {errors.capacity ? (
                <FieldError id="fleet-capacity-error">
                  {errors.capacity}
                </FieldError>
              ) : (
                <FieldHint id="fleet-capacity-hint">
                  Leave blank to use the vehicle&apos;s default. Allowed:{" "}
                  {CAP_MIN} to {CAP_MAX}.
                </FieldHint>
              )}
            </div>
            <div className="grid content-start gap-1.5">
              <Label htmlFor="fleet-zone">Starting zone</Label>
              <Select
                value={zone}
                onValueChange={(v) => {
                  setZone(v);
                  setErrors((x) => ({ ...x, zone: undefined }));
                }}
              >
                <SelectTrigger
                  id="fleet-zone"
                  data-testid="fleet-add-zone"
                  aria-invalid={Boolean(errors.zone)}
                  aria-describedby={
                    errors.zone ? "fleet-zone-error" : undefined
                  }
                >
                  <SelectValue placeholder="Choose a zone" />
                </SelectTrigger>
                <SelectContent>
                  {zones.map((z) => (
                    <SelectItem key={z.id} value={z.id}>
                      {z.name} ({z.code})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.zone && (
                <FieldError id="fleet-zone-error">{errors.zone}</FieldError>
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={saving || noAccounts}
            data-testid="fleet-add-save-btn"
          >
            {saving ? <Busy label="Adding..." /> : "Add courier"}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};

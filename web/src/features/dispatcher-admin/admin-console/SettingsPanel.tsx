import React, { useCallback, useEffect, useState } from 'react';
import { RefreshCw, SlidersHorizontal } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/components/ui/card';
import { DialogFooter } from '@/shared/components/ui/dialog';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { Textarea } from '@/shared/components/ui/textarea';
import { REASON_MAX, REASON_MIN, validateReason } from '../zones/zoneValidation';
import { ApiError } from '../api';
import { listSettings, updateSetting, type Setting } from './consoleApi';
import { allowedText, displayValue, parseSettingInput, previewFor, ruleFor, valueToText } from './settingsRules';
import { AdminDialog, Banner, Busy, FieldError, FieldHint } from './ui';

// Runtime settings (core-api/internal/admin/API.md section 3). Only allowlisted keys can be
// changed; every change needs the current version and a reason, and is written to the audit log.

interface SettingsPanelProps {
  token: string | null;
  onNotice: (message: string) => void;
  onChanged: () => void;
}

export const SettingsPanel: React.FC<SettingsPanelProps> = ({ token, onNotice, onChanged }) => {
  const [settings, setSettings] = useState<Setting[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<ApiError | null>(null);
  const [editing, setEditing] = useState<Setting | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      setSettings(await listSettings(token));
    } catch (err) {
      setLoadError(err instanceof ApiError ? err : new ApiError(0, 'UNKNOWN', "Couldn't load settings. Try again."));
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const showEmpty = !loading && !loadError && settings.length === 0;

  return (
    <Card role="region" aria-labelledby="settings-title">
      <CardHeader className="flex-row flex-wrap items-start justify-between gap-3 space-y-0">
        <div className="grid gap-1.5">
          <CardTitle id="settings-title">System settings</CardTitle>
          <CardDescription>Values the platform reads at run time. Each change needs a reason and is kept in the audit log.</CardDescription>
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={() => void load()}
          disabled={loading}
          aria-label="Refresh settings"
          title="Refresh settings"
        >
          <RefreshCw className={loading ? 'animate-spin' : undefined} aria-hidden="true" />
        </Button>
      </CardHeader>

      <CardContent className="grid gap-4">
        <p className="sr-only" role="status">
          {loading ? 'Loading settings…' : ''}
        </p>

        {loadError && (
          <Banner
            tone="err"
            reference={loadError.correlationId}
            action={
              <div>
                <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
                  Retry
                </Button>
              </div>
            }
          >
            {loadError.status === 403 ? 'Your account is not allowed to view settings.' : loadError.message || "Couldn't load settings. Try again."}
          </Banner>
        )}

        {showEmpty ? (
          <div className="flex flex-col items-center gap-2 rounded-md border border-border bg-surface-subtle px-4 py-12 text-center">
            <SlidersHorizontal size={24} strokeWidth={2} className="text-text-muted" aria-hidden="true" />
            <CardTitle>No settings found</CardTitle>
            <CardDescription>The settings list is empty. Run the database migrations, then refresh.</CardDescription>
          </div>
        ) : (
          <Table aria-label="System settings" data-testid="admin-settings-table" className="min-w-max">
            <TableHeader>
              <TableRow>
                <TableHead>Setting</TableHead>
                <TableHead>Current value</TableHead>
                <TableHead>Version</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading && settings.length === 0
                ? [0, 1, 2, 3, 4].map((i) => (
                    <TableRow key={i} aria-hidden="true">
                      <TableCell>
                        <Skeleton className="h-4 w-40" />
                        <Skeleton className="mt-2 h-3 w-56" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-24" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-4 w-8" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="ml-auto h-8 w-24" />
                      </TableCell>
                    </TableRow>
                  ))
                : settings.map((s) => {
                    const rule = ruleFor(s.key);
                    return (
                      <TableRow key={s.key}>
                        <TableCell>
                          <div className="font-medium">{rule?.label ?? s.key}</div>
                          <div className="font-mono text-xs text-text-muted">{s.key}</div>
                          <div className="mt-0.5 max-w-sm text-xs text-text-muted">{s.description}</div>
                          {rule && <div className="mt-0.5 font-mono text-xs tabular-nums text-text-muted">{allowedText(rule)}</div>}
                        </TableCell>
                        <TableCell className="font-mono tabular-nums">{displayValue(s)}</TableCell>
                        <TableCell className="font-mono tabular-nums">{s.version}</TableCell>
                        <TableCell className="text-right">
                          {rule ? (
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="touch:min-h-[44px]"
                              onClick={() => setEditing(s)}
                              aria-label={`Change ${rule.label}`}
                              data-testid="admin-setting-edit-btn"
                            >
                              Change value
                            </Button>
                          ) : (
                            <Badge variant="neutral">Read only</Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
            </TableBody>
          </Table>
        )}
      </CardContent>

      {editing && (
        <SettingDialog
          setting={editing}
          token={token}
          onClose={() => setEditing(null)}
          onStale={() => {
            setEditing(null);
            void load();
          }}
          onSaved={(saved) => {
            setSettings((prev) => prev.map((s) => (s.key === saved.key ? saved : s)));
            setEditing(null);
            onNotice(`${ruleFor(saved.key)?.label ?? saved.key} saved (version ${saved.version}).`);
            onChanged();
          }}
        />
      )}
    </Card>
  );
};

interface SettingDialogProps {
  setting: Setting;
  token: string | null;
  onClose: () => void;
  onSaved: (saved: Setting) => void;
  /** The latest setting could not be read after a conflict: the panel reloads the list. */
  onStale: () => void;
}

const SettingDialog: React.FC<SettingDialogProps> = ({ setting, token, onClose, onSaved, onStale }) => {
  const rule = ruleFor(setting.key);
  // The version this form is based on. It moves forward when the admin keeps their value
  // after someone else saved first.
  const [base, setBase] = useState<Setting>(setting);
  const [text, setText] = useState(valueToText(setting.value));
  const [reason, setReason] = useState('');
  const [valueError, setValueError] = useState<string | null>(null);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [banner, setBanner] = useState<string | null>(null);
  const [correlationId, setCorrelationId] = useState('');
  const [conflict, setConflict] = useState<Setting | null>(null);
  const [reloadNeeded, setReloadNeeded] = useState(false);
  const [saving, setSaving] = useState(false);

  /** Version conflict: read the latest value and let the admin decide, without losing what they typed. */
  const handleConflict = async (err: ApiError) => {
    try {
      const latest = (await listSettings(token)).find((s) => s.key === setting.key);
      if (latest) {
        setConflict(latest);
        return;
      }
    } catch {
      // Fall through to the plain message below.
    }
    setBanner(err.message);
    setReloadNeeded(true);
  };

  const keepMine = () => {
    if (!conflict) return;
    setBase(conflict);
    setConflict(null);
  };

  const useTheirs = () => {
    if (!conflict) return;
    setBase(conflict);
    setText(valueToText(conflict.value));
    setValueError(null);
    setConflict(null);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBanner(null);
    setCorrelationId('');
    setReloadNeeded(false);

    const parsed = parseSettingInput(setting.key, text);
    const reasonProblem = validateReason(reason);
    setValueError(parsed.ok ? null : parsed.message);
    setReasonError(reasonProblem);
    if (!parsed.ok || reasonProblem) return;

    if (parsed.value === base.value) {
      setBanner('The value is the same as the current one, so there is nothing to save.');
      return;
    }

    setSaving(true);
    try {
      const saved = await updateSetting(token, setting.key, { version: base.version, value: parsed.value, reason: reason.trim() });
      onSaved(saved);
    } catch (err) {
      if (err instanceof ApiError) {
        setCorrelationId(err.correlationId);
        if (err.code === 'CONCURRENT_MODIFICATION') await handleConflict(err);
        else if (err.code === 'REASON_REQUIRED') setReasonError(err.message);
        else if (err.code === 'VALIDATION_FAILED') setValueError(err.message);
        else setBanner(err.message);
      } else {
        setBanner("Couldn't save the setting. Try again.");
      }
    } finally {
      setSaving(false);
    }
  };

  if (!rule) return null;

  const preview = previewFor(setting.key, text);

  return (
    <AdminDialog title={`Change ${rule.label}`} description={rule.help} onClose={onClose} busy={saving} testId="admin-setting-dialog">
      <form onSubmit={submit} noValidate>
        <div className="grid gap-4 px-5 py-5">
          <p className="font-mono text-xs tabular-nums text-text-muted">
            {setting.key} · version {base.version}
          </p>

          {conflict && (
            <Banner
              tone="attn"
              testId="admin-setting-conflict"
              reference={correlationId}
              action={
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" onClick={keepMine} data-testid="admin-setting-conflict-keep-btn">
                    Keep my value on the latest version
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={useTheirs} data-testid="admin-setting-conflict-theirs-btn">
                    Use their value instead
                  </Button>
                </div>
              }
            >
              Someone else changed this setting while you were editing. It is now {displayValue(conflict)} (version {conflict.version}). Your
              value and reason are still in the form.
            </Banner>
          )}

          {banner && (
            <Banner
              tone="err"
              reference={correlationId}
              action={
                reloadNeeded ? (
                  <div>
                    <Button type="button" variant="outline" size="sm" onClick={onStale}>
                      <RefreshCw aria-hidden="true" /> Reload settings
                    </Button>
                  </div>
                ) : undefined
              }
            >
              {banner}
            </Banner>
          )}

          <div className="grid gap-1.5">
            <Label htmlFor="setting-value">New value{rule.unit ? ` (${rule.unit})` : ''}</Label>
            <Input
              id="setting-value"
              data-testid="admin-setting-value-input"
              data-autofocus
              className={rule.kind === 'text' ? undefined : 'font-mono tabular-nums'}
              inputMode={rule.kind === 'decimal' ? 'decimal' : rule.kind === 'integer' ? 'numeric' : 'text'}
              value={text}
              onChange={(e) => {
                setText(e.target.value);
                setValueError(null);
              }}
              disabled={saving}
              aria-invalid={Boolean(valueError)}
              aria-describedby="setting-value-note"
            />
            <div className="flex justify-between gap-3">
              {valueError ? (
                <FieldError id="setting-value-note">{valueError}</FieldError>
              ) : (
                <FieldHint id="setting-value-note">
                  {allowedText(rule)}. Current value: {displayValue(base)}
                </FieldHint>
              )}
              {preview ? (
                <span data-testid="admin-setting-preview" className="shrink-0 font-mono text-xs tabular-nums text-text">
                  {preview}
                </span>
              ) : rule.kind === 'text' ? (
                <span className="shrink-0 font-mono text-xs tabular-nums text-text-muted">
                  {[...text].length}/{rule.max}
                </span>
              ) : null}
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="setting-reason">
              Reason ({REASON_MIN} to {REASON_MAX} characters)
            </Label>
            <Textarea
              id="setting-reason"
              data-testid="admin-setting-reason-input"
              value={reason}
              onChange={(e) => {
                setReason(e.target.value);
                setReasonError(null);
              }}
              disabled={saving}
              rows={3}
              placeholder="Board polls too often at peak hours"
              aria-invalid={Boolean(reasonError)}
              aria-describedby="setting-reason-note"
            />
            <div className="flex justify-between gap-3">
              {reasonError ? (
                <FieldError id="setting-reason-note">{reasonError}</FieldError>
              ) : (
                <FieldHint id="setting-reason-note">Kept in the audit log with your name and the time.</FieldHint>
              )}
              <span className="font-mono text-xs tabular-nums text-text-muted">
                {[...reason.trim()].length}/{REASON_MAX}
              </span>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving || conflict !== null} data-testid="admin-setting-save-btn">
            {saving ? <Busy label="Saving…" /> : 'Save changes'}
          </Button>
        </DialogFooter>
      </form>
    </AdminDialog>
  );
};

import { Badge } from '@/shared/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { Input } from '@/shared/components/ui/input';
import { Button } from '@/shared/components/ui/button';
import { useRef, useState } from 'react';
import { errorMessage, portalRequest } from './api';
import { PortalErrorMessage } from './PortalLayout';

const columns = [
  'merchant_reference', 'pickup_name', 'pickup_phone', 'pickup_address', 'pickup_city', 'pickup_zone_id',
  'dropoff_name', 'dropoff_phone', 'dropoff_address', 'dropoff_city', 'dropoff_zone_id', 'weight_kg', 'length_cm', 'width_cm', 'height_cm', 'description',
] as const;
type Column = typeof columns[number];
type Row = { line: number; key: string; fields: Record<Column, string>; result?: string; error?: string; validation?: string };
const template = columns.join(',') + '\r\n';

function parseCsv(text: string) {
  const rows: string[][] = [];
  let row: string[] = [];
  let value = '';
  let quoted = false;
  let closed = false;
  const endCell = () => { row.push(value.trim()); value = ''; closed = false; };
  const endRow = () => { endCell(); if (row.some(Boolean)) rows.push(row); row = []; };
  for (let index = 0; index < text.length; index++) {
    const character = text[index];
    if (quoted) {
      if (character !== '"') value += character;
      else if (text[index + 1] === '"') { value += '"'; index++; }
      else { quoted = false; closed = true; }
    } else if (character === ',') endCell();
    else if (character === '\n' || character === '\r') {
      if (character === '\r' && text[index + 1] === '\n') index++;
      endRow();
    } else if (character === '"' && !value && !closed) quoted = true;
    else if (character === '"' || closed) throw new Error('Use double quotes to enclose fields that contain commas.');
    else value += character;
  }
  if (quoted) throw new Error('A quoted CSV field is missing its closing quote.');
  endRow();
  const header = rows.shift();
  if (!header || header.length !== columns.length || !columns.every((column, index) => column === header[index])) {
    throw new Error('Use the column names from the CSV template.');
  }
  if (!rows.length || rows.length > 100) throw new Error('Upload between 1 and 100 orders at a time.');
  return rows.map((cells, index): Row => {
    const line = index + 2;
    const fields = Object.fromEntries(header.map((column, cell) => [column, cells[cell] ?? ''])) as Row['fields'];
    const bounded = (value: string, max: number) => Number(value) >= 0.01 && Number(value) <= max;
    let validation: string | undefined;
    if (cells.length !== header.length || columns.some(column => column !== 'merchant_reference' && !fields[column])) {
      validation = 'Fill in every required field.';
    } else if (![fields.pickup_phone, fields.dropoff_phone].every(phone => /^\+[1-9][0-9]{7,14}$/.test(phone))) {
      validation = 'Use international phone numbers such as +94771234567.';
    } else if (!bounded(fields.weight_kg, 30)) {
      validation = 'Weight must be 0.01 to 30 kg.';
    } else if (![fields.length_cm, fields.width_cm, fields.height_cm].every(value => bounded(value, 150))) {
      validation = 'Each dimension must be 0.01 to 150 cm.';
    }
    return { line, key: crypto.randomUUID(), fields, validation };
  });
}

export function MerchantOrderImport({ token, onComplete }: { token: string | null; onComplete: () => void }) {
  const [rows, setRows] = useState<Row[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const importing = useRef(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const hasAttempted = rows.some(row => row.result || row.error);
  return <section className="m6-card">
    <h2>Import orders</h2>
    <p><a download="swiftdrop-orders.csv" href={`data:text/csv;charset=utf-8,${encodeURIComponent(template)}`}>Download CSV template</a></p>
    <p id="csv-contract" className="text-sm text-text-muted">Dimensions and descriptions are checked in the preview. The current merchant order API accepts weight only for parcel details.</p>
    <label className="m6-field">CSV file
      <Input ref={fileInput} type="file" aria-describedby="csv-contract" accept=".csv,text/csv" disabled={loading || busy || hasAttempted}
        data-testid="batch-csv-file-input" onChange={async event => {
          const file = event.target.files?.[0];
          setRows([]);
          setError('');
          if (!file) return;
          setLoading(true);
          try {
            if (file.size > 256_000) throw new Error('Choose a CSV file smaller than 256 KB.');
            setRows(parseCsv((await file.text()).replace(/^\uFEFF/, '')));
          } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not read this CSV file.'); }
          finally { setLoading(false); }
        }} />
    </label>
    <PortalErrorMessage message={error} />
    {rows.length > 0 && <>
      <p role="status">{rows.filter(row => row.result).length} of {rows.filter(row => !row.validation).length} valid orders submitted.</p>
      {busy && <progress className="w-full" aria-label="CSV submission progress" max={rows.filter(row => !row.validation).length}
        value={rows.filter(row => row.result || row.error).length} />}
      <Table aria-label="CSV order preview">
        <TableHeader><TableRow><TableHead>Record</TableHead><TableHead>Destination</TableHead><TableHead>Status</TableHead></TableRow></TableHeader>
        <TableBody>{rows.map(row => <TableRow key={row.key}>
          <TableCell>{row.line}</TableCell><TableCell>{row.fields.dropoff_city}</TableCell>
          <TableCell><Badge variant={row.validation || row.error ? 'error' : 'success'}>
            {row.validation ? 'Invalid' : row.result ? 'Submitted' : row.error ? 'Failed' : 'Ready'}
          </Badge> {row.validation || row.error || row.result}</TableCell>
        </TableRow>)}</TableBody>
      </Table>
      <div className="m6-actions">
        <Button type="button" data-testid="batch-csv-submit-btn"
          disabled={busy || rows.every(row => row.result || row.validation)} onClick={async () => {
            if (importing.current) return;
            importing.current = true;
            setBusy(true);
            setRows(current => current.map(row => ({ ...row, error: undefined })));
            let submitted = 0;
            try {
              for (const row of rows.filter(item => !item.result && !item.validation)) {
                const contact = (prefix: 'pickup' | 'dropoff') => ({
                  name: row.fields[`${prefix}_name`], phone: row.fields[`${prefix}_phone`],
                  address_lines: [row.fields[`${prefix}_address`]], city: row.fields[`${prefix}_city`],
                  zone_id: row.fields[`${prefix}_zone_id`],
                });
                let result: string | undefined;
                let failure: string | undefined;
                try {
                  const order = await portalRequest<{ tracking_number: string }>('/merchant/orders', {
                    method: 'POST', token, idempotencyKey: row.key, body: {
                      merchant_reference: row.fields.merchant_reference, service_level: 'STANDARD',
                      pickup: contact('pickup'), dropoff: contact('dropoff'), parcel: {
                        weight_kg: Number(row.fields.weight_kg),
                      },
                    },
                  });
                  result = order?.tracking_number;
                  if (result) submitted++;
                  else failure = 'No tracking number returned. Retry this record.';
                } catch (cause) { failure = errorMessage(cause); }
                setRows(current => current.map(item => item.key === row.key ? { ...item, result, error: failure } : item));
              }
            } finally {
              importing.current = false;
              setBusy(false);
              if (submitted) onComplete();
            }
          }}>{busy ? 'Submitting…' : hasAttempted ? 'Retry unsuccessful orders' : 'Submit orders'}</Button>
        {hasAttempted && <Button variant="outline" type="button" disabled={busy} onClick={() => {
          if (rows.some(row => !row.result && !row.validation) && !window.confirm('Clear this batch and its retry history? Check your order list before importing again.')) return;
          if (fileInput.current) fileInput.current.value = '';
          setRows([]);
        }}>Clear batch</Button>}
      </div>
    </>}
  </section>;
}

import { useRef, useState } from 'react';
import { errorMessage, portalRequest } from './api';
import { PortalErrorMessage } from './PortalLayout';

const columns = [
  'merchant_reference', 'pickup_name', 'pickup_phone', 'pickup_address', 'pickup_city', 'pickup_zone_id',
  'dropoff_name', 'dropoff_phone', 'dropoff_address', 'dropoff_city', 'dropoff_zone_id', 'weight_kg',
] as const;
type Column = typeof columns[number];
type Row = { line: number; key: string; fields: Record<Column, string>; result?: string; error?: string };
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
  if (!header || header.length !== columns.length || !columns.every(column => header.includes(column))) {
    throw new Error('Use the column names from the CSV template.');
  }
  if (!rows.length || rows.length > 100) throw new Error('Upload between 1 and 100 orders at a time.');
  return rows.map((cells, index): Row => {
    const line = index + 2;
    const fields = Object.fromEntries(header.map((column, cell) => [column, cells[cell] ?? ''])) as Row['fields'];
    if (cells.length !== header.length || columns.some(column => column !== 'merchant_reference' && !fields[column])) {
      throw new Error(`CSV record ${line} has missing fields.`);
    }
    const weight = Number(fields.weight_kg);
    if (!Number.isFinite(weight) || weight <= 0 || weight > 30) {
      throw new Error(`CSV record ${line} needs a weight greater than 0 and at most 30 kg.`);
    }
    return { line, key: crypto.randomUUID(), fields };
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
    <label className="m6-field">CSV file
      <input ref={fileInput} type="file" accept=".csv,text/csv" disabled={loading || busy || hasAttempted}
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
      <p role="status">{rows.filter(row => row.result).length} of {rows.length} orders submitted.</p>
      {hasAttempted && <ul>{rows.map(row => <li key={row.key}>Record {row.line}: {row.result || row.error || 'Waiting'}</li>)}</ul>}
      <div className="m6-actions">
        <button className="m6-btn" type="button" data-testid="batch-csv-submit-btn"
          disabled={busy || rows.every(row => row.result)} onClick={async () => {
            if (importing.current) return;
            importing.current = true;
            setBusy(true);
            let submitted = 0;
            try {
              for (const row of rows.filter(item => !item.result)) {
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
                      pickup: contact('pickup'), dropoff: contact('dropoff'), parcel: { weight_kg: Number(row.fields.weight_kg) },
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
          }}>{busy ? 'Submitting…' : hasAttempted ? 'Retry unsuccessful orders' : 'Submit orders'}</button>
        {hasAttempted && <button className="m6-btn m6-secondary" type="button" disabled={busy} onClick={() => {
          if (rows.some(row => !row.result) && !window.confirm('Clear this batch and its retry history? Check your order list before importing again.')) return;
          if (fileInput.current) fileInput.current.value = '';
          setRows([]);
        }}>Clear batch</button>}
      </div>
    </>}
  </section>;
}

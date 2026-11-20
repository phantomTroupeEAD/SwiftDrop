/**
 * Generates valid bulk CSV data adhering strictly to SwiftDrop's Merchant Import specification
 * (MerchantOrderImport.tsx).
 */
export function generateBulkOrdersCsv(count: number = 32): string {
  const headers = [
    'merchant_reference',
    'pickup_name',
    'pickup_phone',
    'pickup_address',
    'pickup_city',
    'pickup_zone_id',
    'dropoff_name',
    'dropoff_phone',
    'dropoff_address',
    'dropoff_city',
    'dropoff_zone_id',
    'weight_kg',
    'length_cm',
    'width_cm',
    'height_cm',
    'description',
  ].join(',');

  const recipients = [
    { name: 'Nalaka Bandara', phone: '+94771122301', street: '12 Galle Road', city: 'Colombo' },
    { name: 'Kavindu Senanayake', phone: '+94771122302', street: '45 Duplication Road', city: 'Colombo' },
    { name: 'Thilini Jayawardena', phone: '+94771122303', street: '88 Havelock Road', city: 'Colombo' },
    { name: 'Kasun Wickramasinghe', phone: '+94771122304', street: '104 Bauddhaloka Mawatha', city: 'Colombo' },
    { name: 'Sanduni Gunasekara', phone: '+94771122305', street: '33 Kandy Road', city: 'Kandy' },
    { name: 'Chathura Rajapakse', phone: '+94771122306', street: '59 Peradeniya Road', city: 'Kandy' },
    { name: 'Ruvini Alwis', phone: '+94771122307', street: '77 Matara Road', city: 'Galle' },
    { name: 'Dhammika Prasad', phone: '+94771122308', street: '21 Main Street', city: 'Galle' },
  ];

  const rows: string[] = [headers];

  for (let i = 1; i <= count; i++) {
    const r = recipients[(i - 1) % recipients.length];
    const weight = (1.0 + (i % 5) * 0.5).toFixed(1);
    const length = 20 + (i % 10);
    const width = 15 + (i % 8);
    const height = 10 + (i % 6);
    const ref = `ORD-DEMO-${String(i).padStart(3, '0')}`;
    const desc = `E-Commerce Consignment #${i} (${ref})`;

    rows.push(
      [
        ref,
        'SwiftDrop Apex Central Fulfilment',
        '+94770001122',
        '100 Logistics Boulevard',
        'Colombo',
        'ZONE_COLOMBO_01',
        `${r.name} #${i}`,
        r.phone,
        `${r.street} Apt ${i}`,
        r.city,
        'ZONE_COLOMBO_01',
        weight,
        length,
        width,
        height,
        desc,
      ].join(',')
    );
  }

  return rows.join('\r\n') + '\r\n';
}

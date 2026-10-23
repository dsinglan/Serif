// CMS's names for the 21 places of service the rate file uses, word for word
// from CMS's Place of Service Code Set (checked 2026-10-21):
// https://www.cms.gov/medicare/coding-billing/place-of-service-codes/code-sets
const places: Record<string, string> = {
  '02': 'Telehealth Provided Other than in Patient’s Home',
  '04': 'Homeless Shelter',
  '10': 'Telehealth Provided in Patient’s Home',
  '11': 'Office',
  '12': 'Home',
  '15': 'Mobile Unit',
  '17': 'Walk-in Retail Health Clinic',
  '19': 'Off Campus-Outpatient Hospital',
  '20': 'Urgent Care Facility',
  '21': 'Inpatient Hospital',
  '22': 'On Campus-Outpatient Hospital',
  '23': 'Emergency Room – Hospital',
  '24': 'Ambulatory Surgical Center',
  '41': 'Ambulance - Land',
  '50': 'Federally Qualified Health Center',
  '62': 'Comprehensive Outpatient Rehabilitation Facility',
  '65': 'End-Stage Renal Disease Treatment Facility',
  '71': 'Public Health Clinic',
  '72': 'Rural Health Clinic',
  '81': 'Independent Laboratory',
  '99': 'Other Place of Service',
}

// placeLabel is a place-of-service code and CMS's name for it, such as
// "11 Office". A code the list doesn't have, from another file, shows alone.
export function placeLabel(code: string): string {
  const name = places[code]
  return name ? `${code} ${name}` : code
}

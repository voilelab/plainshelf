/**
 * Formatting shared by every reka NumberField. `Intl.NumberFormat` groups
 * thousands ("5,000") and admits three decimals by default; every numeric
 * setting here is a plain integer also read back as text — by the char-count
 * filter's URL and by the e2e suite — so both are turned off.
 */
export const INTEGER_FORMAT_OPTIONS: Intl.NumberFormatOptions = {
  useGrouping: false,
  maximumFractionDigits: 0
};

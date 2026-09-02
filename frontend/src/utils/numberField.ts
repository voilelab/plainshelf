/**
 * Formatting shared by every reka NumberField in the app.
 *
 * The field renders its value through `Intl.NumberFormat`, whose defaults group
 * thousands ("5,000") and admit three decimals. Every numeric setting here is a
 * plain integer that is also read back as text — by the char-count filter's URL
 * and by the e2e suite — so grouping and fractions are both turned off.
 */
export const INTEGER_FORMAT_OPTIONS: Intl.NumberFormatOptions = {
  useGrouping: false,
  maximumFractionDigits: 0
};

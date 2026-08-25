export function commaStringToList(input: string): string[] {
  return input
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

export function listToCommaString(list?: string[]): string {
  if (!list || list.length === 0) {
    return '';
  }
  return list.map((item) => item.trim()).filter((item) => item.length > 0).join(', ');
}

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

export function layerStringToLayers(input: string): string[] {
  return input
    .split('/')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

export function layersToLayerString(layers?: string[]): string {
  if (!layers || layers.length === 0) {
    return '';
  }
  return layers.map((item) => item.trim()).filter((item) => item.length > 0).join('/');
}

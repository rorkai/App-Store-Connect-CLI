function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function flattenObject(value: Record<string, unknown>, prefix = ""): Record<string, unknown> {
  const flattened: Record<string, unknown> = {};

  for (const [key, entry] of Object.entries(value)) {
    const flatKey = prefix ? `${prefix}.${key}` : key;
    if (Array.isArray(entry)) {
      continue;
    }
    if (isPlainObject(entry)) {
      Object.assign(flattened, flattenObject(entry, flatKey));
      continue;
    }
    flattened[flatKey] = entry;
  }

  return flattened;
}

function normalizeResource(resource: unknown): Record<string, unknown> | null {
  if (!isPlainObject(resource)) {
    return null;
  }

  const item: Record<string, unknown> = {};

  if ("id" in resource) {
    item.id = resource.id;
  }
  if ("type" in resource) {
    item.type = resource.type;
  }
  if (isPlainObject(resource.attributes)) {
    Object.assign(item, flattenObject(resource.attributes));
  }

  for (const [key, value] of Object.entries(resource)) {
    if (key === "id" || key === "type" || key === "attributes" || key === "links" || key === "meta" || key === "relationships") {
      continue;
    }
    if (Array.isArray(value)) {
      continue;
    }
    if (isPlainObject(value)) {
      Object.assign(item, flattenObject(value, key));
      continue;
    }
    item[key] = value;
  }

  return Object.keys(item).length > 0 ? item : null;
}

export function parseCommandItems(raw: string): Record<string, unknown>[] {
  const parsed = JSON.parse(raw);

  if (Array.isArray(parsed?.data)) {
    return parsed.data
      .map((item: unknown) => normalizeResource(item))
      .filter((item: Record<string, unknown> | null): item is Record<string, unknown> => item !== null);
  }

  if (parsed?.data) {
    const item = normalizeResource(parsed.data);
    return item ? [item] : [];
  }

  if (Array.isArray(parsed)) {
    return parsed
      .map((item: unknown, index) => {
        const normalized = normalizeResource(item);
        if (!normalized) {
          return null;
        }
        if (!("id" in normalized)) {
          normalized.id = `item-${index + 1}`;
        }
        return normalized;
      })
      .filter((item: Record<string, unknown> | null): item is Record<string, unknown> => item !== null);
  }

  if (isPlainObject(parsed)) {
    for (const [key, value] of Object.entries(parsed)) {
      if (!Array.isArray(value) || !value.every((entry) => isPlainObject(entry))) {
        continue;
      }

      return value.map((entry, index) => {
        const normalized = normalizeResource(entry) ?? {};
        if (!("id" in normalized)) {
          normalized.id = `${key}-${index + 1}`;
        }
        return normalized;
      });
    }

    const flattened = flattenObject(parsed);
    return Object.keys(flattened).length > 0 ? [flattened] : [];
  }

  return [];
}

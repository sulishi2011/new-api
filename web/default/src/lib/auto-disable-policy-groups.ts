export type AutoDisablePolicyGroup = {
  id: string
  name: string
  enabled: boolean
  keywords: string[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function normalizeAutoDisableKeywords(value: unknown): string[] {
  const source = Array.isArray(value) ? value : String(value ?? '').split('\n')
  const seen = new Set<string>()
  const keywords: string[] = []

  for (const item of source) {
    const keyword = String(item ?? '')
      .trim()
      .toLowerCase()
    if (!keyword || seen.has(keyword)) continue
    seen.add(keyword)
    keywords.push(keyword)
  }

  return keywords
}

export function normalizeAutoDisablePolicyGroups(
  value: unknown
): AutoDisablePolicyGroup[] {
  if (!Array.isArray(value)) {
    throw new Error('Policy groups must be an array')
  }

  const ids = new Set<string>()
  const names = new Set<string>()
  return value.map((item, index) => {
    if (!isRecord(item)) {
      throw new Error(`Policy group #${index + 1} must be an object`)
    }

    const id = String(item.id ?? '').trim()
    const name = String(item.name ?? '').trim()
    if (!id) {
      throw new Error(`Policy group #${index + 1} id is required`)
    }
    if (!name) {
      throw new Error(`Policy group #${index + 1} name is required`)
    }
    if (ids.has(id)) {
      throw new Error(`Duplicate policy group id: ${id}`)
    }
    const nameKey = name.toLowerCase()
    if (names.has(nameKey)) {
      throw new Error(`Duplicate policy group name: ${name}`)
    }

    ids.add(id)
    names.add(nameKey)
    return {
      id,
      name,
      enabled: item.enabled !== false,
      keywords: Array.isArray(item.keywords)
        ? item.keywords.map((keyword) => String(keyword ?? ''))
        : String(item.keywords ?? '').split('\n'),
    }
  })
}

export function normalizeAutoDisablePolicyGroupsString(value: string): {
  ok: boolean
  value: string
  groups: AutoDisablePolicyGroup[]
  error?: string
} {
  const raw = value.trim()
  if (!raw) {
    return { ok: true, value: '[]', groups: [] }
  }

  try {
    const groups = normalizeAutoDisablePolicyGroups(JSON.parse(raw))
    return {
      ok: true,
      value: JSON.stringify(groups, null, 2),
      groups,
    }
  } catch (error) {
    return {
      ok: false,
      value: raw,
      groups: [],
      error: error instanceof Error ? error.message : String(error),
    }
  }
}

export function parseAutoDisablePolicyGroups(
  value: string | undefined | null
): AutoDisablePolicyGroup[] {
  const result = normalizeAutoDisablePolicyGroupsString(String(value ?? ''))
  return result.ok ? result.groups : []
}

export function serializeAutoDisablePolicyGroups(
  groups: AutoDisablePolicyGroup[]
): string {
  return JSON.stringify(
    groups.map((group) => ({
      id: group.id,
      name: group.name,
      enabled: group.enabled !== false,
      keywords: Array.isArray(group.keywords)
        ? group.keywords.map((keyword) => String(keyword ?? ''))
        : [],
    })),
    null,
    2
  )
}

export function createAutoDisablePolicyGroup(
  name: string
): AutoDisablePolicyGroup {
  return {
    id: `policy_${Date.now().toString(36)}_${Math.random()
      .toString(36)
      .slice(2, 8)}`,
    name,
    enabled: true,
    keywords: [],
  }
}

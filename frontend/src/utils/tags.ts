// Parse a free-text tags field ("fr, vpn , fast") into a clean string array.
export function parseTags(input: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of input.split(',')) {
    const tag = raw.trim().replace(/\s+/g, ' ')
    if (!tag) continue
    const key = tag.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(tag)
  }
  return out
}

// Join tags back into the comma-separated form used by the text input.
export function formatTags(tags: string[] | undefined): string {
  return (tags ?? []).join(', ')
}

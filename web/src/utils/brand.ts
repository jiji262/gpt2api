export interface BrandParts {
  brand: string
  qq: string
  qqLabel: string
  repo: string
  repoLabel: string
  picUrl: string
  picText: string
  picLabel: string
  sep: string
}

export function brandParts(): BrandParts {
  return {
    brand: '',
    qq: '',
    qqLabel: '',
    repo: '',
    repoLabel: '',
    picUrl: '',
    picText: '',
    picLabel: '',
    sep: '',
  }
}

export function brandPlainText(): string {
  return ''
}

export function printBrandToConsole(): void {
  return undefined
}

export function startBrandGuard(): void {
  return undefined
}

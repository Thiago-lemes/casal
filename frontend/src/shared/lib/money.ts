/** Dinheiro é sempre um inteiro em centavos. Texto só na borda: mostrar e digitar. */

export const MAX_CENTS = 1_000_000_000

export function formatBRL(cents: number, lang = 'pt-BR'): string {
  return new Intl.NumberFormat(lang, { style: 'currency', currency: 'BRL' }).format(cents / 100)
}

/**
 * Texto digitado → centavos inteiros, ou `null` se inválido. Aceita `1.234,56`, `1234,56`, `1234.56`, `12`, `0,5`.
 * Trabalha com o texto (parte inteira e decimais separadas), nunca com `parseFloat * 100`.
 */
export function parseMoney(text: string): number | null {
  const s = text.replace(/^R\$/i, '').replace(/\s/g, '')
  if (!/^[\d.,]+$/.test(s)) return null

  const lastComma = s.lastIndexOf(',')
  const lastDot = s.lastIndexOf('.')
  let whole: string
  let frac = ''

  if (lastComma >= 0) {
    // a vírgula é o decimal; pontos antes dela são milhar
    if (s.indexOf(',') !== lastComma) return null
    whole = s.slice(0, lastComma)
    frac = s.slice(lastComma + 1)
  } else if (lastDot >= 0) {
    const after = s.length - lastDot - 1
    const single = s.indexOf('.') === lastDot
    if (single && after <= 2) {
      whole = s.slice(0, lastDot)
      frac = s.slice(lastDot + 1)
    } else {
      whole = s // ponto de milhar
    }
  } else {
    whole = s
  }

  if (whole === '' || frac.length > 2) return null
  if (whole.includes('.')) {
    if (!/^[1-9]\d{0,2}(\.\d{3})+$/.test(whole)) return null
    whole = whole.replace(/\./g, '')
  } else if (!/^\d+$/.test(whole)) return null
  if (frac !== '' && !/^\d+$/.test(frac)) return null
  if (whole.length > 12) return null

  return Number(whole) * 100 + Number(frac.padEnd(2, '0') || '0')
}

/** Pontos-base (1% = 100) → "62,5%". */
export function splitLabel(points: number, lang = 'pt-BR'): string {
  return `${new Intl.NumberFormat(lang, { maximumFractionDigits: 2 }).format(points / 100)}%`
}

/** Dica de parcelamento. O valor que vale é o do servidor; isto só ajuda a pessoa a conferir. */
export function installmentPreview(totalCents: number, count: number): { each: number; exact: boolean } {
  return { each: Math.floor(totalCents / count), exact: totalCents % count === 0 }
}

/** Centavos → texto editável ("1234,56"), para preencher um campo ao editar. */
export function centsToInput(cents: number): string {
  const whole = Math.floor(cents / 100)
  const rest = cents % 100
  return rest === 0 ? String(whole) : `${whole},${String(rest).padStart(2, '0')}`
}

/** O valor entendido, mostrado sob o campo: "1.234" pode ser 1.234 reais ou 1,234, e a pessoa confere. */
export function interpretedHint(t: (key: string, o?: Record<string, unknown>) => string, cents: number | null, lang: string): string | undefined {
  return cents !== null && cents > 0 && cents <= MAX_CENTS ? t('form.interpreted', { amount: formatBRL(cents, lang) }) : undefined
}

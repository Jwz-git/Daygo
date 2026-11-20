import type * as Backend from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type { WireDTO } from '@/api/dto'

/** Derive API signatures from generated bindings. Arguments/results are
 * JSON data, not instances of Wails' generated model classes. */
export type GeneratedBindings<Names extends keyof typeof Backend> = Partial<{
  [Name in Names]: (typeof Backend)[Name] extends (...args: infer Args) => Promise<infer Result>
    ? (...args: { [Index in keyof Args]: WireDTO<Args[Index]> }) => Promise<WireDTO<Result>>
    : never
}>

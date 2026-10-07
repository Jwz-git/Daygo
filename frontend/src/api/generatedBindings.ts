import type * as Backend from '../../wailsjs/go/app/Backend'
import type { WireDTO } from '@/api/dto'

/** Derive window.go signatures from generated bindings. Arguments/results are
 * JSON data, not instances of Wails' generated model classes. */
export type GeneratedBindings<Names extends keyof typeof Backend> = Partial<{
  [Name in Names]: (typeof Backend)[Name] extends (...args: infer Args) => Promise<infer Result>
    ? (...args: { [Index in keyof Args]: WireDTO<Args[Index]> }) => Promise<WireDTO<Result>>
    : never
}>

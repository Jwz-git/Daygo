import {
  AddProvider,
  DeleteProvider,
  DeleteProviderSecret,
  GetProviderRouting,
  ListProviderModels,
  ListProviders,
  SetProviderRouting,
  SetProviderSecret,
  TestProvider,
  UpdateProvider,
} from '../../wailsjs/go/app/Backend'

import type {
  ProviderDTO,
  ProviderInput,
  ProviderModelsRequest,
  ProviderModelsResult,
  ProviderProtocol,
  ProviderRoutingDTO,
  ProviderTestDraft,
  ProviderTestResult,
} from '@/api/dto'

/** Thrown when the page runs in a plain browser, outside the Wails WebView. */
export const WAILS_UNAVAILABLE = 'wails_unavailable'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/*
 * Dev-browser stand-in: an in-memory provider list, so the settings UI is
 * exercisable outside the WebView. It never runs in production
 * (import.meta.env.DEV) and never substitutes for the backend's validation.
 */
interface DevState {
  providers: ProviderDTO[]
  routing: ProviderRoutingDTO
  nextId: number
}

let dev: DevState | null = null

function devState(): DevState {
  // One seeded provider so the chat flow is exercisable in the dev browser:
  // a conversation without any configured provider cannot send.
  dev ??= {
    providers: [
      {
        id: 'dev-1',
        displayName: '开发供应商',
        protocol: 'openai',
        endpoint: 'https://example.invalid/v1',
        model: 'dev-model',
        hasSecret: false,
      },
    ],
    routing: { chain: ['dev-1'] },
    nextId: 2,
  }
  return dev
}

export async function listProviders(): Promise<ProviderDTO[]> {
  if (hasBridge()) return (await ListProviders()) as unknown as ProviderDTO[]
  if (import.meta.env.DEV) return [...devState().providers]
  throw new Error(WAILS_UNAVAILABLE)
}

export async function addProvider(input: ProviderInput): Promise<string> {
  if (hasBridge()) return AddProvider(input as never)
  if (import.meta.env.DEV) {
    const state = devState()
    const id = `dev-${state.nextId++}`
    state.providers = [
      ...state.providers,
      {
        id,
        displayName: input.displayName,
        protocol: input.protocol,
        endpoint: input.endpoint,
        model: input.model,
        hasSecret: input.secret !== '',
      },
    ]
    if (state.routing.chain.length === 0) state.routing = { chain: [id] }
    return id
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function updateProvider(id: string, input: ProviderInput): Promise<void> {
  if (hasBridge()) return UpdateProvider(id, input as never)
  if (import.meta.env.DEV) {
    const state = devState()
    state.providers = state.providers.map((provider) =>
      provider.id === id
        ? {
            ...provider,
            displayName: input.displayName,
            protocol: input.protocol,
            endpoint: input.endpoint,
            model: input.model,
            hasSecret: input.secret !== '' ? true : provider.hasSecret,
          }
        : provider,
    )
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function deleteProvider(id: string): Promise<void> {
  if (hasBridge()) return DeleteProvider(id)
  if (import.meta.env.DEV) {
    const state = devState()
    state.providers = state.providers.filter((provider) => provider.id !== id)
    state.routing = { chain: state.routing.chain.filter((entry) => entry !== id) }
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function getProviderRouting(): Promise<ProviderRoutingDTO> {
  if (hasBridge()) return (await GetProviderRouting()) as unknown as ProviderRoutingDTO
  if (import.meta.env.DEV) return { chain: [...devState().routing.chain] }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function setProviderRouting(routing: ProviderRoutingDTO): Promise<void> {
  if (hasBridge()) return SetProviderRouting(routing as never)
  if (import.meta.env.DEV) {
    devState().routing = { chain: [...routing.chain] }
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function setProviderSecret(id: string, secret: string): Promise<void> {
  if (hasBridge()) return SetProviderSecret(id, secret)
  if (import.meta.env.DEV) {
    const state = devState()
    state.providers = state.providers.map((provider) =>
      provider.id === id ? { ...provider, hasSecret: true } : provider,
    )
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export async function deleteProviderSecret(id: string): Promise<void> {
  if (hasBridge()) return DeleteProviderSecret(id)
  if (import.meta.env.DEV) {
    const state = devState()
    state.providers = state.providers.map((provider) =>
      provider.id === id ? { ...provider, hasSecret: false } : provider,
    )
    return
  }
  throw new Error(WAILS_UNAVAILABLE)
}

/** Probe a saved provider; its key comes from the keychain, not this call. */
export async function testProvider(id: string): Promise<ProviderTestResult> {
  if (hasBridge()) return (await TestProvider(id)) as unknown as ProviderTestResult
  throw new Error(WAILS_UNAVAILABLE)
}

/** List models for a saved provider or an unsaved draft. */
export async function listProviderModels(
  request: ProviderModelsRequest,
): Promise<ProviderModelsResult> {
  if (hasBridge()) {
    return (await ListProviderModels(request as never)) as unknown as ProviderModelsResult
  }
  if (import.meta.env.DEV) {
    // A plausible fake list keeps the dropdown flow exercisable in a browser.
    return {
      ok: true,
      models: ['dev-mini', 'dev-standard', 'dev-large'],
      errorCode: '',
      message: '',
    }
  }
  throw new Error(WAILS_UNAVAILABLE)
}

export type { ProviderProtocol, ProviderTestDraft, ProviderTestResult }

/// <reference types="vite/client" />

// Build-time flag defined in vite.config.ts. False in the production installer,
// true in dev and in an opt-in VITE_DAYGO_TEST_TOOLS=1 build. Gates the
// test-tools surface so it can be tree-shaken out of shipped packages.
declare const __DAYGO_TEST_TOOLS__: boolean

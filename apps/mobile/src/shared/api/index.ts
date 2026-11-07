import { config } from '@/shared/config/env';

import type { LenapayApi } from './client';
import { createLiveApi } from './live/liveApi';
import { MOCK_EMAIL_CODE } from './mock/backend';
import { createMockApi } from './mock/mockApi';

/**
 * The app's API, chosen once by EXPO_PUBLIC_API_MODE: `mock` (default, in-memory fixtures)
 * or `live` (the Lenapay API). Features import `api` and never know which one they got.
 */
export const api: LenapayApi = config.apiMode === 'live' ? createLiveApi() : createMockApi();

export const isMockApi = config.apiMode === 'mock';

/** The email code the mock backend accepts; null in live mode (shown only as a demo hint). */
export const demoEmailCode = isMockApi ? MOCK_EMAIL_CODE : null;

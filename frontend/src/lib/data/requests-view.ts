import { createViewStore } from './view-store';

export type RequestsView = 'yours' | 'all';

const { store, setView } = createViewStore<RequestsView>('yours');

export const requestsView = store;
export const setRequestsView = setView;

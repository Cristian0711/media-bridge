import { createViewStore } from './view-store';

export type LibraryView = 'yours' | 'all';

const { store, setView } = createViewStore<LibraryView>('yours');

export const libraryView = store;
export const setLibraryView = setView;

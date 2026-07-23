// Adapted from webgl-liquid-glass (https://github.com/clayharmon/webgl-liquid-glass)
// Copyright (c) 2026 Clay Harmon — MIT License. React-facing types removed;
// only the engine types are kept (the Svelte component defines its own props).

import type { Component, ComponentType, SvelteComponent } from 'svelte';

export interface NavItem {
  id: string;
  label: string;
  // Accept both legacy class components (lucide-svelte icons are `typeof Icon`)
  // and Svelte 5 function components.
  icon?: ComponentType<SvelteComponent> | Component<any>;
}

export interface SpringState {
  current: number;
  target: number;
  velocity: number;
}

export interface RenderParams {
  time: number;
  lightPos: [number, number];
  pillX: number;
  pillWidth: number;
  pillHeight: number;
  navRadius: number;
  transitionVel: number;
  pressAmt: number;
  tintColor: [number, number, number];
}

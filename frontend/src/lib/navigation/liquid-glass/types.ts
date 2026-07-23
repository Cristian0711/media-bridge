// Adapted from webgl-liquid-glass (https://github.com/clayharmon/webgl-liquid-glass)
// Copyright (c) 2026 Clay Harmon — MIT License. React-facing types removed;
// only the engine types are kept (the Svelte component defines its own props).

import type { Component } from 'svelte';

export interface NavItem {
  id: string;
  label: string;
  // Lucide-svelte icon component (or any Svelte component taking size/class).
  icon?: Component<any>;
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

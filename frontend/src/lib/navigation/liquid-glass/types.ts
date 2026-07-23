// Adapted from webgl-liquid-glass (https://github.com/clayharmon/webgl-liquid-glass)
// Copyright (c) 2026 Clay Harmon — MIT License. React-facing types removed;
// only the engine types are kept (the Svelte component defines its own props).

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

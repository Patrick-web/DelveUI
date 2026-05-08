<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from "svelte";

  export let min = 0;
  export let max = 100;
  export let value = 0;

  const dispatch = createEventDispatcher();

  let trackEl: HTMLDivElement;
  let fillEl: HTMLDivElement;
  let thumbEl: HTMLDivElement;
  let dragging = false;
  let hover = false;

  function fraction(v: number): number {
    if (max <= min) return 0;
    return Math.max(0, Math.min(1, (v - min) / (max - min)));
  }

  function render(v = value) {
    const pct = fraction(v);
    if (fillEl) fillEl.style.width = `${pct * 100}%`;
    if (thumbEl) thumbEl.style.left = `${pct * 100}%`;
  }

  $: render(value);

  function valueFromClientX(clientX: number): number {
    if (!trackEl) return value;
    const rect = trackEl.getBoundingClientRect();
    const thumbRadius = 6;
    const usableLeft = rect.left + thumbRadius;
    const usableWidth = rect.width - thumbRadius * 2;
    const ratio = Math.max(0, Math.min(1, (clientX - usableLeft) / usableWidth));
    return Math.round(min + ratio * (max - min));
  }

  function apply(v: number) {
    const clamped = Math.max(min, Math.min(max, v));
    if (clamped !== value) {
      value = clamped;
      render(clamped);
      dispatch("change", { value: clamped });
      dispatch("input", { value: clamped });
    }
  }

  function onTrackMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    apply(valueFromClientX(e.clientX));
    dragging = true;
  }

  function onThumbMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    dragging = true;
  }

  function onMouseMove(e: MouseEvent) {
    if (!dragging) return;
    apply(valueFromClientX(e.clientX));
  }

  function onMouseUp() {
    dragging = false;
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "ArrowLeft" || e.key === "ArrowDown") {
      e.preventDefault();
      apply(value - 1);
    } else if (e.key === "ArrowRight" || e.key === "ArrowUp") {
      e.preventDefault();
      apply(value + 1);
    }
  }

  onMount(() => {
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    render(value);
  });

  onDestroy(() => {
    document.removeEventListener("mousemove", onMouseMove);
    document.removeEventListener("mouseup", onMouseUp);
  });
</script>

<div
  class="slider"
  class:dragging
  class:hover={hover && !dragging}
  role="slider"
  tabindex="0"
  aria-valuemin={min}
  aria-valuemax={max}
  aria-valuenow={value}
  on:keydown={onKeyDown}
  on:mouseenter={() => (hover = true)}
  on:mouseleave={() => (hover = false)}
>
  <div class="track" bind:this={trackEl} on:mousedown={onTrackMouseDown} role="presentation">
    <div class="track-fill" bind:this={fillEl}></div>
    <div
      class="thumb"
      bind:this={thumbEl}
      on:mousedown={onThumbMouseDown}
      role="presentation"
    ></div>
  </div>
</div>

<style>
  .slider {
    flex: 1;
    max-width: 200px;
    display: flex;
    align-items: center;
    height: 24px;
    cursor: pointer;
    outline: none;
    border-radius: var(--radius-sm);
  }
  .slider:focus-visible {
    box-shadow: 0 0 0 2px var(--accent-subtle);
  }

  .track {
    position: relative;
    width: 100%;
    height: 6px;
    background: var(--border);
    border-radius: 3px;
    overflow: visible;
  }

  .track-fill {
    position: absolute;
    left: 0;
    top: 0;
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    width: 0%;
    clip-path: polygon(
      0% 0%,
      100% 0%,
      100% 100%,
      0% 100%,
      100% 50%
    );
  }

  .thumb {
    position: absolute;
    top: 50%;
    width: 12px;
    height: 12px;
    margin-left: -6px;
    margin-top: -6px;
    background: var(--accent);
    border: 2px solid var(--bg);
    border-radius: 50%;
    box-shadow: 0 0 0 1px var(--accent-subtle);
    z-index: 2;
    transition: box-shadow 80ms;
  }
  .slider.dragging .thumb,
  .slider:hover .thumb {
    box-shadow: 0 0 0 2px var(--accent-subtle), 0 1px 4px rgba(0, 0, 0, 0.4);
  }
</style>

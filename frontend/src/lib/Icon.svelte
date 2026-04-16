<script lang="ts">
  import Iconify from "@iconify/svelte";
  export let icon: string;
  export let size: number = 14;
  export let color: string | undefined = undefined;
</script>

<span class="icon" style:width="{size}px" style:height="{size}px" style:color={color}>
  <Iconify {icon} width={size} height={size} />
</span>

<style>
  .icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
</style>
